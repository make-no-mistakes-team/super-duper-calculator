package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
)

const (
	sessionCookie    = "calculator_session"
	semanticsVersion = "binary64-v1"
)

type api struct {
	db     *sql.DB
	engine calculation.Engine
}

func newHandler(db *sql.DB) http.Handler {
	a := api{db: db, engine: calculation.New()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", readiness(db))
	mux.HandleFunc("GET /api/session", a.session)
	mux.HandleFunc("POST /api/calculations", a.calculate)
	mux.HandleFunc("GET /api/history", a.history)
	if _, err := os.Stat("web/dist/index.html"); err == nil {
		mux.Handle("GET /", http.FileServer(http.Dir("web/dist")))
	}
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func apiError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, contracts.ErrorResponse{Error: contracts.APIError{Code: code, Params: map[string]any{}}})
}

func randomID(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a api) identity(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && len(cookie.Value) == 64 {
		var expires int64
		err := a.db.QueryRowContext(r.Context(), "SELECT expires_at FROM sessions WHERE id = ?", cookie.Value).Scan(&expires)
		if err == nil && expires > time.Now().Unix() {
			return cookie.Value, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	id, err := randomID(32)
	if err != nil {
		return "", err
	}
	const lifetime = 365 * 24 * time.Hour
	if _, err := a.db.ExecContext(r.Context(), "INSERT INTO sessions (id, expires_at) VALUES (?, ?)", id, time.Now().Add(lifetime).Unix()); err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: id, Path: "/", MaxAge: int(lifetime.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	return id, nil
}

func (a api) session(w http.ResponseWriter, r *http.Request) {
	if _, err := a.identity(w, r); err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"alias": "Гость"})
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
}

func decodeRequest(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return decoder.Decode(new(any))
}

func (a api) calculate(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		apiError(w, http.StatusForbidden, "INVALID_ORIGIN")
		return
	}
	var request contracts.CalculationRequest
	decodeErr := decodeRequest(w, r, &request)
	var sizeError *http.MaxBytesError
	if errors.As(decodeErr, &sizeError) {
		apiError(w, http.StatusRequestEntityTooLarge, "REQUEST_LIMIT")
		return
	}
	if !errors.Is(decodeErr, io.EOF) ||
		request.RequestID == "" || len(request.RequestID) > 128 ||
		(request.AngleUnit != contracts.Degrees && request.AngleUnit != contracts.Radians) || request.Room != nil {
		apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	owner, err := a.identity(w, r)
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	evaluation, err := a.engine.Evaluate(r.Context(), calculation.Input{Expression: request.Expression, AngleUnit: request.AngleUnit})
	if errors.Is(err, calculation.ErrExpressionLimit) {
		apiError(w, http.StatusRequestEntityTooLarge, "EXPRESSION_LIMIT")
		return
	}
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	id, err := randomID(16)
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	record := contracts.CalculationRecord{
		ID: id, RequestID: request.RequestID, Expression: request.Expression,
		Context: contracts.CalculationContext{AngleUnit: request.AngleUnit, SemanticsVersion: semanticsVersion},
		Outcome: evaluation.Outcome, Facts: evaluation.Facts, CreatedAt: time.Now().UTC(),
	}
	outcomeJSON, err := json.Marshal(record.Outcome)
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	factsJSON, err := json.Marshal(record.Facts)
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	result, err := a.db.ExecContext(r.Context(), `
		INSERT INTO calculations (id, session_id, request_id, expression, angle_unit, semantics_version, outcome_json, facts_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (session_id, request_id) DO NOTHING`,
		record.ID, owner, record.RequestID, record.Expression, record.Context.AngleUnit,
		record.Context.SemanticsVersion, string(outcomeJSON), string(factsJSON), record.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	if inserted == 0 {
		record, err = readRecord(a.db.QueryRowContext(r.Context(), `
			SELECT id, request_id, expression, angle_unit, semantics_version, outcome_json, facts_json, created_at
			FROM calculations WHERE session_id = ? AND request_id = ?`, owner, request.RequestID), nil)
		if err != nil {
			apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
			return
		}
		if record.Expression != request.Expression || record.Context.AngleUnit != request.AngleUnit {
			apiError(w, http.StatusConflict, "REQUEST_ID_CONFLICT")
			return
		}
	}
	writeJSON(w, http.StatusOK, contracts.CalculationResponse{
		Calculation: record, Publication: contracts.Publication{Status: "private"},
	})
}

type scanner interface{ Scan(...any) error }

func readRecord(row scanner, seq *int64) (contracts.CalculationRecord, error) {
	var record contracts.CalculationRecord
	var outcomeJSON, factsJSON, createdAt string
	values := []any{&record.ID, &record.RequestID, &record.Expression, &record.Context.AngleUnit,
		&record.Context.SemanticsVersion, &outcomeJSON, &factsJSON, &createdAt}
	if seq != nil {
		values = append([]any{seq}, values...)
	}
	err := row.Scan(values...)
	if err != nil {
		return record, err
	}
	if err := json.Unmarshal([]byte(outcomeJSON), &record.Outcome); err != nil {
		return record, err
	}
	if factsJSON != "null" {
		if err := json.Unmarshal([]byte(factsJSON), &record.Facts); err != nil {
			return record, err
		}
	}
	record.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	return record, err
}

func (a api) history(w http.ResponseWriter, r *http.Request) {
	owner, err := a.identity(w, r)
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
	}
	var before int64 = 1<<63 - 1
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
	}
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT seq, id, request_id, expression, angle_unit, semantics_version, outcome_json, facts_json, created_at
		FROM calculations WHERE session_id = ? AND seq < ? ORDER BY seq DESC LIMIT ?`, owner, before, limit+1)
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	defer rows.Close()
	page := contracts.HistoryPage{Items: []contracts.CalculationRecord{}}
	var lastSeq int64
	for rows.Next() {
		var seq int64
		record, err := readRecord(rows, &seq)
		if err != nil {
			apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
			return
		}
		if len(page.Items) == limit {
			cursor := strconv.FormatInt(lastSeq, 10)
			page.NextCursor = &cursor
			break
		}
		page.Items = append(page.Items, record)
		lastSeq = seq
	}
	if err := rows.Err(); err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	if err := rows.Close(); err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, page)
}
