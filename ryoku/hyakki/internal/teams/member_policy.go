package teams

import "ryoku-rashin/internal/protocol"

func memberTools(team protocol.Team, member protocol.Member) protocol.TeamTools {
	if member.Tools == nil {
		return team.Tools
	}
	return protocol.TeamTools{
		Screen: team.Tools.Screen && member.Tools.Screen,
		Mail:   team.Tools.Mail && member.Tools.Mail,
	}
}

func memberCanDelegate(team protocol.Team, member string) bool {
	if team.Reporting != protocol.ReportingTree {
		return false
	}
	for _, report := range team.Members {
		if report.ReportsTo == member {
			return true
		}
	}
	return false
}

func taskNeedsPlan(team protocol.Team, task protocol.TeamTask) bool {
	return team.PlanApproval && (task.Plan == nil || !task.Plan.Approved)
}

func taskApprovals(team protocol.Team, member protocol.Member, task protocol.TeamTask) string {
	if taskNeedsPlan(team, task) {
		return protocol.ApprovalsReadOnly
	}
	return protocol.MemberApprovals(team.Approvals, member.Trust)
}

func taskWakeAllowed(member protocol.Member, origin string, automatic bool) bool {
	if !automatic {
		return true
	}
	mode := protocol.HeartbeatTask
	switch origin {
	case protocol.TaskFromRoutine:
		mode = protocol.HeartbeatRoutine
	case protocol.TaskFromMessage:
		mode = protocol.HeartbeatMessage
	case protocol.TaskFromHeartbeat:
		mode = protocol.HeartbeatInterval
	}
	return memberWakeAllowed(member, mode)
}

func memberRuntimeKind(member protocol.Member) string {
	if member.Runtime == nil || member.Runtime.Kind == "" {
		return protocol.MemberRuntimeNative
	}
	return member.Runtime.Kind
}
