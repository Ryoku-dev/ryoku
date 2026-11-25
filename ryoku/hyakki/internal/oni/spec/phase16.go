package spec

import "time"

// The Phase 16 sections hold the numbers for voice memory, recall, rewrite
// in place, the focus guard and assignment mode. They are draft values,
// not yet ratified; consumers read them here and none repeats them as a
// literal.

// Memory holds the voice memory numbers: "no, I meant" corrects the last
// note only within CorrectS seconds of saving it, and one forget card names
// at most ForgetMax facts.
type Memory struct {
	CorrectS  int `json:"correctS"`
	ForgetMax int `json:"forgetMax"`
}

// Recall bounds what one recall puts in front of the model: at most
// Passages passages, PassageTokens tokens each, PromptBytes bytes of fenced
// passages in all, and an answer of AnswerTokens tokens within RequestS
// seconds.
type Recall struct {
	Passages      int `json:"passages"`
	PassageTokens int `json:"passageTokens"`
	PromptBytes   int `json:"promptBytes"`
	AnswerTokens  int `json:"answerTokens"`
	RequestS      int `json:"requestS"`
}

// Rewrite bounds one rewrite in place: a selection of at most MaxRunes
// runes, one model request of RequestS seconds, and the original kept in
// memory for UndoS seconds after the result is typed back, then dropped.
type Rewrite struct {
	MaxRunes int `json:"maxRunes"`
	RequestS int `json:"requestS"`
	UndoS    int `json:"undoS"`
}

// Focus holds the focus guard's limits: it is armed for at most
// MaxMinutes minutes, and "let me back" reopens what it closed only within
// ReopenS seconds of the close. Its look comes from the renderer's palette
// roles, never from a colour here.
type Focus struct {
	MaxMinutes int `json:"maxMinutes"`
	ReopenS    int `json:"reopenS"`
}

// Assignment holds assignment mode's numbers. A new problem is first
// noticed on a Gate.WidthPx×Gate.HeightPx grey thumbnail. The last
// CacheViews solved views come back with no model request. One local day
// allows AttemptsPerDay solves and TokensPerDay measured tokens on a
// hosted or paid route. The fast lane gets FastS seconds and is re-sent
// once after HedgeS seconds only on a transient error; the checker lane
// gets CheckerS seconds and gives Steps.Min to Steps.Max steps.
type Assignment struct {
	Gate           AssignmentGate  `json:"gate"`
	CacheViews     int             `json:"cacheViews"`
	AttemptsPerDay int             `json:"attemptsPerDay"`
	TokensPerDay   int             `json:"tokensPerDay"`
	HedgeS         int             `json:"hedgeS"`
	FastS          int             `json:"fastS"`
	CheckerS       int             `json:"checkerS"`
	Steps          AssignmentSteps `json:"steps"`
}

// AssignmentGate is the new-problem gate: the size of the grey thumbnail it
// compares, StillDelta the mean absolute grey difference (0 to 255) at or
// below which the thumbnail counts as unchanged, GoneFraction the share of
// the earlier view's words that, once missing, makes a new problem, and
// CriticalTokens how many of the earlier view's digits and operators may
// change before the view is a new problem (a clock tick changes one).
type AssignmentGate struct {
	WidthPx        int     `json:"widthPx"`
	HeightPx       int     `json:"heightPx"`
	StillDelta     float64 `json:"stillDelta"`
	GoneFraction   float64 `json:"goneFraction"`
	CriticalTokens int     `json:"criticalTokens"`
}

// AssignmentSteps bounds the checker lane's steps.
type AssignmentSteps struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

func secs(n int) time.Duration { return time.Duration(n) * time.Second }

// Correct is how long after a note "no, I meant" may still correct it.
func (m Memory) Correct() time.Duration { return secs(m.CorrectS) }

// Request is the time one recall answer gets.
func (r Recall) Request() time.Duration { return secs(r.RequestS) }

// Request is the time one rewrite request gets.
func (r Rewrite) Request() time.Duration { return secs(r.RequestS) }

// Undo is how long the original is kept after a rewrite is typed back.
func (r Rewrite) Undo() time.Duration { return secs(r.UndoS) }

// Max is the longest the focus guard may stay armed.
func (f Focus) Max() time.Duration { return time.Duration(f.MaxMinutes) * time.Minute }

