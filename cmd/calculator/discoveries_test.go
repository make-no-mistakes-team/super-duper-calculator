package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func saveDiscoveryAction(t *testing.T, db *sql.DB, owner, id, expression string, when time.Time) contracts.CalculationRecord {
	t.Helper()
	if _, err := db.ExecContext(t.Context(),
		"INSERT INTO sessions (id, expires_at) VALUES (?, 4102444800) ON CONFLICT (id) DO NOTHING",
		owner); err != nil {
		t.Fatal(err)
	}
	evaluation, err := calculation.New().Evaluate(t.Context(), calculation.Input{
		Expression: expression, AngleUnit: contracts.Degrees,
	})
	if err != nil {
		t.Fatal(err)
	}
	record := contracts.CalculationRecord{
		ID: id, RequestID: id, Expression: expression,
		Context: contracts.CalculationContext{AngleUnit: contracts.Degrees, SemanticsVersion: calculation.SemanticsVersion},
		Outcome: evaluation.Outcome, Facts: evaluation.Facts, CreatedAt: when.UTC(),
	}
	outcomeJSON, err := json.Marshal(record.Outcome)
	if err != nil {
		t.Fatal(err)
	}
	factsJSON, err := json.Marshal(record.Facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `
		INSERT INTO calculations
			(id, session_id, request_id, expression, angle_unit, semantics_version,
				outcome_json, facts_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, owner, id, expression, record.Context.AngleUnit, record.Context.SemanticsVersion,
		string(outcomeJSON), string(factsJSON), when.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	return record
}

func findDiscoveryAward(t *testing.T, awards []contracts.Achievement, id string) contracts.Achievement {
	t.Helper()
	for _, award := range awards {
		if award.ID == id {
			return award
		}
	}
	t.Fatalf("missing %s in awards %+v", id, awards)
	return contracts.Achievement{}
}

func fixedDiscoveryService(db *sql.DB) *discoveryService {
	service := newDiscoveryService(db, true)
	service.now = func() time.Time {
		return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	}
	return service
}

func TestDiscoveryServiceEightRealRulesAndComments(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := fixedDiscoveryService(db)
	start := service.now().Add(-2 * time.Second)
	expressions := []struct {
		expression string
		newAward   string
	}{
		{"6*7", "answer_found"},
		{"60+7", "six_seven"},
		{"60+9", "nice_number"},
		{"400+4", "result_found"},
		{"((((((2))))))", "bracket_architect"},
		{"sqrt(4)+sin(0)+cos(0)", "scientific_method"},
		{"1+1", ""},
		{"1+1", ""},
		{"1+1", "peer_review"},
	}
	for len(expressions) < 25 {
		expressions = append(expressions, struct{ expression, newAward string }{"1/0", ""})
	}
	expressions[24].newAward = "touch_grass"
	for i, action := range expressions {
		record := saveDiscoveryAction(t, db, "owner", fmt.Sprintf("action-%03d", i),
			action.expression, start.Add(time.Duration(i)*time.Millisecond))
		awards, events, err := service.Process(t.Context(), "owner", record)
		if err != nil {
			t.Fatalf("process action %d: %v", i, err)
		}
		if action.newAward == "" {
			if len(awards) != 0 || len(events) != 0 {
				t.Fatalf("unmatched action %d got awards %+v and events %+v", i, awards, events)
			}
			continue
		}
		if len(awards) != 1 || awards[0].ID != action.newAward || !awards[0].EarnedAt.Equal(record.CreatedAt) {
			t.Fatalf("action %d got awards %+v; want %s at %v", i, awards, action.newAward, record.CreatedAt)
		}
		if len(events) != 1 || events[0].ID != record.ID+":"+action.newAward ||
			events[0].RuleID != action.newAward || events[0].Kind != "comment" ||
			events[0].Scope != "personal" || events[0].Params == nil ||
			!events[0].CreatedAt.Equal(record.CreatedAt) ||
			!events[0].ExpiresAt.Equal(record.CreatedAt.Add(discoveryCommentTTL)) {
			t.Fatalf("action %d got unexpected comment %+v", i, events)
		}
	}
	collection, catalog, err := service.Collection(t.Context(), "owner")
	if err != nil || len(collection) != 8 || len(catalog) != 8 {
		t.Fatalf("catalog/collection = %+v / %+v, %v", catalog, collection, err)
	}
	for _, definition := range catalog {
		if definition.ID == "" || definition.RU.Name == "" || definition.RU.Description == "" ||
			definition.RU.Comment == "" || definition.EN.Name == "" ||
			definition.EN.Description == "" || definition.EN.Comment == "" {
			t.Fatalf("incomplete discovery catalog entry: %+v", definition)
		}
		findDiscoveryAward(t, collection, definition.ID)
	}
	catalog[0].RU.Name = "external mutation"
	_, freshCatalog, err := service.Collection(t.Context(), "owner")
	if err != nil || freshCatalog[0].RU.Name == "external mutation" {
		t.Fatalf("catalog mutated across collections: %+v, %v", freshCatalog, err)
	}
}

func TestDiscoveryProcessLaterActionCannotTakeFirstEligibility(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := fixedDiscoveryService(db)
	start := service.now().Add(-time.Second)
	first := saveDiscoveryAction(t, db, "owner", "first", "60+7", start)
	later := saveDiscoveryAction(t, db, "owner", "later", "60+7", start.Add(time.Millisecond))

	// Later request completion wins the race, but must reconcile the earlier
	// committed action before it can award its own matching rule.
	awards, events, err := service.Process(t.Context(), "owner", later)
	if err != nil || len(awards) != 0 || len(events) != 0 {
		t.Fatalf("later response announced prior eligibility: %+v, %+v, %v", awards, events, err)
	}
	awards, events, err = service.Process(t.Context(), "owner", first)
	if err != nil || len(awards) != 0 || len(events) != 0 {
		t.Fatalf("delayed first response re-announced reconciliation: %+v, %+v, %v", awards, events, err)
	}
	collection, _, err := service.Collection(t.Context(), "owner")
	if err != nil || len(collection) != 1 || collection[0].ID != "six_seven" ||
		!collection[0].EarnedAt.Equal(first.CreatedAt) {
		t.Fatalf("earliest eligible timestamp lost: %+v, %v", collection, err)
	}

	concurrent := saveDiscoveryAction(t, db, "other", "other-first", "60+7", start)
	type result struct {
		awards []contracts.Achievement
		events []contracts.FunEvent
		err    error
	}
	responses := make(chan result, 12)
	var workers sync.WaitGroup
	for range 12 {
		workers.Go(func() {
			newAwards, comments, err := service.Process(t.Context(), "other", concurrent)
			responses <- result{newAwards, comments, err}
		})
	}
	workers.Wait()
	close(responses)
	committed := 0
	for response := range responses {
		if response.err != nil {
			t.Fatal(response.err)
		}
		committed += len(response.awards)
		if len(response.events) != len(response.awards) {
			t.Fatalf("comment escaped a one-time award: %+v", response)
		}
	}
	if committed != 1 {
		t.Fatalf("race announced %d awards, want one", committed)
	}
}

func TestDiscoveryCollectionQuietlyCatchesUpAcrossBatchesAndBadFacts(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := fixedDiscoveryService(db)
	start := service.now().Add(-2 * time.Second)
	for i := range 270 {
		expression := "1+1"
		if i == 269 {
			expression = "60+7"
		}
		saveDiscoveryAction(t, db, "owner", fmt.Sprintf("long-%03d", i),
			expression, start.Add(time.Duration(i)*time.Millisecond))
	}
	if _, err := db.ExecContext(t.Context(),
		"UPDATE calculations SET facts_json = ? WHERE id = 'long-000'", `{"depth":"invalid"}`); err != nil {
		t.Fatal(err)
	}
	firstCollection, catalog, err := service.Collection(t.Context(), "owner")
	if err != nil || len(catalog) != 8 || len(firstCollection) != 3 {
		t.Fatalf("large history reconciliation: %+v, %+v, %v", firstCollection, catalog, err)
	}
	for id, offset := range map[string]int{
		"peer_review": 2, "touch_grass": 24, "six_seven": 269,
	} {
		award := findDiscoveryAward(t, firstCollection, id)
		if !award.EarnedAt.Equal(start.Add(time.Duration(offset) * time.Millisecond)) {
			t.Fatalf("%s earned at %v, want offset %d", id, award.EarnedAt, offset)
		}
	}
	var calculationCount int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM calculations").Scan(&calculationCount); err != nil || calculationCount != 270 {
		t.Fatalf("collection changed saved calculations: %d, %v", calculationCount, err)
	}
	secondCollection, _, err := service.Collection(t.Context(), "owner")
	if err != nil || !reflect.DeepEqual(firstCollection, secondCollection) {
		t.Fatalf("history read re-awarded discovery: %+v versus %+v, %v", firstCollection, secondCollection, err)
	}
	next := saveDiscoveryAction(t, db, "owner", "new-answer", "6*7", start.Add(time.Second))
	awards, events, err := service.Process(t.Context(), "owner", next)
	if err != nil || len(awards) != 1 || awards[0].ID != "answer_found" ||
		len(events) != 1 || events[0].RuleID != "answer_found" {
		t.Fatalf("current action replayed catch-up notifications: %+v, %+v, %v", awards, events, err)
	}
}

func TestDiscoveryOwnershipFailureAndCanceledContext(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := fixedDiscoveryService(db)
	foreign := saveDiscoveryAction(t, db, "other", "other-action", "60+7", service.now())
	if awards, events, err := service.Process(t.Context(), "owner", foreign); !errors.Is(err, sql.ErrNoRows) || len(awards) != 0 || len(events) != 0 {
		t.Fatalf("foreign record awarded owner: %+v, %+v, %v", awards, events, err)
	}
	// Even the caller's record contents cannot manufacture eligibility.
	plain := saveDiscoveryAction(t, db, "owner", "plain", "1+1", service.now())
	forged := plain
	forged.Outcome = contracts.Outcome{Kind: contracts.OutcomeSuccess, Value: "67"}
	if awards, events, err := service.Process(t.Context(), "owner", forged); err != nil || len(awards) != 0 || len(events) != 0 {
		t.Fatalf("caller supplied facts manufactured an award: %+v, %+v, %v", awards, events, err)
	}
	ownerAwards, _, err := service.Collection(t.Context(), "owner")
	if err != nil || len(ownerAwards) != 0 {
		t.Fatalf("owner gained a foreign discovery: %+v, %v", ownerAwards, err)
	}
	otherAwards, _, err := service.Collection(t.Context(), "other")
	if err != nil || len(otherAwards) != 1 || otherAwards[0].ID != "six_seven" {
		t.Fatalf("foreign owner did not retain their discovery: %+v, %v", otherAwards, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if awards, events, err := service.Process(ctx, "other", foreign); !errors.Is(err, context.Canceled) || len(awards) != 0 || len(events) != 0 {
		t.Fatalf("canceled process = %+v, %+v, %v", awards, events, err)
	}
	if awards, catalog, err := service.Collection(ctx, "other"); !errors.Is(err, context.Canceled) || len(awards) != 0 || len(catalog) != 0 {
		t.Fatalf("canceled collection = %+v, %+v, %v", awards, catalog, err)
	}
}

func TestDiscoveryOptionalFailurePreservesCoreAndRecoversQuietly(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := fixedDiscoveryService(db)
	first := saveDiscoveryAction(t, db, "owner", "failing-action", "((((((60+7))))))", service.now())
	if _, err := db.ExecContext(t.Context(), `
		CREATE TRIGGER reject_optional_award BEFORE INSERT ON achievements
		WHEN NEW.achievement_id = 'bracket_architect'
		BEGIN SELECT RAISE(ABORT, 'optional discovery unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if awards, events, err := service.Process(t.Context(), "owner", first); err == nil || len(awards) != 0 || len(events) != 0 {
		t.Fatalf("failed optional write claimed rewards: %+v, %+v, %v", awards, events, err)
	}
	var count int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM calculations WHERE id = ?", first.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("optional failure erased the core record: %d, %v", count, err)
	}
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM achievements").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed optional batch left partial awards: %d, %v", count, err)
	}
	if _, err := db.ExecContext(t.Context(), "DROP TRIGGER reject_optional_award"); err != nil {
		t.Fatal(err)
	}
	awards, _, err := service.Collection(t.Context(), "owner")
	if err != nil || len(awards) != 2 ||
		!findDiscoveryAward(t, awards, "six_seven").EarnedAt.Equal(first.CreatedAt) ||
		!findDiscoveryAward(t, awards, "bracket_architect").EarnedAt.Equal(first.CreatedAt) {
		t.Fatalf("quiet recovery did not backfill first action: %+v, %v", awards, err)
	}
	if newAwards, events, err := service.Process(t.Context(), "owner", first); err != nil || len(newAwards) != 0 || len(events) != 0 {
		t.Fatalf("replay announced recovered award: %+v, %+v, %v", newAwards, events, err)
	}
}

