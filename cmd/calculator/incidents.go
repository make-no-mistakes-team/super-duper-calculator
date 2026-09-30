package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

const (
	incidentWindow   = 60 * time.Second
	incidentCooldown = 120 * time.Second
	incidentTTL      = 10 * time.Second
	incidentRuleID   = "comic_incident"
)

type incidentService struct {
	db  *sql.DB
	now func() time.Time
}

func newIncidentService(db *sql.DB, enabled bool) *incidentService {
	if !enabled {
		return nil
	}
	return &incidentService{db: db, now: time.Now}
}

// Process may announce only the newly committed, owned action passed by the
// calculation handler. Caller-supplied outcome and timestamp are not trusted.
// The sequence bound excludes actions committed after this one, even if their
// optional processing finishes first.
func (s *incidentService) Process(ctx context.Context, owner string, record contracts.CalculationRecord) ([]contracts.FunEvent, error) {
	if s == nil {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var seq int64
	var outcomeJSON, timestamp string
	if err := s.db.QueryRowContext(ctx, `
		SELECT seq, outcome_json, created_at FROM calculations
		WHERE session_id = ? AND id = ?`, owner, record.ID).Scan(&seq, &outcomeJSON, &timestamp); err != nil {
		return nil, fmt.Errorf("find owned incident action: %w", err)
	}
	if !divisionByZero(outcomeJSON) {
		return nil, nil
	}
	acceptedAt, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return nil, fmt.Errorf("parse incident action timestamp: %w", err)
	}
	acceptedAt = acceptedAt.UTC()
	expiresAt := acceptedAt.Add(incidentTTL)
	if !incidentLive(s.now(), acceptedAt, expiresAt) {
		return nil, nil
	}

	// Timestamps are assigned before INSERT, so sequence order need not match
	// timestamp order. Search both sides of this action, then require a single
	// sixty-second span containing it. SQL's unixepoch() range is padded for
	// subsecond rounding; exact inclusion and the error code are checked below.
	// Reading outside the write transaction keeps the single DB connection free
	// and the transaction short. Its claim remains atomic against other writers.
	earliest := acceptedAt.Add(-incidentWindow)
	latest := acceptedAt.Add(incidentWindow)
	rows, err := s.db.QueryContext(ctx, `
		SELECT outcome_json, created_at FROM calculations
		WHERE session_id = ? AND seq < ?
			AND unixepoch(created_at) BETWEEN ? AND ?
		ORDER BY seq DESC`, owner, seq,
		earliest.Unix()-1, latest.Unix()+1)
	if err != nil {
		return nil, fmt.Errorf("read recent incident actions: %w", err)
	}
	lower, upper := 0, 0
	var closestLower, closestUpper time.Time
	eligible := false
	for rows.Next() {
		var candidateJSON, candidateTimestamp string
		if err := rows.Scan(&candidateJSON, &candidateTimestamp); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan recent incident action: %w", err)
		}
		if !divisionByZero(candidateJSON) {
			continue
		}
		candidateAt, err := time.Parse(time.RFC3339Nano, candidateTimestamp)
		if err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("parse recent incident timestamp: %w", err)
		}
		if candidateAt.Before(earliest) || candidateAt.After(latest) {
			continue
		}
		if !candidateAt.After(acceptedAt) {
			lower++
			if lower == 1 || candidateAt.After(closestLower) {
				closestLower = candidateAt
			}
		} else {
			upper++
			if upper == 1 || candidateAt.Before(closestUpper) {
				closestUpper = candidateAt
			}
		}
		// Two candidates on one side already fit the same sixty-second
		// window. A pair straddling this action must fit together, not merely
		// fall within the surrounding 120-second neighborhood.
		eligible = lower == 2 || upper == 2 ||
			(lower > 0 && upper > 0 && closestUpper.Sub(closestLower) <= incidentWindow)
		if eligible {
			break
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate recent incident actions: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close recent incident actions: %w", err)
	}
	if !eligible || !incidentLive(s.now(), acceptedAt, expiresAt) {
		return nil, nil
	}

	// A single conditional upsert arbitrates the per-owner cooldown. The
	// transaction also lets a scene that expired before commit roll back its
	// claim. Neither failure nor suppression modifies the committed action.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin incident claim: %w", err)
	}
	defer tx.Rollback()
	if !incidentLive(s.now(), acceptedAt, expiresAt) {
		return nil, nil
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO personal_effects
			(session_id, last_calculation_id, last_scene_at_ns)
		VALUES (?, ?, ?)
		ON CONFLICT (session_id) DO UPDATE SET
			last_calculation_id = excluded.last_calculation_id,
			last_scene_at_ns = excluded.last_scene_at_ns
		WHERE ? > (SELECT seq FROM calculations
			WHERE session_id = personal_effects.session_id
				AND id = personal_effects.last_calculation_id)
			AND excluded.last_scene_at_ns > personal_effects.last_scene_at_ns
			AND excluded.last_scene_at_ns - personal_effects.last_scene_at_ns >= ?`,
		owner, record.ID, acceptedAt.UnixNano(), seq, int64(incidentCooldown))
	if err != nil {
		return nil, fmt.Errorf("claim incident cooldown: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check incident claim: %w", err)
	}
	if inserted == 0 || !incidentLive(s.now(), acceptedAt, expiresAt) {
		return nil, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit incident claim: %w", err)
	}
	return []contracts.FunEvent{{
		ID:        record.ID + ":" + incidentRuleID,
		RuleID:    incidentRuleID,
		Kind:      "scene",
		Scope:     "personal",
		Params:    map[string]any{},
		CreatedAt: acceptedAt,
		ExpiresAt: expiresAt,
	}}, nil
}

func divisionByZero(raw string) bool {
	var outcome struct {
		Kind  contracts.OutcomeKind `json:"kind"`
		Error *struct {
			Code contracts.MathErrorCode `json:"code"`
		} `json:"error"`
	}
	return json.Unmarshal([]byte(raw), &outcome) == nil &&
		outcome.Kind == contracts.OutcomeError && outcome.Error != nil && outcome.Error.Code == contracts.ErrorDivisionByZero
}

func incidentLive(now, acceptedAt, expiresAt time.Time) bool {
	return !now.Before(acceptedAt) && now.Before(expiresAt)
}
