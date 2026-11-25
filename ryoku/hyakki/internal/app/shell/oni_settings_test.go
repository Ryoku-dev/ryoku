package shell

import (
	"context"
	"image"
	"slices"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"

	"ryoku-rashin/internal/protocol"
)

func (s *stage) oniStatus() protocol.OniStatus {
	s.t.Helper()
	var status protocol.OniStatus
	if err := s.runtime.client.Call(context.Background(), protocol.MOniStatus, protocol.Empty{}, &status); err != nil {
		s.t.Fatalf("oni.status: %v", err)
	}
	return status
}

func newOniStage(t *testing.T) *stage {
	t.Helper()
	// tall enough for the whole section, the preview stands at the size the desktop draws Oni
	stage := newStage(t, image.Pt(1440, 1900), Options{Route: protocol.TabSettings, SettingsSection: "oni"})
	stage.open("settings/oni")
	stage.await("the character picker reads the daemon's characters", func() bool {
		_, ok := stage.named("Character: Oni")
		return ok
	})
	return stage
}

// The picker is the daemon's list: a character that loads becomes the one in
// use, one the loader rejected is shown with why and cannot be picked, and
// reset brings back the shipped Oni.
func TestOniSettingsPickerChoosesRefusesAndResets(t *testing.T) {
	stage := newOniStage(t)
	if got := stage.oniStatus().Character; got != protocol.OniDefaultCharacter {
		t.Fatalf("the demo starts on %q, want oni", got)
	}

	stage.click("Character: Oni")
	stage.click("Moth")
	stage.await("the daemon uses the moth", func() bool { return stage.oniStatus().Character == "testmoth" })
	stage.await("the picker names the moth", func() bool {
		_, ok := stage.named("Character: Moth")
		return ok
	})

	var lantern protocol.OniCharacter
	for _, c := range stage.oniStatus().Characters {
		if c.ID == "lantern" {
			lantern = c
		}
	}
	if lantern.Valid || len(lantern.Errors) == 0 {
		t.Fatalf("the demo's lantern should be rejected with its errors, got %+v", lantern)
	}
	if _, ok := stage.named(lantern.Errors[0]); !ok {
		t.Fatalf("the lantern's error %q is not shown; there is: %v", lantern.Errors[0], stage.names())
	}
	stage.click("Character: Moth")
	var option input.SemanticNode
	found := slices.ContainsFunc(stage.nodes(), func(node input.SemanticNode) bool {
		if node.Desc.Label == "lantern" && node.Desc.Gestures&input.ClickGesture != 0 {
			option = node
			return true
		}
		return false
	})
	if !found || !option.Desc.Disabled {
		t.Fatalf("the lantern is not listed as a choice that cannot be picked: found %v, %+v", found, option.Desc)
	}
	stage.clickAt(centre(option.Desc.Bounds))
	stage.press(key.NameEscape, 0)
	if got := stage.oniStatus().Character; got != "testmoth" {
		t.Fatalf("picking the lantern changed the character to %q", got)
	}

	stage.click("Reset to Oni")
	stage.await("reset returns to oni", func() bool { return stage.oniStatus().Character == protocol.OniDefaultCharacter })
	if got := stage.runtime.store.Snapshot().Settings.Oni.Character; got != protocol.OniDefaultCharacter {
		t.Fatalf("the settings still name %q after reset", got)
	}
}

func TestOniSettingsReduceMotionPersists(t *testing.T) {
	stage := newOniStage(t)
	if got := stage.runtime.store.Snapshot().Settings.Oni.ReduceMotion; got != protocol.OniReduceFollow {
		t.Fatalf("reduce motion starts %q, want follow", got)
	}
	stage.click("Reduce motion: Follow the desktop")
	stage.press(key.NameDownArrow, 0)
	stage.press(key.NameReturn, 0)
	stage.await("the daemon keeps reduce motion on", func() bool {
		var settings protocol.Settings
		if err := stage.runtime.client.Call(context.Background(), protocol.MSettingsGet, protocol.Empty{}, &settings); err != nil {
			t.Fatal(err)
		}
		return settings.Oni.ReduceMotion == protocol.OniReduceOn
	})
	stage.open("settings/agents")
	stage.open("settings/oni")
	stage.await("the select reads what was kept", func() bool {
		_, ok := stage.named("Reduce motion: On")
		return ok
	})
}
