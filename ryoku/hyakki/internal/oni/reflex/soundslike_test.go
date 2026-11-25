package reflex

import "testing"

func TestSoundslike(t *testing.T) {
	cases := []struct {
		a, b     string
		min, max float64
	}{
		{"Fire Fox", "firefox", 1, 1},
		{"fire-fox", "Firefox", 1, 1},
		{"fone", "phone", 1, 1},            // ph said as f
		{"kalkulator", "calculator", 1, 1}, // hard c said as k
		{"firefx", "firefox", 0.8, 0.99},   // one letter lost
		{"terminal", "calculator", 0, 0.5}, // unlike
		{"", "firefox", 0, 0},
	}
	for _, c := range cases {
		got := Soundslike(c.a, c.b)
		if got < c.min || got > c.max {
			t.Errorf("Soundslike(%q, %q) = %v, want %v..%v", c.a, c.b, got, c.min, c.max)
		}
		if back := Soundslike(c.b, c.a); back != got {
			t.Errorf("Soundslike is not symmetric for %q, %q", c.a, c.b)
		}
	}
}

func TestBest(t *testing.T) {
	lim := limits()
	cases := []struct {
		say   string
		names []string
		want  int // -1 for no pick
	}{
		{"firefox", []string{"Files", "Firefox"}, 1},            // exact
		{"firefx", []string{"Files", "Firefox", "Terminal"}, 1}, // close enough, clear margin
		{"qwzx", []string{"Files", "Firefox"}, -1},              // under fuzzyMin
		{"codes", []string{"Code", "Coded"}, -1},                // two families within fuzzyMargin
		{"codes", []string{"Code", "Code OSS"}, 1},              // one family never crowds itself out; the higher score wins
		{"files", []string{"Files Viewer", "Files"}, 1},         // exact wins
		{"file", []string{"Files Viewer", "Files"}, 1},          // family: the higher score
		{"anything", nil, -1},
	}
	for _, c := range cases {
		i, score, ok := Best(c.say, c.names)
		if c.want < 0 {
			if ok {
				t.Errorf("Best(%q, %q) picked %d (%v)", c.say, c.names, i, score)
			}
			continue
		}
		if !ok || i != c.want {
			t.Errorf("Best(%q, %q) = %d, %v, %v; want %d", c.say, c.names, i, score, ok, c.want)
		}
		if score < lim.FuzzyMin {
			t.Errorf("Best(%q) picked under fuzzyMin: %v", c.say, score)
		}
	}
	// The margin case really sits inside the margin and over the floor.
	a, b := Soundslike("codes", "Code"), Soundslike("codes", "Coded")
	if a < lim.FuzzyMin || a-b >= lim.FuzzyMargin || b-a >= lim.FuzzyMargin {
		t.Fatalf("margin fixture does not test the margin: %v %v", a, b)
	}
	if s := Soundslike("codes", "Code OSS"); a-s >= lim.FuzzyMargin {
		t.Fatalf("family fixture does not test the family rule: %v %v", a, s)
	}
}
