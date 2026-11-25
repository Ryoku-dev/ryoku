package shell

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/protocol"
)

// Settings controls must be reached as a person reaches them: navigate to
// their section, then wheel within a bounded window. No giant test canvas
// makes off-screen controls accidentally clickable.
func (s *stage) settingsReveal(name string) {
	s.settingsRevealTarget(name, false)
}

func (s *stage) settingsPoint(name string, interactive bool) (image.Point, bool) {
	for _, node := range s.nodes() {
		if node.Desc.Label != name && node.Desc.Description != name || interactive && node.Desc.Disabled {
			continue
		}
		if point, ok := s.semanticPoint(node, interactive); ok {
			return point, true
		}
	}
	return image.Point{}, false
}

func (s *stage) settingsRevealTarget(name string, interactive bool) {
	s.t.Helper()
	if s.shell.router.Current().Tab != protocol.TabSettings {
		return
	}
	for step := range 240 {
		if _, ok := s.settingsPoint(name, interactive); ok {
			return
		}
		at := f32.Pt(float32(s.size.X)*2/3, float32(s.size.Y)/2)
		delta := float32(80)
		if step >= 80 {
			delta = -80
		}
		if node, ok := s.named(name); ok {
			bounds := node.Desc.Bounds
			// The sidebar is a separate clipped scroller. Wheel over it
			// for a sidebar target, not over the page's content column.
			if bounds.Max.X < s.size.X/3 {
				at.X = float32(centre(bounds).X)
			}
			if centre(bounds).Y < s.size.Y/2 {
				delta = -80
			} else {
				delta = 80
			}
		}
		s.queue(pointer.Event{Kind: pointer.Move, Position: at}, pointer.Event{Kind: pointer.Scroll, Position: at, Scroll: f32.Pt(0, delta)})
		s.settle()
	}
	s.t.Fatalf("Settings control %q was not reachable by scrolling", name)
}

func TestOniSettingsRoutesNavigateToTheirOwnControls(t *testing.T) {
	for _, check := range []struct{ section, control string }{
		{"oni", "Oni on the desktop"},
		{"oni-voice", "Talk to Oni"},
		{"oni-screen", "Let Oni look at a window"},
		{"oni-commands", "Quick commands"},
		{"oni-writing", "Rewrite in place"},
		{"oni-privacy", "Serve Oni's tools to other programs"},
		{"hands", "Hands in a browser"},
		{"parts", "Measure the optional parts again"},
	} {
		t.Run(check.section, func(t *testing.T) {
			s := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabSettings, SettingsSection: check.section, Session: demo.FeatureOni})
			s.open("settings/" + check.section)
			s.settingsReveal(check.control)
		})
	}
}

func TestOniPhase16SettingsWriteOnlyTheirOwnSwitches(t *testing.T) {
	s := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabSettings, SettingsSection: "oni-writing", Session: demo.FeatureOni})
	s.open("settings/oni-writing")
	before := s.daemonOni()
	for _, name := range []string{"Recall what I kept", "Include the Ryoku notes vault", "Rewrite in place", "Rewrite in my voice", "Assignment mode", "Focus guard"} {
		s.toggle(name)
	}
	s.await("Phase 16 switches reach the daemon", func() bool {
		o := s.daemonOni()
		return o.Recall.Enabled && o.Recall.Vault && o.Rewrite.Enabled && o.Rewrite.Voice && o.Assignment.Enabled && o.Feeds.Guard
	})
	after := s.daemonOni()
	if after.Master != before.Master || after.Voice != before.Voice || after.Hands != before.Hands || after.Inspect != before.Inspect || after.ReflexSettings != before.ReflexSettings {
		t.Fatalf("writing settings changed an unrelated feature: before %+v, after %+v", before, after)
	}
}

// A switch has one editable home, even when its dependency is explained on
// another page. Navigation and action plaques are deliberately not settings.
func TestOniSettingsHaveNoDuplicateEditableControls(t *testing.T) {
	homes := make(map[string]string)
	for _, section := range []string{"oni", "oni-voice", "oni-screen", "oni-commands", "oni-writing", "oni-privacy", "hands", "parts"} {
		s := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabSettings, SettingsSection: section, Session: demo.FeatureOni})
		s.open("settings/" + section)
		labels := make(map[string]bool)
		at := f32.Pt(1100, 500)
		for range 100 {
			nodes := s.nodes()
			visible := make(map[string]bool)
			for _, node := range nodes {
				if node.Desc.Class != semantic.Switch || !node.Desc.Bounds.Overlaps(image.Rectangle{Max: s.size}) {
					continue
				}
				label := node.Desc.Label
				if label == "" {
					for _, parent := range nodes {
						if parent.ID == node.ParentID {
							label = parent.Desc.Label
							break
						}
					}
				}
				if label == "" {
					t.Fatal("an editable Settings switch has no semantic label")
				}
				if visible[label] {
					t.Fatalf("%q is editable twice on %s", label, section)
				}
				visible[label], labels[label] = true, true
			}
			s.queue(pointer.Event{Kind: pointer.Scroll, Position: at, Scroll: f32.Pt(0, 100)})
			s.settle()
		}
		for label := range labels {
			if previous, duplicate := homes[label]; duplicate {
				t.Fatalf("%q is editable on both %s and %s", label, previous, section)
			}
			homes[label] = section
		}
	}
	for _, required := range []string{"Talk to Oni", "Let Oni look at a window", "Quick commands", "Rewrite in place", "Focus guard", "Serve Oni's tools to other programs", "Hands in a browser"} {
		if _, ok := homes[required]; !ok {
			t.Fatalf("the audit did not reach %q", required)
		}
	}
}

func TestOniRewriteReviewRequiresExplicitApplyAndOffersUndo(t *testing.T) {
	s := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabSettings, SettingsSection: "oni-writing", Session: demo.FeatureOniPhase16})
	s.open("settings/oni-writing")
	s.settingsReveal("Review rewrite")
	if s.mentions("Text to rewrite") || s.mentions("Undo this rewrite") {
		t.Fatal("opening Settings captured or applied a rewrite")
	}
	s.click("Review rewrite")
	s.await("the exact text is available to review", func() bool { return s.mentions("Text to rewrite") && s.mentions("Rewrite destination") })
	if s.mentions("Undo this rewrite") {
		t.Fatal("reviewing applied a rewrite without approval")
	}
	s.click("Apply reviewed rewrite")
	s.await("the applied rewrite offers undo", func() bool { return s.mentions("Undo this rewrite") })
	s.click("Undo this rewrite")
	s.await("the original was restored", func() bool { return s.mentions("undone") && !s.mentions("Undo this rewrite") })
}
