package protocol

const MTeamsTaskContext = "teams.task_context"

// TaskAncestor is the reason for a task, without copying ancestor transcripts or
// reports into every status response. Order is nearest parent to root.
type TaskAncestor struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Member string `json:"member"`
}

type TeamTaskContext struct {
	Task    TeamTask       `json:"task"`
	Goal    string         `json:"goal"`
	Because []TaskAncestor `json:"because"`
}

// MemberDirectoryEntry is model-visible discovery. Runtime environment, vault
// references and private configuration never enter this directory.
type MemberDirectoryEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Title        string `json:"title"`
	Rank         string `json:"rank,omitempty"`
	Spirit       string `json:"spirit,omitempty"`
	Capabilities string `json:"capabilities,omitempty"`
	ReportsTo    string `json:"reportsTo,omitempty"`
	Paused       bool   `json:"paused,omitempty"`
}
