package term

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Disclosure describes optional content controlled by a labelled plaque.
type Disclosure struct {
	Label    string
	Content  string
	Open     bool
	Focused  bool
	Disabled bool
}

// RenderDisclosure renders the control and only includes content while open.
func RenderDisclosure(th Theme, disclosure Disclosure) string {
	label := disclosure.Label
	if label == "" {
		if disclosure.Open {
			label = "Hide details"
		} else {
			label = "Show details"
		}
	}
	control := RenderPlaque(th, Plaque{Label: label, Kind: PlaqueQuiet, Focused: disclosure.Focused, Disabled: disclosure.Disabled})
	if !disclosure.Open {
		return control
	}
	return lipgloss.JoinVertical(lipgloss.Left, control, ink(th.Roles.Fg2).Render(disclosure.Content))
}

// DisclosureModel is a Bubble Tea open and close model.
type DisclosureModel struct {
	Theme      Theme
	Disclosure Disclosure
	Changed    bool
}

func (m DisclosureModel) Init() tea.Cmd { return nil }

func (m DisclosureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Changed = false
	if m.Disclosure.Disabled {
		return m, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && (key.String() == "space" || key.String() == "enter") {
		m.Disclosure.Open = !m.Disclosure.Open
		m.Changed = true
	}
	return m, nil
}

func (m DisclosureModel) View() tea.View {
	return tea.NewView(RenderDisclosure(m.Theme, m.Disclosure))
}
