package calculation_test

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strconv"
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
		expression string
		code       contracts.MathErrorCode
		stage      contracts.ErrorStage
		span       contracts.SourceSpan
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

func TestMalformedLeadingIdentifierPointsAtStart(t *testing.T) {
	for _, tc := range []struct {
		expression string
		span       contracts.SourceSpan
	}{
		{"a]", contracts.SourceSpan{Start: 0, End: 1}},
		{"  a]", contracts.SourceSpan{Start: 2, End: 3}},
		{"asdasdfasdasfasfasfASFASFASFAFASFASFASFAFASFASFASFASFASFASFASFASFASFdf]asd]gla]hdgasd[gksdgdsgsd DeG",
			contracts.SourceSpan{Start: 0, End: 1}},
	} {
		outcome := evaluate(t, tc.expression).Outcome
		if outcome.Kind != contracts.OutcomeError || outcome.Error == nil || outcome.Error.Code != contracts.ErrorUnknownIdentifier ||
			outcome.Error.Stage != contracts.StageParse || outcome.Error.Span == nil || *outcome.Error.Span != tc.span {
			if outcome.Error == nil {
				t.Errorf("%q: outcome = %+v, want unknown identifier at %v", tc.expression, outcome, tc.span)
			} else {
				t.Errorf("%q: code=%s params=%v span=%v, want unknown identifier at %v", tc.expression,
					outcome.Error.Code, outcome.Error.Params, outcome.Error.Span, tc.span)
			}
		}
	}
	known := evaluate(t, "sin]").Outcome.Error
	if known == nil || known.Code != contracts.ErrorSyntax || known.Span == nil ||
		*known.Span != (contracts.SourceSpan{Start: 3, End: 4}) {
		t.Errorf("known function followed by ] = %+v, want syntax error at ]", known)
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

func TestUTF16ExpressionLimit(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		code             contracts.MathErrorCode
		limited          bool
		span             contracts.SourceSpan
	}{
		{"multibyte character under limit", strings.Repeat("é", 600), "SYNTAX_ERROR", false, contracts.SourceSpan{Start: 0, End: 1}},
		{"BMP character at limit", strings.Repeat(" ", 1023) + "é", "SYNTAX_ERROR", false, contracts.SourceSpan{Start: 1023, End: 1024}},
		{"surrogate pair at limit", strings.Repeat(" ", 1022) + "😀", "SYNTAX_ERROR", false, contracts.SourceSpan{Start: 1022, End: 1024}},
		{"surrogate pair over limit", strings.Repeat(" ", 1023) + "😀", "", true, contracts.SourceSpan{}},
		{"NUL over limit", strings.Repeat("\x00", 1025), "", true, contracts.SourceSpan{}},
	} {
		ev, err := calculation.New().Evaluate(t.Context(), calculation.Input{Expression: tc.expression, AngleUnit: contracts.Degrees})
		if tc.limited {
			if !errors.Is(err, calculation.ErrExpressionLimit) {
				t.Errorf("%s: err = %v, want ErrExpressionLimit", tc.name, err)
			}
		} else if err != nil || ev.Outcome.Error == nil || ev.Outcome.Error.Code != tc.code ||
			ev.Outcome.Error.Span == nil || *ev.Outcome.Error.Span != tc.span {
			t.Errorf("%s: err = %v, outcome %+v, want %s at %v", tc.name, err, ev.Outcome, tc.code, tc.span)
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

func evaluateIn(t *testing.T, expression string, unit contracts.AngleUnit) calculation.Evaluation {
	t.Helper()
	ev, err := calculation.New().Evaluate(t.Context(), calculation.Input{Expression: expression, AngleUnit: unit})
	if err != nil {
		t.Fatalf("%q in %s: unexpected service error %v", expression, unit, err)
	}
	return ev
}

// Scientific part of the mathematical corpus: transcendental values are
// compared with a tolerance, which never turns a small result into zero.
func TestScientificSuccess(t *testing.T) {
	const deg, rad = contracts.Degrees, contracts.Radians
	nearPole := func(x float64) float64 { return -1 / math.Tan((x-90)*math.Pi/180) } // binary64, not exact constants
	for _, tc := range []struct {
		expression string
		unit       contracts.AngleUnit
		want       float64
	}{
		{"sqrt(81)+2^3", deg, 17},
		{"sqrt(0)", deg, 0},
		{"sqrt(2)^2", deg, 2}, // a function takes the power: sqrt(2^2)
		{"abs(-3)", deg, 3},
		{"abs(0)", deg, 0},
		{"exp(0)", deg, 1},
		{"exp(1)", deg, math.E},
		{"exp(-1000)", deg, 0}, // underflow may round to zero
		{"ln(e)", deg, 1},
		{"ln(1)", deg, 0},
		{"log(100)", deg, 2},
		{"log(1)", deg, 0},
		{"log(8,2)", deg, 3},
		{"log(8, 0.5)", deg, -3},
		{"log(1, 2)", deg, 0},
		{"log(0.5, 2)", deg, -1},
		{"pi", deg, math.Pi},
		{"pi", rad, math.Pi},
		{"Pi", deg, math.Pi},
		{"e", deg, math.E},
		{"2*pi", deg, 2 * math.Pi},
		{"SIN(30)", deg, 0.5},
		{"Log(8,2)", deg, 3},

		// Parentheses around a single argument are optional.
		{"sqrt 16", deg, 4},
		{"sin 30", deg, 0.5},
		{"log 8, 2", deg, 3},
		{"-sqrt 4", deg, -2},

		// Nested calls: a comma goes to the function that takes it.
		{"sqrt(sqrt(16))", deg, 2},
		{"log(abs(-8), 2)", deg, 3},
		{"log(sin(30)*16, 2)", deg, 3},
		{"log(2, abs(8))", deg, 1.0 / 3},
		{"log((8), 2)", deg, 3},
		{"log(8+8, 2+2)", deg, 2},
		{"log(sqrt 64, 2)", deg, 3},
		{"log(log(8, 2), 3)", deg, 1},
		{"log (log 100), 2", deg, 1}, // parentheses finish the inner log
		{"log (log(100)), 2", deg, 1},
		{"log ((log 100)), 2", deg, 1},
		{"log (log 8, 2), 3", deg, 1},
		{"log (log 100)", deg, math.Log10(2)},
		{"log(2^3, 2)", deg, 3},

		// Degrees.
		{"sin(30)", deg, 0.5},
		{"sin(90)", deg, 1},
		{"sin(180)", deg, 0},
		{"sin(-30)", deg, -0.5},
		{"sin(390)", deg, 0.5},
		{"cos(60)", deg, 0.5},
		{"cos(90)", deg, 0},
		{"cos(180)", deg, -1},
		{"tan(45)", deg, 1},
		{"tan(135)", deg, -1},
		{"tan(180)", deg, 0},
		{"asin(1)", deg, 90},
		{"asin(-1)", deg, -90},
		{"acos(-1)", deg, 180},
		{"acos(1)", deg, 0},
		{"atan(1)", deg, 45},

		// Radians, including inverse functions returning radians.
		{"sin(pi/2)", rad, 1},
		{"sin(30)", rad, math.Sin(30)},
		{"cos(pi)", rad, -1},
		{"tan(pi/4)", rad, 1},
		{"asin(1)", rad, math.Pi / 2},
		{"acos(-1)", rad, math.Pi},
		{"atan(1)", rad, math.Pi / 4},

		// Near a tangent pole the result is large, not an error.
		{"tan(89.9999)", deg, nearPole(89.9999)},
		{"tan(pi/2 - 1e-10)", rad, math.Tan(math.Pi/2 - 1e-10)},
	} {
		ev := evaluateIn(t, tc.expression, tc.unit)
		got, err := strconv.ParseFloat(ev.Outcome.Value, 64)
		if ev.Outcome.Kind != "success" || err != nil || math.Abs(got-tc.want) > 1e-12*max(1, math.Abs(tc.want)) {
			t.Errorf("%q in %s = %+v, want %v", tc.expression, tc.unit, ev.Outcome, tc.want)
		}
	}
}

// Small valid results survive exactly: no global rounding to zero.
func TestScientificSmallResults(t *testing.T) {
	for _, tc := range []struct {
		expression string
		unit       contracts.AngleUnit
		value      string
	}{
		{"sin(1e-300)", contracts.Radians, "1e-300"},
		{"tan(1e-200)", contracts.Radians, "1e-200"},
		{"sin(pi)", contracts.Radians, strconv.FormatFloat(math.Sin(math.Pi), 'g', -1, 64)},
		{"cos(pi/2)", contracts.Radians, strconv.FormatFloat(math.Cos(math.Pi/2), 'g', -1, 64)},
		{"sin(1e-10)", contracts.Degrees, strconv.FormatFloat(math.Sin(1e-10*math.Pi/180), 'g', -1, 64)},
		{"asin(1e-300)", contracts.Radians, "1e-300"},
	} {
		if ev := evaluateIn(t, tc.expression, tc.unit); ev.Outcome.Kind != "success" || ev.Outcome.Value != tc.value {
			t.Errorf("%q in %s = %+v, want %s", tc.expression, tc.unit, ev.Outcome, tc.value)
		}
	}
}

func TestScientificErrors(t *testing.T) {
	const deg, rad = contracts.Degrees, contracts.Radians
	for _, tc := range []struct {
		expression string
		unit       contracts.AngleUnit
		code       contracts.MathErrorCode
		name       string // WRONG_ARITY and UNKNOWN_IDENTIFIER params
	}{
		{"sqrt(-1)", deg, "DOMAIN_ERROR", ""},
		{"sqrt(-1e-300)", deg, "DOMAIN_ERROR", ""},
		{"ln(0)", deg, "DOMAIN_ERROR", ""},
		{"ln(-1)", deg, "DOMAIN_ERROR", ""},
		{"log(0)", deg, "DOMAIN_ERROR", ""},
		{"log(-1)", deg, "DOMAIN_ERROR", ""},
		{"log(0, 2)", deg, "DOMAIN_ERROR", ""},
		{"log(8, 1)", deg, "DOMAIN_ERROR", ""},
		{"log(8, 0)", deg, "DOMAIN_ERROR", ""},
		{"log(8, -2)", deg, "DOMAIN_ERROR", ""},
		{"asin(1.0000001)", deg, "DOMAIN_ERROR", ""},
		{"asin(-1.0000001)", rad, "DOMAIN_ERROR", ""},
		{"acos(2)", deg, "DOMAIN_ERROR", ""},
		{"acos(-2)", rad, "DOMAIN_ERROR", ""},
		{"exp(1000)", deg, "NUMERIC_OVERFLOW", ""},
		{"exp(exp(10))", deg, "NUMERIC_OVERFLOW", ""},

		// Tangent poles in both units, even when math.Tan is finite.
		{"tan(90)", deg, "DOMAIN_ERROR", ""},
		{"tan(-90)", deg, "DOMAIN_ERROR", ""},
		{"tan(270)", deg, "DOMAIN_ERROR", ""},
		{"tan(450)", deg, "DOMAIN_ERROR", ""},
		{"tan(-630)", deg, "DOMAIN_ERROR", ""},
		{"tan(90+360*1e6)", deg, "DOMAIN_ERROR", ""},
		{"tan(pi/2)", rad, "DOMAIN_ERROR", ""},
		{"tan(-pi/2)", rad, "DOMAIN_ERROR", ""},
		{"tan(3*pi/2)", rad, "DOMAIN_ERROR", ""},
		{"tan(5*pi/2)", rad, "DOMAIN_ERROR", ""},
		{"tan(101*pi/2)", rad, "DOMAIN_ERROR", ""},

		// Unknown and translated names and decimal commas are not syntax.
		{"unknown(1)", deg, "UNKNOWN_IDENTIFIER", "unknown"},
		{"UNKNOWN(1)", deg, "UNKNOWN_IDENTIFIER", "unknown"},
		{"sinus(30)", deg, "UNKNOWN_IDENTIFIER", "sinus"},
		{"sinx", deg, "UNKNOWN_IDENTIFIER", "sinx"},
		{"pie", deg, "UNKNOWN_IDENTIFIER", "pie"},
		{"синус(30)", deg, "SYNTAX_ERROR", ""},
		{"sin(0,5)", deg, "WRONG_ARITY", "sin"},
		{"2*pi(1)", deg, "SYNTAX_ERROR", ""},
		{"2sin(30)", deg, "SYNTAX_ERROR", ""},
		{"sin(30)2", deg, "SYNTAX_ERROR", ""},

		// Arity.
		{"sin(1,2)", deg, "WRONG_ARITY", "sin"},
		{"sqrt(1,2)", deg, "WRONG_ARITY", "sqrt"},
		{"atan(1,2)", deg, "WRONG_ARITY", "atan"},
		{"log(1,2,3)", deg, "WRONG_ARITY", "log"},
		{"sin 1, 2", deg, "WRONG_ARITY", "sin"},
		{"log(sin(1,2), 3)", deg, "WRONG_ARITY", "sin"},
		{"log(sin(1), 2, 3)", deg, "WRONG_ARITY", "log"},
		{"log (log 100), 2, 3", deg, "WRONG_ARITY", "log"},
		{"sin()", deg, "SYNTAX_ERROR", ""},
		{"log(8,)", deg, "SYNTAX_ERROR", ""},
		{"log(,2)", deg, "SYNTAX_ERROR", ""},
		{"sin", deg, "SYNTAX_ERROR", ""},
		{"(8, 2)", deg, "SYNTAX_ERROR", ""},
		{"log((8, 2))", deg, "SYNTAX_ERROR", ""},
		{"1, 2", deg, "SYNTAX_ERROR", ""},
	} {
		ev := evaluateIn(t, tc.expression, tc.unit)
		e := ev.Outcome.Error
		if ev.Outcome.Kind != "error" || e == nil || e.Code != tc.code {
			t.Errorf("%q in %s = %+v %+v, want %s", tc.expression, tc.unit, ev.Outcome, e, tc.code)
			continue
		}
		if tc.name != "" && e.Params["name"] != tc.name {
			t.Errorf("%q: params %v, want name %q", tc.expression, e.Params, tc.name)
		}
	}
}

// Evaluation errors of functions point at the whole call.
func TestScientificErrorSpans(t *testing.T) {
	for _, tc := range []struct {
		expression string
		code       contracts.MathErrorCode
		stage      contracts.ErrorStage
		span       contracts.SourceSpan
	}{
		{"sqrt(-1)", "DOMAIN_ERROR", "evaluate", contracts.SourceSpan{Start: 0, End: 8}},
		{"1+ln(0)", "DOMAIN_ERROR", "evaluate", contracts.SourceSpan{Start: 2, End: 7}},
		{"2*exp(1000)", "NUMERIC_OVERFLOW", "evaluate", contracts.SourceSpan{Start: 2, End: 11}},
		{"log(8, 1)", "DOMAIN_ERROR", "evaluate", contracts.SourceSpan{Start: 0, End: 9}},
		{"sin(1,2)", "WRONG_ARITY", "parse", contracts.SourceSpan{Start: 5, End: 6}},
		{"unknown(1)", "UNKNOWN_IDENTIFIER", "parse", contracts.SourceSpan{Start: 0, End: 7}},
	} {
		ev := evaluate(t, tc.expression)
		e := ev.Outcome.Error
		if e == nil || e.Code != tc.code || e.Stage != tc.stage || e.Span == nil || *e.Span != tc.span {
			t.Errorf("%q = %+v, want %s/%s span %v", tc.expression, e, tc.code, tc.stage, tc.span)
		}
	}
}

func TestFactorialSuccess(t *testing.T) {
	for _, tc := range []struct{ expression, value string }{
		{"0!", "1"},
		{"5!", "120"},
		{"3.0!", "6"},
		{"(2+3)!", "120"},
		{"(3!)!", "720"},
		{"2^3!", "64"},
		{"3!^2", "36"},
		{"(2^3)!", "40320"},
		{"-3!", "-6"},
		{"-0!", "-1"},
		{"(-0)!", "1"},
		{"sqrt(9)!", "6"},
	} {
		if ev := evaluate(t, tc.expression); ev.Outcome.Kind != "success" || ev.Outcome.Value != tc.value {
			t.Errorf("%q = %+v, want %s", tc.expression, ev.Outcome, tc.value)
		}
	}
	ev := evaluate(t, "170!")
	v, err := strconv.ParseFloat(ev.Outcome.Value, 64)
	if ev.Outcome.Kind != "success" || err != nil || math.IsInf(v, 0) || v < 1e306 {
		t.Errorf("170! = %+v, want finite boundary value", ev.Outcome)
	}
}

func TestFactorialErrorsAndFacts(t *testing.T) {
	for _, tc := range []struct {
		expression string
		code       contracts.MathErrorCode
		stage      contracts.ErrorStage
		span       contracts.SourceSpan
	}{
		{"(-3)!", "DOMAIN_ERROR", "evaluate", contracts.SourceSpan{Start: 0, End: 5}},
		{"5.5!", "DOMAIN_ERROR", "evaluate", contracts.SourceSpan{Start: 0, End: 4}},
		{"170.5!", "DOMAIN_ERROR", "evaluate", contracts.SourceSpan{Start: 0, End: 6}},
		{"171!", "NUMERIC_OVERFLOW", "evaluate", contracts.SourceSpan{Start: 0, End: 4}},
		{"(170+1)!", "NUMERIC_OVERFLOW", "evaluate", contracts.SourceSpan{Start: 0, End: 8}},
		{"!3", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 0, End: 1}},
		{"3!!", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 2, End: 3}},
		{"3! !", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 3, End: 4}},
		{"3!2", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 2, End: 3}},
	} {
		ev := evaluate(t, tc.expression)
		e := ev.Outcome.Error
		if e == nil || e.Code != tc.code || e.Stage != tc.stage || e.Span == nil || *e.Span != tc.span {
			t.Errorf("%q = %+v, want %s/%s at %v", tc.expression, e, tc.code, tc.stage, tc.span)
		}
		if tc.stage == "parse" && ev.Facts != nil {
			t.Errorf("%q parse failure supplied facts %+v", tc.expression, ev.Facts)
		}
	}

	for _, tc := range []struct {
		expression string
		facts      contracts.CalculationFacts
	}{
		{"2^3!", contracts.CalculationFacts{Operators: map[string]int{"!": 1, "^": 1}, Functions: map[string]int{}, OperationCount: 2}},
		{"(3!)!", contracts.CalculationFacts{Operators: map[string]int{"!": 2}, Functions: map[string]int{}, OperationCount: 2, Depth: 1}},
		{"sqrt(9)!", contracts.CalculationFacts{Operators: map[string]int{"!": 1}, Functions: map[string]int{"sqrt": 1}, OperationCount: 2, Depth: 1}},
		{"(-3)!", contracts.CalculationFacts{Operators: map[string]int{"!": 1}, Functions: map[string]int{}, OperationCount: 1, Depth: 1}},
	} {
		ev := evaluate(t, tc.expression)
		if ev.Facts == nil || !reflect.DeepEqual(*ev.Facts, tc.facts) {
			t.Errorf("%q facts = %+v, want %+v", tc.expression, ev.Facts, tc.facts)
		}
	}
}

