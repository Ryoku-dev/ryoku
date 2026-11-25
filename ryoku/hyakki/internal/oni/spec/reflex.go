package spec

import "time"

// Reflex holds the quick-command numbers: the storm bound on actions, the
// rule thresholds, the launcher catalog bound, and the routine, reminder,
// decision and ledger limits. Consumers read these values; none repeats
// them as literals.
//
// One utterance plans at most MaxActions actions, and at most
// MaxActionsPerMinute run in any minute. A launch is followed by a
// LaunchCooldownS pause before the next one. A verb left dangling at the end
// of an utterance carries to the next for DanglingVerbS. Only an app or site
// opening at a confidence of at least EarlyFireMin fires before the
// utterance ends; an exact app name scores ExactAppMin. A sounds-like match
// needs FuzzyMin and must lead the runner-up by FuzzyMargin. The browser
// chosen for a site stays the choice for SiteStickyS. The launcher catalog
// holds at most CatalogMax apps.
type Reflex struct {
	MaxActions          int             `json:"maxActions"`
	MaxActionsPerMinute int             `json:"maxActionsPerMinute"`
	LaunchCooldownS     float64         `json:"launchCooldownS"`
	DanglingVerbS       float64         `json:"danglingVerbS"`
	EarlyFireMin        float64         `json:"earlyFireMin"`
	ExactAppMin         float64         `json:"exactAppMin"`
	FuzzyMin            float64         `json:"fuzzyMin"`
	FuzzyMargin         float64         `json:"fuzzyMargin"`
	SiteStickyS         float64         `json:"siteStickyS"`
	CatalogMax          int             `json:"catalogMax"`
	Routines            ReflexRoutines  `json:"routines"`
	Reminders           ReflexReminders `json:"reminders"`
	Decisions           ReflexDecisions `json:"decisions"`
	Ledger              ReflexLedger    `json:"ledger"`
}

// ReflexRoutines: a run of at least OfferMinSteps replayable actions, each
// within GapS of the last, is offered once as a routine; the offer lapses
// to Skip after ChipS. The store holds at most MaxRoutines routines of at
// most MaxSteps steps each.
type ReflexRoutines struct {
	OfferMinSteps int     `json:"offerMinSteps"`
	GapS          float64 `json:"gapS"`
	MaxRoutines   int     `json:"maxRoutines"`
	MaxSteps      int     `json:"maxSteps"`
	ChipS         float64 `json:"chipS"`
}

// ReflexReminders: a reminder lies at most MaxDays ahead and a timer at most
// MaxTimerH. A reminder that comes due while Oni is busy waits up to
// BusyTries times BusyWaitS. The wake path starts the daemon when the next
// reminder is due within WakeHorizonMin; the daemon stays resident while
// the next one is due within ResidencyHorizonMin. The store holds at most
// MaxPending reminders and timers.
type ReflexReminders struct {
	MaxDays             int     `json:"maxDays"`
	MaxTimerH           float64 `json:"maxTimerH"`
	BusyTries           int     `json:"busyTries"`
	BusyWaitS           float64 `json:"busyWaitS"`
	WakeHorizonMin      float64 `json:"wakeHorizonMin"`
	ResidencyHorizonMin float64 `json:"residencyHorizonMin"`
	MaxPending          int     `json:"maxPending"`
}

// ReflexDecisions: a typed decision is taken only at a confidence of at
// least MinConfidence with a lead of MinMargin over the runner-up. One
// decision gets BudgetMs, and at most HostedPerMin hosted decisions are
// asked in any minute.
type ReflexDecisions struct {
	MinConfidence float64 `json:"minConfidence"`
	MinMargin     float64 `json:"minMargin"`
	BudgetMs      int     `json:"budgetMs"`
	HostedPerMin  int     `json:"hostedPerMin"`
}

// ReflexLedger: the local ledger keeps at most MaxRows rows, oldest out
// first.
type ReflexLedger struct {
	MaxRows int `json:"maxRows"`
}

// Decision budget bounds, the same bounds providers.decisions.budgetMs
// accepts.
const (
	reflexBudgetMinMs = 100
	reflexBudgetMaxMs = 5000
)

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// LaunchCooldown is the pause after one launch before the next.
func (r Reflex) LaunchCooldown() time.Duration { return seconds(r.LaunchCooldownS) }

// DanglingVerb is how long a verb left at the end of an utterance carries.
func (r Reflex) DanglingVerb() time.Duration { return seconds(r.DanglingVerbS) }

// SiteSticky is how long the browser chosen for a site stays the choice.
func (r Reflex) SiteSticky() time.Duration { return seconds(r.SiteStickyS) }

// Gap is the longest pause between two steps of one routine run.
func (r ReflexRoutines) Gap() time.Duration { return seconds(r.GapS) }

// Chip is how long a routine offer waits before it lapses to Skip.
func (r ReflexRoutines) Chip() time.Duration { return seconds(r.ChipS) }

// MaxReminder is the farthest ahead a reminder may lie.
func (r ReflexReminders) MaxReminder() time.Duration {
	return time.Duration(r.MaxDays) * 24 * time.Hour
}

// MaxTimer is the longest timer.
func (r ReflexReminders) MaxTimer() time.Duration { return seconds(r.MaxTimerH * 3600) }

