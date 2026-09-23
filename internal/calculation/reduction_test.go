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

func reduceExpression(t *testing.T, expression string, unit contracts.AngleUnit) calculation.Reduction {
	t.Helper()
	result, err := calculation.New().Reduce(t.Context(), calculation.Input{Expression: expression, AngleUnit: unit})
	if err != nil {
		t.Fatalf("Reduce(%q): %v", expression, err)
	}
	return result
}

func TestReductionSteps(t *testing.T) {
	for _, tc := range []struct {
		expression string
		unit       contracts.AngleUnit
		value      string
		steps      int
	}{
		{"2+3*4", contracts.Degrees, "14", 2},
		{"(2+3)*4", contracts.Degrees, "20", 2},
		{"(1-3)^2", contracts.Degrees, "4", 2},
		{"-(2+3)", contracts.Degrees, "-5", 2},
		{"3*-(2+3)", contracts.Degrees, "-15", 3},
		{"2^3^2", contracts.Degrees, "512", 2},
		{"log(8,2)", contracts.Degrees, "3", 1},
		{"sin(30)+cos(60)", contracts.Degrees, "0.9999999999999999", 3},
		{"sin(30)", contracts.Radians, "-0.9880316240928618", 1},
		{"0.1+0.2", contracts.Degrees, "0.30000000000000004", 1},
		{"  (1+2)  ", contracts.Degrees, "3", 1},
		{"42", contracts.Degrees, "42", 0},
		{"-42", contracts.Degrees, "-42", 0},
		{"+42", contracts.Degrees, "42", 0},
		{"2^-2", contracts.Degrees, "0.25", 1},
	} {
		t.Run(tc.expression+"/"+string(tc.unit), func(t *testing.T) {
			result := reduceExpression(t, tc.expression, tc.unit)
			if result.InitialExpression != tc.expression || result.FinalExpression != tc.value ||
				result.Outcome.Kind != "success" || result.Outcome.Value != tc.value || len(result.Steps) != tc.steps {
				t.Fatalf("Reduce(%q) = %+v, want %s in %d steps", tc.expression, result, tc.value, tc.steps)
			}
			for i, step := range result.Steps {
				if i == 0 && step.Before != tc.expression {
					t.Errorf("first before = %q, want source %q", step.Before, tc.expression)
				}
				if step.Span.Start < 0 || step.Span.End < step.Span.Start || step.Span.End > len(step.Before) {
					t.Fatalf("step %d has invalid span %+v in %q", i, step.Span, step.Before)
				}
				if got := step.Before[:step.Span.Start] + step.Replacement + step.Before[step.Span.End:]; got != step.After {
					t.Errorf("step %d replacement yields %q, want %q", i, got, step.After)
				}
				if i+1 < len(result.Steps) && step.After != result.Steps[i+1].Before {
					t.Errorf("step %d after does not equal next before", i)
				}
				for _, text := range []string{step.Before, step.After} {
					evaluation, err := calculation.New().Evaluate(t.Context(), calculation.Input{Expression: text, AngleUnit: tc.unit})
					if err != nil || evaluation.Outcome.Kind != "success" || evaluation.Outcome.Value != tc.value {
						t.Errorf("step %d expression %q = %+v, %v; want %s", i, text, evaluation.Outcome, err, tc.value)
					}
				}
			}
			if tc.steps > 0 && result.Steps[len(result.Steps)-1].After != tc.value {
				t.Errorf("last after = %q, want %q", result.Steps[len(result.Steps)-1].After, tc.value)
			}
			again := reduceExpression(t, tc.expression, tc.unit)
			if !reflect.DeepEqual(result, again) {
				t.Errorf("repeated reduction differs: %+v vs %+v", result, again)
			}
		})
	}
}

