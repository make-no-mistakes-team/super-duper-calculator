package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

const sessionCookie = "calculator_session"

type api struct {
	db                *sql.DB
	engine            calculation.Engine
	publicOrigin      *url.URL
	statisticsEnabled bool
	discoveries       *discoveryService
	incidents         *incidentService
	rooms             *roomService
	actionLocks       *[256]sync.Mutex
}

var errSessionRequired = errors.New("session required")

func newHandler(db *sql.DB, publicOrigin *url.URL, statisticsEnabled, achievementsEnabled bool) http.Handler {
	handler, err := newHandlerWithRooms(db, publicOrigin, statisticsEnabled, achievementsEnabled, roomConfig{})
	if err != nil {
		panic(err)
	}
	return handler
}

func newHandlerWithRooms(db *sql.DB, publicOrigin *url.URL, statisticsEnabled, achievementsEnabled bool, config roomConfig) (http.Handler, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rooms, err := newRoomService(ctx, db, config)
	if err != nil {
		return nil, err
	}
	a := api{db: db, engine: calculation.New(), publicOrigin: publicOrigin, statisticsEnabled: statisticsEnabled,
		discoveries: newDiscoveryService(db, achievementsEnabled), incidents: newIncidentService(db, achievementsEnabled),
		rooms: rooms, actionLocks: new([256]sync.Mutex)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", readiness(db))
	mux.HandleFunc("GET /health/version", versionInfo)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/session", a.session)
	apiMux.HandleFunc("GET /api/capabilities", a.capabilities)
	apiMux.HandleFunc("POST /api/calculations", a.calculate)
	apiMux.HandleFunc("GET /api/history", a.history)
	apiMux.HandleFunc("GET /api/statistics", a.statistics)
	if rooms != nil {
		apiMux.HandleFunc("POST /api/rooms/{code}/join", a.joinRoom)
		apiMux.HandleFunc("POST /api/rooms/{code}/leave", a.leaveRoom)
		apiMux.HandleFunc("PUT /api/rooms/{code}/events/{eventId}/reaction", a.reactRoom)
		mux.HandleFunc("GET /api/rooms/{code}/events", a.roomEvents)
	}
	apiMux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		apiError(w, http.StatusNotFound, "NOT_FOUND")
	})
	guardedAPI := protectAPI(apiMux)
	mux.Handle("/api", guardedAPI)
	mux.Handle("/api/", guardedAPI)
	return mux, nil
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
	if owner, err := a.identity(r); err == nil {
		a.writeSession(w, r, owner)
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
	a.writeSession(w, r, id)
}

