package shell

import (
	"image"
	"strings"
	"testing"
)

// lastLaunch is what the app last asked the desktop to start, and whether it
// asked for the window to arrive in front of a floating Rashin.
func (s *stage) lastLaunch() (string, bool) {
	s.t.Helper()
	if len(s.launched) == 0 {
		s.t.Fatal("nothing was launched")
	}
	return s.launched[len(s.launched)-1], s.fronted[len(s.fronted)-1]
}

// A floating Rashin covers whatever it opens unless the window is asked to
// the front: a terminal that took the keyboard out of sight read as "does
// nothing". Both go through the user's own app for the role.
func TestATerminalAndAFolderAreAskedToTheFront(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("projects/hyperion/overview")

	stage.click("Open a terminal here")
	command, fronted := stage.lastLaunch()
	if !strings.HasPrefix(command, "ryoku-app terminal -- sh -c ") || !strings.HasSuffix(command, " sh /home/nero/Work/hyperion") {
		t.Errorf("the terminal was started as %q", command)
	}
	if !fronted {
		t.Error("the terminal was not asked to the front")
	}

	stage.click("Open the folder")
	command, fronted = stage.lastLaunch()
	if command != "ryoku-app files -- /home/nero/Work/hyperion" {
		t.Errorf("the folder was opened as %q", command)
	}
	if !fronted {
		t.Error("the file manager was not asked to the front")
	}
}

// A command an answer proposes runs in the user's terminal, from the home
// folder like Ask's own tools, and leaves a shell behind it, whatever shell
// that is, so its output stays to be read.
func TestAProposedCommandRunsInATerminalThatStays(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1200))
	stage.open("ask/ask-20261110-094200-a17c")
	stage.click("Run in terminal")
	command, fronted := stage.lastLaunch()
	if !strings.HasPrefix(command, "ryoku-app terminal -- sh -c cd; rashin index list; exec ") || !strings.Contains(command, "${SHELL:-/bin/sh}") {
		t.Errorf("the command was run as %q", command)
	}
	if !fronted {
		t.Error("the terminal was not asked to the front")
	}
}
