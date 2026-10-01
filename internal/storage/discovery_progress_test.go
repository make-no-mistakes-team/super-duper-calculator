package storage_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func TestDiscoveryProgressAndGrantsRollbackTogether(t *testing.T) {
	db, err := storage.Open(t.Context(), privateDatabasePath(t, "discovery-progress.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	firstTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	saveAwardAction(t, db, "owner", "first", firstTime)
	saveAwardAction(t, db, "owner", "second", firstTime.Add(time.Minute))
	expected := storage.DiscoveryProgress{}
	next := storage.DiscoveryProgress{LastSequence: 2, AcceptedCount: 2, StateJSON: `{"successStreak":2}`}
	evidence := storage.DiscoveryEvidence{
		SemanticsVersion: "binary64-v2", Value: "4", Structure: strings.Repeat("a", 64),
		Trig: 1, Expression: strings.Repeat("b", 64), Earliest: firstTime,
	}
	grants := []storage.DiscoveryGrant{
		{CalculationID: "first", IDs: []string{"six_seven"}},
		{CalculationID: "second", IDs: []string{"bracket_architect"}},
	}
	if _, err := db.ExecContext(t.Context(), `
		CREATE TRIGGER reject_discovery_batch BEFORE INSERT ON achievements
		WHEN NEW.achievement_id = 'bracket_architect'
		BEGIN SELECT RAISE(ABORT, 'discovery batch unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	awards, err := storage.CommitDiscoveryProgress(t.Context(), db, "owner", expected, next, grants, evidence)
	if err == nil || len(awards) != 0 {
		t.Fatalf("failed batch claimed awards: %+v, %v", awards, err)
	}
	progress, err := storage.ReadDiscoveryProgress(t.Context(), db, "owner")
	if err != nil || progress != expected {
		t.Fatalf("failed grants advanced checkpoint: %+v, %v", progress, err)
	}
	collection, err := storage.ListAchievements(t.Context(), db, "owner")
	if err != nil || len(collection) != 0 {
		t.Fatalf("failed checkpoint left grants: %+v, %v", collection, err)
	}
	stored, err := storage.ReadDiscoveryEvidence(t.Context(), db, "owner", evidence.SemanticsVersion, evidence.Value, evidence.Expression)
	if err != nil || len(stored.Routes) != 0 || stored.Trig != 0 || !stored.Earliest.IsZero() {
		t.Fatalf("failed grants leaked evidence into later eligibility: %+v, %v", stored, err)
	}
	if _, err := db.ExecContext(t.Context(), "DROP TRIGGER reject_discovery_batch"); err != nil {
		t.Fatal(err)
	}
	awards, err = storage.CommitDiscoveryProgress(t.Context(), db, "owner", expected, next, grants, evidence)
	if err != nil || len(awards["first"]) != 1 || len(awards["second"]) != 1 ||
		!awards["first"][0].EarnedAt.Equal(firstTime) ||
		!awards["second"][0].EarnedAt.Equal(firstTime.Add(time.Minute)) {
		t.Fatalf("atomic retry lost eligible source times: %+v, %v", awards, err)
	}
	progress, err = storage.ReadDiscoveryProgress(t.Context(), db, "owner")
	if err != nil || progress != next {
		t.Fatalf("committed grants omitted checkpoint: %+v, %v", progress, err)
	}
	stored, err = storage.ReadDiscoveryEvidence(t.Context(), db, "owner", evidence.SemanticsVersion, evidence.Value, evidence.Expression)
	if err != nil || !slices.Contains(stored.Routes, evidence.Structure) || stored.Trig != 1 || !stored.Earliest.Equal(firstTime) {
		t.Fatalf("successful retry lost atomic eligibility evidence: %+v, %v", stored, err)
	}
}

func TestDiscoveryProgressRejectsCompetingStalePrefix(t *testing.T) {
	db, err := storage.Open(t.Context(), privateDatabasePath(t, "competing-discoveries.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	firstTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	saveAwardAction(t, db, "owner", "earliest", firstTime)
	saveAwardAction(t, db, "owner", "later", firstTime.Add(time.Minute))
	saveAwardAction(t, db, "other", "foreign", firstTime)
	zero := storage.DiscoveryProgress{}
	first := storage.DiscoveryProgress{LastSequence: 1, AcceptedCount: 1}
	_, err = storage.CommitDiscoveryProgress(t.Context(), db, "owner", zero, first,
		[]storage.DiscoveryGrant{{CalculationID: "earliest", IDs: []string{"six_seven"}}})
	if err != nil {
		t.Fatal(err)
	}
	// Another evaluator computed this later batch from the same original zero.
	// Its otherwise valid source must not award anything or skip the prefix.
	awards, err := storage.CommitDiscoveryProgress(t.Context(), db, "owner", zero,
		storage.DiscoveryProgress{LastSequence: 2, AcceptedCount: 2},
		[]storage.DiscoveryGrant{{CalculationID: "later", IDs: []string{"bracket_architect"}}},
		storage.DiscoveryEvidence{SemanticsVersion: "binary64-v2", Value: "4",
			Structure: strings.Repeat("c", 64), Trig: 2, Expression: strings.Repeat("d", 64), Earliest: firstTime})
	if !errors.Is(err, storage.ErrDiscoveryProgressChanged) || len(awards) != 0 {
		t.Fatalf("stale prefix committed: %+v, %v", awards, err)
	}
	progress, err := storage.ReadDiscoveryProgress(t.Context(), db, "owner")
	if err != nil || progress != first {
		t.Fatalf("stale prefix moved progress: %+v, %v", progress, err)
	}
	stored, err := storage.ReadDiscoveryEvidence(t.Context(), db, "owner", "binary64-v2", "4", strings.Repeat("d", 64))
	if err != nil || len(stored.Routes) != 0 || stored.Trig != 0 || !stored.Earliest.IsZero() {
		t.Fatalf("stale evaluator manufactured future eligibility: %+v, %v", stored, err)
	}
	// Atomic batches must retain direct grants' ownership validation.
	awards, err = storage.CommitDiscoveryProgress(t.Context(), db, "owner", first,
		storage.DiscoveryProgress{LastSequence: 2, AcceptedCount: 2},
		[]storage.DiscoveryGrant{{CalculationID: "foreign", IDs: []string{"bracket_architect"}}})
	if err == nil || len(awards) != 0 {
		t.Fatalf("foreign source committed: %+v, %v", awards, err)
	}
	collection, err := storage.ListAchievements(t.Context(), db, "owner")
	if err != nil || len(collection) != 1 || collection[0].ID != "six_seven" ||
		!collection[0].EarnedAt.Equal(firstTime) {
		t.Fatalf("competing prefix altered collection: %+v, %v", collection, err)
	}
	progress, err = storage.ReadDiscoveryProgress(t.Context(), db, "owner")
	if err != nil || progress != first {
		t.Fatalf("ownership failure advanced checkpoint: %+v, %v", progress, err)
	}
}
