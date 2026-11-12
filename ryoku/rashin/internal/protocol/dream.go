package protocol

import "time"

// Dream phases, in the order a dream usually moves through them. The manager
// says which one it is in; Presenting waits for the user, and Done is set only
// when the user accepts what was presented.
const (
	DreamInterviewing = "interviewing"
	DreamDesigning    = "designing"
	DreamPlanning     = "planning"
	DreamBuilding     = "building"
	DreamTesting      = "testing"
	DreamPresenting   = "presenting"
	DreamDone         = "done"
)

// DreamPhases are the phases in order.
var DreamPhases = []string{DreamInterviewing, DreamDesigning, DreamPlanning, DreamBuilding, DreamTesting, DreamPresenting, DreamDone}

// How the user answers a presentation: DreamAnswerParams.Verdict and
// DreamPresentation.Verdict.
const (
	DreamAccepted = "accepted"
	DreamNotes    = "notes"
)

// Dream is what makes a team a dream: a standing objective the user, as the
// CEO, gave a manager agent, who hires the team it needs and works until the
// objective is built, tested and accepted. A dream team runs without asking
// for confirmation; it asks the user only about the design and the task.
type Dream struct {
	// Statement is what the user wants, in their words; Brief adds what the
	// interview settled.
	Statement string `json:"statement"`
	Brief     string `json:"brief,omitempty"`
	Phase     string `json:"phase"`
	// Summary is the manager's latest one-sentence account of where the
	// dream stands.
	Summary string `json:"summary,omitempty"`
	// Manager is the member that runs the dream and Task the standing task
	// it works the dream in.
	Manager string `json:"manager"`
	Task    string `json:"task,omitempty"`
	// Hired counts the members the manager hired, released ones included.
	Hired int `json:"hired,omitempty"`
	// Rounds counts the presentations made so far.
	Rounds       int                `json:"rounds,omitempty"`
	Presentation *DreamPresentation `json:"presentation,omitempty"`
	Started      time.Time          `json:"started"`
	Done         *time.Time         `json:"done,omitempty"`
}

// DreamPresentation is the manager's hand-over: what was built, how to use
// it, the evidence it works, and the files to look at. Verdict and Note are
// the user's answer; notes send the dream back to work.
type DreamPresentation struct {
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Tests   string    `json:"tests,omitempty"`
	Files   []string  `json:"files,omitempty"`
	At      time.Time `json:"at"`
	Verdict string    `json:"verdict,omitempty"`
	Note    string    `json:"note,omitempty"`
}

// DreamCreateParams starts a dream: the objective, and optionally its name,
// folder and the manager's engine; empty fields take the teams defaults.
// MonthTokens 0 leaves the dream without a token cap.
type DreamCreateParams struct {
	Statement string `json:"statement"`
	Name      string `json:"name,omitempty"`
	Folder    string `json:"folder,omitempty"`
	Harness   string `json:"harness,omitempty"`
	Model     string `json:"model,omitempty"`
	// Options are the manager's harness options, as a member's are.
	Options     map[string]string `json:"options,omitempty"`
	MonthTokens int64             `json:"monthTokens,omitempty"`
}

// DreamAnswerParams answers a dream's presentation: accept it, or send notes
// that keep the team working.
type DreamAnswerParams struct {
	Team    string `json:"team"`
	Verdict string `json:"verdict"`
	Note    string `json:"note,omitempty"`
}

// AgentHireParams lets a dream's manager add a member: its name and title,
// its standing instructions, and optionally its engine; empty takes the
// manager's own. Why is told to the user in the dream's record.
type AgentHireParams struct {
	AgentRef
	Name         string `json:"name"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
	Harness      string `json:"harness,omitempty"`
	Model        string `json:"model,omitempty"`
	Why          string `json:"why,omitempty"`
}

// AgentReleaseParams lets a dream's manager let a member go once its work is
// done; a member with a working task cannot be released.
type AgentReleaseParams struct {
	AgentRef
	Member string `json:"member"`
	Why    string `json:"why,omitempty"`
}

// AgentDreamParams is the manager telling the user where the dream stands.
type AgentDreamParams struct {
	AgentRef
	Phase   string `json:"phase"`
	Summary string `json:"summary"`
	Brief   string `json:"brief,omitempty"`
}

// AgentPresentParams is the manager presenting the finished dream to the
// user; the dream waits in DreamPresenting until the user answers.
type AgentPresentParams struct {
	AgentRef
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tests string   `json:"tests,omitempty"`
	Files []string `json:"files,omitempty"`
}
