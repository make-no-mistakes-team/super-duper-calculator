// Package statistics derives personal metrics from persisted calculations.
package statistics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf16"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

const maxCounter = int64(^uint64(0) >> 1)

// Read reconstructs an owner's statistics from one ordered SQLite snapshot.
// It neither evaluates historical expressions nor changes persisted state.
func Read(ctx context.Context, db *sql.DB, owner string) (stats contracts.PersonalStatistics, err error) {
	if err := ctx.Err(); err != nil {
		return stats, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, expression, outcome_json, facts_json
		FROM calculations
		WHERE session_id = ?
		ORDER BY seq ASC`, owner)
	if err != nil {
		return stats, fmt.Errorf("read statistics: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			stats = contracts.PersonalStatistics{}
			err = errors.Join(err, fmt.Errorf("close statistics rows: %w", closeErr))
		}
	}()

	stats.Operators = make(map[string]int64)
	stats.Functions = make(map[string]int64)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return contracts.PersonalStatistics{}, err
		}
		var id, expression, outcomeJSON string
		var factsJSON sql.NullString
		if err := rows.Scan(&id, &expression, &outcomeJSON, &factsJSON); err != nil {
			return contracts.PersonalStatistics{}, fmt.Errorf("scan statistics record: %w", err)
		}

		var outcome contracts.Outcome
		if err := json.Unmarshal([]byte(outcomeJSON), &outcome); err != nil {
			return contracts.PersonalStatistics{}, fmt.Errorf("decode outcome for calculation %s: %w", id, err)
		}
		switch outcome.Kind {
		case contracts.OutcomeSuccess:
			if outcome.Error != nil || outcome.Value == "" {
				return contracts.PersonalStatistics{}, fmt.Errorf("invalid success outcome for calculation %s", id)
			}
			stats.Successes++
		case contracts.OutcomeError:
			if outcome.Value != "" || outcome.Error == nil || outcome.Error.Code == "" ||
				(outcome.Error.Stage != contracts.StageParse && outcome.Error.Stage != contracts.StageEvaluate) {
				return contracts.PersonalStatistics{}, fmt.Errorf("invalid mathematical outcome for calculation %s", id)
			}
			stats.MathematicalErrors++
			if outcome.Error.Code == contracts.ErrorDivisionByZero {
				stats.DivisionByZeroAttempts++
			}
		default:
			return contracts.PersonalStatistics{}, fmt.Errorf("unknown outcome for calculation %s", id)
		}

		if factsJSON.Valid {
			var facts *contracts.CalculationFacts
			if err := json.Unmarshal([]byte(factsJSON.String), &facts); err != nil {
				return contracts.PersonalStatistics{}, fmt.Errorf("decode facts for calculation %s: %w", id, err)
			}
			if facts != nil {
				if facts.Depth < 0 || facts.OperationCount < 0 {
					return contracts.PersonalStatistics{}, fmt.Errorf("negative parsed metadata for calculation %s", id)
				}
				parsed := outcome.Kind == contracts.OutcomeSuccess || outcome.Error.Stage != contracts.StageParse
				if err := addUsage(stats.Operators, facts.Operators, parsed); err != nil {
					return contracts.PersonalStatistics{}, fmt.Errorf("operators for calculation %s: %w", id, err)
				}
				if err := addUsage(stats.Functions, facts.Functions, parsed); err != nil {
					return contracts.PersonalStatistics{}, fmt.Errorf("functions for calculation %s: %w", id, err)
				}
				if parsed && (stats.MaxParsedDepth == nil || facts.Depth > *stats.MaxParsedDepth) {
					if stats.MaxParsedDepth == nil {
						stats.MaxParsedDepth = new(int)
					}
					*stats.MaxParsedDepth = facts.Depth
				}
			}
		}

		stats.TotalCalculations++
		length := 0
		for _, r := range expression {
			length += utf16.RuneLen(r)
		}
		if stats.LongestExpression == nil || length > stats.LongestExpression.Length {
			if stats.LongestExpression == nil {
				stats.LongestExpression = new(contracts.LongestExpression)
			}
			*stats.LongestExpression = contracts.LongestExpression{
				CalculationID: id,
				Expression:    expression,
				Length:        length,
			}
		}
	}
	if err := rows.Err(); err != nil {
		return contracts.PersonalStatistics{}, fmt.Errorf("iterate statistics records: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return contracts.PersonalStatistics{}, err
	}
	return stats, nil
}

func addUsage(totals map[string]int64, counts map[string]int, include bool) error {
	for name, count := range counts {
		if count < 0 {
			return fmt.Errorf("negative count for %q", name)
		}
		if include && count > 0 {
			if int64(count) > maxCounter-totals[name] {
				return fmt.Errorf("count overflow for %q", name)
			}
			totals[name] += int64(count)
		}
	}
	return nil
}
