package shell

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"ryoku-rashin/internal/protocol"
)

func (s *stage) provider(id string) protocol.ProviderInfo {
	for _, provider := range s.runtime.store.Snapshot().Providers {
		if provider.ID == id {
			return provider
		}
	}
	s.t.Fatalf("the demo daemon has no provider %s", id)
	return protocol.ProviderInfo{}
}

// Every switch in Settings is driven here by the pointer. One that reads its
// own flip a frame late sends back the value it had, and looks fine doing it.
func TestSettingsSwitchesChangeWhatTheyName(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	settings := func() protocol.Settings { return stage.runtime.store.Snapshot().Settings }

	stage.open("settings/providers")
	if !stage.provider("openrouter").Enabled {
		t.Fatal("the demo daemon starts with OpenRouter in use")
	}
	stage.click("OpenRouter")
	stage.click("Use OpenRouter")
	stage.await("the switch stops using the provider", func() bool { return !stage.provider("openrouter").Enabled })
	stage.click("Use OpenRouter")
	stage.await("the switch uses the provider again", func() bool { return stage.provider("openrouter").Enabled })

	stage.open("settings/ask")
	habits := settings().Ask.Habits
	stage.click("Learn from the shell")
	stage.await("Learn from the shell flips", func() bool { return settings().Ask.Habits != habits })

	stage.open("settings/index")
	system, semantic := settings().Index.System, settings().Index.Semantic
	stage.click("Read this system")
	stage.await("Read this system flips", func() bool { return settings().Index.System != system })
	stage.click("Search by meaning")
	stage.await("Search by meaning flips", func() bool { return settings().Index.Semantic != semantic })

	stage.open("settings/agents")
	if stage.shell.preferences.MuteNotices {
		t.Fatal("notices start on")
	}
	stage.click("Tell me when an agent needs me")
	if !stage.shell.preferences.MuteNotices {
		t.Fatal("the notices switch did not turn them off")
	}

	stage.open("settings/appearance")
	stage.click("Reduce motion")
	if !stage.shell.preferences.ReduceMotion {
		t.Fatal("the Reduce motion switch did not turn it on")
	}
}

func TestSettingsSteppersChangeWhatTheyName(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/agents")
	before := stage.runtime.store.Snapshot().Settings.Agent.IdleStopMinutes
	stage.click("Increase")
	stage.await("the idle stop grows by a minute", func() bool {
		return stage.runtime.store.Snapshot().Settings.Agent.IdleStopMinutes == before+1
	})
}

// The sections stand in a column in the middle of the pane. The wheel used
// to move them only with the pointer over that column.
func TestTheWheelMovesSettingsFromBesideItsColumn(t *testing.T) {
	for _, size := range []image.Point{{1440, 900}, {700, 800}} {
		stage := newStage(t, size)
		stage.open("settings")
		top, ok := stage.named("Rashin is on")
		if !ok {
			t.Fatalf("at %v Settings does not open on its first switch: %v", size, stage.names())
		}
		beside := f32.Pt(float32(size.X-12), float32(size.Y/2))
		stage.queue(
			pointer.Event{Kind: pointer.Move, Position: beside},
			pointer.Event{Kind: pointer.Scroll, Position: beside, Scroll: f32.Pt(0, 240)},
		)
		stage.settle()
		if moved, still := stage.named("Rashin is on"); still && moved.Desc.Bounds.Min.Y >= top.Desc.Bounds.Min.Y {
			t.Fatalf("at %v the wheel beside the column left the first switch at %d", size, moved.Desc.Bounds.Min.Y)
		}
	}
}
