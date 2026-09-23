package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func boundaryDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "private", "calculator.sqlite")
	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func apiRequest(handler http.Handler, cookie *http.Cookie, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func browserSession(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	w := apiRequest(handler, nil, http.MethodGet, "/api/session", "")
	if w.Code != http.StatusOK || len(w.Result().Cookies()) != 1 {
		t.Fatalf("session = %d %s", w.Code, w.Body.String())
	}
	return w.Result().Cookies()[0]
}

func decodeBody[T any](t *testing.T, w *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d: %s", w.Code, status, w.Body.String())
	}
	var body T
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v: %s", err, w.Body.String())
	}
	return body
}

func TestSessionBootstrapOwnsIdentityCreation(t *testing.T) {
	db, _ := boundaryDatabase(t)
	handler := newHandler(db, nil)
	body := `{"requestId":"anonymous","expression":"2+2","angleUnit":"deg"}`
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/history", ""},
		{http.MethodPost, "/api/calculations", body},
	} {
		w := apiRequest(handler, nil, tc.method, tc.path, tc.body)
		failure := decodeBody[contracts.ErrorResponse](t, w, http.StatusUnauthorized)
		if failure.Error.Code != "SESSION_REQUIRED" || len(w.Result().Cookies()) != 0 {
			t.Fatalf("missing identity response = %+v", failure)
		}
	}
	foreign := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	foreign.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, foreign)
	decodeBody[contracts.ErrorResponse](t, w, http.StatusForbidden)
	var identities int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&identities); err != nil || identities != 0 {
		t.Fatalf("rejected requests created identities: %d, %v", identities, err)
	}

	cookie := browserSession(t, handler)
	decodeBody[contracts.CalculationResponse](t, apiRequest(handler, cookie, http.MethodPost, "/api/calculations", body), http.StatusOK)
	if _, err := db.Exec("UPDATE sessions SET expires_at = 0 WHERE id = ?", cookie.Value); err != nil {
		t.Fatal(err)
	}
	decodeBody[contracts.ErrorResponse](t, apiRequest(handler, cookie, http.MethodGet, "/api/history", ""), http.StatusUnauthorized)
	w = apiRequest(handler, cookie, http.MethodGet, "/api/session", "")
	decodeBody[map[string]string](t, w, http.StatusOK)
	if len(w.Result().Cookies()) != 1 {
		t.Fatal("expired session was not replaced through bootstrap")
	}
	page := decodeBody[contracts.HistoryPage](t, apiRequest(handler, w.Result().Cookies()[0], http.MethodGet, "/api/history", ""), http.StatusOK)
	if len(page.Items) != 0 {
		t.Fatal("new identity inherited expired identity's history")
	}
	var records int
	if err := db.QueryRow("SELECT COUNT(*) FROM calculations").Scan(&records); err != nil || records != 1 {
		t.Fatalf("session expiry removed committed history: %d, %v", records, err)
	}
}

