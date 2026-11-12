package kit

import (
	"testing"

	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

func TestStepperTyped(t *testing.T) {
	cases := []struct {
		typed           string
		low, high, step int
		want            int
		ok              bool
	}{
		{"37421", 1, 65535, 1, 37421, true},
		{"99999", 1, 65535, 1, 65535, true},
		{"0", 1, 65535, 1, 1, true},
		{"100", 0, 1425, 15, 105, true},
		{"1500", 0, 1425, 15, 1425, true},
		{" 8 ", 0, 10, 1, 8, true},
		{"", 0, 10, 1, 0, false},
		{"-", 0, 10, 1, 0, false},
	}
	for _, test := range cases {
		got, ok := stepperTyped(test.typed, test.low, test.high, test.step)
		if got != test.want || ok != test.ok {
			t.Errorf("stepperTyped(%q, %d, %d, %d) = %d %v, want %d %v", test.typed, test.low, test.high, test.step, got, ok, test.want, test.ok)
		}
	}
}

// stepperFixture lays out an editable port stepper, a stepped minutes
// stepper and a host that counts the Escape presses left to it.
type stepperFixture struct {
	port, minutes Stepper
	escapes       int
	changes       int
	harness       *keyHarness
}

func newStepperFixture(t *testing.T) *stepperFixture {
	th := theme.New(palette.Signature(true))
	f := &stepperFixture{
		port:    Stepper{Value: 5555, Min: 1, Max: 65535, Editable: true},
		minutes: Stepper{Value: 60, Min: 0, Max: 1425, Step: 15, Editable: true},
	}
	f.harness = newKeyHarness(t, func(gtx layout.Context) {
		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.port.Layout(gtx, th) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.minutes.Layout(gtx, th) }),
		)
		if f.port.Changed() {
			f.changes++
		}
		// like a dialog, the host reads Escape after its content
		for {
			event, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				break
			}
			if press, ok := event.(key.Event); ok && press.State == key.Press {
				f.escapes++
			}
		}
	})
	f.harness.settle()
	return f
}

func (f *stepperFixture) focus(s *Stepper) {
	f.harness.frame(func(gtx layout.Context) { s.Focus(gtx) })
	f.harness.settle()
}

// typeOver replaces the field's text, as typing over the selected value does.
func (f *stepperFixture) typeOver(s *Stepper, text string) {
	f.harness.router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: len([]rune(s.editor.Text()))}, Text: text})
	f.harness.settle()
}

func TestStepperTypedValueCommitsOnReturnAndEnter(t *testing.T) {
	f := newStepperFixture(t)
	f.focus(&f.port)
	if !f.harness.focused(&f.port.editor) {
		t.Fatal("an editable stepper did not take focus into its field")
	}
	f.typeOver(&f.port, "37421")
	if f.port.Value != 5555 {
		t.Fatalf("typing alone changed the value to %d", f.port.Value)
	}
	f.harness.press(key.NameReturn)
	if f.port.Value != 37421 || f.changes != 1 {
		t.Fatalf("Return committed %d after %d changes, want 37421 after 1", f.port.Value, f.changes)
	}
	f.typeOver(&f.port, "8080")
	f.harness.press(key.NameEnter)
	if f.port.Value != 8080 {
		t.Fatalf("Enter committed %d, want 8080", f.port.Value)
	}
}

func TestStepperTypedValueCommitsWhenFocusLeaves(t *testing.T) {
	f := newStepperFixture(t)
	f.focus(&f.port)
	f.typeOver(&f.port, "8080")
	f.focus(&f.minutes)
	if f.port.Value != 8080 {
		t.Fatalf("leaving the field committed %d, want 8080", f.port.Value)
	}
}

func TestStepperTypedValueIsClampedAndStepped(t *testing.T) {
	f := newStepperFixture(t)
	f.focus(&f.port)
	f.typeOver(&f.port, "99999")
	f.harness.press(key.NameReturn)
	if f.port.Value != 65535 || f.port.editor.Text() != "65535" {
		t.Fatalf("above the bound committed %d showing %q, want 65535", f.port.Value, f.port.editor.Text())
	}
	f.focus(&f.minutes)
	f.typeOver(&f.minutes, "100")
	f.harness.press(key.NameReturn)
	if f.minutes.Value != 105 {
		t.Fatalf("a typed 100 committed %d, want 105 on the 15 step", f.minutes.Value)
	}
}

func TestStepperEscapeDropsAnEditThenPassesOn(t *testing.T) {
	f := newStepperFixture(t)
	f.focus(&f.port)
	f.typeOver(&f.port, "12")
	f.harness.press(key.NameEscape)
	if f.port.Value != 5555 || f.port.editor.Text() != "5555" || f.escapes != 0 {
		t.Fatalf("Escape on an edit left %d showing %q with %d escapes passed on", f.port.Value, f.port.editor.Text(), f.escapes)
	}
	f.harness.press(key.NameReturn)
	if f.port.Value != 5555 || f.changes != 0 {
		t.Fatalf("the dropped edit was committed later as %d", f.port.Value)
	}
	f.harness.press(key.NameEscape)
	if f.escapes != 1 {
		t.Fatalf("Escape with nothing typed reached the host %d times, want 1", f.escapes)
	}
}

func TestStepperArrowsStillStep(t *testing.T) {
	f := newStepperFixture(t)
	f.focus(&f.port)
	f.harness.press(key.NameUpArrow)
	if f.port.Value != 5556 || f.port.editor.Text() != "5556" {
		t.Fatalf("Up stepped to %d showing %q, want 5556", f.port.Value, f.port.editor.Text())
	}
	f.harness.press(key.NameReturn)
	if f.port.Value != 5556 {
		t.Fatalf("Return after a step committed %d, want 5556", f.port.Value)
	}
	f.focus(&f.minutes)
	f.harness.press(key.NameDownArrow)
	if f.minutes.Value != 45 {
		t.Fatalf("Down stepped minutes to %d, want 45", f.minutes.Value)
	}
}
