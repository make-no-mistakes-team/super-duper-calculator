package main

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func incidentAt(service *incidentService, at time.Time) {
	service.now = func() time.Time { return at }
}

func expectIncident(t *testing.T, events []contracts.FunEvent, record contracts.CalculationRecord) {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("events = %+v, want one incident", events)
	}
	event := events[0]
	if event.ID != record.ID+":comic_incident" ||
		event.RuleID != "comic_incident" || event.Kind != "scene" ||
		event.Scope != "personal" || len(event.Params) != 0 || event.Params == nil ||
		!event.CreatedAt.Equal(record.CreatedAt) ||
		!event.ExpiresAt.Equal(record.CreatedAt.Add(incidentTTL)) {
		t.Fatalf("unexpected incident: %+v for %+v", event, record)
	}
}

func TestIncidentThirdDistinctStoredErrorAndSequenceBound(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := newIncidentService(db, true)
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	first := saveDiscoveryAction(t, db, "owner", "first", "1/0", start)
	second := saveDiscoveryAction(t, db, "owner", "second", "0^-1", start.Add(time.Second))
	incidentAt(service, start.Add(2*time.Second))
	for _, record := range []contracts.CalculationRecord{first, second} {
		events, err := service.Process(t.Context(), "owner", record)
		if err != nil || len(events) != 0 {
			t.Fatalf("premature incident for %s: %+v, %v", record.ID, events, err)
		}
	}
	// Caller data is untrusted; a forged mathematical error must not count.
	ordinary := saveDiscoveryAction(t, db, "owner", "ordinary", "1+1", start.Add(1500*time.Millisecond))
	forged := ordinary
	forged.Outcome = contracts.Outcome{Kind: "error", Error: &contracts.MathError{Code: "DIVISION_BY_ZERO"}}
	if events, err := service.Process(t.Context(), "owner", forged); err != nil || len(events) != 0 {
		t.Fatalf("forged error caused incident: %+v, %v", events, err)
	}
	third := saveDiscoveryAction(t, db, "owner", "third", "1/0", start.Add(2*time.Second))
	// A later committed action cannot supply the third error for third's
	// historical snapshot; the actual third action remains eligible itself.
	fourth := saveDiscoveryAction(t, db, "owner", "fourth", "1/0", start.Add(3*time.Second))
	third.Outcome = contracts.Outcome{Kind: "success", Value: "2"}
	events, err := service.Process(t.Context(), "owner", third)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, third)
	incidentAt(service, fourth.CreatedAt)
	if events, err := service.Process(t.Context(), "owner", fourth); err != nil || len(events) != 0 {
		t.Fatalf("cooldown ignored: %+v, %v", events, err)
	}
	if events, err := service.Process(t.Context(), "owner", third); err != nil || len(events) != 0 {
		t.Fatalf("retry re-announced incident: %+v, %v", events, err)
	}
}

func TestIncidentExcludesLaterActionsFromCurrentHistory(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := newIncidentService(db, true)
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	saveDiscoveryAction(t, db, "owner", "success", "1+1", start)
	second := saveDiscoveryAction(t, db, "owner", "second", "1/0", start.Add(time.Second))
	third := saveDiscoveryAction(t, db, "owner", "third", "1/0", start.Add(2*time.Second))
	fourth := saveDiscoveryAction(t, db, "owner", "fourth", "1/0", start.Add(3*time.Second))
	incidentAt(service, fourth.CreatedAt)
	if events, err := service.Process(t.Context(), "owner", third); err != nil || len(events) != 0 {
		t.Fatalf("future action counted for third: %+v, %v", events, err)
	}
	if events, err := service.Process(t.Context(), "owner", second); err != nil || len(events) != 0 {
		t.Fatalf("later action counted for second: %+v, %v", events, err)
	}
	events, err := service.Process(t.Context(), "owner", fourth)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, fourth)
}