func TestPercentageSuccess(t *testing.T) {
	for _, tc := range []struct{ expression, value string }{
		{"10%", "0.1"},
		{"200+10%", "200.1"},
		{"200*(1+10%)", "220.00000000000003"},
		{"2^100%", "2"},
		{"-50%^2", "-0.25"},
		{"(-50)%^2", "0.25"},
		{"(50%)%", "0.005"},
		{"0%", "0"},
		{"(-0)%", "0"},
		{"(-50)%", "-0.5"},
		{"sqrt(25)%", "0.05"},
		{"1e-323%", "0"}, // binary64 underflow may round to zero
	} {
		if ev := evaluate(t, tc.expression); ev.Outcome.Kind != "success" || ev.Outcome.Value != tc.value {
			t.Errorf("%q = %+v, want %s", tc.expression, ev.Outcome, tc.value)
		}
	}
}

func TestPercentageErrorsAndFacts(t *testing.T) {
	for _, tc := range []struct {
		expression string
		span       contracts.SourceSpan
	}{
		{"%5", contracts.SourceSpan{Start: 0, End: 1}},
		{"50%%", contracts.SourceSpan{Start: 3, End: 4}},
		{"50%! ", contracts.SourceSpan{Start: 3, End: 4}},
		{"2!%", contracts.SourceSpan{Start: 2, End: 3}},
		{"50%2", contracts.SourceSpan{Start: 3, End: 4}},
	} {
		ev := evaluate(t, tc.expression)
		e := ev.Outcome.Error
		if e == nil || e.Code != "SYNTAX_ERROR" || e.Stage != "parse" || e.Span == nil || *e.Span != tc.span || ev.Facts != nil {
			t.Errorf("%q = %+v facts %+v, want SYNTAX_ERROR/parse at %v without facts", tc.expression, e, ev.Facts, tc.span)
		}
	}

	for _, tc := range []struct {
		expression string
		facts      contracts.CalculationFacts
	}{
		{"200+10%", contracts.CalculationFacts{Operators: map[string]int{"+": 1, "%": 1}, Functions: map[string]int{}, OperationCount: 2}},
		{"(50%)%", contracts.CalculationFacts{Operators: map[string]int{"%": 2}, Functions: map[string]int{}, OperationCount: 2, Depth: 1}},
		{"sqrt(25)%", contracts.CalculationFacts{Operators: map[string]int{"%": 1}, Functions: map[string]int{"sqrt": 1}, OperationCount: 2, Depth: 1}},
	} {
		ev := evaluate(t, tc.expression)
		if ev.Facts == nil || !reflect.DeepEqual(*ev.Facts, tc.facts) {
			t.Errorf("%q facts = %+v, want %+v", tc.expression, ev.Facts, tc.facts)
		}
	}
}

