package shell

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/protocol"
)

// the demo daemon's Hands journal now
func (s *stage) handsTasks() []protocol.OniHandsTaskRow {
	s.t.Helper()
	var result protocol.OniHandsResult
	if err := s.runtime.client.Call(context.Background(), protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsTasks}, &result); err != nil {
		s.t.Fatal(err)
	}
	return result.Tasks
}

// The three stages are exclusive switches: turning one on stores its stage
// alone, so choosing one window never turns the separate desktop on, and
// turning the stage that is on off stores off. Each stage shows its own
// groups and no other's.
func TestOniHandsStagesAreExclusiveSwitches(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOni)
	for _, want := range []string{"Hands in a browser", "Hands in one app window", "Hands on a separate desktop", "Hands never drives your whole desktop."} {
		if !stage.mentions(want) {
			t.Fatalf("the page does not say %q; on screen: %v", want, stage.names())
		}
	}
	stage.toggle("Hands in one app window")
	stage.await("the app stage is stored", func() bool { return stage.daemonOni().Hands == protocol.OniHandsApp })
	stage.await("the window picker shows", func() bool { return stage.shows("Pick a window") })
	if stage.shows("The separate desktop") || stage.mentions("Browser: ") || stage.shows("Authorize a tab") {
		t.Fatalf("the app stage shows another stage's groups: %v", stage.names())
	}
	if oni := stage.daemonOni(); !oni.Inspect || !oni.Master || oni.Voice {
		t.Fatalf("the switch reached settings it does not own: %+v", oni)
	}

	stage.toggle("Hands on a separate desktop")
	stage.await("the desktop stage replaces the app stage", func() bool { return stage.daemonOni().Hands == protocol.OniHandsDesktop })
	stage.await("the separate desktop says it can run", func() bool { return stage.shows("weston, niri and grim found") })
	if stage.shows("Pick a window") {
		t.Fatal("the desktop stage still shows the window picker")
	}

	stage.toggle("Hands in a browser")
	stage.await("the browser stage replaces the desktop stage", func() bool { return stage.daemonOni().Hands == protocol.OniHandsBrowser })
	stage.await("the browser is found", func() bool { return stage.shows("Browser: chromium") })
	stage.toggle("Hands in a browser")
	stage.await("turning the stage that is on off stores off", func() bool { return stage.daemonOni().Hands == protocol.OniHandsOff })
}

// At the app stage Choose a window lists the windows by app id with their
// titles, a tiled one marked keys only; Authorize this window binds one; a
// goal starts a task whose first step asks first; Stop ends it; and the
// journal lists codes only, never the goal or a title.
func TestOniHandsAppPickStartStopAndJournal(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOniHandsApp)
	stage.await("no window is authorized", func() bool { return stage.mentions("No window is authorized yet.") })
	if status := stage.handsStatus(); status.State != protocol.OniHandsStateOpen {
		t.Fatalf("the app stage starts at %+v", status)
	}
	stage.click("Choose a window")
	stage.await("the windows are listed", func() bool { return stage.shows("org.gnome.TextEditor") && stage.shows("org.mozilla.firefox") })
	if !stage.mentions("keys only · Example Domain") || stage.mentions("keys only · Notes") {
		t.Fatalf("the tiled window is not the keys-only one: %v", stage.names())
	}
	stage.click("Authorize the window of org.gnome.TextEditor")
	stage.await("the window is authorized", func() bool { return stage.mentions("Authorized: a window of org.gnome.TextEditor.") })

	goal := "write the shopping list Ada asked for"
	stage.field("Goal")
	stage.typeText(goal)
	stage.click("Start Hands")
	stage.await("the task works", func() bool { return stage.shows("Working · step 1 of 25") })
	stage.await("the first step asks first", func() bool { return stage.shows("Send this step?") })
	if !stage.mentions("In the window of org.gnome.TextEditor.") || stage.mentions("Ada asked") {
		t.Fatalf("the running task does not say where it works, or keeps the goal: %v", stage.names())
	}
	stage.click("Allow this step")
	stage.await("the card goes once answered", func() bool { return !stage.shows("Send this step?") })
	stage.click("Stop Hands")
	stage.await("the task stops", func() bool { return stage.shows("Give Hands a goal") })
	if status := stage.handsStatus(); status.State != protocol.OniHandsStateAuthorized || status.App != "org.gnome.TextEditor" {
		t.Fatalf("stop left %+v", status)
	}
	stage.await("the journal reads the stopped task", func() bool { return stage.shows("Stopped") && stage.shows("Interrupted") })

	row := regexp.MustCompile(`^(in [A-Za-z0-9.]+|on the separate desktop) · step \d+ · `)
	rows := map[string]bool{}
	for _, n := range stage.names() {
		if !row.MatchString(n) {
			continue
		}
		rows[n] = true
		for _, word := range []string{"Ada", "shopping", "Notes", "Example Domain"} {
			if strings.Contains(n, word) {
				t.Fatalf("a journal row holds %q: %q", word, n)
			}
		}
	}
	if len(rows) != 4 || !stage.mentions("Hyakki restarted") {
		t.Fatalf("the journal shows %d rows: %v", len(rows), stage.names())
	}
}

