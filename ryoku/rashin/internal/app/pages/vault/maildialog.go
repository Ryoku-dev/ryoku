package vault

import (
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
)

// the security modes in the order the select offers them; none only for a server on this computer
var securityModes = [...]string{protocol.MailStartTLS, protocol.MailTLS, protocol.MailPlain}

// sets where teams send mail from; the password is a mail credential, never typed here
type mailDialog struct {
	shown    bool
	dialog   kit.Dialog
	scroll   look.Scroller
	security look.Select
	secure   int
	password look.Select
	chosen   string
	trusted  []protocol.MailTrust
	busy     bool
	problem  string

	address, name, host, port, username kit.TextField
	save, cancel                        kit.Plaque
}

func (d *mailDialog) open(vault protocol.VaultInfo) {
	d.shown, d.dialog.Open, d.dialog.Width = true, true, 600
	d.busy, d.problem = false, ""
	account := protocol.MailAccount{Port: 587, Security: protocol.MailStartTLS}
	if vault.Mail != nil {
		account = *vault.Mail
	}
	d.address.SetText(account.Address)
	d.name.SetText(account.Name)
	d.host.SetText(account.Host)
	d.port.SetText(strconv.Itoa(account.Port))
	d.username.SetText(account.Username)
	d.secure = 0
	for index, mode := range securityModes {
		if mode == account.Security {
			d.secure = index
		}
	}
	d.chosen = ""
	if credential, ok := mailCredential(vault, account.Credential); ok {
		d.chosen = credential.ID
	}
	d.trusted = append([]protocol.MailTrust(nil), account.Trusted...)
}

func (d *mailDialog) layout(gtx layout.Context, e *env.Env, p *Page) {
	if !d.shown {
		return
	}
	th := e.Theme
	vault := e.Snapshot().Vault
	local := hostrule.IsLocal(d.host.Text())
	if !local && securityModes[d.secure] == protocol.MailPlain {
		d.secure = 0
	}
	account, ready := d.account()
	ready = ready && !d.busy
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.save.Kind, d.save.Label, d.save.Busy = kit.PlaqueGold, i18n.T("Save the account"), d.busy
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.save.Clicked(gtx) && ready {
		d.submit(e, account)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Mail account"),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(520))
			gtx.Constraints.Min.Y = 0
			rows := d.rows(e, p, vault, local)
			return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, rows[index])
			})
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return d.save.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.shown = false
	}
}

func (d *mailDialog) rows(e *env.Env, p *Page, vault protocol.VaultInfo, local bool) []layout.Widget {
	th := e.Theme
	field := func(input *kit.TextField, label, placeholder string, mono bool) layout.Widget {
		input.Label, input.Placeholder, input.Mono = label, placeholder, mono
		return func(gtx layout.Context) layout.Dimensions { return input.Layout(gtx, th.Theme) }
	}
	security := []look.SelectOption{
		{Label: i18n.T("STARTTLS"), Detail: i18n.T("Starts plain and switches to encryption, usually on port 587.")},
		{Label: i18n.T("TLS"), Detail: i18n.T("Encrypted from the start, usually on port 465.")},
	}
	if local {
		security = append(security, look.SelectOption{Label: i18n.T("None"), Detail: i18n.T("Only for a server on this computer.")})
	}
	d.security.Label = i18n.T("Security")
	passwords, ids := d.passwordOptions(vault)
	d.password.Label = i18n.T("Password")
	picked := len(passwords) - 1
	for index, id := range ids {
		if id == d.chosen {
			picked = index
		}
	}
	if d.chosen == "" && len(ids) > 0 {
		d.chosen, picked = ids[0], 0
	}
	return []layout.Widget{
		field(&d.address, i18n.T("Address"), "me@example.org", false),
		field(&d.name, i18n.T("Name"), i18n.T("The name people see"), false),
		field(&d.host, i18n.T("Server"), "smtp.example.org", true),
		field(&d.port, i18n.T("Port"), "587", true),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			dimensions := d.security.LayoutOptions(gtx, th, e.Overlays, security, d.secure)
			if index, changed := d.security.Changed(); changed {
				d.secure = index
			}
			return dimensions
		},
		field(&d.username, i18n.T("Username"), "me@example.org", false),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			dimensions := d.password.LayoutOptions(gtx, th, e.Overlays, passwords, picked)
			if index, changed := d.password.Changed(); changed {
				if index < len(ids) {
					d.chosen = ids[index]
				} else {
					p.openAdd(e, protocol.CredentialMail, func(credential protocol.Credential) { d.chosen = credential.ID })
				}
			}
			return dimensions
		},
		func(gtx layout.Context) layout.Dimensions {
			if d.problem == "" {
				return layout.Dimensions{}
			}
			return look.Paragraph(th, th.T.Caption, th.R.Amber, d.problem)(gtx)
		},
	}
}

// the mail credentials by name, then a way to add one
func (d *mailDialog) passwordOptions(vault protocol.VaultInfo) ([]look.SelectOption, []string) {
	var options []look.SelectOption
	var ids []string
	for _, credential := range sortedCredentials(vault) {
		if credential.Use == protocol.CredentialMail {
			options = append(options, look.SelectOption{Label: credential.Name, Detail: credential.Description})
			ids = append(ids, credential.ID)
		}
	}
	options = append(options, look.SelectOption{Label: i18n.T("Add one"), Detail: i18n.T("Adds the account's password to the vault.")})
	return options, ids
}

// the account as typed, and whether it is whole enough to save
func (d *mailDialog) account() (protocol.MailAccount, bool) {
	port, err := strconv.Atoi(strings.TrimSpace(d.port.Text()))
	account := protocol.MailAccount{
		Address:    strings.TrimSpace(d.address.Text()),
		Name:       strings.TrimSpace(d.name.Text()),
		Host:       strings.ToLower(strings.TrimSpace(d.host.Text())),
		Port:       port,
		Security:   securityModes[d.secure],
		Username:   strings.TrimSpace(d.username.Text()),
		Credential: d.chosen,
		Trusted:    d.trusted,
	}
	ready := err == nil && port >= 1 && port <= 65535 &&
		strings.Contains(account.Address, "@") && account.Host != "" && account.Credential != ""
	return account, ready
}

func (d *mailDialog) submit(e *env.Env, account protocol.MailAccount) {
	d.busy, d.problem = true, ""
	e.Call(protocol.MVaultMail, protocol.MailAccountParams{Account: &account}, nil, func(err error) {
		d.busy = false
		if err != nil {
			d.problem = err.Error()
			return
		}
		d.dialog.Open = false
	})
}
