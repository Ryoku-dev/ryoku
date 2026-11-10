package shell

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-rashin/internal/paths"
)

type windowState struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func loadWindowState() windowState {
	data, err := os.ReadFile(paths.AppState())
	if err != nil {
		return windowState{}
	}
	var state windowState
	if json.Unmarshal(data, &state) != nil {
		return windowState{}
	}
	return state
}

func saveWindowState(state windowState) error {
	if state.Width <= 0 || state.Height <= 0 {
		return nil
	}
	path := paths.AppState()
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
