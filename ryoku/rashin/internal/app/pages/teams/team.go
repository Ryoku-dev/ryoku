package teams

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func (p *Page) layoutTeam(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutTeamHeader(gtx, e, team) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutTeamTabs(gtx, e, team) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return p.rise.Layout(gtx, e.Theme, team.ID+"\x00"+p.place.View, 8, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTeamView(gtx, e, team)
			})
		}),
	)
}

func (p *Page) layoutTeamHeader(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	lead, _ := teamsui.Lead(team.Members)
	pause := look.HeaderAction{Icon: look.Icon.Pause, Label: i18n.T("Pause"), Description: i18n.T("Pause the team: its running tasks stop and its routines wait")}
	if team.Paused {
		pause = look.HeaderAction{Icon: look.Icon.Play, Label: i18n.T("Resume"), Description: i18n.T("Let the team work again")}
		if team.PauseReason == protocol.PausedByBudget {
			pause.Description = i18n.Tf("Raise the limit to %s tokens a month and let the team work again", teamsui.Tokens(raisedLimit(team)))
		}
	}
	facts := []look.HeaderFact{{Text: i18n.Tf("Led by %s, %s", lead.Name, membersWord(len(team.Members)))}}
	if runs := runsFact(e.Snapshot(), team.Members); runs != "" {
		facts = append(facts, look.HeaderFact{Text: runs})
	}
	// the folder, the one link, stands after however many facts there are
	folder := len(facts)
	content := look.PaneHeaderContent{
		Title: team.Name,
		Facts: append(facts, look.HeaderFact{Text: paths.Short(team.Folder), Kind: look.FactLink}),
		Actions: []look.HeaderAction{
			{Icon: look.Icon.Add, Description: i18n.T("Give the team a task")},
			pause,
		},
		Menu: []kit.MenuItem{
			{Icon: look.Icon.Export, Label: i18n.T("Export team")},
			{Divider: true},
			{Icon: look.Icon.Delete, Label: i18n.T("Delete team"), Danger: true},
		},
	}
	content.Claim, content.ClaimTone, _ = teamsui.TeamStanding(team)
	dimensions := p.header.Layout(gtx, e.Theme, e.Overlays, content)
	if index, ok := p.header.ActionClicked(); ok {
		if index == 0 {
			e.NewTeamTask(team.ID)
		} else {
			togglePause(e, team)
		}
	}
	if index, ok := p.header.LinkClicked(); ok && index == folder {
		e.OpenFolder(team.Folder)
	}
	if item, ok := p.header.MenuClicked(); ok {
		if item == 0 {
			exportTeam(e, team)
		} else {
			p.remove.open(team)
		}
	}
	return dimensions
}

func (p *Page) layoutTeamTabs(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	th := e.Theme
	p.tabs.Labels = []string{i18n.T("Overview"), i18n.T("Work"), i18n.T("Routines"), i18n.T("Spend"), i18n.T("Settings")}
	for index, view := range teamViews {
		if view == p.place.View {
			p.tabs.Selected = index
		}
	}
	// a tab label sits 12 inside its box so the strip starts 12 early to put the first label under the title
	dimensions := layout.Inset{Left: 9}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return p.tabs.Layout(gtx, th.Theme)
	})
	if index, changed := p.tabs.Changed(); changed && index < len(teamViews) {
		openTeam(e, team.ID, teamViews[index])
	}
	line := max(1, gtx.Dp(1))
	width := gtx.Constraints.Max.X
	look.FillRect(gtx, image.Rect(0, dimensions.Size.Y-line, width, dimensions.Size.Y), th.R.Vein)
	return layout.Dimensions{Size: image.Pt(width, dimensions.Size.Y)}
}

func (p *Page) layoutTeamView(gtx layout.Context, e *env.Env, team protocol.Team) layout.Dimensions {
	switch p.place.View {
	case env.TeamWork:
		return p.work.layout(gtx, e, team)
	case env.TeamRoutines:
		return p.routines.layout(gtx, e, team)
	case env.TeamSpend:
		return p.spend.layout(gtx, e, p, team)
	case env.TeamSettings:
		return p.settings.layout(gtx, e, p, team)
	default:
		return p.overview.layout(gtx, e, team)
	}
}