func TestDiscoveryHandlerCommitsOnceAndNeverAnnouncesReplay(t *testing.T) {
	db, _ := boundaryDatabase(t)
	enabled := newHandler(db, nil, true, true)
	owner := browserSession(t, enabled)
	body := `{"requestId":"one-action","expression":"60+7","angleUnit":"deg"}`
	first := decodeBody[contracts.CalculationResponse](t,
		apiRequest(enabled, owner, http.MethodPost, "/api/calculations", body), http.StatusOK)
	if first.Calculation.Outcome.Kind != "success" || first.Calculation.Outcome.Value != "67" ||
		len(first.Achievements) != 1 || first.Achievements[0].ID != "six_seven" ||
		len(first.FunEvents) != 1 || first.FunEvents[0].RuleID != "six_seven" {
		t.Fatalf("accepted 67 did not grant its discovery: %+v", first)
	}
	replay := decodeBody[contracts.CalculationResponse](t,
		apiRequest(enabled, owner, http.MethodPost, "/api/calculations", body), http.StatusOK)
	if replay.Calculation.ID != first.Calculation.ID || len(replay.Achievements) != 0 || len(replay.FunEvents) != 0 {
		t.Fatalf("transport replay announced discovery: %+v", replay)
	}
	if conflict := apiRequest(enabled, owner, http.MethodPost, "/api/calculations",
		`{"requestId":"one-action","expression":"69","angleUnit":"deg"}`); conflict.Code != http.StatusConflict {
		t.Fatalf("changed request replay accepted: %d, %s", conflict.Code, conflict.Body.String())
	}
	session := decodeBody[contracts.SessionResponse](t,
		apiRequest(enabled, owner, http.MethodGet, "/api/session", ""), http.StatusOK)
	if !session.DiscoveriesAvailable || len(session.Achievements) != 1 ||
		session.Achievements[0].ID != "six_seven" || len(session.DiscoveryCatalog) != 8 {
		t.Fatalf("session did not recover earned catalog: %+v", session)
	}
	disabled := newHandler(db, nil, true, false)
	off := decodeBody[contracts.SessionResponse](t,
		apiRequest(disabled, owner, http.MethodGet, "/api/session", ""), http.StatusOK)
	if off.DiscoveriesAvailable || len(off.Achievements) != 0 || len(off.DiscoveryCatalog) != 0 {
		t.Fatalf("disabled module leaked collection: %+v", off)
	}
	var count int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM calculations").Scan(&count); err != nil || count != 1 {
		t.Fatalf("replay and session changed history: %d, %v", count, err)
	}
}

