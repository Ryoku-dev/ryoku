package palette

import "testing"

func TestTheZeroModeIsDark(t *testing.T) {
	var mode Mode
	if mode != Dark {
		t.Fatalf("zero Mode = %v, want dark", mode)
	}
}

func TestParseMode(t *testing.T) {
	for _, test := range []struct {
		name    string
		want    Mode
		invalid bool
	}{
		{"", Dark, false},
		{"dark", Dark, false},
		{"light", Light, false},
		{"follow", Follow, false},
		{"wallpaper", Dark, true},
	} {
		got, err := ParseMode(test.name)
		if got != test.want || (err != nil) != test.invalid {
			t.Fatalf("ParseMode(%q) = %v, %v; want %v, invalid %v", test.name, got, err, test.want, test.invalid)
		}
		if !test.invalid && test.name != "" && got.String() != test.name {
			t.Fatalf("%v.String() = %q, want %q", got, got.String(), test.name)
		}
	}
}
