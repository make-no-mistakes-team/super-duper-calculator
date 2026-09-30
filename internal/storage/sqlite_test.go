package storage_test

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func privateDatabasePath(t *testing.T, name string) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(directory, name)
}

func TestOpenPersistsTransactionsAtLiteralPath(t *testing.T) {
	filename := "calculator #%+.sqlite"
	if runtime.GOOS != "windows" {
		filename = "calculator ?mode=memory&_pragma=foreign_keys(OFF)#%+.sqlite"
	}
	path := filepath.Join(t.TempDir(), "private data", filename)
	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var sessions, calculations int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM calculations").Scan(&calculations); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 || calculations != 0 {
		t.Fatalf("fresh database imported records: sessions=%d calculations=%d", sessions, calculations)
	}
	if _, err := db.ExecContext(t.Context(), "CREATE TABLE entries (id INTEGER PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}

	committed, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer committed.Rollback()
	if _, err := committed.ExecContext(t.Context(), "INSERT INTO entries (id, value) VALUES (?, ?)", 1, "committed"); err != nil {
		t.Fatal(err)
	}
	if err := committed.Commit(); err != nil {
		t.Fatal(err)
	}

	rolledBack, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rolledBack.Rollback()
	if _, err := rolledBack.ExecContext(t.Context(), "INSERT INTO entries (id, value) VALUES (?, ?)", 2, "rolled back"); err != nil {
		t.Fatal(err)
	}
	if err := rolledBack.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database missing at the literal filename: %v", err)
	}

	reopened, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var value string
	if err := reopened.QueryRowContext(t.Context(), "SELECT value FROM entries WHERE id = ?", 1).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "committed" {
		t.Fatalf("persisted value = %q, want committed", value)
	}
	var count int
	if err := reopened.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM entries").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("persisted entries = %d, want only the committed entry", count)
	}
}

