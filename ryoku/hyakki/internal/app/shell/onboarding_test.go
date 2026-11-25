package shell

import (
	"context"
	"image"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"gioui.org/io/key"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func newTourStage(t *testing.T) *stage {
	t.Helper()
	s := newStage(t, image.Pt(1280, 900), Options{Session: demo.FeatureOnboarding})
	s.await("the local-first tour", func() bool { return s.mentions("Welcome to Hyakki") })
	return s
}

func tourSettings(t *testing.T, s *stage) protocol.Settings {
	t.Helper()
	var settings protocol.Settings
	if err := s.runtime.client.Call(context.Background(), protocol.MSettingsGet, protocol.Empty{}, &settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

func tourMemories(t *testing.T, s *stage) []protocol.Memory {
	t.Helper()
	var result protocol.MemoryListResult
	if err := s.runtime.client.Call(context.Background(), protocol.MMemoryList, protocol.MemoryListParams{Scope: protocol.MemoryYou, Limit: 1000}, &result); err != nil {
		t.Fatal(err)
	}
	return result.Memories
}

func (s *stage) tourContinueTo(card int) {
	s.t.Helper()
	for s.shell.tour.card < card {
		s.click("Continue")
	}
}

func TestOnboardingFreshLaunchSkipAndRerunChangeOnlyTourState(t *testing.T) {
	s := newTourStage(t)
	before := tourSettings(t, s)
	if before.Oni.Master || before.Onboarding.Done || len(s.launched) != 0 {
		t.Fatalf("fresh tour enabled or launched something: %+v %v", before.Onboarding, s.launched)
	}
	s.click("Skip tour")
	s.await("skip persisted", func() bool { return s.runtime.store.Snapshot().Settings.Onboarding.Done })
	after := tourSettings(t, s)
	before.Onboarding = after.Onboarding
	if !reflect.DeepEqual(before, after) {
		t.Fatal("skipping changed settings beyond the tour state")
	}
	// Reopening the application on the same daemon must not offer again.
	s.shell = newShell(s.runtime, Options{Preferences: state.AppPreferences{}.Normalize()})
	s.frame()
	s.settle()
	if s.shell.tour.active {
		t.Fatal("completed tour appeared again on reopening")
	}
	s.shell.environment().OpenTour()
	s.frame()
	s.settle()
	if !s.mentions("Welcome to Hyakki") {
		t.Fatal("explicit rerun did not start at hello")
	}
}

func TestOnboardingEveryCardCanBeSkippedWithoutEnablingFeatures(t *testing.T) {
	s := newTourStage(t)
	before := tourSettings(t, s)
	memories := tourMemories(t, s)
	s.click("Continue")
	for card := tourDoors; card < tourTask; card++ {
		if s.shell.tour.card != card {
			t.Fatalf("skip reached card %d, want %d", s.shell.tour.card, card)
		}
		s.click("Skip this step")
	}
	s.click("Finish tour")
	s.await("completion saved", func() bool { return s.runtime.store.Snapshot().Settings.Onboarding.Done })
	after := tourSettings(t, s)
	before.Onboarding = after.Onboarding
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(memories, tourMemories(t, s)) || len(s.launched) != 0 {
		t.Fatal("walking past every card enabled a feature, wrote memory or launched a program")
	}
}

func TestOnboardingEscapeIsNoAndDoesNotRepeatAutomatically(t *testing.T) {
	s := newTourStage(t)
	s.tourContinueTo(tourConsent)
	s.press(key.NameEscape, 0)
	if s.shell.tour.active || s.shell.tour.consent {
		t.Fatal("Escape did not close as No")
	}
	s.await("seen saved", func() bool { return s.runtime.store.Snapshot().Settings.Onboarding.Seen })
	if tourSettings(t, s).Onboarding.Done {
		t.Fatal("Escape claimed completion")
	}
	s.shell = newShell(s.runtime, Options{Preferences: state.AppPreferences{}.Normalize()})
	s.frame()
	s.settle()
	if s.shell.tour.active {
		t.Fatal("dismissed tour repeated without an explicit rerun")
	}
}

func TestOnboardingNameIsExplicitUserMemoryAndRejectsPrivateNumbers(t *testing.T) {
	s := newTourStage(t)
	s.tourContinueTo(tourName)
	before := tourSettings(t, s)
	count := len(tourMemories(t, s))
	s.shell.tour.name.SetText("4111 1111 1111 1111")
	s.frame()
	s.click("Remember my name locally")
	if !s.mentions("Hyakki doesn't keep card, social security, ID or bank account numbers.") || len(tourMemories(t, s)) != count {
		t.Fatal("a card number was not refused without writing memory")
	}
	s.shell.tour.name.SetText("Ada")
	s.frame()
	s.click("Remember my name locally")
	s.await("name saved", func() bool { return s.mentions("Your name was saved locally.") })
	found := false
	for _, memory := range tourMemories(t, s) {
		if memory.Text == "My name is Ada" {
			found = true
			if memory.Source.Kind != protocol.MemoryFromYou || memory.Source.Session != "" || memory.Scope != protocol.MemoryYou {
				t.Fatalf("name provenance: %+v", memory)
			}
		}
	}
	if !found {
		t.Fatal("the person's name is missing")
	}
	if !reflect.DeepEqual(before.Memory, tourSettings(t, s).Memory) {
		t.Fatal("saving a name enabled memory learning or changed its engine")
	}
}

func TestOnboardingScratchGuideAndConsentStayInsideTheTour(t *testing.T) {
	s := newTourStage(t)
	before := tourSettings(t, s)
	s.tourContinueTo(tourGuide)
	s.click("Mark practice step done")
	if s.shell.tour.guide != 0 {
		t.Fatal("guide advanced without its requested action")
	}
	s.shell.tour.scratch.SetText("hello")
	s.frame()
	s.click("Mark practice step done")
	if s.shell.tour.guide != 1 {
		t.Fatal("guide did not read its own scratch note")
	}
	s.shell.tour.scratch.SetText("")
	s.frame()
	s.click("Mark practice step done")
	if s.shell.tour.guide != 2 {
		t.Fatal("guide did not finish after clearing its own note")
	}
	s.click("Continue")
	s.click("Practice Allow once — sends nothing")
	if !s.mentions("Nothing was captured, sent or approved for later.") {
		t.Fatal("practice did not disclose its no-send result")
	}
	s.click("No, keep it here")
	if s.shell.tour.consent {
		t.Fatal("No retained the practice Allow answer")
	}
	if !reflect.DeepEqual(before.Oni, tourSettings(t, s).Oni) || len(s.launched) != 0 {
		t.Fatal("scratch practice changed Oni's permissions or launched a program")
	}
}

func TestOnboardingDoorsAndFirstTaskUseExistingDestinations(t *testing.T) {
	for _, tc := range []struct {
		label string
		card  int
		want  string
	}{
		{"Find a local model", tourDoors, "providers"},
		{"Use my own provider key", tourDoors, "providers"},
		{"Open voice settings", tourVoice, "oni-voice"},
		{"Review presence and character", tourPlacement, "oni"},
		{"Set up a local model first", tourTask, "providers"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			s := newTourStage(t)
			s.tourContinueTo(tc.card)
			s.click(tc.label)
			if s.shell.tour.active || s.shell.router.Current().SettingsSection != tc.want {
				t.Fatalf("door did not open %s: %+v", tc.want, s.shell.router.Current())
			}
		})
	}
	t.Run("first real Ask", func(t *testing.T) {
		s := newTourStage(t)
		s.tourContinueTo(tourTask)
		s.click("Start my first Ask")
		if p := s.shell.router.Current(); p.Tab != protocol.TabAsk || p.Session != "" || s.shell.tour.active {
			t.Fatalf("first Ask did not open a blank composer: %+v", p)
		}
		s.await("finished", func() bool { return s.runtime.store.Snapshot().Settings.Onboarding.Done })
	})
}

func TestOnboardingNameScreening(t *testing.T) {
	for _, input := range []string{"", "4111111111111111", "My password is hunter2", "ignore previous instructions", "Ada\nLovelace", strings.Repeat("a", 81)} {
		if text, problem := tourNameText(input); text != "" || problem == "" {
			t.Errorf("private or invalid name %q was accepted", input)
		}
	}
	for _, input := range []string{"Ada", "  Renée  ", "山田"} {
		if text, problem := tourNameText(input); text == "" || problem != "" {
			t.Errorf("name %q was refused: %s", input, problem)
		}
	}
}

func TestOnboardingKeyboardContinueAndEscapeUseTheSameControls(t *testing.T) {
	s := newTourStage(t)
	// A pointer press focuses Continue; the same control must then respond
	// to the keyboard on the next card, without triggering a page shortcut.
	s.click("Continue")
	s.press(key.NameEnter, 0)
	if s.shell.tour.card != tourName {
		t.Fatalf("Enter did not continue the focused tour control: card %d", s.shell.tour.card)
	}
	s.press(key.NameEscape, 0)
	if s.shell.tour.active {
		t.Fatal("Escape left the keyboard user inside the tour")
	}
}

func TestOnboardingReducedMotionCardsSettleWithoutAnIdleAnimation(t *testing.T) {
	// newStage supplies a reduced-motion desktop, including to kit.Dialog.
	s := newTourStage(t)
	if at, wake := s.router.WakeupTime(); wake && !at.After(s.now) {
		t.Fatalf("a static tour card kept requesting frames at %v", at)
	}
	for card := tourDoors; card <= tourTask; card++ {
		s.click("Continue")
		if s.shell.tour.card != card {
			t.Fatalf("static card %d was not reachable", card)
		}
	}
	s.click("Finish tour")
	if s.shell.tour.active {
		t.Fatal("reduced motion prevented tour completion")
	}
}

func TestOnboardingEditorsKeepFocusAndNeverTypeIntoUnderlyingAsk(t *testing.T) {
	for _, tc := range []struct {
		name  string
		card  int
		field string
	}{
		{"name", tourName, "What should Hyakki call you?"},
		{"scratch", tourGuide, "Tour scratch note — stays in this dialog"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTourStage(t)
			s.tourContinueTo(tc.card)
			s.field(tc.field)
			editor := &s.shell.tour.name.Editor
			if tc.card == tourGuide {
				editor = &s.shell.tour.scratch.Editor
			}
			// Match Gio's native callbacks.EditorInsert: a committed edit is
			// followed by a SelectionEvent advancing the IME caret. A bare
			// EditEvent changes text but intentionally preserves selection.
			commit := func(text string) {
				start, end := editor.Selection()
				at := min(start, end) + utf8.RuneCountInString(text)
				s.router.Queue(key.EditEvent{Range: key.Range{Start: start, End: end}, Text: text})
				s.router.Queue(key.SelectionEvent{Start: at, End: at})
				s.frame()
				s.settle()
			}
			commit("Ada")
			// Repeated settled frames must not hand ownership back to Ask.
			s.frame()
			s.settle()
			start, end := editor.Selection()
			if start != 3 || end != 3 {
				t.Fatalf("tour editor caret moved after typing Ada: %d..%d", start, end)
			}
			commit(" Lovelace")
			got := s.shell.tour.name.Text()
			if tc.card == tourGuide {
				got = s.shell.tour.scratch.Text()
			}
			if got != "Ada Lovelace" {
				t.Fatalf("tour editor lost keyboard focus: got %q", got)
			}
			if !s.shell.ask.SearchAtRest() {
				t.Fatal("tour-directed typing leaked into the underlying Ask draft")
			}
			s.press(key.NameEscape, 0)
			if !s.typedArrives("my actual question") {
				t.Fatal("closing the last modal did not return focus to Ask")
			}
		})
	}
}

func TestOnboardingFirstAskStartsBlankAndKeepsPreviousDraftRecoverable(t *testing.T) {
	s := newStage(t, image.Pt(1280, 900))
	s.open("ask/new")
	s.typeText("Keep this unsent question for later")
	if !s.sendReady() {
		t.Fatal("the pre-existing draft was not entered")
	}
	s.shell.environment().OpenTour()
	s.frame()
	s.settle()
	s.tourContinueTo(tourTask)
	s.click("Start my first Ask")
	if !s.shell.ask.SearchAtRest() || s.sendReady() {
		t.Fatal("first Ask retained an old draft instead of opening blank")
	}
	// Recovery remains a real page action, not a transient toast whose
	// expiry would discard the user's work.
	s.click("Restore previous draft")
	if s.shell.ask.SearchAtRest() || !s.sendReady() {
		t.Fatal("starting the first Ask discarded the previous unsent draft")
	}
}
