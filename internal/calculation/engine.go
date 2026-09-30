// Package calculation defines the interface implemented by the expression engine.
package calculation

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

// ErrExpressionLimit is a rejected request, not a persisted mathematical outcome.
var ErrExpressionLimit = errors.New("expression limit exceeded")

// Budgets from specs/calculation-engine.md.
const (
	maxLength  = 1024 // UTF-16 code units
	maxTokens  = 256
	maxNesting = 32
)

type Input struct {
	Expression string
	AngleUnit  contracts.AngleUnit
}

type Evaluation struct {
	Outcome contracts.Outcome
	Facts   *contracts.CalculationFacts
}

// Evaluate returns mathematical failures in Outcome. Internal failures and work
// limit violations use the error result; callers must not persist those as maths.
type Engine interface {
	Evaluate(context.Context, Input) (Evaluation, error)
}

type engine struct{}

// New returns the Engine.
func New() Engine { return engine{} }

func (engine) Evaluate(ctx context.Context, in Input) (Evaluation, error) {
	if err := ctx.Err(); err != nil {
		return Evaluation{}, err
	}

	unit := in.AngleUnit
	switch unit {
	case "":
		unit = contracts.Degrees
	case contracts.Degrees, contracts.Radians:
	default:
		return Evaluation{Outcome: contracts.Outcome{Kind: contracts.OutcomeError, Error: &contracts.MathError{
			Code: contracts.ErrorUnsupportedFeature, Stage: contracts.StageEvaluate, Params: map[string]any{"angleUnit": string(unit)}}}}, nil
	}

	// Facts exist only for parsed expressions; an evaluation error keeps them.
	facts := &contracts.CalculationFacts{Operators: map[string]int{}, Functions: map[string]int{}}
	tokens, starts, merr := tokenize(in.Expression)
	var expr expr
	if merr == nil {
		expr, merr = parse(tokens, starts, unit, facts)
	}
	if merr != nil {
		if merr.Code == contracts.ErrorExpressionLimit {
			return Evaluation{}, ErrExpressionLimit
		}
		return Evaluation{Outcome: contracts.Outcome{Kind: contracts.OutcomeError, Error: merr}}, nil
	}

	v, merr := expr()
	if merr != nil {
		return Evaluation{Outcome: contracts.Outcome{Kind: contracts.OutcomeError, Error: merr}, Facts: facts}, nil
	}
	if v == 0 {
		v = 0 // -0 -> 0
	}
	return Evaluation{Outcome: contracts.Outcome{
		Kind:  contracts.OutcomeSuccess,
		Value: strconv.FormatFloat(v, 'g', -1, 64),
	}, Facts: facts}, nil
}

