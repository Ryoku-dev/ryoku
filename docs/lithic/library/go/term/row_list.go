package term

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// RowList describes a keyboard-selectable set of tablets.
type RowList struct {
	Rows     []Tablet
	Selected int
	Focused  bool
}

// RenderRowList renders rows with one selected slab.
func RenderRowList(th Theme, list RowList) string {
	rows := make([]string, len(list.Rows))
	for i, row := range list.Rows {
		row.Selected = i == list.Selected
		row.Focused = list.Focused && i == list.Selected
		rows[i] = RenderTablet(th, row)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// RowListModel is an arrow-key Bubble Tea row list.
type RowListModel struct {
	Theme     Theme
	List      RowList
	Activated int
	Changed   bool
}

func (m RowListModel) Init() tea.Cmd { return nil }

func (m RowListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Changed, m.Activated = false, -1
	key, ok := msg.(tea.KeyPressMsg)
	if !ok || len(m.List.Rows) == 0 {
		return m, nil
	}
	before := m.List.Selected
	switch key.String() {
	case "up", "k", "shift+tab":
		m.List.Selected--
	case "down", "j", "tab":
		m.List.Selected++
	case "home":
		m.List.Selected = 0
	case "end":
		m.List.Selected = len(m.List.Rows) - 1
	case "enter":
		m.Activated = m.List.Selected
	}
	m.List.Selected = clampIndex(m.List.Selected, len(m.List.Rows))
	m.Changed = before != m.List.Selected
	return m, nil
}

func (m RowListModel) View() tea.View { return tea.NewView(RenderRowList(m.Theme, m.List)) }
