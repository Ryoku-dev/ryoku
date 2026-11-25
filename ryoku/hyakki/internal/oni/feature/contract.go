// Package feature is Oni's feature contract and its policy: which features
// exist, what each may send off the computer, the consent each needs, and
// whether the settings allow it. It never sees a character profile, so a
// character changes how Oni looks and talks, never what Oni may do.
package feature

import "ryoku-rashin/internal/protocol"

// Feature is one entry of the contract.
type Feature = protocol.OniFeature

// CapabilitySet is every feature's decision under one set of settings.
type CapabilitySet = protocol.OniCapabilitySet

// Stages say how far a feature reaches in this build.
const (
	// StageReadOnly observes, answers and speaks; it drives no input.
	StageReadOnly = "read-only"

	// StageReversible acts only through reversible seams such as launching
	// an app, snapping a window or changing the volume.
	StageReversible = "reversible"

	// StageGuarded drives input, but only inside one scope the user picked
	// and authorized, such as one browser tab: safe steps are announced and
	// run, anything risky is handed to the user, and every stop path ends
	// it at once.
	StageGuarded = "guarded"

	// StageUnavailable is not compiled into this build; no setting turns it
	// on.
	StageUnavailable = "unavailable"
)

// Egress classes say where a feature's data may go.
const (
	// EgressLocal keeps everything on this computer.
	EgressLocal = "local"

	// EgressDisclosed may reach the model route the user chose, hosted or
	// local; what leaves and where is shown before it is sent.
	EgressDisclosed = "disclosed"
)

// Feature ids.
const (
	Inspect        = "vision.inspect"
	Voice          = "voice"
	VoiceDictation = "voice.dictation"
	Reflex         = "reflex"
	FeedGuard      = "feeds.guard"
	HandsBrowser   = "hands.browser"
	HandsApp       = "hands.app"
	HandsDesktop   = "hands.desktop"
	Guide          = "guide"
	GuideKeep      = "guide.keep"
	GuideHints     = "guide.hints"
	// The reflex family: each needs Reflex and its own switch under
	// oni.reflexSettings.
	Routines  = "reflex.routines"
	Reminders = "reflex.reminders"
	Notes     = "reflex.notes"
	Decisions = "reflex.decisions"
	// Phase 16: forgetting a memory fact by voice, recall, rewrite in place
	// and assignment mode.
	MemoryForget = "memory.forget"
	Recall       = "recall"
	Rewrite      = "rewrite"
	Assignment   = "assignment"
)

// askEgress is where a typed question about a window may go. A voice ask is
// the same Ask turn on the same route, so voice shares it; dictation types
// into the user's window and only a polish chosen in Settings leaves, shown
// first like any Ask.
const askEgress = EgressDisclosed

var features = []Feature{
	{ID: Inspect, Stage: StageReadOnly, Setting: "oni.inspect", Egress: askEgress, Consent: ConsentDisclosure},
	{ID: Voice, Stage: StageReadOnly, Setting: "oni.voice", Egress: askEgress, Consent: ConsentUserInitiated},
	{ID: VoiceDictation, Stage: StageReversible, Setting: "oni.voiceSettings.dictation.enabled", Egress: askEgress, Consent: ConsentUserInitiated},
	{ID: Reflex, Stage: StageReversible, Setting: "oni.reflex", Egress: EgressDisclosed, Consent: ConsentAnnounced},
	{ID: FeedGuard, Stage: StageReversible, Setting: "oni.feeds.guard", Egress: EgressLocal, Consent: ConsentPerAction},
	// A routine is kept only on a yes to its offer and replays with no
	// model. Reminders and notes stay on this computer and announce
	// themselves. A decision may reach the hosted credit route, shown on a
	// disclosure card first; the local route sends nothing off the computer.
	{ID: Routines, Stage: StageReversible, Setting: "oni.reflexSettings.routines", Egress: EgressLocal, Consent: ConsentPerAction},
	{ID: Reminders, Stage: StageReversible, Setting: "oni.reflexSettings.reminders", Egress: EgressLocal, Consent: ConsentAnnounced},
	{ID: Notes, Stage: StageReversible, Setting: "oni.reflexSettings.notes", Egress: EgressLocal, Consent: ConsentAnnounced},
	{ID: Decisions, Stage: StageReversible, Setting: "oni.reflexSettings.decisions", Egress: EgressDisclosed, Consent: ConsentDisclosure},
	// A guide sends one picture of the perched window per step to the Ask
	// route, shown first like any screen question. Keeping a guide and
	// replaying it read the window on this computer and send nothing, and a
	// hint comes from what was kept, never from a picture.
	{ID: Guide, Stage: StageReadOnly, Setting: "oni.guide.enabled", Egress: askEgress, Consent: ConsentDisclosure},
	{ID: GuideKeep, Stage: StageReadOnly, Setting: "oni.guide.keep", Egress: EgressLocal, Consent: ConsentPerAction},
	{ID: GuideHints, Stage: StageReadOnly, Setting: "oni.guide.hints", Egress: EgressLocal, Consent: ConsentAnnounced},
	// Forgetting a fact by voice names the facts on a card and forgets them
	// only on a yes; it shares the notes switch, as correcting the last
	// note does. Recall sends the passages it found to the Ask route: a
	// local route sends nothing off the computer, a hosted one shows their
	// titles and the route first. A rewrite shows the exact selection and
	// the route before it is sent, then types the result back over the
	// selection. Assignment mode shows the picture and the route before
	// each problem is sent.
	{ID: MemoryForget, Stage: StageReversible, Setting: "oni.reflexSettings.notes", Egress: EgressLocal, Consent: ConsentPerAction},
	{ID: Recall, Stage: StageReadOnly, Setting: "oni.recall.enabled", Egress: EgressDisclosed, Consent: ConsentDisclosure},
	{ID: Rewrite, Stage: StageReversible, Setting: "oni.rewrite.enabled", Egress: EgressDisclosed, Consent: ConsentDisclosure},
	{ID: Assignment, Stage: StageReadOnly, Setting: "oni.assignment.enabled", Egress: EgressDisclosed, Consent: ConsentDisclosure},
	// Hands in a browser works only in the one tab the user authorized, and
	// sends a picture of it with its text and control names on the Ask
	// route, shown first like any screen question. Each action is sorted on
	// its own: a safe step is announced and run, and anything risky is
	// handed to the user, who presses it themselves. At the app stage Hands
	// works in one live window the user picked for the task, or on Hyakki's
	// own nested desktop; at the desktop stage only on the nested desktop.
	// The stages are exclusive, and none reaches the user's whole live
	// desktop.
	{ID: HandsBrowser, Stage: StageGuarded, Setting: "oni.hands", Egress: EgressDisclosed, Consent: ConsentFirm},
	{ID: HandsApp, Stage: StageGuarded, Setting: "oni.hands", Egress: EgressDisclosed, Consent: ConsentFirm},
	{ID: HandsDesktop, Stage: StageGuarded, Setting: "oni.hands", Egress: EgressDisclosed, Consent: ConsentFirm},
}

// Features returns the contract in a fixed order. The slice is a copy.
func Features() []Feature {
	return append([]Feature(nil), features...)
}

// Lookup returns the contract entry for id.
func Lookup(id string) (Feature, bool) {
	for _, f := range features {
		if f.ID == id {
			return f, true
		}
	}
	return Feature{}, false
}
