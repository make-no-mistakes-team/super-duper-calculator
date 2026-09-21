package calculation_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
)

func evaluate(t *testing.T, expression string) calculation.Evaluation {
	t.Helper()
	ev, err := calculation.New().Evaluate(t.Context(), calculation.Input{Expression: expression, AngleUnit: contracts.Degrees})
	if err != nil {
		t.Fatalf("%q: unexpected service error %v", expression, err)
	}
	return ev
}

// Arithmetic part of the mathematical corpus (specs/quality-and-testing.md).
func TestArithmeticSuccess(t *testing.T) {
	for _, tc := range []struct{ expression, value string }{
		// Precedence and associativity.
		{"2+3*4", "14"},
		{"(2+3)*4", "20"},
		{"8/4*2", "4"},
		{"2^3^2", "512"},
		{"-2^2", "-4"},
		{"(-2)^2", "4"},
		{"2^-3", "0.125"},
		{"2^-2^2", "0.0625"},
		{"10-4-3", "3"},
		{"2*-3", "-6"},
		{"--2", "2"},
		{"-+-2", "2"},
		{"-(2+3)", "-5"},

		// Literals.
		{"12", "12"},
		{"12.5", "12.5"},
		{".5", "0.5"},
		{"5.", "5"},
		{"1.25e-3", "0.00125"},
		{"1e+2", "100"},
		{".5+1.25e-3", "0.50125"},

		// Whitespace between tokens does not change the result.
		{" 2 + 3 * 4 ", "14"},
		{"2\t+\n3\r\n*4", "14"},
		{"( 2+3 )*4", "20"},
		{"2 ^ - 3", "0.125"},

		// Numerical model.
		{"0^0", "1"},
		{"0.1+0.2", "0.30000000000000004"},
		{"-0", "0"},
		{"0*-1", "0"},
		{"1e-400", "0"},
	} {
		ev := evaluate(t, tc.expression)
		if ev.Outcome.Kind != "success" || ev.Outcome.Value != tc.value {
			t.Errorf("%q = %+v, want %s", tc.expression, ev.Outcome, tc.value)
		}
	}
}

func TestArithmeticErrors(t *testing.T) {
	for _, tc := range []struct {
		expression, code, stage string
		span                    contracts.SourceSpan
	}{
		// Evaluation errors point at the failed operation.
		{"1/0", "DIVISION_BY_ZERO", "evaluate", contracts.SourceSpan{Start: 0, End: 3}},
		{"1/-0", "DIVISION_BY_ZERO", "evaluate", contracts.SourceSpan{Start: 0, End: 4}},
		{"0^-1", "DIVISION_BY_ZERO", "evaluate", contracts.SourceSpan{Start: 0, End: 4}},
		{"(1/0)+2", "DIVISION_BY_ZERO", "evaluate", contracts.SourceSpan{Start: 1, End: 4}},
		{"(-8)^(1/3)", "DOMAIN_ERROR", "evaluate", contracts.SourceSpan{Start: 0, End: 10}},
		{"1e308*10", "NUMERIC_OVERFLOW", "evaluate", contracts.SourceSpan{Start: 0, End: 8}},
		{"1e400", "NUMERIC_OVERFLOW", "parse", contracts.SourceSpan{Start: 0, End: 5}},

		// Incomplete input and trailing text never succeed on a valid prefix.
		{"", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 0, End: 0}},
		{"5*(", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 3, End: 3}},
		{"2+(", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 3, End: 3}},
		{"(2+3", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 4, End: 4}},
		{"2+", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 2, End: 2}},
		{"2+)", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 2, End: 2}},
		{"2)", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 1, End: 2}},
		{"2+3 trailing", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 4, End: 12}},
		{"*2", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 0, End: 1}},

		// No implicit multiplication.
		{"2pi", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 1, End: 2}},
		{"2(3+4)", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 1, End: 2}},
		{"(1)(2)", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 3, End: 4}},

		// A number broken by whitespace is not a new spelling of a number.
		{"1 2", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 2, End: 3}},
		{"1 .5", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 2, End: 4}},
		{"1. 5", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 3, End: 4}},
		{"1e 5", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 1, End: 2}},
		{"1.2.3", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 3, End: 4}},
		{".", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 0, End: 1}},

		// Decimal commas, assignment, variables, strings, property access, calls.
		{"1,5", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 1, End: 2}},
		{"x=1", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 1, End: 2}},
		{"2=2", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 1, End: 2}},
		{`"2"`, "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 0, End: 1}},
		{"a.b", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 1, End: 2}},
		{"f(1)", "UNKNOWN_IDENTIFIER", "parse", contracts.SourceSpan{Start: 0, End: 1}},
	} {
		ev := evaluate(t, tc.expression)
		e := ev.Outcome.Error
		if ev.Outcome.Kind != "error" || e == nil {
			t.Errorf("%q = %+v, want %s", tc.expression, ev.Outcome, tc.code)
			continue
		}
		if e.Code != tc.code || e.Stage != tc.stage || e.Span == nil || *e.Span != tc.span {
			t.Errorf("%q = %s/%s span %v, want %s/%s span %v", tc.expression, e.Code, e.Stage, e.Span, tc.code, tc.stage, tc.span)
		}
	}
}

