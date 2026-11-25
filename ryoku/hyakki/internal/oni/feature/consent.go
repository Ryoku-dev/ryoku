package feature

import (
	"time"

	"ryoku-rashin/internal/protocol"
)

// Consent classes say what the user must do before a feature acts.
const (
	// ConsentUserInitiated starts every use from the user's own act, such as
	// holding the talk chord; nothing happens without it.
	ConsentUserInitiated = "user-initiated"

	// ConsentDisclosure shows exactly what will leave and where before it is
	// sent, once a session or every time as Settings.oni.disclosure says.
	ConsentDisclosure = "disclosure"

	// ConsentAnnounced acts without asking but announces every action, and
	// one switch stops it.
	ConsentAnnounced = "announced"

	// ConsentPerAction asks before each action.
	ConsentPerAction = "per-action"

	// ConsentFirm sorts every action on its own. A safe step, such as
	// filling a field, an ordinary click or a scroll, is announced and run.
	// Anything risky, or anything the rules cannot place, is handed to the
	// user: Hands points at the control and waits, and the user presses it
	// themselves. Hands never presses it, no answer makes it, and a spoken
	// yes approves nothing.
	ConsentFirm = "firm"
)

// ConsentTTL bounds how long a consent question stays answerable.
const ConsentTTL = 120 * time.Second

// MustAsk reports whether feature id must ask before this use. asked says
// whether the user already answered yes in this session. Anything the
// contract does not know asks.
func MustAsk(id string, s protocol.OniSettings, asked bool) bool {
	f, ok := Lookup(id)
	if !ok {
		return true
	}
	switch f.Consent {
	case ConsentUserInitiated, ConsentAnnounced:
		return false
	case ConsentDisclosure:
		return s.Disclosure != protocol.OniDisclosureSession || !asked
	}
	return true
}
