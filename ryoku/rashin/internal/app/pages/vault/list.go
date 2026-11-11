package vault

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	th := e.Theme
	vault := e.Snapshot().Vault
	rows := make([]look.SideRow, 0, len(vault.Credentials))
	for _, credential := range sortedCredentials(vault) {
		row := look.SideRow{ID: credential.ID, Title: credential.Name, Facts: whereItGoes(credential)}
		row.Trail = kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, teamCount(len(credential.Teams)))
		rows = append(rows, row)
	}
	p.list.Empty = i18n.T("Credentials you add are listed here.")
	if vault.State != protocol.VaultOpen {
		p.list.Empty = teamsui.VaultState(vault)
	}
	selected := ""
	if _, ok := credentialByID(vault, p.credential); ok {
		selected = p.credential
	}
	dimensions := p.list.Layout(gtx, th, e.Overlays, []look.SideGroup{{Rows: rows}}, selected)
	for _, event := range p.list.Events() {
		if event.Kind == look.SideOpen {
			p.openCredential(e, event.ID)
		}
	}
	return dimensions
}

// the hosts a header credential may go to, or what a mail password is for
func whereItGoes(credential protocol.Credential) string {
	if credential.Use == protocol.CredentialMail {
		return i18n.T("Mail account password")
	}
	return strings.Join(credential.Hosts, " ")
}

func teamCount(count int) string {
	switch count {
	case 0:
		return i18n.T("no team")
	case 1:
		return i18n.T("1 team")
	}
	return i18n.Tf("%d teams", count)
}
