package reflex

import (
	"regexp"
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/feature"
)

// No intent may send, buy, pay, delete, submit, post, install, remove,
// erase, format, reboot, shut down or raise privileges, and no intent may
// type into or send keys to another window.
var forbiddenIntent = regexp.MustCompile(`(?i)send|buy|pay|purchase|order|delete|submit|post|install|remove|erase|format|reboot|shutdown|sudo|type|key|chord|close_tab|next_tab|click|password`)

func TestRegistryHoldsNoForbiddenIntent(t *testing.T) {
	for _, in := range Registry() {
		if forbiddenIntent.MatchString(string(in.ID)) {
			t.Errorf("registry holds %q", in.ID)
		}
	}
}

func TestEveryIntentIsReversible(t *testing.T) {
	for _, in := range Registry() {
		if in.Stage != feature.StageReversible {
			t.Errorf("%s stage %q", in.ID, in.Stage)
		}
	}
}

func TestRegistryIdsAreUniqueAndLookedUp(t *testing.T) {
	seen := map[Intent]bool{}
	for _, in := range Registry() {
		if seen[in.ID] {
			t.Errorf("%s twice", in.ID)
		}
		seen[in.ID] = true
		got, ok := Lookup(in.ID)
		if !ok || got != in {
			t.Errorf("Lookup(%s) = %+v, %v", in.ID, got, ok)
		}
	}
	if _, ok := Lookup("close_tab"); ok {
		t.Error("close_tab is in the registry")
	}
}

func TestPressOnlyAndReplayableSets(t *testing.T) {
	var press, replay []Intent
	for _, in := range Registry() {
		if in.PressOnly {
			press = append(press, in.ID)
		}
		if in.Replayable {
			replay = append(replay, in.ID)
		}
		if in.PressOnly && in.Replayable {
			t.Errorf("%s is both Press-only and replayable", in.ID)
		}
	}
	if want := []Intent{RememberNote, RunRoutine, SettingFlip}; !slices.Equal(press, want) {
		t.Errorf("press-only %v, want %v", press, want)
	}
	want := []Intent{OpenApp, OpenSite, WebSearch, NewTab, SnapLeft, SnapRight, Maximize, Minimize, VolumeUp, VolumeDown, Mute, PlayPause, Screenshot}
	if !slices.Equal(replay, want) {
		t.Errorf("replayable %v, want %v", replay, want)
	}
}

func TestRegistryIsACopy(t *testing.T) {
	r := Registry()
	r[0].ID = "changed"
	if Registry()[0].ID == "changed" {
		t.Error("Registry exposes its backing slice")
	}
}

// Every action a rule can plan carries a registry intent.
func TestRulesPlanOnlyRegistryIntents(t *testing.T) {
	for _, c := range parseCases {
		p, _ := Parse(c.say, testNow, nil, testCat)
		for _, a := range p.Actions {
			if _, ok := Lookup(a.Intent); !ok {
				t.Errorf("%q planned %q, not in the registry", c.say, a.Intent)
			}
		}
	}
}

func TestFlipsNeverWiden(t *testing.T) {
	widen := regexp.MustCompile(`^(?:voice|reflex|inspect|guide|decisions|hands|openMic|wake|keep|hints|memory)=on$`)
	for _, f := range Flips() {
		if widen.MatchString(f) {
			t.Errorf("flip %q turns a guarded feature on", f)
		}
	}
	for _, c := range parseCases {
		p, _ := Parse(c.say, testNow, nil, testCat)
		for _, a := range p.Actions {
			if a.Intent == SettingFlip && !slices.Contains(Flips(), a.Arg) {
				t.Errorf("%q planned flip %q outside Flips", c.say, a.Arg)
			}
		}
	}
}
