package protocol

import (
	"slices"
	"strings"
	"time"
)

// The tools Hyakki's MCP bridge (hyakki mcp) may offer a program the user
// approved: list and add reminders, search memory and suggest a note, read
// the focus guard, show a short line on Oni's pill, and ask about the
// perched window. Each one is off until the user turns it on.
const (
	OniMCPRemindersList = "reminders_list"
	OniMCPRemindersAdd  = "reminders_add"
	OniMCPNotesSearch   = "notes_search"
	OniMCPNotesAdd      = "notes_add"
	OniMCPFocusStatus   = "focus_status"
	OniMCPNotify        = "notify"
	OniMCPScreenAsk     = "screen_ask"
)

// OniMCPTools are every tool the MCP bridge may offer, in the order it lists
// them.
var OniMCPTools = []string{OniMCPRemindersList, OniMCPRemindersAdd, OniMCPNotesSearch, OniMCPNotesAdd, OniMCPFocusStatus, OniMCPNotify, OniMCPScreenAsk}

// Limits on Settings.oni.mcp and the bridge's calls.
const (
	OniMCPClientsMax    = 16
	OniMCPClientNameMax = 64  // runes
	OniNotifyMax        = 120 // runes
	OniReminderTextMax  = 120 // runes
)

// OniMCPSettings is Settings.oni.mcp; only the Hyakki app changes it.
// Enabled is the bridge's one switch: off, a hello from it is refused.
// Tools names the tools turned on; an unknown or false one is dropped.
// Clients are the programs the user approved, by the name each gives when it
// connects; any other program connects and waits, and every call it makes
// is refused until the user approves it.
type OniMCPSettings struct {
	Enabled bool            `json:"enabled"`
	Tools   map[string]bool `json:"tools,omitempty"`
	Clients []string        `json:"clients,omitempty"`
}

// ValidOniMCPClientName reports whether n can name a program connecting to
// the MCP bridge: one trimmed line of printable text of at most
// OniMCPClientNameMax runes.
func ValidOniMCPClientName(n string) bool {
	return n == strings.TrimSpace(n) && ValidOniText(n, OniMCPClientNameMax, false)
}

// CleanOniMCPTools keeps the tools turned on that the bridge knows, nil when
// none is on.
func CleanOniMCPTools(tools map[string]bool) map[string]bool {
	var out map[string]bool
	for _, id := range OniMCPTools {
		if tools[id] {
			if out == nil {
				out = make(map[string]bool, len(OniMCPTools))
			}
			out[id] = true
		}
	}
	return out
}

// CleanOniMCPClients is clients with each name once, in order; ok is false
// when a name is invalid or more than OniMCPClientsMax remain.
func CleanOniMCPClients(clients []string) ([]string, bool) {
	var out []string
	for _, name := range clients {
		if !ValidOniMCPClientName(name) {
			return nil, false
		}
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out, len(out) <= OniMCPClientsMax
}

// Where a program connected to the MCP bridge stands: approved in Settings,
// or waiting for the user to approve it.
const (
	OniMCPClientApproved = "approved"
	OniMCPClientWaiting  = "waiting"
)

// OniMCPClient is one program the bridge knows: its name, whether it is
// approved or waiting, and whether it is connected now.
type OniMCPClient struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	Connected bool   `json:"connected,omitempty"`
}

// OniMCPStatus answers oni.mcp: the switch, the tools turned on, and the
// approved programs with any that connected and wait. A program asking for
// itself sees its own entry alone.
type OniMCPStatus struct {
	Enabled bool           `json:"enabled"`
	Tools   []string       `json:"tools"`
	Clients []OniMCPClient `json:"clients"`
}

// OniNotifyParams is one oni.notify call: a line of at most OniNotifyMax
// runes shown on Oni's pill with the name of the program that sent it.
type OniNotifyParams struct {
	Text string `json:"text"`
}

// ValidOniNotifyText reports whether t may be shown on Oni's pill: one line
// of printable text of at most OniNotifyMax runes.
func ValidOniNotifyText(t string) bool { return ValidOniText(t, OniNotifyMax, false) }

// OnboardingSettings is Settings.onboarding, the first-run tour; only the
// Hyakki app changes it. Seen is set once the tour was shown, Done once it
// was finished or skipped. The tour turns nothing on by itself.
type OnboardingSettings struct {
	Seen bool `json:"seen"`
	Done bool `json:"done"`
}

// What kind of optional part a component is: a downloaded model, a voice
// pack, a browser profile, data the user imported, a cache, or a program the
// distribution installs.
const (
	ComponentModel   = "model"
	ComponentPack    = "pack"
	ComponentProfile = "profile"
	ComponentData    = "data"
	ComponentCache   = "cache"
	ComponentProgram = "program"
)

// ComponentKinds are every component kind.
var ComponentKinds = []string{ComponentModel, ComponentPack, ComponentProfile, ComponentData, ComponentCache, ComponentProgram}

