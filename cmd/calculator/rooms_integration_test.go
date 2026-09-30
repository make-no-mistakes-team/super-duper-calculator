package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

type roomTestApp struct {
	t       *testing.T
	path    string
	db      *sql.DB
	handler http.Handler
	config  roomConfig
}

func newRoomTestApp(t *testing.T) *roomTestApp {
	t.Helper()
	a := &roomTestApp{t: t, path: filepath.Join(t.TempDir(), "private", "rooms.sqlite"), config: roomConfig{
		Enabled: true, Code: "demo", PublicationEnabled: true, ReactionsEnabled: true, EffectsEnabled: true,
	}}
	a.open()
	t.Cleanup(func() { _ = a.db.Close() })
	return a
}

func (a *roomTestApp) open() {
	a.t.Helper()
	var err error
	a.db, err = storage.Open(a.t.Context(), a.path)
	if err != nil {
		a.t.Fatal(err)
	}
	a.handler, err = newHandlerWithRooms(a.db, nil, true, false, a.config)
	if err != nil {
		a.t.Fatal(err)
	}
}

func (a *roomTestApp) restart() {
	a.t.Helper()
	if err := a.db.Close(); err != nil {
		a.t.Fatal(err)
	}
	a.open()
}

func (a *roomTestApp) send(cookie *http.Cookie, method, path string, payload any) *httptest.ResponseRecorder {
	a.t.Helper()
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			a.t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(a.t.Context())
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if payload != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	a.handler.ServeHTTP(w, r)
	return w
}

func roomDecode[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", response.Code, response.Body.String())
	}
	var result T
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func (a *roomTestApp) person() *http.Cookie {
	a.t.Helper()
	response := a.send(nil, http.MethodGet, "/api/session", nil)
	if response.Code != http.StatusOK || len(response.Result().Cookies()) != 1 {
		a.t.Fatalf("session: %d %s", response.Code, response.Body.String())
	}
	return response.Result().Cookies()[0]
}

func (a *roomTestApp) join(cookie *http.Cookie) contracts.RoomJoinResponse {
	a.t.Helper()
	return roomDecode[contracts.RoomJoinResponse](a.t, a.send(cookie, http.MethodPost, "/api/rooms/demo/join", struct{}{}))
}

func (a *roomTestApp) calculate(cookie *http.Cookie, id, expression string, room *contracts.RoomContext) contracts.CalculationResponse {
	a.t.Helper()
	return roomDecode[contracts.CalculationResponse](a.t, a.send(cookie, http.MethodPost, "/api/calculations", contracts.CalculationRequest{
		RequestID: id, Expression: expression, AngleUnit: contracts.Degrees, Room: room,
	}))
}

func (a *roomTestApp) reaction(cookie *http.Cookie, eventID string, selected any) contracts.RoomReactionResponse {
	a.t.Helper()
	return roomDecode[contracts.RoomReactionResponse](a.t, a.send(cookie, http.MethodPut,
		"/api/rooms/demo/events/"+eventID+"/reaction", map[string]any{"reactionId": selected}))
}