// tokenize splits expression into numbers ("12", ".5", "1.25e-3"), names,
// operators, parentheses and commas. Tokens are lowercase ("1E5" -> "1e5",
// "SIN" -> "sin"). Unary signs stay separate tokens: -2^2 is "-" "2" "^" "2".
// starts[k] is the byte offset of tokens[k]. Only ASCII is accepted.
func tokenize(expression string) ([]string, []int, *contracts.MathError) {
	length := 0
	for _, r := range expression {
		length += utf16.RuneLen(r)
		if length > maxLength {
			return nil, nil, &contracts.MathError{Code: contracts.ErrorExpressionLimit, Stage: contracts.StageParse,
				Params: map[string]any{"length": maxLength}}
		}
	}

	tokens := make([]string, 0, maxTokens)
	starts := make([]int, 0, maxTokens)
	for i := 0; i < len(expression); {
		start := i
		b := expression[i]
		switch {
		case b == ' ', b == '\t', b == '\n', b == '\r':
			i++

		case b == '+', b == '-', b == '*', b == '/', b == '^', b == '(', b == ')', b == ',', b == '!', b == '%':
			tokens = append(tokens, expression[i:i+1])
			i++

		case b >= '0' && b <= '9', b == '.':
			j := i
			digits := 0
			for j < len(expression) && expression[j] >= '0' && expression[j] <= '9' {
				j++
				digits++
			}
			if j < len(expression) && expression[j] == '.' {
				j++
				for j < len(expression) && expression[j] >= '0' && expression[j] <= '9' {
					j++
					digits++
				}
			}
			if digits == 0 {
				return nil, nil, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
					Params: map[string]any{"expected": "digit"}, Span: &contracts.SourceSpan{Start: i, End: j}}
			}

			if j < len(expression) {
				switch expression[j] {
				case 'E':
					fallthrough
				case 'e':
					e := j
					j++
					if j < len(expression) && (expression[j] == '+' || expression[j] == '-') {
						j++
					}
					start := j
					for j < len(expression) && expression[j] >= '0' && expression[j] <= '9' {
						j++
					}
					if j == start {
						return nil, nil, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
							Params: map[string]any{"expected": "exponent"}, Span: &contracts.SourceSpan{Start: e, End: j}}
					}
				}
			}

			// 1.2.3, 1e5e3 and 2pi (no implicit multiplication) end up here.
			if j < len(expression) {
				switch c := expression[j]; {
				case c >= 'A' && c <= 'Z':
					fallthrough
				case c >= 'a' && c <= 'z', c == '.':
					return nil, nil, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
						Params: map[string]any{"unexpected": string(c)}, Span: &contracts.SourceSpan{Start: j, End: j + 1}}
				}
			}

			tokens = append(tokens, strings.ToLower(expression[i:j]))
			i = j

		case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z':
			word := make([]byte, 0, 8)
			j := i
		scan:
			for ; j < len(expression); j++ {
				c := expression[j]
				switch {
				case c >= 'A' && c <= 'Z':
					c |= 0x20
					fallthrough
				case c >= 'a' && c <= 'z':
					word = append(word, c)
				default:
					break scan
				}
			}
			tokens = append(tokens, string(word))
			i = j

		default:
			if b < 0x80 {
				return nil, nil, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
					Params: map[string]any{"unexpected": expression[i : i+1]}, Span: &contracts.SourceSpan{Start: i, End: i + 1}}
			}
			r, _ := utf8.DecodeRuneInString(expression[i:])
			return nil, nil, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
				Params: map[string]any{"unexpected": string(r)}, Span: &contracts.SourceSpan{Start: i, End: i + utf16.RuneLen(r)}}
		}
		if len(starts) < len(tokens) {
			starts = append(starts, start)
		}
		if len(tokens) > maxTokens {
			return nil, nil, &contracts.MathError{Code: contracts.ErrorExpressionLimit, Stage: contracts.StageParse,
				Params: map[string]any{"tokens": maxTokens}}
		}
	}
	return tokens, starts, nil
}

// expr is a parsed expression: numbers and operations alike are functions,
// an operation calls the expressions of its operands.
type expr func() (float64, *contracts.MathError)

// operator describes one entry of the operators table.
type operator struct {
	prec    int  // higher binds stronger
	right   bool // right-associative
	arity   int  // 0: constant, 1: prefix/postfix, 2: infix or a two-argument function
	fn      bool // named function: prefix, counted in facts.Functions
	postfix bool // postfix operator: applies immediately to the preceding primary
	apply   func(x []float64, unit contracts.AngleUnit) (float64, *contracts.MathError)
}

// reduceDegrees splits x degrees into quarter turns n (0..3) and a remainder
// y in radians within ±45°. The split happens in degrees, where it is exact,
// so only y goes through the inexact pi/180: sin 180 is exactly 0.
func reduceDegrees(x float64) (float64, int) {
	r := math.Mod(x, 360)
	n := math.RoundToEven(r / 90)                 // 45 stays in quarter 0: tan 45 is tan(pi/4)
	return (r - 90*n) * math.Pi / 180, int(n) & 3 // -90 is the same as 270
}

func remainder(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) {
	if x[1] == 0 {
		return 0, &contracts.MathError{Code: contracts.ErrorDivisionByZero, Stage: contracts.StageEvaluate, Params: map[string]any{}}
	}
	return math.Mod(x[0], x[1]), nil
}

