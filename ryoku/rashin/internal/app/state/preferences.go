package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-rashin/internal/paths"
)

const (
	DefaultZoom = 100
	MinZoom     = 80
	MaxZoom     = 150
	ZoomStep    = 10
)

// the stored words of the look, read by palette.ParseLook; Rashin starts on standard and low
const (
	ContrastStandard = "standard"
	ContrastHigh     = "high"
	TintOff          = "off"
	TintLow          = "low"
	TintMedium       = "medium"
)

// window and frame state only, nothing the daemon owns
type AppPreferences struct {
	Width         int  `json:"width,omitempty"`
	Height        int  `json:"height,omitempty"`
	ZoomPercent   int  `json:"zoomPercent,omitempty"`
	SidebarFolded bool `json:"sidebarFolded,omitempty"`
	SidePanelOpen bool `json:"sidePanelOpen,omitempty"`
	ReduceMotion  bool `json:"reduceMotion,omitempty"`
	// the look's contrast and tint words
	Contrast string `json:"contrast,omitempty"`
	Tint     string `json:"tint,omitempty"`
	// notices are on until asked off, so the absent key means on
	MuteNotices bool `json:"muteNotices,omitempty"`
	// where Rashin mode stood when the window closed
	Last LastPlace `json:"last,omitzero"`
	// where Teams mode stood, and whether the window closed in it
	Teams   LastPlace `json:"teams,omitzero"`
	InTeams bool      `json:"inTeams,omitempty"`
	// next steps hidden on a team's Overview, step keys by team id
	HiddenSteps map[string][]string `json:"hiddenSteps,omitempty"`
}

// the work the window had open when it closed
type LastPlace struct {
	Tab      string `json:"tab,omitempty"`
	Session  string `json:"session,omitempty"`
	Project  string `json:"project,omitempty"`
	View     string `json:"view,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	// a Teams place, whose View is a team page's tab
	Team       string `json:"team,omitempty"`
	Task       string `json:"task,omitempty"`
	Routine    string `json:"routine,omitempty"`
	Credential string `json:"credential,omitempty"`
}

func (p AppPreferences) Normalize() AppPreferences {
	if p.ZoomPercent == 0 {
		p.ZoomPercent = DefaultZoom
	}
	p.ZoomPercent = min(MaxZoom, max(MinZoom, p.ZoomPercent))
	switch p.Contrast {
	case ContrastStandard, ContrastHigh:
	default:
		p.Contrast = ContrastStandard
	}
	switch p.Tint {
	case TintOff, TintLow, TintMedium:
	default:
		p.Tint = TintLow
	}
	return p
}

func LoadAppPreferences() AppPreferences {
	data, err := os.ReadFile(paths.AppState())
	if err != nil {
		return (AppPreferences{}).Normalize()
	}
	var preferences AppPreferences
	if json.Unmarshal(data, &preferences) != nil {
		return (AppPreferences{}).Normalize()
	}
	return preferences.Normalize()
}

func SaveAppPreferences(preferences AppPreferences) error {
	preferences = preferences.Normalize()
	if preferences.Width <= 0 || preferences.Height <= 0 {
		return nil
	}
	path := paths.AppState()
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := json.Marshal(preferences)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
