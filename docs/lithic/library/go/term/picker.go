package term

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Picker describes a labelled menu anchor.
type Picker struct {
	Label    string
	Value    string
	Options  []string
	Selected int
	Open     bool
	Focused  bool
	Disabled bool
	Width    int
}

// RenderPicker renders the anchor and its real menu while open.
func RenderPicker(th Theme, picker Picker) string {
	value := picker.Value
	if value == "" && len(picker.Options) != 0 {
		value = picker.Options[clampIndex(picker.Selected, len(picker.Options))]
	}
	width := picker.Width
	if width <= 0 {
		width = tokens.SizePickerWidthMin / tokens.SpaceS2
	}
	content := fit(picker.Label+": "+value+"  "+glyphs().Chevron, width)
	anchor := framed(th, content, th.Roles.Field, picker.Focused, picker.Disabled)
	if !picker.Open {
		return anchor
	}
	items := make([]MenuItem, len(picker.Options))
	for i, option := range picker.Options {
		items[i] = MenuItem{Label: option}
	}
	return lipgloss.JoinVertical(lipgloss.Left, anchor, RenderMenu(th, Menu{Items: items, Selected: picker.Selected, Focused: true, Width: width}))
}

// PickerModel is a Bubble Tea picker and menu model.
type PickerModel struct {
	Theme   Theme
	Picker  Picker
	Changed bool
}

func (m PickerModel) Init() tea.Cmd { return nil }

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Changed = false
	if m.Picker.Disabled {
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if !m.Picker.Open {
		if key.String() == "enter" || key.String() == "space" {
			m.Picker.Open = true
		}
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		m.Picker.Selected--
	case "down", "j":
		m.Picker.Selected++
	case "enter":
		m.Picker.Selected = clampIndex(m.Picker.Selected, len(m.Picker.Options))
		if len(m.Picker.Options) != 0 {
			m.Picker.Value = m.Picker.Options[m.Picker.Selected]
			m.Changed = true
		}
		m.Picker.Open = false
	case "esc":
		m.Picker.Open = false
	}
	m.Picker.Selected = clampIndex(m.Picker.Selected, len(m.Picker.Options))
	return m, nil
}

func (m PickerModel) View() tea.View { return tea.NewView(RenderPicker(m.Theme, m.Picker)) }
