package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
)

func TestReductionAPIUsesOwnedSavedOutcomeWithoutSideEffects(t *testing.T) {
	db, _ := boundaryDatabase(t)
	handler := newHandler(db, nil, true)
	owner := browserSession(t, handler)
	other := browserSession(t, handler)
	caps := decodeBody[contracts.Capabilities](t,
		apiRequest(handler, nil, http.MethodGet, "/api/capabilities", ""), http.StatusOK)
	if !caps.Features.ReductionPlayback {
		t.Fatal("working reduction endpoint was not advertised")
	}
	decodeBody[contracts.ErrorResponse](t,
		apiRequest(handler, nil, http.MethodGet, "/api/calculations/unknown/reduction", ""), http.StatusUnauthorized)

	for i, tc := range []struct {
		expression string
		unit       contracts.AngleUnit
		value      string
		steps      int
	}{
		{"2+3*4", contracts.Degrees, "14", 2},
		{"sqrt(81)+2^3", contracts.Degrees, "17", 3},
		{"(1-3)^2", contracts.Degrees, "4", 2},
		{"42", contracts.Degrees, "42", 0},
		{"0.1+0.2", contracts.Degrees, "0.30000000000000004", 1},
		{"sin(90)", contracts.Degrees, "1", 1},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			request, _ := json.Marshal(contracts.CalculationRequest{
				RequestID: fmt.Sprintf("reduction-%d", i), Expression: tc.expression, AngleUnit: tc.unit,
			})
			saved := decodeBody[contracts.CalculationResponse](t,
				apiRequest(handler, owner, http.MethodPost, "/api/calculations", string(request)), http.StatusOK).Calculation
			if saved.Outcome.Value != tc.value {
				t.Fatalf("saved outcome = %+v, want %s", saved.Outcome, tc.value)
			}
			path := "/api/calculations/" + saved.ID + "/reduction"
			foreign := decodeBody[contracts.ErrorResponse](t,
				apiRequest(handler, other, http.MethodGet, path, ""), http.StatusNotFound)
			if foreign.Error.Code != "NOT_FOUND" {
				t.Errorf("foreign record error = %+v", foreign)
			}
			response := apiRequest(handler, owner, http.MethodGet, path, "")
			sequence := decodeBody[contracts.ReductionResponse](t, response, http.StatusOK)
			if response.Header().Get("Cache-Control") != "no-store" || sequence.CalculationID != saved.ID ||
				sequence.InitialExpression != saved.Expression || sequence.FinalExpression != saved.Outcome.Value ||
				len(sequence.Steps) != tc.steps {
				t.Fatalf("reduction = %+v, saved = %+v", sequence, saved)
			}
			current := sequence.InitialExpression
			for j, step := range sequence.Steps {
				if step.Before != current || step.Span.Start < 0 || step.Span.End > len(step.Before) ||
					step.Span.Start >= step.Span.End ||
					step.Before[:step.Span.Start]+step.Replacement+step.Before[step.Span.End:] != step.After {
					t.Fatalf("step %d invalid: %+v", j, step)
				}
				current = step.After
			}
			if len(sequence.Steps) > 0 && current != saved.Outcome.Value {
				t.Errorf("last after = %q, want %q", current, saved.Outcome.Value)
			}
		})
	}

	historyBefore := decodeBody[contracts.HistoryPage](t,
		apiRequest(handler, owner, http.MethodGet, "/api/history", ""), http.StatusOK)
	statsBefore := decodeBody[contracts.PersonalStatistics](t,
		apiRequest(handler, owner, http.MethodGet, "/api/statistics", ""), http.StatusOK)
	var awardsBefore int
	if err := db.QueryRow("SELECT COUNT(*) FROM achievements").Scan(&awardsBefore); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		for _, record := range historyBefore.Items {
			decodeBody[contracts.ReductionResponse](t,
				apiRequest(handler, owner, http.MethodGet, "/api/calculations/"+record.ID+"/reduction", ""), http.StatusOK)
		}
	}
	historyAfter := decodeBody[contracts.HistoryPage](t,
		apiRequest(handler, owner, http.MethodGet, "/api/history", ""), http.StatusOK)
	statsAfter := decodeBody[contracts.PersonalStatistics](t,
		apiRequest(handler, owner, http.MethodGet, "/api/statistics", ""), http.StatusOK)
	var awardsAfter int
	if err := db.QueryRow("SELECT COUNT(*) FROM achievements").Scan(&awardsAfter); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(historyBefore, historyAfter) || !reflect.DeepEqual(statsBefore, statsAfter) || awardsBefore != awardsAfter {
		t.Error("reduction reads changed history, statistics, or achievements")
	}
}