func TestRemainderSuccess(t *testing.T) {
	for _, tc := range []struct{ expression, value string }{
		{"mod(-7,3)", "-1"},
		{"mod(7,-3)", "1"},
		{"mod(-7,-3)", "-1"},
		{"mod(5.5,2)", "1.5"},
		{"mod(-5.5,2)", "-1.5"},
		{"mod(5.5,-2)", "1.5"},
		{"mod(-6,3)", "0"}, // math.Mod returns -0; the application returns 0
		{"mod(-0,3)", "0"},
		{"mod(-1e308,1e308)", "0"},
		{"mod(5,1e308)", "5"},
		{"mod(1e-300,1e308)", "1e-300"},
		{"MOD(10,4)", "2"},
		{"mod(1+4,2*2)", "1"},
		{"log(mod(8,3),2)", "1"},
		{"mod(5!,3)", "0"},
		{"mod(50%,0.25)", "0"},
	} {
		if ev := evaluate(t, tc.expression); ev.Outcome.Kind != "success" || ev.Outcome.Value != tc.value {
			t.Errorf("%q = %+v, want %s", tc.expression, ev.Outcome, tc.value)
		}
	}
}

func TestParenthesizedFunctionsFinishBeforeFollowingOperators(t *testing.T) {
	for _, tc := range []struct{ expression, value string }{
		{"mod(7,3)^2", "1"},
		{"mod(7,3)^0", "1"},
		{"mod(log(100),3)", "2"},
		{"sin(90)^0", "1"},
		{"log(100)^2", "4"},
		{"log(100)!", "2"},
		{"(log (8),2)", "3"},
		{"mod((log (8),2),2)", "1"},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			outcome := evaluate(t, tc.expression).Outcome
			if outcome.Kind != "success" || outcome.Value != tc.value {
				t.Fatalf("%s = %+v; want %s", tc.expression, outcome, tc.value)
			}
		})
	}
	if outcome := evaluate(t, "mod(1,log(10),10)").Outcome; outcome.Kind != "error" || outcome.Error.Code != "WRONG_ARITY" {
		t.Fatalf("closed nested function captured an outer argument: %+v", outcome)
	}
}

