package calculation_test

import (
	"reflect"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
)

func TestMathematicalCapabilitiesCannotBeMutatedByCallers(t *testing.T) {
	before := calculation.MathematicalCapabilities()
	changed := calculation.MathematicalCapabilities()
	changed.SemanticsVersion = "changed"
	changed.Operators[0] = "changed"
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