// operators is everything parse knows besides numbers and parentheses; the
// language is extended by adding entries. Prefix signs are
// keyed "u"+token. Functions are prefix operators with the precedence of a
// sign, so parentheses around arguments are optional: sin 3^2 is sin(9).
// A comma gives the nearest function a second argument and switches it to
// the name+"," entry: log 8 is log10(8), log 8, 2 and log(8, 2) are log2(8).
// mod alone requires both parentheses and exactly two arguments.
var operators = map[string]operator{
	"+": {prec: 1, arity: 2, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return x[0] + x[1], nil }},
	"-": {prec: 1, arity: 2, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return x[0] - x[1], nil }},
	"*": {prec: 2, arity: 2, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return x[0] * x[1], nil }},
	"/": {prec: 2, arity: 2, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) {
		if x[1] == 0 {
			return 0, &contracts.MathError{Code: contracts.ErrorDivisionByZero, Stage: contracts.StageEvaluate, Params: map[string]any{}}
		}
		return x[0] / x[1], nil
	}},
	"u+": {prec: 3, arity: 1, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return x[0], nil }},
	"u-": {prec: 3, arity: 1, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return -x[0], nil }},
	"^": {prec: 4, right: true, arity: 2, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) {
		if x[0] == 0 && x[1] < 0 {
			return 0, &contracts.MathError{Code: contracts.ErrorDivisionByZero, Stage: contracts.StageEvaluate, Params: map[string]any{}}
		}
		return math.Pow(x[0], x[1]), nil // (-8)^(1/3) is NaN -> DOMAIN_ERROR
	}},
	"!": {prec: 5, arity: 1, postfix: true, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) {
		n := x[0]
		if n < 0 || math.Trunc(n) != n {
			return 0, &contracts.MathError{Code: contracts.ErrorDomain, Stage: contracts.StageEvaluate, Params: map[string]any{}}
		}
		if n > 170 {
			return 0, &contracts.MathError{Code: contracts.ErrorNumericOverflow, Stage: contracts.StageEvaluate, Params: map[string]any{}}
		}
		result := 1.0
		for i := 2; i <= int(n); i++ {
			result *= float64(i)
		}
		return result, nil
	}},
	"%": {prec: 5, arity: 1, postfix: true, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) {
		return x[0] / 100, nil
	}},

	// Constants do not depend on the angle unit.
	"pi": {arity: 0, apply: func(_ []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return math.Pi, nil }},
	"e":  {arity: 0, apply: func(_ []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return math.E, nil }},

	// NaN and Inf results become DOMAIN_ERROR and NUMERIC_OVERFLOW in parse,
	// so sqrt(-1), asin(2) and exp(1000) need no checks of their own.
	"sqrt": {prec: 3, arity: 1, fn: true, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return math.Sqrt(x[0]), nil }},
	"abs":  {prec: 3, arity: 1, fn: true, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return math.Abs(x[0]), nil }},
	"exp":  {prec: 3, arity: 1, fn: true, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) { return math.Exp(x[0]), nil }},
	"ln": {prec: 3, arity: 1, fn: true, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) {
		if x[0] <= 0 { // ln(0) is -Inf, not an overflow
			return 0, &contracts.MathError{Code: contracts.ErrorDomain, Stage: contracts.StageEvaluate, Params: map[string]any{}}
		}
		return math.Log(x[0]), nil
	}},
	"log": {prec: 3, arity: 1, fn: true, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) {
		if x[0] <= 0 {
			return 0, &contracts.MathError{Code: contracts.ErrorDomain, Stage: contracts.StageEvaluate, Params: map[string]any{}}
		}
		return math.Log10(x[0]), nil
	}},
	"log,": {prec: 3, arity: 2, fn: true, apply: func(x []float64, _ contracts.AngleUnit) (float64, *contracts.MathError) {
		if x[0] <= 0 || x[1] <= 0 || x[1] == 1 { // log(x, base)
			return 0, &contracts.MathError{Code: contracts.ErrorDomain, Stage: contracts.StageEvaluate, Params: map[string]any{}}
		}
		return math.Log(x[0]) / math.Log(x[1]), nil
	}},
	"mod":  {prec: 3, arity: 2, fn: true, apply: remainder},
	"mod,": {prec: 3, arity: 2, fn: true, apply: remainder},
	"sin": {prec: 3, arity: 1, fn: true, apply: func(x []float64, unit contracts.AngleUnit) (float64, *contracts.MathError) {
		if unit == contracts.Degrees {
			y, n := reduceDegrees(x[0])
			return [4]float64{math.Sin(y), math.Cos(y), -math.Sin(y), -math.Cos(y)}[n], nil
		}
		return math.Sin(x[0]), nil
	}},
	"cos": {prec: 3, arity: 1, fn: true, apply: func(x []float64, unit contracts.AngleUnit) (float64, *contracts.MathError) {
		if unit == contracts.Degrees {
			y, n := reduceDegrees(x[0])
			return [4]float64{math.Cos(y), -math.Sin(y), -math.Cos(y), math.Sin(y)}[n], nil
		}
		return math.Cos(x[0]), nil
	}},
	"tan": {prec: 3, arity: 1, fn: true, apply: func(x []float64, unit contracts.AngleUnit) (float64, *contracts.MathError) {
		// Poles are compared exactly: math.Tan(math.Pi/2) is finite, and a
		// tolerance would hide small valid results.
		if unit == contracts.Degrees {
			y, n := reduceDegrees(x[0])
			switch {
			case n&1 == 0:
				return math.Tan(y), nil
			case y == 0: // odd multiples of 90
				return 0, &contracts.MathError{Code: contracts.ErrorDomain, Stage: contracts.StageEvaluate, Params: map[string]any{}}
			}
			return -1 / math.Tan(y), nil
		}
		if q := (x[0] - math.Pi/2) / math.Pi; math.Abs(q) < 1<<52 && q == math.Round(q) {
			// above 2^52 every float is an integer, and pi multiples are meaningless
			return 0, &contracts.MathError{Code: contracts.ErrorDomain, Stage: contracts.StageEvaluate, Params: map[string]any{}}
		}
		return math.Tan(x[0]), nil
	}},
	"asin": {prec: 3, arity: 1, fn: true, apply: func(x []float64, unit contracts.AngleUnit) (float64, *contracts.MathError) {
		if unit == contracts.Degrees {
			return math.Asin(x[0]) * 180 / math.Pi, nil
		}
		return math.Asin(x[0]), nil
	}},
	"acos": {prec: 3, arity: 1, fn: true, apply: func(x []float64, unit contracts.AngleUnit) (float64, *contracts.MathError) {
		if unit == contracts.Degrees {
			return math.Acos(x[0]) * 180 / math.Pi, nil
		}
		return math.Acos(x[0]), nil
	}},
	"atan": {prec: 3, arity: 1, fn: true, apply: func(x []float64, unit contracts.AngleUnit) (float64, *contracts.MathError) {
		if unit == contracts.Degrees {
			return math.Atan(x[0]) * 180 / math.Pi, nil
		}
		return math.Atan(x[0]), nil
	}},
}