func TestIncidentSixtySecondBoundaryAndFractionalPrecision(t *testing.T) {
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		first time.Duration
		last  time.Duration
		want  bool
	}{
		{"different fractional precision", 120 * time.Millisecond, 60*time.Second + 100*time.Millisecond, true},
		{"exactly sixty seconds", 100 * time.Millisecond, 60*time.Second + 100*time.Millisecond, true},
		{"one nanosecond too old", 100 * time.Millisecond, 60*time.Second + 100*time.Millisecond + time.Nanosecond, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := boundaryDatabase(t)
			service := newIncidentService(db, true)
			saveDiscoveryAction(t, db, "owner", "first", "1/0", start.Add(tc.first))
			saveDiscoveryAction(t, db, "owner", "second", "1/0", start.Add(time.Second))
			third := saveDiscoveryAction(t, db, "owner", "third", "1/0", start.Add(tc.last))
			incidentAt(service, third.CreatedAt.Add(time.Millisecond))
			events, err := service.Process(t.Context(), "owner", third)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want {
				expectIncident(t, events, third)
			} else if len(events) != 0 {
				t.Fatalf("out-of-window action counted: %+v", events)
			}
		})
	}
}

func TestIncidentTimestampInversionAndExactSpan(t *testing.T) {
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		prior []time.Duration
		want  bool
	}{
		{"descending timestamps", []time.Duration{2 * time.Second, time.Second}, true},
		{"mixed span exceeds sixty seconds", []time.Duration{-59 * time.Second, 2 * time.Second}, false},
		{"mixed exact sixty seconds", []time.Duration{-58 * time.Second, 2 * time.Second}, true},
		{"mixed one nanosecond over sixty seconds", []time.Duration{-58*time.Second - time.Nanosecond, 2 * time.Second}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := boundaryDatabase(t)
			service := newIncidentService(db, true)
			for i, offset := range tc.prior {
				saveDiscoveryAction(t, db, "owner", string(rune('a'+i)), "1/0", start.Add(offset))
			}
			current := saveDiscoveryAction(t, db, "owner", "current", "1/0", start)
			incidentAt(service, start.Add(3*time.Second))
			events, err := service.Process(t.Context(), "owner", current)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want {
				expectIncident(t, events, current)
				if events, err := service.Process(t.Context(), "owner", current); err != nil || len(events) != 0 {
					t.Fatalf("inverted action replayed incident: %+v, %v", events, err)
				}
			} else if len(events) != 0 {
				t.Fatalf("actions spanning more than sixty seconds announced incident: %+v", events)
			}
		})
	}
}

func TestIncidentInvertedTimestampsExcludeLaterActions(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := newIncidentService(db, true)
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	saveDiscoveryAction(t, db, "owner", "first", "1/0", start.Add(2*time.Second))
	current := saveDiscoveryAction(t, db, "owner", "current", "1/0", start)
	later := saveDiscoveryAction(t, db, "owner", "later", "1/0", start.Add(time.Second))
	incidentAt(service, start.Add(3*time.Second))
	if events, err := service.Process(t.Context(), "owner", current); err != nil || len(events) != 0 {
		t.Fatalf("later sequence supplied inverted timestamp incident: %+v, %v", events, err)
	}
	events, err := service.Process(t.Context(), "owner", later)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, later)
}

