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
	"log"
	_ "embed"
	_ "modernc.org/sqlite"
)
//go:embed configuration.sql
var configuration string

// Open opens a file-backed SQLite database.
// An empty path uses data/calculator.sqlite. The caller must close the database
// after its requests and transactions have finished.
func Open(ctx context.Context, path string) (_ *sql.DB, err error) {
	var db *sql.DB
	err = ctx.Err(); if err != nil {
		return nil, err
	}
	if path == "" {
		path = "data/calculator.sqlite"
	}
	path, err = filepath.Abs(path); if err != nil {	
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	log.Printf("path resolved")
	_, file_stat_err := os.Stat(path);

	new_flag := errors.Is(file_stat_err, os.ErrNotExist)
	if new_flag {
		//create new .sqlite file
		log.Printf("create new file")
		err = os.MkdirAll(filepath.Dir(path), 0o700); if err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600); if err != nil {
			return nil, fmt.Errorf("create database file: %w", err)
		}
		err = file.Close(); if err != nil {
			return nil, fmt.Errorf("close new-created database file: %w", err)
		}
	}
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
	db, err = sql.Open("sqlite", dsn.String()); if err != nil {
		return nil, fmt.Errorf("configure SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	err = db.PingContext(ctx); if err != nil {
		return nil, fmt.Errorf("ping SQLite:%w", err)
	}
	log.Printf("sql ping success")
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return nil, fmt.Errorf("initialize SQLite: %w", err)
	}
	if journalMode != "wal" {
		return nil, fmt.Errorf("SQLite WAL mode is unavailable (journal mode %q)", journalMode)
	}
	log.Printf("sql wal checked")
	if _, err := db.ExecContext(ctx, "BEGIN IMMEDIATE; ROLLBACK"); err != nil {
		return nil, fmt.Errorf("check SQLite write access: %w", err)
	}
	log.Printf("sql write access checked")
	if new_flag{
		tx, err := db.BeginTx(ctx,nil); if err != nil {
			return nil, fmt.Errorf("begin transaction: %w", err)
		}
		_, err = tx.ExecContext(ctx, configuration); if err != nil {
			return nil, fmt.Errorf("configure database: %w", err) //maybe rollback
		}
		err = tx.Commit(); if err != nil {
			return nil, fmt.Errorf("commit transaction: %w", err)
		}
		log.Printf("sql database configuration finished")
	}else{
		rows, err := db.QueryContext(ctx, "SELECT name, sql FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%';"); if err!=nil {
			return nil, fmt.Errorf("get scheme: %w", err)
		}
		actual_config:=""
		for rows.Next() {
			var name, sqlText string
			err = rows.Scan(&name, &sqlText); if err!=nil{
				return nil, fmt.Errorf("parse row: %w", err)
			}
			actual_config = actual_config+sqlText+";\n"
		}
		if actual_config != configuration {
			return nil, fmt.Errorf("config difference: '%w'", actual_config)	
		}
		log.Printf("sql configuration verified")
	}
	defer func() {
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close SQLite: %w", closeErr))
			}
		}
	}()
	return db, nil;
}