func (a api) writeSession(w http.ResponseWriter, r *http.Request, owner string) {
	identity := sha256.Sum256([]byte("calculator-public-identity:" + owner))
	response := contracts.SessionResponse{Alias: displayAlias(owner), Identity: hex.EncodeToString(identity[:])}
	if a.discoveries != nil {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		awards, catalog, err := a.discoveries.Collection(ctx, owner)
		cancel()
		if err == nil {
			response.Achievements = awards
			response.DiscoveryCatalog = catalog
			response.DiscoveriesAvailable = true
		}
	}
	writeJSON(w, http.StatusOK, response)
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
	key := sha256.Sum256([]byte(owner + ":" + request.RequestID))
	lock := &a.actionLocks[key[0]]
	lock.Lock()
	defer lock.Unlock()
	previous, err := a.recordByAction(r, owner, request.RequestID)
	switch {
	case err == nil:
		meta, err := a.actionMeta(r.Context(), owner, request.RequestID)
		if err != nil {
			apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
			return
		}
		if meta.status == "pending" {
			meta = a.finishUnavailable(owner, request.RequestID, meta)
		}
		writeCalculation(w, previous, request, meta, nil, nil)
		return
	case !errors.Is(err, sql.ErrNoRows):
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	if request.Room != nil {
		if a.rooms == nil {
			apiError(w, http.StatusBadRequest, "UNSUPPORTED_CONTEXT")
			return
		}
		if !a.rooms.validCode(request.Room.Code) {
			apiError(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
	}
	evaluation, err := a.engine.Evaluate(r.Context(), calculation.Input{Expression: request.Expression, AngleUnit: request.AngleUnit})
	if errors.Is(err, calculation.ErrExpressionLimit) {
		apiError(w, http.StatusRequestEntityTooLarge, string(contracts.ErrorExpressionLimit))
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
		Context: contracts.CalculationContext{AngleUnit: request.AngleUnit, SemanticsVersion: calculation.SemanticsVersion},
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
	meta := roomActionMeta{status: "private"}
	if request.Room != nil {
		meta.roomCode = sql.NullString{String: request.Room.Code, Valid: true}
		meta.publish = sql.NullBool{Bool: request.Room.Publish, Valid: true}
		if request.Room.Publish && record.Outcome.Kind == contracts.OutcomeSuccess {
			meta.status = "pending"
		}
	}
	result, err := a.db.ExecContext(r.Context(), `
		INSERT INTO calculations (id, session_id, request_id, expression, angle_unit, semantics_version,
			outcome_json, facts_json, created_at, room_code, room_publish, publication_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (session_id, request_id) DO NOTHING`,
		record.ID, owner, record.RequestID, record.Expression, record.Context.AngleUnit,
		record.Context.SemanticsVersion, string(outcomeJSON), string(factsJSON), record.CreatedAt.Format(time.RFC3339Nano),
		meta.roomCode, meta.publish, meta.status)
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
		meta, err = a.actionMeta(r.Context(), owner, request.RequestID)
		if err != nil {
			apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
			return
		}
	} else if meta.status == "pending" {
		publicID, publishErr := a.rooms.publish(r.Context(), owner, record)
		if publishErr != nil {
			meta = a.finishUnavailable(owner, request.RequestID, meta)
		} else {
			meta.status = "published"
			meta.eventID = sql.NullString{String: publicID, Valid: true}
		}
	}
	var achievements []contracts.Achievement
	var events []contracts.FunEvent
	if inserted != 0 && a.discoveries != nil {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		achievements, events, err = a.discoveries.Process(ctx, owner, record)
		cancel()
		if err != nil {
			achievements, events = nil, nil
		}
	}
	if inserted != 0 && a.incidents != nil {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		incidentEvents, incidentErr := a.incidents.Process(ctx, owner, record)
		cancel()
		if incidentErr == nil {
			events = append(events, incidentEvents...)
		}
	}
	writeCalculation(w, record, request, meta, achievements, events)
}

func (a api) recordByAction(r *http.Request, owner, requestID string) (contracts.CalculationRecord, error) {
	return storage.ReadCalculationRecord(a.db.QueryRowContext(r.Context(), `
		SELECT `+storage.CalculationRecordColumns+`
		FROM calculations WHERE session_id = ? AND request_id = ?`, owner, requestID), nil, storage.StrictFacts)
}

type roomActionMeta struct {
	roomCode sql.NullString
	publish  sql.NullBool
	status   string
	eventID  sql.NullString
}

func (a api) actionMeta(ctx context.Context, owner, requestID string) (roomActionMeta, error) {
	var meta roomActionMeta
	err := a.db.QueryRowContext(ctx, `SELECT room_code, room_publish, publication_status, public_event_id
		FROM calculations WHERE session_id = ? AND request_id = ?`, owner, requestID).
		Scan(&meta.roomCode, &meta.publish, &meta.status, &meta.eventID)
	return meta, err
}

func (a api) finishUnavailable(owner, requestID string, known roomActionMeta) roomActionMeta {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = a.db.ExecContext(ctx, `UPDATE calculations SET publication_status = 'unavailable'
		WHERE session_id = ? AND request_id = ? AND publication_status = 'pending'`, owner, requestID)
	meta, err := a.actionMeta(ctx, owner, requestID)
	if err == nil && meta.status != "pending" {
		return meta
	}
	known.status = "unavailable"
	return known
}

func writeCalculation(w http.ResponseWriter, record contracts.CalculationRecord, request contracts.CalculationRequest, meta roomActionMeta,
	achievements []contracts.Achievement, events []contracts.FunEvent) {
	contextMatches := request.Room == nil && !meta.roomCode.Valid && !meta.publish.Valid ||
		request.Room != nil && meta.roomCode.Valid && meta.publish.Valid &&
			meta.roomCode.String == request.Room.Code && meta.publish.Bool == request.Room.Publish
	if record.Expression != request.Expression || record.Context.AngleUnit != request.AngleUnit || !contextMatches {
		apiError(w, http.StatusConflict, "REQUEST_ID_CONFLICT")
		return
	}
	publication := contracts.Publication{Status: meta.status}
	if meta.eventID.Valid {
		publication.EventID = meta.eventID.String
	}
	writeJSON(w, http.StatusOK, contracts.CalculationResponse{
		Calculation: record, Publication: publication,
		Achievements: achievements, FunEvents: events,
	})
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
		SELECT `+storage.SequencedCalculationRecordColumns+`
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
		record, err := storage.ReadCalculationRecord(rows, &seq, storage.StrictFacts)
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