func TestIncidentConcurrentClaimsRetryAndCooldownBoundary(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := newIncidentService(db, true)
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	saveDiscoveryAction(t, db, "owner", "first", "1/0", start)
	saveDiscoveryAction(t, db, "owner", "second", "1/0", start.Add(time.Second))
	current := saveDiscoveryAction(t, db, "owner", "current", "1/0", start.Add(2*time.Second))
	incidentAt(service, current.CreatedAt.Add(time.Millisecond))
	type result struct {
		events []contracts.FunEvent
		err    error
	}
	claims := make(chan result, 24)
	var group sync.WaitGroup
	for range 24 {
		group.Go(func() {
			events, err := service.Process(t.Context(), "owner", current)
			claims <- result{events, err}
		})
	}
	group.Wait()
	close(claims)
	winners := 0
	for claim := range claims {
		if claim.err != nil {
			t.Fatal(claim.err)
		}
		if len(claim.events) != 0 {
			expectIncident(t, claim.events, current)
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent calls announced %d incidents, want one", winners)
	}
	if events, err := service.Process(t.Context(), "owner", current); err != nil || len(events) != 0 {
		t.Fatalf("retry after claim: %+v, %v", events, err)
	}

	saveDiscoveryAction(t, db, "owner", "after-one", "1/0", start.Add(120*time.Second))
	saveDiscoveryAction(t, db, "owner", "after-two", "1/0", start.Add(121*time.Second))
	tooEarly := saveDiscoveryAction(t, db, "owner", "too-early", "1/0",
		current.CreatedAt.Add(incidentCooldown-time.Nanosecond))
	incidentAt(service, tooEarly.CreatedAt.Add(time.Millisecond))
	if events, err := service.Process(t.Context(), "owner", tooEarly); err != nil || len(events) != 0 {
		t.Fatalf("cooldown allowed early incident: %+v, %v", events, err)
	}
	exact := saveDiscoveryAction(t, db, "owner", "exact", "1/0", current.CreatedAt.Add(incidentCooldown))
	incidentAt(service, exact.CreatedAt)
	events, err := service.Process(t.Context(), "owner", exact)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, exact)
}

func TestIncidentConcurrentDifferentActionsAndOutOfOrderCompletion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reverse bool
	}{{"concurrent", false}, {"later-first", true}} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := boundaryDatabase(t)
			service := newIncidentService(db, true)
			start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
			saveDiscoveryAction(t, db, "owner", "first", "1/0", start)
			saveDiscoveryAction(t, db, "owner", "second", "1/0", start.Add(time.Second))
			third := saveDiscoveryAction(t, db, "owner", "third", "1/0", start.Add(2*time.Second))
			fourth := saveDiscoveryAction(t, db, "owner", "fourth", "1/0", start.Add(2*time.Second+time.Nanosecond))
			incidentAt(service, fourth.CreatedAt)
			if tc.reverse {
				events, err := service.Process(t.Context(), "owner", fourth)
				if err != nil {
					t.Fatal(err)
				}
				expectIncident(t, events, fourth)
				if events, err := service.Process(t.Context(), "owner", third); err != nil || len(events) != 0 {
					t.Fatalf("older completion displaced newer claim: %+v, %v", events, err)
				}
				return
			}
			claims := make(chan []contracts.FunEvent, 2)
			failures := make(chan error, 2)
			var group sync.WaitGroup
			for _, record := range []contracts.CalculationRecord{third, fourth} {
				group.Go(func() {
					events, err := service.Process(t.Context(), "owner", record)
					claims <- events
					failures <- err
				})
			}
			group.Wait()
			close(claims)
			close(failures)
			for err := range failures {
				if err != nil {
					t.Fatal(err)
				}
			}
			winners := 0
			for events := range claims {
				if len(events) != 0 {
					if events[0].ID != third.ID+":comic_incident" && events[0].ID != fourth.ID+":comic_incident" {
						t.Fatalf("claim belonged to an unknown action: %+v", events)
					}
					winners++
				}
			}
			if winners != 1 {
				t.Fatalf("distinct concurrent actions produced %d scenes, want one", winners)
			}
		})
	}
}

func TestIncidentOwnerIsolationAndUnownedRecord(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := newIncidentService(db, true)
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	for _, owner := range []string{"one", "two"} {
		saveDiscoveryAction(t, db, owner, owner+"-first", "1/0", start)
		saveDiscoveryAction(t, db, owner, owner+"-second", "1/0", start.Add(time.Second))
	}
	foreign := saveDiscoveryAction(t, db, "one", "one-third", "1/0", start.Add(2*time.Second))
	incidentAt(service, foreign.CreatedAt)
	if events, err := service.Process(t.Context(), "two", foreign); !errors.Is(err, sql.ErrNoRows) || len(events) != 0 {
		t.Fatalf("foreign action accepted: %+v, %v", events, err)
	}
	one, err := service.Process(t.Context(), "one", foreign)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, one, foreign)
	other := saveDiscoveryAction(t, db, "two", "two-third", "1/0", foreign.CreatedAt)
	two, err := service.Process(t.Context(), "two", other)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, two, other)
}