func TestDiscoveryConcurrentHTTPRetriesAnnounceOnlyOnce(t *testing.T) {
	db, _ := boundaryDatabase(t)
	handler := newHandler(db, nil, true, true)
	owner := browserSession(t, handler)
	const body = `{"requestId":"concurrent-discovery","expression":"60+7","angleUnit":"deg"}`
	responses := make(chan *httptest.ResponseRecorder, 8)
	var requests sync.WaitGroup
	for range 8 {
		requests.Add(1)
		go func() {
			defer requests.Done()
			responses <- apiRequest(handler, owner, http.MethodPost, "/api/calculations", body)
		}()
	}
	requests.Wait()
	close(responses)
	var awards, events int
	var calculationID string
	for response := range responses {
		data := decodeBody[contracts.CalculationResponse](t, response, http.StatusOK)
		if calculationID != "" && calculationID != data.Calculation.ID {
			t.Fatal("concurrent retries created different records")
		}
		calculationID = data.Calculation.ID
		awards += len(data.Achievements)
		events += len(data.FunEvents)
	}
	if awards != 1 || events != 1 {
		t.Fatalf("concurrent replies announced %d awards and %d events", awards, events)
	}
	history := decodeBody[contracts.HistoryPage](t, apiRequest(handler, owner, http.MethodGet, "/api/history", ""), http.StatusOK)
	if len(history.Items) != 1 {
		t.Fatalf("concurrent replies stored %d calculations", len(history.Items))
	}
}

