package protocol

import "time"

// Goal is an objective an Agent session keeps working toward: after each
// turn Rashin sends the agent on until it reports the goal done or blocked,
// the turn budget runs out, or the user pauses or stops it.
type Goal struct {
	Objective string `json:"objective"`
	// Status is active, paused, done, blocked or stopped.
	Status string `json:"status"`
	// Reason is the agent's summary when it ended the goal, or why Rashin
	// paused or stopped it.
	Reason string `json:"reason,omitempty"`
	// Turns counts the turns run toward it, MaxTurns is the budget.
	Turns    int       `json:"turns"`
	MaxTurns int       `json:"maxTurns"`
	Started  time.Time `json:"started"`
	// Ended is set once it is done, blocked or stopped.
	Ended *time.Time `json:"ended,omitempty"`
}

// Goal statuses.
const (
	GoalActive  = "active"
	GoalPaused  = "paused"
	GoalDone    = "done"
	GoalBlocked = "blocked"
	GoalStopped = "stopped"
)

// Goal operations.
const (
	GoalSet      = "set"
	GoalPause    = "pause"
	GoalResume   = "resume"
	GoalStop     = "stop"
	GoalTurns    = "turns"
	GoalGet      = "get"
	GoalComplete = "complete"
	GoalBlock    = "block"
	// GoalClear puts away a goal that is done, blocked or stopped.
	GoalClear = "clear"
)

// GoalParams changes a session's goal. Set takes Objective and an optional
// MaxTurns; Turns takes MaxTurns; Complete and Block take the agent's
// Summary; Get changes nothing.
type GoalParams struct {
	ID        string `json:"id"`
	Op        string `json:"op"`
	Objective string `json:"objective,omitempty"`
	MaxTurns  int    `json:"maxTurns,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

// GoalData is the data of a goal event: the goal as it now stands.
type GoalData struct {
	Goal Goal `json:"goal"`
}
