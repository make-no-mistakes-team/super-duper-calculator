package calculation_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
)

func TestMathematicalCapabilitiesPreservePublicLanguage(t *testing.T) {
	available := calculation.MathematicalCapabilities()
	if available.SemanticsVersion != "binary64-v1" {
		t.Errorf("semantics version = %q", available.SemanticsVersion)
	}
	if !slices.Equal(available.Operators, []string{"+", "-", "*", "/", "^", "!", "%"}) {
		t.Errorf("operator order = %v", available.Operators)
	}
	if !slices.Equal(available.Functions["log"], []int{1, 2}) ||
		!slices.Equal(available.Functions["mod"], []int{2}) {
		t.Errorf("public log/mod arities = %v/%v", available.Functions["log"], available.Functions["mod"])
	}
	for _, internalName := range []string{"u+", "u-", "pi", "e", "log,", "mod,"} {
		if slices.Contains(available.Operators, internalName) {
			t.Errorf("internal entry %q advertised as an operator", internalName)
		}
		if _, exists := available.Functions[internalName]; exists {
			t.Errorf("internal entry %q advertised as a function", internalName)
		}
	}
}

func TestMathematicalCapabilitiesCannotBeMutatedByCallers(t *testing.T) {
	before := calculation.MathematicalCapabilities()
	changed := calculation.MathematicalCapabilities()
	changed.SemanticsVersion = "changed"
	changed.Operators[0] = "changed"
	changed.AngleUnits[0] = contracts.Radians
	changed.DefaultAngleUnit = contracts.Radians
	changed.Limits.ExpressionLength = 1
	changed.Features.Factorial = false
	for name, arities := range changed.Functions {
		arities[0] = 99
		delete(changed.Functions, name)
	}
	changed.Functions["changed"] = []int{99}

	after := calculation.MathematicalCapabilities()
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("caller mutation changed canonical metadata: before %+v, after %+v", before, after)
	}
	result, err := calculation.New().Evaluate(t.Context(), calculation.Input{Expression: "log(8,2)+mod(7,3)+5!"})
	if err != nil || result.Outcome.Kind != contracts.OutcomeSuccess || result.Outcome.Value != "124" {
		t.Fatalf("caller mutation changed engine behavior: outcome %+v, err %v", result.Outcome, err)
	}
}
