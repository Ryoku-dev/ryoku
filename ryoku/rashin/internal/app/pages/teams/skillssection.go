package teams

import (
	"slices"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/skillsui"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a team's members get a set only while it is on here and in Settings, so a set off for every model stands disabled
type skillsSection struct {
	list skillsui.List
	// by team and set, as one tab shows every team in turn
	rows map[string]*skillsToggle
}

type skillsToggle struct {
	toggle kit.Toggle
	held   teamsui.Held[bool]
	// whether the team gets the set as this frame shows it
	shown bool
}

func (k *skillsSection) row(team, set string) *skillsToggle {
	if k.rows == nil {
		k.rows = make(map[string]*skillsToggle)
	}
	key := team + "\x00" + set
	row := k.rows[key]
	if row == nil {
		row = new(skillsToggle)
		k.rows[key] = row
	}
	return row
}

func (k *skillsSection) layout(e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	result, ready := k.list.Sets(e)
	rows := make([]look.Setting, 0, len(result.Sets))
	for _, set := range result.Sets {
		row := k.row(team.ID, set.ID)
		row.shown = row.held.Show(!slices.Contains(team.SkillsOff, set.ID))
		everywhere := skillsui.On(snapshot.Settings.Skills, set.ID)
		detail := set.Summary
		if !everywhere {
			detail = i18n.T("Off for every model in Settings, Skills.")
		}
		name := i18n.Tf("%s for this team", set.Name)
		rows = append(rows, look.Setting{Title: set.Name, Detail: detail, Control: func(gtx layout.Context) layout.Dimensions {
			row.toggle.Value = row.shown && everywhere
			if !everywhere {
				gtx = gtx.Disabled()
			}
			dimensions := look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return row.toggle.Layout(gtx, th.Theme) })
			if row.toggle.Changed() && everywhere {
				on := row.toggle.Value
				// what other toggles still hold goes too, so a quick second flip does not undo the first
				off := team.SkillsOff
				for _, other := range result.Sets {
					off = skillsui.Turn(off, other.ID, k.row(team.ID, other.ID).shown)
				}
				row.held.Hold(on)
				updateTeam(e, team.ID, map[string]any{"skillsOff": skillsui.Turn(off, set.ID, on)}, row.held.Drop)
			}
			return dimensions
		}})
	}
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Skills"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("The built-in skill sets this team's members get, while the set is on in Settings too."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		}
		switch {
		case k.list.Problem() != "":
			children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, i18n.Tf("Rashin could not list its skills: %s", k.list.Problem()))))
		case !ready:
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SkeletonRows(gtx, th, 4, true) }))
		default:
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