func TestIncidentRestartPreservesCooldown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "calculator.sqlite")
	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	saveDiscoveryAction(t, db, "owner", "first", "1/0", start)
	saveDiscoveryAction(t, db, "owner", "second", "1/0", start.Add(time.Second))
	third := saveDiscoveryAction(t, db, "owner", "third", "1/0", start.Add(2*time.Second))
	service := newIncidentService(db, true)
	incidentAt(service, third.CreatedAt)
	events, err := service.Process(t.Context(), "owner", third)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, third)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	restarted := newIncidentService(reopened, true)
	saveDiscoveryAction(t, reopened, "owner", "after-one", "1/0", start.Add(120*time.Second))
	saveDiscoveryAction(t, reopened, "owner", "after-two", "1/0", start.Add(121*time.Second))
	early := saveDiscoveryAction(t, reopened, "owner", "early", "1/0", third.CreatedAt.Add(incidentCooldown-time.Nanosecond))
	incidentAt(restarted, early.CreatedAt)
	if events, err := restarted.Process(t.Context(), "owner", early); err != nil || len(events) != 0 {
		t.Fatalf("restart forgot cooldown: %+v, %v", events, err)
	}
	exact := saveDiscoveryAction(t, reopened, "owner", "exact", "1/0", third.CreatedAt.Add(incidentCooldown))
	incidentAt(restarted, exact.CreatedAt)
	events, err = restarted.Process(t.Context(), "owner", exact)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, exact)
}

func TestIncidentExpiredProcessingDoesNotClaimCooldown(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := newIncidentService(db, true)
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	saveDiscoveryAction(t, db, "owner", "first", "1/0", start)
	saveDiscoveryAction(t, db, "owner", "second", "1/0", start.Add(time.Second))
	third := saveDiscoveryAction(t, db, "owner", "third", "1/0", start.Add(2*time.Second))
	incidentAt(service, third.CreatedAt.Add(incidentTTL))
	if events, err := service.Process(t.Context(), "owner", third); err != nil || len(events) != 0 {
		t.Fatalf("expired action announced: %+v, %v", events, err)
	}
	fourth := saveDiscoveryAction(t, db, "owner", "fourth", "1/0", start.Add(3*time.Second))
	incidentAt(service, fourth.CreatedAt)
	events, err := service.Process(t.Context(), "owner", fourth)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, fourth)
}

func TestIncidentExpirationDuringClaimRollsBack(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := newIncidentService(db, true)
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	saveDiscoveryAction(t, db, "owner", "first", "1/0", start)
	saveDiscoveryAction(t, db, "owner", "second", "1/0", start.Add(time.Second))
	third := saveDiscoveryAction(t, db, "owner", "third", "1/0", start.Add(2*time.Second))
	checks := 0
	service.now = func() time.Time {
		checks++
		if checks >= 4 {
			return third.CreatedAt.Add(incidentTTL)
		}
		return third.CreatedAt
	}
	if events, err := service.Process(t.Context(), "owner", third); err != nil || len(events) != 0 {
		t.Fatalf("expired in-flight claim announced: %+v, %v", events, err)
	}
	var cooldowns int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM personal_effects WHERE session_id = 'owner'").Scan(&cooldowns); err != nil || cooldowns != 0 {
		t.Fatalf("expired claim consumed cooldown: %d, %v", cooldowns, err)
	}
	fourth := saveDiscoveryAction(t, db, "owner", "fourth", "1/0", start.Add(3*time.Second))
	incidentAt(service, fourth.CreatedAt)
	events, err := service.Process(t.Context(), "owner", fourth)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, fourth)
}

