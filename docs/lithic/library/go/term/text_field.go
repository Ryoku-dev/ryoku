package term

import (
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// TextField describes a terminal field and its visual state.
type TextField struct {
	Label       string
	Value       string
	Placeholder string
	Help        string
	Error       string
	Secret      bool
	Reveal      bool
	Focused     bool
	Disabled    bool
	Width       int
}

// RenderTextField renders a field without owning interaction state.
func RenderTextField(th Theme, field TextField) string {
	return renderTextField(th, field, nil, 0)
}

func renderTextFieldOnPlane(th Theme, field TextField, planeBackground color.Color, planeWidth int) string {
	return renderTextField(th, field, planeBackground, planeWidth)
}

func renderTextField(th Theme, field TextField, planeBackground color.Color, planeWidth int) string {
	width := field.Width
	if width <= 0 {
		width = tokens.SizeTextFieldDefaultWidth / tokens.SpaceS2
	}
	value := field.Value
	colour := th.Roles.Fg
	if field.Secret && !field.Reveal {
		value = strings.Repeat(glyphs().SecretMask, len([]rune(value)))
	}
	if value == "" {
		value, colour = field.Placeholder, th.Roles.Fg3
	}
	if field.Disabled {
		colour = th.Roles.Fg3
	}
	body := lipgloss.NewStyle().Foreground(colour).Background(th.Roles.Stone3).Render(fit(value, width))
	if field.Error != "" {
		if planeBackground != nil {
			body = framedOnPlaneWithBorder(th, body, th.Roles.Stone3, planeBackground, th.Roles.Jasper, field.Disabled)
		} else {
			body = framedWithBorder(th, body, th.Roles.Stone3, th.Roles.Jasper, field.Disabled)
		}
	} else if planeBackground != nil {
		body = framedOnPlane(th, body, th.Roles.Stone3, planeBackground, field.Focused, field.Disabled)
	} else {
		body = framed(th, body, th.Roles.Stone3, field.Focused, field.Disabled)
	}
	parts := make([]string, 0, 3)
	appendLine := func(text string, foreground color.Color, raw bool) {
		if planeBackground == nil {
			if raw {
				parts = append(parts, text)
			} else {
				parts = append(parts, ink(foreground).Render(text))
			}
			return
		}
		if raw {
			parts = append(parts, renderPlaneRaw(planeWidth, planeBackground, text))
		} else {
			parts = append(parts, renderPlaneText(planeWidth, planeBackground, foreground, text, false))
		}
	}
	if field.Label != "" {
		appendLine(field.Label, th.Roles.Fg2, false)
	}
	appendLine(body, th.Roles.Fg, true)
	if field.Error != "" {
		appendLine(field.Error, th.Roles.Jasper, false)
	} else if field.Help != "" {
		appendLine(field.Help, th.Roles.Fg3, false)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// TextFieldModel is a keyboard-editable Bubble Tea field.
type TextFieldModel struct {
	Theme     Theme
	Field     TextField
	Limit     int
	Submitted bool
	Changed   bool
}

// NewTextFieldModel creates a focused terminal field model.
func NewTextFieldModel(th Theme, field TextField) TextFieldModel {
	field.Focused = true
	return TextFieldModel{Theme: th, Field: field}
}

func (m TextFieldModel) Init() tea.Cmd { return nil }

func (m TextFieldModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Submitted, m.Changed = false, false
	if m.Field.Disabled {
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if !m.Field.Focused {
		if key.String() == "tab" {
			m.Field.Focused = true
		}
		return m, nil
	}
	switch key.String() {
	case "enter":
		m.Submitted = true
	case "esc":
		m.Field.Focused = false
	case "tab":
		m.Field.Focused = true
	case "ctrl+r":
		if m.Field.Secret {
			m.Field.Reveal = !m.Field.Reveal
		}
	default:
		m.Field.Value, m.Changed = editText(m.Field.Value, key.String(), m.Limit)
	}
	return m, nil
}

func (m TextFieldModel) View() tea.View { return tea.NewView(RenderTextField(m.Theme, m.Field)) }
