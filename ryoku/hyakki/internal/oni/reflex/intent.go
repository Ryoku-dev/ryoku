// Package reflex turns a short spoken command into a plan: a bounded list
// of reversible actions taken from a fixed registry, and one short line Oni
// says about them. It is pure: it reads no file, starts no process, opens no
// socket and never looks at a clock. Every time it needs is handed in, and
// the daemon carries out a plan through executors that live elsewhere.
//
// Nothing in the registry sends, buys, pays, deletes, submits, posts,
// installs or removes anything, and no intent types into another window.
package reflex

import (
	"slices"

	"ryoku-rashin/internal/oni/feature"
)

// Intent is one registry id.
type Intent string

// The registry ids. Tab actions are limited to opening a new tab through
// the browser's own launcher; closing or moving between tabs would need key
// chords sent into another window, so Oni says it can't do that here.
const (
	OpenApp       Intent = "open_app"       // Arg: the catalog name
	OpenSite      Intent = "open_site"      // Arg: an https address
	WebSearch     Intent = "web_search"     // Arg: the words to search for
	NewTab        Intent = "new_tab"        // Arg: empty
	SnapLeft      Intent = "snap_left"      // Arg: empty
	SnapRight     Intent = "snap_right"     // Arg: empty
	Maximize      Intent = "maximize"       // Arg: empty
	Minimize      Intent = "minimize"       // Arg: empty
	VolumeUp      Intent = "volume_up"      // Arg: empty
	VolumeDown    Intent = "volume_down"    // Arg: empty
	Mute          Intent = "mute"           // Arg: MuteOn or MuteOff
	PlayPause     Intent = "play_pause"     // Arg: MediaPlay, MediaPause or MediaToggle
	Screenshot    Intent = "screenshot"     // Arg: empty
	RememberNote  Intent = "remember_note"  // Arg: the words to remember
	RunRoutine    Intent = "run_routine"    // Arg: a routine name, or empty for the latest
	SetTimer      Intent = "set_timer"      // Arg: the whole clause, for the reminder parser
	SetReminder   Intent = "set_reminder"   // Arg: the whole clause, for the reminder parser
	CancelTimer   Intent = "cancel_timer"   // Arg: CancelTimers, CancelReminder, CancelReminders or CancelAll
	ListReminders Intent = "list_reminders" // Arg: empty
	SelfMove      Intent = "self_move"      // Arg: a corner pin, "tl", "tr", "bl" or "br"
	SelfSize      Intent = "self_size"      // Arg: SizeBigger or SizeSmaller
	SelfHome      Intent = "self_home"      // Arg: empty
	SettingFlip   Intent = "setting_flip"   // Arg: "key=value", see Flips
)

// Arg values for the intents that take a fixed word.
const (
	MuteOn  = "on"
	MuteOff = "off"

	MediaPlay   = "play"
	MediaPause  = "pause"
	MediaToggle = "toggle"

	CancelTimers    = "timers"
	CancelReminder  = "reminder" // the next one due only
	CancelReminders = "reminders"
	CancelAll       = "all"

	SizeBigger  = "bigger"
	SizeSmaller = "smaller"
)

// Info describes one intent. Stage is always the reflex feature's stage.
// A Replayable intent may be kept as a step of a routine; a PressOnly
// intent is planned only for an utterance that came with a Press (the
// chord or the toggle), never from wake or open mic alone.
type Info struct {
	ID         Intent
	Stage      string
	Replayable bool
	PressOnly  bool
}

var registry = []Info{
	{OpenApp, feature.StageReversible, true, false},
	{OpenSite, feature.StageReversible, true, false},
	{WebSearch, feature.StageReversible, true, false},
	{NewTab, feature.StageReversible, true, false},
	{SnapLeft, feature.StageReversible, true, false},
	{SnapRight, feature.StageReversible, true, false},
	{Maximize, feature.StageReversible, true, false},
	{Minimize, feature.StageReversible, true, false},
	{VolumeUp, feature.StageReversible, true, false},
	{VolumeDown, feature.StageReversible, true, false},
	{Mute, feature.StageReversible, true, false},
	{PlayPause, feature.StageReversible, true, false},
	{Screenshot, feature.StageReversible, true, false},
	{RememberNote, feature.StageReversible, false, true},
	{RunRoutine, feature.StageReversible, false, true},
	{SetTimer, feature.StageReversible, false, false},
	{SetReminder, feature.StageReversible, false, false},
	{CancelTimer, feature.StageReversible, false, false},
	{ListReminders, feature.StageReversible, false, false},
	{SelfMove, feature.StageReversible, false, false},
	{SelfSize, feature.StageReversible, false, false},
	{SelfHome, feature.StageReversible, false, false},
	{SettingFlip, feature.StageReversible, false, true},
}

// Registry returns every intent, in a fixed order. The slice is a copy.
func Registry() []Info { return slices.Clone(registry) }

// Lookup returns the registry entry for id.
func Lookup(id Intent) (Info, bool) {
	i := slices.IndexFunc(registry, func(in Info) bool { return in.ID == id })
	if i < 0 {
		return Info{}, false
	}
	return registry[i], true
}

// Action is one planned step. Confidence is 0..1.
type Action struct {
	Intent     Intent
	Arg        string
	Confidence float64
}

// Plan is what one utterance asks for. Actions holds at most the spec's
// reflex.maxActions entries; Line is what Oni says, one sentence of at most
// five words. A plan with no actions and a Line is an honest refusal ("I
// can't do that here."); a plan with neither is a verb still waiting for
// its object (see Session).
type Plan struct {
	Actions []Action
	Line    string
}