func TestOpenEnforcesForeignKeysAfterReconnect(t *testing.T) {
	db, err := storage.Open(t.Context(), privateDatabasePath(t, "constraints.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(t.Context(), `
		CREATE TABLE parent (id INTEGER PRIMARY KEY);
		CREATE TABLE child (parent_id INTEGER NOT NULL REFERENCES parent(id));
		INSERT INTO parent (id) VALUES (1);
		INSERT INTO child (parent_id) VALUES (1);
	`); err != nil {
		t.Fatal(err)
	}

	requireForeignKey := func(t *testing.T) {
		t.Helper()
		_, err := db.ExecContext(t.Context(), "INSERT INTO child (parent_id) VALUES (?)", 999)
		var sqliteErr *sqlite.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
			t.Fatalf("orphan insert error = %v, want SQLite foreign-key violation", err)
		}
	}
	t.Run("initial", requireForeignKey)
	// Closing the idle connection forces the next statement to use a new one.
	db.SetMaxIdleConns(0)
	db.SetMaxIdleConns(1)
	t.Run("recreated", requireForeignKey)
}

func TestOpenPreservesInvalidDatabase(t *testing.T) {
	path := privateDatabasePath(t, "invalid.sqlite")
	original := bytes.Repeat([]byte("not a SQLite database\n"), 32)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := storage.Open(t.Context(), path)
	if db != nil {
		t.Cleanup(func() { _ = db.Close() })
	}
	if err == nil {
		t.Fatal("opening an invalid database succeeded")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, original) {
		t.Fatal("opening an invalid database changed the existing file")
	}
}

func TestOpenMigratesPopulatedVersionZeroAndReopens(t *testing.T) {
	path := privateDatabasePath(t, "legacy.sqlite")
	legacy := createVersionZeroDatabase(t, path, false)
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	requireLegacyRecord(t, db)
	requireSchemaVersion(t, db, 5)
	if _, err := db.ExecContext(t.Context(), "INSERT INTO sessions (id, expires_at) VALUES (?, ?)", "session-1", 100); err != nil {
		t.Fatalf("core schema not initialized: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	requireLegacyRecord(t, reopened)
	requireSchemaVersion(t, reopened, 5)
	var sessions int
	if err := reopened.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sessions WHERE id = ?", "session-1").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 {
		t.Fatalf("reopened sessions = %d, want 1", sessions)
	}
}

func TestOpenMigrationFailureRollsBackAndCanRetry(t *testing.T) {
	path := privateDatabasePath(t, "conflicting.sqlite")
	legacy := createVersionZeroDatabase(t, path, true)
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := storage.Open(t.Context(), path)
	if db != nil {
		t.Cleanup(func() { _ = db.Close() })
	}
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		t.Fatalf("migration error = %v, want underlying SQLite error", err)
	}
	if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), "calculations") {
		t.Fatalf("migration error exposes file path or SQL details: %v", err)
	}

	afterFailure, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	requireLegacyRecord(t, afterFailure)
	requireSchemaVersion(t, afterFailure, 0)
	var sessions int
	if err := afterFailure.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatal("failed migration retained a table created before the error")
	}
	var conflicting int
	if err := afterFailure.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM calculations").Scan(&conflicting); err != nil {
		t.Fatalf("migration changed the preexisting table: %v", err)
	}
	if conflicting != 1 {
		t.Fatalf("preexisting calculations = %d, want 1", conflicting)
	}
	if _, err := afterFailure.ExecContext(t.Context(), "DROP TABLE calculations"); err != nil {
		t.Fatal(err)
	}
	if err := afterFailure.Close(); err != nil {
		t.Fatal(err)
	}

	retried, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = retried.Close() })
	requireLegacyRecord(t, retried)
	if _, err := retried.ExecContext(t.Context(), "INSERT INTO sessions (id, expires_at) VALUES (?, ?)", "recovered", 100); err != nil {
		t.Fatalf("retry did not finish the core migration: %v", err)
	}
}

func TestOpenRejectsNewerSchemaWithoutLosingRecords(t *testing.T) {
	path := privateDatabasePath(t, "newer.sqlite")
	legacy := createVersionZeroDatabase(t, path, false)
	if _, err := legacy.ExecContext(t.Context(), "PRAGMA user_version = 2147483647"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := storage.Open(t.Context(), path)
	if db != nil {
		t.Cleanup(func() { _ = db.Close() })
	}
	if err == nil {
		t.Fatal("newer schema unexpectedly opened")
	}
	if strings.Contains(err.Error(), path) {
		t.Fatalf("schema error exposes database path: %v", err)
	}
	preserved, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = preserved.Close() })
	requireLegacyRecord(t, preserved)
	requireSchemaVersion(t, preserved, 2147483647)
}

func createVersionZeroDatabase(t *testing.T, path string, conflicting bool) *sql.DB {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `
		CREATE TABLE legacy_notes (id INTEGER PRIMARY KEY, content TEXT NOT NULL);
		INSERT INTO legacy_notes (id, content) VALUES (1, 'keep this legacy record');
	`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if conflicting {
		if _, err := db.ExecContext(t.Context(), `
			CREATE TABLE calculations (id INTEGER PRIMARY KEY);
			INSERT INTO calculations (id) VALUES (17);
		`); err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
	}
	return db
}

func requireSchemaVersion(t *testing.T, db *sql.DB, wantVersion int) {
	t.Helper()
	var version int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != wantVersion {
		t.Fatalf("schema version = %d, want %d", version, wantVersion)
	}
}

func requireLegacyRecord(t *testing.T, db *sql.DB) {
	t.Helper()
	var content string
	if err := db.QueryRowContext(t.Context(), "SELECT content FROM legacy_notes WHERE id = 1").Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content != "keep this legacy record" {
		t.Fatalf("legacy record = %q", content)
	}
}

func TestOpenExpressionAngleMigrationPreservesHistoryAndProgress(t *testing.T) {
	path := privateDatabasePath(t, "expression-angle-upgrade.sqlite")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	for _, migration := range []string{
		"migrations/001_core.sql", "migrations/002_achievements.sql",
		"migrations/003_personal_effects.sql", "migrations/004_discovery_progress.sql",
	} {
		schema, err := os.ReadFile(migration)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := legacy.ExecContext(t.Context(), string(schema)); err != nil {
			t.Fatal(err)
		}
	}
	when := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)
	createdAt := when.Format(time.RFC3339Nano)
	const degreeOutcome = `{"kind":"success","value":"1"}`
	const radianOutcome = `{"kind":"success","value":"6.123233995736757e-17"}`
	const radianFacts = `{"operators":{"/":1},"functions":{"cos":1},"operationCount":2,"depth":1}`
	if _, err := legacy.ExecContext(t.Context(), `
		PRAGMA foreign_keys = ON;
		INSERT INTO sessions (id, expires_at) VALUES ('owner', 4102444800), ('other', 4102444801);
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), `
		INSERT INTO calculations
			(seq, id, session_id, request_id, expression, angle_unit, semantics_version, outcome_json, facts_json, created_at)
		VALUES
			(7, 'degree-source', 'owner', 'degree-action', 'sin(90)', 'deg', 'binary64-v1', ?, NULL, ?),
			(19, 'radian-source', 'owner', 'radian-action', 'cos(pi/2)', 'rad', 'binary64-v1', ?, ?, ?),
			(23, 'removed-source', 'other', 'removed-action', '2', 'deg', 'binary64-v1', '{"kind":"success","value":"2"}', NULL, ?);
	`, degreeOutcome, createdAt, radianOutcome, radianFacts, createdAt, createdAt); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), `
		DELETE FROM calculations WHERE id = 'removed-source';
		INSERT INTO discovery_progress (session_id, last_sequence, accepted_count) VALUES ('owner', 19, 2);
		PRAGMA user_version = 4;
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), `
		INSERT INTO achievements (session_id, achievement_id, calculation_id, earned_at)
			VALUES ('owner', 'scientific_method', 'degree-source', ?)`, createdAt); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), `
		INSERT INTO personal_effects (session_id, last_calculation_id, last_scene_at_ns)
			VALUES ('owner', 'radian-source', ?)`, when.UnixNano()); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	requireSchemaVersion(t, upgraded, 5)
	var obsoleteColumns, foreignKeys, expiry, highWatermark int64
	if err := upgraded.QueryRowContext(t.Context(),
		"SELECT COUNT(*) FROM pragma_table_info('calculations') WHERE name = 'angle_unit'").Scan(&obsoleteColumns); err != nil || obsoleteColumns != 0 {
		t.Fatalf("obsolete context column remains: %d, %v", obsoleteColumns, err)
	}
	if err := upgraded.QueryRowContext(t.Context(), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign keys disabled after upgrade: %d, %v", foreignKeys, err)
	}
	if err := upgraded.QueryRowContext(t.Context(), "SELECT expires_at FROM sessions WHERE id = 'owner'").Scan(&expiry); err != nil || expiry != 4102444800 {
		t.Fatalf("session expiry changed: %d, %v", expiry, err)
	}
	if err := upgraded.QueryRowContext(t.Context(), "SELECT seq FROM sqlite_sequence WHERE name = 'calculations'").Scan(&highWatermark); err != nil || highWatermark != 23 {
		t.Fatalf("sequence high-water mark changed: %d, %v", highWatermark, err)
	}
	// Saved source, formatting, facts and historical semantics must survive
	// byte-for-byte; the upgrade must never recompute old results.
	for _, saved := range []struct {
		id, requestID, expression, outcome, facts string
		sequence                                  int64
	}{
		{"radian-source", "radian-action", "cos(pi/2)", radianOutcome, radianFacts, 19},
		{"degree-source", "degree-action", "sin(90)", degreeOutcome, "", 7},
	} {
		var requestID, expression, version, outcome, timestamp string
		var facts sql.NullString
		var sequence int64
		if err := upgraded.QueryRowContext(t.Context(), `
			SELECT seq, request_id, expression, semantics_version, outcome_json, facts_json, created_at
			FROM calculations WHERE session_id = 'owner' AND id = ?`, saved.id).
			Scan(&sequence, &requestID, &expression, &version, &outcome, &facts, &timestamp); err != nil {
			t.Fatal(err)
		}
		if sequence != saved.sequence || requestID != saved.requestID || expression != saved.expression ||
			version != "binary64-v1" || outcome != saved.outcome || timestamp != createdAt ||
			facts.Valid != (saved.facts != "") || facts.String != saved.facts {
			t.Fatalf("historical record changed: seq=%d request=%q source=%q version=%q outcome=%q facts=%+v timestamp=%q",
				sequence, requestID, expression, version, outcome, facts, timestamp)
		}
	}
	cursor := int64(24)
	for _, want := range []struct {
		id       string
		sequence int64
	}{{"radian-source", 19}, {"degree-source", 7}} {
		var sequence int64
		record, err := storage.ReadCalculationRecord(upgraded.QueryRowContext(t.Context(), `
			SELECT `+storage.SequencedCalculationRecordColumns+` FROM calculations
			WHERE session_id = 'owner' AND seq < ? ORDER BY seq DESC LIMIT 1`, cursor), &sequence, storage.StrictFacts)
		if err != nil || record.ID != want.id || sequence != want.sequence ||
			record.Context.SemanticsVersion != "binary64-v1" || !record.CreatedAt.Equal(when) {
			t.Fatalf("upgraded history pagination changed: %+v, seq=%d, %v", record, sequence, err)
		}
		cursor = sequence
	}
	awards, err := storage.ListAchievements(t.Context(), upgraded, "owner")
	if err != nil || len(awards) != 1 || awards[0].ID != "scientific_method" || !awards[0].EarnedAt.Equal(when) {
		t.Fatalf("saved awards changed: %+v, %v", awards, err)
	}
	var awardSource, effectSource string
	var sceneTime int64
	if err := upgraded.QueryRowContext(t.Context(), `
		SELECT calculation_id FROM achievements WHERE session_id = 'owner' AND achievement_id = 'scientific_method'`).
		Scan(&awardSource); err != nil || awardSource != "degree-source" {
		t.Fatalf("saved award source changed: %q, %v", awardSource, err)
	}
	if err := upgraded.QueryRowContext(t.Context(), `
		SELECT last_calculation_id, last_scene_at_ns FROM personal_effects WHERE session_id = 'owner'`).
		Scan(&effectSource, &sceneTime); err != nil || effectSource != "radian-source" || sceneTime != when.UnixNano() {
		t.Fatalf("saved effect cooldown changed: %q, %d, %v", effectSource, sceneTime, err)
	}
	progress, err := storage.ReadDiscoveryProgress(t.Context(), upgraded, "owner")
	if err != nil || progress != (storage.DiscoveryProgress{LastSequence: 19, AcceptedCount: 2}) {
		t.Fatalf("discovery checkpoint changed: %+v, %v", progress, err)
	}
	violations, err := upgraded.QueryContext(t.Context(), "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if violations.Next() {
		t.Fatal("upgrade left dangling source references")
	}
	if err := violations.Err(); err != nil {
		t.Fatal(err)
	}
	if err := violations.Close(); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"DELETE FROM calculations WHERE id = 'degree-source'",
		"DELETE FROM calculations WHERE id = 'radian-source'",
		"UPDATE achievements SET session_id = 'other' WHERE session_id = 'owner'",
	} {
		_, err := upgraded.ExecContext(t.Context(), statement)
		var sqliteErr *sqlite.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != sqlite3.SQLITE_CONSTRAINT {
			t.Fatalf("saved source constraint lost: %q returned %v", statement, err)
		}
	}
	var retainedSources, retainedAward, retainedEffect int
	if err := upgraded.QueryRowContext(t.Context(), `
		SELECT
			(SELECT COUNT(*) FROM calculations WHERE session_id = 'owner' AND id IN ('degree-source', 'radian-source')),
			(SELECT COUNT(*) FROM achievements WHERE session_id = 'owner' AND achievement_id = 'scientific_method' AND calculation_id = 'degree-source'),
			(SELECT COUNT(*) FROM personal_effects WHERE session_id = 'owner' AND last_calculation_id = 'radian-source')
	`).Scan(&retainedSources, &retainedAward, &retainedEffect); err != nil ||
		retainedSources != 2 || retainedAward != 1 || retainedEffect != 1 {
		t.Fatalf("rejected source changes modified records or ownership: sources=%d award=%d effect=%d, %v",
			retainedSources, retainedAward, retainedEffect, err)
	}
	if _, err := upgraded.ExecContext(t.Context(), `
		INSERT INTO calculations (id, session_id, request_id, expression, semantics_version, outcome_json, created_at)
		VALUES ('new-source', 'owner', 'new-action', 'sin(90°)', 'binary64-v2', ?, ?)`, degreeOutcome, createdAt); err != nil {
		t.Fatalf("angle-free calculation could not be saved after upgrade: %v", err)
	}
	var newSequence int64
	if err := upgraded.QueryRowContext(t.Context(), "SELECT seq FROM calculations WHERE id = 'new-source'").
		Scan(&newSequence); err != nil || newSequence != 24 {
		t.Fatalf("new action reused historical sequence: %d, %v", newSequence, err)
	}
	newAwards, err := storage.CommitDiscoveryProgress(t.Context(), upgraded, "owner", progress,
		storage.DiscoveryProgress{LastSequence: 24, AcceptedCount: 3},
		[]storage.DiscoveryGrant{{CalculationID: "new-source", IDs: []string{"scientific_method"}}})
	if err != nil || len(newAwards) != 0 {
		t.Fatalf("upgraded checkpoint re-awarded existing discovery: %+v, %v", newAwards, err)
	}
	if err := upgraded.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	progress, err = storage.ReadDiscoveryProgress(t.Context(), reopened, "owner")
	if err != nil || progress != (storage.DiscoveryProgress{LastSequence: 24, AcceptedCount: 3}) {
		t.Fatalf("upgraded checkpoint did not survive restart: %+v, %v", progress, err)
	}
	awards, err = storage.ListAchievements(t.Context(), reopened, "owner")
	if err != nil || len(awards) != 1 || !awards[0].EarnedAt.Equal(when) {
		t.Fatalf("upgraded awards did not survive restart: %+v, %v", awards, err)
	}
}

func TestOpenDiscoveryProgressMigrationPreservesHistoryAndAwards(t *testing.T) {
	path := privateDatabasePath(t, "discovery-upgrade.sqlite")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	for _, migration := range []string{
		"migrations/001_core.sql", "migrations/002_achievements.sql", "migrations/003_personal_effects.sql",
	} {
		schema, err := os.ReadFile(migration)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := legacy.ExecContext(t.Context(), string(schema)); err != nil {
			t.Fatal(err)
		}
	}
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	// Seed the historical schema directly, independently of current helpers.
	if _, err := legacy.ExecContext(t.Context(), `
		INSERT INTO sessions (id, expires_at) VALUES ('owner', 4102444800);
		INSERT INTO calculations
			(id, session_id, request_id, expression, angle_unit, semantics_version, outcome_json, facts_json, created_at)
		VALUES ('legacy-source', 'owner', 'legacy-source', '((((((60+7))))))', 'deg', 'binary64-v1',
			'{"kind":"success","value":"67"}', NULL, ?)`, when.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	// This fixture predates discovery_progress; seed its original award directly.
	if _, err := legacy.ExecContext(t.Context(), `
		INSERT INTO achievements (session_id, achievement_id, calculation_id, earned_at)
		VALUES ('owner', 'six_seven', 'legacy-source', ?)`, when.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(t.Context(), "PRAGMA user_version = 3"); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	requireSchemaVersion(t, upgraded, 5)
	var source, expression string
	if err := upgraded.QueryRowContext(t.Context(), `
		SELECT a.calculation_id, c.expression FROM achievements a
		JOIN calculations c ON c.session_id = a.session_id AND c.id = a.calculation_id
		WHERE a.session_id = 'owner' AND a.achievement_id = 'six_seven'`).Scan(&source, &expression); err != nil ||
		source != "legacy-source" || expression != "((((((60+7))))))" {
		t.Fatalf("migration changed history or award source: %q, %q, %v", source, expression, err)
	}
	awards, err := storage.ListAchievements(t.Context(), upgraded, "owner")
	if err != nil || len(awards) != 1 || !awards[0].EarnedAt.Equal(when) {
		t.Fatalf("migration changed earnedAt: %+v, %v", awards, err)
	}
	progress, err := storage.ReadDiscoveryProgress(t.Context(), upgraded, "owner")
	if err != nil || progress != (storage.DiscoveryProgress{}) {
		t.Fatalf("migration skipped unreconciled history: %+v, %v", progress, err)
	}
	// Re-evaluating the preserved prefix advances progress without replacing
	// its existing award or claiming a fresh announcement.
	awarded, err := storage.CommitDiscoveryProgress(t.Context(), upgraded, "owner",
		progress, storage.DiscoveryProgress{LastSequence: 1, AcceptedCount: 1},
		[]storage.DiscoveryGrant{{CalculationID: "legacy-source", IDs: []string{"six_seven"}}})
	if err != nil || len(awarded) != 0 {
		t.Fatalf("upgraded history re-awarded discovery: %+v, %v", awarded, err)
	}
}
