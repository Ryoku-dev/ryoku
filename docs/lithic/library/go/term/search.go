package term

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Search describes a terminal search field.
type Search struct {
	Value       string
	Placeholder string
	Shortcut    string
	Focused     bool
	Disabled    bool
	Width       int
}

// RenderSearch renders a searchable field and shortcut or clear action.
func RenderSearch(th Theme, search Search) string {
	width := search.Width
	if width <= 0 {
		width = tokens.SizeSearchWidth / tokens.SpaceS2
	}
	value := search.Value
	if value == "" {
		value = search.Placeholder
	}
	trail := search.Shortcut
	if search.Value != "" {
		trail = "Esc clear"
	}
	gap := 0
	if trail != "" {
		gap = 2
	}
	valueWidth := max(1, width-lipgloss.Width(trail)-gap)
	fitted := fit(value, valueWidth)
	valueColour := th.Roles.Fg
	if search.Value == "" || search.Disabled {
		valueColour = th.Roles.Fg3
	}
	fill := fieldFill(th, th.Roles.Field, search.Focused, search.Disabled)
	content := renderPlaneLine(lipgloss.Width(fitted), fill, planeSpan{text: fitted, ink: valueColour})
	if trail != "" {
		content += renderPlaneLine(gap+lipgloss.Width(trail), fill, planeSpan{text: "  " + trail, ink: th.Roles.Fg3})
	}
	return stone(th, content, fill, nil, search.Disabled)
}

// SearchModel is a keyboard-editable Bubble Tea search model.
type SearchModel struct {
	Theme   Theme
	Search  Search
	Limit   int
	Changed bool
}

// NewSearchModel creates a focused search model.
func NewSearchModel(th Theme, search Search) SearchModel {
	search.Focused = true
	return SearchModel{Theme: th, Search: search}
}

func (m SearchModel) Init() tea.Cmd { return nil }

func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Changed = false
	if m.Search.Disabled {
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if key.String() == "esc" {
		m.Changed = m.Search.Value != ""
		m.Search.Value = ""
		m.Search.Focused = true
		return m, nil
	}
	m.Search.Value, m.Changed = editText(m.Search.Value, key.String(), m.Limit)
	return m, nil
}

func (m SearchModel) View() tea.View { return tea.NewView(RenderSearch(m.Theme, m.Search)) }
