package shell

import (
	"os"
	"path/filepath"
	"testing"
)

func writeStory(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "motion.story")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStoryboardReadsEventsInOrderAndKeepsTypedTextWhole(t *testing.T) {
	events, err := readStoryboard(writeStory(t, "# a comment\n0 route ask/new\n\n200 key ctrl+k\n240 type hyperion project\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].command != "route" || events[1].at != 200 || events[2].args[0] != "hyperion project" {
		t.Fatalf("events = %#v", events)
	}
}

func TestStoryboardClickIsAPressThenAReleaseInTheirOwnFrames(t *testing.T) {
	events, err := readStoryboard(writeStory(t, "100 click 20 30\n200 secondary 40 50\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"press", "release", "press-secondary", "release"}
	if len(events) != len(want) {
		t.Fatalf("events = %#v", events)
	}
	for index, command := range want {
		if events[index].command != command {
			t.Fatalf("event %d = %q, want %q", index, events[index].command, command)
		}
	}
	if events[1].args[0] != "20" || events[3].args[1] != "50" {
		t.Fatalf("a release lost where it happens: %#v", events)
	}
}

func TestStoryboardRefusesTimeThatRunsBackwards(t *testing.T) {
	if _, err := readStoryboard(writeStory(t, "200 key ctrl+k\n100 key escape\n")); err == nil {
		t.Fatal("a story whose time runs backwards was accepted")
	}
}
