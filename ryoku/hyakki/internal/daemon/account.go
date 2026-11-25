package daemon

import (
	"context"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// Account is the Ryoku Account service (internal/account), as the daemon
// drives it. Every method is the Rashin app's alone: the account's balance
// and identity are the user's, and only the app may sign in or out.
type Account interface {
	Status() protocol.AccountState
	SignIn(ctx context.Context) (protocol.AccountState, error)
	Cancel() protocol.AccountState
	Refresh(ctx context.Context) (protocol.AccountState, error)
	SignOut(ctx context.Context) protocol.AccountState
	SettingsChanged()
	OnChange(func())
}

func (d *Daemon) accountCall(ctx context.Context, c *connection, method string) (any, error) {
	if err := requireApp(c); err != nil {
		return nil, err
	}
	a := d.deps.Account
	if a == nil {
		return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Ryoku Accounts is not part of this build."))
	}
	switch method {
	case protocol.MAccountStatus:
		return a.Status(), nil
	case protocol.MAccountSignIn:
		return a.SignIn(ctx)
	case protocol.MAccountCancel:
		return a.Cancel(), nil
	case protocol.MAccountRefresh:
		return a.Refresh(ctx)
	case protocol.MAccountSignOut:
		return a.SignOut(ctx), nil
	}
	return nil, protocol.Errorf(protocol.CodeUnsupported, "%s", method)
}

// accountChanged sends the account's state to the app windows, and drops a
// reflex decision card's binding to an account no longer signed in.
func (d *Daemon) accountChanged() {
	if d.deps.Account != nil {
		d.Broadcast(protocol.EvtAccountChanged, d.deps.Account.Status())
		d.reflexAccountChanged()
	}
}
