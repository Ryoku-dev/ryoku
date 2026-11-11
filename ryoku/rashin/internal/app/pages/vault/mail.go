package vault

import (
	"strconv"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the account teams send mail from, and whether its server takes the login
type mailSlab struct {
	setup, change, test kit.Plaque
	testing             bool
	result              *protocol.MailTestResult
}

func (m *mailSlab) layout(gtx layout.Context, e *env.Env, p *Page, vault protocol.VaultInfo) layout.Dimensions {
	th := e.Theme
	account := vault.Mail
	m.setup.Kind, m.setup.Small, m.setup.Label = kit.PlaqueLine, true, i18n.T("Set up the mail account")
	m.change.Kind, m.change.Small, m.change.Label = kit.PlaqueQuiet, true, i18n.T("Change")
	m.test.Kind, m.test.Small, m.test.Label, m.test.Busy = kit.PlaqueLine, true, i18n.T("Test the login"), m.testing
	if m.setup.Clicked(gtx) || m.change.Clicked(gtx) {
		p.account.open(vault)
	}
	if m.test.Clicked(gtx) && !m.testing && account != nil {
		m.check(e)
	}
	body := func(gtx layout.Context) layout.Dimensions {
		if account == nil {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("Teams with the mail tool send from an account you set here. Its password stays in the vault and every mail asks you first."))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.setup.Layout(gtx, th.Theme) }),
			)
		}
		password := i18n.T("None chosen")
		if credential, ok := mailCredential(vault, account.Credential); ok {
			password = credential.Name
		}
		sender := account.Address
		if account.Name != "" {
			sender = i18n.Tf("%s <%s>", account.Name, account.Address)
		}
		rows := []look.FactRow{
			{Label: i18n.T("Address"), Value: sender},
			{Label: i18n.T("Server"), Value: account.Host + ":" + strconv.Itoa(account.Port), Mono: true},
			{Label: i18n.T("Security"), Value: securityWord(account.Security)},
			{Label: i18n.T("Password"), Value: password, Mono: true},
		}
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Facts(gtx, th, rows) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.test.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return m.change.Layout(gtx, th.Theme) }),
				)
			}),
		}
		if result := m.result; result != nil {
			word, tone := i18n.T("Login works"), palette.Alive
			if !result.OK {
				word, tone = i18n.T("Login failed"), palette.Failed
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(kit.Claim(th.Theme, tone, word)),
						layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
						layout.Flexed(1, look.Paragraph(th, th.T.Caption, th.R.Fg3, result.Text)),
					)
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(look.SectionTitle(th, i18n.T("Mail account"), 0)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.SlabContent(gtx, th.Theme, body) }),
	)
}

// connects and logs in without sending anything
func (m *mailSlab) check(e *env.Env) {
	m.testing, m.result = true, nil
	var result protocol.MailTestResult
	e.Call(protocol.MVaultMailTest, nil, &result, func(err error) {
		m.testing = false
		if err != nil {
			result = protocol.MailTestResult{Text: err.Error()}
		}
		m.result = &result
	})
}

func securityWord(security string) string {
	switch security {
	case protocol.MailTLS:
		return i18n.T("TLS")
	case protocol.MailPlain:
		return i18n.T("None, a server on this computer")
	}
	return i18n.T("STARTTLS")
}