func TestReductionAPIRejectsUnavailableRecordsAndCanBeDisabled(t *testing.T) {
	db, _ := boundaryDatabase(t)
	enabled := newHandlerWithPlayback(db, nil, true, true)
	disabled := newHandlerWithPlayback(db, nil, true, false)
	owner := browserSession(t, enabled)
	post := func(id, expression string) contracts.CalculationRecord {
		t.Helper()
		body, _ := json.Marshal(contracts.CalculationRequest{RequestID: id, Expression: expression, AngleUnit: contracts.Degrees})
		return decodeBody[contracts.CalculationResponse](t,
			apiRequest(enabled, owner, http.MethodPost, "/api/calculations", string(body)), http.StatusOK).Calculation
	}
	success := post("success", "2+3")
	failure := post("failure", "1/0")
	for _, id := range []string{"missing", failure.ID} {
		wantCode, wantStatus := "NOT_FOUND", http.StatusNotFound
		if id == failure.ID {
			wantCode, wantStatus = "REDUCTION_UNAVAILABLE", http.StatusConflict
		}
		got := decodeBody[contracts.ErrorResponse](t,
			apiRequest(enabled, owner, http.MethodGet, "/api/calculations/"+id+"/reduction", ""), wantStatus)
		if got.Error.Code != wantCode {
			t.Errorf("record %s error = %+v, want %s", id, got, wantCode)
		}
	}
	path := "/api/calculations/" + success.ID + "/reduction"
	if _, err := db.Exec("UPDATE calculations SET semantics_version = ? WHERE id = ?", "old-rules", success.ID); err != nil {
		t.Fatal(err)
	}
	old := decodeBody[contracts.ErrorResponse](t, apiRequest(enabled, owner, http.MethodGet, path, ""), http.StatusConflict)
	if old.Error.Code != "REDUCTION_UNAVAILABLE" {
		t.Errorf("old semantics = %+v", old)
	}
	if _, err := db.Exec("UPDATE calculations SET semantics_version = ? WHERE id = ?", semanticsVersion, success.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE calculations SET outcome_json = ? WHERE id = ?", `{"kind":"success","value":"999"}`, success.ID); err != nil {
		t.Fatal(err)
	}
	mismatch := decodeBody[contracts.ErrorResponse](t, apiRequest(enabled, owner, http.MethodGet, path, ""), http.StatusConflict)
	if mismatch.Error.Code != "REDUCTION_UNAVAILABLE" {
		t.Errorf("mismatched saved value = %+v", mismatch)
	}
	long := post("intermediate-limit", "0.1+0.2+"+strings.Repeat("0", 1005)+"1")
	if long.Outcome.Kind != "success" {
		t.Fatalf("ordinary calculation should succeed: %+v", long.Outcome)
	}
	limited := decodeBody[contracts.ErrorResponse](t,
		apiRequest(enabled, owner, http.MethodGet, "/api/calculations/"+long.ID+"/reduction", ""), http.StatusConflict)
	if limited.Error.Code != "REDUCTION_UNAVAILABLE" {
		t.Errorf("unreplayable intermediate = %+v", limited)
	}
	caps := decodeBody[contracts.Capabilities](t, apiRequest(disabled, nil, http.MethodGet, "/api/capabilities", ""), http.StatusOK)
	if caps.Features.ReductionPlayback {
		t.Fatal("disabled playback advertised as available")
	}
	decodeBody[contracts.ErrorResponse](t, apiRequest(disabled, owner, http.MethodGet, path, ""), http.StatusNotFound)
	body, _ := json.Marshal(contracts.CalculationRequest{RequestID: "while-disabled", Expression: "7+8", AngleUnit: contracts.Degrees})
	ordinary := decodeBody[contracts.CalculationResponse](t,
		apiRequest(disabled, owner, http.MethodPost, "/api/calculations", string(body)), http.StatusOK)
	if ordinary.Calculation.Outcome.Value != "15" {
		t.Errorf("ordinary calculation while playback disabled = %+v", ordinary.Calculation.Outcome)
	}
}

