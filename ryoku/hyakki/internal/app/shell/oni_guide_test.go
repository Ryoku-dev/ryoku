package shell

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/protocol"
)

// newGuideStage navigates directly to the screen sub-page. Its test viewport
// is bounded; later groups are reached with normal wheel input.
func newGuideStage(t *testing.T, session string) *stage {
	t.Helper()
	stage := newStage(t, image.Pt(1440, 2200), Options{Route: protocol.TabSettings, SettingsSection: "oni-screen", Session: session})
	stage.open("settings/oni-screen")
	stage.settingsReveal("Show me how, step by step")
	stage.await("the guide switch is drawn", func() bool { return stage.mentions("Show me how, step by step") })
	return stage
}

// kept reads the demo's keeps file, the one Settings lists
func (s *stage) kept() []keep.Entry {
	s.t.Helper()
	entries, err := keep.NewStore(s.runtime.demoKeeps).List()
	if err != nil {
		s.t.Fatal(err)
	}
	return entries
}

// otherClient is a second connection to the demo, as the CLI or the
// presenter would make: what it starts, the app only hears
func (s *stage) otherClient() *client.Conn {
	s.t.Helper()
	conn, err := client.Dial(context.Background(), client.Options{Client: protocol.ClientCLI, Socket: filepath.Join(s.runtime.tempDir, "oni.sock")})
	if err != nil {
		s.t.Fatal(err)
	}
	s.t.Cleanup(func() { conn.Close() })
	return conn
}

func guideAction(t *testing.T, conn *client.Conn, action string) protocol.OniAskResult {
	t.Helper()
	params := protocol.OniGuideParams{Action: action}
	if action == protocol.OniGuideStart {
		params.Goal = "save this file"
	}
	var result protocol.OniAskResult
	if err := conn.Call(context.Background(), protocol.MOniGuide, params, &result); err != nil {
		t.Fatalf("guide %s: %v", action, err)
	}
	return result
}

// drawFor draws frames for a while, long enough for frames from the demo
// to arrive if anything were listening
func (s *stage) drawFor(d time.Duration) {
	for end := time.Now().Add(d); time.Now().Before(end); {
		time.Sleep(2 * time.Millisecond)
		s.frame()
	}
	s.settle()
}

// Each switch writes its own setting, and each says what it needs while
// that is off.
func TestOniGuideSwitchesPatchSettingsWithTheirDependencies(t *testing.T) {
	stage := newGuideStage(t, demo.FeatureOni)
	if g := stage.daemonOni().Guide; g.Enabled || g.Keep || g.Hints {
		t.Fatalf("guides start on: %+v", g)
	}
	for _, want := range []string{"Needs Show me how, step by step", "Needs Offer to keep a walkthrough", "Turn on Show me how, step by step first."} {
		if !stage.mentions(want) {
			t.Fatalf("the off switches do not say %q; on screen: %v", want, stage.names())
		}
	}
	stage.toggle("Show me how, step by step")
	stage.await("guides turn on", func() bool { return stage.daemonOni().Guide.Enabled })
	stage.toggle("Offer to keep a walkthrough")
	stage.await("Keep turns on", func() bool { return stage.daemonOni().Guide.Keep })
	stage.toggle("Hints from kept walkthroughs")
	stage.await("hints turn on", func() bool { return stage.daemonOni().Guide.Hints })
	if stage.mentions("Needs Show me how") || stage.mentions("Needs Offer to keep") {
		t.Fatal("a dependency that is on is still said to be needed")
	}
	if oni := stage.daemonOni(); !oni.Inspect || oni.Voice || oni.Disclosure != protocol.OniDisclosureSession {
		t.Fatalf("the guide switches reached settings they do not own: %+v", oni)
	}
	if _, ok := stage.editorNamed("Goal"); !ok {
		t.Fatalf("with guides on there is no goal to start; on screen: %v", stage.names())
	}

	stage.toggle("Let Oni look at a window")
	stage.await("looking turns off", func() bool { return !stage.daemonOni().Inspect })
	if !stage.mentions("Needs Let Oni look at a window") || !stage.mentions("Turn on Let Oni look at a window first.") {
		t.Fatalf("with looking off the guide does not say what it needs; on screen: %v", stage.names())
	}
}

