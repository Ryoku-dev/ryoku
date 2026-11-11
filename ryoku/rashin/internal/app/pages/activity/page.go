// Package activity is the Activity destination: the teams' ledger of what was
// done, by whom and whether anyone was watching.
package activity

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the filters in the order their chips stand
var filters = [...]string{
	protocol.ActivityAll, protocol.ActivityUnwatched, protocol.ActivityApprovals,
	protocol.ActivityCredentials, protocol.ActivityFailures,
}

// the widest a full page's column grows
const columnWidth = 994

type Page struct {
	list    look.SideList
	header  look.PaneHeader
	scroll  look.Scroller
	filters kit.Filters
	teams   look.Select
	team    string
	ledger  ledger
	more    kit.Plaque
}

func New() *Page { return &Page{} }

func (p *Page) Select(e *env.Env, place env.TeamsPlace) {
	p.team = place.Team
	// arriving reads the newest entries again
	p.ledger.pulse = ""
}

// Activity is a full page, so its list only says what stands beside it
func (p *Page) List(gtx layout.Context, e *env.Env) look.ListDimensions {
	p.list.Empty = i18n.T("What every team did, newest first.")
	return p.list.Layout(gtx, e.Theme, e.Overlays, nil, "")
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	snapshot := e.Snapshot()
	if _, ok := teamIndex(snapshot.Teams, p.team); !ok {
		p.team = ""
	}
	filter := filters[min(max(p.filters.Selected, 0), len(filters)-1)]
	p.ledger.ensure(e, filter, p.team, pulse(snapshot))
	items := p.items(gtx)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			content := look.PaneHeaderContent{Title: i18n.T("Activity"), Scrolled: p.scroll.Position.First > 0 || p.scroll.Position.Offset > 0}
			return p.header.Layout(gtx, e.Theme, e.Overlays, content)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Centred(gtx, e.Class, columnWidth, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: e.Theme.Sp.S5, Bottom: e.Theme.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					claim, proof := ledgerLead(gtx.Now, p.ledger.entries)
					return look.Monument(gtx, e.Theme, claim, proof)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Centred(gtx, e.Class, columnWidth, func(gtx layout.Context) layout.Dimensions { return p.layoutFilters(gtx, e, snapshot.Teams) })
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(items) == 0 {
				return p.layoutEmpty(gtx, e)
			}
			return p.scroll.Layout(gtx, e.Theme, len(items), func(gtx layout.Context, index int) layout.Dimensions {
				return look.Centred(gtx, e.Class, columnWidth, func(gtx layout.Context) layout.Dimensions {
					return p.layoutItem(gtx, e, filter, items[index])
				})
			})
		}),
	)
}

func (p *Page) layoutFilters(gtx layout.Context, e *env.Env, teams []protocol.Team) layout.Dimensions {
	th := e.Theme
	p.filters.Labels = []string{i18n.T("All"), i18n.T("Nobody watching"), i18n.T("Approvals"), i18n.T("Credentials"), i18n.T("Failures")}
	options := make([]string, 0, len(teams)+1)
	options = append(options, i18n.T("Every team"))
	for _, team := range teams {
		options = append(options, team.Name)
	}
	selected, _ := teamIndex(teams, p.team)
	p.teams.Form, p.teams.Label = look.SelectQuiet, i18n.T("Team")
	return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.filters.Layout(gtx, th.Theme)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dimensions := p.teams.Layout(gtx, th, e.Overlays, options, selected)
				if index, changed := p.teams.Changed(); changed {
					p.team = ""
					if index > 0 && index <= len(teams) {
						p.team = teams[index-1].ID
					}
				}
				return dimensions
			}),
		)
	})
}

func (p *Page) New(e *env.Env) { e.NewTeamTask(p.team) }

func (p *Page) NewLabel() string { return i18n.T("New task") }

func (p *Page) Overlay(gtx layout.Context, e *env.Env) bool { return false }

// the select's index for a team: 0 is every team
func teamIndex(teams []protocol.Team, id string) (int, bool) {
	if id == "" {
		return 0, true
	}
	for index, team := range teams {
		if team.ID == id {
			return index + 1, true
		}
	}
	return 0, false
}
