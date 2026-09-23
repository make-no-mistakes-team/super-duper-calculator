package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
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
	db           *sql.DB
	engine       calculation.Engine
	publicOrigin *url.URL
}

var errSessionRequired = errors.New("session required")

func newHandler(db *sql.DB, publicOrigin *url.URL) http.Handler {
	a := api{db: db, engine: calculation.New(), publicOrigin: publicOrigin}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", readiness(db))
	mux.HandleFunc("GET /health/version", versionInfo)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/session", a.session)
	apiMux.HandleFunc("GET /api/capabilities", capabilities)
	apiMux.HandleFunc("POST /api/calculations", a.calculate)
	apiMux.HandleFunc("GET /api/history", a.history)
	apiMux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		apiError(w, http.StatusNotFound, "NOT_FOUND")
	})
	guardedAPI := protectAPI(apiMux)
	mux.Handle("/api", guardedAPI)
	mux.Handle("/api/", guardedAPI)
	if _, err := os.Stat("web/dist/index.html"); err == nil {
		files := http.FileServer(http.Dir("web/dist"))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				http.NotFound(w, r)
				return
			}
			files.ServeHTTP(w, r)
		})
	}
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
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

func (a api) identity(r *http.Request) (string, error) {
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
	return "", errSessionRequired
}

func identityError(w http.ResponseWriter, err error) {
	if errors.Is(err, errSessionRequired) {
		apiError(w, http.StatusUnauthorized, "SESSION_REQUIRED")
		return
	}
	apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
}

func (a api) session(w http.ResponseWriter, r *http.Request) {
	if !a.sameOrigin(r) {
		apiError(w, http.StatusForbidden, "INVALID_ORIGIN")
		return
	}
	if _, err := a.identity(r); err == nil {
		writeJSON(w, http.StatusOK, map[string]string{"alias": "Гость"})
		return
	} else if !errors.Is(err, errSessionRequired) {
		identityError(w, err)
		return
	}
	id, err := randomID(32)
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	const lifetime = 365 * 24 * time.Hour
	if _, err := a.db.ExecContext(r.Context(), "INSERT INTO sessions (id, expires_at) VALUES (?, ?)", id, time.Now().Add(lifetime).Unix()); err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: id, Path: "/", MaxAge: int(lifetime.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil || (a.publicOrigin != nil && a.publicOrigin.Scheme == "https"),
	})
	writeJSON(w, http.StatusOK, map[string]string{"alias": "Гость"})
}

func decodeRequest(w http.ResponseWriter, r *http.Request, dst any) error {
	// Includes JSON escaping of the full 1,024 UTF-16-unit expression.
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return decoder.Decode(new(any))
}

func (a api) calculate(w http.ResponseWriter, r *http.Request) {
	if !a.sameOrigin(r) {
		apiError(w, http.StatusForbidden, "INVALID_ORIGIN")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	// Missing/null fields are malformed requests, unlike an explicitly empty
	// expression, which is an accepted mathematical syntax error.
	var input struct {
		RequestID  *string              `json:"requestId"`
		Expression *string              `json:"expression"`
		AngleUnit  *contracts.AngleUnit `json:"angleUnit"`
		Room       json.RawMessage      `json:"room"`
	}
	decodeErr := decodeRequest(w, r, &input)
	var sizeError *http.MaxBytesError
	if errors.As(decodeErr, &sizeError) {
		apiError(w, http.StatusRequestEntityTooLarge, "REQUEST_LIMIT")
		return
	}
	if !errors.Is(decodeErr, io.EOF) ||
		input.RequestID == nil || input.Expression == nil || input.AngleUnit == nil ||
		*input.RequestID == "" || len(*input.RequestID) > 128 ||
		(*input.AngleUnit != contracts.Degrees && *input.AngleUnit != contracts.Radians) {
		apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	request := contracts.CalculationRequest{RequestID: *input.RequestID, Expression: *input.Expression, AngleUnit: *input.AngleUnit}
	if input.Room != nil {
		var room struct {
			Code    *string `json:"code"`
			Publish *bool   `json:"publish"`
		}
		decoder := json.NewDecoder(bytes.NewReader(input.Room))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&room); err != nil || room.Code == nil || *room.Code == "" || room.Publish == nil {
			apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		request.Room = &contracts.RoomContext{Code: *room.Code, Publish: *room.Publish}
	}

	owner, err := a.identity(r)
	if err != nil {
		identityError(w, err)
		return
	}
	previous, err := a.recordByAction(r, owner, request.RequestID)
	switch {
	case err == nil:
		writeCalculation(w, previous, request)
		return
	case !errors.Is(err, sql.ErrNoRows):
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	if request.Room != nil {
		// Rooms are not enabled. Still detect changed context on a previously
		// accepted private action before rejecting a new unsupported action.
		apiError(w, http.StatusBadRequest, "UNSUPPORTED_CONTEXT")
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
		record, err = a.recordByAction(r, owner, request.RequestID)
		if err != nil {
			apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
			return
		}
	}
	writeCalculation(w, record, request)
}

func (a api) recordByAction(r *http.Request, owner, requestID string) (contracts.CalculationRecord, error) {
	return readRecord(a.db.QueryRowContext(r.Context(), `
		SELECT id, request_id, expression, angle_unit, semantics_version, outcome_json, facts_json, created_at
		FROM calculations WHERE session_id = ? AND request_id = ?`, owner, requestID), nil)
}

func writeCalculation(w http.ResponseWriter, record contracts.CalculationRecord, request contracts.CalculationRequest) {
	if record.Expression != request.Expression || record.Context.AngleUnit != request.AngleUnit || request.Room != nil {
		apiError(w, http.StatusConflict, "REQUEST_ID_CONFLICT")
		return
	}
	writeJSON(w, http.StatusOK, contracts.CalculationResponse{
		Calculation: record, Publication: contracts.Publication{Status: "private"},
	})
}

type scanner interface{ Scan(...any) error }

func readRecord(row scanner, seq *int64) (contracts.CalculationRecord, error) {
	var record contracts.CalculationRecord
	var outcomeJSON, createdAt string
	var factsJSON sql.NullString
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
	if factsJSON.Valid && factsJSON.String != "null" {
		if err := json.Unmarshal([]byte(factsJSON.String), &record.Facts); err != nil {
			return record, err
		}
	}
	record.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	return record, err
}

func (a api) history(w http.ResponseWriter, r *http.Request) {
	owner, err := a.identity(r)
	if err != nil {
		identityError(w, err)
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	limit := 50
	if raw := query.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
	}
	var before int64 = 1<<63 - 1
	if raw := query.Get("cursor"); raw != "" {
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		var owned bool
		if err := a.db.QueryRowContext(r.Context(),
			"SELECT EXISTS(SELECT 1 FROM calculations WHERE session_id = ? AND seq = ?)",
			owner, before).Scan(&owned); err != nil {
			apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
			return
		}
		if !owned {
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