// The kept walkthroughs are listed by goal and app, never a picture or the
// window's title; Forget drops one and Forget all every one.
func TestOniKeptWalkthroughsListAndForget(t *testing.T) {
	stage := newGuideStage(t, demo.FeatureOniGuide)
	stage.settingsReveal("clear the scrollback")
	stage.await("the keeps are read", func() bool { return stage.shows("clear the scrollback") })
	for _, want := range []string{"print the packing list", "turn on spell checking", "clear the scrollback"} {
		if !stage.shows(want) {
			t.Fatalf("%q is not listed; on screen: %v", want, stage.names())
		}
	}
	if !stage.showsPrefix("In org.example.Notes · 3 steps · kept ") || !stage.showsPrefix("In org.example.Terminal · 2 steps · kept ") {
		t.Fatalf("a keep does not say its app and steps; on screen: %v", stage.names())
	}
	// the notes window is called Packing list; no keep holds or shows it
	if stage.mentions("Packing list") {
		t.Fatalf("a window title is shown: %v", stage.names())
	}

	stage.click("Forget “clear the scrollback”")
	stage.await("the terminal keep is forgotten", func() bool { return !stage.shows("clear the scrollback") })
	if entries := stage.kept(); len(entries) != 2 {
		t.Fatalf("forgetting one left %d", len(entries))
	}
	stage.click("Forget all")
	stage.await("every keep is forgotten", func() bool { return stage.shows("Nothing kept yet") })
	if entries := stage.kept(); len(entries) != 0 {
		t.Fatalf("forgetting all left %d", len(entries))
	}
}

