package main

import (
	"net/http"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/statistics"
)

func (a api) statistics(w http.ResponseWriter, r *http.Request) {
	if !a.statisticsEnabled {
		apiError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	owner, err := a.identity(r)
	if err != nil {
		identityError(w, err)
		return
	}
	result, err := statistics.Read(r.Context(), a.db, owner)
	if err != nil {
		apiError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
