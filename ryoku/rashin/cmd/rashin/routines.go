package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "routines", Group: "lifecycle", Usage: "wake", Summary: i18n.T("Start the daemon when a routine is on."), Run: runRoutines})
}

// runRoutines wakes the daemon for routines after login, a reboot or a
// daemon exit; the wake unit runs it every half hour. It decides from the
// teams' state on disk, so it never starts the daemon for nothing.
func runRoutines(args []string) int {
	if len(args) != 1 || args[0] != "wake" {
		return usageError("routines wake")
	}
	settings, err := config.Load()
	if err != nil {
		return report(err)
	}
	if !settings.Enabled || !routinesOn() {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Start: true})
	if err != nil {
		return report(err)
	}
	conn.Close()
	return 0
}

// routinesOn reports any routine that is on in a team that is not paused,
// read from the teams' state file without the daemon.
func routinesOn() bool {
	raw, err := os.ReadFile(filepath.Join(paths.TeamsDir(), "state.json"))
	if err != nil {
		return false
	}
	var state struct {
		Teams []struct {
			ID     string `json:"id"`
			Paused bool   `json:"paused"`
		} `json:"teams"`
		Routines []struct {
			Team    string `json:"team"`
			Enabled bool   `json:"enabled"`
		} `json:"routines"`
	}
	if json.Unmarshal(raw, &state) != nil {
		return false
	}
	working := make(map[string]bool, len(state.Teams))
	for _, team := range state.Teams {
		working[team.ID] = !team.Paused
	}
	for _, r := range state.Routines {
		if r.Enabled && working[r.Team] {
			return true
		}
	}
	return false
}