func TestUTF16ReductionSpanBoundaries(t *testing.T) {
	for _, tc := range []struct {
		offset int
		byteAt int
		valid  bool
	}{
		{0, 0, true}, {1, 1, true}, {2, 0, false}, {3, 5, true}, {4, 6, true},
	} {
		got, valid := byteOffsetUTF16("a😀b", tc.offset)
		if got != tc.byteAt || valid != tc.valid {
			t.Errorf("offset %d = %d, %t; want %d, %t", tc.offset, got, valid, tc.byteAt, tc.valid)
		}
	}
}

type inconsistentEngine struct{ calculation.ReductionEngine }

func (e inconsistentEngine) Reduce(ctx context.Context, in calculation.Input) (calculation.Reduction, error) {
	sequence, err := e.ReductionEngine.Reduce(ctx, in)
	if len(sequence.Steps) > 0 {
		sequence.Steps[0].After = "not the next expression"
	}
	return sequence, err
}

type queryingEngine struct {
	calculation.ReductionEngine
	db *sql.DB
}

func (e queryingEngine) Reduce(ctx context.Context, in calculation.Input) (calculation.Reduction, error) {
	var count int
	if err := e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM calculations").Scan(&count); err != nil {
		return calculation.Reduction{}, err
	}
	return e.ReductionEngine.Reduce(ctx, in)
}

func TestReductionReleasesSQLiteBeforeGeneration(t *testing.T) {
	db, _ := boundaryDatabase(t)
	base := newHandler(db, nil, true)
	owner := browserSession(t, base)
	saved := decodeBody[contracts.CalculationResponse](t,
		apiRequest(base, owner, http.MethodPost, "/api/calculations", `{"requestId":"released","expression":"2+3","angleUnit":"deg"}`), http.StatusOK).Calculation
	a := api{db: db, engine: queryingEngine{ReductionEngine: calculation.New(), db: db}, reductionEnabled: true}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/calculations/{id}/reduction", a.reduction)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/api/calculations/"+saved.ID+"/reduction", nil).WithContext(ctx)
	request.AddCookie(owner)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	decodeBody[contracts.ReductionResponse](t, response, http.StatusOK)
}

func TestReductionAPIRejectsInconsistentEngineTrace(t *testing.T) {
	db, _ := boundaryDatabase(t)
	base := newHandler(db, nil, true)
	owner := browserSession(t, base)
	body := `{"requestId":"trace","expression":"2+3*4","angleUnit":"deg"}`
	saved := decodeBody[contracts.CalculationResponse](t,
		apiRequest(base, owner, http.MethodPost, "/api/calculations", body), http.StatusOK).Calculation
	a := api{db: db, engine: inconsistentEngine{calculation.New()}, reductionEnabled: true}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/calculations/{id}/reduction", a.reduction)
	got := decodeBody[contracts.ErrorResponse](t,
		apiRequest(mux, owner, http.MethodGet, "/api/calculations/"+saved.ID+"/reduction", ""), http.StatusConflict)
	if got.Error.Code != "REDUCTION_UNAVAILABLE" {
		t.Errorf("inconsistent trace = %+v", got)
	}
}