func TestRemainderErrorsAndFacts(t *testing.T) {
	for _, tc := range []struct {
		expression string
		code       contracts.MathErrorCode
		stage      contracts.ErrorStage
		span       contracts.SourceSpan
	}{
		{"mod(1,0)", "DIVISION_BY_ZERO", "evaluate", contracts.SourceSpan{Start: 0, End: 8}},
		{"mod(1,-0)", "DIVISION_BY_ZERO", "evaluate", contracts.SourceSpan{Start: 0, End: 9}},
		{"1+mod(1,0)", "DIVISION_BY_ZERO", "evaluate", contracts.SourceSpan{Start: 2, End: 10}},
		{"mod(1)", "WRONG_ARITY", "parse", contracts.SourceSpan{Start: 5, End: 6}},
		{"mod(1,2,3)", "WRONG_ARITY", "parse", contracts.SourceSpan{Start: 7, End: 8}},
		{"mod()", "WRONG_ARITY", "parse", contracts.SourceSpan{Start: 4, End: 5}},
		{"mod(1,)", "WRONG_ARITY", "parse", contracts.SourceSpan{Start: 6, End: 7}},
		{"mod(,2)", "WRONG_ARITY", "parse", contracts.SourceSpan{Start: 4, End: 5}},
		{"mod", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 3, End: 3}},
		{"mod 1,2", "SYNTAX_ERROR", "parse", contracts.SourceSpan{Start: 4, End: 5}},
		{"mod(1/0,3)", "DIVISION_BY_ZERO", "evaluate", contracts.SourceSpan{Start: 4, End: 7}},
	} {
		ev := evaluate(t, tc.expression)
		e := ev.Outcome.Error
		if e == nil || e.Code != tc.code || e.Stage != tc.stage || e.Span == nil || *e.Span != tc.span {
			t.Errorf("%q = %+v, want %s/%s at %v", tc.expression, e, tc.code, tc.stage, tc.span)
			continue
		}
		if tc.code == "WRONG_ARITY" && e.Params["name"] != "mod" {
			t.Errorf("%q params = %v, want name mod", tc.expression, e.Params)
		}
		if tc.stage == "parse" && ev.Facts != nil {
			t.Errorf("%q parse failure supplied facts %+v", tc.expression, ev.Facts)
		}
	}
	ev := evaluate(t, "1/(-0%)")
	if e := ev.Outcome.Error; e == nil || e.Code != "DIVISION_BY_ZERO" || e.Stage != "evaluate" ||
		e.Span == nil || *e.Span != (contracts.SourceSpan{Start: 0, End: 7}) {
		t.Errorf("1/(-0%%) = %+v, want DIVISION_BY_ZERO/evaluate at 0..7", ev.Outcome)
	}

	for _, tc := range []struct {
		expression string
		facts      contracts.CalculationFacts
	}{
		{"mod(-7,3)", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{"mod": 1}, OperationCount: 1, Depth: 1}},
		{"mod(5!,3)%", contracts.CalculationFacts{Operators: map[string]int{"!": 1, "%": 1}, Functions: map[string]int{"mod": 1}, OperationCount: 3, Depth: 1}},
		{"mod(1,0)", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{"mod": 1}, OperationCount: 1, Depth: 1}},
		{"mod(1/0,3)", contracts.CalculationFacts{Operators: map[string]int{"/": 1}, Functions: map[string]int{"mod": 1}, OperationCount: 2, Depth: 1}},
	} {
		ev := evaluate(t, tc.expression)
		if ev.Facts == nil || !reflect.DeepEqual(*ev.Facts, tc.facts) {
			t.Errorf("%q facts = %+v, want %+v", tc.expression, ev.Facts, tc.facts)
		}
	}
}

