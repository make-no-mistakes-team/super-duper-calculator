// Command reduction-preview prints the engine's steps without starting the API.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
)

func main() {
	angle := flag.String("angle", "deg", "angle unit: deg or rad")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/reduction-preview [-angle deg|rad] 'expression'")
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	result, err := calculation.New().Reduce(context.Background(), calculation.Input{
		Expression: flag.Arg(0), AngleUnit: contracts.AngleUnit(*angle),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if result.Outcome.Kind == "error" {
		fmt.Fprintf(os.Stderr, "Математическая ошибка: %s\n", result.Outcome.Error.Code)
		os.Exit(1)
	}

	fmt.Printf("Исходное выражение: %s\n", result.InitialExpression)
	if len(result.Steps) == 0 {
		fmt.Println("Шагов нет.")
	}
	for i, step := range result.Steps {
		fmt.Printf("%d. %s\n", i+1, step.Before)
		fmt.Printf("   [%d:%d] %q → %s\n", step.Span.Start, step.Span.End,
			step.Before[step.Span.Start:step.Span.End], step.Replacement)
		fmt.Printf("   %s\n", step.After)
	}
	fmt.Printf("Итог: %s\n", result.FinalExpression)
}
