package protocol

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MTeamsAgentHands is a team member's request that Hyakki's own Hands work
// toward a goal: in the nested desktop Hyakki owns, or in one live window
// the user picked. The member supplies the goal only; Hands' own loop acts,
// and the member gets no stream of actions. The whole live desktop is never
// a scope a member may name.
const MTeamsAgentHands = "teams.agent_hands"

// ApprovalHands is a member's Hands request. Its Steps are the actions the
// scope allows, its Detail names the scope and the window, and it can only
// be allowed once: an allowed one becomes a grant bound to the task, its
// run, the scope and the window, kept in memory until the task's run ends,
// the grant expires or Hyakki restarts.
const ApprovalHands = "hands"

// Limits on teams.agent_hands values.
const (
	// AgentHandsGoalMax bounds the goal, in runes, as oni.hands bounds one.
	AgentHandsGoalMax = OniHandsGoalMax
	// AgentHandsWindowMax bounds a window's id, in bytes.
	AgentHandsWindowMax = 64
)

// AgentHandsParams asks for Hands in Scope (OniHandsScopeNested, or
// OniHandsScopeWindow with Window, the id of one live window as the
// window picker lists it) toward Goal.
type AgentHandsParams struct {
	AgentRef
	Goal   string `json:"goal"`
	Scope  string `json:"scope"`
	Window string `json:"window,omitempty"`
}

// AgentHandsScopes are the scopes a team member may ask Hands for, in a
// fixed order. The browser tab is the user's own and the whole desktop is
// never offered.
var AgentHandsScopes = []string{OniHandsScopeNested, OniHandsScopeWindow}

// ValidAgentHandsWindow reports whether id can name a live window: 1 to
// AgentHandsWindowMax printable ASCII characters without spaces.
func ValidAgentHandsWindow(id string) bool {
	if id == "" || len(id) > AgentHandsWindowMax {
		return false
	}
	for i := range len(id) {
		if id[i] <= ' ' || id[i] > '~' {
			return false
		}
	}
	return true
}

// ValidAgentHandsGoal reports whether goal is one line of 1 to
// AgentHandsGoalMax runes without control or formatting characters.
func ValidAgentHandsGoal(goal string) bool {
	if strings.TrimSpace(goal) == "" || !utf8.ValidString(goal) || utf8.RuneCountInString(goal) > AgentHandsGoalMax {
		return false
	}
	for _, r := range goal {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}
