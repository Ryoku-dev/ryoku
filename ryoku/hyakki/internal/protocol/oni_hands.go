package protocol

import (
	"slices"
	"strings"
	"time"
	"unicode"
)

// What oni.hands does: open the Hands browser, list its tabs, authorize one
// of them, start a task toward a goal in the authorized tab, stop it, resume
// a paused one, report where Hands stands, or forget the Hands browser,
// deleting its profile. At the app stage it also lists the live windows a
// task may work in (windows) and binds one (select); at the app and desktop
// stages it lists the task journal (tasks) and deletes it (clear). Select
// and clear come only from the app; authorize, start and resume only from
// the app or the surface drawing Oni; forget from the app alone; stop comes
// from anyone who may call Oni at all.
const (
	OniHandsOpen      = "open"
	OniHandsTabs      = "tabs"
	OniHandsAuthorize = "authorize"
	OniHandsStart     = "start"
	OniHandsStop      = "stop"
	OniHandsResume    = "resume"
	OniHandsStatus    = "status"
	OniHandsForget    = "forget"
	OniHandsWindows   = "windows"
	OniHandsSelect    = "select"
	OniHandsTasks     = "tasks"
	OniHandsClear     = "clear"
)

// OniHandsActions are every oni.hands action.
var OniHandsActions = []string{
	OniHandsOpen, OniHandsTabs, OniHandsAuthorize, OniHandsStart, OniHandsStop, OniHandsResume, OniHandsStatus, OniHandsForget,
	OniHandsWindows, OniHandsSelect, OniHandsTasks, OniHandsClear,
}

// Limits on oni.hands values.
const (
	// OniHandsGoalMax bounds a task's goal, in runes.
	OniHandsGoalMax = 300
	// OniHandsTabIDMax bounds a tab's id, in bytes.
	OniHandsTabIDMax = 64
	// OniHandsTitleMax bounds a tab's title, in runes.
	OniHandsTitleMax = 200
	// OniHandsOriginMax bounds an origin, in bytes.
	OniHandsOriginMax = 255
)

// Where Hands stands, as oni.hands answers: no Hands browser is open; it is
// open with no tab authorized; a tab is authorized and no task runs; or a
// task is working, waiting on a handoff card for the person, or paused.
const (
	OniHandsStateClosed     = "closed"
	OniHandsStateOpen       = "open"
	OniHandsStateAuthorized = "authorized"
	OniHandsStateWorking    = "working"
	OniHandsStateWaiting    = "waiting"
	OniHandsStatePaused     = "paused"
)

// OniHandsStates are every state oni.hands reports.
var OniHandsStates = []string{OniHandsStateClosed, OniHandsStateOpen, OniHandsStateAuthorized, OniHandsStateWorking, OniHandsStateWaiting, OniHandsStatePaused}

// OniHandsPhases are the states of a running task, the phases an active
// hands frame shows.
var OniHandsPhases = []string{OniHandsStateWorking, OniHandsStateWaiting, OniHandsStatePaused}

// Where Hands works: one browser tab at the browser stage; one live window
// the person picked, or Hyakki's own nested desktop, at the app stage; the
// nested desktop at the desktop stage. The person's whole live desktop is
// never a scope.
const (
	OniHandsScopeBrowser = "browser"
	OniHandsScopeWindow  = "window"
	OniHandsScopeNested  = "nested"
	OniHandsScopeDesktop = "desktop"
)

// OniHandsScopes are every scope, in a fixed order.
var OniHandsScopes = []string{OniHandsScopeBrowser, OniHandsScopeWindow, OniHandsScopeNested, OniHandsScopeDesktop}

// OniHandsStageScopes says which scopes each stage works in; a stage not
// named works in none.
var OniHandsStageScopes = map[string][]string{
	OniHandsBrowser: {OniHandsScopeBrowser},
	OniHandsApp:     {OniHandsScopeWindow, OniHandsScopeNested},
	OniHandsDesktop: {OniHandsScopeDesktop},
}

// OniHandsStageScope says whether a task at stage may work in scope.
func OniHandsStageScope(stage, scope string) bool {
	return slices.Contains(OniHandsStageScopes[stage], scope)
}

