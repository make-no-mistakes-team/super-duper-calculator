package storage_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func saveAwardAction(t *testing.T, db *sql.DB, owner, id string, createdAt time.Time) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO sessions (id, expires_at) VALUES (?, 4102444800)
		ON CONFLICT (id) DO NOTHING`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO calculations
		(id, session_id, request_id, expression, semantics_version, outcome_json, facts_json, created_at)
		VALUES (?, ?, ?, '((((((60+7))))))', 'binary64-v1',
		'{"kind":"success","value":"67"}', NULL, ?)`,
		id, owner, id, createdAt.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
}

func TestAchievementsAwardOncePerOwnerAndRetainFirstTime(t *testing.T) {
	db, err := storage.Open(t.Context(), privateDatabasePath(t, "awards.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	firstTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	laterTime := firstTime.Add(time.Hour)
	saveAwardAction(t, db, "owner-a", "first-action", firstTime)
	saveAwardAction(t, db, "owner-a", "later-action", laterTime)
	saveAwardAction(t, db, "owner-b", "other-action", laterTime)

	type attempt struct {
		awards map[string][]contracts.Achievement
		err    error
	}
	attempts := make(chan attempt, 16)
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			awards, err := storage.CommitDiscoveryProgress(t.Context(), db, "owner-a",
				storage.DiscoveryProgress{}, storage.DiscoveryProgress{LastSequence: 1, AcceptedCount: 1},
				[]storage.DiscoveryGrant{{CalculationID: "first-action", IDs: []string{"six_seven"}}})
			attempts <- attempt{awards, err}
		})
	}
	group.Wait()
	close(attempts)
	newAwards := 0
	for result := range attempts {
		if result.err != nil && !errors.Is(result.err, storage.ErrDiscoveryProgressChanged) {
			t.Fatal(result.err)
		}
		for _, award := range result.awards["first-action"] {
			if award.ID != "six_seven" || !award.EarnedAt.Equal(firstTime) {
				t.Fatalf("unexpected new award: %+v", award)
			}
			newAwards++
		}
	}
	if newAwards != 1 {
		t.Fatalf("concurrent grants announced %d new awards, want one", newAwards)
	}
	first := storage.DiscoveryProgress{LastSequence: 1, AcceptedCount: 1}
	later := storage.DiscoveryProgress{LastSequence: 2, AcceptedCount: 2}
	if awards, err := storage.CommitDiscoveryProgress(t.Context(), db, "owner-a", first, later,
		[]storage.DiscoveryGrant{{CalculationID: "later-action", IDs: []string{"six_seven"}}}); err != nil || len(awards) != 0 {
		t.Fatalf("later eligible action re-awarded: %+v, %v", awards, err)
	}
	if awards, err := storage.CommitDiscoveryProgress(t.Context(), db, "owner-b",
		storage.DiscoveryProgress{}, storage.DiscoveryProgress{LastSequence: 3, AcceptedCount: 1},
		[]storage.DiscoveryGrant{{CalculationID: "other-action", IDs: []string{"six_seven"}}}); err != nil || len(awards["other-action"]) != 1 || !awards["other-action"][0].EarnedAt.Equal(laterTime) {
		t.Fatalf("independent owner's award: %+v, %v", awards, err)
	}
	for range 2 {
		collection, err := storage.ListAchievements(t.Context(), db, "owner-a")
		if err != nil || len(collection) != 1 || !collection[0].EarnedAt.Equal(firstTime) {
			t.Fatalf("collection changed the first earned time: %+v, %v", collection, err)
		}
	}
	if awards, err := storage.CommitDiscoveryProgress(t.Context(), db, "owner-a", later,
		storage.DiscoveryProgress{LastSequence: 3, AcceptedCount: 3},
		[]storage.DiscoveryGrant{{CalculationID: "other-action", IDs: []string{"bracket_architect"}}}); !errors.Is(err, sql.ErrNoRows) || len(awards) != 0 {
		t.Fatalf("foreign action allowed award: %+v, %v", awards, err)
	}
	if progress, err := storage.ReadDiscoveryProgress(t.Context(), db, "owner-a"); err != nil || progress != later {
		t.Fatalf("foreign action advanced checkpoint: %+v, %v", progress, err)
	}
	if _, err := db.Exec(`INSERT INTO achievements (session_id, achievement_id, calculation_id, earned_at)
		VALUES ('owner-a', 'bracket_architect', 'other-action', ?)`, laterTime.Format(time.RFC3339Nano)); err == nil {
		t.Fatal("database accepted an achievement backed by another owner's action")
	}
	otherCollection, err := storage.ListAchievements(t.Context(), db, "owner-b")
	if err != nil || len(otherCollection) != 1 || otherCollection[0].ID != "six_seven" {
		t.Fatalf("foreign attempt changed other owner's collection: %+v, %v", otherCollection, err)
	}
	unknownCollection, err := storage.ListAchievements(t.Context(), db, "unknown-owner")
	if err != nil || len(unknownCollection) != 0 {
		t.Fatalf("unknown owner saw awards: %+v, %v", unknownCollection, err)
	}
}

