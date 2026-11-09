package term

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Toggle describes a terminal on or off choice.
type Toggle struct {
	Label    string
	On       bool
	Focused  bool
	Disabled bool
}

// RenderToggle renders a track, knob, and label.
func RenderToggle(th Theme, toggle Toggle) string {
	gutter := focusGutter(th, toggle.Focused && !toggle.Disabled)
	g := glyphs()
	var track string
	if toggle.On && !toggle.Disabled {
		track = plane(th.Roles.OnSel, th.Roles.Sel).Render("[  " + g.ToggleKnob + "]")
	} else {
		knob := ink(th.Roles.Fg2).Render(g.ToggleKnob)
		if toggle.Disabled {
			knob = ink(th.Roles.Fg3).Render(g.ToggleKnob)
		}
		track = ink(th.Roles.Fg3).Render("[") + knob + ink(th.Roles.Fg3).Render("  ]")
	}
	labelColour := th.Roles.Fg2
	if toggle.Disabled {
		labelColour = th.Roles.Fg3
	}
	label := lipgloss.NewStyle().
		Foreground(labelColour).
		Bold(toggle.Focused && !toggle.Disabled).
		Render(toggle.Label)
	if toggle.Label == "" {
		return gutter + track
	}
	return gutter + track + "  " + label
}

// ToggleModel is a Space-driven Bubble Tea toggle.
type ToggleModel struct {
	Theme   Theme
	Toggle  Toggle
	Changed bool
}

func (m ToggleModel) Init() tea.Cmd { return nil }

func (m ToggleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Changed = false
	if m.Toggle.Disabled {
		return m, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && (key.String() == "space" || key.String() == "enter") {
		m.Toggle.On = !m.Toggle.On
		m.Changed = true
	}
	return m, nil
}

func (m ToggleModel) View() tea.View { return tea.NewView(RenderToggle(m.Theme, m.Toggle)) }