// A record of hints offered that cannot be read is said in Kept
// walkthroughs with Start over, which moves it aside and leaves the keeps
// listed; Forget all clears the record with the keeps.
func TestOniKeptWalkthroughsStartTheHintRecordOver(t *testing.T) {
	stage := newGuideStage(t, demo.FeatureOniGuide)
	stage.settingsReveal("clear the scrollback")
	stage.await("the keeps are read", func() bool { return stage.shows("clear the scrollback") })
	hints := keep.NewStore(stage.runtime.demoKeeps).Hints()
	if err := os.WriteFile(hints.Path(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	stage.click("Forget “clear the scrollback”")
	stage.await("the damaged hint record is said", func() bool { return stage.mentions("hint record could not be read") })
	if !stage.shows("print the packing list") {
		t.Fatalf("the keeps went with the hint record's damage; on screen: %v", stage.names())
	}
	stage.click("Start over")
	stage.await("the hint record starts over", func() bool { return !stage.mentions("hint record could not be read") })
	if raw, err := os.ReadFile(hints.DamagedPath()); err != nil || string(raw) != "{not json" {
		t.Fatalf("the damaged record was not kept aside: %q, %v", raw, err)
	}
	if err := hints.Check(); err != nil {
		t.Fatalf("after Start over the record = %v", err)
	}
	if entries := stage.kept(); len(entries) != 2 {
		t.Fatalf("Start over over the hint record left %d keeps", len(entries))
	}

	key := keep.KeyFor("org.example.Notes", "print the packing list")
	if got, err := hints.Offer("org.example.Notes", []string{key}, time.Now()); err != nil || got != key {
		t.Fatalf("offer = %q, %v", got, err)
	}
	stage.click("Forget all")
	stage.await("every keep is forgotten", func() bool { return stage.shows("Nothing kept yet") })
	if _, err := os.Stat(hints.Path()); !os.IsNotExist(err) {
		t.Fatalf("the hint record outlived Forget all: %v", err)
	}
}

// A guide started here asks before its picture, shows each step with its
// checklist and Next, and at the end asks whether to keep it; Keep it
// writes it to the keeps listed below.
func TestOniGuideFromSettingsWalksAndKeeps(t *testing.T) {
	stage := newGuideStage(t, demo.FeatureOniGuide)
	stage.field("Goal")
	stage.typeText("how do I save this file")
	stage.click("Show me how")
	stage.awaitConsent()
	stage.click("Allow once")
	stage.await("step 1 shows", func() bool { return stage.shows("Guide step: Open the File menu.") })
	for _, want := range []string{"Step 1", "Now: Open the File menu.", "Next: Click Save.", "Click it in the window, or press Next.", "Circle it again", "Stop"} {
		if !stage.shows(want) {
			t.Fatalf("step 1 lacks %q; on screen: %v", want, stage.names())
		}
	}
	stage.click("Next")
	stage.await("step 2 shows", func() bool { return stage.shows("Guide step: Click Save.") && stage.shows("Done: Open the File menu.") })
	if stage.shows("Allow once") {
		t.Fatal("once per session asked again for step 2")
	}
	stage.click("Next")
	stage.await("the keep question opens", func() bool { return stage.shows("Keep these steps for another time?") && stage.shows("Keep it") })
	stage.click("Keep it")
	stage.await("the walkthrough is kept and listed", func() bool { return stage.shows("how do I save this file") && !stage.shows("Keep it") })
	if entries := stage.kept(); len(entries) != 4 {
		t.Fatalf("keeping left %d entries", len(entries))
	}
	if !stage.showsPrefix("Last guide: Done. The packing list is saved.") {
		t.Fatalf("the panel does not say how the guide ended; on screen: %v", stage.names())
	}
}

// The step panel hears a guide another client runs only while it is drawn:
// away from Settings nothing listens, so a keep offer made meanwhile is
// only asked about once the panel is back.
func TestOniGuidePanelListensOnlyWhileShown(t *testing.T) {
	stage := newGuideStage(t, demo.FeatureOniGuide)
	stage.await("the goal field is drawn", func() bool { _, ok := stage.editorNamed("Goal"); return ok })
	cli := stage.otherClient()
	started := guideAction(t, cli, protocol.OniGuideStart)
	var step protocol.OniAskResult
	if err := cli.Call(context.Background(), protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: started.Consent.ID, Answer: "yes"}, &step); err != nil {
		t.Fatal(err)
	}
	stage.await("the panel hears step 1 from the other client", func() bool { return stage.shows("Guide step: Open the File menu.") })

	stage.open("ask")
	guideAction(t, cli, protocol.OniGuideNext)
	end := guideAction(t, cli, protocol.OniGuideNext)
	if end.Keep == nil {
		t.Fatalf("the other client's guide offered nothing: %+v", end)
	}
	stage.drawFor(300 * time.Millisecond)
	if stage.mentions("Keep these steps for another time?") {
		t.Fatal("the keep question opened while Settings was not shown: the panel was still listening")
	}

	stage.open("settings/oni-screen")
	stage.await("back on the panel, the waiting offer is asked about", func() bool { return stage.shows("Keep these steps for another time?") })
	stage.click("Not now")
	stage.await("the question closes", func() bool { return !stage.shows("Keep it") })
	stage.await("the skip lands", func() bool { return stage.mentions("Not kept. Oni will ask a model again next time.") })
	var kept protocol.OniKeepResult
	err := cli.Call(context.Background(), protocol.MOniKeep, protocol.OniKeepParams{ID: end.Keep.ID}, &kept)
	if err == nil || !strings.Contains(err.Error(), "not waiting") {
		t.Fatalf("Not now did not skip the offer: %v, %+v", err, kept)
	}
	if entries := stage.kept(); len(entries) != 3 {
		t.Fatalf("Not now kept something: %d entries", len(entries))
	}
}

// Without a text reader the guide says what that takes away; with one it
// says nothing about it.
func TestOniGuideSaysWhenNoTextReaderIsInstalled(t *testing.T) {
	without := newGuideStage(t, demoNoOCR)
	if !without.shows("Text reader needed") || !without.mentions("tesseract is not installed") {
		t.Fatalf("no text reader is not said; on screen: %v", without.names())
	}
	with := newGuideStage(t, demo.FeatureOniGuide)
	if with.shows("Text reader needed") {
		t.Fatal("a machine with a text reader is told it needs one")
	}
}