// The angle unit is the whole mathematical context besides the expression:
// pi keeps its value, and only trigonometry depends on the unit.
func TestAngleUnit(t *testing.T) {
	for _, expression := range []string{"pi", "e", "2*pi", "sqrt(2)", "log(8,2)", "exp(1)", "ln(pi)", "1/0", "sqrt(-1)"} {
		d, r := evaluateIn(t, expression, contracts.Degrees), evaluateIn(t, expression, contracts.Radians)
		if !reflect.DeepEqual(d, r) {
			t.Errorf("%q: deg %+v, rad %+v", expression, d.Outcome, r.Outcome)
		}
	}
	// In degrees pi is still 3.14159..., not 180 degrees.
	for _, tc := range [][2]string{{"sin(pi)", "sin(3.141592653589793)"}, {"cos(pi/2)", "cos(1.5707963267948966)"}} {
		if p, n := evaluate(t, tc[0]), evaluate(t, tc[1]); !reflect.DeepEqual(p.Outcome, n.Outcome) {
			t.Errorf("%q in deg = %+v, want %q = %+v", tc[0], p.Outcome, tc[1], n.Outcome)
		}
	}
	// The first-use default is degrees.
	if d, u := evaluateIn(t, "sin(90)", contracts.Degrees), evaluateIn(t, "sin(90)", ""); !reflect.DeepEqual(d, u) {
		t.Errorf("sin(90) default = %+v, want degrees %+v", u.Outcome, d.Outcome)
	}
	if ev := evaluateIn(t, "sin(90)", "grad"); ev.Outcome.Kind != "error" {
		t.Errorf("sin(90) in grad = %+v, want an error", ev.Outcome)
	}
}

