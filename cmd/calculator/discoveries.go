package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/discovery"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

const (
	discoveryBatchSize  = 128
	discoveryCommentTTL = 30 * time.Second
)

type discoveryService struct {
	db             *sql.DB
	rules          discovery.Rules
	catalog        []contracts.DiscoveryDefinition
	previousLimit  int
	followupCounts []int64
	now            func() time.Time
}

func newDiscoveryService(db *sql.DB, enabled bool) *discoveryService {
	if !enabled {
		return nil
	}
	config := discovery.DefaultConfig()
	rules, err := discovery.New(config)
	if err != nil {
		panic(fmt.Sprintf("invalid default discovery rules: %v", err))
	}
	authored := discovery.Catalog()
	catalog := make([]contracts.DiscoveryDefinition, len(authored))
	for i, definition := range authored {
		catalog[i] = contracts.DiscoveryDefinition{
			ID: definition.ID,
			RU: contracts.DiscoveryText{
				Name: definition.RU.Name, Description: definition.RU.Description, Comment: definition.RU.Comment,
			},
			EN: contracts.DiscoveryText{
				Name: definition.EN.Name, Description: definition.EN.Description, Comment: definition.EN.Comment,
			},
		}
	}
	return &discoveryService{
		db: db, rules: rules, catalog: catalog,
		previousLimit:  max(config.PeerReviewCount-1, 0),
		followupCounts: []int64{50, 100},
		now:            time.Now,
	}
}

// Process is only for a newly committed calculation. It finds the first eligible
// owned action for each discovery before awarding the current one, so completion
// order cannot move an achievement's earned time to a later action.
func (s *discoveryService) Process(ctx context.Context, owner string, record contracts.CalculationRecord) ([]contracts.Achievement, []contracts.FunEvent, error) {
	if s == nil {
		return nil, nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	var through int64
	var timestamp string
	if err := s.db.QueryRowContext(ctx,
		`SELECT seq, created_at FROM calculations WHERE session_id = ? AND id = ?`,
		owner, record.ID).Scan(&through, &timestamp); err != nil {
		return nil, nil, fmt.Errorf("find owned discovery action: %w", err)
	}
	awards, accepted, err := s.reconcile(ctx, owner, through, record.ID)
	if err != nil {
		return nil, nil, err
	}
	// An expired comment is not revived if optional processing finished late.
	now := s.now()
	events := make([]contracts.FunEvent, 0, len(awards))
	for _, award := range awards {
		expiresAt := award.EarnedAt.Add(discoveryCommentTTL)
		if award.EarnedAt.After(now) || !expiresAt.After(now) {
			continue
		}
		events = append(events, contracts.FunEvent{
			ID:        record.ID + ":" + award.ID,
			RuleID:    award.ID,
			Kind:      "comment",
			Scope:     "personal",
			Params:    map[string]any{},
			CreatedAt: award.EarnedAt,
			ExpiresAt: expiresAt,
		})
	}
	for _, threshold := range s.followupCounts {
		if accepted != threshold {
			continue
		}
		acceptedAt, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return nil, nil, fmt.Errorf("parse discovery followup timestamp: %w", err)
		}
		expiresAt := acceptedAt.Add(discoveryCommentTTL)
		if !acceptedAt.After(now) && expiresAt.After(now) {
			events = append(events, contracts.FunEvent{
				ID:     record.ID + ":touch_grass:" + fmt.Sprint(threshold),
				RuleID: "touch_grass", Kind: "comment", Scope: "personal",
				Params: map[string]any{"count": threshold}, CreatedAt: acceptedAt, ExpiresAt: expiresAt,
			})
		}
	}
	return awards, events, nil
}

// Collection repairs missing awards from authoritative history without
// announcing them, then returns the persisted collection and authored copy.
func (s *discoveryService) Collection(ctx context.Context, owner string) ([]contracts.Achievement, []contracts.DiscoveryDefinition, error) {
	if s == nil {
		return nil, nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	var through int64
	if err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(seq), 0) FROM calculations WHERE session_id = ?", owner).Scan(&through); err != nil {
		return nil, nil, fmt.Errorf("find owned discovery history: %w", err)
	}
	if _, _, err := s.reconcile(ctx, owner, through, ""); err != nil {
		return nil, nil, err
	}
	awards, err := storage.ListAchievements(ctx, s.db, owner)
	if err != nil {
		return nil, nil, fmt.Errorf("list owned discoveries: %w", err)
	}
	return awards, append([]contracts.DiscoveryDefinition(nil), s.catalog...), nil
}