func TestFacts(t *testing.T) {
	for _, tc := range []struct {
		expression string
		facts      contracts.CalculationFacts
	}{
		{"7", contracts.CalculationFacts{Operators: map[string]int{}, OperationCount: 0, Depth: 0}},
		{"-7", contracts.CalculationFacts{Operators: map[string]int{}, OperationCount: 0, Depth: 0}},
		{"+7", contracts.CalculationFacts{Operators: map[string]int{}, OperationCount: 0, Depth: 0}},
		{"2+3*4", contracts.CalculationFacts{Operators: map[string]int{"+": 1, "*": 1}, OperationCount: 2, Depth: 0}},
		{"2^3^2", contracts.CalculationFacts{Operators: map[string]int{"^": 2}, OperationCount: 2, Depth: 0}},
		{"1-2-3", contracts.CalculationFacts{Operators: map[string]int{"-": 2}, OperationCount: 2, Depth: 0}},
		// Depth counts parentheses, including ones the tree does not need.
		{"(1)", contracts.CalculationFacts{Operators: map[string]int{}, OperationCount: 0, Depth: 1}},
		{"((1+2))*3", contracts.CalculationFacts{Operators: map[string]int{"+": 1, "*": 1}, OperationCount: 2, Depth: 2}},
		{"(1+(2*(3-4)))/(5)", contracts.CalculationFacts{Operators: map[string]int{"+": 1, "*": 1, "-": 1, "/": 1}, OperationCount: 4, Depth: 3}},
		// Facts come with a mathematical error.
		{"(1/0)+2", contracts.CalculationFacts{Operators: map[string]int{"/": 1, "+": 1}, OperationCount: 2, Depth: 1}},
	} {
		ev := evaluate(t, tc.expression)
		want := tc.facts
		want.Functions = map[string]int{}
		if ev.Facts == nil || !reflect.DeepEqual(*ev.Facts, want) {
			t.Errorf("%q facts = %+v, want %+v", tc.expression, ev.Facts, want)
		}
	}
}

// Unparsed text does not produce invented facts.
func TestNoFactsWithoutParse(t *testing.T) {
	for _, expression := range []string{"2+(", "2+3 trailing", "2(3+4)", "1/0 +", "%"} {
		if ev := evaluate(t, expression); ev.Facts != nil {
			t.Errorf("%q facts = %+v, want none", expression, *ev.Facts)
		}
	}
}

func TestLimits(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		limited          bool
	}{
		{"256 tokens", strings.Repeat("1+", 127) + "1", false},
		{"257 tokens", strings.Repeat("1+", 128) + "1", true},
		{"32 levels", strings.Repeat("(", 32) + "1" + strings.Repeat(")", 32), false},
		{"33 levels", strings.Repeat("(", 33) + "1" + strings.Repeat(")", 33), true},
		{"long input", strings.Repeat(" ", 1025) + "1", true},
	} {
		ev, err := calculation.New().Evaluate(t.Context(), calculation.Input{Expression: tc.expression, AngleUnit: contracts.Degrees})
		switch {
		case tc.limited && !errors.Is(err, calculation.ErrExpressionLimit):
			t.Errorf("%s: err = %v, outcome %+v, want ErrExpressionLimit", tc.name, err, ev.Outcome)
		case !tc.limited && (err != nil || ev.Outcome.Kind != "success"):
			t.Errorf("%s: err = %v, outcome %+v, want success", tc.name, err, ev.Outcome)
		}
	}
}

func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := calculation.New().Evaluate(ctx, calculation.Input{Expression: "1+1"}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
