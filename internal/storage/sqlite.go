package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// openError keeps driver and OS errors available to errors.Is/As without
// putting their potentially sensitive messages in startup logs.
type openError struct {
	stage string
	cause error
}

func (e *openError) Error() string { return "open SQLite database: " + e.stage + " failed" }
func (e *openError) Unwrap() error { return e.cause }

func openFailure(stage string, cause error) error {
	return &openError{stage: stage, cause: cause}
}

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
		return nil, openFailure("resolve path", err)
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, openFailure("create directory", err)
	}
	if err := requirePrivatePath(parent, true, true); err != nil {
		return nil, openFailure("inspect directory", err)
	}
	// SQLite can reuse existing sidecars during recovery. Reject unsafe files
	// before the driver has an opportunity to read or modify them.
	for _, suffix := range [...]string{"-wal", "-shm", "-journal"} {
		if err := requirePrivatePath(path+suffix, false, false); err != nil {
			return nil, openFailure("inspect sidecar", err)
		}
	}

	// Never truncate an existing file or open and close it outside SQLite:
	// closing another descriptor can release SQLite's POSIX transaction locks.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	switch {
	case err == nil:
		if err := file.Close(); err != nil {
			return nil, openFailure("close new file", err)
		}
	case !errors.Is(err, os.ErrExist):
		return nil, openFailure("create file", err)
	default:
		if err := requirePrivatePath(path, false, true); err != nil {
			return nil, openFailure("inspect file", err)
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
		return nil, openFailure("configure connection", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, openFailure("close connection", closeErr))
			}
		}
	}()

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return nil, openFailure("initialize connection", err)
	}
	if journalMode != "wal" {
		return nil, openFailure("verify WAL mode", errors.New("SQLite WAL mode unavailable"))
	}
	// Verify write access without changing data; read-write opens can fall back
	// to read-only connections. No transaction remains open after startup.
	if _, err := db.ExecContext(ctx, "BEGIN IMMEDIATE; ROLLBACK"); err != nil {
		return nil, openFailure("check write access", err)
	}
	if err := migrate(ctx, db); err != nil {
		return nil, openFailure("migrate schema", err)
	}
	return db, nil
}

// requirePrivatePath uses metadata only: opening an existing SQLite database
// with a second descriptor could release its POSIX advisory locks on close.
func requirePrivatePath(path string, directory, required bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && !required {
		return nil
	}
	if err != nil {
		return err
	}
	if directory && !info.IsDir() || !directory && !info.Mode().IsRegular() {
		return errors.New("unsafe database path type")
	}
	if !privateStorage(info) {
		return errors.New("unsafe database path permissions")
	}
	return nil
}

type migration struct {
	version int
	sql     string
}

func readMigrations() ([]migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		number, title, hasTitle := strings.Cut(strings.TrimSuffix(name, ".sql"), "_")
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") || !hasTitle || title == "" || len(number) != 3 {
			return nil, errors.New("invalid migration filename")
		}
		version, err := strconv.Atoi(number)
		if err != nil || version != len(migrations)+1 {
			return nil, errors.New("migrations must have consecutive versions starting at 001")
		}
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}
		sql := string(body)
		if strings.TrimSpace(sql) == "" {
			return nil, errors.New("empty migration")
		}
		migrations = append(migrations, migration{version: version, sql: sql})
	}
	if len(migrations) == 0 {
		return nil, errors.New("no migrations embedded")
	}
	return migrations, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := readMigrations()
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version < 0 || version > len(migrations) {
		return fmt.Errorf("unsupported schema version %d", version)
	}
	for _, migration := range migrations[version:] {
		if _, err := tx.ExecContext(ctx, migration.sql); err != nil {
			return fmt.Errorf("apply migration %03d: %w", migration.version, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", migration.version)); err != nil {
			return fmt.Errorf("record migration %03d: %w", migration.version, err)
		}
	}
	return tx.Commit()
}
