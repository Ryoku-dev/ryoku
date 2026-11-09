package term

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Filter is one labelled filter choice.
type Filter struct {
	Label    string
	Count    int
	Disabled bool
}

// Filters describes a filter strip.
type Filters struct {
	Items    []Filter
	Selected int
	Focused  bool
}

// RenderFilters renders filter chips with one selection plane.
func RenderFilters(th Theme, filters Filters) string {
	parts := make([]string, len(filters.Items))
	for i, item := range filters.Items {
		label := item.Label
		if item.Count >= 0 {
			label += " " + strconv.Itoa(item.Count)
		}
		kind := PlaqueQuiet
		if i == filters.Selected {
			kind = PlaqueOn
		}
		parts[i] = RenderPlaque(th, Plaque{Label: label, Kind: kind, Focused: filters.Focused && i == filters.Selected, Disabled: item.Disabled})
	}
	return strings.Join(parts, " ")
}

// FiltersModel is an arrow-key Bubble Tea filter strip.
type FiltersModel struct {
	Theme   Theme
	Filters Filters
	Changed bool
}

func (m FiltersModel) Init() tea.Cmd { return nil }

func (m FiltersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Changed = false
	key, ok := msg.(tea.KeyPressMsg)
	if !ok || len(m.Filters.Items) == 0 {
		return m, nil
	}
	before := m.Filters.Selected
	switch key.String() {
	case "left", "shift+tab":
		m.Filters.Selected--
	case "right", "tab":
		m.Filters.Selected++
	case "home":
		m.Filters.Selected = 0
	case "end":
		m.Filters.Selected = len(m.Filters.Items) - 1
	}
	m.Filters.Selected = clampIndex(m.Filters.Selected, len(m.Filters.Items))
	for m.Filters.Items[m.Filters.Selected].Disabled && m.Filters.Selected != before {
		if m.Filters.Selected > before {
			m.Filters.Selected++
		} else {
			m.Filters.Selected--
		}
		m.Filters.Selected = clampIndex(m.Filters.Selected, len(m.Filters.Items))
	}
	m.Changed = before != m.Filters.Selected
	return m, nil
}

func (m FiltersModel) View() tea.View { return tea.NewView(RenderFilters(m.Theme, m.Filters)) }
