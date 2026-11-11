package teams

import (
	"strings"

	"ryoku-rashin/internal/protocol"
)

// readable are the Teams methods anything on the socket may call: they change
// nothing and show no secret. Credential values never leave the vault, and
// vault events and activity are redacted before they are recorded.
var readable = map[string]bool{
	protocol.MTeamsList:           true,
	protocol.MTeamsTemplates:      true,
	protocol.MTeamsTasks:          true,
	protocol.MTeamsRoutines:       true,
	protocol.MTeamsRoutinePreview: true,
	protocol.MTeamsApprovals:      true,
	protocol.MTeamsActivity:       true,
	protocol.MTeamsSpend:          true,
	protocol.MVaultInfo:           true,
	protocol.MVaultEvents:         true,
}

// UserOnly reports whether a method is the user's alone to call, so the
// daemon serves it only to the Rashin app. Team members are agents running as
// the user with shell tools; one that reaches the socket must not decide its
// own approvals, change the vault, or widen what a team may do. Every
// teams.* and vault.* method is the user's except the teams.agent_* methods,
// which prove their caller with a run token instead, and the readable ones.
// A method added later is the user's until it is listed as readable.
func UserOnly(method string) bool {
	if !IsMethod(method) || strings.HasPrefix(method, "teams.agent_") {
		return false
	}
	return !readable[method]
}
