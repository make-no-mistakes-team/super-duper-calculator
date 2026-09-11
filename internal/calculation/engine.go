// Package calculation defines the interface implemented by the expression engine.
package calculation

import (
	"context"
	"errors"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

// ErrExpressionLimit is a rejected request, not a persisted mathematical outcome.
var ErrExpressionLimit = errors.New("expression limit exceeded")

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
