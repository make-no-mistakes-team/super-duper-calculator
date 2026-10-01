package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/discovery"
)

// ErrDiscoveryProgressChanged means another reconciliation committed the prefix.
// The caller must reload progress and re-evaluate its uncommitted batch.
var ErrDiscoveryProgressChanged = errors.New("discovery progress changed")

// DiscoveryProgress describes a completely evaluated owner history prefix.
// AcceptedCount is the owner's ordinal, not the database-wide sequence.
type DiscoveryProgress struct {
	LastSequence  int64
	AcceptedCount int64
	StateJSON     string
}

type DiscoveryGrant struct {
	CalculationID string
	IDs           []string
}

// DiscoveryEvidence is a bounded update to indexed, parser-derived history.
type DiscoveryEvidence struct {
	SemanticsVersion string
	Value            string
	Structure        string
	Trig             uint8
	Expression       string
	Earliest         time.Time
}

func ReadDiscoveryEvidence(ctx context.Context, db *sql.DB, owner, semantics, value, expression string) (discovery.Evidence, error) {
	var evidence discovery.Evidence
	rows, err := db.QueryContext(ctx, `SELECT structure FROM discovery_routes
		WHERE session_id = ? AND semantics_version = ? AND value = ?`, owner, semantics, value)
	if err != nil {
		return evidence, err
	}
	for rows.Next() {
		var structure string
		if err := rows.Scan(&structure); err != nil {
			_ = rows.Close()
			return evidence, err
		}
		evidence.Routes = append(evidence.Routes, structure)
	}
	err = rows.Err()
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return evidence, err
	}
	err = db.QueryRowContext(ctx, `SELECT variants FROM discovery_trig
		WHERE session_id = ? AND semantics_version = ? AND value = ?`, owner, semantics, value).Scan(&evidence.Trig)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return evidence, err
	}
	if expression != "" {
		var timestamp string
		err = db.QueryRowContext(ctx, `SELECT first_success_at FROM discovery_expressions
			WHERE session_id = ? AND semantics_version = ? AND expression_identity = ?`, owner, semantics, expression).Scan(&timestamp)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return evidence, err
		}
		if err == nil {
			evidence.Earliest, err = time.Parse(time.RFC3339Nano, timestamp)
			if err != nil {
				return evidence, err
			}
		}
	}
	return evidence, nil
}

func ReadDiscoveryProgress(ctx context.Context, db *sql.DB, owner string) (DiscoveryProgress, error) {
	var progress DiscoveryProgress
	err := db.QueryRowContext(ctx, `
		SELECT last_sequence, accepted_count, state_json FROM discovery_progress WHERE session_id = ?`,
		owner).Scan(&progress.LastSequence, &progress.AcceptedCount, &progress.StateJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return DiscoveryProgress{}, nil
	}
	return progress, err
}

// CommitDiscoveryProgress atomically grants a bounded batch and advances its
// checkpoint. The compare-and-swap is the first write, so competing readers
// cannot skip a prefix or award a later source using a stale snapshot.
// All work inside the transaction uses tx, never the single-connection db.
func CommitDiscoveryProgress(ctx context.Context, db *sql.DB, owner string, expected, next DiscoveryProgress, grants []DiscoveryGrant, evidence ...DiscoveryEvidence) (map[string][]contracts.Achievement, error) {
	if next.LastSequence <= expected.LastSequence || next.AcceptedCount <= expected.AcceptedCount {
		return nil, errors.New("discovery progress must advance")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO discovery_progress (session_id, last_sequence, accepted_count)
		VALUES (?, 0, 0) ON CONFLICT (session_id) DO NOTHING`, owner); err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE discovery_progress SET last_sequence = ?, accepted_count = ?, state_json = ?
		WHERE session_id = ? AND last_sequence = ? AND accepted_count = ? AND state_json = ?`,
		next.LastSequence, next.AcceptedCount, next.StateJSON, owner, expected.LastSequence, expected.AcceptedCount, expected.StateJSON)
	if err != nil {
		return nil, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if changed == 0 {
		return nil, ErrDiscoveryProgressChanged
	}
	for _, item := range evidence {
		if item.Structure != "" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO discovery_routes (session_id, semantics_version, value, structure)
				SELECT ?, ?, ?, ? WHERE (SELECT COUNT(*) FROM discovery_routes
				WHERE session_id = ? AND semantics_version = ? AND value = ?) < 3
				ON CONFLICT DO NOTHING`, owner, item.SemanticsVersion, item.Value, item.Structure,
				owner, item.SemanticsVersion, item.Value); err != nil {
				return nil, err
			}
		}
		if item.Trig != 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO discovery_trig (session_id, semantics_version, value, variants)
				VALUES (?, ?, ?, ?) ON CONFLICT (session_id, semantics_version, value)
				DO UPDATE SET variants = discovery_trig.variants | excluded.variants`,
				owner, item.SemanticsVersion, item.Value, item.Trig); err != nil {
				return nil, err
			}
		}
		if item.Expression != "" && !item.Earliest.IsZero() {
			if _, err := tx.ExecContext(ctx, `INSERT INTO discovery_expressions
				(session_id, semantics_version, expression_identity, first_success_at)
				VALUES (?, ?, ?, ?) ON CONFLICT (session_id, semantics_version, expression_identity)
				DO UPDATE SET first_success_at = excluded.first_success_at`,
				owner, item.SemanticsVersion, item.Expression, item.Earliest.UTC().Format(time.RFC3339Nano)); err != nil {
				return nil, err
			}
		}
	}
	var awarded map[string][]contracts.Achievement
	for _, grant := range grants {
		items, err := grantAchievements(ctx, tx, owner, grant.CalculationID, grant.IDs)
		if err != nil {
			return nil, err
		}
		if len(items) != 0 {
			if awarded == nil {
				awarded = make(map[string][]contracts.Achievement)
			}
			awarded[grant.CalculationID] = append(awarded[grant.CalculationID], items...)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return awarded, nil
}
