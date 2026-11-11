package agent

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a select for each thing the session's harness lets you choose, shown only once the harness has said so
type harnessControls struct {
	session  string
	revision uint64
	current  [2]string

	model, mode choiceList
	extras      []extraChoice

	modelSelect, modeSelect look.Select
	extraSelects            map[string]*look.Select
	approvals               env.Approvals

	// sessions whose harness was asked to come up, so opening one asks once
	started map[string]bool
	// the one being brought up right now
	waking string
	// the one this page brought up and nothing was asked of yet
	woke string
}

type choiceList struct {
	options  []look.SelectOption
	ids      []string
	selected int
}

type extraChoice struct {
	id, name, category string
	list               choiceList
}

func listOf(choices []protocol.Choice, current string) choiceList {
	list := choiceList{options: make([]look.SelectOption, len(choices)), ids: make([]string, len(choices)), selected: -1}
	for index, choice := range choices {
		list.options[index], list.ids[index] = env.ChoiceOption(choice), choice.ID
		if choice.ID == current {
			list.selected = index
		}
	}
	return list
}

// rebuilds the lists only when the harness reported something new: a model list runs to hundreds
func (c *harnessControls) sync(attached state.Attached) {
	info := attached.Info
	revision := attached.Model.ChoiceRevision()
	current := [2]string{info.Model, info.Mode}
	if c.session == info.ID && c.revision == revision && c.current == current && c.extraSelects != nil {
		return
	}
	c.session, c.revision, c.current = info.ID, revision, current
	if c.extraSelects == nil {
		c.extraSelects = make(map[string]*look.Select)
	}
	model, mode := attached.Model.ModelChoice(), attached.Model.ModeChoice()
	if model.Current == "" {
		model.Current = info.Model
	}
	if mode.Current == "" {
		mode.Current = info.Mode
	}
	c.model, c.mode = listOf(model.Options, model.Current), listOf(mode.Options, mode.Current)
	c.extras = c.extras[:0]
	for _, option := range attached.Model.Options() {
		if len(option.Choices) == 0 {
			continue
		}
		c.extras = append(c.extras, extraChoice{id: option.ID, name: option.Name, category: option.Category, list: listOf(option.Choices, option.Current)})
		if c.extraSelects[option.ID] == nil {
			c.extraSelects[option.ID] = new(look.Select)
		}
	}
}

func (c *harnessControls) known() bool {
	return len(c.model.ids) > 0 || len(c.mode.ids) > 0 || len(c.extras) > 0
}

// brings the harness of a session that never ran up, so its choices are there before the first prompt
func (c *harnessControls) wake(e *env.Env, info protocol.SessionInfo) {
	if c.known() || info.Alive || info.State == protocol.StateWorking || c.started[info.ID] {
		return
	}
	if c.started == nil {
		c.started = make(map[string]bool)
	}
	c.started[info.ID] = true
	c.waking, c.woke = info.ID, info.ID
	e.Call(protocol.MSessionStart, protocol.SessionRef{ID: info.ID}, new(protocol.Empty), func(err error) {
		if c.waking == info.ID {
			c.waking = ""
		}
		if err != nil {
			e.Notify(err.Error())
		}
	})
}

// a harness brought up only to be looked at is stopped when its session is left, or every one clicked through would stay running
func (c *harnessControls) leave(e *env.Env, id string) {
	if id == "" || id != c.woke {
		return
	}
	c.woke = ""
	delete(c.started, id)
	info, known := e.Snapshot().Sessions[id]
	if known && info.Turns == 0 && info.State != protocol.StateWorking && info.State != protocol.StateWaiting {
		e.Call(protocol.MSessionStop, protocol.SessionRef{ID: id}, new(protocol.Empty), nil)
	}
}

func optionIcon(category string) *widget.Icon {
	if category == "thought_level" {
		return look.Icon.Thought
	}
	return nil
}

// home is the composer's text: a choice made in its row gives the keyboard back to it
func (c *harnessControls) widgets(e *env.Env, snapshot state.Snapshot, info protocol.SessionInfo, home any) []layout.Widget {
	th := e.Theme
	quiet := func(selection *look.Select, label string) {
		selection.Form, selection.Label, selection.Return = look.SelectQuiet, label, home
	}
	controls := make([]layout.Widget, 0, len(c.extras)+4)
	if c.waking == info.ID && !c.known() {
		name := harnessName(snapshot, info.Harness)
		controls = append(controls, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(kit.RawText(th.Theme, th.T.Label, th.R.Fg3, i18n.Tf("Starting %s", name))),
				)
			})
		})
	}
	if len(c.model.ids) > 0 {
		quiet(&c.modelSelect, i18n.T("Model"))
		controls = append(controls, func(gtx layout.Context) layout.Dimensions {
			return c.modelSelect.LayoutOptions(gtx, th, e.Overlays, c.model.options, c.model.selected)
		})
	}
	// a mode with nothing to switch to is not a choice
	if len(c.mode.ids) > 1 {
		quiet(&c.modeSelect, i18n.T("Mode"))
		controls = append(controls, func(gtx layout.Context) layout.Dimensions {
			return c.modeSelect.LayoutOptions(gtx, th, e.Overlays, c.mode.options, c.mode.selected)
		})
	}
	for _, extra := range c.extras {
		selection := c.extraSelects[extra.id]
		quiet(selection, extra.name)
		selection.Icon = optionIcon(extra.category)
		controls = append(controls, func(gtx layout.Context) layout.Dimensions {
			return selection.LayoutOptions(gtx, th, e.Overlays, extra.list.options, extra.list.selected)
		})
	}
	controls = append(controls, func(gtx layout.Context) layout.Dimensions {
		return c.approvals.Layout(gtx, e, env.AgentApprovals, "", snapshot.Settings.Agent.Approvals, home)
	})
	return controls
}

func (c *harnessControls) update(e *env.Env, id string) {
	if index, changed := c.modelSelect.Changed(); changed && index >= 0 && index < len(c.model.ids) {
		e.Call(protocol.MSessionSetModel, protocol.SetModelParams{ID: id, Model: c.model.ids[index]}, new(protocol.Empty), e.Report)
	}
	if index, changed := c.modeSelect.Changed(); changed && index >= 0 && index < len(c.mode.ids) {
		e.Call(protocol.MSessionSetMode, protocol.SetModeParams{ID: id, Mode: c.mode.ids[index]}, new(protocol.Empty), e.Report)
	}
	for _, extra := range c.extras {
		if index, changed := c.extraSelects[extra.id].Changed(); changed && index >= 0 && index < len(extra.list.ids) {
			params := protocol.SetOptionParams{ID: id, Option: extra.id, Value: extra.list.ids[index]}
			e.Call(protocol.MSessionSetOption, params, new(protocol.Empty), e.Report)
		}
	}
	if value, changed := c.approvals.Changed(); changed {
		e.SetApprovals(env.AgentApprovals, value)
	}
}