// Where a component stands: not on this computer, whole, partly downloaded,
// present and unusable, or not looked for (listing probes nothing that
// would start a program).
const (
	ComponentAbsent   = "absent"
	ComponentPresent  = "present"
	ComponentPartial  = "partial"
	ComponentUnusable = "unusable"
	ComponentUnknown  = "unknown"
)

// ComponentStates are every component state.
var ComponentStates = []string{ComponentAbsent, ComponentPresent, ComponentPartial, ComponentUnusable, ComponentUnknown}

// ComponentIDMax bounds a component id, in bytes.
const ComponentIDMax = 128

// Component is one optional part: its id (a lowercase word naming where it
// comes from, a colon and its name), kind, title and state, its size on
// disk and where it lies, whether Hyakki may remove it, the setting that
// uses it and what uses it in words, why removing it is refused now (empty
// when it is free), and, for a program, the packages that provide it and
// why Hyakki would run it.
type Component struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Title     string   `json:"title"`
	State     string   `json:"state"`
	Bytes     int64    `json:"bytes"`
	Path      string   `json:"path,omitempty"`
	Removable bool     `json:"removable"`
	Feature   string   `json:"feature,omitempty"`
	Uses      string   `json:"uses,omitempty"`
	InUse     string   `json:"inUse,omitempty"`
	Needs     []string `json:"needs,omitempty"`
	Why       string   `json:"why,omitempty"`
}

// ValidComponentID reports whether id can name a component: a lowercase
// word, a colon and a name of lowercase letters, digits and . _ - :, not
// starting with a dot or a dash, at most ComponentIDMax bytes.
func ValidComponentID(id string) bool {
	prefix, name, ok := strings.Cut(id, ":")
	if !ok || prefix == "" || name == "" || len(id) > ComponentIDMax || name[0] == '.' || name[0] == '-' {
		return false
	}
	for _, r := range prefix {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-', r == ':':
		default:
			return false
		}
	}
	return true
}

// ComponentsResult answers components.list and components.remove with every
// component after the call.
type ComponentsResult struct {
	Components []Component `json:"components"`
}

// ComponentRemoveParams names the component components.remove deletes.
type ComponentRemoveParams struct {
	ID string `json:"id"`
}

// The stores Oni's local activity is read from and oni.activity.clear
// deletes: the quick-command ledger, the Hands ledger, the assignment
// ledger, the kept walkthroughs, the routines, the reminders and the chats
// imported for recall. All names every one.
const (
	OniActivityReflexLog     = "reflex_log"
	OniActivityHandsLog      = "hands_log"
	OniActivityAssignment    = "assignment"
	OniActivityWalkthroughs  = "walkthroughs"
	OniActivityRoutines      = "routines"
	OniActivityReminders     = "reminders"
	OniActivityRecallImports = "recall_imports"
	OniActivityAll           = "all"
)

// OniActivityStores are every activity store, all excepted.
var OniActivityStores = []string{OniActivityReflexLog, OniActivityHandsLog, OniActivityAssignment, OniActivityWalkthroughs, OniActivityRoutines, OniActivityReminders, OniActivityRecallImports}

// OniActivityRowsMax bounds the rows oni.activity returns.
const OniActivityRowsMax = 50

// ValidOniActivityStores reports whether stores names one or more activity
// stores, or all, each once.
func ValidOniActivityStores(stores []string) bool {
	if len(stores) == 0 || len(stores) > len(OniActivityStores) {
		return false
	}
	for i, s := range stores {
		if s == OniActivityAll && len(stores) != 1 || s != OniActivityAll && !slices.Contains(OniActivityStores, s) || slices.Contains(stores[:i], s) {
			return false
		}
	}
	return true
}

// OniActivityRow is one ledger row: when, the store it came from, the
// action, intent, tool or lane code, the route, how long it took, the
// measured tokens with their estimated or unmetered flags, and the outcome
// code. Never a transcript, selection, passage, picture, feed or window
// title.
type OniActivityRow struct {
	At        time.Time `json:"at"`
	Store     string    `json:"store"`
	Action    string    `json:"action"`
	Route     string    `json:"route,omitempty"`
	Ms        int64     `json:"ms,omitempty"`
	Input     int       `json:"input,omitempty"`
	Output    int       `json:"output,omitempty"`
	Estimated bool      `json:"estimated,omitempty"`
	Unmetered bool      `json:"unmetered,omitempty"`
	Outcome   string    `json:"outcome"`
}

// OniActivityResult answers oni.activity and oni.activity.clear: how many
// rows or items each store holds, the newest rows across the ledgers, at
// most OniActivityRowsMax, and when the oldest counted row was written.
// Counts contains only successfully read stores. Errors names unreadable
// stores with content-free explanations; a missing count is unknown, not zero.
// A partial result remains usable for reading or clearing healthy stores.
type OniActivityResult struct {
	Counts map[string]int    `json:"counts"`
	Errors map[string]string `json:"errors,omitempty"`
	Rows   []OniActivityRow  `json:"rows"`
	Since  time.Time         `json:"since,omitzero"`
}

// OniActivityClearParams names the stores oni.activity.clear deletes.
type OniActivityClearParams struct {
	Stores []string `json:"stores"`
}
