package shell

import (
	"gioui.org/io/semantic"
	"image"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestLearningSectionsShowSavedWorkAndOffExplanation(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabLearning})
	stage.open("learning")
	stage.await("learned skills", func() bool { _, ok := stage.named("release-notes-from-git-log"); return ok })
	stage.reveal("Project decisions")
	stage.await("decisions from the project", func() bool { _, ok := stage.named("- A branch switch or an update never loses local work"); return ok })
	stage.reveal("Rules")
	stage.await("the rules it kept", func() bool { _, ok := stage.named("no-em-dash"); return ok })
	stage.reveal("Tool reports")
	stage.await("reports kept while collection is off", func() bool {
		_, notice := stage.named("This feature is off")
		_, report := stage.named("edit #4")
		return notice && report
	})
	stage.click("Open Features")
	if stage.shell.router.Tab != protocol.TabSettings {
		t.Fatal("Open Features did not open Settings")
	}
}

func TestLearningCanOpenAndDeleteManagedSkills(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabLearning})
	stage.open("learning/skills")
	stage.await("managed skills", func() bool { _, ok := stage.named("release-notes-from-git-log"); return ok })
	stage.click("Open skill")
	stage.await("skill contents", func() bool {
		return containsLearningText(stage.names(), "Write release notes from the Git log")
	})
	stage.click("Delete skill")
	stage.await("managed skill removal", func() bool { _, ok := stage.named("release-notes-from-git-log"); return !ok })
}

func TestLearningCanRemoveOneDecisionWithoutLosingItsBank(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabLearning})
	stage.open("learning/decisions")
	const decision = "- Everything works on systemd and runit, and on Hyprland and niri"
	stage.await("stored decision", func() bool { _, ok := stage.named(decision); return ok })
	stage.click("Remove decision")
	stage.await("removed decision", func() bool { _, ok := stage.named(decision); return !ok })
	if _, ok := stage.named("hyperion"); !ok {
		t.Fatal("removing a decision removed its project bank")
	}
}

func TestLearningRuleForgeNeedsAnEnabledFeatureAndSavesAfterReview(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabLearning})
	stage.open("learning/rules")
	stage.await("rule forge", func() bool { _, ok := stage.named("Make a rule from a complaint"); return ok })
	stage.click("Make a rule from a complaint")
	stage.clickLearningEditor("Conversation ID")
	stage.typeText("agent-done")
	stage.clickLearningEditor("What went wrong?")
	stage.typeText("The answer used an em dash despite the style rule")
	stage.reveal("Propose rule")
	stage.click("Propose rule")
	stage.await("forged rule proposal", func() bool { _, ok := stage.named("no-em-dash-in-replies"); return ok })
	stage.reveal("Review and save")
	stage.click("Review and save")
	stage.reveal("Save rule")
	stage.click("Save rule")
	stage.await("saved rule", func() bool { _, ok := stage.named("no-em-dash-in-replies"); return ok })
}

func TestLearningClearsOnePrivateToolReport(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabLearning})
	stage.open("learning/reports")
	stage.await("private report", func() bool { _, ok := stage.named("edit #4"); return ok })
	stage.click("Clear report")
	stage.await("removed private report", func() bool { _, ok := stage.named("edit #4"); return !ok })
	if _, ok := stage.named("grep #3"); !ok {
		t.Fatal("clearing one report cleared another")
	}
}

func (stage *stage) clickLearningEditor(label string) {
	stage.t.Helper()
	var field image.Rectangle
	for _, node := range stage.nodes() {
		bounds := node.Desc.Bounds
		if node.Desc.Class == semantic.Editor && node.Desc.Label == label &&
			!bounds.Empty() && bounds.In(image.Rectangle{Max: stage.size}) &&
			(field.Empty() || bounds.Dx()*bounds.Dy() < field.Dx()*field.Dy()) {
			field = bounds
		}
	}
	if field.Empty() {
		stage.t.Fatalf("no visible editor called %q", label)
	}
	// the editable first line is shorter than the semantic box around the multiline field
	stage.clickAt(field.Min.Add(image.Pt(16, 12)))
}

func containsLearningText(names []string, fragment string) bool {
	for _, name := range names {
		if strings.Contains(name, fragment) {
			return true
		}
	}
	return false
}
