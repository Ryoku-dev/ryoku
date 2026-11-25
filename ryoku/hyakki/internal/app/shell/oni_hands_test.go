package shell

import (
	"context"
	"encoding/json"
	"image"
	"regexp"
	"strings"
	"testing"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/protocol"
)

// newHandsStage opens Settings at Oni › Hands on the demo. The window is
// shorter than what follows the page's head, so the head stays at the top
// as the page grows; the list is not bottomed out on an earlier section.
func newHandsStage(t *testing.T, session string) *stage {
	t.Helper()
	height := 3200
	if session == demo.FeatureOni {
		height = 1800
	}
	stage := newStage(t, image.Pt(1440, height), Options{Route: protocol.TabSettings, SettingsSection: "hands", Session: session})
	stage.open("settings/hands")
	stage.await("the Hands page is drawn", func() bool { return stage.shows("Before you use Hands") })
	return stage
}

// where the demo daemon's Hands stand now
func (s *stage) handsStatus() protocol.OniHandsResult {
	s.t.Helper()
	var result protocol.OniHandsResult
	if err := s.runtime.client.Call(context.Background(), protocol.MOniHands, protocol.OniHandsParams{Action: protocol.OniHandsStatus}, &result); err != nil {
		s.t.Fatal(err)
	}
	return result
}

func (s *stage) setOni(patch map[string]any) {
	s.t.Helper()
	raw, _ := json.Marshal(map[string]any{"oni": patch})
	if err := s.runtime.client.Call(context.Background(), protocol.MSettingsSet, protocol.SettingsSetParams{Patch: raw}, new(json.RawMessage)); err != nil {
		s.t.Fatal(err)
	}
	s.settle()
}

// The switch writes oni.hands and nothing else, and says what it needs;
// turned on, the page finds the browser and offers the rest.
func TestOniHandsSwitchWritesTheStageAndSaysWhatItNeeds(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOni)
	if oni := stage.daemonOni(); oni.Hands != protocol.OniHandsOff {
		t.Fatalf("hands start at %q", oni.Hands)
	}
	if stage.shows("Authorize a tab") || stage.mentions("Browser: ") {
		t.Fatalf("Hands off shows its tab picker or browser; on screen: %v", stage.names())
	}
	for _, want := range []string{"Only this app turns Hands on", "Oni stays running while the Hands browser is open", "A page can influence what Hands types into a field", "The log is read while Hands is on."} {
		if !stage.mentions(want) {
			t.Fatalf("the page does not say %q; on screen: %v", want, stage.names())
		}
	}
	stage.toggle("Hands in a browser")
	stage.await("hands turn on", func() bool { return stage.daemonOni().Hands == protocol.OniHandsBrowser })
	stage.await("the browser is found", func() bool { return stage.shows("Browser: chromium") })
	if oni := stage.daemonOni(); !oni.Inspect || !oni.Master || oni.Voice || oni.Guide.Enabled {
		t.Fatalf("the switch reached settings it does not own: %+v", oni)
	}
	stage.setOni(map[string]any{"inspect": false})
	stage.await("the switch says it needs looking at a window", func() bool { return stage.mentions("Needs Looking at a window") })
	stage.await("the browser row goes with Hands", func() bool { return !stage.shows("Browser: chromium") })
	stage.setOni(map[string]any{"inspect": true})
	stage.toggle("Hands in a browser")
	stage.await("hands turn off", func() bool { return stage.daemonOni().Hands == protocol.OniHandsOff })
}

// Open lists the tabs by title and origin; Authorize binds one; a goal
// starts a task whose first step asks before anything is sent, Allow
// answers it, Stop ends it, and the log lists codes only: no goal word and
// no page title reaches any row.
func TestOniHandsOpenAuthorizeStartAllowStop(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOniHands)
	stage.await("the browser is found", func() bool { return stage.shows("Browser: chromium") })
	stage.click("Open the Hands browser")
	stage.await("the tabs are listed", func() bool { return stage.shows("Contact us") && stage.shows("http://127.0.0.1:8137") })
	if !stage.shows("Example Domain") || !stage.shows("https://example.org") {
		t.Fatalf("the second tab is not listed; on screen: %v", stage.names())
	}
	if status := stage.handsStatus(); status.State != protocol.OniHandsStateOpen {
		t.Fatalf("open left %+v", status)
	}
	stage.click("Authorize the tab on http://127.0.0.1:8137")
	stage.await("the tab is authorized", func() bool { return stage.mentions("Authorized: http://127.0.0.1:8137.") })

	goal := "fill the contact form with name Ada"
	stage.field("Goal")
	stage.typeText(goal)
	stage.click("Start Hands")
	stage.await("the task works", func() bool { return stage.shows("Working · step 1 of 25") })
	stage.await("the first step asks first", func() bool { return stage.shows("Send this step?") })
	if !stage.mentions("its visible text and its controls' names") || !stage.mentions("It goes to ") {
		t.Fatalf("the step card does not say what goes and where; on screen: %v", stage.names())
	}
	if stage.mentions("name Ada") {
		t.Fatal("the goal stays on screen after Start")
	}
	stage.click("Allow this step")
	stage.await("the card goes once answered", func() bool { return !stage.shows("Send this step?") })
	if status := stage.handsStatus(); status.State != protocol.OniHandsStateWorking {
		t.Fatalf("allowing the step left %+v", status)
	}

	stage.click("Stop Hands")
	stage.await("the task stops", func() bool { return stage.shows("Give Hands a goal") })
	if status := stage.handsStatus(); status.State != protocol.OniHandsStateAuthorized {
		t.Fatalf("stop left %+v", status)
	}

	stage.await("the log is read", func() bool { return stage.shows("Fill a field") })
	row := regexp.MustCompile(`^[A-Z][a-z]{2} \d\d:\d\d:\d\d · task [a-z0-9_-]+ · step \d+ · `)
	rows := 0
	for _, n := range stage.names() {
		if !row.MatchString(n) {
			continue
		}
		rows++
		for _, word := range []string{"Ada", "contact form", "Contact us", "Example Domain", "127.0.0.1", "example.org"} {
			if strings.Contains(n, word) {
				t.Fatalf("a log row holds %q: %q", word, n)
			}
		}
	}
	if rows != 4 || !stage.mentions("Handed to you: it submits") || !stage.mentions("Refused: a private field") {
		t.Fatalf("the log shows %d rows: %v", rows, stage.names())
	}
}