// Reopen is how long after a close "let me back" may reopen it.
func (f Focus) Reopen() time.Duration { return secs(f.ReopenS) }

// Hedge is how long the fast lane waits before its one re-send.
func (a Assignment) Hedge() time.Duration { return secs(a.HedgeS) }

// Fast is the time the fast lane gets.
func (a Assignment) Fast() time.Duration { return secs(a.FastS) }

// Checker is the time the checker lane gets.
func (a Assignment) Checker() time.Duration { return secs(a.CheckerS) }

// Ceilings the loader holds the draft values under, whatever the spec
// says: a rewrite no bigger than a page or two, a guard no longer than a
// working day, a correction window no longer than a few minutes, and a
// thumbnail small enough to compare at every window event.
const (
	rewriteMaxRunesCeiling = 20000
	focusMaxMinutesCeiling = 12 * 60
	memoryCorrectCeilingS  = 600
	assignmentGateMaxPx    = 128
)

func (s *OniSpec) checkPhase16(bad func(string, ...any)) {
	for _, d := range []struct {
		path string
		v    int
	}{
		{"memory.correctS", s.Memory.CorrectS},
		{"memory.forgetMax", s.Memory.ForgetMax},
		{"recall.passages", s.Recall.Passages},
		{"recall.passageTokens", s.Recall.PassageTokens},
		{"recall.promptBytes", s.Recall.PromptBytes},
		{"recall.answerTokens", s.Recall.AnswerTokens},
		{"recall.requestS", s.Recall.RequestS},
		{"rewrite.maxRunes", s.Rewrite.MaxRunes},
		{"rewrite.requestS", s.Rewrite.RequestS},
		{"rewrite.undoS", s.Rewrite.UndoS},
		{"focus.maxMinutes", s.Focus.MaxMinutes},
		{"focus.reopenS", s.Focus.ReopenS},
		{"assignment.gate.widthPx", s.Assignment.Gate.WidthPx},
		{"assignment.gate.heightPx", s.Assignment.Gate.HeightPx},
		{"assignment.cacheViews", s.Assignment.CacheViews},
		{"assignment.attemptsPerDay", s.Assignment.AttemptsPerDay},
		{"assignment.tokensPerDay", s.Assignment.TokensPerDay},
		{"assignment.hedgeS", s.Assignment.HedgeS},
		{"assignment.fastS", s.Assignment.FastS},
		{"assignment.checkerS", s.Assignment.CheckerS},
		{"assignment.steps.min", s.Assignment.Steps.Min},
		{"assignment.steps.max", s.Assignment.Steps.Max},
	} {
		if d.v <= 0 {
			bad("%s: must be positive", d.path)
		}
	}
	if s.Memory.CorrectS > memoryCorrectCeilingS {
		bad("memory.correctS: at most %d", memoryCorrectCeilingS)
	}
	if s.Rewrite.MaxRunes > rewriteMaxRunesCeiling {
		bad("rewrite.maxRunes: at most %d", rewriteMaxRunesCeiling)
	}
	if s.Focus.MaxMinutes > focusMaxMinutesCeiling {
		bad("focus.maxMinutes: at most %d", focusMaxMinutesCeiling)
	}
	if s.Focus.ReopenS > s.Focus.MaxMinutes*60 {
		bad("focus.reopenS: no longer than the guard itself")
	}
	if g := s.Assignment.Gate; g.WidthPx > assignmentGateMaxPx || g.HeightPx > assignmentGateMaxPx {
		bad("assignment.gate: at most %d px a side", assignmentGateMaxPx)
	}
	if a := s.Assignment; a.HedgeS >= a.FastS {
		bad("assignment.hedgeS: must come before the fast lane's time is up")
	}
	if st := s.Assignment.Steps; st.Min > st.Max {
		bad("assignment.steps: min above max")
	}
	if g := s.Assignment.Gate; !(g.StillDelta > 0 && g.StillDelta <= 255) {
		bad("assignment.gate.stillDelta: want 0 < stillDelta <= 255")
	}
	if g := s.Assignment.Gate; !(g.GoneFraction > 0 && g.GoneFraction <= 1) {
		bad("assignment.gate.goneFraction: want 0 < goneFraction <= 1")
	}
	if s.Assignment.Gate.CriticalTokens < 0 {
		bad("assignment.gate.criticalTokens: must not be negative")
	}
}
