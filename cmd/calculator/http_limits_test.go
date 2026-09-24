package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func assertGuardError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("HTTP status = %d, want %d; body = %q", response.Code, status, response.Body.String())
	}
	var body struct {
		Error struct {
			Code   string         `json:"code"`
			Params map[string]any `json:"params"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != code || body.Error.Params == nil {
		t.Fatalf("error = %+v, want code %q and params object", body.Error, code)
	}
}

func TestAPIRateLimitRetryAndRejectedState(t *testing.T) {
	db, err := storage.Open(t.Context(), filepath.Join(t.TempDir(), "private", "rate.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Unix(1, 0)
	guard := newAPIGuard(apiLimitConfig{
		rate: 0.5, burst: 1, active: 1, timeout: time.Second, now: func() time.Time { return now },
	})
	handler := guard.wrap(newHandler(db, nil, true, false))
	var sessionCookie *http.Cookie
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if sessionCookie != nil {
			r.AddCookie(sessionCookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if response := request(http.MethodGet, "/api/capabilities", ""); response.Code != http.StatusOK {
		t.Fatalf("capabilities status = %d", response.Code)
	}
	rejected := request(http.MethodGet, "/api/session", "")
	assertGuardError(t, rejected, http.StatusTooManyRequests, "RATE_LIMITED")
	if retry := rejected.Header().Get("Retry-After"); retry != "2" {
		t.Fatalf("Retry-After = %q, want 2", retry)
	}
	if cookies := rejected.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("rejection set session cookies: %v", cookies)
	}
	var sessions, calculations int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM calculations").Scan(&calculations); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 || calculations != 0 {
		t.Fatalf("rejection created sessions=%d calculations=%d", sessions, calculations)
	}
	now = now.Add(time.Second)
	if retry := request(http.MethodGet, "/api/session", "").Header().Get("Retry-After"); retry != "1" {
		t.Fatalf("partly refilled Retry-After = %q, want 1", retry)
	}
	now = now.Add(time.Second)
	session := request(http.MethodGet, "/api/session", "")
	if session.Code != http.StatusOK || len(session.Result().Cookies()) != 1 {
		t.Fatalf("refilled session status = %d; cookies = %v", session.Code, session.Result().Cookies())
	}
	sessionCookie = session.Result().Cookies()[0]
	calculation := `{"requestId":"rate-check","expression":"1+1","angleUnit":"deg"}`
	rejected = request(http.MethodPost, "/api/calculations", calculation)
	assertGuardError(t, rejected, http.StatusTooManyRequests, "RATE_LIMITED")
	if err := db.QueryRow("SELECT COUNT(*) FROM calculations").Scan(&calculations); err != nil {
		t.Fatal(err)
	}
	if calculations != 0 {
		t.Fatalf("rejected action changed history count to %d", calculations)
	}
	now = now.Add(2 * time.Second)
	if response := request(http.MethodPost, "/api/calculations", calculation); response.Code != http.StatusOK {
		t.Fatalf("refilled calculation status = %d; body = %q", response.Code, response.Body.String())
	}
	rejected = request(http.MethodPost, "/api/calculations", `{"requestId":"another","expression":"3","angleUnit":"deg"}`)
	assertGuardError(t, rejected, http.StatusTooManyRequests, "RATE_LIMITED")
	if err := db.QueryRow("SELECT COUNT(*) FROM calculations").Scan(&calculations); err != nil {
		t.Fatal(err)
	}
	if calculations != 1 {
		t.Fatalf("rejected action changed history count to %d", calculations)
	}
}

func TestAPIActiveSaturationRecovers(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	guard := newAPIGuard(apiLimitConfig{
		rate: 1, burst: 3, active: 1, timeout: time.Minute, now: time.Now,
	})
	handler := guard.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hold" {
			close(entered)
			<-release
		}
		_, _ = io.WriteString(w, "ok")
	}))
	first := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() {
		handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/hold", nil))
		close(finished)
	}()
	<-entered
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/later", nil))
	assertGuardError(t, second, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
	if second.Header().Get("Retry-After") != "" {
		t.Fatal("saturation was reported as a rate retry")
	}
	unblock()
	<-finished
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d", first.Code)
	}
	third := httptest.NewRecorder()
	handler.ServeHTTP(third, httptest.NewRequest(http.MethodGet, "/later", nil))
	if third.Code != http.StatusOK {
		t.Fatalf("post-release status = %d", third.Code)
	}
}

func TestAPIPanicDiscardsPartialResponseAndLogDetails(t *testing.T) {
	var logs bytes.Buffer
	oldLogOutput := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(oldLogOutput) })
	guard := newAPIGuard(apiLimitConfig{
		rate: 1, burst: 2, active: 1, timeout: time.Second, now: time.Now,
	})
	handler := guard.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panic" {
			_, _ = io.WriteString(w, "ok")
			return
		}
		w.Header().Set("Set-Cookie", "secret=unsafe")
		_, _ = io.WriteString(w, "unsafe response bytes")
		panic("SQLITE private exception: secret=unsafe")
	}))
	failed := httptest.NewRecorder()
	handler.ServeHTTP(failed, httptest.NewRequest(http.MethodGet, "/panic", nil))
	assertGuardError(t, failed, http.StatusInternalServerError, "SERVICE_UNAVAILABLE")
	if failed.Header().Get("Set-Cookie") != "" || strings.Contains(failed.Body.String(), "unsafe") {
		t.Fatalf("panic leaked buffered headers or body: %q, %q", failed.Header().Get("Set-Cookie"), failed.Body.String())
	}
	if strings.Contains(logs.String(), "SQLITE") || strings.Contains(logs.String(), "secret") || !strings.Contains(logs.String(), "category=panic") {
		t.Fatalf("unsafe or missing panic diagnostic: %q", logs.String())
	}
	recovered := httptest.NewRecorder()
	handler.ServeHTTP(recovered, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if recovered.Code != http.StatusOK || recovered.Body.String() != "ok" {
		t.Fatalf("post-panic response = %d %q", recovered.Code, recovered.Body.String())
	}
}

func TestAPITimeoutDiscardsPartialResponseAndHoldsSlot(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	workerFinished := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	guard := newAPIGuard(apiLimitConfig{
		rate: 1, burst: 2, active: 1, timeout: 30 * time.Millisecond, now: time.Now,
	})
	handler := guard.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(workerFinished)
		w.Header().Set("Set-Cookie", "secret=unsafe")
		_, _ = io.WriteString(w, "SQLITE private data")
		close(entered)
		<-r.Context().Done()
		<-release
		_, _ = io.WriteString(w, "late data")
	}))
	failed := httptest.NewRecorder()
	handler.ServeHTTP(failed, httptest.NewRequest(http.MethodGet, "/wait", nil))
	<-entered
	assertGuardError(t, failed, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
	if failed.Header().Get("Set-Cookie") != "" || strings.Contains(failed.Body.String(), "SQLITE") {
		t.Fatalf("timeout leaked buffered response: %q, %q", failed.Header().Get("Set-Cookie"), failed.Body.String())
	}
	saturated := httptest.NewRecorder()
	handler.ServeHTTP(saturated, httptest.NewRequest(http.MethodGet, "/wait", nil))
	assertGuardError(t, saturated, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
	unblock()
	<-workerFinished
	if failed.Header().Get("Set-Cookie") != "" || strings.Contains(failed.Body.String(), "late data") {
		t.Fatalf("timed-out handler leaked late bytes: %q", failed.Body.String())
	}
}

func TestBoundedListenerReleasesConnectionsAndUnblocksClose(t *testing.T) {
	underlying, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := limitConnections(underlying, 1)
	t.Cleanup(func() { _ = listener.Close() })
	peer1, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer peer1.Close()
	conn1, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()
	peer2, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer peer2.Close()
	type acceptResult struct {
		conn net.Conn
		err  error
	}
	accepted := make(chan acceptResult, 1)
	go func() {
		conn, err := listener.Accept()
		accepted <- acceptResult{conn, err}
	}()
	select {
	case result := <-accepted:
		if result.conn != nil {
			_ = result.conn.Close()
		}
		t.Fatal("listener admitted a connection while capacity was occupied")
	case <-time.After(50 * time.Millisecond):
	}
	if err := conn1.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-accepted:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if err := result.conn.Close(); err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closing a connection did not release listener capacity")
	}
	conn3, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn3.Close()
	acceptedAgain, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	blocked := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if conn != nil {
			_ = conn.Close()
		}
		blocked <- err
	}()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-blocked:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("blocked Accept after Close = %v, want net.ErrClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("closing a full listener did not unblock Accept")
	}
	if err := acceptedAgain.Close(); err != nil {
		t.Fatal(err)
	}
}