// Why a task is paused. Each names a rule, never page or window text. Only
// the person's Resume button goes on from a pause.
const (
	// OniHandsPauseFocus: another window, or none, took focus.
	OniHandsPauseFocus = "focus"
	// OniHandsPauseWindow: the windows could not be read, or the target can
	// no longer be told apart.
	OniHandsPauseWindow = "window"
	// OniHandsPauseNavigation: the tab went to another origin.
	OniHandsPauseNavigation = "navigation"
	// OniHandsPauseHidden: the tab or its window went out of view.
	OniHandsPauseHidden = "hidden"
	// OniHandsPauseCovered: the target is covered, fullscreen over Oni, or
	// Oni stepped aside, so Hands cannot be seen working.
	OniHandsPauseCovered = "covered"
	// OniHandsPauseDialog, OniHandsPausePopup and OniHandsPauseDownload:
	// the page opened a dialog, a popup or a download, none of which Hands
	// ever acts on.
	OniHandsPauseDialog   = "dialog"
	OniHandsPausePopup    = "popup"
	OniHandsPauseDownload = "download"
	// The guarded app and desktop scopes' reasons: the target moved to
	// another output, its title or geometry changed, one of Hyakki's own or
	// a private window took focus or covers it, the person pressed a key,
	// or no presenter is drawn.
	OniHandsPauseOutput    = "output"
	OniHandsPauseTitle     = "title"
	OniHandsPauseGeometry  = "geometry"
	OniHandsPauseSelf      = "self"
	OniHandsPausePrivate   = "private"
	OniHandsPauseKey       = "key"
	OniHandsPausePresenter = "presenter"
)

// OniHandsBrowserPauses are the reasons a task in the browser tab pauses.
// A lost presenter or a closed tab stops a browser task rather than pausing
// it.
var OniHandsBrowserPauses = []string{
	OniHandsPauseFocus, OniHandsPauseWindow, OniHandsPauseNavigation, OniHandsPauseHidden,
	OniHandsPauseCovered, OniHandsPauseDialog, OniHandsPausePopup, OniHandsPauseDownload,
}

// OniHandsAppPauses are the reasons a task outside a browser tab pauses: in
// a live window or on the nested desktop.
var OniHandsAppPauses = []string{
	OniHandsPauseFocus, OniHandsPauseWindow, OniHandsPauseHidden, OniHandsPauseCovered,
	OniHandsPauseOutput, OniHandsPauseTitle, OniHandsPauseGeometry, OniHandsPauseSelf,
	OniHandsPausePrivate, OniHandsPauseKey, OniHandsPausePresenter,
}

// OniHandsPauses are every pause reason of every scope.
var OniHandsPauses = append(slices.Clone(OniHandsBrowserPauses),
	OniHandsPauseOutput, OniHandsPauseTitle, OniHandsPauseGeometry, OniHandsPauseSelf,
	OniHandsPausePrivate, OniHandsPauseKey, OniHandsPausePresenter,
)

// OniHandsScopePauses are the pause reasons of scope.
func OniHandsScopePauses(scope string) []string {
	if scope == OniHandsScopeBrowser {
		return OniHandsBrowserPauses
	}
	return OniHandsAppPauses
}

// How a task ended, as the inactive hands frame that ends it says: a rule's
// word, never page or window text. Done is the only outcome of a task that
// reached its goal; every other word is a stop, a limit, a lapse or a
// failure.
const (
	// OniHandsEndDone: the task reached its goal and the page agreed.
	OniHandsEndDone = "done"
	// OniHandsEndAsked and OniHandsEndAnswered: Hands asked the person
	// something, or answered without acting.
	OniHandsEndAsked    = "asked"
	OniHandsEndAnswered = "answered"
	// OniHandsEndStopped: a stop, from anyone.
	OniHandsEndStopped = "stopped"
	// OniHandsEndBudget, OniHandsEndRepeat, OniHandsEndUnconfirmed and
	// OniHandsEndRefused: the step budget ran out, the same action was asked
	// for too often, too many "done" replies the page contradicted, or the
	// reply contract was broken twice in a row.
	OniHandsEndBudget      = "budget"
	OniHandsEndRepeat      = "repeat"
	OniHandsEndUnconfirmed = "unconfirmed"
	OniHandsEndRefused     = "refused"
	// OniHandsEndExpired and OniHandsEndAway: a handoff card lapsed, or a
	// pause was not resumed in time.
	OniHandsEndExpired = "expired"
	OniHandsEndAway    = "away"
	// OniHandsEndTimeout, OniHandsEndFailed, OniHandsEndClosed and
	// OniHandsEndUnread: a step took too long or failed, the tab or window
	// closed, or typed text could not be read back.
	OniHandsEndTimeout = "timeout"
	OniHandsEndFailed  = "failed"
	OniHandsEndClosed  = "closed"
	OniHandsEndUnread  = "unread"
)

