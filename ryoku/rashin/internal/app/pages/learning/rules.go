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

type ruleActions struct{ edit, remove kit.Plaque }

type rulesSection struct {
	rules       []protocol.Rule
	actions     map[string]*ruleActions
	pending     uint64
	err         string
	edit        ruleEditor
	forge       ruleForge
	newRule     kit.Plaque
	forgeButton kit.Plaque
}

func (s *rulesSection) load(e *env.Env) {
	s.pending++
	request := s.pending
	var result protocol.LearningRulesResult
	e.Call(protocol.MLearningRules, protocol.LearningRulesParams{}, &result, func(err error) {
		if request != s.pending {
			return
		}
		if err != nil {
			s.err = err.Error()
			return
		}
		s.rules, s.err = result.Rules, ""
	})
}

func (s *rulesSection) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	if s.actions == nil {
		s.actions = make(map[string]*ruleActions)
	}
	rows := []look.Setting{}
	if s.err != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Could not read rules"), Detail: s.err})
	}
	for _, rule := range s.rules {
		rule := rule
		key := rule.Project + "\x00" + rule.Name
		actions := s.actions[key]
		if actions == nil {
			actions = &ruleActions{}
			s.actions[key] = actions
		}
		detail := rule.Description
		if rule.Project != "" {
			detail = projectName(rule.Project) + " · " + detail
		} else {
			detail = i18n.T("Every project") + " · " + detail
		}
		if !rule.Enabled {
			detail += " · " + i18n.T("Off")
		}
		if rule.Error != "" {
			detail += " · " + rule.Error
		}
		rows = append(rows, look.Setting{Title: rule.Name, Detail: detail, Below: true, Control: func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if actions.edit.Clicked(gtx) {
						s.edit.open(rule, true)
					}
					actions.edit.Kind, actions.edit.Small, actions.edit.Label = kit.PlaqueTonal, true, i18n.T("Edit rule")
					return actions.edit.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if actions.remove.Clicked(gtx) {
						s.remove(e, rule)
					}
					actions.remove.Kind, actions.remove.Small, actions.remove.Label = kit.PlaqueDangerLine, true, i18n.T("Delete rule")
					return actions.remove.Layout(gtx, th.Theme)
				}),
			)
		}})
	}
	if len(s.rules) == 0 && s.err == "" {
		rows = append(rows, look.Setting{Title: i18n.T("No rules yet"), Detail: i18n.T("Create one, or make one from a complaint about a conversation.")})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.forge.layout(gtx, e, s) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.edit.layout(gtx, e, s) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.newRule.Clicked(gtx) {
						s.edit.open(protocol.Rule{Enabled: true, Interrupt: protocol.RuleInterruptNever, Repeat: protocol.RuleRepeatOnce}, false)
					}
					s.newRule.Kind, s.newRule.Small, s.newRule.Label = kit.PlaqueTonal, true, i18n.T("New rule")
					return s.newRule.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.forgeButton.Clicked(gtx) {
						s.forge.open = true
					}
					s.forgeButton.Kind, s.forgeButton.Small, s.forgeButton.Label = kit.PlaqueTonal, true, i18n.T("Make a rule from a complaint")
					if !e.Feature(protocol.FeatureRules) {
						return s.forgeButton.Layout(gtx.Disabled(), th.Theme)
					}
					return s.forgeButton.Layout(gtx, th.Theme)
				}),
			)
		}),
	)
}

func (s *rulesSection) remove(e *env.Env, rule protocol.Rule) {
	e.Call(protocol.MLearningRuleDelete, protocol.RuleRef{Name: rule.Name, Project: rule.Project}, new(protocol.Empty), func(err error) {
		if err != nil {
			e.Report(err)
			return
		}
		s.load(e)
		e.Notify(i18n.T("Rule deleted."))
	})
}

type ruleEditor struct {
	editing                                     bool
	original                                    protocol.Rule
	existing                                    bool
	name, description, project, question, scope kit.TextField
	condition, globs, body                      look.MultiLine
	interrupt, repeat                           look.Select
	enabled, always                             kit.Toggle
	save, cancel                                kit.Plaque
	busy                                        bool
	problem                                     string
}

func (f *ruleEditor) open(rule protocol.Rule, existing bool) {
	f.original, f.existing, f.editing, f.problem, f.busy = rule, existing, true, "", false
	f.name.SetText(rule.Name)
	f.description.SetText(rule.Description)
	f.project.SetText(rule.Project)
	f.question.SetText(rule.Question)
	f.scope.SetText(rule.Scope)
	f.condition.SetText(strings.Join(rule.Condition, "\n"))
	f.globs.SetText(strings.Join(rule.Globs, "\n"))
	f.body.SetText(rule.Body)
	f.enabled.Value, f.always.Value = rule.Enabled, rule.AlwaysApply
}

var interrupts = []string{protocol.RuleInterruptNever, protocol.RuleInterruptProseOnly, protocol.RuleInterruptToolOnly, protocol.RuleInterruptAlways}
var repeats = []string{protocol.RuleRepeatOnce, protocol.RuleRepeatAfterGap, protocol.RuleRepeatAlways}

