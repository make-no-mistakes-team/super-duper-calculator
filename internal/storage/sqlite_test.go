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
	var version, sessions, calculations int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("fresh schema version = %d, want 1", version)
	}
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
	requireLegacyRecord(t, db, 1)
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
	requireLegacyRecord(t, reopened, 1)
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
	requireLegacyRecord(t, afterFailure, 0)
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
	requireLegacyRecord(t, retried, 1)
	if _, err := retried.ExecContext(t.Context(), "INSERT INTO sessions (id, expires_at) VALUES (?, ?)", "recovered", 100); err != nil {
		t.Fatalf("retry did not finish the core migration: %v", err)
	}
}

func TestOpenRejectsNewerSchemaWithoutLosingRecords(t *testing.T) {
	path := privateDatabasePath(t, "newer.sqlite")
	legacy := createVersionZeroDatabase(t, path, false)
	if _, err := legacy.ExecContext(t.Context(), "PRAGMA user_version = 2"); err != nil {
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
	requireLegacyRecord(t, preserved, 2)
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

func requireLegacyRecord(t *testing.T, db *sql.DB, wantVersion int) {
	t.Helper()
	var version int
	if err := db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != wantVersion {
		t.Fatalf("schema version = %d, want %d", version, wantVersion)
	}
	var content string
	if err := db.QueryRowContext(t.Context(), "SELECT content FROM legacy_notes WHERE id = 1").Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content != "keep this legacy record" {
		t.Fatalf("legacy record = %q", content)
	}
}
