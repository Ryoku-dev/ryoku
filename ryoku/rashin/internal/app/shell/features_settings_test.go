package shell

import (
	"encoding/json"
	"image"
	"slices"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"ryoku-rashin/internal/protocol"
)

// Keep the owner-facing set explicit so a removed or missing switch cannot make
// a bulk action pass by changing the catalogue the assertion reads from.
var discoverableFeatureIDs = []string{
	"compare", "research", "schedules", "prompts", "enhance", "askwriting", "analyze", "audio", "video", "share",
	"quote", "history", "searchsteps", "mindmap", "artifacts", "collections", "profile", "setupcards", "mcp", "diskcare",
}

func (s *stage) wantFeatures(enabled ...string) {
	s.t.Helper()
	features := s.settings().Features
	for _, id := range discoverableFeatureIDs {
		if got, want := features[id], slices.Contains(enabled, id); got != want {
			s.t.Errorf("feature %s = %v, want %v", id, got, want)
		}
	}
	for id, on := range features {
		if on && !slices.Contains(discoverableFeatureIDs, id) {
			s.t.Errorf("an unexpected feature is on: %s", id)
		}
	}
}

// Settings is one scrollable document. Reach the actual named control by wheel
// rather than bypassing the input router or its off-screen hit regions.
func (s *stage) scrollToFeatureControl(name string) {
	s.t.Helper()
	for range 120 {
		scroll := float32(40)
		if node, ok := s.named(name); ok {
			at := centre(node.Desc.Bounds)
			if at.In(image.Rect(0, 100, s.size.X, s.size.Y-20)) {
				return
			}
			if at.Y < 100 {
				scroll = -scroll
			}
		}
		at := f32.Pt(float32(s.size.X-20), float32(s.size.Y/2))
		s.queue(
			pointer.Event{Kind: pointer.Move, Position: at},
			pointer.Event{Kind: pointer.Scroll, Position: at, Scroll: f32.Pt(0, scroll)},
		)
		s.settle()
	}
	s.t.Fatalf("could not scroll to %q; on screen: %v", name, s.names())
}

func TestFeaturesSettingsRouteOpensFromDemoStartupAndUIFocus(t *testing.T) {
	t.Run("demo route", func(t *testing.T) {
		stage := newStage(t, image.Pt(1440, 900))
		stage.open("settings/features")
		if place := stage.shell.router.Current(); place.Tab != protocol.TabSettings || place.SettingsSection != "features" {
			t.Fatalf("demo route opened %#v, want settings/features", place)
		}
		if _, shown := stage.named("0 of 20 on"); !shown {
			t.Fatalf("the demo route did not show Features; on screen: %v", stage.names())
		}
		stage.wantFeatures()
	})
	t.Run("startup", func(t *testing.T) {
		stage := newStage(t, image.Pt(1440, 900))
		stage.shell = newShell(stage.runtime, Options{Route: protocol.TabSettings, SettingsSection: "features"})
		stage.frame()
		stage.settle()
		if place := stage.shell.router.Current(); place.Tab != protocol.TabSettings || place.SettingsSection != "features" {
			t.Fatalf("startup opened %#v, want settings/features", place)
		}
		if _, shown := stage.named("0 of 20 on"); !shown {
			t.Fatalf("startup lost the Features section after its first frame; on screen: %v", stage.names())
		}
	})
	t.Run("running app", func(t *testing.T) {
		stage := newStage(t, image.Pt(1440, 900))
		stage.open("ask/new")
		data, err := json.Marshal(protocol.UIOpenParams{Tab: protocol.TabSettings, View: "features"})
		if err != nil {
			t.Fatal(err)
		}
		if err := stage.runtime.store.Apply(protocol.Event{Event: protocol.EvtUIFocus, Data: data}); err != nil {
			t.Fatal(err)
		}
		stage.frame()
		stage.settle()
		if place := stage.shell.router.Current(); place.Tab != protocol.TabSettings || place.SettingsSection != "features" {
			t.Fatalf("ui.open focus opened %#v, want settings/features", place)
		}
		if _, shown := stage.named("0 of 20 on"); !shown {
			t.Fatalf("ui.open focus did not show Features; on screen: %v", stage.names())
		}
	})
}