func choiceIndex(options []string, value string) int {
	for index, choice := range options {
		if choice == value {
			return index
		}
	}
	return 0
}

func nonblankLines(text string) []string {
	var result []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}

func (f *ruleEditor) value() protocol.Rule {
	rule := f.original
	rule.Name = strings.TrimSpace(f.name.Text())
	rule.Description = strings.TrimSpace(f.description.Text())
	rule.Project = strings.TrimSpace(f.project.Text())
	rule.Question = strings.TrimSpace(f.question.Text())
	rule.Scope = strings.TrimSpace(f.scope.Text())
	rule.Condition = nonblankLines(f.condition.Text())
	rule.Globs = nonblankLines(f.globs.Text())
	rule.Body = strings.TrimSpace(f.body.Text())
	rule.Interrupt = interrupts[choiceIndex(interrupts, f.original.Interrupt)]
	rule.Repeat = repeats[choiceIndex(repeats, f.original.Repeat)]
	rule.Enabled, rule.AlwaysApply = f.enabled.Value, f.always.Value
	return rule
}

func (f *ruleEditor) layout(gtx layout.Context, e *env.Env, section *rulesSection) layout.Dimensions {
	if !f.editing {
		return layout.Dimensions{}
	}
	th := e.Theme
	if chosen, ok := f.interrupt.Changed(); ok && chosen < len(interrupts) {
		f.original.Interrupt = interrupts[chosen]
	}
	if chosen, ok := f.repeat.Changed(); ok && chosen < len(repeats) {
		f.original.Repeat = repeats[chosen]
	}
	if f.cancel.Clicked(gtx) {
		f.editing = false
		return layout.Dimensions{}
	}
	if f.save.Clicked(gtx) && !f.busy {
		rule := f.value()
		if rule.Name == "" || rule.Body == "" {
			f.problem = i18n.T("Give the rule a name and a body.")
		} else {
			f.busy, f.problem = true, ""
			var saved protocol.Rule
			e.Call(protocol.MLearningRuleSave, protocol.RuleSaveParams{Rule: rule, Overwrite: f.existing}, &saved, func(err error) {
				f.busy = false
				if err != nil {
					f.problem = err.Error()
					return
				}
				f.editing = false
				section.load(e)
				e.Notify(i18n.T("Rule saved."))
			})
		}
	}
	f.name.Label, f.description.Label, f.project.Label = i18n.T("Name"), i18n.T("Description"), i18n.T("Project folder (blank for every project)")
	f.question.Label, f.scope.Label = i18n.T("Question for the judge (optional)"), i18n.T("Scope (optional)")
	f.condition.Label, f.globs.Label, f.body.Label = i18n.T("Conditions (one pattern per line)"), i18n.T("File patterns (one per line)"), i18n.T("Rule text")
	f.condition.Lines, f.globs.Lines, f.body.Lines = 2, 2, 4
	widgets := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Edit rule"))(gtx)
		},
		func(gtx layout.Context) layout.Dimensions {
			if f.existing {
				gtx = gtx.Disabled()
			}
			return f.name.Layout(gtx, th.Theme)
		},
		func(gtx layout.Context) layout.Dimensions { return f.description.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if f.existing {
				gtx = gtx.Disabled()
			}
			return f.project.Layout(gtx, th.Theme)
		},
		func(gtx layout.Context) layout.Dimensions { return f.body.Layout(gtx, th) },
		func(gtx layout.Context) layout.Dimensions { return f.condition.Layout(gtx, th) },
		func(gtx layout.Context) layout.Dimensions { return f.question.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return f.scope.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return f.globs.Layout(gtx, th) },
		func(gtx layout.Context) layout.Dimensions {
			return kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Interrupt"))(gtx)
		},
		func(gtx layout.Context) layout.Dimensions {
			return f.interrupt.Layout(gtx, th, e.Overlays, []string{i18n.T("Never"), i18n.T("Prose only"), i18n.T("Tool only"), i18n.T("Always")}, choiceIndex(interrupts, f.original.Interrupt))
		},
		func(gtx layout.Context) layout.Dimensions {
			return kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Repeat"))(gtx)
		},
		func(gtx layout.Context) layout.Dimensions {
			return f.repeat.Layout(gtx, th, e.Overlays, []string{i18n.T("Once"), i18n.T("After a gap"), i18n.T("Every time")}, choiceIndex(repeats, f.original.Repeat))
		},
		func(gtx layout.Context) layout.Dimensions { return switchRow(gtx, th, &f.enabled, i18n.T("Enabled")) },
		func(gtx layout.Context) layout.Dimensions {
			return switchRow(gtx, th, &f.always, i18n.T("Always apply"))
		},
		func(gtx layout.Context) layout.Dimensions {
			return look.Paragraph(th, th.T.Detail, th.R.Jasper, f.problem)(gtx)
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					f.save.Kind, f.save.Label, f.save.Busy = kit.PlaqueGold, i18n.T("Save rule"), f.busy
					return f.save.Layout(gtx, th.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					f.cancel.Kind, f.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
					return f.cancel.Layout(gtx, th.Theme)
				}),
			)
		},
	}
	return layout.Inset{Top: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, len(widgets))
			for index, widget := range widgets {
				children[index] = layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Inset{Bottom: th.Sp.S2}.Layout(gtx, widget) })
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func switchRow(gtx layout.Context, th *look.Theme, toggle *kit.Toggle, label string) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return toggle.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(kit.Text(th.Theme, th.T.Body, th.R.Fg, label)),
	)
}

