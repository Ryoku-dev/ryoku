package teams

import (
	"strings"

	"ryoku-rashin/internal/protocol"
)

func normaliseReporting(team *protocol.Team, renamed map[string]string) error {
	implicit := team.Reporting == ""
	if implicit {
		team.Reporting = protocol.ReportingTree
	}
	lead := ""
	for i := range team.Members {
		if team.Members[i].Lead {
			lead = team.Members[i].ID
			break
		}
	}
	if implicit && lead == "" && len(team.Members) > 0 {
		team.Members[0].Lead = true
		lead = team.Members[0].ID
	}
	for i := range team.Members {
		member := &team.Members[i]
		member.ReportsTo = strings.TrimSpace(member.ReportsTo)
		if id, exists := renamed[member.ReportsTo]; exists {
			member.ReportsTo = id
		}
		if implicit && !member.Lead && member.ReportsTo == "" {
			member.ReportsTo = lead
		}
	}
	if err := protocol.ValidateReporting(*team); err != nil {
		return protocol.Errorf(protocol.CodeBadRequest, "%v", err)
	}
	return nil
}