func TestCompareModelsSwitchChangesTheDaemonAndAnswerSurface(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1600))
	const answer = "ask/ask-20261110-094200-a17c"
	stage.open(answer)
	stage.wantFeatures()
	if _, shown := stage.named("Compare"); shown {
		t.Fatal("an answer offers Compare while Compare models is off")
	}

	stage.open("settings/features")
	stage.scrollToFeatureControl("Compare models")
	stage.click("Compare models")
	stage.await("Compare models reaches settings.features.compare", func() bool { return stage.settings().Features["compare"] })
	stage.wantFeatures("compare")
	stage.open("settings/features")
	if _, shown := stage.named("1 of 20 on"); !shown {
		t.Fatalf("the feature count did not follow the switch; on screen: %v", stage.names())
	}

	stage.open(answer)
	stage.await("Compare appears on the answer", func() bool { _, shown := stage.named("Compare"); return shown })
	stage.open("settings/features")
	stage.scrollToFeatureControl("Compare models")
	stage.click("Compare models")
	stage.await("Compare models turns off again", func() bool { return !stage.settings().Features["compare"] })
	stage.wantFeatures()
	stage.open(answer)
	if _, shown := stage.named("Compare"); shown {
		t.Fatal("the answer still offers Compare after its feature was turned off")
	}
}

func TestFeatureBulkButtonsChangeAllTwentyFeatures(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/features")
	stage.wantFeatures()
	stage.click("Turn all on")
	stage.await("all twenty optional features turn on", func() bool {
		for _, id := range discoverableFeatureIDs {
			if !stage.settings().Features[id] {
				return false
			}
		}
		return true
	})
	stage.wantFeatures(discoverableFeatureIDs...)
	if _, shown := stage.named("20 of 20 on"); !shown {
		t.Fatalf("the count does not show every feature on; on screen: %v", stage.names())
	}
	stage.click("Turn all off")
	stage.await("all twenty optional features turn off", func() bool {
		for _, id := range discoverableFeatureIDs {
			if stage.settings().Features[id] {
				return false
			}
		}
		return true
	})
	stage.wantFeatures()
	if _, shown := stage.named("0 of 20 on"); !shown {
		t.Fatalf("the count does not show every feature off; on screen: %v", stage.names())
	}
}

func TestRashinExplainsAllOffAndLinksToFeatures(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/rashin")
	if _, shown := stage.named("20 optional features are off"); !shown {
		t.Fatalf("Rashin does not explain the quiet default; on screen: %v", stage.names())
	}
	stage.click("Open Features")
	if _, shown := stage.named("0 of 20 on"); !shown {
		t.Fatalf("Features did not open its controls; on screen: %v", stage.names())
	}
	stage.click("Turn all on")
	stage.await("Compare models turns on", func() bool { return stage.settings().Features["compare"] })
	stage.open("settings/rashin")
	if _, shown := stage.named("20 optional features are off"); shown {
		t.Fatal("the all-off explanation remains after features turn on")
	}
}

func TestFeatureSettingsOpensTheDesktopTheiaControls(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/features")
	stage.scrollToFeatureControl("Open Theia settings")
	stage.click("Open Theia settings")
	if len(stage.argv) != 1 {
		t.Fatalf("the desktop was asked to start %v, want one Theia settings launch", stage.argv)
	}
	want := []string{"ryoku-shell", "hub", "open", "rashin/theia-on"}
	if !slices.Equal(stage.argv[0], want) {
		t.Fatalf("Theia settings argv = %q, want %q", stage.argv[0], want)
	}
	if _, fronted := stage.lastLaunch(); !fronted {
		t.Fatal("Theia settings was not asked to arrive in front of Rashin")
	}
}

func TestEnabledPromptLibraryLinksToItsSettingsBlock(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/features")
	if _, shown := stage.named("Prompt library settings"); shown {
		t.Fatal("Prompt library settings is offered while Prompt library is off")
	}
	stage.scrollToFeatureControl("Prompt library")
	stage.click("Prompt library")
	stage.await("Prompt library turns on", func() bool { return stage.settings().Features["prompts"] })
	stage.wantFeatures("prompts")
	stage.scrollToFeatureControl("Prompt library settings")
	stage.click("Prompt library settings")
	if _, shown := stage.named("Add prompt"); !shown {
		t.Fatalf("Prompt library settings did not reach its exact block; on screen: %v", stage.names())
	}
}
