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
	available.Features.Rooms = a.rooms != nil
	available.Features.RoomReactions = a.rooms != nil && a.rooms.config.ReactionsEnabled
	available.Features.RoomPublicationControl = a.rooms != nil && a.rooms.config.PublicationEnabled
	writeJSON(w, http.StatusOK, available)
}
