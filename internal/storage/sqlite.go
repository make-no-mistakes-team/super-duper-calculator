package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Open opens a file-backed SQLite database.
// An empty path uses data/calculator.sqlite. The caller must close the database
// after its requests and transactions have finished.
func Open(ctx context.Context, path string) (_ *sql.DB, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" {
		path = "data/calculator.sqlite"
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	// Never truncate an existing file or open and close it outside SQLite:
	// closing another descriptor can release SQLite's POSIX transaction locks.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	switch {
	case err == nil:
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close new database file: %w", err)
		}
	case !errors.Is(err, os.ErrExist):
		return nil, fmt.Errorf("create database file: %w", err)
	default:
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect database file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("database path %q is not a regular file", path)
		}
	}

	// Driver DSN pragmas run for every physical connection, including replacements.
	query := url.Values{
		"mode": {"rw"},
		"_pragma": {
			"busy_timeout(5000)",
			"foreign_keys(ON)",
			"journal_mode(WAL)",
			"synchronous(FULL)",
		},
	}
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dsn := url.URL{Scheme: "file", Path: uriPath, RawQuery: query.Encode()}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("configure SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close SQLite: %w", closeErr))
			}
		}
	}()

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return nil, fmt.Errorf("initialize SQLite: %w", err)
	}
	if journalMode != "wal" {
		return nil, fmt.Errorf("SQLite WAL mode is unavailable (journal mode %q)", journalMode)
	}
	// Verify write access without changing data; read-write opens can fall back
	// to read-only connections. No transaction remains open after startup.
	if _, err := db.ExecContext(ctx, "BEGIN IMMEDIATE; ROLLBACK"); err != nil {
		return nil, fmt.Errorf("check SQLite write access: %w", err)
	}
	return db, nil
}
