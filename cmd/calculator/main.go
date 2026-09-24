package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	publicOrigin, err := parsePublicOrigin(os.Getenv("PUBLIC_ORIGIN"))
	if err != nil {
		return errors.New("invalid PUBLIC_ORIGIN")
	}
	statisticsEnabled := true
	if value := os.Getenv("STATISTICS_ENABLED"); value != "" {
		statisticsEnabled, err = strconv.ParseBool(value)
		if err != nil {
			return errors.New("invalid STATISTICS_ENABLED")
		}
	}
	achievementsEnabled := true
	if value := os.Getenv("ACHIEVEMENTS_ENABLED"); value != "" {
		achievementsEnabled, err = strconv.ParseBool(value)
		if err != nil {
			return errors.New("invalid ACHIEVEMENTS_ENABLED")
		}
	}
	openCtx, cancelOpen := context.WithTimeout(ctx, 10*time.Second)
	db, err := storage.Open(openCtx, os.Getenv("DATABASE_PATH"))
	cancelOpen()
	if err != nil {
		return fmt.Errorf("database startup: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			runErr = errors.Join(runErr, errors.New("database close failed"))
		}
	}()

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	handler, err := withClient(newHandler(db, publicOrigin, statisticsEnabled, achievementsEnabled), os.Getenv("WEB_ASSETS_DIR"))
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		ErrorLog:          log.New(safeServerLog{}, "", 0),
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.New("HTTP listen failed")
	}
	listener = limitConnections(listener, 128)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()
	log.Printf("HTTP listening on http://%s", listener.Addr())

	var serveResult error
	var serveDone bool
	select {
	case serveResult = <-serveErr:
		serveDone = true
	case <-ctx.Done():
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return errors.New("HTTP shutdown failed")
	}
	if !serveDone {
		serveResult = <-serveErr
	}
	if !errors.Is(serveResult, http.ErrServerClosed) {
		return errors.New("HTTP serve failed")
	}
	return nil
}

func readiness(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var schemaVersion int
		if err := db.QueryRowContext(ctx, "PRAGMA schema_version").Scan(&schemaVersion); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "{\"status\":\"unavailable\"}\n")
			return
		}
		_, _ = io.WriteString(w, "{\"status\":\"ok\"}\n")
	}
}
