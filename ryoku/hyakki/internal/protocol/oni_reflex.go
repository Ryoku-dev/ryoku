package protocol

import (
	"strings"
	"time"
)

// What oni.reminders does: list the reminders and timers kept, add one,
// cancel the one ID names, or cancel every one.
const (
	OniRemindersList      = "list"
	OniRemindersAdd       = "add"
	OniRemindersCancel    = "cancel"
	OniRemindersCancelAll = "cancel_all"
)

// OniRemindersActions are every oni.reminders action.
var OniRemindersActions = []string{OniRemindersList, OniRemindersAdd, OniRemindersCancel, OniRemindersCancelAll}

// OniReminderIDLen is the length of a reminder's id: lowercase hex.
const OniReminderIDLen = 16

// The kinds of a kept reminder: one with its own words, or a timer.
const (
	OniReminderKindReminder = "reminder"
	OniReminderKindTimer    = "timer"
)

// OniRemindersParams lists, adds or cancels reminders. ID, a reminder's
// id, comes with a cancel only. Kind, Due and Text come with an add only:
// a reminder needs its words, one line of at most OniReminderTextMax runes,
// and a timer may have them; the store keeps its own limits on how many
// wait and how far ahead.
type OniRemindersParams struct {
	Action string    `json:"action"`
	ID     string    `json:"id,omitempty"`
	Kind   string    `json:"kind,omitempty"`
	Due    time.Time `json:"due,omitzero"`
	Text   string    `json:"text,omitempty"`
}

// OniReminderInfo is one kept reminder or timer: its id, its kind, when it
// is due and its own words, empty for a timer that has none. Late marks one
// that came due while it could not be said. No transcript rides it.
type OniReminderInfo struct {
	ID   string    `json:"id"`
	Kind string    `json:"kind"`
	Due  time.Time `json:"due"`
	Text string    `json:"text,omitempty"`
	Late bool      `json:"late,omitempty"`
}

// OniRemindersResult answers oni.reminders with the reminders and timers
// kept after the call, soonest first. NoWake is set when the shared wake
// unit cannot be armed on this computer: the daemon then stays up while any
// reminder is pending, so one can still ring.
type OniRemindersResult struct {
	Reminders []OniReminderInfo `json:"reminders"`
	NoWake    bool              `json:"noWake,omitempty"`
}

// What oni.routines does: list the kept routines, replay one (the most
// recently used when no name is given), forget the one Name names, or
// forget every one.
const (
	OniRoutinesList      = "list"
	OniRoutinesRun       = "run"
	OniRoutinesForget    = "forget"
	OniRoutinesForgetAll = "forget_all"
)

// OniRoutinesActions are every oni.routines action.
var OniRoutinesActions = []string{OniRoutinesList, OniRoutinesRun, OniRoutinesForget, OniRoutinesForgetAll}

// OniRoutineNameMax bounds a routine's name, in runes.
const OniRoutineNameMax = 60

// OniRoutinesParams lists, replays or forgets routines. Name comes with a
// run, where it is optional, and a forget, where it is required.
type OniRoutinesParams struct {
	Action string `json:"action"`
	Name   string `json:"name,omitempty"`
}

// OniRoutineInfo is one kept routine: its name, how many steps it replays,
// when it was saved and when it was last used. Its steps' words stay in the
// store.
type OniRoutineInfo struct {
	Name  string    `json:"name"`
	Steps int       `json:"steps"`
	Saved time.Time `json:"saved"`
	Used  time.Time `json:"used"`
}

// OniRoutinesResult answers oni.routines with the routines kept after the
// call, most recently used first. Ran says a run replayed its routine.
type OniRoutinesResult struct {
	Routines []OniRoutineInfo `json:"routines"`
	Ran      bool             `json:"ran,omitempty"`
}

// What became of one planned reflex action, as its ledger row says: done;
// unsupported where this desktop cannot do it; refused by a rule, a missing
// press or a cap; held by the launch cooldown; skipped because its app is
// not installed; or failed.
const (
	OniReflexDone        = "done"
	OniReflexUnsupported = "unsupported"
	OniReflexRefused     = "refused"
	OniReflexCooldown    = "cooldown"
	OniReflexSkipped     = "skipped"
	OniReflexFailed      = "failed"
)

// OniReflexOutcomes are every ledger outcome.
var OniReflexOutcomes = []string{OniReflexDone, OniReflexUnsupported, OniReflexRefused, OniReflexCooldown, OniReflexSkipped, OniReflexFailed}

// OniReflexRow is one ledger row: when, the registry intent, the decision
// route that named it (DecisionRules, DecisionLocal or DecisionEmber), how
// long deciding took, its confidence and its outcome. It holds no words and
// no transcript.
type OniReflexRow struct {
	At         time.Time `json:"at"`
	Intent     string    `json:"intent"`
	Route      string    `json:"route"`
	Ms         int64     `json:"ms"`
	Confidence float64   `json:"confidence"`
	Outcome    string    `json:"outcome"`
}

// OniReflexLogResult answers oni.reflex.log with the ledger, newest first.
type OniReflexLogResult struct {
	Rows []OniReflexRow `json:"rows"`
}

// ValidOniReminderID reports whether id can name a kept reminder:
// OniReminderIDLen lowercase hex digits.
func ValidOniReminderID(id string) bool {
	if len(id) != OniReminderIDLen {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ValidOniRoutineName reports whether name can name a routine: trimmed,
// non-empty, one line of at most OniRoutineNameMax runes with no control or
// invisible formatting characters.
func ValidOniRoutineName(name string) bool {
	return name != "" && name == strings.TrimSpace(name) && ValidOniText(name, OniRoutineNameMax, false)
}
