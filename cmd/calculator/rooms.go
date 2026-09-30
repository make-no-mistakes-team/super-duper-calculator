package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

const (
	roomFeedLimit    = 100
	roomJournalLimit = 512
	roomViewTTL      = 45 * time.Second
	roomHeartbeat    = 15 * time.Second
)

var (
	errRoomNotFound      = errors.New("room not found")
	errRoomViewNotFound  = errors.New("room view not found")
	errRoomEventNotFound = errors.New("room event not found")
	errRoomDisabled      = errors.New("room publication disabled")
	errRoomReactionLimit = errors.New("room reaction limit")
	errRoomViewLimit     = errors.New("room view limit")
	errRoomRateLimit     = errors.New("room rate limit")
)

type roomConfig struct {
	Enabled            bool
	Code               string
	PublicationEnabled bool
	ReactionsEnabled   bool
	EffectsEnabled     bool
	Shutdown           <-chan struct{}
}

type roomCalculation struct {
	public    contracts.RoomCalculationEvent
	author    string
	reactions map[string]string // session owner -> reaction ID
}

type roomMessage struct {
	id   string
	kind string
	data []byte
}

type roomView struct {
	owner     string
	expiresAt time.Time
	stream    chan roomMessage
}

type roomQuota struct {
	tokens float64
	last   time.Time
}

// roomService serializes short room mutations. No subscriber holds the lock or
// a SQLite connection while waiting for network output.
type roomService struct {
	db               *sql.DB
	config           roomConfig
	now              func() time.Time
	mu               sync.Mutex
	epoch            string
	seq              int64
	events           []*roomCalculation
	byID             map[string]*roomCalculation
	publishedCount   int64
	reactionCount    int
	views            map[string]*roomView
	journal          []roomMessage
	streams          chan struct{}
	streamQuota      roomQuota
	reactionQuota    roomQuota
	perOwnerReaction map[string]roomQuota
}

