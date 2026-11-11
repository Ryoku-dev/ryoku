package teams

import (
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Access is what a team session's sandbox adds for its team: the folders it
// may change and the paths it sees as empty, as absolute paths. Zero for any
// other session.
func (s *Service) Access(info protocol.SessionInfo) protocol.TeamAccess {
	if info.Team == "" {
		return protocol.TeamAccess{}
	}
	team, ok := s.state.team(info.Team)
	if !ok {
		return protocol.TeamAccess{}
	}
	return protocol.TeamAccess{Writable: expandAll(team.Access.Writable), Hidden: expandAll(team.Access.Hidden)}
}

// expandAll reads each path's leading ~ as the home folder.
func expandAll(list []string) []string {
	if len(list) == 0 {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, path := range list {
		out = append(out, paths.Expand(path))
	}
	return out
}