// A task a restart interrupted starts again only from its button, and only
// with the goal said again.
func TestOniHandsInterruptedTaskStartsAgainOnlyWithItsGoal(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOniHandsApp)
	stage.await("the interrupted task is listed", func() bool { return stage.shows("Interrupted") })
	if !stage.mentions("Nothing starts by itself.") {
		t.Fatalf("the row does not say it waits for the button: %v", stage.names())
	}
	stage.click("Start this task again")
	if status := stage.handsStatus(); status.State != protocol.OniHandsStateOpen || status.Task != "" {
		t.Fatalf("Start again with no goal started %+v", status)
	}
	var interrupted string
	for _, task := range stage.handsTasks() {
		if task.State == "interrupted" {
			interrupted = task.ID
		}
	}
	stage.click("Choose a window")
	stage.await("the windows are listed", func() bool { return stage.shows("org.gnome.TextEditor") })
	stage.click("Authorize the window of org.gnome.TextEditor")
	stage.await("the window is authorized", func() bool { return stage.mentions("Authorized: a window of org.gnome.TextEditor.") })
	stage.field("Goal")
	stage.typeText("finish the list")
	stage.click("Start this task again")
	stage.await("the interrupted task works again", func() bool { return stage.handsStatus().Task == interrupted })
}

// A task paused because the person pressed a key says so, and Resume goes
// on from its step.
func TestOniHandsAppPausedByAKeyResumes(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOniHandsAppPaused)
	stage.await("the pause is shown", func() bool { return stage.shows("Paused · step 4 of 25") })
	if !stage.mentions("You pressed a key.") || !stage.mentions("Any key you press pauses it") {
		t.Fatalf("the pause does not say why: %v", stage.names())
	}
	stage.click("Resume Hands")
	stage.await("the task works again", func() bool { return stage.shows("Working · step 4 of 25") })
}

// Without weston and niri the separate desktop says what is missing, and
// Start starts nothing.
func TestOniHandsDesktopSaysWhatIsMissing(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOniHandsNoNest)
	stage.await("the missing programs are named", func() bool { return stage.shows("Needs weston and niri") })
	if !stage.mentions("never uses yours instead") {
		t.Fatalf("the page does not say it never falls back: %v", stage.names())
	}
	stage.field("Goal")
	stage.typeText("tidy the downloads folder")
	if node, ok := stage.named("Start Hands"); ok {
		stage.clickAt(centre(node.Desc.Bounds))
		stage.settle()
	}
	if status := stage.handsStatus(); status.State != protocol.OniHandsStateClosed || status.Task != "" {
		t.Fatalf("Start without the desktop left %+v", status)
	}
}

// Delete all asks first; Cancel keeps the journal and Delete them empties it.
func TestOniHandsDeleteAllAsksFirst(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOniHandsDesktop)
	stage.await("the journal is read", func() bool { return stage.shows("Interrupted") })
	stage.click("Delete every task in the journal")
	stage.await("the question opens", func() bool { return stage.shows("Delete every task?") })
	stage.click("Cancel")
	stage.await("the question closes", func() bool { return !stage.shows("Delete every task?") })
	if len(stage.handsTasks()) != 3 {
		t.Fatal("Cancel deleted the journal")
	}
	stage.click("Delete every task in the journal")
	stage.await("the question opens again", func() bool { return stage.shows("Delete every task?") })
	stage.click("Delete them")
	stage.await("the journal is empty", func() bool { return stage.shows("No task yet") })
	if len(stage.handsTasks()) != 0 {
		t.Fatal("Delete them left tasks")
	}
}