func TestRoomsExplicitPublicationAndPrivateHistory(t *testing.T) {
	a := newRoomTestApp(t)
	author, observer := a.person(), a.person()
	public := &contracts.RoomContext{Code: "demo", Publish: true}
	private := &contracts.RoomContext{Code: "demo", Publish: false}
	old := a.calculate(author, "before-join", "3+4", nil)
	a.join(author)
	if got := a.join(observer).Snapshot; len(got.Calculations) != 0 || got.Aggregates != (contracts.RoomAggregates{}) {
		t.Fatalf("join published history: %+v", got)
	}
	for _, test := range []struct {
		id, expression string
		room           *contracts.RoomContext
	}{
		{"personal-tab", "6*7", nil}, {"publication-off", "42", private}, {"math-error", "1/0", public},
	} {
		if got := a.calculate(author, test.id, test.expression, test.room); got.Publication.Status != "private" {
			t.Fatalf("%s: publication = %+v", test.id, got.Publication)
		}
	}
	if got := a.join(observer).Snapshot.Aggregates; got != (contracts.RoomAggregates{}) {
		t.Fatalf("private and failed calculations changed public aggregates: %+v", got)
	}
	published := a.calculate(author, "public", "2+3", public)
	if published.Publication.Status != "published" || published.Publication.EventID == "" {
		t.Fatalf("publication: %+v", published)
	}
	retry := a.calculate(author, "public", "2+3", public)
	if retry.Calculation.ID != published.Calculation.ID || retry.Publication != published.Publication {
		t.Fatalf("retry changed action: %+v", retry)
	}
	for _, changed := range []*contracts.RoomContext{nil, private, {Code: "other", Publish: true}} {
		response := a.send(author, http.MethodPost, "/api/calculations", contracts.CalculationRequest{
			RequestID: "public", Expression: "2+3", AngleUnit: contracts.Degrees, Room: changed,
		})
		if response.Code != http.StatusConflict {
			t.Errorf("changed room context returned %d: %s", response.Code, response.Body.String())
		}
	}
	snapshot := a.join(observer).Snapshot
	if len(snapshot.Calculations) != 1 || snapshot.Calculations[0].ID != published.Publication.EventID || snapshot.Calculations[0].Value != "5" {
		t.Fatalf("public feed: %+v", snapshot)
	}
	if snapshot.Aggregates != (contracts.RoomAggregates{PublishedCalculations: 1}) {
		t.Fatalf("public retry/conflicts changed aggregates: %+v", snapshot.Aggregates)
	}
	encoded, _ := json.Marshal(snapshot)
	for _, secret := range []string{author.Value, observer.Value, published.Calculation.ID, old.Calculation.ID, `"requestId"`, `"sessionId"`, `"calculationId"`} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("public snapshot leaked %q", secret)
		}
	}
	history := roomDecode[contracts.HistoryPage](t, a.send(author, http.MethodGet, "/api/history", nil))
	if len(history.Items) != 5 {
		t.Fatalf("personal history has %d records, want 5", len(history.Items))
	}
	if history := roomDecode[contracts.HistoryPage](t, a.send(observer, http.MethodGet, "/api/history", nil)); len(history.Items) != 0 {
		t.Fatalf("observer sees author history: %+v", history)
	}
}

func TestRoomsPresenceDeduplicatesTabsAndChecksOwnership(t *testing.T) {
	a := newRoomTestApp(t)
	first, second := a.person(), a.person()
	one, two := a.join(first), a.join(first)
	if one.ViewID == two.ViewID || two.Snapshot.Presence != 1 || one.Snapshot.Participant != two.Snapshot.Participant {
		t.Fatalf("tabs have wrong identity/presence: %+v %+v", one, two)
	}
	three := a.join(second)
	if three.Snapshot.Presence != 2 || three.Snapshot.Participant.ID == one.Snapshot.Participant.ID {
		t.Fatalf("independent identity: %+v", three)
	}
	leave := func(cookie *http.Cookie, view string) int {
		return a.send(cookie, http.MethodPost, "/api/rooms/demo/leave", map[string]string{"viewId": view}).Code
	}
	if status := leave(second, one.ViewID); status != http.StatusNotFound {
		t.Fatalf("foreign leave: %d", status)
	}
	if status := leave(first, one.ViewID); status != http.StatusNoContent {
		t.Fatalf("leave: %d", status)
	}
	if got := a.join(second).Snapshot.Presence; got != 2 {
		t.Fatalf("leaving one tab removed identity: %d", got)
	}
	if status := leave(first, two.ViewID); status != http.StatusNoContent {
		t.Fatalf("leave final tab: %d", status)
	}
	if got := a.join(second).Snapshot.Presence; got != 1 {
		t.Fatalf("final tab still present: %d", got)
	}
	if response := a.send(nil, http.MethodPost, "/api/rooms/demo/join", struct{}{}); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous join status: %d", response.Code)
	}
	if response := a.send(first, http.MethodPost, "/api/rooms/missing/join", struct{}{}); response.Code != http.StatusNotFound {
		t.Fatalf("unknown room: %d", response.Code)
	}
}

