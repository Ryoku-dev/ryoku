package protocol

import "time"

const (
	MTeamsAgentPlan     = "teams.agent_plan"
	MTeamsAgentHelp     = "teams.agent_help"
	MTeamsAgentEscalate = "teams.agent_escalate"
	MTeamsMessages      = "teams.messages"
	MTeamsTaskComment   = "teams.task_comment"
	MTeamsTaskReassign  = "teams.task_reassign"
)

// TeamPlan is a versioned strategy, distinct from permission to perform any
// action. The Summoner's acceptance never widens the member's trust or grants.
type TeamPlan struct {
	Summary  string   `json:"summary"`
	Steps    []string `json:"steps"`
	Revision uint64   `json:"revision"`
	Approved bool     `json:"approved"`
}

type AgentPlanParams struct {
	AgentRef
	Summary string   `json:"summary"`
	Steps   []string `json:"steps"`
}

type AgentHelpParams struct {
	AgentRef
	Member string `json:"member"`
	Title  string `json:"title"`
	Brief  string `json:"brief"`
}

type AgentEscalateParams struct {
	AgentRef
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// TeamMessage routes a request without making its target an assignee. To is
// the common/nearest manager, empty for the Summoner; Member is the requested
// peer, when present. A task's single assignee remains unchanged.
type TeamMessage struct {
	ID        string    `json:"id"`
	Team      string    `json:"team"`
	Task      string    `json:"task"`
	From      string    `json:"from,omitempty"`
	To        string    `json:"to,omitempty"`
	Member    string    `json:"member,omitempty"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Created   time.Time `json:"created"`
	Delivered bool      `json:"delivered,omitempty"`
	// SourceSession binds agent requests to their durable originating execution,
	// never its secret run token. Empty legacy identities cannot wake work.
	SourceSession string `json:"sourceSession,omitempty"`
	// Retired requests remain visible history, but were not delivered and must
	// never wake automatic work after termination or execution replacement.
	Retired bool `json:"retired,omitempty"`
}

type TeamMessagesParams struct {
	Team   string `json:"team,omitempty"`
	Task   string `json:"task,omitempty"`
	Before string `json:"before,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type TeamMessagesResult struct {
	Messages []TeamMessage `json:"messages"`
	Next     string        `json:"next,omitempty"`
}

type TeamTaskCommentParams struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

type TeamTaskReassignParams struct {
	ID     string `json:"id"`
	Member string `json:"member"`
}
