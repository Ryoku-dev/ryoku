package vault

import (
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// how many uses the board and a credential's page show
const recentLimit = 20

// the vault's record of uses, for the board or for one credential
type recent struct {
	events  []protocol.VaultEvent
	key     string
	loading bool
}

func (r *recent) stale() { r.key = "" }

// reads the record again when the vault changed, never on a clock
func (r *recent) ensure(e *env.Env, credential string, vault protocol.VaultInfo) {
	if vault.State != protocol.VaultOpen {
		r.events, r.key = nil, ""
		return
	}
	key := credential + "\x00" + pulse(vault)
	if key == r.key || r.loading {
		return
	}
	r.loading = true
	var result protocol.VaultEventsResult
	e.Call(protocol.MVaultEvents, protocol.VaultEventsParams{Credential: credential, Limit: recentLimit}, &result, func(err error) {
		r.loading, r.key = false, key
		if err != nil {
			e.Notify(err.Error())
			return
		}
		r.events = result.Events
	})
}

// changes whenever a credential is added, replaced, removed or used
func pulse(vault protocol.VaultInfo) string {
	var b strings.Builder
	for _, credential := range vault.Credentials {
		b.WriteString(credential.ID)
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(credential.Version))
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(credential.Uses))
		b.WriteByte(' ')
	}
	return b.String()
}

// one use: when, which credential, for which team and where, and what came of it
func layoutEvent(gtx layout.Context, th *look.Theme, teams []protocol.Team, event protocol.VaultEvent, named bool) layout.Dimensions {
	word, tone := outcomeWord(event.Outcome)
	where := make([]string, 0, 2)
	if event.Team != "" {
		where = append(where, teamsui.TeamName(teams, event.Team))
	}
	if event.Host != "" {
		where = append(where, event.Host)
	}
	children := []layout.FlexChild{look.Cell(gtx.Dp(96), kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, look.Ago(gtx.Now, event.Time)))}
	if named {
		children = append(children, look.Cell(gtx.Dp(168), kit.Text(th.Theme, th.T.Mono, th.R.Fg, event.Name)))
	}
	children = append(children,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(th.Theme, th.T.Body, th.R.Fg2, strings.Join(where, " · "))),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if event.Detail == "" {
						return layout.Dimensions{}
					}
					return kit.Text(th.Theme, th.T.Caption, th.R.Fg3, event.Detail)(gtx)
				}),
			)
		}),
	)
	if event.Unwatched {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, i18n.T("Nobody watching"))),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Rigid(kit.Claim(th.Theme, tone, word)),
	)
	return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}
