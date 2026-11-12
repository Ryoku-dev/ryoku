package teamsui

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func TeamByID(teams []protocol.Team, id string) (protocol.Team, bool) {
	for _, team := range teams {
		if team.ID == id {
			return team, true
		}
	}
	return protocol.Team{}, false
}

func TeamName(teams []protocol.Team, id string) string {
	if team, ok := TeamByID(teams, id); ok {
		return team.Name
	}
	return i18n.T("A removed team")
}

// the member's name, or the lead's when id is empty, as tasks without one go to the lead
func MemberName(team protocol.Team, id string) string {
	for _, member := range team.Members {
		if member.ID == id || id == "" && member.Lead {
			return member.Name
		}
	}
	return ""
}

// the name of a team's member by the team's id, the lead's when member is empty
func MemberOf(snapshot state.Snapshot, teamID, member string) string {
	team, _ := TeamByID(snapshot.Teams, teamID)
	return MemberName(team, member)
}

// a row's second line: who works on the task on which team, "Curator on News digest"
func TaskFacts(snapshot state.Snapshot, task protocol.TeamTask) string {
	name := TeamName(snapshot.Teams, task.Team)
	if member := MemberOf(snapshot, task.Team, task.Member); member != "" {
		return i18n.Tf("%s on %s", member, name)
	}
	return name
}

// the lead and where it stands; the daemon keeps exactly one, so the first
// member stands in while an edit is unsaved, and -1 says there is no member
func Lead(members []protocol.Member) (protocol.Member, int) {
	for index, member := range members {
		if member.Lead {
			return member, index
		}
	}
	if len(members) > 0 {
		return members[0], 0
	}
	return protocol.Member{}, -1
}