func TestRoomsConcurrentCalculationRetryPublishesOnce(t *testing.T) {
	a := newRoomTestApp(t)
	person := a.person()
	payload := contracts.CalculationRequest{RequestID: "concurrent", Expression: "6*7", AngleUnit: contracts.Degrees, Room: &contracts.RoomContext{Code: "demo", Publish: true}}
	responses := make(chan *httptest.ResponseRecorder, 12)
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() { responses <- a.send(person, http.MethodPost, "/api/calculations", payload) })
	}
	wg.Wait()
	close(responses)
	var first contracts.CalculationResponse
	for response := range responses {
		got := roomDecode[contracts.CalculationResponse](t, response)
		if first.Calculation.ID == "" {
			first = got
		}
		if got.Calculation.ID != first.Calculation.ID || got.Publication != first.Publication {
			t.Fatalf("concurrent replay differs: %+v / %+v", got, first)
		}
	}
	if snapshot := a.join(person).Snapshot; len(snapshot.Calculations) != 1 {
		t.Fatalf("duplicate public events: %+v", snapshot)
	}
	var records int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM calculations").Scan(&records); err != nil || records != 1 {
		t.Fatalf("records = %d, error = %v", records, err)
	}
}

func TestRoomsUnavailablePublicationNeverReplaysAfterRestart(t *testing.T) {
	a := newRoomTestApp(t)
	person := a.person()
	a.config.PublicationEnabled = false
	a.restart()
	if a.join(person).Snapshot.PublicationEnabled {
		t.Fatal("snapshot advertised disabled publication")
	}
	room := &contracts.RoomContext{Code: "demo", Publish: true}
	original := a.calculate(person, "unavailable", "17", room)
	if original.Publication.Status != "unavailable" || original.Calculation.Outcome.Value != "17" {
		t.Fatalf("disabled publication lost calculation: %+v", original)
	}
	a.config.PublicationEnabled = true
	a.restart()
	retry := a.calculate(person, "unavailable", "17", room)
	if retry.Calculation.ID != original.Calculation.ID || retry.Publication.Status != "unavailable" {
		t.Fatalf("retry after enabling: %+v", retry)
	}
	if snapshot := a.join(person).Snapshot; len(snapshot.Calculations) != 0 {
		t.Fatalf("unavailable action published later: %+v", snapshot)
	}
	// Simulate a crash after the private commit but before the publication attempt.
	if _, err := a.db.Exec("UPDATE calculations SET publication_status = 'pending' WHERE id = ?", original.Calculation.ID); err != nil {
		t.Fatal(err)
	}
	a.restart()
	if got := a.calculate(person, "unavailable", "17", room); got.Publication.Status != "unavailable" {
		t.Fatalf("interrupted action replayed: %+v", got)
	}
}

