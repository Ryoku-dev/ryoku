package vault

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

type removeDialog struct {
	credential protocol.Credential
	confirm    kit.Confirm
}

func (d *removeDialog) open(credential protocol.Credential) {
	d.credential, d.confirm.Dialog.Open, d.confirm.Danger = credential, true, true
}

func (d *removeDialog) layout(gtx layout.Context, e *env.Env, p *Page) {
	if d.credential.ID == "" {
		return
	}
	name := d.credential.Name
	body := i18n.Tf("Teams can no longer send %s, and every version of its value is deleted for good.", name)
	if d.credential.Use == protocol.CredentialMail {
		body = i18n.Tf("Teams can no longer send mail with %s as its password, and every version of its value is deleted for good.", name)
	}
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.Tf("Remove %s?", name), body, i18n.T("Remove"), "") {
		id := d.credential.ID
		e.Call(protocol.MVaultRemove, protocol.VaultRef{ID: id}, nil, func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			if p.credential == id {
				e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabVault})
			}
		})
	}
	if d.confirm.Closed() {
		d.credential = protocol.Credential{}
	}
}
