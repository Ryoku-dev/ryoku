package term

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
)

// Stepper describes a bounded numeric choice.
type Stepper struct {
	Value    int
	Min      int
	Max      int
	Step     int
	Unit     string
	Focused  bool
	Disabled bool
}

// RenderStepper renders bounded decrement and increment plaques.
func RenderStepper(th Theme, stepper Stepper) string {
	step := stepper.Step
	if step == 0 {
		step = 1
	}
	minusDisabled := stepper.Disabled || stepper.Value-step < stepper.Min
	plusDisabled := stepper.Disabled || stepper.Value+step > stepper.Max
	value := strconv.Itoa(stepper.Value)
	if stepper.Unit != "" {
		value += " " + ink(th.Roles.Fg3).Render(stepper.Unit)
	}
	return RenderPlaque(th, Plaque{Label: "-", Disabled: minusDisabled}) + "  " + value + "  " + RenderPlaque(th, Plaque{Label: "+", Disabled: plusDisabled, Focused: stepper.Focused})
}

// StepperModel is a keyboard-controlled Bubble Tea stepper.
type StepperModel struct {
	Theme   Theme
	Stepper Stepper
	Changed bool
}

func (m StepperModel) Init() tea.Cmd { return nil }

func (m StepperModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.Changed = false
	if m.Stepper.Disabled {
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	step := m.Stepper.Step
	if step == 0 {
		step = 1
	}
	next := m.Stepper.Value
	switch key.String() {
	case "left", "down", "-":
		next -= step
	case "right", "up", "+":
		next += step
	}
	next = max(m.Stepper.Min, min(m.Stepper.Max, next))
	m.Changed = next != m.Stepper.Value
	m.Stepper.Value = next
	return m, nil
}

func (m StepperModel) View() tea.View { return tea.NewView(RenderStepper(m.Theme, m.Stepper)) }