func TestRoomsReactionsAndPeerReviewedPersist(t *testing.T) {
	a := newRoomTestApp(t)
	author, first, second, third := a.person(), a.person(), a.person(), a.person()
	event := a.calculate(author, "review-target", "8*8", &contracts.RoomContext{Code: "demo", Publish: true}).Publication.EventID
	assertAggregates := func(active int) {
		t.Helper()
		if got := a.join(author).Snapshot.Aggregates; got != (contracts.RoomAggregates{PublishedCalculations: 1, ActiveReactions: active}) {
			t.Fatalf("aggregates = %+v, want 1 published and %d active reactions", got, active)
		}
	}
	assertAggregates(0)
	own := a.reaction(author, event, "applause")
	assertAggregates(1)
	one := a.reaction(first, event, "wow")
	repeated := a.reaction(first, event, "wow")
	if one.Sequence != repeated.Sequence || repeated.Reactions["wow"] != 1 {
		t.Fatalf("reaction retry changed count/sequence: %+v %+v", one, repeated)
	}
	assertAggregates(2)
	switched := a.reaction(first, event, "laugh")
	assertAggregates(2)
	if switched.Reactions["wow"] != 0 || switched.Reactions["laugh"] != 1 || switched.Reactions["applause"] != own.Reactions["applause"] {
		t.Fatalf("reaction replacement: %+v", switched)
	}
	two := a.reaction(second, event, "thinking")
	assertAggregates(3)
	if len(two.AchievementIDs) != 0 {
		t.Fatalf("author counted as peer: %+v", two)
	}
	three := a.reaction(third, event, "applause")
	assertAggregates(4)
	if len(three.AchievementIDs) != 1 || three.AchievementIDs[0] != "peer_reviewed" {
		t.Fatalf("three peers did not award badge: %+v", three)
	}
	removed := a.reaction(third, event, nil)
	assertAggregates(3)
	if removed.ReactionID != nil || removed.Reactions["applause"] != 1 || len(removed.AchievementIDs) != 1 {
		t.Fatalf("removal revoked badge/count wrong: %+v", removed)
	}
	for _, payload := range []any{map[string]any{}, map[string]any{"reactionId": "unknown"}, map[string]any{"reactionId": 1}} {
		if response := a.send(first, http.MethodPut, "/api/rooms/demo/events/"+event+"/reaction", payload); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid reaction: %d %s", response.Code, response.Body.String())
		}
	}
	a.restart()
	snapshot := a.join(first).Snapshot
	if len(snapshot.Calculations) != 1 || len(snapshot.Calculations[0].AchievementIDs) != 1 || snapshot.Calculations[0].Reactions["laugh"] != 1 {
		t.Fatalf("restart lost reaction/badge: %+v", snapshot)
	}
	if snapshot.Aggregates != (contracts.RoomAggregates{PublishedCalculations: 1, ActiveReactions: 3}) {
		t.Fatalf("restart changed aggregates: %+v", snapshot.Aggregates)
	}
	if repeated := a.reaction(first, event, "laugh"); repeated.Sequence != snapshot.Sequence {
		t.Fatalf("persisted reaction was counted again: %+v", repeated)
	}
}

type roomSSEFrame struct{ id, kind, data string }

func roomOpenSSE(t *testing.T, server *httptest.Server, person *http.Cookie, view, cursor string) (*http.Response, *bufio.Scanner) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/api/rooms/demo/events?viewId="+view, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(person)
	if cursor != "" {
		request.Header.Set("Last-Event-ID", cursor)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("SSE status %d: %s", response.StatusCode, body)
	}
	if response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("SSE content type: %s", response.Header.Get("Content-Type"))
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	return response, scanner
}

func roomReadSSE(t *testing.T, scanner *bufio.Scanner) roomSSEFrame {
	t.Helper()
	var frame roomSSEFrame
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if frame.kind != "" {
				return frame
			}
			continue
		}
		if value, ok := strings.CutPrefix(line, "id: "); ok {
			frame.id = value
		}
		if value, ok := strings.CutPrefix(line, "event: "); ok {
			frame.kind = value
		}
		if value, ok := strings.CutPrefix(line, "data: "); ok {
			frame.data = value
		}
	}
	t.Fatalf("SSE ended before event: %v", scanner.Err())
	return frame
}

func TestRoomsSSEReplayAndRestartSnapshot(t *testing.T) {
	a := newRoomTestApp(t)
	person := a.person()
	joined := a.join(person)
	server := httptest.NewServer(a.handler)
	defer server.Close()
	stream, scanner := roomOpenSSE(t, server, person, joined.ViewID, "")
	initial := roomReadSSE(t, scanner)
	if initial.kind != "snapshot" {
		t.Fatalf("first SSE event = %+v", initial)
	}
	_ = stream.Body.Close()
	published := a.calculate(person, "offline", "9*9", &contracts.RoomContext{Code: "demo", Publish: true})
	stream, scanner = roomOpenSSE(t, server, person, joined.ViewID, initial.id)
	replayed := roomReadSSE(t, scanner)
	if replayed.kind != "calculation" || !strings.Contains(replayed.data, published.Publication.EventID) {
		t.Fatalf("replay failed: %+v", replayed)
	}
	_ = stream.Body.Close()
	server.Close()
	a.restart()
	joined = a.join(person)
	server = httptest.NewServer(a.handler)
	defer server.Close()
	stream, scanner = roomOpenSSE(t, server, person, joined.ViewID, replayed.id)
	reset := roomReadSSE(t, scanner)
	if reset.kind != "snapshot" || strings.HasPrefix(reset.id, strings.Split(initial.id, ":")[0]+":") || !strings.Contains(reset.data, published.Publication.EventID) {
		t.Fatalf("restart did not reset cursor/retain feed: %+v", reset)
	}
	_ = stream.Body.Close()
}