func newRoomService(ctx context.Context, db *sql.DB, config roomConfig) (*roomService, error) {
	if !config.Enabled {
		return nil, nil
	}
	if config.Code == "" || len(config.Code) > 32 || strings.Trim(config.Code, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" {
		return nil, errors.New("invalid room code")
	}
	epoch, err := randomID(12)
	if err != nil {
		return nil, err
	}
	s := &roomService{
		db: db, config: config, now: time.Now, epoch: epoch,
		byID: make(map[string]*roomCalculation), views: make(map[string]*roomView),
		streams: make(chan struct{}, 64), perOwnerReaction: make(map[string]roomQuota),
	}
	s.streamQuota = roomQuota{tokens: 64, last: s.now()}
	s.reactionQuota = roomQuota{tokens: 100, last: s.now()}
	// An interrupted attempt has no public event. Never publish it on recovery.
	if _, err := db.ExecContext(ctx, "UPDATE calculations SET publication_status = 'unavailable' WHERE publication_status = 'pending'"); err != nil {
		return nil, err
	}
	if !config.EffectsEnabled {
		if _, err := db.ExecContext(ctx, `DELETE FROM room_answer_42 WHERE room_code = ?`, config.Code); err != nil {
			return nil, err
		}
	}
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func roomParticipant(owner, code string) contracts.RoomParticipant {
	id := sha256.Sum256([]byte("calculator-room-participant:" + code + ":" + owner))
	return contracts.RoomParticipant{ID: hex.EncodeToString(id[:]), Alias: displayAlias(owner)}
}

func displayAlias(owner string) string {
	alias := sha256.Sum256([]byte("calculator-display-alias:" + owner))
	return "Гость " + strings.ToUpper(hex.EncodeToString(alias[:4]))
}

func (s *roomService) load(ctx context.Context) error {
	if err := s.db.QueryRowContext(ctx, `SELECT published_calculations FROM room_counters WHERE room_code = ?`,
		s.config.Code).Scan(&s.publishedCount); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT seq, id, session_id, alias, expression, value, angle_unit, created_at
		FROM room_events WHERE room_code = ? ORDER BY seq DESC LIMIT ?`, s.config.Code, roomFeedLimit)
	if err != nil {
		return err
	}
	for rows.Next() {
		var seq int64
		var id, owner, alias, expression, value, angle, created string
		if err := rows.Scan(&seq, &id, &owner, &alias, &expression, &value, &angle, &created); err != nil {
			_ = rows.Close()
			return err
		}
		when, err := time.Parse(time.RFC3339Nano, created)
		if err != nil {
			_ = rows.Close()
			return err
		}
		participant := roomParticipant(owner, s.config.Code)
		participant.Alias = alias
		event := &roomCalculation{author: owner, reactions: make(map[string]string), public: contracts.RoomCalculationEvent{
			ID: id, Order: seq, Participant: participant, Expression: expression, Value: value,
			AngleUnit: contracts.AngleUnit(angle), CreatedAt: when,
			Reactions: map[string]int{}, AchievementIDs: []string{},
		}}
		s.events = append(s.events, event)
		s.byID[id] = event
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for i, j := 0, len(s.events)-1; i < j; i, j = i+1, j-1 {
		s.events[i], s.events[j] = s.events[j], s.events[i]
	}
	rows, err = s.db.QueryContext(ctx, `SELECT r.event_id, r.session_id, r.reaction_id
		FROM room_reactions r JOIN room_events e ON e.id = r.event_id WHERE e.room_code = ?`, s.config.Code)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, owner, reaction string
		if err := rows.Scan(&id, &owner, &reaction); err != nil {
			_ = rows.Close()
			return err
		}
		if event := s.byID[id]; event != nil {
			event.reactions[owner] = reaction
			event.public.Reactions[reaction]++
			s.reactionCount++
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT b.event_id, b.rule_id FROM room_badges b
		JOIN room_events e ON e.id = b.event_id WHERE e.room_code = ?`, s.config.Code)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, rule string
		if err := rows.Scan(&id, &rule); err != nil {
			_ = rows.Close()
			return err
		}
		if event := s.byID[id]; event != nil {
			event.public.AchievementIDs = append(event.public.AchievementIDs, rule)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	return rows.Close()
}

func (s *roomService) validCode(code string) bool { return code == s.config.Code }

func (s *roomService) snapshotLocked(owner string) contracts.RoomSnapshot {
	s.expireViewsLocked()
	events := make([]contracts.RoomCalculationEvent, 0, len(s.events))
	myReactions := make(map[string]string)
	for _, event := range s.events {
		copyEvent := event.public
		copyEvent.Reactions = cloneReactionCounts(event.public.Reactions)
		copyEvent.AchievementIDs = append([]string{}, event.public.AchievementIDs...)
		events = append(events, copyEvent)
		if reaction, ok := event.reactions[owner]; ok {
			myReactions[event.public.ID] = reaction
		}
	}
	return contracts.RoomSnapshot{
		Code: s.config.Code, Epoch: s.epoch, Sequence: s.seq,
		PublicationEnabled: s.config.PublicationEnabled,
		Participant:        roomParticipant(owner, s.config.Code),
		Presence:           s.presenceLocked(), Calculations: events, MyReactions: myReactions,
		Aggregates: contracts.RoomAggregates{
			PublishedCalculations: s.publishedCount, ActiveReactions: s.reactionCount,
		},
	}
}

func cloneReactionCounts(value map[string]int) map[string]int {
	out := make(map[string]int, len(value))
	for key, count := range value {
		out[key] = count
	}
	return out
}

func (s *roomService) aggregatesLocked() contracts.RoomAggregates {
	return contracts.RoomAggregates{
		PublishedCalculations: s.publishedCount, ActiveReactions: s.reactionCount,
	}
}

func (s *roomService) presenceLocked() int {
	owners := make(map[string]struct{}, len(s.views))
	for _, view := range s.views {
		owners[view.owner] = struct{}{}
	}
	return len(owners)
}

func (s *roomService) expireViewsLocked() {
	now := s.now()
	old := s.presenceLocked()
	for id, view := range s.views {
		if view.stream == nil && !now.Before(view.expiresAt) {
			delete(s.views, id)
		}
	}
	if s.presenceLocked() != old {
		s.broadcastLocked("presence", map[string]int{"participants": s.presenceLocked()})
	}
}

func (s *roomService) join(owner string) (contracts.RoomJoinResponse, error) {
	viewID, err := randomID(16)
	if err != nil {
		return contracts.RoomJoinResponse{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireViewsLocked()
	if len(s.views) >= 256 {
		return contracts.RoomJoinResponse{}, errRoomViewLimit
	}
	old := s.presenceLocked()
	s.views[viewID] = &roomView{owner: owner, expiresAt: s.now().Add(roomViewTTL)}
	if s.presenceLocked() != old {
		s.broadcastLocked("presence", map[string]int{"participants": s.presenceLocked()})
	}
	return contracts.RoomJoinResponse{ViewID: viewID, Snapshot: s.snapshotLocked(owner)}, nil
}

func spendQuota(quota *roomQuota, now time.Time, rate, capacity float64) bool {
	if elapsed := now.Sub(quota.last).Seconds(); elapsed > 0 {
		quota.tokens += elapsed * rate
		if quota.tokens > capacity {
			quota.tokens = capacity
		}
		quota.last = now
	}
	if quota.tokens < 1 {
		return false
	}
	quota.tokens--
	return true
}

func (s *roomService) admitStream() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return spendQuota(&s.streamQuota, s.now(), 10, 64)
}

func (s *roomService) admitReactionLocked(owner string) bool {
	now := s.now()
	ownerQuota, exists := s.perOwnerReaction[owner]
	if !exists {
		ownerQuota = roomQuota{tokens: 6, last: now}
	}
	if !spendQuota(&ownerQuota, now, 2, 6) || !spendQuota(&s.reactionQuota, now, 20, 100) {
		return false
	}
	s.perOwnerReaction[owner] = ownerQuota
	if len(s.perOwnerReaction) > 2048 {
		for id, quota := range s.perOwnerReaction {
			if now.Sub(quota.last) > time.Minute {
				delete(s.perOwnerReaction, id)
			}
		}
	}
	return true
}

func (s *roomService) leave(owner, viewID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireViewsLocked()
	view := s.views[viewID]
	if view == nil || view.owner != owner {
		return false
	}
	old := s.presenceLocked()
	if view.stream != nil {
		close(view.stream)
	}
	delete(s.views, viewID)
	if s.presenceLocked() != old {
		s.broadcastLocked("presence", map[string]int{"participants": s.presenceLocked()})
	}
	return true
}

func (s *roomService) broadcastLocked(kind string, payload any) {
	s.seq++
	id := fmt.Sprintf("%s:%d", s.epoch, s.seq)
	data, _ := json.Marshal(payload)
	message := roomMessage{id: id, kind: kind, data: data}
	s.journal = append(s.journal, message)
	if len(s.journal) > roomJournalLimit {
		s.journal = s.journal[len(s.journal)-roomJournalLimit:]
	}
	for _, view := range s.views {
		if view.stream == nil {
			continue
		}
		select {
		case view.stream <- message:
		default:
			close(view.stream)
			view.stream = nil
			view.expiresAt = s.now().Add(roomViewTTL)
		}
	}
}

func (s *roomService) subscribe(owner, viewID, lastID string) (chan roomMessage, []roomMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireViewsLocked()
	view := s.views[viewID]
	if view == nil || view.owner != owner {
		return nil, nil, errRoomViewNotFound
	}
	if view.stream != nil {
		close(view.stream)
	}
	stream := make(chan roomMessage, 64)
	view.stream = stream
	var initial []roomMessage
	if epoch, number, ok := parseRoomCursor(lastID); ok && epoch == s.epoch &&
		(number == s.seq || len(s.journal) > 0 && number >= cursorNumber(s.journal[0].id)-1 && number < s.seq) {
		for _, message := range s.journal {
			if cursorNumber(message.id) > number {
				initial = append(initial, withoutReplayAnnouncements(message))
			}
		}
	} else {
		snapshot := s.snapshotLocked(owner)
		data, _ := json.Marshal(snapshot)
		initial = []roomMessage{{id: fmt.Sprintf("%s:%d", s.epoch, s.seq), kind: "snapshot", data: data}}
	}
	return stream, initial, nil
}

// Missed state changes are replayed, but a reconnect is not an instruction to
// perform an old theatrical effect or announce an already-earned badge.
func withoutReplayAnnouncements(message roomMessage) roomMessage {
	if message.kind != "calculation" && message.kind != "reaction" {
		return message
	}
	var payload map[string]json.RawMessage
	if json.Unmarshal(message.data, &payload) != nil {
		return message
	}
	delete(payload, "funEvents")
	data, err := json.Marshal(payload)
	if err != nil {
		return message
	}
	message.data = data
	return message
}

func (s *roomService) disconnect(owner, viewID string, stream chan roomMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if view := s.views[viewID]; view != nil && view.owner == owner && view.stream == stream {
		view.stream = nil
		view.expiresAt = s.now().Add(roomViewTTL)
	}
}

func parseRoomCursor(id string) (string, int64, bool) {
	epoch, sequence, ok := strings.Cut(id, ":")
	if !ok || epoch == "" {
		return "", 0, false
	}
	var number int64
	if _, err := fmt.Sscanf(sequence, "%d", &number); err != nil || number < 0 || fmt.Sprintf("%d", number) != sequence {
		return "", 0, false
	}
	return epoch, number, true
}

func cursorNumber(id string) int64 {
	_, number, _ := parseRoomCursor(id)
	return number
}
