package storage_test

import (
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/storage"
)

func TestCalculationRecordFactsPolicies(t *testing.T) {
	db, err := storage.Open(t.Context(), privateDatabasePath(t, "record-facts.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 123, time.FixedZone("source", 3600))
	saveAwardAction(t, db, "owner", "action", createdAt)
	valid := &contracts.CalculationFacts{
		Depth: 2, OperationCount: 3,
		Operators: map[string]int{"+": 2}, Functions: map[string]int{"sqrt": 1},
	}
	for _, test := range []struct {
		name           string
		facts          any
		strictError    bool
		strictFacts    *contracts.CalculationFacts
		discoveryFacts *contracts.CalculationFacts
	}{
		{name: "SQL null", facts: nil},
		{name: "JSON null", facts: "null"},
		{name: "valid", facts: `{"depth":2,"operationCount":3,"operators":{"+":2},"functions":{"sqrt":1}}`, strictFacts: valid, discoveryFacts: valid},
		{name: "broken JSON", facts: "broken", strictError: true},
		{name: "wrong field type", facts: `{"depth":"invalid"}`, strictError: true},
		{name: "negative depth", facts: `{"depth":-1}`, strictFacts: &contracts.CalculationFacts{Depth: -1}},
		{name: "negative operation count", facts: `{"operationCount":-1}`, strictFacts: &contracts.CalculationFacts{OperationCount: -1}},
		{name: "negative operator count", facts: `{"operators":{"+":-1}}`, strictFacts: &contracts.CalculationFacts{Operators: map[string]int{"+": -1}}},
		{name: "negative function count", facts: `{"functions":{"sqrt":-1}}`, strictFacts: &contracts.CalculationFacts{Functions: map[string]int{"sqrt": -1}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := db.ExecContext(t.Context(), "UPDATE calculations SET facts_json = ? WHERE id = 'action'", test.facts); err != nil {
				t.Fatal(err)
			}
			for _, policy := range []storage.FactsPolicy{storage.StrictFacts, storage.DiscoveryFacts} {
				// Exercise both the single-row replay and sequenced history paths.
				for _, sequenced := range []bool{false, true} {
					columns := storage.CalculationRecordColumns
					var seq int64
					var sequence *int64
					if sequenced {
						columns = storage.SequencedCalculationRecordColumns
						sequence = &seq
					}
					record, err := storage.ReadCalculationRecord(db.QueryRowContext(t.Context(),
						"SELECT "+columns+" FROM calculations WHERE id = 'action'"), sequence, policy)
					wantError := policy == storage.StrictFacts && test.strictError
					if (err != nil) != wantError {
						t.Fatalf("policy %v, sequenced %v: error = %v, want error %v", policy, sequenced, err, wantError)
					}
					if wantError {
						continue
					}
					wantFacts := test.strictFacts
					if policy == storage.DiscoveryFacts {
						wantFacts = test.discoveryFacts
					}
					if !reflect.DeepEqual(record.Facts, wantFacts) {
						t.Fatalf("policy %v: facts = %+v, want %+v", policy, record.Facts, wantFacts)
					}
					if record.ID != "action" || record.RequestID != "action" || record.Expression != "((((((60+7))))))" ||
						record.Context.SemanticsVersion != "binary64-v1" ||
						record.Outcome.Kind != contracts.OutcomeSuccess || record.Outcome.Value != "67" || !record.CreatedAt.Equal(createdAt) ||
						(sequenced && seq != 1) {
						t.Fatalf("optional facts changed authoritative record: %+v, sequence %d", record, seq)
					}
				}
			}
		})
	}
}

func TestDiscoveryRecordReaderKeepsAuthoritativeFieldsStrict(t *testing.T) {
	db, err := storage.Open(t.Context(), privateDatabasePath(t, "record-source.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	saveAwardAction(t, db, "owner", "action", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	for _, test := range []struct {
		name       string
		projection string
	}{
		{"source", "id, request_id, NULL AS expression, semantics_version, outcome_json, facts_json, created_at"},
		{"outcome", "id, request_id, expression, semantics_version, 'broken' AS outcome_json, facts_json, created_at"},
		{"timestamp", "id, request_id, expression, semantics_version, outcome_json, facts_json, 'broken' AS created_at"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, policy := range []storage.FactsPolicy{storage.StrictFacts, storage.DiscoveryFacts} {
				if _, err := storage.ReadCalculationRecord(db.QueryRowContext(t.Context(),
					"SELECT "+test.projection+" FROM calculations WHERE id = 'action'"), nil, policy); err == nil {
					t.Fatalf("policy %v tolerated invalid %s", policy, test.name)
				}
			}
		})
	}
	if _, err := storage.ReadCalculationRecord(db.QueryRowContext(t.Context(),
		"SELECT "+storage.CalculationRecordColumns+" FROM calculations WHERE id = 'missing'"), nil, storage.DiscoveryFacts); err != sql.ErrNoRows {
		t.Fatalf("missing action error = %v, want sql.ErrNoRows", err)
	}
}