func TestDiscoveryUsageFollowupsUseAcceptedSequenceWithoutNewAwards(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := fixedDiscoveryService(db)
	start := service.now().Add(-time.Second)
	records := make([]contracts.CalculationRecord, 101)
	for i := range records {
		records[i] = saveDiscoveryAction(t, db, "owner", fmt.Sprintf("usage-%d", i+1), "1/0", start.Add(time.Duration(i)*time.Millisecond))
	}
	for _, number := range []int{49, 50, 51, 99, 100, 101} {
		awards, events, err := service.Process(t.Context(), "owner", records[number-1])
		if err != nil {
			t.Fatal(err)
		}
		if len(awards) != 0 {
			t.Fatalf("followup created new achievements: %+v", awards)
		}
		if number == 50 || number == 100 {
			if len(events) != 1 || events[0].RuleID != "touch_grass" || events[0].Params["count"] != int64(number) {
				t.Fatalf("action %d followup = %+v", number, events)
			}
		} else if len(events) != 0 {
			t.Fatalf("action %d announced a non-milestone: %+v", number, events)
		}
	}
	collection, _, err := service.Collection(t.Context(), "owner")
	if err != nil || len(collection) != 1 || collection[0].ID != "touch_grass" {
		t.Fatalf("usage followups changed the one-time collection: %+v, %v", collection, err)
	}
}

