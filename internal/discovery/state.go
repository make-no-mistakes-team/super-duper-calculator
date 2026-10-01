package discovery

import (
	"math"
	"strconv"
	"time"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

// State is bounded progress over accepted actions; it never stores history.
// Match reads the prior state, then Advance consumes exactly one new action.
type State struct {
	LastWasError     bool   `json:"lastWasError,omitempty"`
	SuccessStreak    int    `json:"successStreak,omitempty"`
	IncreasingStreak int    `json:"increasingStreak,omitempty"`
	LastValue        string `json:"lastValue,omitempty"`
	PreviousValue    string `json:"previousValue,omitempty"`
	PeerExpression   string `json:"peerExpression,omitempty"`
	PeerStreak       int    `json:"peerStreak,omitempty"`
	ErrorKinds       uint8  `json:"errorKinds,omitempty"`
}

// Evidence is the indexed history relevant to one successful parsed action.
// Routes contains at most three distinct parser-produced structures.
type Evidence struct {
	Routes   []string
	Trig     uint8
	Earliest time.Time
}

func numericValue(outcome contracts.Outcome) (float64, bool) {
	if outcome.Kind != contracts.OutcomeSuccess {
		return 0, false
	}
	value, err := strconv.ParseFloat(outcome.Value, 64)
	return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func errorKind(outcome contracts.Outcome) uint8 {
	if outcome.Kind != contracts.OutcomeError || outcome.Error == nil {
		return 0
	}
	switch outcome.Error.Code {
	case contracts.ErrorSyntax:
		return 1
	case contracts.ErrorDivisionByZero:
		return 2
	case contracts.ErrorDomain:
		return 4
	default:
		return 0
	}
}

// TrigVariant requires an actual parsed direct trigonometric function.
func TrigVariant(facts *contracts.CalculationFacts) uint8 {
	if facts == nil || (facts.Functions["sin"] <= 0 && facts.Functions["cos"] <= 0 && facts.Functions["tan"] <= 0) {
		return 0
	}
	if facts.TrigWithDegrees {
		return 1
	}
	if facts.Operators["°"] > 0 {
		return 0
	}
	return 2
}

func (s *State) Advance(record contracts.CalculationRecord) {
	s.ErrorKinds |= errorKind(record.Outcome)
	value, numeric := numericValue(record.Outcome)
	if record.Outcome.Kind != contracts.OutcomeSuccess {
		s.LastWasError = record.Outcome.Kind == contracts.OutcomeError
		s.SuccessStreak = 0
		s.IncreasingStreak = 0
		s.LastValue, s.PreviousValue = "", ""
		s.PeerExpression, s.PeerStreak = "", 0
		return
	}
	last, lastErr := strconv.ParseFloat(s.LastValue, 64)
	if numeric && lastErr == nil && value > last {
		s.IncreasingStreak = min(s.IncreasingStreak+1, 5)
	} else {
		s.IncreasingStreak = 1
	}
	s.SuccessStreak = min(s.SuccessStreak+1, 10)
	s.PreviousValue, s.LastValue = s.LastValue, record.Outcome.Value
	if s.PeerStreak > 0 && s.PeerExpression == record.Expression {
		s.PeerStreak++
	} else {
		s.PeerExpression, s.PeerStreak = record.Expression, 1
	}
	s.LastWasError = false
}
