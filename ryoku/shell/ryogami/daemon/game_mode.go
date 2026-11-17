package main

import (
	"os"
	"path/filepath"
)

// Mirrors the path ryoku-cmd-game-mode writes, env overrides included.
func gameModeStatePath() string {
	if f := os.Getenv("RYOKU_GAMEMODE_STATE_FILE"); f != "" {
		return f
	}
	dir := os.Getenv("RYOKU_STATE_PATH")
	if dir == "" {
		dir = filepath.Join(stateHome(), "ryoku")
	}
	return filepath.Join(dir, "game-mode.enabled")
}

func gameModeOn() bool {
	_, err := os.Stat(gameModeStatePath())
	return err == nil
}
