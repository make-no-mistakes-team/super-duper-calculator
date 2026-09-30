package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

func assertRoomEnglishName(t *testing.T, participant contracts.RoomParticipant) {
	t.Helper()
	if participant.ID == "" || !regexp.MustCompile(`^[A-Z][a-z]+ [A-Z][a-z]+$`).MatchString(participant.Alias) {
		t.Fatalf("expected public identity with English adjective/noun alias, got %+v", participant)
	}
}

func TestRoomIdentityNamesSurviveTabsRestartAndAttributeOwnFeed(t *testing.T) {
	a := newRoomTestApp(t)
	author, observer := a.person(), a.person()
	private := a.calculate(author, "identity-private", "12345+6789", nil)
	// Publication without visiting the room must assign the same durable name as join.
	public := a.calculate(author, "identity-public", "3*7", &contracts.RoomContext{Code: "demo", Publish: true})
	first, second := a.join(author), a.join(author)
	assertRoomEnglishName(t, first.Snapshot.Participant)
	if first.ViewID == second.ViewID || first.Snapshot.Participant != second.Snapshot.Participant {
		t.Fatalf("separate tabs changed identity: %+v / %+v", first, second)
	}
	other := a.join(observer).Snapshot.Participant
	assertRoomEnglishName(t, other)
	if other.ID == first.Snapshot.Participant.ID || other.Alias == first.Snapshot.Participant.Alias {
		t.Fatalf("independent participants share identity or alias: %+v / %+v", first.Snapshot.Participant, other)
	}
	a.calculate(observer, "observer-public", "4*8", &contracts.RoomContext{Code: "demo", Publish: true})
	a.restart()
	after := a.join(author).Snapshot
	if after.Participant != first.Snapshot.Participant || a.join(observer).Snapshot.Participant != other {
		t.Fatal("restart changed a participant's public identity")
	}
	if len(after.Calculations) != 2 {
		t.Fatalf("public feed contains %d calculations, want 2", len(after.Calculations))
	}
	mine := 0
	for _, calculation := range after.Calculations {
		if calculation.Participant.ID == after.Participant.ID {
			mine++
			if calculation.ID != public.Publication.EventID || calculation.Participant != after.Participant {
				t.Fatalf("own-feed comparison or persisted attribution is inconsistent: %+v", calculation)
			}
		} else if calculation.Participant != other {
			t.Fatalf("foreign calculation has wrong author: %+v", calculation)
		}
	}
	if mine != 1 {
		t.Fatalf("filtering by self participant ID selected %d calculations, want 1", mine)
	}
	encoded, err := json.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{author.Value, observer.Value, private.Calculation.ID, public.Calculation.ID, "12345+6789"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("public identity projection leaked private value %q", secret)
		}
	}
	history := roomDecode[contracts.HistoryPage](t, a.send(observer, http.MethodGet, "/api/history", nil))
	if len(history.Items) != 1 || history.Items[0].Expression != "4*8" {
		t.Fatalf("observer's private history includes another participant: %+v", history)
	}
}

func TestRoomIdentityNamesAreUniqueForIndependentSessions(t *testing.T) {
	a := newRoomTestApp(t)
	aliases := make(map[string]string)
	for i := range 16 {
		person := a.person()
		participant := a.join(person).Snapshot.Participant
		assertRoomEnglishName(t, participant)
		if existing, exists := aliases[participant.Alias]; exists {
			t.Fatalf("participants %s and %s received duplicate alias %q", existing, participant.ID, participant.Alias)
		}
		aliases[participant.Alias] = participant.ID
		if got := a.join(person).Snapshot.Participant; got != participant {
			t.Fatalf("participant %d changed on rejoin: %+v / %+v", i, participant, got)
		}
	}
}