// reconcile resumes a durable owner/sequence prefix in ascending order. Batches
// close their rows before atomically committing grants and progress. A competing
// reconciler's checkpoint invalidates the entire uncommitted batch.
// Only awards backed by currentID may be returned to an active calculation;
// historical catch-up, including a prior action found by Process, stays quiet.
func (s *discoveryService) reconcile(ctx context.Context, owner string, through int64, currentID string) ([]contracts.Achievement, int64, error) {
	var currentAwards []contracts.Achievement
	for {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		progress, err := storage.ReadDiscoveryProgress(ctx, s.db, owner)
		if err != nil {
			return nil, 0, fmt.Errorf("read owned discovery progress: %w", err)
		}
		if progress.LastSequence >= through {
			accepted := progress.AcceptedCount
			if currentID != "" && progress.LastSequence > through {
				// A delayed action still uses its own authoritative ordinal.
				// Normal processing needs no count query; only the newer suffix
				// is counted when another reconciliation has passed this action.
				var newer int64
				if err := s.db.QueryRowContext(ctx, `
					SELECT COUNT(*) FROM calculations
					WHERE session_id = ? AND seq > ? AND seq <= ?`,
					owner, through, progress.LastSequence).Scan(&newer); err != nil {
					return nil, 0, fmt.Errorf("find discovery action ordinal: %w", err)
				}
				accepted -= newer
			}
			return currentAwards, accepted, nil
		}
		collection, err := storage.ListAchievements(ctx, s.db, owner)
		if err != nil {
			return nil, 0, fmt.Errorf("read owned discoveries: %w", err)
		}
		seen := make(map[string]bool, len(s.catalog))
		for _, award := range collection {
			seen[award.ID] = true
		}
		previous, err := s.predecessors(ctx, owner, progress.LastSequence)
		if err != nil {
			return nil, 0, err
		}
		rows, err := s.db.QueryContext(ctx, `
			SELECT `+storage.SequencedCalculationRecordColumns+`
			FROM calculations
			WHERE session_id = ? AND seq > ? AND seq <= ?
			ORDER BY seq ASC LIMIT ?`, owner, progress.LastSequence, through, discoveryBatchSize)
		if err != nil {
			return nil, 0, fmt.Errorf("read owned discovery history: %w", err)
		}
		var batch []storage.DiscoveryGrant
		next := progress
		for rows.Next() {
			if err := ctx.Err(); err != nil {
				_ = rows.Close()
				return nil, 0, err
			}
			var seq int64
			record, err := storage.ReadCalculationRecord(rows, &seq, storage.DiscoveryFacts)
			if err != nil {
				_ = rows.Close()
				return nil, 0, fmt.Errorf("decode discovery calculation: %w", err)
			}
			next.LastSequence = seq
			next.AcceptedCount++
			matched := s.rules.Match(discovery.Input{
				Calculation: record, AcceptedCount: next.AcceptedCount, Previous: previous,
			})
			var first []string
			for _, id := range matched {
				if !seen[id] {
					seen[id] = true
					first = append(first, id)
				}
			}
			if len(first) != 0 {
				batch = append(batch, storage.DiscoveryGrant{CalculationID: record.ID, IDs: first})
			}
			// The configured streak length bounds memory even across many
			// history batches; newest prior action stays at index zero.
			if s.previousLimit != 0 {
				if len(previous) < s.previousLimit {
					previous = append(previous, contracts.CalculationRecord{})
				}
				copy(previous[1:], previous[:len(previous)-1])
				previous[0] = record
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, 0, fmt.Errorf("iterate discovery history: %w", err)
		}
		if err := rows.Close(); err != nil {
			return nil, 0, fmt.Errorf("close discovery history: %w", err)
		}
		if next == progress {
			return nil, 0, errors.New("discovery history ended before snapshot")
		}
		awarded, err := storage.CommitDiscoveryProgress(ctx, s.db, owner, progress, next, batch)
		if errors.Is(err, storage.ErrDiscoveryProgressChanged) {
			continue
		}
		if err != nil {
			return nil, 0, fmt.Errorf("commit owned discoveries: %w", err)
		}
		currentAwards = append(currentAwards, awarded[currentID]...)
	}
}

// predecessors recovers only the bounded streak context immediately before the
// checkpoint. History and awards remain authoritative; no history is copied into
// progress, and optional corrupt facts keep the same tolerance as catch-up.
func (s *discoveryService) predecessors(ctx context.Context, owner string, through int64) ([]contracts.CalculationRecord, error) {
	previous := make([]contracts.CalculationRecord, 0, s.previousLimit)
	if through == 0 || s.previousLimit == 0 {
		return previous, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+storage.CalculationRecordColumns+`
		FROM calculations WHERE session_id = ? AND seq <= ?
		ORDER BY seq DESC LIMIT ?`, owner, through, s.previousLimit)
	if err != nil {
		return nil, fmt.Errorf("read discovery predecessors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		record, err := storage.ReadCalculationRecord(rows, nil, storage.DiscoveryFacts)
		if err != nil {
			return nil, fmt.Errorf("decode discovery predecessor: %w", err)
		}
		previous = append(previous, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discovery predecessors: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close discovery predecessors: %w", err)
	}
	return previous, nil
}