func TestDiscoveryHTTPOptionalFailureStillConfirmsCalculation(t *testing.T) {
	db, _ := boundaryDatabase(t)
	handler := newHandler(db, nil, true, true)
	owner := browserSession(t, handler)
	if _, err := db.Exec(`
		CREATE TRIGGER reject_http_award BEFORE INSERT ON achievements
		BEGIN SELECT RAISE(ABORT, 'optional award unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	const body = `{"requestId":"optional-failure","expression":"60+7","angleUnit":"deg"}`
	data := decodeBody[contracts.CalculationResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", body), http.StatusOK)
	if data.Calculation.Outcome.Value != "67" || len(data.Achievements) != 0 || len(data.FunEvents) != 0 {
		t.Fatalf("optional failure altered or overclaimed response: %+v", data)
	}
	if _, err := db.Exec("DROP TRIGGER reject_http_award"); err != nil {
		t.Fatal(err)
	}
	session := decodeBody[contracts.SessionResponse](t, apiRequest(handler, owner, http.MethodGet, "/api/session", ""), http.StatusOK)
	if len(session.Achievements) != 1 || session.Achievements[0].ID != "six_seven" {
		t.Fatalf("quiet bootstrap failed to recover persisted eligibility: %+v", session)
	}
	retry := decodeBody[contracts.CalculationResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", body), http.StatusOK)
	if retry.Calculation.ID != data.Calculation.ID || len(retry.FunEvents) != 0 || len(retry.Achievements) != 0 {
		t.Fatalf("recovery retry replayed optional effects: %+v", retry)
	}
}

func TestDiscoveryDelayedProcessingPersistsAwardWithoutExpiredComment(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := fixedDiscoveryService(db)
	old := saveDiscoveryAction(t, db, "owner", "delayed-action", "60+7",
		service.now().Add(-discoveryCommentTTL-time.Second))
	awards, events, err := service.Process(t.Context(), "owner", old)
	if err != nil || len(awards) != 1 || awards[0].ID != "six_seven" || len(events) != 0 {
		t.Fatalf("stale action should award quietly: %+v, %+v, %v", awards, events, err)
	}
	collection, _, err := service.Collection(t.Context(), "owner")
	if err != nil || len(collection) != 1 || !collection[0].EarnedAt.Equal(old.CreatedAt) {
		t.Fatalf("expired comment changed durable award: %+v, %v", collection, err)
	}
}

func TestDiscoveryCommentLifetimeBoundaries(t *testing.T) {
	for _, test := range []struct {
		name    string
		age     time.Duration
		comment bool
	}{
		{"future", -time.Nanosecond, false},
		{"accepted now", 0, true},
		{"just before expiry", discoveryCommentTTL - time.Nanosecond, true},
		{"at expiry", discoveryCommentTTL, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, _ := boundaryDatabase(t)
			service := fixedDiscoveryService(db)
			record := saveDiscoveryAction(t, db, "owner", "action", "60+7", service.now().Add(-test.age))
			awards, events, err := service.Process(t.Context(), "owner", record)
			if err != nil || len(awards) != 1 || awards[0].ID != "six_seven" || !awards[0].EarnedAt.Equal(record.CreatedAt) {
				t.Fatalf("comment timing altered durable award: %+v, %v", awards, err)
			}
			want := 0
			if test.comment {
				want = 1
			}
			if len(events) != want {
				t.Fatalf("comments = %+v, want %d", events, want)
			}
		})
	}
}

func TestDisabledDiscoveryServiceIsInert(t *testing.T) {
	if service := newDiscoveryService(nil, false); service != nil {
		t.Fatalf("disabled constructor produced service: %+v", service)
	}
	var service *discoveryService
	if awards, events, err := service.Process(t.Context(), "owner", contracts.CalculationRecord{}); err != nil || len(awards) != 0 || len(events) != 0 {
		t.Fatalf("disabled process = %+v, %+v, %v", awards, events, err)
	}
	if awards, catalog, err := service.Collection(t.Context(), "owner"); err != nil || len(awards) != 0 || len(catalog) != 0 {
		t.Fatalf("disabled collection = %+v, %+v, %v", awards, catalog, err)
	}
}

func TestDiscoveryInterruptedCatchupResumesAcrossReopenAndStreakBoundary(t *testing.T) {
	db, path := boundaryDatabase(t)
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	total := 3*discoveryBatchSize + 7
	var last contracts.CalculationRecord
	for i := range total {
		expression := "1/0"
		switch {
		case i >= discoveryBatchSize-2 && i <= discoveryBatchSize:
			expression = "2+2"
		case i == 2*discoveryBatchSize+5:
			expression = "60+7"
		case i == total-1:
			expression = "400+4"
		}
		last = saveDiscoveryAction(t, db, "owner", fmt.Sprintf("resume-%03d", i),
			expression, start.Add(time.Duration(i)*time.Second))
	}
	// Interrupt at a deterministic commit boundary, not via elapsed time.
	// The first bounded batch must remain durable when the next cannot commit.
	if _, err := db.ExecContext(t.Context(), fmt.Sprintf(`
		CREATE TRIGGER interrupt_catchup BEFORE UPDATE ON discovery_progress
		WHEN NEW.accepted_count > %d
		BEGIN SELECT RAISE(ABORT, 'catchup interrupted'); END`, discoveryBatchSize)); err != nil {
		t.Fatal(err)
	}
	service := fixedDiscoveryService(db)
	if awards, _, err := service.Collection(t.Context(), "owner"); err == nil || len(awards) != 0 {
		t.Fatalf("interrupted collection claimed completion: %+v, %v", awards, err)
	}
	progress, err := storage.ReadDiscoveryProgress(t.Context(), db, "owner")
	if err != nil || progress.AcceptedCount != discoveryBatchSize || progress.LastSequence != discoveryBatchSize {
		t.Fatalf("interrupted checkpoint = %+v, %v", progress, err)
	}
	// A mandatory field outside the bounded predecessor window is now unreadable.
	// Successful resumption proves this evaluated prefix is not replayed.
	if _, err := db.ExecContext(t.Context(), `
		DROP TRIGGER interrupt_catchup;
		UPDATE calculations SET outcome_json = 'broken old payload' WHERE id = 'resume-000'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	service = fixedDiscoveryService(reopened)
	collection, _, err := service.Collection(t.Context(), "owner")
	if err != nil || len(collection) != 4 {
		t.Fatalf("resumed collection = %+v, %v", collection, err)
	}
	for id, offset := range map[string]int{
		"touch_grass": 24, "peer_review": discoveryBatchSize,
		"six_seven": 2*discoveryBatchSize + 5, "result_found": total - 1,
	} {
		award := findDiscoveryAward(t, collection, id)
		if !award.EarnedAt.Equal(start.Add(time.Duration(offset) * time.Second)) {
			t.Fatalf("%s source moved: %+v", id, award)
		}
	}
	progress, err = storage.ReadDiscoveryProgress(t.Context(), reopened, "owner")
	if err != nil || progress.AcceptedCount != int64(total) || progress.LastSequence != int64(total) {
		t.Fatalf("resumed checkpoint = %+v, %v", progress, err)
	}
	awards, events, err := service.Process(t.Context(), "owner", last)
	if err != nil || len(awards) != 0 || len(events) != 0 {
		t.Fatalf("quiet catchup was announced later: %+v, %+v, %v", awards, events, err)
	}
}

func TestDiscoveryIncrementalActionsDoNotReplayEvaluatedPrefix(t *testing.T) {
	db, _ := boundaryDatabase(t)
	service := fixedDiscoveryService(db)
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for i := range 24 {
		saveDiscoveryAction(t, db, "owner", fmt.Sprintf("incremental-%02d", i), "1/0", start)
	}
	if _, _, err := service.Collection(t.Context(), "owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `
		UPDATE calculations SET outcome_json = 'unreadable evaluated prefix'
		WHERE id = 'incremental-00'`); err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		saveDiscoveryAction(t, db, "other", fmt.Sprintf("foreign-%02d", i), "1/0", start)
	}
	current := saveDiscoveryAction(t, db, "owner", "ordinal-25", "1/0", start.Add(time.Minute))
	awards, events, err := service.Process(t.Context(), "owner", current)
	if err != nil || len(awards) != 1 || awards[0].ID != "touch_grass" ||
		!awards[0].EarnedAt.Equal(current.CreatedAt) || len(events) != 0 {
		t.Fatalf("incremental owner ordinal failed: %+v, %+v, %v", awards, events, err)
	}
	next := saveDiscoveryAction(t, db, "owner", "incremental-answer", "60+7", start.Add(2*time.Minute))
	awards, _, err = service.Process(t.Context(), "owner", next)
	if err != nil || len(awards) != 1 || awards[0].ID != "six_seven" {
		t.Fatalf("normal action restarted prefix or replayed grants: %+v, %v", awards, err)
	}
	progress, err := storage.ReadDiscoveryProgress(t.Context(), db, "owner")
	if err != nil || progress.AcceptedCount != 26 || progress.LastSequence != 31 {
		t.Fatalf("owner progress counted foreign actions: %+v, %v", progress, err)
	}
}

func TestDiscoveryCompetingReconciliationsKeepEarliestSource(t *testing.T) {
	db, _ := boundaryDatabase(t)
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	total := 2*discoveryBatchSize + 9
	firstMatch := discoveryBatchSize + 3
	var last contracts.CalculationRecord
	for i := range total {
		expression := "1/0"
		if i == firstMatch || i == total-1 {
			expression = "60+7"
		}
		last = saveDiscoveryAction(t, db, "owner", fmt.Sprintf("competing-%03d", i),
			expression, start.Add(time.Duration(i)*time.Second))
	}
	services := []*discoveryService{fixedDiscoveryService(db), fixedDiscoveryService(db)}
	startWorkers := make(chan struct{})
	failures := make(chan error, 8)
	var workers sync.WaitGroup
	for i := range 8 {
		workers.Go(func() {
			<-startWorkers
			service := services[i%len(services)]
			if i%2 == 0 {
				awards, events, err := service.Process(t.Context(), "owner", last)
				if err == nil && (len(awards) != 0 || len(events) != 0) {
					err = fmt.Errorf("later action announced historical awards: %+v, %+v", awards, events)
				}
				failures <- err
			} else {
				_, _, err := service.Collection(t.Context(), "owner")
				failures <- err
			}
		})
	}
	close(startWorkers)
	workers.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	collection, _, err := services[0].Collection(t.Context(), "owner")
	if err != nil || len(collection) != 2 {
		t.Fatalf("competing collection = %+v, %v", collection, err)
	}
	award := findDiscoveryAward(t, collection, "six_seven")
	if !award.EarnedAt.Equal(start.Add(time.Duration(firstMatch) * time.Second)) {
		t.Fatalf("competing reconciliation moved earnedAt: %+v", award)
	}
	var source string
	if err := db.QueryRowContext(t.Context(), `
		SELECT calculation_id FROM achievements WHERE session_id = 'owner' AND achievement_id = 'six_seven'`).Scan(&source); err != nil ||
		source != fmt.Sprintf("competing-%03d", firstMatch) {
		t.Fatalf("competing source = %q, %v", source, err)
	}
	progress, err := storage.ReadDiscoveryProgress(t.Context(), db, "owner")
	if err != nil || progress.AcceptedCount != int64(total) || progress.LastSequence != int64(total) {
		t.Fatalf("competing prefix skipped actions: %+v, %v", progress, err)
	}
}
