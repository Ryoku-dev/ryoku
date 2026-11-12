package term

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// MenuItemKind selects a menu row treatment.
type MenuItemKind uint8

const (
	MenuItemNormal MenuItemKind = iota
	MenuItemDanger
	MenuItemDivider
)

// MenuItem is one menu row.
type MenuItem struct {
	Label    string
	Shortcut string
	Kind     MenuItemKind
	Disabled bool
}

// Menu describes an open menu.
type Menu struct {
	Items    []MenuItem
	Selected int
	Focused  bool
	Width    int
}

// RenderMenu renders menu rows inside a stone4 cut frame.
func RenderMenu(th Theme, menu Menu) string {
	width := menu.Width
	if width <= 0 {
		width = tokens.SizeMenuWidthMin / tokens.SpaceS2
	}
	rows := make([]string, len(menu.Items))
	for i, item := range menu.Items {
		if item.Kind == MenuItemDivider {
			rule := ink(th.Roles.Vein).Background(th.Roles.Stone4).Render(strings.Repeat(glyphs().Rule, width))
			rows[i] = lipgloss.NewStyle().Width(width).Background(th.Roles.Stone4).Render(rule)
			continue
		}
		focused := menu.Focused && i == menu.Selected && !item.Disabled
		colour := th.Roles.Fg
		if item.Kind == MenuItemDanger {
			colour = th.Roles.Jasper
		}
		if item.Disabled {
			colour = th.Roles.Fg3
		}
		shortcutWidth := lipgloss.Width(item.Shortcut)
		gap := 0
		if item.Shortcut != "" {
			gap = 2
		}
		labelWidth := max(1, width-2-shortcutWidth-gap)
		row := focusGutterOn(th, focused, th.Roles.Stone4) +
			lipgloss.NewStyle().Foreground(colour).Background(th.Roles.Stone4).Bold(focused).Render(fit(item.Label, labelWidth))
		if item.Shortcut != "" {
			row += lipgloss.NewStyle().Foreground(th.Roles.Fg3).Background(th.Roles.Stone4).Render("  " + item.Shortcut)
		}
		rows[i] = lipgloss.NewStyle().Width(width).MaxWidth(width).Background(th.Roles.Stone4).Render(row)
	}
	content := lipgloss.NewStyle().
		Width(width).
		Background(th.Roles.Stone4).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	return framed(th, content, th.Roles.Stone4, false, false)
}

// MenuModel is an arrow-key Bubble Tea menu.
type MenuModel struct {
	Theme     Theme
	Menu      Menu
	Activated int
	Cancelled bool
}

func (m MenuModel) Init() tea.Cmd { return nil }

func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Activated, m.Cancelled = -1, false
	key, ok := msg.(tea.KeyPressMsg)
	if !ok || len(m.Menu.Items) == 0 {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		m.Menu.Selected--
	case "down", "j":
		m.Menu.Selected++
	case "home":
		m.Menu.Selected = 0
	case "end":
		m.Menu.Selected = len(m.Menu.Items) - 1
	case "enter":
		item := m.Menu.Items[clampIndex(m.Menu.Selected, len(m.Menu.Items))]
		if !item.Disabled && item.Kind != MenuItemDivider {
			m.Activated = m.Menu.Selected
		}
	case "esc":
		m.Cancelled = true
	}
	m.Menu.Selected = clampIndex(m.Menu.Selected, len(m.Menu.Items))
	for (m.Menu.Items[m.Menu.Selected].Disabled || m.Menu.Items[m.Menu.Selected].Kind == MenuItemDivider) && len(m.Menu.Items) > 1 {
		if key.String() == "up" || key.String() == "k" {
			m.Menu.Selected = clampIndex(m.Menu.Selected-1, len(m.Menu.Items))
		} else {
			m.Menu.Selected = clampIndex(m.Menu.Selected+1, len(m.Menu.Items))
		}
		break
	}
	return m, nil
}

func (m MenuModel) View() tea.View { return tea.NewView(RenderMenu(m.Theme, m.Menu)) }