// OniHandsEnds are every way a task ends, in the loop's order.
var OniHandsEnds = []string{
	OniHandsEndDone, OniHandsEndAsked, OniHandsEndAnswered, OniHandsEndStopped, OniHandsEndBudget,
	OniHandsEndRepeat, OniHandsEndUnconfirmed, OniHandsEndRefused, OniHandsEndExpired, OniHandsEndAway,
	OniHandsEndTimeout, OniHandsEndFailed, OniHandsEndClosed, OniHandsEndUnread,
}

// OniHandsParams is one oni.hands action. Tab, a tab's id from tabs, comes
// with authorize only; Window, a window's id from windows, with select
// only. Goal, what the user wants done, comes with start, which may also
// name its Scope (empty: the stage's own) and the Task, the id of a waiting
// or interrupted journal row, it starts. The goal is never kept: a task
// started again is started with its goal said again.
type OniHandsParams struct {
	Action string `json:"action"`
	Tab    string `json:"tab,omitempty"`
	Goal   string `json:"goal,omitempty"`
	Window string `json:"window,omitempty"`
	Scope  string `json:"scope,omitempty"`
	Task   string `json:"task,omitempty"`
}

// OniHandsTab is one tab of the Hands browser: its id, its origin and its
// title. Title is page text: one line, stripped of control and formatting
// characters, at most OniHandsTitleMax runes, and never kept.
type OniHandsTab struct {
	ID     string `json:"id"`
	Origin string `json:"origin"`
	Title  string `json:"title"`
}

// OniHandsWindow is one live window a task may work in: its id, its app id
// and its title, and whether Hands may click in it (the desktop reports
// where it is) or only type and press keys. Title is window text: one line,
// stripped, at most OniHandsTitleMax runes, never kept, sent to the app
// alone.
type OniHandsWindow struct {
	ID        string `json:"id"`
	App       string `json:"app"`
	Title     string `json:"title"`
	Clickable bool   `json:"clickable"`
}

// OniHandsTaskRow is one Hands journal row: the task's id, its state, its
// scope, the app id it works in, where it came from, its step, why it ended
// or paused, and when it was queued and ended. It holds no goal, title, URL
// or typed word.
type OniHandsTaskRow struct {
	ID     string    `json:"id"`
	State  string    `json:"state"`
	Scope  string    `json:"scope"`
	App    string    `json:"app,omitempty"`
	Origin string    `json:"origin"`
	Step   int       `json:"step"`
	At     time.Time `json:"at"`
	Ended  time.Time `json:"ended,omitzero"`
	Reason string    `json:"reason,omitempty"`
}

// OniHandsResult answers oni.hands with where Hands stands after the call.
// Browser names the Chromium-family program Hands would open, by its
// command's name (ValidOniHandsBrowser), on every answer; it is empty when
// none is installed. Tabs comes with tabs only. Origin is the authorized
// tab's while one is authorized. Step of Max and Pause come with a running
// task, Pause only while it is paused. Line is what Oni says about it, if
// anything. At the app and desktop stages Scope is the running task's, App
// the selected window's app id, Task the running task's id, Windows comes
// with windows only and Tasks with tasks, and Missing names the programs
// the nested desktop needs and this computer lacks.
type OniHandsResult struct {
	State   string            `json:"state"`
	Browser string            `json:"browser,omitempty"`
	Tabs    []OniHandsTab     `json:"tabs,omitempty"`
	Origin  string            `json:"origin,omitempty"`
	Step    int               `json:"step,omitempty"`
	Max     int               `json:"max,omitempty"`
	Pause   string            `json:"pause,omitempty"`
	Line    string            `json:"line,omitempty"`
	Scope   string            `json:"scope,omitempty"`
	App     string            `json:"app,omitempty"`
	Task    string            `json:"task,omitempty"`
	Windows []OniHandsWindow  `json:"windows,omitempty"`
	Tasks   []OniHandsTaskRow `json:"tasks,omitempty"`
	Missing []string          `json:"missing,omitempty"`
}

// ValidOniHandsBrowser reports whether name can name the Hands browser's
// program: a command's base name of 1 to 32 lowercase letters, digits, '-'
// and '_', such as "chromium" or "google-chrome-stable".
func ValidOniHandsBrowser(name string) bool { return oniHandsCode(name) }

