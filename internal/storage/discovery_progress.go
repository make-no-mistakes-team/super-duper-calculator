package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

// ErrDiscoveryProgressChanged means another reconciliation committed the prefix.
// The caller must reload progress and re-evaluate its uncommitted batch.
var ErrDiscoveryProgressChanged = errors.New("discovery progress changed")

// DiscoveryProgress describes a completely evaluated owner history prefix.
// AcceptedCount is the owner's ordinal, not the database-wide sequence.
type DiscoveryProgress struct {
	LastSequence  int64
	AcceptedCount int64
}

type DiscoveryGrant struct {
	CalculationID string
	IDs           []string
}

func ReadDiscoveryProgress(ctx context.Context, db *sql.DB, owner string) (DiscoveryProgress, error) {
	var progress DiscoveryProgress
	err := db.QueryRowContext(ctx, `
		SELECT last_sequence, accepted_count FROM discovery_progress WHERE session_id = ?`,
		owner).Scan(&progress.LastSequence, &progress.AcceptedCount)
	if errors.Is(err, sql.ErrNoRows) {
		return DiscoveryProgress{}, nil
	}
	return progress, err
}

// CommitDiscoveryProgress atomically grants a bounded batch and advances its
// checkpoint. The compare-and-swap is the first write, so competing readers
// cannot skip a prefix or award a later source using a stale snapshot.
// All work inside the transaction uses tx, never the single-connection db.
func CommitDiscoveryProgress(ctx context.Context, db *sql.DB, owner string, expected, next DiscoveryProgress, grants []DiscoveryGrant) (map[string][]contracts.Achievement, error) {
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
		UPDATE discovery_progress SET last_sequence = ?, accepted_count = ?
		WHERE session_id = ? AND last_sequence = ? AND accepted_count = ?`,
		next.LastSequence, next.AcceptedCount, owner, expected.LastSequence, expected.AcceptedCount)
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
