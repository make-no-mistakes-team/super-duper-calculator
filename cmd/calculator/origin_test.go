package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func TestPublicOriginBoundary(t *testing.T) {
	db, err := storage.Open(t.Context(), filepath.Join(t.TempDir(), "private", "origin.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	publicOrigin, err := parsePublicOrigin("https://calculator.example")
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(db, publicOrigin, true, false)
	session := httptest.NewRecorder()
	// TLS terminates at the proxy; the Go request itself uses plain HTTP.
	handler.ServeHTTP(session, httptest.NewRequest(http.MethodGet, "http://backend:8080/api/session", nil))
	cookies := session.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("proxied HTTPS session lacks a secure HttpOnly cookie: %v", cookies)
	}
	for _, tc := range []struct {
		origin string
		status int
	}{
		{"https://calculator.example", http.StatusOK},
		{"https://calculator.example:443", http.StatusOK},
		{"http://calculator.example", http.StatusForbidden},
		{"https://calculator.example:444", http.StatusForbidden},
		{"https://other.example", http.StatusForbidden},
		{"https://calculator.example/path", http.StatusForbidden},
		{"null", http.StatusForbidden},
	} {
		request := httptest.NewRequest(http.MethodPost, "http://backend:8080/api/calculations",
			strings.NewReader(`{"requestId":"origin-check","expression":"1+1","angleUnit":"deg"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", tc.origin)
		request.AddCookie(cookies[0])
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Errorf("Origin %q: status %d, want %d", tc.origin, response.Code, tc.status)
		}
	}
}

func TestUntrustedForwardedOrigin(t *testing.T) {
	db, err := storage.Open(t.Context(), filepath.Join(t.TempDir(), "private", "direct.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler := newHandler(db, nil, true, false)
	request := httptest.NewRequest(http.MethodPost, "http://calculator.example/api/calculations",
		strings.NewReader(`{"requestId":"spoof","expression":"1","angleUnit":"deg"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("X-Forwarded-Host", "attacker.example")
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("forged forwarded headers accepted: %d", response.Code)
	}
	request.Header.Del("Origin")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-site mutation without Origin accepted: %d", response.Code)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM calculations").Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected origins persisted calculations: count %d, err %v", count, err)
	}
}

func TestInvalidPublicOrigin(t *testing.T) {
	for _, value := range []string{"calculator.example", "ftp://calculator.example", "https://user@calculator.example", "https://calculator.example/", "https://calculator.example?x=1", "https://calculator.example#fragment", "https://calculator.example:99999"} {
		if _, err := parsePublicOrigin(value); err == nil {
			t.Errorf("accepted invalid PUBLIC_ORIGIN %q", value)
		}
	}
}

func TestInvalidPublicOriginDoesNotDiscloseConfiguration(t *testing.T) {
	const credential = "private-origin-credential"
	_, err := parsePublicOrigin("https://user:" + credential + "@calculator.example:%")
	if err == nil || strings.Contains(err.Error(), credential) {
		t.Fatalf("unsafe configuration error: %v", err)
	}
}
