package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/discovery"
)

var ErrUnknownAchievement = errors.New("unknown achievement")

// grantAchievements validates ownership and IDs inside the atomic discovery
// checkpoint transaction. First awards retain their original source and time.
func grantAchievements(ctx context.Context, tx *sql.Tx, owner, calculationID string, ids []string) ([]contracts.Achievement, error) {
	for _, id := range ids {
		if !discovery.Known(id) {
			return nil, ErrUnknownAchievement
		}
	}
	var createdAt string
	if err := tx.QueryRowContext(ctx,
		"SELECT created_at FROM calculations WHERE session_id = ? AND id = ?",
		owner, calculationID).Scan(&createdAt); err != nil {
		return nil, err
	}
	earnedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, errors.New("invalid calculation timestamp")
	}
	earnedAt = earnedAt.UTC()
	var granted []contracts.Achievement
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO achievements (session_id, achievement_id, calculation_id, earned_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (session_id, achievement_id) DO NOTHING`,
			owner, id, calculationID, earnedAt.Format(time.RFC3339Nano))
		if err != nil {
			return nil, err
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if inserted != 0 {
			granted = append(granted, contracts.Achievement{ID: id, EarnedAt: earnedAt})
		}
	}
	return granted, nil
}

// ListAchievements reads one owner's collection in stable ID order. Reading does
// not award anything or request announcements.
func ListAchievements(ctx context.Context, db *sql.DB, owner string) ([]contracts.Achievement, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT achievement_id, earned_at FROM achievements
		WHERE session_id = ? ORDER BY achievement_id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	collection := make([]contracts.Achievement, 0)
	for rows.Next() {
		var item contracts.Achievement
		var earnedAt string
		if err := rows.Scan(&item.ID, &earnedAt); err != nil {
			return nil, err
		}
		item.EarnedAt, err = time.Parse(time.RFC3339Nano, earnedAt)
		if err != nil {
			return nil, errors.New("invalid achievement timestamp")
		}
		collection = append(collection, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return collection, nil
}
