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

// The member's name, or the configured root/synthesizer when id is empty.
func MemberName(team protocol.Team, id string) string {
	for _, member := range team.Members {
		if member.ID == id || id == "" && member.Lead {
			return member.Name
		}
	}
	return ""
}

// The member's name by team id, using its configured default when member is empty.
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

// Lead is the explicitly configured tree root or peer synthesizer. A peer's
// position in the member list never gives it authority over the others.
func Lead(team protocol.Team) (protocol.Member, int) {
	for index, member := range team.Members {
		if member.Lead {
			return member, index
		}
	}
	return protocol.Member{}, -1
}

// ReportingSummary describes the configured role, not a spirit's rank.
func ReportingSummary(team protocol.Team) string {
	lead, at := Lead(team)
	if team.Reporting == protocol.ReportingPeers {
		if at >= 0 {
			return i18n.Tf("Peers report to Summoner; %s synthesizes", lead.Name)
		}
		return i18n.T("Peers report to Summoner")
	}
	if at >= 0 {
		return i18n.Tf("Led by %s", lead.Name)
	}
	return i18n.T("No lead configured")
}

// DefaultAssignee is the same explicit seat selected by the runtime for an
// empty task/routine member. No synthesizer means a peer must be chosen.
func DefaultAssignee(snapshot state.Snapshot, team protocol.Team) (string, string) {
	member, at := Lead(team)
	if at < 0 {
		return i18n.T("Choose a member"), i18n.T("No default assignee is configured. Choose who takes the work.")
	}
	engine := MemberEngine(snapshot, member).Line()
	if team.Reporting == protocol.ReportingPeers {
		return i18n.Tf("Synthesizer: %s", member.Name), i18n.Tf("%s. Takes this work without authority to assign it to peers.", engine)
	}
	return i18n.Tf("Lead: %s", member.Name), engine
}