// BusyWait is one wait while Oni is busy; a due reminder waits at most
// BusyTries of them.
func (r ReflexReminders) BusyWait() time.Duration { return seconds(r.BusyWaitS) }

// WakeHorizon is how soon the next reminder must be due for the wake path
// to start the daemon.
func (r ReflexReminders) WakeHorizon() time.Duration { return seconds(r.WakeHorizonMin * 60) }

// ResidencyHorizon is how soon the next reminder must be due for the daemon
// to stay resident.
func (r ReflexReminders) ResidencyHorizon() time.Duration {
	return seconds(r.ResidencyHorizonMin * 60)
}

// Budget is the time one typed decision gets.
func (d ReflexDecisions) Budget() time.Duration {
	return time.Duration(d.BudgetMs) * time.Millisecond
}

func (s *OniSpec) checkReflex(bad func(string, ...any)) {
	r := s.Reflex
	for _, d := range []struct {
		path string
		v    float64
	}{
		{"reflex.launchCooldownS", r.LaunchCooldownS},
		{"reflex.danglingVerbS", r.DanglingVerbS},
		{"reflex.siteStickyS", r.SiteStickyS},
		{"reflex.routines.gapS", r.Routines.GapS},
		{"reflex.routines.chipS", r.Routines.ChipS},
		{"reflex.reminders.maxTimerH", r.Reminders.MaxTimerH},
		{"reflex.reminders.busyWaitS", r.Reminders.BusyWaitS},
		{"reflex.reminders.wakeHorizonMin", r.Reminders.WakeHorizonMin},
	} {
		if !(d.v > 0) {
			bad("%s: must be positive", d.path)
		}
	}
	for _, d := range []struct {
		path string
		v    int
	}{
		{"reflex.maxActions", r.MaxActions},
		{"reflex.catalogMax", r.CatalogMax},
		{"reflex.routines.maxRoutines", r.Routines.MaxRoutines},
		{"reflex.routines.maxSteps", r.Routines.MaxSteps},
		{"reflex.reminders.maxDays", r.Reminders.MaxDays},
		{"reflex.reminders.busyTries", r.Reminders.BusyTries},
		{"reflex.reminders.maxPending", r.Reminders.MaxPending},
		{"reflex.decisions.hostedPerMin", r.Decisions.HostedPerMin},
		{"reflex.ledger.maxRows", r.Ledger.MaxRows},
	} {
		if d.v < 1 {
			bad("%s: must be at least 1", d.path)
		}
	}

	if r.MaxActions >= 1 && r.MaxActionsPerMinute < r.MaxActions {
		bad("reflex.maxActionsPerMinute: must admit one full utterance (at least maxActions)")
	}
	if !(r.EarlyFireMin > 0 && r.EarlyFireMin <= 1) {
		bad("reflex.earlyFireMin: must lie in (0, 1]")
	}
	if !(r.ExactAppMin >= r.EarlyFireMin && r.ExactAppMin <= 1) {
		bad("reflex.exactAppMin: must lie in [earlyFireMin, 1]")
	}
	if !(r.FuzzyMin > 0 && r.FuzzyMin < r.ExactAppMin) {
		bad("reflex.fuzzyMin: must lie in (0, exactAppMin)")
	}
	if !(r.FuzzyMargin > 0 && r.FuzzyMin+r.FuzzyMargin <= 1) {
		bad("reflex.fuzzyMargin: must be positive with fuzzyMin + fuzzyMargin <= 1")
	}

	rt := r.Routines
	if !(rt.OfferMinSteps >= 2 && rt.OfferMinSteps <= rt.MaxSteps) {
		bad("reflex.routines.offerMinSteps: must lie in [2, maxSteps]")
	}

	rm := r.Reminders
	if rm.MaxDays >= 1 && !(rm.MaxTimerH < float64(rm.MaxDays)*24) {
		bad("reflex.reminders.maxTimerH: a timer must be shorter than the longest reminder (maxDays·24)")
	}
	if !(rm.ResidencyHorizonMin > rm.WakeHorizonMin) {
		bad("reflex.reminders.residencyHorizonMin: the daemon must stay resident past the wake horizon")
	}
	if rm.BusyTries >= 1 && rm.BusyWaitS > 0 && !(float64(rm.BusyTries)*rm.BusyWaitS < rm.WakeHorizonMin*60) {
		bad("reflex.reminders.busyWaitS: the busy wait (busyTries·busyWaitS) must end within the wake horizon")
	}

	dc := r.Decisions
	if !(dc.MinConfidence > 0 && dc.MinConfidence <= 1) {
		bad("reflex.decisions.minConfidence: must lie in (0, 1]")
	}
	if !(dc.MinMargin > 0 && dc.MinMargin < 1) {
		bad("reflex.decisions.minMargin: must lie in (0, 1)")
	}
	if !(dc.BudgetMs >= reflexBudgetMinMs && dc.BudgetMs <= reflexBudgetMaxMs) {
		bad("reflex.decisions.budgetMs: must lie in [%d, %d]", reflexBudgetMinMs, reflexBudgetMaxMs)
	}
	if dc.HostedPerMin >= 1 && dc.HostedPerMin > r.MaxActionsPerMinute {
		bad("reflex.decisions.hostedPerMin: must not exceed maxActionsPerMinute")
	}
}
