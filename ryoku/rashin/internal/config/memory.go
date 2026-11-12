package config

import (
	"net/url"
	"slices"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// HonchoWorkspace is the workspace Rashin keeps on a Honcho server unless the
// user names another. It is never shared with another program's users.
const HonchoWorkspace = "rashin-desktop"

// memoryFadeMonths are the values FadeMonths may take; 0 never fades.
var memoryFadeMonths = []int{0, 6, 12, 18}

// defaultMemory is memory before the first-run card is answered: off, with
// the choices that card turns on ready.
func defaultMemory() protocol.MemorySettings {
	return protocol.MemorySettings{
		Sessions:        true,
		Agents:          true,
		TeamSuggestions: protocol.TeamSuggestAsk,
		Model:           protocol.LearnLocalFirst,
		QuietMinutes:    10,
		Engine:          protocol.MemoryBuiltin,
		Hindsight:       protocol.HindsightSettings{Local: true},
		Honcho:          protocol.HonchoSettings{Workspace: HonchoWorkspace},
	}
}

// validateMemory normalises memory's settings: unknown enums fall back to the
// defaults, numbers are clamped and an unusable address is dropped.
func validateMemory(m *protocol.MemorySettings, d protocol.MemorySettings) {
	if m.TeamSuggestions != protocol.TeamSuggestAsk && m.TeamSuggestions != protocol.TeamSuggestNever {
		m.TeamSuggestions = d.TeamSuggestions
	}
	if m.Model != protocol.LearnLocal && m.Model != protocol.LearnLocalFirst && m.Model != protocol.LearnAny {
		m.Model = d.Model
	}
	if !slices.Contains(memoryFadeMonths, m.FadeMonths) {
		m.FadeMonths = d.FadeMonths
	}
	m.QuietMinutes = clamp(m.QuietMinutes, 2, 120, d.QuietMinutes)
	if m.Engine != protocol.MemoryBuiltin && m.Engine != protocol.MemoryHindsight && m.Engine != protocol.MemoryHoncho {
		m.Engine = d.Engine
	}
	m.Hindsight.URL = strings.TrimSpace(m.Hindsight.URL)
	if !engineAddress(m.Hindsight.URL) {
		m.Hindsight.URL = ""
	}
	m.Hindsight.Key = strings.TrimSpace(m.Hindsight.Key)
	m.Honcho.URL = strings.TrimSpace(m.Honcho.URL)
	if !engineAddress(m.Honcho.URL) {
		m.Honcho.URL = ""
	}
	m.Honcho.Workspace = strings.TrimSpace(m.Honcho.Workspace)
	if !protocol.ValidHonchoID(m.Honcho.Workspace) {
		m.Honcho.Workspace = d.Honcho.Workspace
	}
	m.Honcho.Key = strings.TrimSpace(m.Honcho.Key)
}

// checkMemory refuses a patch that points an engine at something that is not
// an http or https address, or names a workspace Honcho would refuse.
func checkMemory(m protocol.MemorySettings) error {
	for _, address := range []string{m.Hindsight.URL, m.Honcho.URL} {
		if !engineAddress(strings.TrimSpace(address)) {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Use an http or https address, such as http://127.0.0.1:8000."))
		}
	}
	if workspace := strings.TrimSpace(m.Honcho.Workspace); workspace != "" && !protocol.ValidHonchoID(workspace) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A Honcho workspace uses only letters, digits, - and _."))
	}
	return nil
}

// engineAddress accepts an empty address or an absolute http(s) URL with a
// host and no credentials in it.
func engineAddress(address string) bool {
	if address == "" {
		return true
	}
	u, err := url.Parse(address)
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
