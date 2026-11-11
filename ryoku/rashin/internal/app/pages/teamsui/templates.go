package teamsui

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// Templates are the ready-made teams, asked of the daemon once for each
// connection to it.
type Templates struct {
	list  []protocol.TeamTemplate
	asked bool
	epoch int
}

// the templates, empty until the daemon answers; a refusal is told once and
// not asked again until the connection changes
func (t *Templates) List(e *env.Env) []protocol.TeamTemplate {
	snapshot := e.Snapshot()
	if snapshot.Connection == state.Ready && (!t.asked || t.epoch != snapshot.Epoch) {
		t.asked, t.epoch = true, snapshot.Epoch
		var result protocol.TeamTemplatesResult
		e.Call(protocol.MTeamsTemplates, protocol.Empty{}, &result, func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			t.list = result.Templates
		})
	}
	return t.list
}

// a template's purpose says what the team would be for; the blank one has
// none, so its description stands in
func TemplateLine(template protocol.TeamTemplate) string {
	if template.Purpose != "" {
		return template.Purpose
	}
	return template.Description
}

// TeamStart offers the templates to start a first team from, each opening
// the draft of a new team filled from it.
type TeamStart struct {
	tablets map[string]*kit.Tablet
	create  map[string]*kit.Plaque
}

func (s *TeamStart) Layout(gtx layout.Context, e *env.Env, templates []protocol.TeamTemplate) layout.Dimensions {
	th := e.Theme
	if s.tablets == nil {
		s.tablets = make(map[string]*kit.Tablet)
		s.create = make(map[string]*kit.Plaque)
	}
	return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Start a team"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("Pick what it is for. You can change its members, tools and limits before it is made."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		}
		if len(templates) == 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.SkeletonRows(gtx, th, 5, true)
			}))
		}
		for _, template := range templates {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return s.layoutTemplate(gtx, e, template)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (s *TeamStart) layoutTemplate(gtx layout.Context, e *env.Env, template protocol.TeamTemplate) layout.Dimensions {
	th := e.Theme
	tablet, create := s.tablets[template.ID], s.create[template.ID]
	if tablet == nil {
		tablet, create = new(kit.Tablet), new(kit.Plaque)
		s.tablets[template.ID], s.create[template.ID] = tablet, create
	}
	tablet.Pane = th.R.Stone1
	create.Kind, create.Small, create.Label = kit.PlaqueLine, true, i18n.T("Create")
	if tablet.Clicked(gtx) || create.Clicked(gtx) {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Draft: env.DraftTeam, Template: template.ID})
	}
	content := kit.TabletContent{
		Title: template.Name, Facts: TemplateLine(template), Action: create,
		Mark: Mark(th, template.Name, 32, false),
	}
	return tablet.Layout(gtx, th.Theme, content, false, false)
}
