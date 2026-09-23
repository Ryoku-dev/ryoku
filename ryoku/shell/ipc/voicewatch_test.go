package main

import "testing"

// #244: the dictation surface stayed open after Voxtype returned to idle,
// because the only close path was a second tap. The watcher must close on the
// idle transition, but not on the transient idle that immediately follows
// `record start` before Voxtype has actually begun.

func TestVoiceStopOnIdleWaitsForActive(t *testing.T) {
	cases := []struct {
		name      string
		sawActive bool
		class     string
		wantClose bool
		wantAct   bool
	}{
		{"initial idle ignored", false, "idle", false, false},
		{"recording arms", false, "recording", false, true},
		{"transcribing keeps armed", true, "transcribing", false, true},
		{"idle after active closes", true, "idle", true, true},
		{"stopped after active closes", true, "stopped", true, true},
		{"case insensitive", true, "IDLE", true, true},
	}
	for _, tc := range cases {
		closeNow, next := voiceStopOnIdle(tc.sawActive, tc.class)
		if closeNow != tc.wantClose || next != tc.wantAct {
			t.Errorf("%s: voiceStopOnIdle(%v,%q)=(%v,%v), want (%v,%v)",
				tc.name, tc.sawActive, tc.class, closeNow, next, tc.wantClose, tc.wantAct)
		}
	}
}

func TestParseVoxtypeClass(t *testing.T) {
	if c, ok := parseVoxtypeClass(`{"text":"🎙️","class":"recording","alt":"recording"}`); !ok || c != "recording" {
		t.Errorf("class not parsed: %q %v", c, ok)
	}
	// Some builds carry the state only in `alt`.
	if c, ok := parseVoxtypeClass(`{"class":"","alt":"idle"}`); !ok || c != "idle" {
		t.Errorf("alt fallback not parsed: %q %v", c, ok)
	}
	for _, junk := range []string{"", "not json", "[1,2]", `{"text":"x"}`} {
		if _, ok := parseVoxtypeClass(junk); ok {
			t.Errorf("junk accepted: %q", junk)
		}
	}
}
