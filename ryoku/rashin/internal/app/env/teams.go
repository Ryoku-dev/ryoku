package env

// TeamsPlace is a place in Teams mode: a destination, protocol.TabInbox to
// protocol.TabActivity, and what is open in it. Draft opens a new thing in the
// work pane instead: DraftTeam (from Template) or DraftRoutine (for Team).
type TeamsPlace struct {
	Tab        string
	Team       string
	View       string
	Task       string
	Routine    string
	Credential string
	Draft      string
	Template   string
}

// Drafts a Teams page opens in its work pane.
const (
	DraftTeam    = "team"
	DraftRoutine = "routine"
)

// Views of a team page, as TeamsPlace.View names them.
const (
	TeamOverview = "overview"
	TeamWork     = "work"
	TeamRoutines = "routines"
	TeamSpend    = "spend"
	TeamSettings = "settings"
)

// TeamsPage is a Teams mode destination: a page that opens a place.
type TeamsPage interface {
	Page
	// Select shows place, which belongs to this page's destination.
	Select(e *Env, place TeamsPlace)
}
