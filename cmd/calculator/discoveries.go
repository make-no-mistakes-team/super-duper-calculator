package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
			ID: definition.ID, Secret: definition.Secret,
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
		var state discovery.State
		if progress.StateJSON != "" {
			if err := json.Unmarshal([]byte(progress.StateJSON), &state); err != nil {
				return nil, 0, fmt.Errorf("decode discovery state: %w", err)
			}
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
		type action struct {
			sequence int64
			record   contracts.CalculationRecord
		}
		actions := make([]action, 0, discoveryBatchSize)
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
			actions = append(actions, action{seq, record})
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, 0, fmt.Errorf("iterate discovery history: %w", err)
		}
		if err := rows.Close(); err != nil {
			return nil, 0, fmt.Errorf("close discovery history: %w", err)
		}
		// The sole SQLite connection is free before indexed evidence reads.
		// Batch-local caches include the uncommitted prefix without history scans.
		type valueKey struct{ semantics, value string }
		type expressionKey struct{ semantics, expression string }
		values := make(map[valueKey]discovery.Evidence)
		expressions := make(map[expressionKey]time.Time)
		var evidenceUpdates []storage.DiscoveryEvidence
		for _, action := range actions {
			record := action.record
			next.LastSequence = action.sequence
			next.AcceptedCount++
			var evidence discovery.Evidence
			if record.Outcome.Kind == contracts.OutcomeSuccess && record.Facts != nil {
				vkey := valueKey{record.Context.SemanticsVersion, record.Outcome.Value}
				ekey := expressionKey{record.Context.SemanticsVersion, record.Facts.NormalizedExpression}
				var ok bool
				evidence, ok = values[vkey]
				earliest, expressionLoaded := expressions[ekey]
				if !ok || !expressionLoaded {
					stored, err := storage.ReadDiscoveryEvidence(ctx, s.db, owner,
						vkey.semantics, vkey.value, ekey.expression)
					if err != nil {
						return nil, 0, fmt.Errorf("read discovery evidence: %w", err)
					}
					if !ok {
						evidence = stored
					}
					if !expressionLoaded {
						earliest = stored.Earliest
					}
				}
				evidence.Earliest = earliest
				matched := s.rules.Match(discovery.Input{Calculation: record,
					AcceptedCount: next.AcceptedCount, State: &state, Evidence: evidence})
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
				route := record.Facts.StructureIdentity
				if route != "" && len(evidence.Routes) < 3 && !slices.Contains(evidence.Routes, route) {
					evidence.Routes = append(evidence.Routes, route)
				}
				trig := discovery.TrigVariant(record.Facts)
				evidence.Trig |= trig
				if earliest.IsZero() || record.CreatedAt.Before(earliest) {
					earliest = record.CreatedAt
				}
				values[vkey] = evidence
				expressions[ekey] = earliest
				evidenceUpdates = append(evidenceUpdates, storage.DiscoveryEvidence{
					SemanticsVersion: vkey.semantics, Value: vkey.value,
					Structure: route, Trig: trig, Expression: ekey.expression, Earliest: earliest,
				})
			} else {
				var first []string
				for _, id := range s.rules.Match(discovery.Input{Calculation: record,
					AcceptedCount: next.AcceptedCount, State: &state}) {
					if !seen[id] {
						seen[id] = true
						first = append(first, id)
					}
				}
				if len(first) != 0 {
					batch = append(batch, storage.DiscoveryGrant{CalculationID: record.ID, IDs: first})
				}
			}
			state.Advance(record)
		}
		if next == progress {
			return nil, 0, errors.New("discovery history ended before snapshot")
		}
		stateJSON, err := json.Marshal(state)
		if err != nil {
			return nil, 0, fmt.Errorf("encode discovery state: %w", err)
		}
		next.StateJSON = string(stateJSON)
		awarded, err := storage.CommitDiscoveryProgress(ctx, s.db, owner, progress, next, batch, evidenceUpdates...)
		if errors.Is(err, storage.ErrDiscoveryProgressChanged) {
			continue
		}
		if err != nil {
			return nil, 0, fmt.Errorf("commit owned discoveries: %w", err)
		}
		currentAwards = append(currentAwards, awarded[currentID]...)
	}
}
