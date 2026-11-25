package learning

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type skillRow struct{ open, remove kit.Plaque }

type skillsSection struct {
	result  protocol.LearningSkillsResult
	rows    map[string]*skillRow
	detail  protocol.SkillContent
	opened  string
	pending uint64
	err     string
}

func (s *skillsSection) load(e *env.Env) {
	s.pending++
	request := s.pending
	var result protocol.LearningSkillsResult
	e.Call(protocol.MLearningSkills, nil, &result, func(err error) {
		if request != s.pending {
			return
		}
		if err != nil {
			s.err = err.Error()
			return
		}
		s.result, s.err = result, ""
	})
}

func (s *skillsSection) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	if s.rows == nil {
		s.rows = make(map[string]*skillRow)
	}
	rows := []look.Setting{}
	if s.err != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Could not read learned skills"), Detail: s.err})
	}
	for _, skill := range s.result.Skills {
		skill := skill
		state := s.rows[skill.Name]
		if state == nil {
			state = &skillRow{}
			s.rows[skill.Name] = state
		}
		detail := skill.Description
		if skill.Shadowed {
			detail += " " + i18n.T("A built-in or authored skill with this name takes precedence.")
		}
		rows = append(rows, look.Setting{Title: skill.Name, Detail: detail, Below: true, Control: func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if state.open.Clicked(gtx) {
						if s.opened == skill.Name {
							s.opened = ""
						} else {
							s.read(e, skill.Name)
						}
					}
					state.open.Kind, state.open.Small, state.open.Label = kit.PlaqueTonal, true, i18n.T("Open skill")
					return state.open.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if state.remove.Clicked(gtx) {
						s.delete(e, skill.Name)
					}
					state.remove.Kind, state.remove.Small, state.remove.Label = kit.PlaqueDangerLine, true, i18n.T("Delete skill")
					return state.remove.Layout(gtx, th.Theme)
				}),
			)
		}})
	}
	if len(s.result.Skills) == 0 && s.err == "" {
		rows = append(rows, look.Setting{Title: i18n.T("No learned skills yet"), Detail: i18n.T("Hyakki will list skills it writes here.")})
	}
	if s.opened != "" {
		text := s.detail.Text
		if text == "" {
			text = i18n.T("Reading skill…")
		}
		rows = append(rows, look.Setting{Title: s.opened, Detail: strings.TrimSpace(text), Below: true})
	}
	return look.SettingsSlab(gtx, th, rows)
}

func (s *skillsSection) read(e *env.Env, name string) {
	s.opened, s.detail = name, protocol.SkillContent{}
	var result protocol.SkillContent
	e.Call(protocol.MSkillsGet, protocol.SkillParams{ID: protocol.ManagedSkillSet + "/" + name}, &result, func(err error) {
		if s.opened != name {
			return
		}
		if err != nil {
			s.detail.Text = err.Error()
			return
		}
		s.detail = result
	})
}

func (s *skillsSection) delete(e *env.Env, name string) {
	var result protocol.LearningSkillsResult
	e.Call(protocol.MLearningSkillDelete, protocol.LearningSkillParams{Name: name}, &result, func(err error) {
		if err != nil {
			e.Report(err)
			return
		}
		s.result = result
		if s.opened == name {
			s.opened, s.detail = "", protocol.SkillContent{}
		}
		e.Notify(i18n.T("Skill deleted."))
	})
}
