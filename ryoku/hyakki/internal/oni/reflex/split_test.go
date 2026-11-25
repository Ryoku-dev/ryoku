package reflex

import (
	"slices"
	"testing"
)

func TestSplit(t *testing.T) {
	cases := []struct {
		say  string
		want []string
	}{
		{"search for salt and pepper", []string{"search for salt and pepper"}},
		{"open firefox and turn it up", []string{"open firefox", "turn it up"}},
		{"open firefox and then mute", []string{"open firefox", "mute"}},
		{"open firefox, then take a screenshot", []string{"open firefox", "take a screenshot"}},
		{"open firefox open files", []string{"open firefox", "open files"}},
		{"Open firefox. Mute.", []string{"Open firefox", "Mute"}},
		{"remember that I need to open the door and lock it", []string{"remember that I need to open the door and lock it"}},
		{"remember that the car is on level 3. Open firefox.", []string{"remember that the car is on level 3", "Open firefox"}},
		{"remind me at 5 p.m. tomorrow to call", []string{"remind me at 5 p.m. tomorrow to call"}},
		{"look up how to open a jar", []string{"look up how to open a jar"}},
		{"search for cats and open firefox", []string{"search for cats", "open firefox"}},
		{"play pause", []string{"play pause"}},
		{"cancel timers and reminders", []string{"cancel timers and reminders"}},
		{"open firefox and", []string{"open firefox"}},
		{"turn it up turn it up", []string{"turn it up", "turn it up"}},
		{"open firefox and launch", []string{"open firefox", "launch"}},
		{"I insert a table", []string{"I insert a table"}},
		{"", nil},
	}
	for _, c := range cases {
		if got := Split(c.say); !slices.Equal(got, c.want) {
			t.Errorf("Split(%q) = %q, want %q", c.say, got, c.want)
		}
	}
}
