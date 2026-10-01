package statistics_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/statistics"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func openStatisticsDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "calculations.sqlite")
	return reopenStatisticsDB(t, path), path
}

func reopenStatisticsDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func addOwner(t *testing.T, db *sql.DB, owner string) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), `INSERT INTO sessions (id, expires_at) VALUES (?, 4102444800)`, owner); err != nil {
		t.Fatal(err)
	}
}

func addCalculation(t *testing.T, db *sql.DB, owner, id, requestID, expression, outcome string, facts any) {
	t.Helper()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO calculations
			(id, session_id, request_id, expression, semantics_version, outcome_json, facts_json, created_at)
		VALUES (?, ?, ?, ?, 'historical-version', ?, ?, '2026-01-01T00:00:00Z')
		ON CONFLICT (session_id, request_id) DO NOTHING`, id, owner, requestID, expression, outcome, facts)
	if err != nil {
		t.Fatal(err)
	}
}

func recordCount(t *testing.T, db *sql.DB, owner string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM calculations WHERE session_id = ?`, owner).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestReadReconstructsOwnedHistoryAndSurvivesRestart(t *testing.T) {
	db, path := openStatisticsDB(t)
	addOwner(t, db, "alice")
	addOwner(t, db, "bob")
	addCalculation(t, db, "alice", "sum", "a1", "1+2+3", `{"kind":"success","value":"6"}`,
		`{"operators":{"+":2},"functions":{},"operationCount":2,"depth":0}`)
	addCalculation(t, db, "alice", "domain", "a2", "sqrt(-1)",
		`{"kind":"error","error":{"code":"DOMAIN_ERROR","stage":"evaluate"}}`,
		`{"operators":{},"functions":{"sqrt":1},"operationCount":1,"depth":1}`)
	addCalculation(t, db, "alice", "division", "a3", "8/0+sqrt(1)",
		`{"kind":"error","error":{"code":"DIVISION_BY_ZERO","stage":"evaluate"}}`,
		`{"operators":{"/":1,"+":1},"functions":{"sqrt":1},"operationCount":3,"depth":1}`)
	addCalculation(t, db, "alice", "syntax", "a4", "broken()",
		`{"kind":"error","error":{"code":"SYNTAX_ERROR","stage":"parse"}}`, nil)
	// Even if historical parse failures contain partial facts, those facts are not parsed usage.
	addCalculation(t, db, "alice", "emoji", "a5", "🙂🙂🙂🙂🙂🙂",
		`{"kind":"error","error":{"code":"SYNTAX_ERROR","stage":"parse"}}`,
		`{"operators":{"+":99},"functions":{"sqrt":99},"operationCount":198,"depth":99}`)
	addCalculation(t, db, "alice", "tie", "a6", "123456789012", `{"kind":"success","value":"123456789012"}`,
		`{"operators":{},"functions":{},"operationCount":0,"depth":0}`)
	addCalculation(t, db, "bob", "foreign", "b1", "1/0",
		`{"kind":"error","error":{"code":"DIVISION_BY_ZERO","stage":"evaluate"}}`,
		`{"operators":{"/":1},"functions":{},"operationCount":1,"depth":0}`)

	depth := 1
	want := contracts.PersonalStatistics{
		TotalCalculations: 6, Successes: 2, MathematicalErrors: 4, DivisionByZeroAttempts: 1,
		Operators: map[string]int64{"+": 3, "/": 1}, Functions: map[string]int64{"sqrt": 2},
		LongestExpression: &contracts.LongestExpression{CalculationID: "emoji", Expression: "🙂🙂🙂🙂🙂🙂", Length: 12},
		MaxParsedDepth:    &depth,
	}
	// A read must work when the physical SQLite connection explicitly disallows writes.
	if _, err := db.ExecContext(t.Context(), `PRAGMA query_only = ON`); err != nil {
		t.Fatal(err)
	}
	got, err := statistics.Read(t.Context(), db, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statistics = %+v, want %+v", got, want)
	}
	if n := recordCount(t, db, "alice"); n != 6 {
		t.Fatalf("read changed persisted history: %d records, want 6", n)
	}
	bob, err := statistics.Read(t.Context(), db, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if bob.TotalCalculations != 1 || bob.Successes != 0 || bob.MathematicalErrors != 1 ||
		bob.DivisionByZeroAttempts != 1 || bob.MaxParsedDepth == nil || *bob.MaxParsedDepth != 0 ||
		bob.Operators["+"] != 0 || bob.Operators["/"] != 1 {
		t.Fatalf("other owner's statistics = %+v", bob)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = reopenStatisticsDB(t, path)
	restored, err := statistics.Read(t.Context(), db, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, want) {
		t.Fatalf("statistics after restart = %+v, want %+v", restored, want)
	}
}

func TestReadEmptyAndHistoricalMissingFactsWithUniqueActions(t *testing.T) {
	db, _ := openStatisticsDB(t)
	addOwner(t, db, "owner")
	empty, err := statistics.Read(t.Context(), db, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if empty.TotalCalculations != 0 || empty.LongestExpression != nil || empty.MaxParsedDepth != nil ||
		empty.Operators == nil || empty.Functions == nil {
		t.Fatalf("empty statistics = %+v", empty)
	}
	encoded, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"operators", "functions"} {
		if string(fields[name]) != "{}" {
			t.Fatalf("empty %s serialized as %s, want {}", name, fields[name])
		}
	}
	for _, name := range []string{"longestExpression", "maxParsedDepth"} {
		if string(fields[name]) != "null" {
			t.Fatalf("empty %s serialized as %s, want null", name, fields[name])
		}
	}

	addCalculation(t, db, "owner", "first", "action-1", "42", `{"kind":"success","value":"42"}`, nil)
	addCalculation(t, db, "owner", "parse", "action-2", "?",
		`{"kind":"error","error":{"code":"SYNTAX_ERROR","stage":"parse"}}`, "null")
	withoutFacts, err := statistics.Read(t.Context(), db, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if withoutFacts.TotalCalculations != 2 || withoutFacts.Successes != 1 ||
		withoutFacts.MathematicalErrors != 1 || withoutFacts.MaxParsedDepth != nil ||
		withoutFacts.LongestExpression == nil || withoutFacts.LongestExpression.CalculationID != "first" {
		t.Fatalf("historical records without facts = %+v", withoutFacts)
	}

	// A network retry shares its action ID; a deliberate repeat with the same source does not.
	addCalculation(t, db, "owner", "retry", "action-1", "42", `{"kind":"success","value":"42"}`,
		`{"operators":{"+":7},"functions":{},"operationCount":7,"depth":7}`)
	addCalculation(t, db, "owner", "repeat", "action-3", "42", `{"kind":"success","value":"42"}`,
		`{"operators":{},"functions":{},"operationCount":0,"depth":0}`)
	result, err := statistics.Read(t.Context(), db, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCalculations != 3 || result.Successes != 2 || result.MathematicalErrors != 1 ||
		result.MaxParsedDepth == nil || *result.MaxParsedDepth != 0 ||
		result.LongestExpression == nil || result.LongestExpression.CalculationID != "first" ||
		len(result.Operators) != 0 || len(result.Functions) != 0 || recordCount(t, db, "owner") != 3 {
		t.Fatalf("unique accepted actions = %+v", result)
	}
	foreign, err := statistics.Read(t.Context(), db, "nonexistent-owner")
	if err != nil || foreign.TotalCalculations != 0 || foreign.LongestExpression != nil || foreign.MaxParsedDepth != nil {
		t.Fatalf("unowned history = %+v, error %v", foreign, err)
	}
}

func TestReadRejectsUnusablePersistedMetricsWithoutChangingHistory(t *testing.T) {
	const success = `{"kind":"success","value":"1"}`
	const parsed = `{"operators":{},"functions":{},"operationCount":0,"depth":0}`
	cases := []struct {
		name, outcome string
		facts         any
	}{
		{"malformed outcome", "{", parsed},
		{"missing outcome kind", "{}", parsed},
		{"error without code", `{"kind":"error","error":{"stage":"evaluate"}}`, parsed},
		{"malformed facts", success, "{"},
		{"negative operator usage", success, `{"operators":{"+":-1},"functions":{},"operationCount":0,"depth":0}`},
		{"negative function usage", success, `{"operators":{},"functions":{"sqrt":-1},"operationCount":0,"depth":0}`},
		{"negative operation count", success, `{"operators":{},"functions":{},"operationCount":-1,"depth":0}`},
		{"negative depth", success, `{"operators":{},"functions":{},"operationCount":0,"depth":-1}`},
		{"negative ignored parse facts", `{"kind":"error","error":{"code":"SYNTAX_ERROR","stage":"parse"}}`,
			`{"operators":{"+":-1},"functions":{},"operationCount":0,"depth":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := openStatisticsDB(t)
			addOwner(t, db, "owner")
			addCalculation(t, db, "owner", "sound", "a1", "1", success, parsed)
			addCalculation(t, db, "owner", "corrupt", "a2", "2", tc.outcome, tc.facts)
			result, err := statistics.Read(t.Context(), db, "owner")
			if err == nil {
				t.Fatalf("corrupt persisted metric returned %+v", result)
			}
			if n := recordCount(t, db, "owner"); n != 2 {
				t.Fatalf("corrupt statistics read changed history: %d records", n)
			}
		})
	}
}

func TestReadPropagatesCancellationDeadlineAndQueryErrors(t *testing.T) {
	db, _ := openStatisticsDB(t)
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := statistics.Read(cancelled, db, "owner"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read error = %v", err)
	}

	// storage.Open limits the pool to one connection. Holding it forces Read to
	// wait for the caller's deadline instead of waiting indefinitely for SQLite.
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	deadline, stop := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer stop()
	if _, err := statistics.Read(deadline, db, "owner"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired read error = %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := statistics.Read(t.Context(), db, "owner"); err == nil {
		t.Fatal("read through a closed database succeeded")
	}
}