func TestRoomsSharedAnswerNeedsThreePublishedIdentities(t *testing.T) {
	a := newRoomTestApp(t)
	first, second, third, fourth := a.person(), a.person(), a.person(), a.person()
	public := &contracts.RoomContext{Code: "demo", Publish: true}
	private := &contracts.RoomContext{Code: "demo", Publish: false}
	a.calculate(first, "a", "42", public)
	a.calculate(first, "a", "42", public)
	a.calculate(first, "b", "6*7", public)
	a.calculate(second, "private", "42", private)
	a.calculate(third, "personal", "42", nil)
	a.calculate(second, "public", "42", public)
	var scenes int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM room_effect_state").Scan(&scenes); err != nil || scenes != 0 {
		t.Fatalf("early shared scene: count=%d error=%v", scenes, err)
	}
	joined := a.join(first)
	server := httptest.NewServer(a.handler)
	defer server.Close()
	stream, scanner := roomOpenSSE(t, server, first, joined.ViewID, "")
	defer stream.Body.Close()
	_ = roomReadSSE(t, scanner)
	a.calculate(third, "public", "40+2", public)
	frame := roomReadSSE(t, scanner)
	var payload struct {
		FunEvents []contracts.FunEvent `json:"funEvents"`
	}
	if err := json.Unmarshal([]byte(frame.data), &payload); err != nil {
		t.Fatal(err)
	}
	if frame.kind != "calculation" || len(payload.FunEvents) != 1 || payload.FunEvents[0].RuleID != "shared_answer" || payload.FunEvents[0].ID == "" || !payload.FunEvents[0].ExpiresAt.After(payload.FunEvents[0].CreatedAt) {
		t.Fatalf("third identity event: %+v %s", frame, frame.data)
	}
	a.calculate(fourth, "public", "42", public)
	frame = roomReadSSE(t, scanner)
	if err := json.Unmarshal([]byte(frame.data), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.FunEvents) != 0 {
		t.Fatalf("cooldown did not suppress repeated scene: %s", frame.data)
	}
	// SQL state is keyed by identity; retries and repeated 42s from one person do not add voters.
	var identities int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM room_answer_42").Scan(&identities); err != nil || identities != 4 {
		t.Fatalf("42 identities=%d error=%v", identities, err)
	}
}

func TestRoomsSSERejectsForeignView(t *testing.T) {
	a := newRoomTestApp(t)
	first, second := a.person(), a.person()
	joined := a.join(first)
	response := a.send(second, http.MethodGet, fmt.Sprintf("/api/rooms/demo/events?viewId=%s", joined.ViewID), nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("foreign view status %d: %s", response.Code, response.Body.String())
	}
}

