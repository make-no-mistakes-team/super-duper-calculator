package main

import (
	"net/http"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

var coreCapabilities = contracts.Capabilities{
	SemanticsVersion: semanticsVersion,
	Operators:        []string{"+", "-", "*", "/", "^"},
	Functions: map[string][]int{
		"sqrt": {1}, "abs": {1}, "exp": {1}, "ln": {1}, "log": {1, 2},
		"sin": {1}, "cos": {1}, "tan": {1}, "asin": {1}, "acos": {1}, "atan": {1},
	},
	AngleUnits:       []contracts.AngleUnit{contracts.Degrees, contracts.Radians},
	DefaultAngleUnit: contracts.Degrees,
	Limits:           contracts.CapabilityLimits{ExpressionLength: 1024, Tokens: 256, Nesting: 32},
	Features: contracts.CapabilityFeatures{
		Factorial: false, Percentage: false, Remainder: false, Statistics: false,
		Achievements: false, Themes: false, MinimalPresentation: false,
		Localization: false, ReductionPlayback: false, Rooms: false,
		RoomReactions: false, RoomPublicationControl: false,
	},
}

func (a api) capabilities(w http.ResponseWriter, _ *http.Request) {
	available := coreCapabilities
	available.Features.Statistics = a.statisticsEnabled
	writeJSON(w, http.StatusOK, available)
}
