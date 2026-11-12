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

// the templates of one level, in the order the daemon offers them; the level is empty for the blank one
type templateGroup struct {
	level     string
	templates []protocol.TeamTemplate
}

// the levels in the order they are offered; a template of no level known here stands last with the blank one
var templateLevels = []string{protocol.LevelLight, protocol.LevelMedium, protocol.LevelAdvanced, ""}

// light first and the blank one last, so the picker reads from what asks least to what asks most
func templateGroups(templates []protocol.TeamTemplate) []templateGroup {
	groups := make([]templateGroup, 0, len(templateLevels))
	for _, level := range templateLevels {
		group := templateGroup{level: level}
		for _, template := range templates {
			if templateLevel(template) == level {
				group.templates = append(group.templates, template)
			}
		}
		if len(group.templates) > 0 {
			groups = append(groups, group)
		}
	}
	return groups
}

func templateLevel(template protocol.TeamTemplate) string {
	switch template.Level {
	case protocol.LevelLight, protocol.LevelMedium, protocol.LevelAdvanced:
		return template.Level
	}
	return ""
}

// a group's name and, for a level, one line on what it asks of the user
func levelHeader(th *look.Theme, level string) layout.Widget {
	title, line := i18n.T("From scratch"), ""
	switch level {
	case protocol.LevelLight:
		title, line = i18n.T("Light"), i18n.T("One member and nothing to set up first.")
	case protocol.LevelMedium:
		title, line = i18n.T("Medium"), i18n.T("A lead who hands work to a member, and a credential it may ask for.")
	case protocol.LevelAdvanced:
		title, line = i18n.T("Advanced"), i18n.T("Members on different models, routines of their own and the credentials they need.")
	}
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg, title))}
		if line != "" {
			children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, line)))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// LayoutGroups lays the templates out under their level headers, each template drawn by row.
func LayoutGroups(gtx layout.Context, th *look.Theme, templates []protocol.TeamTemplate, row func(gtx layout.Context, template protocol.TeamTemplate) layout.Dimensions) layout.Dimensions {
	var children []layout.FlexChild
	for index, group := range templateGroups(templates) {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
		}
		children = append(children,
			layout.Rigid(levelHeader(th, group.level)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		)
		for position, template := range group.templates {
			if position > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row(gtx, template) }))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
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
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Pick what it is for. You can change its members, tools and limits before it is made."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		}
		if len(templates) == 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.SkeletonRows(gtx, th, 5, true)
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return LayoutGroups(gtx, th, templates, func(gtx layout.Context, template protocol.TeamTemplate) layout.Dimensions {
				return s.layoutTemplate(gtx, e, template)
			})
		}))
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
	tablet.Pane = th.R.Slab
	create.Kind, create.Small, create.Label = kit.PlaqueTonal, true, i18n.T("Create")
	if tablet.Clicked(gtx) || create.Clicked(gtx) {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Draft: env.DraftTeam, Template: template.ID})
	}
	content := kit.TabletContent{
		Title: template.Name, Facts: TemplateLine(template), Action: create,
		Mark: Mark(th, template.Name, 32, false),
	}
	return tablet.Layout(gtx, th.Theme, content, false, false)
}
