package shell

import (
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/io/semantic"
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

// A command an answer proposes is shown in the terminal as if typed, runs
// from the home folder like Ask's own tools, says so when it fails, and
// leaves the user's shell behind it. A slow command in a bare window with
// nothing but a cursor read as "does nothing". What the terminal is handed is
// run here the way the terminal would run it.
func TestAProposedCommandShowsItselfRunsAndLeavesAShell(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1200))
	stage.open("ask/ask-20261110-094200-a17c")
	stage.click("Run in terminal")
	if _, fronted := stage.lastLaunch(); !fronted {
		t.Error("the terminal was not asked to the front")
	}
	argv := stage.argv[len(stage.argv)-1]
	if len(argv) < 5 || strings.Join(argv[:3], " ") != "ryoku-app terminal --" {
		t.Fatalf("the terminal was started as %q", argv)
	}
	home, tools := t.TempDir(), t.TempDir()
	for _, check := range []struct{ stub, want string }{
		{"#!/bin/sh\necho listed \"$@\"\n", "$ rashin index list\nlisted index list\n" + home + "\n"},
		{"#!/bin/sh\nexit 3\n", "$ rashin index list\n[exit 3]\n" + home + "\n"},
	} {
		if err := os.WriteFile(filepath.Join(tools, "rashin"), []byte(check.stub), 0o755); err != nil {
			t.Fatal(err)
		}
		run := exec.Command(argv[3], argv[4:]...)
		// the shell left behind is pwd, so it says where the command ran
		run.Env = []string{"HOME=" + home, "PATH=" + tools + ":/usr/bin:/bin", "SHELL=/bin/pwd"}
		printed, err := run.Output()
		if err != nil {
			t.Fatal(err)
		}
		plain := strings.NewReplacer("\x1b[2m", "", "\x1b[0m", "").Replace(string(printed))
		if plain != check.want {
			t.Errorf("the terminal showed %q, want %q", plain, check.want)
		}
	}
}

// Run on a command an answer offers runs it in the conversation: no terminal
// is opened, the command stands where a question would, its output and the
// answer made from it follow, and a second command waits for the first.
func TestRunPutsAProposedCommandInTheConversation(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1200))
	stage.open("ask/ask-20261110-094200-a17c")
	stage.click("Run")
	stage.await("the command stands in the conversation", func() bool {
		_, shown := stage.named("$ rashin index list")
		return shown
	})
	if len(stage.launched) != 0 {
		t.Fatalf("a terminal was opened: %q", stage.launched)
	}
	stage.click("Run")
	stage.await("the answer made from what it printed", func() bool {
		_, shown := stage.named("The hyperion index is still being built: 438 of 5,790 files are in.")
		return shown
	})
	ran := 0
	for _, name := range stage.names() {
		if name == "$ rashin index list" {
			ran++
		}
	}
	if ran != 1 {
		t.Fatalf("the command was run %d times: a press during the run started another", ran)
	}
	if len(stage.launched) != 0 {
		t.Fatalf("a terminal was opened: %q", stage.launched)
	}
}

// A command that asks for a password has no keyboard in the conversation, so
// its one action stays the terminal. Every other command runs in place, with
// the terminal a small second choice.
func TestOnlyACommandThatNeedsOneIsSentToATerminal(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1200))
	stage.open("ask/ask-20261110-094200-a17c")
	// a plaque's words are a node of their own, so only what can be pressed is counted
	controls := func(name string) int {
		count := 0
		for _, node := range stage.nodes() {
			if node.Desc.Class == semantic.Button && (node.Desc.Label == name || node.Desc.Description == name) {
				count++
			}
		}
		return count
	}
	if here, terminal := controls("Run"), controls("Run in terminal"); here != 3 || terminal != 4 {
		t.Fatalf("%d commands run in place and %d controls open a terminal, want 3 and 4 (one each for those, and the one that needs it)", here, terminal)
	}
}
