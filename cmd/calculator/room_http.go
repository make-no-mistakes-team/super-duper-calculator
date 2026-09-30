package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"
)

func roomJSONRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return false
	}
	err = decodeRequest(w, r, dst)
	var sizeError *http.MaxBytesError
	if errors.As(err, &sizeError) {
		apiError(w, http.StatusRequestEntityTooLarge, "REQUEST_LIMIT")
		return false
	}
	if !errors.Is(err, io.EOF) {
		apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return false
	}
	return true
}

func (a api) roomOwner(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !a.sameOrigin(r) {
		apiError(w, http.StatusForbidden, "INVALID_ORIGIN")
		return "", false
	}
	owner, err := a.identity(r)
	if err != nil {
		identityError(w, err)
		return "", false
	}
	if !a.rooms.validCode(r.PathValue("code")) {
		apiError(w, http.StatusNotFound, "NOT_FOUND")
		return "", false
	}
	return owner, true
}

func (a api) joinRoom(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.roomOwner(w, r)
	if !ok {
		return
	}
	var input struct{}
	if !roomJSONRequest(w, r, &input) {
		return
	}
	joined, err := a.rooms.joinWithContext(r.Context(), owner)
	if err != nil {
		if errors.Is(err, errRoomViewLimit) {
			w.Header().Set("Retry-After", "1")
			apiError(w, http.StatusTooManyRequests, "RATE_LIMITED")
			return
		}
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, joined)
}

func (a api) leaveRoom(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.roomOwner(w, r)
	if !ok {
		return
	}
	var input struct {
		ViewID string `json:"viewId"`
	}
	if !roomJSONRequest(w, r, &input) {
		return
	}
	if len(input.ViewID) != 32 || !a.rooms.leave(owner, input.ViewID) {
		apiError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (a api) reactRoom(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.roomOwner(w, r)
	if !ok {
		return
	}
	var input struct {
		ReactionID json.RawMessage `json:"reactionId"`
	}
	if !roomJSONRequest(w, r, &input) {
		return
	}
	if input.ReactionID == nil || len(input.ReactionID) > 64 {
		apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	var selected *string
	if !bytes.Equal(input.ReactionID, []byte("null")) {
		var id string
		if err := json.Unmarshal(input.ReactionID, &id); err != nil {
			apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		selected = &id
	}
	if selected != nil {
		if _, ok := allowedRoomReactions[*selected]; !ok {
			apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
	}
	response, err := a.rooms.react(r.Context(), owner, r.PathValue("eventId"), selected)
	switch {
	case errors.Is(err, errRoomEventNotFound):
		apiError(w, http.StatusNotFound, "NOT_FOUND")
	case errors.Is(err, errRoomDisabled):
		apiError(w, http.StatusBadRequest, "UNSUPPORTED_CONTEXT")
	case errors.Is(err, errRoomReactionLimit), errors.Is(err, errRoomRateLimit):
		w.Header().Set("Retry-After", "1")
		apiError(w, http.StatusTooManyRequests, "RATE_LIMITED")
	case err != nil:
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
	default:
		writeJSON(w, http.StatusOK, response)
	}
}

func (a api) roomEvents(w http.ResponseWriter, r *http.Request) {
	if !a.sameOrigin(r) {
		apiError(w, http.StatusForbidden, "INVALID_ORIGIN")
		return
	}
	if !a.rooms.validCode(r.PathValue("code")) {
		apiError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	if !a.rooms.admitStream() {
		w.Header().Set("Retry-After", "1")
		apiError(w, http.StatusTooManyRequests, "RATE_LIMITED")
		return
	}
	select {
	case a.rooms.streams <- struct{}{}:
		defer func() { <-a.rooms.streams }()
	default:
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	authContext, cancelAuth := context.WithTimeout(r.Context(), 3*time.Second)
	owner, err := a.identity(r.WithContext(authContext))
	cancelAuth()
	if err != nil {
		identityError(w, err)
		return
	}
	viewID := r.URL.Query().Get("viewId")
	if len(viewID) != 32 {
		apiError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	lastID := r.Header.Get("Last-Event-ID")
	if lastID == "" {
		lastID = r.URL.Query().Get("cursor")
	}
	stream, initial, err := a.rooms.subscribe(owner, viewID, lastID)
	if err != nil {
		apiError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	defer a.rooms.disconnect(owner, viewID, stream)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	controller := http.NewResponseController(w)
	for _, message := range initial {
		if err := writeRoomMessage(w, controller, message); err != nil {
			return
		}
	}
	if len(initial) == 0 {
		if err := writeRoomHeartbeat(w, controller); err != nil {
			return
		}
	}
	ticker := time.NewTicker(roomHeartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-a.rooms.config.Shutdown:
			return
		case message, open := <-stream:
			if !open || writeRoomMessage(w, controller, message) != nil {
				return
			}
		case <-ticker.C:
			a.rooms.mu.Lock()
			a.rooms.expireViewsLocked()
			a.rooms.mu.Unlock()
			if err := writeRoomHeartbeat(w, controller); err != nil {
				return
			}
		}
	}
}

func writeRoomMessage(w http.ResponseWriter, controller *http.ResponseController, message roomMessage) error {
	if err := controller.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", message.id, message.kind, message.data); err != nil {
		return err
	}
	return controller.Flush()
}

func writeRoomHeartbeat(w http.ResponseWriter, controller *http.ResponseController) error {
	if err := controller.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
		return err
	}
	return controller.Flush()
}
