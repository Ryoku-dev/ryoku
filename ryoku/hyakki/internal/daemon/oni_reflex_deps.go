package daemon

import (
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/routine"
	"ryoku-rashin/internal/reflex/act"
)

// OniReflexDeps are what quick commands act through; each nil field is the
// real one, built on first use and never while reflex is off.
type OniReflexDeps struct {
	// Exec carries out the executor intents.
	Exec act.Executor
	// Catalog resolves spoken app names; it is the executors' catalog.
	Catalog reflex.Catalog
	// Reminders and Routines are the reflex stores.
	Reminders *remind.Store
	Routines  *routine.Store
	// Notify shows a desktop notification, used when Oni is hidden.
	Notify func(summary, body string) error
	// Wake holds or releases the reminders' share of the wake unit; nil
	// is the real unit. Tests always inject it.
	Wake func(on bool) error
	// Clock times the ledger and the caps.
	Clock presence.Clock
}
