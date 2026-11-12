package teamsui

import "ryoku-rashin/internal/protocol"

// Ordinary leaves the dreams out, which show only under Dreams
func Ordinary(teams []protocol.Team) []protocol.Team {
	kept := make([]protocol.Team, 0, len(teams))
	for _, team := range teams {
		if team.Dream == nil {
			kept = append(kept, team)
		}
	}
	return kept
}

// Dreams keeps only the teams that are dreams
func Dreams(teams []protocol.Team) []protocol.Team {
	var dreams []protocol.Team
	for _, team := range teams {
		if team.Dream != nil {
			dreams = append(dreams, team)
		}
	}
	return dreams
}