func TestIncidentSQLFailurePreservesCommittedAction(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := newIncidentService(db, true)
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	saveDiscoveryAction(t, db, "owner", "first", "1/0", start)
	saveDiscoveryAction(t, db, "owner", "second", "1/0", start.Add(time.Second))
	third := saveDiscoveryAction(t, db, "owner", "third", "1/0", start.Add(2*time.Second))
	incidentAt(service, third.CreatedAt)
	if _, err := db.ExecContext(t.Context(), `
		CREATE TRIGGER reject_incident BEFORE INSERT ON personal_effects
		BEGIN SELECT RAISE(ABORT, 'optional incident unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if events, err := service.Process(t.Context(), "owner", third); err == nil || len(events) != 0 {
		t.Fatalf("failed optional write announced event: %+v, %v", events, err)
	}
	var saved, cooldowns int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM calculations WHERE id = ?", third.ID).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM personal_effects WHERE session_id = 'owner'").Scan(&cooldowns); err != nil {
		t.Fatal(err)
	}
	if saved != 1 || cooldowns != 0 {
		t.Fatalf("optional failure changed core action or cooldown: saved=%d cooldowns=%d", saved, cooldowns)
	}
	if _, err := db.ExecContext(t.Context(), "DROP TRIGGER reject_incident"); err != nil {
		t.Fatal(err)
	}
	next := saveDiscoveryAction(t, db, "owner", "next", "1/0", start.Add(3*time.Second))
	incidentAt(service, next.CreatedAt)
	events, err := service.Process(t.Context(), "owner", next)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, next)
}

func TestIncidentMigrationFromPopulatedVersionTwoPreservesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "legacy.sqlite")
	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	prior := saveDiscoveryAction(t, db, "owner", "award", "60+7", start)
	saveDiscoveryAction(t, db, "owner", "first", "1/0", start.Add(time.Second))
	saveDiscoveryAction(t, db, "owner", "second", "1/0", start.Add(2*time.Second))
	if _, err := db.ExecContext(t.Context(), `
		INSERT INTO achievements (session_id, achievement_id, calculation_id, earned_at)
		VALUES ('owner', 'six_seven', ?, ?)`,
		prior.ID, prior.CreatedAt.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	// Reconstruct a populated v2 file by removing only later schema objects.
	if _, err := db.ExecContext(t.Context(), `
		DROP INDEX calculations_owner_effect_window;
		DROP TABLE personal_effects;
		DROP TABLE discovery_progress;
		PRAGMA user_version = 2`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	migrated, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = migrated.Close() })
	var version, actions, awards int
	for _, query := range []struct {
		statement string
		target    *int
	}{
		{"PRAGMA user_version", &version},
		{"SELECT COUNT(*) FROM calculations WHERE session_id = 'owner'", &actions},
		{"SELECT COUNT(*) FROM achievements WHERE session_id = 'owner' AND achievement_id = 'six_seven' AND calculation_id = 'award'", &awards},
	} {
		if err := migrated.QueryRowContext(t.Context(), query.statement).Scan(query.target); err != nil {
			t.Fatal(err)
		}
	}
	if version != 4 || actions != 3 || awards != 1 {
		t.Fatalf("v2 migration lost history: version=%d actions=%d awards=%d", version, actions, awards)
	}
	third := saveDiscoveryAction(t, migrated, "owner", "third", "1/0", start.Add(3*time.Second))
	service := newIncidentService(migrated, true)
	incidentAt(service, third.CreatedAt)
	events, err := service.Process(t.Context(), "owner", third)
	if err != nil {
		t.Fatal(err)
	}
	expectIncident(t, events, third)
}

func TestDisabledIncidentServiceAndCanceledContext(t *testing.T) {
	if newIncidentService(nil, false) != nil {
		t.Fatal("disabled incident service initialized")
	}
	var disabled *incidentService
	if events, err := disabled.Process(t.Context(), "owner", contracts.CalculationRecord{}); err != nil || len(events) != 0 {
		t.Fatalf("disabled incident = %+v, %v", events, err)
	}
	db, _ := boundaryDatabase(t)
	service := newIncidentService(db, true)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if events, err := service.Process(ctx, "owner", contracts.CalculationRecord{ID: "action"}); !errors.Is(err, context.Canceled) || len(events) != 0 {
		t.Fatalf("canceled incident = %+v, %v", events, err)
	}
}