func TestRoomIdentityReactionAndPeerReviewedAttributionOverSSE(t *testing.T) {
	a := newRoomTestApp(t)
	author := a.person()
	joined := a.join(author)
	event := a.calculate(author, "named-review", "8*8", &contracts.RoomContext{Code: "demo", Publish: true}).Publication.EventID
	server := httptest.NewServer(a.handler)
	defer server.Close()
	stream, scanner := roomOpenSSE(t, server, author, joined.ViewID, "")
	defer stream.Body.Close()
	initial := roomReadSSE(t, scanner)
	if initial.kind != "snapshot" {
		t.Fatalf("initial SSE event = %+v", initial)
	}
	var lastParticipant contracts.RoomParticipant
	for i := range 3 {
		peer := a.person()
		// Reactions also assign names before the first join.
		a.reaction(peer, event, "applause")
		frame := roomReadSSE(t, scanner)
		var delta struct {
			ParticipantID string                    `json:"participantId"`
			Participant   contracts.RoomParticipant `json:"participant"`
			FunEvents     []contracts.FunEvent      `json:"funEvents"`
		}
		if err := json.Unmarshal([]byte(frame.data), &delta); err != nil {
			t.Fatal(err)
		}
		if frame.kind != "reaction" {
			t.Fatalf("reaction event = %+v", frame)
		}
		assertRoomEnglishName(t, delta.Participant)
		if delta.ParticipantID != delta.Participant.ID || delta.Participant.ID == joined.Snapshot.Participant.ID {
			t.Fatalf("reaction actor does not identify peer: %s", frame.data)
		}
		if got := a.join(peer).Snapshot.Participant; got != delta.Participant {
			t.Fatalf("reaction-before-join identity changed: %+v / %+v", delta.Participant, got)
		}
		// Joining produces a presence frame for the author's live stream.
		if presence := roomReadSSE(t, scanner); presence.kind != "presence" {
			t.Fatalf("expected peer presence after join, got %+v", presence)
		}
		if i < 2 && len(delta.FunEvents) != 0 {
			t.Fatalf("peer_reviewed announced before three peers: %s", frame.data)
		}
		if i == 2 {
			if len(delta.FunEvents) != 1 || delta.FunEvents[0].RuleID != "peer_reviewed" {
				t.Fatalf("missing named peer_reviewed event: %s", frame.data)
			}
			params := delta.FunEvents[0].Params
			assertRoomIdentityParam(t, params, "author", joined.Snapshot.Participant)
			assertRoomIdentityParam(t, params, "triggeredBy", delta.Participant)
			if params["eventId"] != event {
				t.Fatalf("peer_reviewed points to wrong calculation: %+v", params)
			}
		}
		lastParticipant = delta.Participant
	}
	_ = stream.Body.Close()
	// Replay includes the same public identity, so reconnect cannot change the displayed actor.
	stream, scanner = roomOpenSSE(t, server, author, joined.ViewID, initial.id)
	defer stream.Body.Close()
	for i := range 3 {
		frame := roomReadSSE(t, scanner)
		var delta struct {
			Participant contracts.RoomParticipant `json:"participant"`
		}
		if err := json.Unmarshal([]byte(frame.data), &delta); err != nil {
			t.Fatal(err)
		}
		if frame.kind != "reaction" {
			t.Fatalf("replayed frame = %+v", frame)
		}
		if i == 2 && delta.Participant != lastParticipant {
			t.Fatalf("replayed actor changed: %+v / %+v", delta.Participant, lastParticipant)
		}
		if presence := roomReadSSE(t, scanner); presence.kind != "presence" {
			t.Fatalf("replayed presence = %+v", presence)
		}
	}
}