// parse builds an expr from tokenize output with the shunting-yard algorithm:
// instead of writing postfix notation, every operator popped from the stack
// takes its operands from the operand stack and pushes back one expr.
// Every binary operator and function call is one operation recorded in facts;
// facts.Depth is the deepest parenthesis nesting. Every operand carries the
// span of its source text, evaluation errors point at the failed operation.
func parse(tokens []string, starts []int, unit contracts.AngleUnit, facts *contracts.CalculationFacts) (expr, *contracts.MathError) {
	type operand struct {
		expr expr
		span contracts.SourceSpan
	}
	type pending struct {
		key string // operators key or "("
		at  int    // token index
	}
	operands := make([]operand, 0, len(tokens))
	ops := make([]pending, 0, len(tokens))
	depth := 0
	expectOperand := true

	fail := func(code contracts.MathErrorCode, span contracts.SourceSpan, params map[string]any) *contracts.MathError {
		return &contracts.MathError{Code: code, Stage: contracts.StageParse, Params: params, Span: &span}
	}
	end := 0 // offset after the last token: "5*(" fails at {3, 3}
	if len(tokens) > 0 {
		end = starts[len(tokens)-1] + len(tokens[len(tokens)-1])
	}

	reduce := func() *contracts.MathError {
		p := ops[len(ops)-1]
		ops = ops[:len(ops)-1]
		o := operators[p.key]
		switch {
		case o.fn:
			facts.Functions[strings.TrimSuffix(p.key, ",")]++
			facts.OperationCount++
		case o.postfix, o.arity == 2: // a unary sign is part of a signed number, not an operation
			facts.Operators[p.key]++
			facts.OperationCount++
		}
		if len(operands) < o.arity {
			return &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse, Params: map[string]any{"expected": "operand"}}
		}
		args := append([]operand(nil), operands[len(operands)-o.arity:]...)
		operands = operands[:len(operands)-o.arity]

		span := contracts.SourceSpan{Start: starts[p.at], End: starts[p.at] + len(tokens[p.at])} // constant
		if len(args) > 0 {
			if o.postfix {
				span.Start = args[0].span.Start
			} else {
				span.End = args[len(args)-1].span.End
			}
			if !o.fn && o.arity == 2 { // infix: starts at the left operand
				span.Start = args[0].span.Start
			}
		}
		operands = append(operands, operand{span: span, expr: func() (float64, *contracts.MathError) {
			x := make([]float64, len(args))
			for k, a := range args {
				v, merr := a.expr()
				if merr != nil {
					return 0, merr
				}
				x[k] = v
			}
			v, merr := o.apply(x, unit)
			switch {
			case merr != nil:
			case math.IsNaN(v):
				merr = &contracts.MathError{Code: contracts.ErrorDomain, Stage: contracts.StageEvaluate, Params: map[string]any{}}
			case math.IsInf(v, 0):
				merr = &contracts.MathError{Code: contracts.ErrorNumericOverflow, Stage: contracts.StageEvaluate, Params: map[string]any{}}
			default:
				return v, nil
			}
			if merr.Span == nil {
				s := span
				merr.Span = &s
			}
			return 0, merr
		}})
		return nil
	}

	for k, t := range tokens {
		at := contracts.SourceSpan{Start: starts[k], End: starts[k] + len(t)}
		switch c := t[0]; {
		case t == "(":
			if !expectOperand { // 2(3), (1)(2)
				return nil, fail(contracts.ErrorSyntax, at, map[string]any{"expected": "operator"})
			}
			if depth++; depth > maxNesting {
				return nil, fail(contracts.ErrorExpressionLimit, at, map[string]any{"depth": maxNesting})
			}
			facts.Depth = max(facts.Depth, depth)
			ops = append(ops, pending{key: "(", at: k})

		case t == ")":
			if expectOperand { // (), 2+)
				if len(ops) >= 2 && ops[len(ops)-1].key == "(" &&
					(ops[len(ops)-2].key == "mod" || ops[len(ops)-2].key == "mod,") {
					return nil, fail(contracts.ErrorWrongArity, at, map[string]any{"name": "mod"})
				}
				return nil, fail(contracts.ErrorSyntax, contracts.SourceSpan{Start: at.Start, End: at.Start}, map[string]any{"expected": "operand"})
			}
			for len(ops) > 0 && ops[len(ops)-1].key != "(" {
				if merr := reduce(); merr != nil {
					return nil, merr
				}
			}
			if len(ops) == 0 {
				return nil, fail(contracts.ErrorSyntax, at, map[string]any{"unexpected": t})
			}
			if len(ops) >= 2 && ops[len(ops)-2].key == "mod" {
				return nil, fail(contracts.ErrorWrongArity, at, map[string]any{"name": "mod"})
			}
			operands[len(operands)-1].span = contracts.SourceSpan{Start: starts[ops[len(ops)-1].at], End: at.End}
			ops = ops[:len(ops)-1]
			depth--
			// Complete a function call before an enclosing call consumes its
			// comma or a following operator acts on the result. A prefix log
			// may still take its base after a parenthesized operand:
			// log (log 100), 2.
			if len(ops) > 0 && operators[ops[len(ops)-1].key].fn {
				continuesPrefixLog := ops[len(ops)-1].key == "log" && k+1 < len(tokens) && tokens[k+1] == ","
				for i := len(ops) - 2; continuesPrefixLog && i >= 0; i-- {
					if ops[i].key == "(" {
						continuesPrefixLog = i == 0 || !operators[ops[i-1].key].fn
						break
					}
				}
				if !continuesPrefixLog {
					if merr := reduce(); merr != nil {
						return nil, merr
					}
				}
			}

		case t == ",":
			if expectOperand { // log(, 2), log 8,,2
				if len(ops) >= 2 && ops[len(ops)-1].key == "(" && ops[len(ops)-2].key == "mod" {
					return nil, fail(contracts.ErrorWrongArity, at, map[string]any{"name": "mod"})
				}
				return nil, fail(contracts.ErrorSyntax, contracts.SourceSpan{Start: at.Start, End: at.Start}, map[string]any{"expected": "operand"})
			}
			// The comma belongs to the nearest function that takes another
			// argument: log(sin(30), 2) and log 8*2, 2 finish sin(30) and 8*2
			// first. Inside parentheses only the function right before "("
			// may take it: sin(1, 2) is WRONG_ARITY, not a comma for an outer log.
			skipped := "" // sin 1, 2: a function that takes no more arguments
			for len(ops) > 0 && ops[len(ops)-1].key != "(" {
				p := ops[len(ops)-1]
				if operators[p.key].fn {
					if _, ok := operators[p.key+","]; ok {
						break
					}
					if skipped == "" {
						skipped = p.key
					}
				}
				if merr := reduce(); merr != nil {
					return nil, merr
				}
			}
			f := len(ops) - 1
			if f >= 0 && ops[f].key == "(" { // log(8, 2): the function is under "("
				f--
				if f >= 0 && operators[ops[f].key].fn {
					skipped = ops[f].key
				} else {
					f = -1
				}
			}
			if f >= 0 {
				if _, ok := operators[ops[f].key+","]; !ok {
					f = -1
				}
			}
			switch {
			case f < 0 && skipped != "": // sin(1, 2), log(8, 2, 3), sin 1, 2
				return nil, fail(contracts.ErrorWrongArity, at, map[string]any{"name": strings.TrimSuffix(skipped, ",")})
			case f < 0: // (8, 2), 1, 2
				return nil, fail(contracts.ErrorSyntax, at, map[string]any{"unexpected": t})
			}
			ops[f].key += ","
			expectOperand = true

		case t == "!" || t == "%":
			if expectOperand {
				return nil, fail(contracts.ErrorSyntax, at, map[string]any{"expected": "operand"})
			}
			if k > 0 && (tokens[k-1] == "!" || tokens[k-1] == "%") {
				return nil, fail(contracts.ErrorSyntax, at, map[string]any{"expected": "operator"})
			}
			ops = append(ops, pending{key: t, at: k})
			if merr := reduce(); merr != nil {
				return nil, merr
			}

		case c >= '0' && c <= '9', c == '.':
			if !expectOperand { // 1 2
				return nil, fail(contracts.ErrorSyntax, at, map[string]any{"expected": "operator"})
			}
			v, err := strconv.ParseFloat(t, 64)
			if err != nil { // 1e400
				return nil, fail(contracts.ErrorNumericOverflow, at, map[string]any{"literal": t})
			}
			operands = append(operands, operand{span: at, expr: func() (float64, *contracts.MathError) { return v, nil }})
			expectOperand = false

		case !expectOperand:
			o, ok := operators[t]
			if !ok || o.arity != 2 || o.fn { // 2+3 trailing, 2 sin 3
				return nil, fail(contracts.ErrorSyntax, at, map[string]any{"expected": "operator"})
			}
			for len(ops) > 0 {
				p := operators[ops[len(ops)-1].key].prec // 0 for "("
				if p < o.prec || p == o.prec && o.right {
					break
				}
				if merr := reduce(); merr != nil {
					return nil, merr
				}
			}
			ops = append(ops, pending{key: t, at: k})
			expectOperand = true

		default: // an operand is expected
			key := "u" + t // prefix sign
			if o, ok := operators[t]; ok && (o.fn || o.arity == 0) {
				key = t
			}
			if o, ok := operators[key]; ok && (o.fn || o.arity < 2) {
				if t == "mod" && (k+1 == len(tokens) || tokens[k+1] != "(") {
					if k+1 == len(tokens) {
						return nil, fail(contracts.ErrorSyntax, contracts.SourceSpan{Start: at.End, End: at.End}, map[string]any{"expected": "("})
					}
					next := contracts.SourceSpan{Start: starts[k+1], End: starts[k+1] + len(tokens[k+1])}
					return nil, fail(contracts.ErrorSyntax, next, map[string]any{"expected": "("})
				}
				ops = append(ops, pending{key: key, at: k}) // prefix: pushed without popping anything
				if o.arity == 0 {                           // a constant is an operand at once
					if merr := reduce(); merr != nil {
						return nil, merr
					}
					expectOperand = false
				}
				continue
			}
			if c >= 'a' && c <= 'z' {
				return nil, fail(contracts.ErrorUnknownIdentifier, at, map[string]any{"name": t})
			}
			return nil, fail(contracts.ErrorSyntax, at, map[string]any{"expected": "operand"}) // *2, 1+,5
		}
	}

	if expectOperand { // "", 2+, -, sin
		return nil, fail(contracts.ErrorSyntax, contracts.SourceSpan{Start: end, End: end}, map[string]any{"expected": "operand"})
	}
	for len(ops) > 0 {
		if ops[len(ops)-1].key == "(" {
			return nil, fail(contracts.ErrorSyntax, contracts.SourceSpan{Start: end, End: end}, map[string]any{"expected": ")"})
		}
		if merr := reduce(); merr != nil {
			return nil, merr
		}
	}
	if len(operands) != 1 {
		return nil, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse, Params: map[string]any{"expected": "operator"}}
	}
	return operands[0].expr, nil
}
