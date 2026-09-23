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

// GrantAchievements persists eligible IDs for an already committed, owned action.
// It returns only new awards after commit. The first awarding action supplies
// earnedAt; later actions and retries cannot replace it. Eligibility is decided
// by discovery rules, not by this storage operation.
func GrantAchievements(ctx context.Context, db *sql.DB, owner, calculationID string, ids []string) ([]contracts.Achievement, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	for _, id := range ids {
		if !discovery.Known(id) {
			return nil, ErrUnknownAchievement
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
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
	if err := tx.Commit(); err != nil {
		return nil, err
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
