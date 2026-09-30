package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func TestRoomParticipantNameStableAcrossConcurrentCallsAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "names.sqlite")
	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(t.Context(), `INSERT INTO sessions (id, expires_at) VALUES ('owner', 9999999999)`); err != nil {
		t.Fatal(err)
	}
	const callers = 20
	results := make(chan contracts.RoomParticipant, callers)
	errors := make(chan error, callers)
	var workers sync.WaitGroup
	for range callers {
		workers.Go(func() {
			participant, err := ensureRoomParticipant(t.Context(), db, "demo", "owner")
			results <- participant
			errors <- err
		})
	}
	workers.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first contracts.RoomParticipant
	for participant := range results {
		if first.ID == "" {
			first = participant
		}
		if participant != first {
			t.Fatalf("one owner received multiple identities: %+v and %+v", first, participant)
		}
	}
	if !regexp.MustCompile(`^[A-Z][a-z]+ [A-Z][a-z]+$`).MatchString(first.Alias) {
		t.Fatalf("unexpected name: %q", first.Alias)
	}
	if first.ID != roomParticipant("owner", "demo").ID {
		t.Fatalf("public identity changed: %+v", first)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ensureRoomParticipant(t.Context(), db, "demo", "owner")
	if err != nil || after != first {
		t.Fatalf("restart changed participant: %+v, %v", after, err)
	}
}

func TestRoomParticipantNameCollisionsAndRoomScope(t *testing.T) {
	db, err := storage.Open(t.Context(), filepath.Join(t.TempDir(), "private", "names.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(t.Context(), `INSERT INTO sessions (id, expires_at) VALUES ('first', 9999999999), ('second', 9999999999)`); err != nil {
		t.Fatal(err)
	}
	fixed := func() (string, error) { return "Brave Otter", nil }
	first, err := ensureRoomParticipantWithAlias(t.Context(), db, "demo", "first", fixed)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	second, err := ensureRoomParticipantWithAlias(t.Context(), db, "demo", "second", func() (string, error) {
		calls++
		if calls == 1 {
			return "Brave Otter", nil
		}
		return "Calm Whale", nil
	})
	if err != nil || calls != 2 || second.Alias != "Calm Whale" || second.ID == first.ID {
		t.Fatalf("collision not resolved: %+v calls=%d err=%v", second, calls, err)
	}
	otherRoom, err := ensureRoomParticipantWithAlias(t.Context(), db, "other", "first", fixed)
	if err != nil || otherRoom.Alias != first.Alias || otherRoom.ID == first.ID {
		t.Fatalf("wrong room scope: %+v, %v", otherRoom, err)
	}
	entropyErr := errors.New("entropy unavailable")
	known, err := ensureRoomParticipantWithAlias(t.Context(), db, "demo", "first", func() (string, error) {
		return "", entropyErr
	})
	if err != nil || known != first {
		t.Fatalf("existing name depends on randomness: %+v, %v", known, err)
	}
	_, err = ensureRoomParticipantWithAlias(t.Context(), db, "new", "first", func() (string, error) {
		return "", entropyErr
	})
	if !errors.Is(err, entropyErr) {
		t.Fatalf("allocation error = %v", err)
	}
}

func TestRoomParticipantsUpgradeVersionFive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "legacy.sqlite")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = legacy.Close() })
	migrations, err := filepath.Glob("../../internal/storage/migrations/00[1-5]_*.sql")
	if err != nil || len(migrations) != 5 {
		t.Fatalf("legacy migrations = %v, %v", migrations, err)
	}
	for _, path := range migrations {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := legacy.ExecContext(t.Context(), string(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, owner := range []string{"author", "reactor", "contributor"} {
		if _, err := legacy.ExecContext(t.Context(), `INSERT INTO sessions (id, expires_at) VALUES (?, 9999999999)`, owner); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 2 {
		id := fmt.Sprintf("calc-%d", i)
		if _, err := legacy.ExecContext(t.Context(), `INSERT INTO calculations
			(id, session_id, request_id, expression, angle_unit, semantics_version, outcome_json, created_at)
			VALUES (?, 'author', ?, '42', 'deg', '1', '{}', '2026-01-01T00:00:00Z')`, id, id); err != nil {
			t.Fatal(err)
		}
		if _, err := legacy.ExecContext(t.Context(), `INSERT INTO room_events
			(id, calculation_id, session_id, room_code, alias, expression, value, angle_unit, created_at)
			VALUES (?, ?, 'author', 'demo', 'Гость old', '42', '42', 'deg', '2026-01-01T00:00:00Z')`, id, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := legacy.ExecContext(t.Context(), `INSERT INTO room_reactions VALUES ('calc-0', 'reactor', 'spark')`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), `INSERT INTO room_answer_42 VALUES ('demo', 'contributor', 123)`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), `PRAGMA user_version = 5`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := reconcileRoomParticipants(t.Context(), db, "demo"); err != nil {
		t.Fatal(err)
	}
	author, err := ensureRoomParticipant(t.Context(), db, "demo", "author")
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM room_participants WHERE room_code = 'demo'`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("backfilled participants = %d, %v", count, err)
	}
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM room_events WHERE alias = ?`, author.Alias).Scan(&count); err != nil || count != 2 {
		t.Fatalf("renamed feed entries = %d, %v", count, err)
	}
	if err := reconcileRoomParticipants(t.Context(), db, "demo"); err != nil {
		t.Fatal(err)
	}
	again, err := ensureRoomParticipant(t.Context(), db, "demo", "author")
	if err != nil || again != author {
		t.Fatalf("backfill reassigned name: %+v, %v", again, err)
	}
}