func TestRoomsFeedEvictionKeepsPersonalHistoryAndDeletesReactions(t *testing.T) {
	a := newRoomTestApp(t)
	author, observer := a.person(), a.person()
	public := &contracts.RoomContext{Code: "demo", Publish: true}
	first := a.calculate(author, "entry-0", "1000", public)
	a.reaction(observer, first.Publication.EventID, "wow")
	if got := a.join(observer).Snapshot.Aggregates; got != (contracts.RoomAggregates{PublishedCalculations: 1, ActiveReactions: 1}) {
		t.Fatalf("pre-eviction aggregates: %+v", got)
	}
	for i := 1; i <= roomFeedLimit; i++ {
		// Reopening also checks that eviction uses durable order, across epochs.
		// Each process receives fewer than the configured HTTP burst budget.
		if i%40 == 0 {
			a.restart()
		}
		a.calculate(author, fmt.Sprintf("entry-%d", i), fmt.Sprint(1000+i), public)
	}
	snapshot := a.join(observer).Snapshot
	if len(snapshot.Calculations) != roomFeedLimit || snapshot.Calculations[0].Expression != "1001" || snapshot.Calculations[roomFeedLimit-1].Expression != "1100" {
		t.Fatalf("retained feed does not contain last 100: %+v", snapshot.Calculations)
	}
	if snapshot.Aggregates != (contracts.RoomAggregates{PublishedCalculations: 101, ActiveReactions: 0}) {
		t.Fatalf("eviction changed published count or retained active reaction: %+v", snapshot.Aggregates)
	}
	response := a.send(observer, http.MethodPut, "/api/rooms/demo/events/"+first.Publication.EventID+"/reaction", map[string]string{"reactionId": "laugh"})
	if response.Code != http.StatusNotFound {
		t.Fatalf("reaction on evicted event: %d %s", response.Code, response.Body.String())
	}
	var privateRecords, publicRecords, reactionRecords int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM calculations").Scan(&privateRecords); err != nil {
		t.Fatal(err)
	}
	if err := a.db.QueryRow("SELECT COUNT(*) FROM room_events").Scan(&publicRecords); err != nil {
		t.Fatal(err)
	}
	if err := a.db.QueryRow("SELECT COUNT(*) FROM room_reactions").Scan(&reactionRecords); err != nil {
		t.Fatal(err)
	}
	if privateRecords != 101 || publicRecords != 100 || reactionRecords != 0 {
		t.Fatalf("retention: personal=%d public=%d reactions=%d", privateRecords, publicRecords, reactionRecords)
	}
	retry := a.calculate(author, "entry-0", "1000", public)
	snapshot = a.join(observer).Snapshot
	if snapshot.Aggregates != (contracts.RoomAggregates{PublishedCalculations: 101, ActiveReactions: 0}) {
		t.Fatalf("evicted action retry changed aggregates: %+v", snapshot.Aggregates)
	}
	if retry.Publication != first.Publication || len(snapshot.Calculations) != 100 {
		t.Fatalf("retry republished evicted action: %+v", retry)
	}
	a.restart()
	if got := a.join(observer).Snapshot.Aggregates; got != snapshot.Aggregates {
		t.Fatalf("restart after eviction changed aggregates: %+v, want %+v", got, snapshot.Aggregates)
	}
}

