package spec

import "slices"

// GuidePhaseIDs are the phases a guided walkthrough moves through (see
// Guide.Phases).
var GuidePhaseIDs = []string{"idle", "reading", "pointing", "waiting", "explaining", "keeping", "done", "winding", "nudging"}

// Guide holds the guided walkthrough's numbers: the step ceiling, the one
// read timeout, the select-and-ask repeat guard, the Keep store, the settle
// watcher, proactive hints and the guide phase to character state map.
type Guide struct {
	MaxSteps          int                   `json:"maxSteps"`
	ReadTimeoutS      float64               `json:"readTimeoutS"`
	PickRepeatGuardMs float64               `json:"pickRepeatGuardMs"`
	Keep              GuideKeep             `json:"keep"`
	Settle            GuideSettle           `json:"settle"`
	Hints             GuideHints            `json:"hints"`
	Phases            map[string]GuidePhase `json:"phases"`
}

// GuideKeep: the Keep offer lapses to Skip after ChipS; the store holds at
// most MaxEntries walkthroughs, least recently used out first; a request
// matches a kept goal at a similarity of at least MatchMin; a replayed step
// finds its control at a score of at least FindMin out of 100; an entry
// missed MissesToStale times in a row retires.
type GuideKeep struct {
	ChipS         float64 `json:"chipS"`
	MaxEntries    int     `json:"maxEntries"`
	MatchMin      float64 `json:"matchMin"`
	FindMin       int     `json:"findMin"`
	MissesToStale int     `json:"missesToStale"`
}

// GuideSettle is the watcher that runs after an advance: it reduces the
// window to WidthPx×HeightPx gray fingerprints every FrameMs; a mean change
// of at least MoveDelta has moved, at most StillDelta is still. NapMs is the
// fallback wait when no fingerprint can be taken, and the watcher stops
// after MaxFrames frames.
type GuideSettle struct {
	WidthPx    int     `json:"widthPx"`
	HeightPx   int     `json:"heightPx"`
	MoveDelta  float64 `json:"moveDelta"`
	StillDelta float64 `json:"stillDelta"`
	FrameMs    float64 `json:"frameMs"`
	NapMs      float64 `json:"napMs"`
	MaxFrames  int     `json:"maxFrames"`
}

// GuideHints: at most one hint every CooldownS; an untouched hint retracts
// after RetractS; with OncePerKeyPerDay a kept walkthrough is hinted at most
// once a day.
type GuideHints struct {
	CooldownS        float64 `json:"cooldownS"`
	RetractS         float64 `json:"retractS"`
	OncePerKeyPerDay bool    `json:"oncePerKeyPerDay"`
}

// GuidePhase is the character state a guide phase derives. A phase that
// derives none (NoState) states its Reason; a phase with a state has none.
type GuidePhase struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// GuideState returns the character state a guide phase derives (NoState
// when the phase leaves the state alone).
func (s *OniSpec) GuideState(phase string) (state string, ok bool) {
	p, ok := s.Guide.Phases[phase]
	return p.State, ok
}

// SettleBudgetMs is the longest the settle watcher runs after one advance.
func (g GuideSettle) SettleBudgetMs() float64 {
	return float64(g.MaxFrames) * g.FrameMs
}

func (s *OniSpec) checkGuide(bad func(string, ...any)) {
	g := s.Guide
	if g.MaxSteps < 1 {
		bad("guide.maxSteps: must be at least 1")
	}
	for _, d := range []struct {
		path string
		v    float64
	}{
		{"guide.readTimeoutS", g.ReadTimeoutS},
		{"guide.pickRepeatGuardMs", g.PickRepeatGuardMs},
		{"guide.keep.chipS", g.Keep.ChipS},
		{"guide.settle.stillDelta", g.Settle.StillDelta},
		{"guide.settle.frameMs", g.Settle.FrameMs},
		{"guide.hints.cooldownS", g.Hints.CooldownS},
		{"guide.hints.retractS", g.Hints.RetractS},
	} {
		if !(d.v > 0) {
			bad("%s: must be positive", d.path)
		}
	}

	k := g.Keep
	if k.MaxEntries < 1 {
		bad("guide.keep.maxEntries: must be at least 1")
	}
	if !(k.MatchMin > 0 && k.MatchMin <= 1) {
		bad("guide.keep.matchMin: must lie in (0, 1]")
	}
	if !(k.FindMin > 0 && k.FindMin <= 100) {
		bad("guide.keep.findMin: must lie in (0, 100]")
	}
	if k.MissesToStale < 1 {
		bad("guide.keep.missesToStale: must be at least 1")
	}

	st := g.Settle
	if st.WidthPx < 1 || st.HeightPx < 1 {
		bad("guide.settle: a fingerprint needs widthPx and heightPx of at least 1")
	}
	if !(st.MoveDelta > st.StillDelta && st.MoveDelta <= 255) {
		bad("guide.settle.moveDelta: must lie in (stillDelta, 255]")
	}
	if !(st.NapMs >= st.FrameMs) {
		bad("guide.settle.napMs: must last at least one frame")
	}
	if st.MaxFrames < 1 {
		bad("guide.settle.maxFrames: must be at least 1")
	} else if !(st.SettleBudgetMs() >= st.NapMs) {
		bad("guide.settle.maxFrames: the watcher must outlast the nap (maxFrames·frameMs >= napMs)")
	}

	if !(g.Hints.RetractS < g.Hints.CooldownS) {
		bad("guide.hints.retractS: a hint must retract before the cooldown ends")
	}

	if err := sameSet("guide.phases", g.Phases, GuidePhaseIDs); err != nil {
		bad("%v", err)
	}
	for _, id := range sortedKeys(g.Phases) {
		p := g.Phases[id]
		switch {
		case p.State != NoState && !slices.Contains(StateIDs, p.State):
			bad("guide.phases.%s.state: unknown state %q", id, p.State)
		case p.State == NoState && p.Reason == "":
			bad("guide.phases.%s: a phase that derives no state must give a reason", id)
		case p.State != NoState && p.Reason != "":
			bad("guide.phases.%s.reason: only a phase that derives no state gives a reason", id)
		}
	}
	if idle, ok := g.Phases["idle"]; ok && idle.State != NoState {
		bad("guide.phases.idle: an idle guide derives no state")
	}
}
