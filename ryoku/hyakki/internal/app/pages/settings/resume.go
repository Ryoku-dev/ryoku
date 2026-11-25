package settings

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
)

type resumeBlock struct {
	window look.Select
}

func (p *Page) resumeBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	th := e.Theme
	minutes := e.Snapshot().Settings.Ask.ResumeMinutes
	values := []int{0, 10, 30, 60}
	labels := []string{i18n.T("Off"), i18n.T("10 minutes"), i18n.T("30 minutes"), i18n.T("60 minutes")}
	selected := 0
	for index, value := range values {
		if value == minutes {
			selected = index
		}
	}
	if minutes != 0 && minutes != 10 && minutes != 30 && minutes != 60 {
		selected = len(values)
		values = append(values, minutes)
		labels = append(labels, i18n.Tf("%d minutes", minutes))
	}
	rows := []look.Setting{{
		Title: i18n.T("Resume conversation"), Detail: i18n.T("Opening Ask returns to your latest conversation within this window. New ask always starts fresh."),
		Control: func(gtx layout.Context) layout.Dimensions {
			p.resume.window.Label = i18n.T("Resume conversation")
			dimensions := p.resume.window.Layout(gtx, th, e.Overlays, labels, selected)
			if index, changed := p.resume.window.Changed(); changed && index >= 0 && index < len(values) {
				set(e, "ask", map[string]any{"resumeMinutes": values[index]})
			}
			return dimensions
		},
	}}
	return spacedBlocks(th, blocks, teamsGroup(th, i18n.T("Pick up where you left off"), "", rows, nil))
}
