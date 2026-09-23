package main

import (
	"database/sql"
	"errors"
	"net/http"
	"unicode/utf16"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
)

func (a api) reduction(w http.ResponseWriter, r *http.Request) {
	if !a.reductionEnabled {
		apiError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	owner, err := a.identity(r)
	if err != nil {
		identityError(w, err)
		return
	}
	// QueryRow releases the SQLite connection when readRecord scans it. The
	// potentially longer reduction runs only after the owned row is closed.
	record, err := readRecord(a.db.QueryRowContext(r.Context(), `
		SELECT id, request_id, expression, angle_unit, semantics_version, outcome_json, facts_json, created_at
		FROM calculations WHERE id = ? AND session_id = ?`, r.PathValue("id"), owner), nil)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		apiError(w, http.StatusNotFound, "NOT_FOUND")
		return
	case err != nil:
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	if record.Outcome.Kind != "success" || record.Context.SemanticsVersion != semanticsVersion {
		apiError(w, http.StatusConflict, "REDUCTION_UNAVAILABLE")
		return
	}

	sequence, err := a.engine.Reduce(r.Context(), calculation.Input{
		Expression: record.Expression, AngleUnit: record.Context.AngleUnit,
	})
	if err != nil {
		if errors.Is(err, calculation.ErrExpressionLimit) || errors.Is(err, calculation.ErrReductionInconsistent) {
			apiError(w, http.StatusConflict, "REDUCTION_UNAVAILABLE")
		} else if r.Context().Err() == nil {
			apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		}
		return
	}
	if !validReduction(sequence, record) {
		apiError(w, http.StatusConflict, "REDUCTION_UNAVAILABLE")
		return
	}

	steps := make([]contracts.ReductionStep, len(sequence.Steps))
	for i, step := range sequence.Steps {
		steps[i] = contracts.ReductionStep{
			Before: step.Before, Span: step.Span, Replacement: step.Replacement, After: step.After,
		}
	}
	writeJSON(w, http.StatusOK, contracts.ReductionResponse{
		CalculationID: record.ID, InitialExpression: sequence.InitialExpression,
		Steps: steps, FinalExpression: sequence.FinalExpression,
	})
}

func validReduction(sequence calculation.Reduction, record contracts.CalculationRecord) bool {
	if sequence.InitialExpression != record.Expression || sequence.Outcome.Kind != "success" ||
		sequence.Outcome.Value != record.Outcome.Value || sequence.FinalExpression != record.Outcome.Value {
		return false
	}
	if record.Facts != nil && len(sequence.Steps) < record.Facts.OperationCount {
		return false
	}
	current := sequence.InitialExpression
	for _, step := range sequence.Steps {
		if step.Before != current || step.Replacement == "" || step.Span.Start >= step.Span.End {
			return false
		}
		start, startOK := byteOffsetUTF16(step.Before, step.Span.Start)
		end, endOK := byteOffsetUTF16(step.Before, step.Span.End)
		if !startOK || !endOK || start >= end ||
			step.Before[:start]+step.Replacement+step.Before[end:] != step.After {
			return false
		}
		current = step.After
	}
	return len(sequence.Steps) == 0 || current == sequence.FinalExpression
}

func byteOffsetUTF16(value string, offset int) (int, bool) {
	if offset < 0 {
		return 0, false
	}
	units := 0
	for at, r := range value {
		if units == offset {
			return at, true
		}
		units += utf16.RuneLen(r)
		if units > offset {
			return 0, false
		}
	}
	return len(value), units == offset
}
