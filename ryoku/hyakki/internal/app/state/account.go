package state

import (
	"encoding/json"

	"ryoku-rashin/internal/protocol"
)

// accountState is the Ryoku Account as the daemon last told it. Each value
// replaces the last whole, so a snapshot can share it: nothing writes into it.
type accountState struct {
	account protocol.AccountState
	// known is false until the daemon said anything about the account
	known bool
}

// applyLocked applies account.changed; handled is false for any other event.
func (a *accountState) applyLocked(event protocol.Event) (handled bool, err error) {
	if event.Event != protocol.EvtAccountChanged {
		return false, nil
	}
	var next protocol.AccountState
	if err := json.Unmarshal(event.Data, &next); err != nil {
		return true, err
	}
	a.account, a.known = next, true
	return true, nil
}

func (a *accountState) snapshotLocked(into *Snapshot) {
	into.Account, into.AccountKnown = a.account, a.known
}

// SetAccount stores the account as an account.* call answered it, ahead of
// the account.changed that follows.
func (s *Store) SetAccount(account protocol.AccountState) {
	s.mu.Lock()
	s.accountState.account, s.accountState.known = account, true
	invalidate := s.changedLocked()
	s.mu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}
