package main

import (
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func TestStatisticsAPITracksOwnedAcceptedActions(t *testing.T) {
	db, path := boundaryDatabase(t)
	handler := newHandler(db, nil, true, false)
	decodeBody[contracts.ErrorResponse](t, apiRequest(handler, nil, http.MethodGet, "/api/statistics", ""), http.StatusUnauthorized)
	owner := browserSession(t, handler)
	other := browserSession(t, handler)
	get := func(cookie *http.Cookie) contracts.PersonalStatistics {
		t.Helper()
		return decodeBody[contracts.PersonalStatistics](t, apiRequest(handler, cookie, http.MethodGet, "/api/statistics", ""), http.StatusOK)
	}
	empty := get(owner)
	if empty.TotalCalculations != 0 || empty.LongestExpression != nil || empty.MaxParsedDepth != nil || empty.Operators == nil || empty.Functions == nil {
		t.Fatalf("empty statistics = %+v", empty)
	}
	body := `{"requestId":"scientific","expression":"sqrt(81)+2^3","angleUnit":"deg"}`
	first := decodeBody[contracts.CalculationResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", body), http.StatusOK)
	var retries sync.WaitGroup
	for range 8 {
		retries.Go(func() {
			response := apiRequest(handler, owner, http.MethodPost, "/api/calculations", body)
			if response.Code != http.StatusOK {
				t.Errorf("retry status = %d", response.Code)
			}
		})
	}
	retries.Wait()
	for _, next := range []string{
		strings.Replace(body, "scientific", "new-deliberate-action", 1),
		`{"requestId":"division","expression":"1/0","angleUnit":"deg"}`,
		`{"requestId":"syntax","expression":"sqrt(","angleUnit":"deg"}`,
	} {
		decodeBody[contracts.CalculationResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", next), http.StatusOK)
	}
	decodeBody[contracts.ErrorResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", `{"requestId":"rejected","expression":"1"}`), http.StatusBadRequest)
	oversized := `{"requestId":"too-long","expression":"` + strings.Repeat("1", 1025) + `","angleUnit":"deg"}`
	decodeBody[contracts.ErrorResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", oversized), http.StatusRequestEntityTooLarge)
	stats := get(owner)
	if stats.TotalCalculations != 4 || stats.Successes != 2 || stats.MathematicalErrors != 2 || stats.DivisionByZeroAttempts != 1 {
		t.Fatalf("action totals = %+v", stats)
	}
	if !reflect.DeepEqual(stats.Operators, map[string]int64{"+": 2, "^": 2, "/": 1}) || !reflect.DeepEqual(stats.Functions, map[string]int64{"sqrt": 2}) {
		t.Fatalf("usage counted retries or unparsed input: operators=%v functions=%v", stats.Operators, stats.Functions)
	}
	if stats.MaxParsedDepth == nil || *stats.MaxParsedDepth != 1 || stats.LongestExpression == nil ||
		stats.LongestExpression.CalculationID != first.Calculation.ID || stats.LongestExpression.Expression != "sqrt(81)+2^3" || stats.LongestExpression.Length != 12 {
		t.Fatalf("expression metrics = %+v", stats)
	}
	if foreign := get(other); foreign.TotalCalculations != 0 || foreign.LongestExpression != nil {
		t.Fatalf("another identity saw statistics: %+v", foreign)
	}
	spoofed := decodeBody[contracts.PersonalStatistics](t, apiRequest(handler, other, http.MethodGet, "/api/statistics?owner="+owner.Value+"&alias=owner", ""), http.StatusOK)
	if spoofed.TotalCalculations != 0 {
		t.Fatalf("client-supplied owner changed metrics: %+v", spoofed)
	}
	if again := get(owner); !reflect.DeepEqual(stats, again) {
		t.Fatalf("reading changed statistics: %+v != %+v", again, stats)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	decodeBody[contracts.ErrorResponse](t, apiRequest(handler, owner, http.MethodGet, "/api/statistics", ""), http.StatusServiceUnavailable)
	reopened, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	handler = newHandler(reopened, nil, true, false)
	if afterRestart := get(owner); !reflect.DeepEqual(stats, afterRestart) {
		t.Fatalf("restart changed derived metrics: %+v != %+v", afterRestart, stats)
	}
	history := decodeBody[contracts.HistoryPage](t, apiRequest(handler, owner, http.MethodGet, "/api/history", ""), http.StatusOK)
	if len(history.Items) != 4 {
		t.Fatalf("statistics reads changed history: %d records", len(history.Items))
	}
}

func TestStatisticsDisableAndDerivationFailureLeaveCoreAvailable(t *testing.T) {
	db, _ := boundaryDatabase(t)
	enabled := newHandler(db, nil, true, false)
	disabled := newHandler(db, nil, false, false)
	owner := browserSession(t, enabled)
	body := `{"requestId":"first","expression":"60+7","angleUnit":"deg"}`
	first := decodeBody[contracts.CalculationResponse](t, apiRequest(enabled, owner, http.MethodPost, "/api/calculations", body), http.StatusOK)
	caps := decodeBody[contracts.Capabilities](t, apiRequest(disabled, nil, http.MethodGet, "/api/capabilities", ""), http.StatusOK)
	if caps.Features.Statistics {
		t.Fatal("disabled statistics advertised as available")
	}
	decodeBody[contracts.ErrorResponse](t, apiRequest(disabled, owner, http.MethodGet, "/api/statistics", ""), http.StatusNotFound)
	decodeBody[contracts.CalculationResponse](t, apiRequest(disabled, owner, http.MethodPost, "/api/calculations", strings.Replace(body, "first", "while-disabled", 1)), http.StatusOK)
	stats := decodeBody[contracts.PersonalStatistics](t, apiRequest(enabled, owner, http.MethodGet, "/api/statistics", ""), http.StatusOK)
	if stats.TotalCalculations != 2 {
		t.Fatalf("statistics missed action recorded while disabled: %+v", stats)
	}
	caps = decodeBody[contracts.Capabilities](t, apiRequest(enabled, nil, http.MethodGet, "/api/capabilities", ""), http.StatusOK)
	if !caps.Features.Statistics {
		t.Fatal("working statistics endpoint was not advertised")
	}

	// This remains valid history JSON, but cannot supply a measured negative count.
	if _, err := db.Exec(`UPDATE calculations SET facts_json =
		'{"operators":{"+":-1},"functions":{},"operationCount":1,"depth":0}' WHERE id = ?`, first.Calculation.ID); err != nil {
		t.Fatal(err)
	}
	failure := decodeBody[contracts.ErrorResponse](t, apiRequest(enabled, owner, http.MethodGet, "/api/statistics", ""), http.StatusServiceUnavailable)
	if failure.Error.Code != "SERVICE_UNAVAILABLE" || len(failure.Error.Params) != 0 {
		t.Fatalf("derived-data failure exposed details: %+v", failure)
	}
	decodeBody[contracts.CalculationResponse](t, apiRequest(enabled, owner, http.MethodPost, "/api/calculations", strings.Replace(body, "first", "while-metrics-fail", 1)), http.StatusOK)
	history := decodeBody[contracts.HistoryPage](t, apiRequest(enabled, owner, http.MethodGet, "/api/history", ""), http.StatusOK)
	if len(history.Items) != 3 {
		t.Fatalf("optional statistics failure affected core history: %d", len(history.Items))
	}
	if _, err := db.Exec("UPDATE calculations SET facts_json = NULL WHERE id = ?", first.Calculation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE achievements"); err != nil {
		t.Fatal(err)
	}
	stats = decodeBody[contracts.PersonalStatistics](t, apiRequest(enabled, owner, http.MethodGet, "/api/statistics", ""), http.StatusOK)
	if stats.TotalCalculations != 3 {
		t.Fatalf("optional award storage affected statistics: %+v", stats)
	}
	decodeBody[contracts.CalculationResponse](t, apiRequest(enabled, owner, http.MethodPost, "/api/calculations", strings.Replace(body, "first", "without-awards", 1)), http.StatusOK)
	history = decodeBody[contracts.HistoryPage](t, apiRequest(enabled, owner, http.MethodGet, "/api/history", ""), http.StatusOK)
	if len(history.Items) != 4 {
		t.Fatalf("missing award storage affected history: %d", len(history.Items))
	}
}
