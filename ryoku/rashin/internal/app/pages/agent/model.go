package agent

import (
	"slices"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the model a new session starts on, remembered per harness; empty leaves it to the harness
type modelPick struct {
	selection look.Select
	read      kit.Plaque
	reader    env.HarnessProbe
}

// a pick the list no longer shows stays as itself so the select never claims the default for it
func (m *modelPick) choices(info protocol.HarnessInfo, picked string) ([]look.SelectOption, []string) {
	ids := []string{""}
	options := []look.SelectOption{{Label: env.HarnessDefaultLabel(info)}}
	found := picked == ""
	for _, choice := range info.Capabilities.ModelList {
		ids = append(ids, choice.ID)
		options = append(options, env.ChoiceOption(choice))
		found = found || choice.ID == picked
	}
	if !found {
		ids = append(ids, picked)
		options = append(options, look.SelectOption{Label: picked, Detail: i18n.T("Not in the harness's list")})
	}
	return options, ids
}

// a harness that never ran has no list yet: reading it starts the harness once in the sandbox without a prompt
func (m *modelPick) layout(gtx layout.Context, e *env.Env, harness protocol.HarnessInfo, form look.SelectForm, home any) layout.Dimensions {
	th := e.Theme
	info := m.reader.Info(e.Snapshot(), harness.ID)
	if info.Capabilities == nil || len(info.Capabilities.ModelList) == 0 {
		m.read.Kind, m.read.Small = kit.PlaqueQuiet, true
		m.read.Label, m.read.Busy = i18n.T("Read the list"), m.reader.Busy
		ready := info.Installed && !m.reader.Busy
		if m.read.Clicked(gtx) && ready {
			m.reader.Run(e, protocol.HarnessProbeParams{ID: info.ID}, info.Name, false)
		}
		if !ready {
			gtx = gtx.Disabled()
		}
		return m.read.Layout(gtx, th.Theme)
	}
	picked := e.AgentModels[info.ID]
	options, ids := m.choices(info, picked)
	if index, changed := m.selection.Changed(); changed && index >= 0 && index < len(ids) && ids[index] != picked {
		e.SetAgentModel(info.ID, ids[index])
		picked = ids[index]
	}
	m.selection.Form, m.selection.Label, m.selection.Return = form, i18n.T("Model"), home
	return m.selection.LayoutOptions(gtx, th, e.Overlays, options, max(0, slices.Index(ids, picked)))
}

// what the last read said, as rows for under the controls; none while it has nothing to say
func (m *modelPick) noteRows(th *look.Theme) []layout.FlexChild {
	if m.reader.Result == "" {
		return nil
	}
	ink := th.R.Fg3
	if m.reader.Failed {
		ink = th.R.Jasper
	}
	return []layout.FlexChild{
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Caption, ink, m.reader.Result)),
	}
}
