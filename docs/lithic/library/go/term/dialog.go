package term

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"strings"
)

// Dialog describes a protected terminal decision.
type Dialog struct {
	Title       string
	Body        string
	Actions     []Plaque
	FocusAction int
	Open        bool
	Width       int
}

// RenderDialog renders a stone4 protected plane and its actions.
func RenderDialog(th Theme, dialog Dialog) string {
	if !dialog.Open {
		return ""
	}
	width := dialog.Width
	if width <= 0 {
		width = tokens.SizeDialogWidth / tokens.SpaceS2
	}
	parts := make([]string, 0, 3)
	if dialog.Title != "" {
		parts = append(parts, renderPlaneText(width, th.Roles.Stone4, th.Roles.Fg, dialog.Title, true))
	}
	parts = append(parts, renderPlaneText(width, th.Roles.Stone4, th.Roles.Fg2, dialog.Body, false))
	if len(dialog.Actions) != 0 {
		actions := make([]string, len(dialog.Actions))
		for i, action := range dialog.Actions {
			action.Focused = i == dialog.FocusAction
			actions[i] = RenderPlaque(th, action)
		}
		parts = append(parts, renderPlaneRaw(width, th.Roles.Stone4, strings.Join(actions, "  ")))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return framed(th, content, th.Roles.Stone4, false, false)
}

// DialogModel is a keyboard-contained Bubble Tea dialog.
type DialogModel struct {
	Theme     Theme
	Dialog    Dialog
	Chosen    int
	Cancelled bool
}

func (m DialogModel) Init() tea.Cmd { return nil }

func (m DialogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Chosen, m.Cancelled = -1, false
	if !m.Dialog.Open {
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "left", "shift+tab":
		m.Dialog.FocusAction--
	case "right", "tab":
		m.Dialog.FocusAction++
	case "enter":
		m.Dialog.FocusAction = clampIndex(m.Dialog.FocusAction, len(m.Dialog.Actions))
		if len(m.Dialog.Actions) != 0 && !m.Dialog.Actions[m.Dialog.FocusAction].Disabled {
			m.Chosen = m.Dialog.FocusAction
			m.Dialog.Open = false
		}
	case "esc":
		m.Cancelled = true
		m.Dialog.Open = false
	}
	m.Dialog.FocusAction = clampIndex(m.Dialog.FocusAction, len(m.Dialog.Actions))
	return m, nil
}

func (m DialogModel) View() tea.View { return tea.NewView(RenderDialog(m.Theme, m.Dialog)) }
