package shell

import (
	"testing"

	"ryoku-rashin/internal/app/look"
)

func TestPanelDocksOnlyWhileContentKeepsItsMinimum(t *testing.T) {
	// rail 72, panel 440, two margins and two gaps of 8, content 480
	const rail = 72 + 440 + 32 + 480
	// the open sidebar is 192 wider than the rail
	const expanded = rail + 192
	tests := []struct {
		name   string
		width  float32
		class  look.Class
		folded bool
		want   bool
	}{
		{"medium below", rail - 1, look.Medium, false, false},
		{"medium at", rail, look.Medium, false, true},
		{"expanded below", expanded - 1, look.Expanded, false, false},
		{"expanded at", expanded, look.Expanded, false, true},
		{"folded uses the rail", rail, look.Expanded, true, true},
		{"compact never docks", 2000, look.Compact, true, false},
	}
	for _, test := range tests {
		if got := panelDockEligible(test.width, test.class, test.folded); got != test.want {
			t.Errorf("%s: docks = %v, want %v", test.name, got, test.want)
		}
	}
}