func TestReductionOrderAndGrouping(t *testing.T) {
	result := reduceExpression(t, "sin(30)+cos(60)", contracts.Degrees)
	if got := result.Steps[0].Before[result.Steps[0].Span.Start:result.Steps[0].Span.End]; got != "sin(30)" {
		t.Errorf("first redex = %q, want sin(30)", got)
	}
	if got := result.Steps[1].Before[result.Steps[1].Span.Start:result.Steps[1].Span.End]; got != "cos(60)" {
		t.Errorf("second redex = %q, want cos(60)", got)
	}
	negative := reduceExpression(t, "(1-3)^2", contracts.Degrees)
	if got := negative.Steps[0].After; got != "((-2))^2" {
		t.Errorf("negative intermediate = %q, want grouped -2", got)
	}
}

func TestReductionPreservesNestedExpressions(t *testing.T) {
	for _, expression := range []string{
		"2*(3+4)+5*(6-9)",
		"1/(2-3)",
		"2^-3^2",
		"-2^2",
		"-(2+3)^2",
		"log(8,2+1)",
		"log(8*2,2)",
		"sin 3^2",
		"sqrt(81)+2^3",
		"(-2)^3",
		"(2+3)*(4-7)",
		"1/(2+3*4)",
		"1-2*3+4",
		"((1+2)/(3+4))",
		"1e-10+2e-10",
		"abs(1-3)",
	} {
		t.Run(expression, func(t *testing.T) {
			in := calculation.Input{Expression: expression, AngleUnit: contracts.Degrees}
			evaluation, err := calculation.New().Evaluate(t.Context(), in)
			if err != nil || evaluation.Outcome.Kind != "success" {
				t.Fatalf("Evaluate(%q) = %+v, %v", expression, evaluation.Outcome, err)
			}
			result := reduceExpression(t, expression, contracts.Degrees)
			if result.FinalExpression != evaluation.Outcome.Value {
				t.Errorf("final = %q, Evaluate = %q", result.FinalExpression, evaluation.Outcome.Value)
			}
			for i, step := range result.Steps {
				for _, text := range []string{step.Before, step.After} {
					got, err := calculation.New().Evaluate(t.Context(), calculation.Input{Expression: text, AngleUnit: contracts.Degrees})
					if err != nil || got.Outcome.Kind != "success" || got.Outcome.Value != evaluation.Outcome.Value {
						t.Errorf("step %d expression %q = %+v, %v; original = %+v", i, text, got.Outcome, err, evaluation.Outcome)
					}
				}
			}
		})
	}
}

func TestReductionErrorsMatchEvaluation(t *testing.T) {
	for _, expression := range []string{"sqrt(-1)", "1/0", "sqrt(81", "3!", strings.Repeat("1+", 128) + "1"} {
		in := calculation.Input{Expression: expression, AngleUnit: contracts.Degrees}
		evaluation, evalErr := calculation.New().Evaluate(t.Context(), in)
		result, reduceErr := calculation.New().Reduce(t.Context(), in)
		if !errors.Is(evalErr, calculation.ErrExpressionLimit) && evalErr != nil {
			t.Fatalf("Evaluate(%q): %v", expression, evalErr)
		}
		if !errors.Is(reduceErr, evalErr) || (evalErr == nil && reduceErr != nil) {
			t.Errorf("Reduce(%q) err = %v, Evaluate err = %v", expression, reduceErr, evalErr)
		}
		if evalErr == nil && (!reflect.DeepEqual(result.Outcome, evaluation.Outcome) || len(result.Steps) != 0) {
			t.Errorf("Reduce(%q) = %+v, Evaluate = %+v", expression, result, evaluation)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := calculation.New().Reduce(ctx, calculation.Input{Expression: "1+1"}); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled reduction error = %v, want context.Canceled", err)
	}
}

func TestReductionAtTokenBudget(t *testing.T) {
	expression := strings.Repeat("1+", 127) + "1"
	result := reduceExpression(t, expression, contracts.Degrees)
	if result.FinalExpression != "128" || len(result.Steps) != 127 {
		t.Errorf("reduction at token budget = %q in %d steps", result.FinalExpression, len(result.Steps))
	}
}