// What became of one Hands action, as its ledger row says: a safe step
// ran; it was handed to the person, who presses it themselves; it was
// refused before anything touched the page; or it failed.
const (
	OniHandsOutcomeDone    = "done"
	OniHandsOutcomeHandoff = "handoff"
	OniHandsOutcomeRefused = "refused"
	OniHandsOutcomeFailed  = "failed"
)

// OniHandsOutcomes are every ledger outcome.
var OniHandsOutcomes = []string{OniHandsOutcomeDone, OniHandsOutcomeHandoff, OniHandsOutcomeRefused, OniHandsOutcomeFailed}

// OniHandsRow is one Hands ledger row: when, the task's short id, the step,
// the tool asked for, its outcome, the rule's code behind a handoff,
// refusal or failure, and how long it took. It holds no control name, no
// page text and no typed word.
type OniHandsRow struct {
	At      time.Time `json:"at"`
	Task    string    `json:"task"`
	Step    int       `json:"step"`
	Tool    string    `json:"tool"`
	Outcome string    `json:"outcome"`
	Reason  string    `json:"reason,omitempty"`
	Ms      int64     `json:"ms"`
}

// OniHandsLogResult answers oni.hands.log with the ledger, newest first.
type OniHandsLogResult struct {
	Rows []OniHandsRow `json:"rows"`
}

// ValidOniHandsTabID reports whether id can name a Hands browser tab: 1 to
// OniHandsTabIDMax ASCII letters and digits.
func ValidOniHandsTabID(id string) bool {
	if id == "" || len(id) > OniHandsTabIDMax {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// ValidOniHandsWindowID reports whether id can name a live window to
// select: 1 to OniHandsTabIDMax ASCII letters and digits, as niri's and
// Hyprland's window ids are.
func ValidOniHandsWindowID(id string) bool { return ValidOniHandsTabID(id) }

// OniHandsAppMax bounds an app id or an output name, in bytes.
const OniHandsAppMax = 128

// ValidOniHandsApp reports whether s is shaped like an app id or an output
// name: 1 to OniHandsAppMax ASCII letters and digits joined by '.', '-',
// '_' or '+', starting with a letter or digit. No title or URL has it.
func ValidOniHandsApp(s string) bool {
	if s == "" || len(s) > OniHandsAppMax {
		return false
	}
	for i := range len(s) {
		c := s[i]
		alnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if i == 0 && !alnum || !alnum && c != '.' && c != '-' && c != '_' && c != '+' {
			return false
		}
	}
	return true
}

// OniHandsTaskIDBytes is the length of a Hands journal task id.
const OniHandsTaskIDBytes = 32

// ValidOniHandsTaskID reports whether id can name a journal task:
// OniHandsTaskIDBytes lowercase hex digits.
func ValidOniHandsTaskID(id string) bool {
	if len(id) != OniHandsTaskIDBytes {
		return false
	}
	for i := range len(id) {
		if c := id[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ValidOniHandsOrigin reports whether origin is one Hands may bind: "file://"
// or an http or https scheme://host[:port], lowercase, with no user, path,
// query or fragment.
func ValidOniHandsOrigin(origin string) bool {
	if origin == "file://" {
		return true
	}
	if len(origin) > OniHandsOriginMax {
		return false
	}
	host, ok := strings.CutPrefix(origin, "https://")
	if !ok {
		host, ok = strings.CutPrefix(origin, "http://")
	}
	if !ok || host == "" || host != strings.ToLower(host) {
		return false
	}
	for _, r := range host {
		if r > unicode.MaxASCII || unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune("/?#@\\", r) {
			return false
		}
	}
	return true
}

// ValidOniHandsTab reports whether t can go on the wire: a valid id and
// origin, and a title of one line of at most OniHandsTitleMax runes, empty
// for a page with none.
func ValidOniHandsTab(t OniHandsTab) bool {
	return ValidOniHandsTabID(t.ID) && ValidOniHandsOrigin(t.Origin) && (t.Title == "" || ValidOniText(t.Title, OniHandsTitleMax, false))
}

// ValidOniHandsRow reports whether r is a ledger row that names only codes:
// a task id and a tool of lowercase letters, digits, '-' and '_', a known
// outcome, a reason code or none, and no negative step or time.
func ValidOniHandsRow(r OniHandsRow) bool {
	return oniHandsCode(r.Task) && oniHandsCode(r.Tool) && slices.Contains(OniHandsOutcomes, r.Outcome) &&
		(r.Reason == "" || oniHandsCode(r.Reason)) && r.Step >= 0 && r.Ms >= 0 && !r.At.IsZero()
}

func oniHandsCode(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}
