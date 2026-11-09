package term

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Tabs describes up to five resource tabs.
type Tabs struct {
	Labels   []string
	Selected int
	Focused  bool
	Width    int
}

// RenderTabs renders labels and a blade below the selected tab.
func RenderTabs(th Theme, tabs Tabs) string {
	if len(tabs.Labels) == 0 {
		return ""
	}
	selected := clampIndex(tabs.Selected, len(tabs.Labels))
	parts := make([]string, len(tabs.Labels))
	selectedStart, selectedWidth, used := 0, 0, 0
	for i, label := range tabs.Labels {
		if i != 0 {
			used++
		}
		focused := tabs.Focused && i == selected
		colour := th.Roles.Fg3
		if i == selected {
			colour = th.Roles.Fg
		}
		labelPart := lipgloss.NewStyle().
			Foreground(colour).
			Bold(i == selected || focused).
			Render(" " + label + " ")
		part := focusGutter(th, focused) + labelPart
		parts[i] = part
		if i == selected {
			selectedStart = used + 2
			selectedWidth = lipgloss.Width(labelPart)
		}
		used += lipgloss.Width(part)
	}
	strip := strings.Join(parts, " ")
	width := tabs.Width
	if width <= 0 {
		width = lipgloss.Width(strip)
	}
	strip = lipgloss.NewStyle().Width(width).MaxWidth(width).Render(strip)
	before := min(width, selectedStart)
	bladeWidth := min(max(0, width-before), selectedWidth)
	after := max(0, width-before-bladeWidth)
	line := ink(th.Roles.Vein).Render(strings.Repeat(glyphs().BladeTrack, before)) +
		renderBladeColor(th, bladeWidth, 1, th.Roles.Fg) +
		ink(th.Roles.Vein).Render(strings.Repeat(glyphs().BladeTrack, after))
	return strip + "\n" + line
}

// TabsModel is a Left and Right driven Bubble Tea tab strip.
type TabsModel struct {
	Theme   Theme
	Tabs    Tabs
	Changed bool
}

func (m TabsModel) Init() tea.Cmd { return nil }

func (m TabsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Changed = false
	key, ok := msg.(tea.KeyPressMsg)
	if !ok || len(m.Tabs.Labels) == 0 {
		return m, nil
	}
	before := m.Tabs.Selected
	switch key.String() {
	case "left":
		m.Tabs.Selected--
	case "right":
		m.Tabs.Selected++
	case "home":
		m.Tabs.Selected = 0
	case "end":
		m.Tabs.Selected = len(m.Tabs.Labels) - 1
	}
	m.Tabs.Selected = clampIndex(m.Tabs.Selected, len(m.Tabs.Labels))
	m.Changed = before != m.Tabs.Selected
	return m, nil
}

func (m TabsModel) View() tea.View { return tea.NewView(RenderTabs(m.Theme, m.Tabs)) }
