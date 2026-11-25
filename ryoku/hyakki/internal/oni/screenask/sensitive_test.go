package screenask

import (
	"math"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
)

func TestSensitiveByLabel(t *testing.T) {
	field := feature.Box{X: 0.4, Y: 0.4, W: 0.3, H: 0.05}
	cases := []struct {
		label   string
		private bool
	}{
		{"Password", true},
		{"New password", true},
		{"confirmNewPassword", true},
		{"user_passcode", true},
		{"Enter PIN", true},
		{"CVV", true},
		{"Card number", true},
		{"Credit card", true},
		{"Security code", true},
		{"Account number", true},
		{"Routing number", true},
		{"SSN", true},
		{"Passport", true},
		{"One-time code", true},
		{"Verification code", true},
		{"Recovery phrase", true},
		{"API key", true},
		{"Mot de passe", true},
		{"Passwort", true},
		{"パスワード", true},
		{"Pinned tabs", false},
		{"Spinner", false},
		{"Opinion poll", false},
		{"Passing grade", false},
		{"Account settings", false},
		{"Add card", false},
		{"Search", false},
		{"Save", false},
		{"", false},
	}
	for _, c := range cases {
		reason, refused := Sensitive(c.label, nil, field)
		if refused != c.private {
			t.Errorf("Sensitive(%q) = %v (%s), want %v", c.label, refused, reason, c.private)
		}
		if refused && reason == "" {
			t.Errorf("Sensitive(%q) refused without a reason", c.label)
		}
	}
}

func TestSensitiveByTextBeside(t *testing.T) {
	field := feature.Box{X: 0.4, Y: 0.4, W: 0.3, H: 0.05}
	cases := []struct {
		name    string
		words   []Word
		private bool
	}{
		{"caption to the left", []Word{{"Password", feature.Box{X: 0.25, Y: 0.41, W: 0.1, H: 0.03}}}, true},
		{"caption above", []Word{{"PIN", feature.Box{X: 0.4, Y: 0.36, W: 0.04, H: 0.03}}}, true},
		{"placeholder inside", []Word{{"Card", feature.Box{X: 0.42, Y: 0.41, W: 0.05, H: 0.03}}, {"number", feature.Box{X: 0.48, Y: 0.411, W: 0.06, H: 0.03}}}, true},
		{"phrase split and listed out of order", []Word{{"number", feature.Box{X: 0.3, Y: 0.412, W: 0.06, H: 0.03}}, {"Account", feature.Box{X: 0.21, Y: 0.41, W: 0.08, H: 0.03}}}, true},
		{"caption far below", []Word{{"Password", feature.Box{X: 0.4, Y: 0.8, W: 0.1, H: 0.03}}}, false},
		{"caption far right", []Word{{"Password", feature.Box{X: 0.9, Y: 0.41, W: 0.08, H: 0.03}}}, false},
		{"harmless caption", []Word{{"Username", feature.Box{X: 0.25, Y: 0.41, W: 0.1, H: 0.03}}}, false},
		{"words on two rows do not join", []Word{{"Card", feature.Box{X: 0.3, Y: 0.36, W: 0.05, H: 0.02}}, {"number", feature.Box{X: 0.36, Y: 0.415, W: 0.06, H: 0.02}}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, refused := Sensitive("", c.words, field)
			if refused != c.private {
				t.Fatalf("refused %v (%s), want %v", refused, reason, c.private)
			}
		})
	}
}

// A point replayed without a model (a recorded pointing shown again) goes
// through the same refusal as a fresh reply.
func TestSensitiveRefusesReplayedPoints(t *testing.T) {
	words := []Word{{"Card", feature.Box{X: 0.15, Y: 0.612, W: 0.05, H: 0.03}}, {"number", feature.Box{X: 0.215, Y: 0.611, W: 0.06, H: 0.03}}}
	replays := []struct {
		name    string
		p       Pointing
		private bool
	}{
		{"recorded element named private", Pointing{Kind: KindElement, ID: "2", Label: "Password", Box: &feature.Box{X: 0.3, Y: 0.4, W: 0.3, H: 0.05}}, true},
		{"recorded box beside a private caption", Pointing{Kind: KindBox, Label: "", Box: &feature.Box{X: 0.3, Y: 0.6, W: 0.3, H: 0.05}}, true},
		{"recorded box at a plain button", Pointing{Kind: KindBox, Label: "Export", Box: &feature.Box{X: 0.69, Y: 0.09, W: 0.09, H: 0.05}}, false},
	}
	for _, r := range replays {
		t.Run(r.name, func(t *testing.T) {
			_, refused := Sensitive(r.p.Label, words, *r.p.Box)
			if refused != r.private {
				t.Fatalf("refused %v, want %v", refused, r.private)
			}
		})
	}
}

func TestDwell(t *testing.T) {
	cases := []struct {
		words int
		want  time.Duration
	}{
		{-5, 10 * time.Second},
		{0, 10 * time.Second},
		{10, 10 * time.Second},
		{11, 10*time.Second + 400*time.Millisecond},
		{20, 14 * time.Second},
		{135, 60 * time.Second},
		{136, 60 * time.Second},
		{math.MaxInt, 60 * time.Second},
	}
	for _, c := range cases {
		if got := Dwell(c.words); got != c.want {
			t.Errorf("Dwell(%d) = %v, want %v", c.words, got, c.want)
		}
	}
}
