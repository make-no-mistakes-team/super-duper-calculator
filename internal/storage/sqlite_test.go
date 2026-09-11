package storage_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

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
	db, err := storage.Open(t.Context(), filepath.Join(t.TempDir(), "constraints.sqlite"))
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
	path := filepath.Join(t.TempDir(), "invalid.sqlite")
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
