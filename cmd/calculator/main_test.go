package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func TestReadinessTracksDatabaseAccess(t *testing.T) {
	db, err := storage.Open(t.Context(), filepath.Join(t.TempDir(), "private", "health.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler := readiness(db)

	checkResponse := func(wantCode int, wantStatus string) {
		t.Helper()
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/health/ready", nil).WithContext(t.Context())
		handler.ServeHTTP(response, request)
		if response.Code != wantCode {
			t.Fatalf("HTTP status = %d, want %d", response.Code, wantCode)
		}
		var body struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Status != wantStatus {
			t.Fatalf("status = %q, want %q", body.Status, wantStatus)
		}
	}

	checkResponse(http.StatusOK, "ok")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	checkResponse(http.StatusServiceUnavailable, "unavailable")
}
