package vault

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// one credential on the board: its name, what it is for, where it may go,
// who may use it and how it was used, never its value
type tablet struct {
	name                  kit.Click
	replace, edit, remove kit.Plaque
}

func (t *tablet) layout(gtx layout.Context, e *env.Env, p *Page, teams []protocol.Team, credential protocol.Credential) layout.Dimensions {
	th := e.Theme
	t.replace.Kind, t.replace.Small, t.replace.Label = kit.PlaqueLine, true, i18n.T("Replace value")
	t.edit.Kind, t.edit.Small, t.edit.Label = kit.PlaqueQuiet, true, i18n.T("Edit")
	t.remove.Kind, t.remove.Small, t.remove.Label = kit.PlaqueQuiet, true, i18n.T("Remove")
	if t.name.Clicked(gtx) || t.edit.Clicked(gtx) {
		p.openCredential(e, credential.ID)
	}
	if t.replace.Clicked(gtx) {
		p.replace.open(credential)
	}
	if t.remove.Clicked(gtx) {
		p.remove.open(credential)
	}
	return plane(gtx, th, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return t.name.Layout(gtx, kit.Text(th.Theme, th.T.Mono, th.R.Fg, credential.Name))
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, i18n.Tf("fp %s", credential.Fingerprint))),
				)
			}),
		}
		if credential.Description != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, credential.Description)),
			)
		}
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
		if credential.Use == protocol.CredentialMail {
			children = append(children, layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Mail account password"))))
		} else {
			children = append(children, layout.Rigid(chips(th, credential.Hosts)))
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, teamNames(teams, credential.Teams)+" · "+usedWords(gtx, credential))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.replace.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.edit.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.remove.Layout(gtx, th.Theme) }),
				)
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// when it was last used and how often, or that it never was
func usedWords(gtx layout.Context, credential protocol.Credential) string {
	if credential.LastUsed == nil || credential.Uses == 0 {
		return i18n.T("never used")
	}
	if credential.Uses == 1 {
		return i18n.Tf("used %s, once", look.Ago(gtx.Now, *credential.LastUsed))
	}
	return i18n.Tf("used %s, %d times", look.Ago(gtx.Now, *credential.LastUsed), credential.Uses)
}
