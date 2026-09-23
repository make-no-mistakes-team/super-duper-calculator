package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func TestCalculationHistoryAndIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calculator.sqlite")
	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	handler := newHandler(db)

	type browser struct{ cookie *http.Cookie }
	personA, personB := &browser{}, &browser{}
	send := func(person *browser, method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(t.Context())
		if person.cookie != nil {
			request.AddCookie(person.cookie)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if cookies := response.Result().Cookies(); len(cookies) > 0 {
			person.cookie = cookies[0]
		}
		return response
	}
	post := func(person *browser, id, expression string) (int, contracts.CalculationResponse) {
		t.Helper()
		body, _ := json.Marshal(contracts.CalculationRequest{RequestID: id, Expression: expression, AngleUnit: contracts.Degrees})
		response := send(person, http.MethodPost, "/api/calculations", body)
		var result contracts.CalculationResponse
		if response.Code == http.StatusOK {
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
		}
		return response.Code, result
	}
	page := func(person *browser, query string) contracts.HistoryPage {
		t.Helper()
		response := send(person, http.MethodGet, "/api/history"+query, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("history status = %d: %s", response.Code, response.Body.String())
		}
		var result contracts.HistoryPage
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	if status, result := post(personA, "action-1", "2+3*4"); status != 200 || result.Calculation.Outcome.Value != "14" {
		t.Fatalf("calculation = %d %+v", status, result)
	}
	foreignBody, _ := json.Marshal(contracts.CalculationRequest{RequestID: "foreign", Expression: "1+1", AngleUnit: contracts.Degrees})
	foreignRequest := httptest.NewRequest(http.MethodPost, "/api/calculations", bytes.NewReader(foreignBody))
	foreignRequest.Header.Set("Origin", "https://other.example")
	foreignRequest.AddCookie(personA.cookie)
	foreignResponse := httptest.NewRecorder()
	handler.ServeHTTP(foreignResponse, foreignRequest)
	if foreignResponse.Code != http.StatusForbidden {
		t.Fatalf("foreign origin status = %d", foreignResponse.Code)
	}
	if status, result := post(personA, "action-1", "2+3*4"); status != 200 || result.Calculation.Outcome.Value != "14" {
		t.Fatalf("retry = %d %+v", status, result)
	}
	if status, _ := post(personA, "action-1", "9"); status != http.StatusConflict {
		t.Fatalf("changed retry status = %d", status)
	}
	if status, result := post(personA, "action-2", "5*("); status != 200 || result.Calculation.Outcome.Error == nil || result.Calculation.Outcome.Error.Code != "SYNTAX_ERROR" {
		t.Fatalf("math error = %d %+v", status, result)
	}
	first := page(personA, "?limit=1")
	if len(first.Items) != 1 || first.Items[0].Expression != "5*(" || first.NextCursor == nil {
		t.Fatalf("first history page = %+v", first)
	}
	second := page(personA, "?limit=1&cursor="+*first.NextCursor)
	if len(second.Items) != 1 || second.Items[0].Expression != "2+3*4" || second.NextCursor != nil {
		t.Fatalf("second history page = %+v", second)
	}
	if history := page(personB, ""); len(history.Items) != 0 {
		t.Fatalf("second browser can see personal history: %+v", history)
	}
	if status, _ := post(personB, "action-1", "7"); status != 200 {
		t.Fatalf("second browser status = %d", status)
	}
	if history := page(personA, ""); len(history.Items) != 2 {
		t.Fatalf("first browser history changed: %+v", history)
	}
	if status, _ := post(personA, "too-long", strings.Repeat("1", 1025)); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("limit status = %d", status)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	handler = newHandler(db)
	if history := page(personA, ""); len(history.Items) != 2 {
		t.Fatalf("history after restart = %+v", history)
	}
}