func TestRoomIdentitySharedAnswerNamesTriggerAndDistinctContributors(t *testing.T) {
	a := newRoomTestApp(t)
	people := []*http.Cookie{a.person(), a.person(), a.person()}
	participants := make(map[string]contracts.RoomParticipant)
	var joined contracts.RoomJoinResponse
	for _, person := range people {
		joined = a.join(person)
		participants[joined.Snapshot.Participant.ID] = joined.Snapshot.Participant
	}
	server := httptest.NewServer(a.handler)
	defer server.Close()
	stream, scanner := roomOpenSSE(t, server, people[2], joined.ViewID, "")
	defer stream.Body.Close()
	_ = roomReadSSE(t, scanner)
	public := &contracts.RoomContext{Code: "demo", Publish: true}
	for i, person := range people {
		a.calculate(person, fmt.Sprintf("named-42-%d", i), "6*7", public)
		frame := roomReadSSE(t, scanner)
		var delta struct {
			FunEvents []contracts.FunEvent `json:"funEvents"`
		}
		if err := json.Unmarshal([]byte(frame.data), &delta); err != nil {
			t.Fatal(err)
		}
		if frame.kind != "calculation" {
			t.Fatalf("calculation frame = %+v", frame)
		}
		if i < 2 {
			if len(delta.FunEvents) != 0 {
				t.Fatalf("shared answer triggered before three contributors: %s", frame.data)
			}
			continue
		}
		if len(delta.FunEvents) != 1 || delta.FunEvents[0].RuleID != "shared_answer" {
			t.Fatalf("third contribution has no shared_answer: %s", frame.data)
		}
		params := delta.FunEvents[0].Params
		assertRoomIdentityParam(t, params, "triggeredBy", joined.Snapshot.Participant)
		encoded, err := json.Marshal(params["contributors"])
		if err != nil {
			t.Fatal(err)
		}
		var contributors []contracts.RoomParticipant
		if err := json.Unmarshal(encoded, &contributors); err != nil {
			t.Fatal(err)
		}
		if len(contributors) != 3 {
			t.Fatalf("expected three named contributors: %+v", params)
		}
		for _, contributor := range contributors {
			if want, found := participants[contributor.ID]; !found || contributor != want {
				t.Fatalf("unknown, duplicate or renamed contributor: %+v", contributor)
			}
			delete(participants, contributor.ID)
		}
		if len(participants) != 0 {
			t.Fatalf("missing contributors: %+v", participants)
		}
	}
}

func assertRoomIdentityParam(t *testing.T, params map[string]any, key string, want contracts.RoomParticipant) {
	t.Helper()
	encoded, err := json.Marshal(params[key])
	if err != nil {
		t.Fatal(err)
	}
	var got contracts.RoomParticipant
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s = %+v, want %+v", key, got, want)
	}
}

func TestRoomIdentitySharedAnswerAfterCooldownKeepsExactlyThreeContributors(t *testing.T) {
	a := newRoomTestApp(t)
	service, err := newRoomService(t.Context(), a.db, a.config)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	people := make([]*http.Cookie, 5)
	participants := make([]contracts.RoomParticipant, len(people))
	byID := make(map[string]contracts.RoomParticipant)
	for i := range people {
		people[i] = a.person()
		participants[i] = a.join(people[i]).Snapshot.Participant
		byID[participants[i].ID] = participants[i]
	}
	// Exercise the persisted rule transaction with a controlled clock, without
	// waiting two minutes or changing the clock of a live HTTP server.
	contribute := func(index int) []contracts.FunEvent {
		t.Helper()
		tx, err := a.db.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		events, err := service.claimSharedAnswer(t.Context(), tx, people[index].Value, participants[index], service.now())
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		return events
	}
	for i := range 3 {
		events := contribute(i)
		if i < 2 && len(events) != 0 || i == 2 && len(events) != 1 {
			t.Fatalf("initial threshold at contributor %d: %+v", i, events)
		}
	}
	now = now.Add(119 * time.Second)
	for i := range people {
		if events := contribute(i); len(events) != 0 {
			t.Fatalf("cooldown allowed an early announcement: %+v", events)
		}
	}
	var eligible int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM room_answer_42 WHERE room_code = ?", a.config.Code).Scan(&eligible); err != nil {
		t.Fatal(err)
	}
	if eligible != 5 {
		t.Fatalf("test requires five eligible participants, got %d", eligible)
	}
	now = now.Add(time.Second)
	events := contribute(4)
	if len(events) != 1 || events[0].RuleID != "shared_answer" {
		t.Fatalf("cooldown boundary did not emit shared_answer: %+v", events)
	}
	assertRoomIdentityParam(t, events[0].Params, "triggeredBy", participants[4])
	encoded, err := json.Marshal(events[0].Params["contributors"])
	if err != nil {
		t.Fatal(err)
	}
	var contributors []contracts.RoomParticipant
	if err := json.Unmarshal(encoded, &contributors); err != nil {
		t.Fatal(err)
	}
	if len(contributors) != 3 {
		t.Fatalf("five eligible identities must produce exactly three contributors: %+v", contributors)
	}
	seen := make(map[string]bool)
	for _, contributor := range contributors {
		if contributor != byID[contributor.ID] || seen[contributor.ID] {
			t.Fatalf("invalid, renamed or repeated contributor: %+v", contributor)
		}
		seen[contributor.ID] = true
	}
	if !seen[participants[4].ID] {
		t.Fatalf("contributors omitted triggering participant: %+v", contributors)
	}
}
