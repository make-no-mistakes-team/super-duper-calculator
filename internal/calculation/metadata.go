package calculation

import (
	"slices"
	"strings"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

// SemanticsVersion identifies the engine's persisted mathematical semantics.
const SemanticsVersion = "binary64-v2"

// mathematicalCapabilities is derived once from the parser's executable table.
// It stays private; callers receive their own mutable collections.
var mathematicalCapabilities = func() contracts.Capabilities {
	available := contracts.Capabilities{
		SemanticsVersion: SemanticsVersion,
		Operators:        make([]string, 0),
		Functions:        make(map[string][]int),
		Limits: contracts.CapabilityLimits{
			ExpressionLength: maxLength, Tokens: maxTokens, Nesting: maxNesting,
		},
	}
	for name, operation := range operators {
		switch {
		case operation.fn:
			// Comma entries are parser variants, not separately named functions.
			name = strings.TrimSuffix(name, ",")
			if !slices.Contains(available.Functions[name], operation.arity) {
				available.Functions[name] = append(available.Functions[name], operation.arity)
			}
		case operation.postfix || operation.arity == 2:
			// Constants and the internal unary-sign helpers are not advertised.
			available.Operators = append(available.Operators, name)
		}
	}
	// Preserve the public order: precedence first, spelling for equal precedence.
	slices.SortFunc(available.Operators, func(a, b string) int {
		if difference := operators[a].prec - operators[b].prec; difference != 0 {
			return difference
		}
		return strings.Compare(a, b)
	})
	for _, arities := range available.Functions {
		slices.Sort(arities)
	}
	available.Features.Factorial = slices.Contains(available.Operators, "!")
	available.Features.Percentage = slices.Contains(available.Operators, "%")
	available.Features.Remainder = slices.Contains(available.Functions["mod"], 2)
	return available
}()

// MathematicalCapabilities returns the engine's mathematical language and work
// budgets. HTTP adapters add deployment-specific and optional feature flags.
// Mutating the result cannot change future results or the engine's language.
func MathematicalCapabilities() contracts.Capabilities {
	available := mathematicalCapabilities
	available.Operators = slices.Clone(available.Operators)
	available.Functions = make(map[string][]int, len(mathematicalCapabilities.Functions))
	for name, arities := range mathematicalCapabilities.Functions {
		available.Functions[name] = slices.Clone(arities)
	}
	return available
}
