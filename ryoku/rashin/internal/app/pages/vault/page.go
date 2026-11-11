// Package vault is the Vault destination: the credentials teams may use
// without ever seeing them, where each may go, what asked for one, and the
// mail account.
package vault

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

type Page struct {
	list       look.SideList
	credential string

	board  board
	detail detail

	add     addDialog
	replace replaceDialog
	remove  removeDialog
	account mailDialog
}

func New() *Page { return &Page{} }

func (p *Page) Select(e *env.Env, place env.TeamsPlace) {
	p.credential = place.Credential
	// opening the page again reads the record again, the record has no event of its own
	p.board.recent.stale()
	p.detail.recent.stale()
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	vault := e.Snapshot().Vault
	if credential, ok := credentialByID(vault, p.credential); ok {
		return p.detail.layout(gtx, e, p, credential)
	}
	return p.board.layout(gtx, e, p)
}

func (p *Page) New(e *env.Env) { p.openAdd(e, protocol.CredentialHeader, nil) }

func (p *Page) NewLabel() string { return i18n.T("Add credential") }

// the mail dialog goes first so the add dialog it opens stands above it
func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool {
	p.account.layout(gtx, e, p)
	p.add.layout(gtx, e)
	p.replace.layout(gtx, e)
	p.remove.layout(gtx, e, p)
	return look.AnyOpen(&p.account.dialog, &p.add.dialog, &p.replace.dialog, &p.remove.confirm.Dialog)
}

// a closed vault takes nothing, so the dialog would only end in an error
func (p *Page) openAdd(e *env.Env, use string, added func(protocol.Credential)) {
	vault := e.Snapshot().Vault
	if vault.State != protocol.VaultOpen {
		e.Notify(teamsui.VaultState(vault))
		return
	}
	p.add.open(use, added)
}

func (p *Page) openCredential(e *env.Env, id string) {
	e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabVault, Credential: id})
}