type ruleForge struct {
	open, busy                    bool
	session                       kit.TextField
	complaint, feedback           look.MultiLine
	propose, revise, save, cancel kit.Plaque
	proposal                      protocol.RuleForgeResult
	problem                       string
}

func (f *ruleForge) layout(gtx layout.Context, e *env.Env, section *rulesSection) layout.Dimensions {
	if !f.open || !e.Feature(protocol.FeatureRules) {
		return layout.Dimensions{}
	}
	th := e.Theme
	f.session.Label, f.complaint.Label, f.feedback.Label = i18n.T("Conversation ID"), i18n.T("What went wrong?"), i18n.T("How should it change? (optional)")
	f.complaint.Lines, f.feedback.Lines = 3, 2
	if f.cancel.Clicked(gtx) {
		f.open = false
		return layout.Dimensions{}
	}
	if (f.propose.Clicked(gtx) || f.revise.Clicked(gtx)) && !f.busy {
		if strings.TrimSpace(f.session.Text()) == "" || strings.TrimSpace(f.complaint.Text()) == "" {
			f.problem = i18n.T("Name a conversation and describe what went wrong.")
		} else {
			var previous *protocol.Rule
			if f.proposal.OK {
				previous = &f.proposal.Rule
			}
			f.busy, f.problem = true, ""
			var result protocol.RuleForgeResult
			e.Call(protocol.MLearningRuleForge, protocol.RuleForgeParams{Session: strings.TrimSpace(f.session.Text()), Complaint: strings.TrimSpace(f.complaint.Text()), Feedback: strings.TrimSpace(f.feedback.Text()), Previous: previous}, &result, func(err error) {
				f.busy = false
				if err != nil {
					f.problem = err.Error()
					return
				}
				f.proposal = result
				if !result.OK {
					f.problem = result.Message
				}
			})
		}
	}
	if f.save.Clicked(gtx) && f.proposal.OK {
		section.edit.open(f.proposal.Rule, false)
		f.open = false
	}
	widgets := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Make a rule from a complaint"))(gtx)
		},
		func(gtx layout.Context) layout.Dimensions { return f.session.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return f.complaint.Layout(gtx, th) },
		func(gtx layout.Context) layout.Dimensions { return f.feedback.Layout(gtx, th) },
		func(gtx layout.Context) layout.Dimensions {
			return look.Paragraph(th, th.T.Detail, th.R.Jasper, f.problem)(gtx)
		},
	}
	if f.proposal.OK {
		widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
			return look.SettingsSlab(gtx, th, []look.Setting{{Title: f.proposal.Rule.Name, Detail: f.proposal.Rule.Body}, {Title: i18n.T("Matched in the conversation"), Detail: f.proposal.Matched}})
		})
	}
	widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
		actions := []layout.Widget{
			func(gtx layout.Context) layout.Dimensions {
				f.propose.Kind, f.propose.Label, f.propose.Busy = kit.PlaqueTonal, i18n.T("Propose rule"), f.busy
				return f.propose.Layout(gtx, th.Theme)
			},
			func(gtx layout.Context) layout.Dimensions {
				f.revise.Kind, f.revise.Label = kit.PlaqueQuiet, i18n.T("Revise")
				if !f.proposal.OK {
					return f.revise.Layout(gtx.Disabled(), th.Theme)
				}
				return f.revise.Layout(gtx, th.Theme)
			},
			func(gtx layout.Context) layout.Dimensions {
				f.save.Kind, f.save.Label = kit.PlaqueGold, i18n.T("Review and save")
				if !f.proposal.OK {
					return f.save.Layout(gtx.Disabled(), th.Theme)
				}
				return f.save.Layout(gtx, th.Theme)
			},
			func(gtx layout.Context) layout.Dimensions {
				f.cancel.Kind, f.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
				return f.cancel.Layout(gtx, th.Theme)
			},
		}
		return look.Flow{Gap: th.Sp.S2, LineGap: th.Sp.S2}.Layout(gtx, len(actions), func(gtx layout.Context, index int) layout.Dimensions { return actions[index](gtx) })
	})
	return layout.Inset{Top: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, len(widgets))
			for index, widget := range widgets {
				children[index] = layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Inset{Bottom: th.Sp.S2}.Layout(gtx, widget) })
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}
