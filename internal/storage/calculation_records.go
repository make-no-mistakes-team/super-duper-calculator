package storage

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

// CalculationRecordColumns is the persisted record projection, shared by replay,
// history and discovery queries. Sequence is pagination/progress metadata only.
const CalculationRecordColumns = "id, request_id, expression, semantics_version, outcome_json, facts_json, created_at"
const SequencedCalculationRecordColumns = "seq, " + CalculationRecordColumns

// RecordScanner is satisfied by both a single SQL row and an open rows cursor.
type RecordScanner interface {
	Scan(...any) error
}

// FactsPolicy distinguishes mandatory history/replay decoding from optional
// discovery eligibility metadata. Authoritative fields are strict in both modes.
type FactsPolicy uint8

const (
	StrictFacts FactsPolicy = iota
	DiscoveryFacts
)

type rawCalculationRecord struct {
	record      contracts.CalculationRecord
	outcomeJSON string
	factsJSON   sql.NullString
	createdAt   string
}

// ReadCalculationRecord reads the shared projection. A non-nil sequence requires
// SequencedCalculationRecordColumns; otherwise use CalculationRecordColumns.
// DiscoveryFacts discards malformed or negative optional facts, never source,
// outcome or timestamp errors. StrictFacts retains ordinary replay/history policy.
func ReadCalculationRecord(row RecordScanner, sequence *int64, policy FactsPolicy) (contracts.CalculationRecord, error) {
	var raw rawCalculationRecord
	var err error
	if sequence == nil {
		err = row.Scan(&raw.record.ID, &raw.record.RequestID, &raw.record.Expression,
			&raw.record.Context.SemanticsVersion,
			&raw.outcomeJSON, &raw.factsJSON, &raw.createdAt)
	} else {
		err = row.Scan(sequence, &raw.record.ID, &raw.record.RequestID, &raw.record.Expression,
			&raw.record.Context.SemanticsVersion,
			&raw.outcomeJSON, &raw.factsJSON, &raw.createdAt)
	}
	if err != nil {
		return raw.record, err
	}
	if err := json.Unmarshal([]byte(raw.outcomeJSON), &raw.record.Outcome); err != nil {
		return raw.record, err
	}
	if raw.factsJSON.Valid {
		var facts *contracts.CalculationFacts
		err := json.Unmarshal([]byte(raw.factsJSON.String), &facts)
		if policy == DiscoveryFacts {
			if err == nil && validDiscoveryFacts(facts) {
				raw.record.Facts = facts
			}
		} else {
			if err != nil {
				return raw.record, err
			}
			raw.record.Facts = facts
		}
	}
	raw.record.CreatedAt, err = time.Parse(time.RFC3339Nano, raw.createdAt)
	return raw.record, err
}

func validDiscoveryFacts(facts *contracts.CalculationFacts) bool {
	if facts == nil || facts.Depth < 0 || facts.OperationCount < 0 {
		return false
	}
	for _, count := range facts.Operators {
		if count < 0 {
			return false
		}
	}
	for _, count := range facts.Functions {
		if count < 0 {
			return false
		}
	}
	for _, identity := range []string{facts.NormalizedExpression, facts.StructureIdentity} {
		if identity == "" {
			continue
		}
		if len(identity) != 64 {
			return false
		}
		for _, c := range identity {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	return true
}
