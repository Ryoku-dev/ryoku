package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"ryoku-rashin/internal/protocol"
)

type fakeAccount struct{ calls []string }

func (f *fakeAccount) Status() protocol.AccountState {
	f.calls = append(f.calls, "status")
	return protocol.AccountState{}
}
func (f *fakeAccount) SignIn(context.Context) (protocol.AccountState, error) {
	f.calls = append(f.calls, "signin")
	return protocol.AccountState{}, nil
}
func (f *fakeAccount) Cancel() protocol.AccountState {
	f.calls = append(f.calls, "cancel")
	return protocol.AccountState{}
}
func (f *fakeAccount) Refresh(context.Context) (protocol.AccountState, error) {
	f.calls = append(f.calls, "refresh")
	return protocol.AccountState{}, nil
}
func (f *fakeAccount) SignOut(context.Context) protocol.AccountState {
	f.calls = append(f.calls, "signout")
	return protocol.AccountState{}
}
func (f *fakeAccount) SettingsChanged() {}
func (f *fakeAccount) OnChange(func())  {}

// The account is the user's: no agent, CLI or other client may read it, sign
// in or sign out, however it names the method.
func TestAccountMethodsAreTheAppsAlone(t *testing.T) {
	for _, method := range []string{protocol.MAccountStatus, protocol.MAccountSignIn, protocol.MAccountCancel, protocol.MAccountRefresh, protocol.MAccountSignOut} {
		acct := &fakeAccount{}
		d := &Daemon{deps: Deps{Account: acct}}
		_, err := d.dispatch(context.Background(), &connection{}, method, json.RawMessage(`{"plan":"plus"}`))
		var refused *protocol.Error
		if !errors.As(err, &refused) || refused.Code != protocol.CodeForbidden || len(acct.calls) != 0 {
			t.Fatalf("another client calling %s: err %v, calls %v", method, err, acct.calls)
		}
		if _, err := d.dispatch(context.Background(), &connection{app: true}, method, json.RawMessage(`{"plan":"plus"}`)); err != nil || len(acct.calls) != 1 {
			t.Fatalf("the app calling %s: err %v, calls %v", method, err, acct.calls)
		}
	}
}

// A settings patch that points the account at another server is the app's
// alone too, since tokens follow that address.
func TestAccountServerSettingIsTheAppsAlone(t *testing.T) {
	if !settingsForApp(json.RawMessage(`{"providers":{"account":"https://evil.example.com"}}`)) {
		t.Fatal("an agent could point the Ryoku Account at another server")
	}
}

// The account's credits are the user's to spend: a model of it named on the
// socket by anything but the app is no pick, however the call names it.
func TestHostedModelsNeedTheApp(t *testing.T) {
	d := &Daemon{}
	calls := map[string]string{
		protocol.MSessionNew:      `{"kind":"ask","model":"ember/rashin-fast"}`,
		protocol.MSessionSetModel: `{"id":"ask-1","model":"EMBER/rashin-fast"}`,
		protocol.MAsk:             `{"question":"hi","model":" ember/rashin-fast"}`,
		protocol.MGlance:          `{"mode":"ask","model":"ember/rashin-fast"}`,
		protocol.MCompareStart:    `{"id":"ask-1","models":["openrouter/x","ember/rashin-fast"]}`,
	}
	for method, params := range calls {
		err := d.mainGate(&connection{}, method, json.RawMessage(params))
		var refused *protocol.Error
		if !errors.As(err, &refused) || refused.Code != protocol.CodeForbidden {
			t.Errorf("another client calling %s with a hosted model: %v", method, err)
		}
		if err := d.mainGate(&connection{daemon: d}, method, json.RawMessage(params)); err == nil {
			t.Errorf("an agent's tool host calling %s with a hosted model was allowed", method)
		}
		if err := d.mainGate(&connection{app: true}, method, json.RawMessage(params)); err != nil {
			t.Errorf("the app calling %s with a hosted model: %v", method, err)
		}
	}
	for _, method := range []string{protocol.MSessionNew, protocol.MAsk} {
		params := map[string]string{protocol.MSessionNew: `{"kind":"ask","model":"openrouter/x"}`, protocol.MAsk: `{"question":"hi","model":"auto"}`}[method]
		if err := d.mainGate(&connection{}, method, json.RawMessage(params)); err != nil {
			t.Errorf("another client calling %s with an ordinary model: %v", method, err)
		}
	}
}

// A default Ask model that is hosted would spend the account on every
// question any client asks.
func TestHostedDefaultModelSettingIsTheAppsAlone(t *testing.T) {
	if !settingsForApp(json.RawMessage(`{"ask":{"model":"ember/rashin-fast"}}`)) {
		t.Fatal("a client could make a hosted model the default of every question")
	}
	for _, patch := range []string{`{"ask":{"model":"auto"}}`, `{"ask":{"habits":true}}`, `{"agent":{"model":"x"}}`} {
		if settingsForApp(json.RawMessage(patch)) {
			t.Fatalf("%s needs the app", patch)
		}
	}
}