func TestRoomsPublicStorageFailurePreservesCalculation(t *testing.T) {
	a := newRoomTestApp(t)
	person := a.person()
	if _, err := a.db.Exec(`CREATE TRIGGER fail_room_publication BEFORE INSERT ON room_events BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	public := &contracts.RoomContext{Code: "demo", Publish: true}
	result := a.calculate(person, "database-failure", "20+22", public)
	if result.Publication.Status != "unavailable" || result.Calculation.Outcome.Value != "42" {
		t.Fatalf("publication fault lost core result: %+v", result)
	}
	if _, err := a.db.Exec("DROP TRIGGER fail_room_publication"); err != nil {
		t.Fatal(err)
	}
	retry := a.calculate(person, "database-failure", "20+22", public)
	if retry.Calculation.ID != result.Calculation.ID || retry.Publication.Status != "unavailable" || len(a.join(person).Snapshot.Calculations) != 0 {
		t.Fatalf("storage failure retry published retroactively: %+v", retry)
	}
	history := roomDecode[contracts.HistoryPage](t, a.send(person, http.MethodGet, "/api/history", nil))
	if len(history.Items) != 1 || history.Items[0].ID != result.Calculation.ID {
		t.Fatalf("private commit was lost: %+v", history)
	}
}

func TestRoomsExpiredViewsAndReplayWindow(t *testing.T) {
	a := newRoomTestApp(t)
	person := a.person()
	service, err := newRoomService(t.Context(), a.db, a.config)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	joined, err := service.join(person.Value)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(roomViewTTL)
	if _, _, err := service.subscribe(person.Value, joined.ViewID, ""); err != errRoomViewNotFound {
		t.Fatalf("expired view subscribed: %v", err)
	}
	joined, err = service.join(person.Value)
	if err != nil {
		t.Fatal(err)
	}
	cursor := fmt.Sprintf("%s:%d", joined.Snapshot.Epoch, joined.Snapshot.Sequence)
	service.mu.Lock()
	for range roomJournalLimit + 1 {
		service.broadcastLocked("presence", map[string]int{"participants": 1})
	}
	service.mu.Unlock()
	stream, messages, err := service.subscribe(person.Value, joined.ViewID, cursor)
	if err != nil {
		t.Fatal(err)
	}
	defer service.disconnect(person.Value, joined.ViewID, stream)
	if len(messages) != 1 || messages[0].kind != "snapshot" {
		t.Fatalf("expired replay cursor did not get snapshot: %+v", messages)
	}
}

func TestRoomsReplayDoesNotReannounceExpiredEffects(t *testing.T) {
	a := newRoomTestApp(t)
	person := a.person()
	service, err := newRoomService(t.Context(), a.db, a.config)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	joined, err := service.join(person.Value)
	if err != nil {
		t.Fatal(err)
	}
	cursor := fmt.Sprintf("%s:%d", joined.Snapshot.Epoch, joined.Snapshot.Sequence)
	service.mu.Lock()
	service.broadcastLocked("calculation", map[string]any{
		"calculation": map[string]string{"id": "retained-calculation"},
		"funEvents":   []contracts.FunEvent{{ID: "expired-scene", RuleID: "shared_answer", Kind: "scene", Scope: "room", CreatedAt: now, ExpiresAt: now.Add(10 * time.Second)}},
	})
	service.mu.Unlock()
	now = now.Add(11 * time.Second)
	stream, messages, err := service.subscribe(person.Value, joined.ViewID, cursor)
	if err != nil {
		t.Fatal(err)
	}
	defer service.disconnect(person.Value, joined.ViewID, stream)
	if len(messages) != 1 || messages[0].kind != "calculation" {
		t.Fatalf("retained calculation missing from replay: %+v", messages)
	}
	var payload struct {
		FunEvents []contracts.FunEvent `json:"funEvents"`
	}
	if err := json.Unmarshal(messages[0].data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.FunEvents) != 0 {
		t.Fatalf("replay reannounced expired effects: %s", messages[0].data)
	}
}

func TestRoomsUnavailableFallbackKeepsOriginalContext(t *testing.T) {
	a := newRoomTestApp(t)
	owner := a.person()
	known := roomActionMeta{
		roomCode: sql.NullString{String: "demo", Valid: true},
		publish:  sql.NullBool{Bool: true, Valid: true},
		status:   "pending",
	}
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	meta := (api{db: a.db}).finishUnavailable(owner.Value, "lost-response", known)
	if meta.status != "unavailable" || meta.roomCode != known.roomCode || meta.publish != known.publish {
		t.Fatalf("database failure lost action context: %+v", meta)
	}
	w := httptest.NewRecorder()
	writeCalculation(w, contracts.CalculationRecord{
		Expression: "6*7", Context: contracts.CalculationContext{AngleUnit: contracts.Degrees},
	}, contracts.CalculationRequest{
		Expression: "6*7", AngleUnit: contracts.Degrees,
		Room: &contracts.RoomContext{Code: "demo", Publish: true},
	}, meta, nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("saved calculation became HTTP %d: %s", w.Code, w.Body.String())
	}
}

func TestRoomsGracefulShutdownClosesActiveSSE(t *testing.T) {
	a := newRoomTestApp(t)
	person := a.person()
	stop := make(chan struct{})
	config := a.config
	config.Shutdown = stop
	handler, err := newHandlerWithRooms(a.db, nil, true, false, config)
	if err != nil {
		t.Fatal(err)
	}
	joined := roomDecode[contracts.RoomJoinResponse](t, func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/rooms/demo/join", strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(person)
		handler.ServeHTTP(w, r)
		return w
	}())
	server := httptest.NewServer(handler)
	defer server.Close()
	response, scanner := roomOpenSSE(t, server, person, joined.ViewID, "")
	if frame := roomReadSSE(t, scanner); frame.kind != "snapshot" {
		t.Fatalf("initial frame: %+v", frame)
	}
	close(stop)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Config.Shutdown(ctx); err != nil {
		t.Fatalf("active SSE prevented graceful shutdown: %v", err)
	}
	_ = response.Body.Close()
}
