package feature

import "ryoku-rashin/internal/protocol"

// Request asks whether a feature may run.
type Request struct {
	Feature string `json:"feature"`
}

// Decision is the policy's answer. Reason is one of the Reason codes.
type Decision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
}

// Reason codes, in the order Decide checks them.
const (
	ReasonUnknown     = "unknown-feature"
	ReasonUnavailable = "unavailable"
	ReasonOniOff      = "oni-off"
	ReasonFeatureOff  = "feature-off"
	ReasonAllowed     = "allowed"
)

// Decide answers r under s. Pass protocol.EffectiveOni so Hyakki's own switch
// counts. A feature not compiled into this build is unavailable whatever the
// settings say; the hands stage setting counts only for the one stage in
// it: the stages are exclusive, with no ladder.
func Decide(r Request, s protocol.OniSettings) Decision {
	f, ok := Lookup(r.Feature)
	if !ok {
		return Decision{Reason: ReasonUnknown}
	}
	if f.Stage == StageUnavailable {
		return Decision{Reason: ReasonUnavailable}
	}
	if !s.Master {
		return Decision{Reason: ReasonOniOff}
	}
	if !switchOn(f.ID, s) {
		return Decision{Reason: ReasonFeatureOff}
	}
	return Decision{Allow: true, Reason: ReasonAllowed}
}

// switchOn is a feature's own switch together with the ones it builds on: a
// guide reads the window, so it needs looking at windows; Keep keeps and
// replays guides, so it needs the guide; a hint offers what was kept, so it
// needs Keep. Routines, reminders, notes and decisions are reflexes, so
// each needs Reflex. Hands looks at what it works in as a guide looks at
// its window, so every stage needs looking at windows too; each Hands row
// is on only at its own stage. Forgetting by voice shares the notes
// switch; assignment mode reads the window, so it needs looking at windows.
func switchOn(id string, s protocol.OniSettings) bool {
	switch id {
	case Inspect:
		return s.Inspect
	case Voice:
		return s.Voice
	case VoiceDictation:
		return s.VoiceSettings.Dictation.Enabled
	case Reflex:
		return s.Reflex
	case FeedGuard:
		return s.Feeds.Guard
	case Guide:
		return s.Guide.Enabled && s.Inspect
	case GuideKeep:
		return s.Guide.Keep && switchOn(Guide, s)
	case GuideHints:
		return s.Guide.Hints && switchOn(GuideKeep, s)
	case Routines:
		return s.ReflexSettings.Routines && s.Reflex
	case Reminders:
		return s.ReflexSettings.Reminders && s.Reflex
	case Notes:
		return s.ReflexSettings.Notes && s.Reflex
	case Decisions:
		return s.ReflexSettings.Decisions && s.Reflex
	case HandsBrowser:
		return s.Hands == protocol.OniHandsBrowser && s.Inspect
	case HandsApp:
		return s.Hands == protocol.OniHandsApp && s.Inspect
	case HandsDesktop:
		return s.Hands == protocol.OniHandsDesktop && s.Inspect
	case MemoryForget:
		return switchOn(Notes, s)
	case Recall:
		return s.Recall.Enabled
	case Rewrite:
		return s.Rewrite.Enabled
	case Assignment:
		return s.Assignment.Enabled && s.Inspect
	}
	return false
}

// Capabilities decides every feature in contract order.
func Capabilities(s protocol.OniSettings) CapabilitySet {
	set := CapabilitySet{Master: s.Master, Features: make([]protocol.OniCapability, 0, len(features))}
	for _, f := range features {
		d := Decide(Request{Feature: f.ID}, s)
		set.Features = append(set.Features, protocol.OniCapability{
			ID:      f.ID,
			Stage:   f.Stage,
			Egress:  f.Egress,
			Consent: f.Consent,
			Allow:   d.Allow,
			Reason:  d.Reason,
		})
	}
	return set
}
