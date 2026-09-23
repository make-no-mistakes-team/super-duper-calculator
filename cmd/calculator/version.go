package main

import (
	"net/http"
	"runtime/debug"
)

type buildVersion struct {
	Version   string `json:"version"`
	Revision  string `json:"revision,omitempty"`
	Modified  *bool  `json:"modified,omitempty"`
	GoVersion string `json:"goVersion,omitempty"`
}

var runningVersion = readBuildVersion()

func readBuildVersion() buildVersion {
	version := buildVersion{Version: "unknown"}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if info.Main.Version != "" {
		version.Version = info.Main.Version
	}
	version.GoVersion = info.GoVersion
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			version.Revision = setting.Value
		case "vcs.modified":
			modified := setting.Value == "true"
			version.Modified = &modified
		}
	}
	return version
}

// Build metadata remains available when the API is saturated or SQLite is down.
func versionInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, runningVersion)
}