func TestScientificFacts(t *testing.T) {
	for _, tc := range []struct {
		expression string
		facts      contracts.CalculationFacts
	}{
		{"sqrt(81)+2^3", contracts.CalculationFacts{Operators: map[string]int{"+": 1, "^": 1}, Functions: map[string]int{"sqrt": 1}, OperationCount: 3, Depth: 1}},
		{"sqrt(sqrt(16))", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{"sqrt": 2}, OperationCount: 2, Depth: 2}},
		{"log(sin(30), 2)", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{"log": 1, "sin": 1}, OperationCount: 2, Depth: 2}},
		{"log(100)+log(8,2)", contracts.CalculationFacts{Operators: map[string]int{"+": 1}, Functions: map[string]int{"log": 2}, OperationCount: 3, Depth: 1}},
		{"log (log 100), 2", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{"log": 2}, OperationCount: 2, Depth: 1}},
		{"sin 30", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{"sin": 1}, OperationCount: 1, Depth: 0}},
		{"SIN(30)", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{"sin": 1}, OperationCount: 1, Depth: 1}},
		// Constants are operands, not operations.
		{"2*pi", contracts.CalculationFacts{Operators: map[string]int{"*": 1}, Functions: map[string]int{}, OperationCount: 1, Depth: 0}},
		{"e", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{}, OperationCount: 0, Depth: 0}},
		// Facts come with a mathematical error.
		{"sqrt(-1)", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{"sqrt": 1}, OperationCount: 1, Depth: 1}},
		{"tan(90)", contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{"tan": 1}, OperationCount: 1, Depth: 1}},
	} {
		ev := evaluate(t, tc.expression)
		if ev.Facts == nil || !reflect.DeepEqual(*ev.Facts, tc.facts) {
			t.Errorf("%q facts = %+v, want %+v", tc.expression, ev.Facts, tc.facts)
		}
	}
	// Letters in unparsed input are not function calls.
	for _, expression := range []string{"unknown(1)", "sinx", "sin(1,2)", "sin(", "2 sin(30)", "sqrt(4) trailing"} {
		if ev := evaluate(t, expression); ev.Facts != nil {
			t.Errorf("%q facts = %+v, want none", expression, *ev.Facts)
		}
	}
}
