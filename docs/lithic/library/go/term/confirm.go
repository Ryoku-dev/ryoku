package term

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Confirm describes a protected named action.
type Confirm struct {
	Title        string
	Body         string
	Verb         string
	Kind         PlaqueKind
	RequiredName string
	TypedName    string
	FocusField   bool
	Open         bool
	Width        int
}

// RenderConfirm composes a dialog, optional name field, and named action.
func RenderConfirm(th Theme, confirm Confirm) string {
	if !confirm.Open {
		return ""
	}
	enabled := confirm.RequiredName == "" || confirm.TypedName == confirm.RequiredName
	dialogWidth := confirm.Width
	if dialogWidth <= 0 {
		dialogWidth = tokens.SizeDialogWidth / tokens.SpaceS2
	}
	body := confirm.Body
	if confirm.RequiredName != "" {
		fieldWidth := max(tokens.SizeTextFieldDefaultWidth/tokens.SpaceS2, dialogWidth-horizontalPadding(tokens.SpaceS4)*2)
		field := renderTextFieldOnPlane(th, TextField{Label: "Type " + confirm.RequiredName + " to continue", Value: confirm.TypedName, Focused: confirm.FocusField, Width: fieldWidth}, th.Roles.Stone4, dialogWidth)
		body = lipgloss.JoinVertical(lipgloss.Left, body, field)
	}
	focusAction := 1
	if confirm.FocusField {
		focusAction = -1
	}
	dialog := Dialog{Title: confirm.Title, Body: body, Open: true, Width: dialogWidth, FocusAction: focusAction, Actions: []Plaque{{Label: "Cancel", Kind: PlaqueQuiet}, {Label: confirm.Verb, Kind: confirm.Kind, Disabled: !enabled}}}
	return RenderDialog(th, dialog)
}

// ConfirmModel is a Bubble Tea typed-name confirmation model.
type ConfirmModel struct {
	Theme     Theme
	Confirm   Confirm
	Accepted  bool
	Cancelled bool
}

func (m ConfirmModel) Init() tea.Cmd { return nil }

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Accepted, m.Cancelled = false, false
	if !m.Confirm.Open {
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.Cancelled, m.Confirm.Open = true, false
	case "tab", "shift+tab":
		m.Confirm.FocusField = !m.Confirm.FocusField
	case "enter":
		if !m.Confirm.FocusField && (m.Confirm.RequiredName == "" || m.Confirm.TypedName == m.Confirm.RequiredName) {
			m.Accepted, m.Confirm.Open = true, false
		}
	default:
		if m.Confirm.FocusField {
			m.Confirm.TypedName, _ = editText(m.Confirm.TypedName, key.String(), 0)
		}
	}
	return m, nil
}

func (m ConfirmModel) View() tea.View { return tea.NewView(RenderConfirm(m.Theme, m.Confirm)) }
