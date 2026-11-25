package demo

import (
	"encoding/json"
	"os"
	"time"

	"ryoku-rashin/internal/protocol"
)

// HYAKKI_DEMO_ACCOUNT starts the demo's Ryoku Account in a state: unset is
// signed out, or unconfigured, signing_in, signed_in, signed_in_empty (signed
// in with no Ember credits), error
const demoAccountEnv = "HYAKKI_DEMO_ACCOUNT"

const demoUnits = 1_000_000_000

func demoAccountBase(state string) protocol.AccountState {
	return protocol.AccountState{
		State: state, Server: "https://accounts.example.com", UnitsPerCredit: demoUnits,
		Models: []protocol.AccountModel{{ID: "rashin-fast", Name: "Qwen3.8 Flash", Logo: "qwen"}, {ID: "rashin-balanced", Name: "MiniMax M3.1 Flash", Logo: "minimax"}},
		Notice: "Hosted models send what you write in that conversation to Ryoku's service and an AI provider to answer it.",
		Manage: "https://accounts.example.com/account",
	}
}

func demoSigningIn() protocol.AccountState {
	st := demoAccountBase(protocol.AccountSigningIn)
	st.Code = &protocol.AccountCode{UserCode: "BCDF-GHJK", VerificationURI: "https://accounts.example.com/device", VerificationURIComplete: "https://accounts.example.com/device?user_code=BCDF-GHJK", ExpiresAt: time.Now().Add(9*time.Minute + 41*time.Second)}
	return st
}

func demoSignedIn() protocol.AccountState {
	st := demoAccountBase(protocol.AccountSignedIn)
	st.Account = &protocol.AccountIdentity{ID: "ra_demo2b8kq4n7x3h5m9", Display: "ada@example.com"}
	st.Balance = &protocol.AccountBalance{Available: 187_423_916_002, Held: 52_374_000, Read: time.Now().Add(-40 * time.Second)}
	return st
}

func initialAccount() protocol.AccountState {
	switch os.Getenv(demoAccountEnv) {
	case "unconfigured":
		return protocol.AccountState{State: protocol.AccountUnconfigured}
	case "signing_in":
		return demoSigningIn()
	case "signed_in":
		return demoSignedIn()
	case "signed_in_empty":
		st := demoSignedIn()
		st.Balance.Available, st.Balance.Held = 0, 0
		return st
	case "error":
		st := demoAccountBase(protocol.AccountSignedOut)
		st.Error = "Sign-in is blocked: Ryoku Account refused this app (unauthorized_client)."
		return st
	}
	return demoAccountBase(protocol.AccountSignedOut)
}

// the demo account: signing in waits until the next refresh approves it,
// and a refresh reads the balance again, after the request that held some
// of it has ended
func (s *server) callAccount(method string, _ json.RawMessage) (any, error) {
	s.accountState()
	switch method {
	case protocol.MAccountStatus:
	case protocol.MAccountSignIn:
		*s.account = demoSigningIn()
	case protocol.MAccountCancel:
		*s.account = demoAccountBase(protocol.AccountSignedOut)
	case protocol.MAccountRefresh:
		switch s.account.State {
		case protocol.AccountSigningIn:
			*s.account = demoSignedIn()
		case protocol.AccountSignedIn:
			s.account.Balance.Held = 0
			s.account.Balance.Read = time.Now()
		}
	case protocol.MAccountSignOut:
		*s.account = demoAccountBase(protocol.AccountSignedOut)
	default:
		return nil, notFound(method)
	}
	s.queueEvent(protocol.EvtAccountChanged, *s.account)
	return *s.account, nil
}

// the demo account, made on first use so a snapshot or a model list sees the
// same state the account methods do
func (s *server) accountState() *protocol.AccountState {
	if s.account == nil {
		account := initialAccount()
		s.account = &account
	}
	return s.account
}