// A task paused on the demo says why, and Resume goes on from its step.
func TestOniHandsPausedSaysWhyAndResumes(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOniHandsPaused)
	stage.await("the pause is shown", func() bool { return stage.shows("Paused · step 4 of 25") })
	if !stage.mentions("Another window took focus.") {
		t.Fatalf("the pause does not say why; on screen: %v", stage.names())
	}
	stage.click("Resume Hands")
	stage.await("the task works again", func() bool { return stage.shows("Working · step 4 of 25") })
	if _, ok := stage.named("Resume Hands"); ok {
		t.Fatal("Resume stays offered on a task that works")
	}
}

// With no Chromium-family browser, the page says so and how to get one,
// and its Open opens nothing.
func TestOniHandsSaysWhenNoBrowserIsFound(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOniHandsNoBrowser)
	stage.await("the missing browser is named", func() bool { return stage.shows("No Chromium-family browser found") })
	if !stage.mentions("Firefox, Flatpak and snap builds cannot be driven") {
		t.Fatalf("the page does not say which builds are refused; on screen: %v", stage.names())
	}
	if node, ok := stage.named("Open the Hands browser"); ok {
		stage.clickAt(centre(node.Desc.Bounds))
		stage.settle()
	}
	if status := stage.handsStatus(); status.State != protocol.OniHandsStateClosed {
		t.Fatalf("Open with no browser left %+v", status)
	}
}

// Forget asks first, Cancel leaves the profile, Forget it deletes it; while
// the Hands browser is open, Forget is not offered.
func TestOniHandsForgetAsksFirst(t *testing.T) {
	stage := newHandsStage(t, demo.FeatureOniHands)
	stage.await("the browser is found", func() bool { return stage.shows("Browser: chromium") })
	stage.click("Forget the Hands browser and its profile")
	stage.await("the question opens", func() bool { return stage.shows("Forget the Hands browser?") })
	stage.click("Cancel")
	stage.await("the question closes", func() bool { return !stage.shows("Forget the Hands browser?") })
	if stage.mentions("The Hands browser is forgotten") {
		t.Fatal("Cancel forgot the Hands browser")
	}
	stage.click("Forget the Hands browser and its profile")
	stage.await("the question opens again", func() bool { return stage.shows("Forget the Hands browser?") })
	stage.click("Forget it")
	stage.await("the profile is forgotten", func() bool { return stage.mentions("The Hands browser is forgotten") })

	stage.click("Open the Hands browser")
	stage.await("the browser opens", func() bool { return stage.mentions("Close the Hands browser first.") })
	if node, ok := stage.named("Forget the Hands browser and its profile"); ok {
		stage.clickAt(centre(node.Desc.Bounds))
		stage.settle()
	}
	if stage.shows("Forget the Hands browser?") {
		t.Fatal("Forget asked while the Hands browser is open")
	}
}

// Hands stands under Oni: Oni's own row leads there, and the page is
// headed as part of Oni.
func TestOniHandsIsAPageUnderOni(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1600), Options{Route: protocol.TabSettings, SettingsSection: "oni", Session: demo.FeatureOni})
	stage.open("settings/oni")
	stage.reveal("Open the Hands settings")
	stage.click("Open the Hands settings")
	stage.await("the Hands page heads the column", func() bool {
		node, ok := stage.named("Oni › Hands")
		return ok && node.Desc.Bounds.Min.Y >= 0 && node.Desc.Bounds.Max.Y < 400 && stage.mentions("Only this app turns Hands on")
	})
}
