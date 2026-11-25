package shell

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"ryoku-rashin/internal/protocol"
)

const (
	askedHosted = "openrouter/qwen/qwen2.5-vl-72b-instruct"
	askedLocal  = "rashin-models/gemma-3-4b-it"
	askedAnswer = "It is a packing list with nine items. Three are ticked: passport, charger and the rain jacket."
)

func newOniAskStage(t *testing.T, session string) *stage {
	t.Helper()
	// Navigate to the screen sub-page, not the former all-in-one Oni stage.
	stage := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabSettings, SettingsSection: "oni-screen", Session: session})
	stage.open("settings/oni-screen")
	stage.await("the screen-ask entry is there", func() bool {
		_, ok := stage.named("Question")
		return ok
	})
	return stage
}

// askOni types a question about the perched window and sends it
func (s *stage) askOni(question string) {
	s.t.Helper()
	field, ok := s.editorNamed("Question")
	if !ok {
		s.t.Fatalf("there is no question field; on screen: %v", s.names())
	}
	s.clickAt(centre(field.Desc.Bounds))
	s.typeText(question)
	s.click("Ask Oni")
}

// whether any line on screen starts with prefix
func (s *stage) showsPrefix(prefix string) bool {
	for _, name := range s.names() {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func (s *stage) awaitConsent() {
	s.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !s.shows("Allow once") {
		if time.Now().After(deadline) {
			s.t.Fatalf("the picture never waited for a yes; on screen: %v", s.names())
		}
		time.Sleep(2 * time.Millisecond)
		s.frame()
	}
	s.settle()
}

// The question shows the exact picture, where it would go and how it was
// taken; nothing comes back before Allow once, and after it the answer does.
func TestOniAskShowsThePictureAndSendsOnlyOnAllow(t *testing.T) {
	stage := newOniAskStage(t, "")
	stage.askOni("what is this window for?")
	stage.awaitConsent()
	for _, want := range []string{"Send this picture?", "Goes to " + askedHosted, "The window's own picture", "960 × 620 pixels", "The picture that would be sent, 960 by 620 pixels", "Cancel"} {
		if !stage.shows(want) {
			t.Fatalf("the question lacks %q; on screen: %v", want, stage.names())
		}
	}
	if stage.shows("Oni's answer") || stage.showsPrefix(askedAnswer) {
		t.Fatal("an answer came back before the picture was allowed")
	}

	stage.click("Allow once")
	stage.await("the answer arrives", func() bool { return stage.shows("Oni's answer") })
	if stage.shows("Allow once") {
		t.Fatal("the question stayed open after Allow once")
	}
	if !stage.showsPrefix(askedAnswer) {
		t.Fatalf("the answer text is missing; on screen: %v", stage.names())
	}

	// once per session: the same destination is not asked about again
	stage.click("Ask Oni")
	stage.await("the second answer arrives", func() bool { return stage.shows("Oni's answer") && !stage.shows("Allow once") })

	// every time: it is
	stage.click("Ask before sending: Once per session")
	stage.click("Every time")
	stage.await("the setting is every time", func() bool {
		return stage.runtime.store.Snapshot().Settings.Oni.Disclosure == protocol.OniDisclosureAlways
	})
	stage.click("Ask Oni")
	stage.awaitConsent()
}

// Cancel, Escape and the question running out of time each send nothing,
// and the next question is asked about again.
func TestOniAskCancelEscapeAndExpirySendNothing(t *testing.T) {
	stage := newOniAskStage(t, "")

	stage.askOni("what is this window for?")
	stage.awaitConsent()
	stage.click("Cancel")
	stage.await("cancel closes the question", func() bool { return !stage.shows("Allow once") })
	if !stage.shows("Nothing was sent") || stage.shows("Oni's answer") {
		t.Fatalf("cancel did not say nothing was sent; on screen: %v", stage.names())
	}

	stage.click("Ask Oni")
	stage.awaitConsent()
	stage.press(key.NameEscape, 0)
	stage.await("escape closes the question", func() bool { return !stage.shows("Allow once") })
	if !stage.shows("Nothing was sent") || stage.shows("Oni's answer") {
		t.Fatalf("escape did not refuse; on screen: %v", stage.names())
	}

	// a refusal is never remembered: the next question asks again
	stage.click("Ask Oni")
	stage.awaitConsent()
	stage.now = stage.now.Add(121 * time.Second)
	stage.frame()
	stage.settle()
	if stage.shows("Allow once") {
		t.Fatal("the question outlived its two minutes")
	}
	if !stage.shows("The picture waited too long and was dropped. Nothing was sent.") || stage.shows("Oni's answer") {
		t.Fatalf("expiry did not say the picture was dropped; on screen: %v", stage.names())
	}
}

// A local route says the picture stays on this computer.
func TestOniAskOnALocalRouteStaysOnThisComputer(t *testing.T) {
	stage := newOniAskStage(t, "oni-local")
	stage.askOni("what is this window for?")
	stage.awaitConsent()
	if !stage.shows("Show Oni this picture?") || !stage.shows("Stays on this computer") || stage.showsPrefix("Goes to ") {
		t.Fatalf("the local question; on screen: %v", stage.names())
	}
	stage.click("Allow once")
	stage.await("the answer arrives", func() bool { return stage.shows("Oni's answer") })
	if !stage.showsPrefix(askedAnswer) || !stage.showsPrefix(askedAnswer+"\nAnswered by "+askedLocal+" on this computer.") {
		t.Fatalf("the local answer; on screen: %v", stage.names())
	}
}

// Letting go leaves nothing to ask about, and the refusal is said; with
// looking at windows off the entry cannot be used, and the switches are the
// user's settings.
func TestOniAskEntryWaitsForThePerchAndTheLookingSwitch(t *testing.T) {
	stage := newOniAskStage(t, "")
	stage.click("Let go of the window")
	stage.askOni("what is this window for?")
	stage.await("the refusal is said", func() bool { return stage.shows("Oni could not ask") })
	if stage.shows("Allow once") || !stage.showsPrefix("Oni is not perched on a window.") {
		t.Fatalf("an ask with nothing perched; on screen: %v", stage.names())
	}

	stage.click("Let Oni look at a window")
	stage.await("looking is off", func() bool { return !stage.runtime.store.Snapshot().Settings.Oni.Inspect })
	if !stage.shows("Turn on Let Oni look at a window first.") {
		t.Fatalf("the entry does not say why it is unavailable; on screen: %v", stage.names())
	}
	stage.click("Follow the focused window")
	stage.await("follow focus is on", func() bool { return stage.runtime.store.Snapshot().Settings.Oni.PerchFollowsFocus })
}
