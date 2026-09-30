package main

import (
	"net/http"

	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
)

func (a api) capabilities(w http.ResponseWriter, _ *http.Request) {
	available := calculation.MathematicalCapabilities()
	available.Features.Statistics = a.statisticsEnabled
	available.Features.Achievements = a.discoveries != nil
	available.Features.Themes = true
	writeJSON(w, http.StatusOK, available)
}