func TestAchievementBatchFailureReturnsNoUncommittedAwards(t *testing.T) {
	path := privateDatabasePath(t, "rollback.sqlite")
	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	earnedAt := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	saveAwardAction(t, db, "owner", "action", earnedAt)
	expected := storage.DiscoveryProgress{}
	next := storage.DiscoveryProgress{LastSequence: 1, AcceptedCount: 1}
	if awards, err := storage.CommitDiscoveryProgress(t.Context(), db, "owner", expected, next,
		[]storage.DiscoveryGrant{
			{CalculationID: "action", IDs: []string{"six_seven"}},
			{CalculationID: "action", IDs: []string{"unknown"}},
		}); !errors.Is(err, storage.ErrUnknownAchievement) || len(awards) != 0 {
		t.Fatalf("invalid batch: %+v, %v", awards, err)
	}
	if progress, err := storage.ReadDiscoveryProgress(t.Context(), db, "owner"); err != nil || progress != expected {
		t.Fatalf("unknown ID advanced checkpoint: %+v, %v", progress, err)
	}
	if collection, err := storage.ListAchievements(t.Context(), db, "owner"); err != nil || len(collection) != 0 {
		t.Fatalf("unknown ID left a partial collection: %+v, %v", collection, err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_award BEFORE INSERT ON achievements
		WHEN NEW.achievement_id = 'bracket_architect'
		BEGIN SELECT RAISE(ABORT, 'isolated award failure'); END`); err != nil {
		t.Fatal(err)
	}
	ids := []string{"six_seven", "bracket_architect"}
	grants := []storage.DiscoveryGrant{{CalculationID: "action", IDs: ids}}
	if awards, err := storage.CommitDiscoveryProgress(t.Context(), db, "owner", expected, next, grants); err == nil || len(awards) != 0 {
		t.Fatalf("failed transaction claimed awards: %+v, %v", awards, err)
	}
	collection, err := storage.ListAchievements(t.Context(), db, "owner")
	if err != nil || len(collection) != 0 {
		t.Fatalf("failed batch left a partial collection: %+v, %v", collection, err)
	}
	if progress, err := storage.ReadDiscoveryProgress(t.Context(), db, "owner"); err != nil || progress != expected {
		t.Fatalf("failed batch advanced checkpoint: %+v, %v", progress, err)
	}
	var savedOutcome string
	if err := db.QueryRow("SELECT outcome_json FROM calculations WHERE id = 'action'").Scan(&savedOutcome); err != nil || savedOutcome != `{"kind":"success","value":"67"}` {
		t.Fatalf("optional failure damaged calculation: %q, %v", savedOutcome, err)
	}
	if _, err := db.Exec("DROP TRIGGER reject_award"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if awards, err := storage.CommitDiscoveryProgress(ctx, db, "owner", expected, next, grants); !errors.Is(err, context.Canceled) || len(awards) != 0 {
		t.Fatalf("canceled transaction claimed awards: %+v, %v", awards, err)
	}
	if progress, err := storage.ReadDiscoveryProgress(t.Context(), db, "owner"); err != nil || progress != expected {
		t.Fatalf("canceled transaction advanced checkpoint: %+v, %v", progress, err)
	}
	grants[0].IDs = append(ids, "six_seven")
	awards, err := storage.CommitDiscoveryProgress(t.Context(), db, "owner", expected, next, grants)
	if err != nil || len(awards["action"]) != 2 {
		t.Fatalf("retry after repair: %+v, %v", awards, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	collection, err = storage.ListAchievements(t.Context(), reopened, "owner")
	if err != nil || len(collection) != 2 {
		t.Fatalf("reopened collection: %+v, %v", collection, err)
	}
	for _, award := range collection {
		if !award.EarnedAt.Equal(earnedAt) {
			t.Fatalf("restart changed earned time: %+v", award)
		}
	}
	if progress, err := storage.ReadDiscoveryProgress(t.Context(), reopened, "owner"); err != nil || progress != next {
		t.Fatalf("restart lost committed checkpoint: %+v, %v", progress, err)
	}
	if awards, err := storage.CommitDiscoveryProgress(t.Context(), reopened, "owner", expected, next, grants); !errors.Is(err, storage.ErrDiscoveryProgressChanged) || len(awards) != 0 {
		t.Fatalf("restart accepted stale prefix: %+v, %v", awards, err)
	}
	saveAwardAction(t, reopened, "owner", "later-action", earnedAt.Add(time.Hour))
	if awards, err := storage.CommitDiscoveryProgress(t.Context(), reopened, "owner", next,
		storage.DiscoveryProgress{LastSequence: 2, AcceptedCount: 2},
		[]storage.DiscoveryGrant{{CalculationID: "later-action", IDs: ids}}); err != nil || len(awards) != 0 {
		t.Fatalf("restart allowed duplicate awards: %+v, %v", awards, err)
	}
	collection, err = storage.ListAchievements(t.Context(), reopened, "owner")
	if err != nil || len(collection) != 2 {
		t.Fatalf("later action changed reopened collection: %+v, %v", collection, err)
	}
	for _, award := range collection {
		if !award.EarnedAt.Equal(earnedAt) {
			t.Fatalf("later action replaced original earned time: %+v", award)
		}
	}
}

func TestAchievementMigrationUpgradesCoreHistory(t *testing.T) {
	path := privateDatabasePath(t, "upgrade.sqlite")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	coreSchema, err := os.ReadFile("migrations/001_core.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(string(coreSchema) + "; PRAGMA user_version = 1"); err != nil {
		t.Fatal(err)
	}
	earnedAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	// Historical fixtures retain their original required columns independently
	// of helpers that insert into the current schema.
	if _, err := old.ExecContext(t.Context(), `
		INSERT INTO sessions (id, expires_at) VALUES ('legacy-owner', 4102444800);
		INSERT INTO calculations
			(id, session_id, request_id, expression, angle_unit, semantics_version, outcome_json, facts_json, created_at)
		VALUES ('legacy-action', 'legacy-owner', 'legacy-action', '((((((60+7))))))',
			'deg', 'binary64-v1', '{"kind":"success","value":"67"}', NULL, ?)`,
		earnedAt.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	awards, err := storage.CommitDiscoveryProgress(t.Context(), upgraded, "legacy-owner",
		storage.DiscoveryProgress{}, storage.DiscoveryProgress{LastSequence: 1, AcceptedCount: 1},
		[]storage.DiscoveryGrant{{CalculationID: "legacy-action", IDs: []string{"six_seven"}}})
	if err != nil || len(awards["legacy-action"]) != 1 || !awards["legacy-action"][0].EarnedAt.Equal(earnedAt) {
		t.Fatalf("upgrade lost the original action context: %+v, %v", awards, err)
	}
	var expression string
	if err := upgraded.QueryRow("SELECT expression FROM calculations WHERE id = 'legacy-action'").Scan(&expression); err != nil || expression != "((((((60+7))))))" {
		t.Fatalf("upgrade changed history: %q, %v", expression, err)
	}
}
