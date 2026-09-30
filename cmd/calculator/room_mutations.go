package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

func (s *roomService) publish(ctx context.Context, owner string, record contracts.CalculationRecord) (string, error) {
	if !s.config.PublicationEnabled {
		return "", errRoomDisabled
	}
	if record.Outcome.Kind != contracts.OutcomeSuccess {
		return "", errors.New("only successful calculations may be published")
	}
	id, err := randomID(16)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	when := s.now().UTC()
	participant := roomParticipant(owner, s.config.Code)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO room_events
		(id, calculation_id, session_id, room_code, alias, expression, value, angle_unit, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, record.ID, owner, s.config.Code, participant.Alias, record.Expression,
		record.Outcome.Value, record.Context.AngleUnit, when.Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	order, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	result, err = tx.ExecContext(ctx, `UPDATE calculations SET publication_status = 'published', public_event_id = ?
		WHERE id = ? AND session_id = ? AND publication_status = 'pending'`, id, record.ID, owner)
	if err != nil {
		return "", err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return "", errors.New("publication state changed")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO room_counters (room_code, published_calculations)
		VALUES (?, 1) ON CONFLICT (room_code) DO UPDATE SET
		published_calculations = published_calculations + 1`, s.config.Code); err != nil {
		return "", err
	}
	var roomEvents []contracts.FunEvent
	if s.config.EffectsEnabled && record.Outcome.Value == "42" {
		roomEvents, err = s.claimSharedAnswer(ctx, tx, owner, when)
		if err != nil {
			return "", err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM room_events WHERE room_code = ? AND seq NOT IN
		(SELECT seq FROM room_events WHERE room_code = ? ORDER BY seq DESC LIMIT ?)`,
		s.config.Code, s.config.Code, roomFeedLimit); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	event := &roomCalculation{author: owner, reactions: map[string]string{}, public: contracts.RoomCalculationEvent{
		ID: id, Order: order, Participant: participant, Expression: record.Expression,
		Value: record.Outcome.Value, AngleUnit: record.Context.AngleUnit, CreatedAt: when,
		Reactions: map[string]int{}, AchievementIDs: []string{},
	}}
	s.events = append(s.events, event)
	s.byID[id] = event
	s.publishedCount++
	if len(s.events) > roomFeedLimit {
		s.reactionCount -= len(s.events[0].reactions)
		delete(s.byID, s.events[0].public.ID)
		s.events = s.events[1:]
	}
	s.broadcastLocked("calculation", map[string]any{
		"calculation": event.public, "aggregates": s.aggregatesLocked(), "funEvents": roomEvents,
	})
	return id, nil
}

// claimSharedAnswer runs inside the same transaction as the triggering public
// event. Only the latest published 42 per identity is needed for this window.
func (s *roomService) claimSharedAnswer(ctx context.Context, tx *sql.Tx, owner string, when time.Time) ([]contracts.FunEvent, error) {
	ns := when.UnixNano()
	if _, err := tx.ExecContext(ctx, `INSERT INTO room_answer_42 (room_code, session_id, last_at_ns)
		VALUES (?, ?, ?) ON CONFLICT (room_code, session_id) DO UPDATE SET last_at_ns = excluded.last_at_ns`,
		s.config.Code, owner, ns); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM room_answer_42 WHERE room_code = ? AND last_at_ns < ?`,
		s.config.Code, ns-int64(60*time.Second)); err != nil {
		return nil, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM room_answer_42 WHERE room_code = ?`, s.config.Code).Scan(&count); err != nil {
		return nil, err
	}
	if count < 3 {
		return nil, nil
	}
	var last int64
	err := tx.QueryRowContext(ctx, `SELECT last_scene_at_ns FROM room_effect_state WHERE room_code = ?`, s.config.Code).Scan(&last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil && ns-last < int64(120*time.Second) {
		return nil, nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO room_effect_state (room_code, last_scene_at_ns) VALUES (?, ?)
		ON CONFLICT (room_code) DO UPDATE SET last_scene_at_ns = excluded.last_scene_at_ns`, s.config.Code, ns); err != nil {
		return nil, err
	}
	id, err := randomID(16)
	if err != nil {
		return nil, err
	}
	return []contracts.FunEvent{{
		ID: id, RuleID: "shared_answer", Kind: "scene", Scope: "room",
		Params: map[string]any{}, CreatedAt: when, ExpiresAt: when.Add(10 * time.Second),
	}}, nil
}

var allowedRoomReactions = map[string]struct{}{
	"laugh": {}, "wow": {}, "applause": {}, "thinking": {},
}

func (s *roomService) react(ctx context.Context, owner, eventID string, selected *string) (contracts.RoomReactionResponse, error) {
	if !s.config.ReactionsEnabled {
		return contracts.RoomReactionResponse{}, errRoomDisabled
	}
	if selected != nil {
		if _, ok := allowedRoomReactions[*selected]; !ok {
			return contracts.RoomReactionResponse{}, errors.New("invalid reaction")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.admitReactionLocked(owner) {
		return contracts.RoomReactionResponse{}, errRoomRateLimit
	}
	event := s.byID[eventID]
	if event == nil {
		return contracts.RoomReactionResponse{}, errRoomEventNotFound
	}
	old, hasOld := event.reactions[owner]
	if selected == nil && !hasOld || selected != nil && hasOld && old == *selected {
		return s.reactionResponseLocked(event, selected), nil
	}
	if !hasOld && selected != nil && len(event.reactions) >= 256 {
		return contracts.RoomReactionResponse{}, errRoomReactionLimit
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contracts.RoomReactionResponse{}, err
	}
	defer tx.Rollback()
	if selected == nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM room_reactions WHERE event_id = ? AND session_id = ?`, eventID, owner)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO room_reactions (event_id, session_id, reaction_id) VALUES (?, ?, ?)
			ON CONFLICT (event_id, session_id) DO UPDATE SET reaction_id = excluded.reaction_id`, eventID, owner, *selected)
	}
	if err != nil {
		return contracts.RoomReactionResponse{}, err
	}
	var otherParticipants int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM room_reactions
		WHERE event_id = ? AND session_id <> ?`, eventID, event.author).Scan(&otherParticipants); err != nil {
		return contracts.RoomReactionResponse{}, err
	}
	badge := false
	if otherParticipants >= 3 && len(event.public.AchievementIDs) == 0 {
		result, err := tx.ExecContext(ctx, `INSERT INTO room_badges (event_id, rule_id, awarded_at)
			VALUES (?, 'peer_reviewed', ?) ON CONFLICT DO NOTHING`, eventID, s.now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return contracts.RoomReactionResponse{}, err
		}
		if count, err := result.RowsAffected(); err != nil {
			return contracts.RoomReactionResponse{}, err
		} else {
			badge = count == 1
		}
	}
	if err := tx.Commit(); err != nil {
		return contracts.RoomReactionResponse{}, err
	}
	if hasOld {
		event.public.Reactions[old]--
		if event.public.Reactions[old] == 0 {
			delete(event.public.Reactions, old)
		}
	}
	if selected == nil {
		delete(event.reactions, owner)
		s.reactionCount--
	} else {
		event.reactions[owner] = *selected
		event.public.Reactions[*selected]++
		if !hasOld {
			s.reactionCount++
		}
	}
	var funEvents []contracts.FunEvent
	if badge {
		event.public.AchievementIDs = append(event.public.AchievementIDs, "peer_reviewed")
		if s.config.EffectsEnabled {
			if id, err := randomID(16); err == nil {
				when := s.now().UTC()
				funEvents = []contracts.FunEvent{{
					ID: id, RuleID: "peer_reviewed", Kind: "comment", Scope: "room",
					Params:    map[string]any{"eventId": eventID, "authorId": event.public.Participant.ID},
					CreatedAt: when, ExpiresAt: when.Add(10 * time.Second),
				}}
			}
		}
	}
	s.broadcastLocked("reaction", map[string]any{
		"eventId": eventID, "participantId": roomParticipant(owner, s.config.Code).ID,
		"reactionId": selected, "reactions": event.public.Reactions,
		"achievementIds": event.public.AchievementIDs, "funEvents": funEvents,
		"aggregates": s.aggregatesLocked(),
	})
	return s.reactionResponseLocked(event, selected), nil
}

func (s *roomService) reactionResponseLocked(event *roomCalculation, selected *string) contracts.RoomReactionResponse {
	return contracts.RoomReactionResponse{
		EventID: event.public.ID, ReactionID: selected,
		Reactions:      cloneReactionCounts(event.public.Reactions),
		AchievementIDs: append([]string{}, event.public.AchievementIDs...),
		Epoch:          s.epoch, Sequence: s.seq,
	}
}
