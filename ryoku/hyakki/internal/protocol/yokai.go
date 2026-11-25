package protocol

const (
	TeamsMembersMax = 100
	ReportingTree   = "tree"
	ReportingPeers  = "peers"

	ApprovalMemberBudget = "member-budget"
	ApprovalStrategy     = "strategy"

	MemberRuntimeNative  = "native"
	MemberRuntimeCLI     = "cli"
	MemberRuntimeCommand = "command"
	MemberRuntimeWebhook = "webhook"

	HeartbeatTask     = "task"
	HeartbeatRoutine  = "routine"
	HeartbeatMessage  = "message"
	HeartbeatInterval = "interval"
)

// MemberRuntime is owner-authored execution configuration. Native is the
// default. Command is an argv, never a shell fragment. SecretEnv and Credential
// name vault entries; no credential value belongs in this document.
type MemberRuntime struct {
	Kind       string            `json:"kind"`
	Command    []string          `json:"command,omitempty"`
	Model      string            `json:"model,omitempty"`
	URL        string            `json:"url,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	SecretEnv  map[string]string `json:"secretEnv,omitempty"`
	Credential string            `json:"credential,omitempty"`
}

// A nil Heartbeat retains ordinary task and routine wakes. An explicit empty
// Modes list parks automatic wakes until the owner changes the configuration.
type MemberHeartbeat struct {
	Modes        []string `json:"modes"`
	EveryMinutes int      `json:"everyMinutes,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
}

// MemberApprovals may narrow the team's posture, never widen it. Invalid
// stored policies fail closed; edit validation supplies the actionable error.
func MemberApprovals(team, trust string) string {
	if !KnownApprovals(team) {
		return ApprovalsReadOnly
	}
	if trust == "" {
		return team
	}
	if !KnownApprovals(trust) || team == ApprovalsReadOnly || trust == ApprovalsReadOnly {
		return ApprovalsReadOnly
	}
	if team == ApprovalsAsk || trust == ApprovalsAsk {
		return ApprovalsAsk
	}
	return ApprovalsAuto
}