func TestConcurrentActionReplayAndContextConflicts(t *testing.T) {
	db, _ := boundaryDatabase(t)
	handler := newHandler(db, nil)
	owner := browserSession(t, handler)
	body := `{"requestId":"same-action","expression":"0.1+0.2","angleUnit":"deg"}`
	responses := make(chan *httptest.ResponseRecorder, 16)
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() { responses <- apiRequest(handler, owner, http.MethodPost, "/api/calculations", body) })
	}
	group.Wait()
	close(responses)
	var savedID string
	for response := range responses {
		saved := decodeBody[contracts.CalculationResponse](t, response, http.StatusOK)
		if saved.Calculation.Outcome.Value != "0.30000000000000004" {
			t.Fatalf("canonical value changed: %+v", saved.Calculation)
		}
		if savedID == "" {
			savedID = saved.Calculation.ID
		} else if saved.Calculation.ID != savedID {
			t.Fatal("concurrent retries returned distinct records")
		}
	}
	for _, changed := range []string{
		`{"requestId":"same-action","expression":"1","angleUnit":"deg"}`,
		`{"requestId":"same-action","expression":"0.1+0.2","angleUnit":"rad"}`,
		`{"requestId":"same-action","expression":"0.1+0.2","angleUnit":"deg","room":{"code":"demo","publish":true}}`,
		`{"requestId":"same-action","expression":"0.1+0.2","angleUnit":"deg","room":{"code":"demo","publish":false}}`,
	} {
		decodeBody[contracts.ErrorResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", changed), http.StatusConflict)
	}
	freshRoom := `{"requestId":"room-action","expression":"1","angleUnit":"deg","room":{"code":"demo","publish":false}}`
	decodeBody[contracts.ErrorResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", freshRoom), http.StatusBadRequest)
	freshAction := strings.Replace(body, "same-action", "deliberate-action", 1)
	decodeBody[contracts.CalculationResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", freshAction), http.StatusOK)
	other := browserSession(t, handler)
	foreignReplay := decodeBody[contracts.CalculationResponse](t, apiRequest(handler, other, http.MethodPost, "/api/calculations", body), http.StatusOK)
	if foreignReplay.Calculation.ID == savedID {
		t.Fatal("another identity retrieved the original action")
	}
	page := decodeBody[contracts.HistoryPage](t, apiRequest(handler, owner, http.MethodGet, "/api/history", ""), http.StatusOK)
	if len(page.Items) != 2 {
		t.Fatalf("history contains %d actions, want two deliberate actions", len(page.Items))
	}
}

func TestHistoryPaginationPreservesOwnedHistoricalRecords(t *testing.T) {
	db, _ := boundaryDatabase(t)
	handler := newHandler(db, nil)
	owner := browserSession(t, handler)
	other := browserSession(t, handler)
	insert := func(i int, identity string) {
		t.Helper()
		_, err := db.Exec(`INSERT INTO calculations
			(id, session_id, request_id, expression, angle_unit, semantics_version, outcome_json, facts_json, created_at)
			VALUES (?, ?, ?, ?, 'rad', 'historical-version', ?, NULL, '2026-01-01T00:00:00Z')`,
			fmt.Sprint(i), identity, fmt.Sprint(i), fmt.Sprintf("historical(%d)", i), `{"kind":"success","value":"0.30000000000000004"}`)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := range 122 {
		insert(i, owner.Value)
	}
	insert(1000, other.Value)
	page := decodeBody[contracts.HistoryPage](t, apiRequest(handler, owner, http.MethodGet, "/api/history", ""), http.StatusOK)
	if len(page.Items) != 50 || page.NextCursor == nil {
		t.Fatalf("default page = %d items, cursor %v", len(page.Items), page.NextCursor)
	}
	decodeBody[contracts.ErrorResponse](t, apiRequest(handler, other, http.MethodGet, "/api/history?cursor="+*page.NextCursor, ""), http.StatusBadRequest)
	for _, query := range []string{"?cursor=not-a-cursor", "?cursor=%zz", "?cursor=0", "?limit=101"} {
		decodeBody[contracts.ErrorResponse](t, apiRequest(handler, owner, http.MethodGet, "/api/history"+query, ""), http.StatusBadRequest)
	}
	insert(2000, owner.Value)
	seen := make(map[string]bool)
	wantID := 121
	for {
		for _, record := range page.Items {
			if record.ID != fmt.Sprint(wantID) || seen[record.ID] {
				t.Fatalf("unstable historical ordering: got %s, want %d", record.ID, wantID)
			}
			if record.Expression != fmt.Sprintf("historical(%d)", wantID) || record.Context.AngleUnit != contracts.Radians ||
				record.Context.SemanticsVersion != "historical-version" || record.Outcome.Value != "0.30000000000000004" || record.Facts != nil {
				t.Fatalf("historical context/outcome changed: %+v", record)
			}
			seen[record.ID] = true
			wantID--
		}
		if page.NextCursor == nil {
			break
		}
		page = decodeBody[contracts.HistoryPage](t, apiRequest(handler, owner, http.MethodGet, "/api/history?limit=100&cursor="+*page.NextCursor, ""), http.StatusOK)
	}
	if wantID != -1 || len(seen) != 122 {
		t.Fatalf("history skipped records: next expected %d, seen %d", wantID, len(seen))
	}
	latest := decodeBody[contracts.HistoryPage](t, apiRequest(handler, owner, http.MethodGet, "/api/history?limit=100&owner="+other.Value+"&alias=other", ""), http.StatusOK)
	if len(latest.Items) != 100 || latest.Items[0].ID != "2000" {
		t.Fatal("new record missing, maximum page incorrect, or client owner overrode session")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM calculations").Scan(&count); err != nil || count != 124 {
		t.Fatalf("reads changed saved records: %d, %v", count, err)
	}
}

func TestStorageFailuresDoNotConfirmOrEraseActions(t *testing.T) {
	db, path := boundaryDatabase(t)
	handler := newHandler(db, nil)
	owner := browserSession(t, handler)
	body := `{"requestId":"committed","expression":"67","angleUnit":"deg"}`
	original := decodeBody[contracts.CalculationResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", body), http.StatusOK)
	const unsafeDetail = "private SQL diagnostic must not escape"
	if _, err := db.Exec(`CREATE TRIGGER reject_calculation BEFORE INSERT ON calculations BEGIN SELECT RAISE(ABORT, '` + unsafeDetail + `'); END`); err != nil {
		t.Fatal(err)
	}
	failedBody := strings.Replace(body, "committed", "failed-write", 1)
	failed := apiRequest(handler, owner, http.MethodPost, "/api/calculations", failedBody)
	decodeBody[contracts.ErrorResponse](t, failed, http.StatusServiceUnavailable)
	if strings.Contains(failed.Body.String(), unsafeDetail) {
		t.Fatal("storage error detail escaped")
	}
	if _, err := db.Exec("DROP TRIGGER reject_calculation"); err != nil {
		t.Fatal(err)
	}

	blocker, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	tx, err := blocker.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE sessions SET expires_at = expires_at WHERE id = ?", owner.Value); err != nil {
		t.Fatal(err)
	}
	decodeBody[contracts.ErrorResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", failedBody), http.StatusServiceUnavailable)
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	recovered := decodeBody[contracts.CalculationResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", failedBody), http.StatusOK)
	replay := decodeBody[contracts.CalculationResponse](t, apiRequest(handler, owner, http.MethodPost, "/api/calculations", failedBody), http.StatusOK)
	if recovered.Calculation.ID != replay.Calculation.ID {
		t.Fatal("retry after write recovery duplicated the action")
	}
	page := decodeBody[contracts.HistoryPage](t, apiRequest(handler, owner, http.MethodGet, "/api/history", ""), http.StatusOK)
	if len(page.Items) != 2 || page.Items[1].ID != original.Calculation.ID {
		t.Fatalf("write failure corrupted committed history: %+v", page)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	decodeBody[contracts.ErrorResponse](t, apiRequest(handler, owner, http.MethodGet, "/api/history", ""), http.StatusServiceUnavailable)
}

func TestUnknownAPIRoutesRemainJSONWithoutSideEffects(t *testing.T) {
	db, _ := boundaryDatabase(t)
	handler := newHandler(db, nil)
	for _, path := range []string{"/api", "/api/missing", "/api/calculations/not-owned/reduction"} {
		w := apiRequest(handler, nil, http.MethodGet, path, "")
		failure := decodeBody[contracts.ErrorResponse](t, w, http.StatusNotFound)
		if failure.Error.Code != "NOT_FOUND" || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || bytes.Contains(w.Body.Bytes(), []byte("<html")) {
			t.Fatalf("unknown API response = %s", w.Body.String())
		}
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count); err != nil || count != 0 {
		t.Fatalf("unknown API created identities: %d, %v", count, err)
	}
}
