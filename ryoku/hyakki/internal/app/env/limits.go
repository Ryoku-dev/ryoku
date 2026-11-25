package env

import (
	"time"

	"ryoku-rashin/internal/protocol"
)

// a place that shows the providers' allowances asks for fresh figures while it is drawn; the daemon
// paces the account reads, this only keeps every frame from asking
type LimitsWatch struct {
	asked time.Time
}

// once a minute at most, and straight away the first time
func (w *LimitsWatch) Show(e *Env, now time.Time) {
	if !w.asked.IsZero() && now.Sub(w.asked) < time.Minute && !now.Before(w.asked) {
		return
	}
	w.asked = now
	e.Call(protocol.MProvidersLimits, protocol.ProviderParams{}, new(protocol.Empty), nil)
}

// the next time the place is shown it asks again
func (w *LimitsWatch) Forget() { w.asked = time.Time{} }
