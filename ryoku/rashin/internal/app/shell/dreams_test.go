package shell

import (
	"image"

	"slices"
	"testing"

	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

func dreamOf(s *stage, id string) protocol.Team {
	team, _ := teamsui.TeamByID(s.runtime.store.Snapshot().Teams, id)
	return team
}

func TestTheTeamsListLeavesDreamsOut(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("teams")
	for _, name := range []string{"ESP32 Wi-Fi setup page", "Receipt renamer"} {
		if slices.Contains(stage.names(), name) {
			t.Errorf("the Teams list shows the dream %q", name)
		}
	}
	stage.open("dreams")
	for _, name := range []string{"ESP32 Wi-Fi setup page", "Receipt renamer"} {
		if !slices.Contains(stage.names(), name) {
			t.Errorf("Dreams does not show %q", name)
		}
	}
}

func TestADreamStartsFromTheNewDreamDialog(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("dreams")
	stage.click("New dream")
	statement := "Make a tea timer that rings on my phone."
	stage.click("Your dream")
	stage.typeText(statement)
	stage.click("Start")
	var made protocol.Team
	stage.await("dream_create made the dream", func() bool {
		for _, team := range stage.runtime.store.Snapshot().Teams {
			if team.Dream != nil && team.Dream.Statement == statement {
				made = team
				return true
			}
		}
		return false
	})
	if made.Approvals != protocol.ApprovalsAuto || made.Budget.MonthTokens != 0 {
		t.Errorf("the dream runs with approvals %q and a cap of %d, want auto and none", made.Approvals, made.Budget.MonthTokens)
	}
	stage.await("the dream's page opened", func() bool {
		return stage.shell.router.Tab == protocol.TabDreams && stage.shell.router.Current().Team == made.ID
	})
}

func TestADreamQuestionIsAnsweredOnItsPage(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1400))
	stage.open("dreams/dream-esp32")
	// a choice row's area runs on under the card, so it is pressed where its words stand
	choice, ok := stage.named("Ryoku's dark look")
	if !ok {
		t.Fatalf("the question shows no choice; there is %v", stage.names())
	}
	stage.clickAt(choice.Desc.Bounds.Min.Add(image.Pt(24, 12)))
	stage.await("the question was answered", func() bool {
		for _, approval := range stage.runtime.store.Snapshot().Approvals {
			if approval.ID == "approval-esp32-look" {
				return approval.Status != protocol.ApprovalPending && approval.Choice == "Ryoku's dark look"
			}
		}
		return false
	})
}

func TestAcceptingAPresentationEndsTheDream(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1400))
	stage.open("dreams/dream-receipts")
	stage.click("Accept")
	stage.await("the dream is done", func() bool {
		dream := dreamOf(stage, "dream-receipts")
		return dream.Dream != nil && dream.Dream.Phase == protocol.DreamDone && dream.Dream.Done != nil
	})
	if task := stage.runtime.store.Snapshot().TeamTasks["dream-receipts-run"]; task.Status != protocol.TaskDone {
		t.Errorf("the standing task is %q after Accept, want done", task.Status)
	}
}

func TestNotesSendTheDreamBackToWork(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1400))
	stage.open("dreams/dream-receipts")
	stage.click("Send notes")
	stage.click("Your notes")
	stage.typeText("Put the shop first in the name.")
	stage.click("Send")
	stage.await("the notes reached the dream", func() bool {
		dream := dreamOf(stage, "dream-receipts")
		return dream.Dream != nil && dream.Dream.Presentation.Verdict == protocol.DreamNotes &&
			dream.Dream.Presentation.Note == "Put the shop first in the name." && dream.Dream.Phase == protocol.DreamBuilding
	})
	if task := stage.runtime.store.Snapshot().TeamTasks["dream-receipts-run"]; task.Status != protocol.TaskWorking {
		t.Errorf("the manager is %q after the notes, want working", task.Status)
	}
}

func TestADreamFocusOpensDreams(t *testing.T) {
	for focus, want := range map[protocol.UIOpenParams]string{
		{Tab: protocol.TabDreams, Team: "new"}:         "draft",
		{Tab: protocol.TabDreams, Team: "dream-esp32"}: "dream-esp32",
	} {
		place, ok := focusedTeams(focus)
		got := place.Team
		if place.Draft != "" {
			got = "draft"
		}
		if !ok || place.Tab != protocol.TabDreams || got != want {
			t.Errorf("focus %+v opened %+v, want %s", focus, place, want)
		}
	}
}

// a dream starts its own manager without a separate account or setup terminal
func TestADreamStartsWithoutAccountSetup(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1400))
	stage.open("dreams")
	stage.click("New dream")
	stage.click("Your dream")
	stage.typeText("Make a plant watering reminder.")
	stage.click("Start")
	stage.await("the manager starts without sign-in", func() bool {
		for _, team := range stage.runtime.store.Snapshot().Teams {
			if team.Dream != nil && team.Dream.Statement == "Make a plant watering reminder." {
				task := stage.runtime.store.Snapshot().TeamTasks[team.Dream.Task]
				return task.Status == protocol.TaskWorking
			}
		}
		return false
	})
	if _, ok := stage.named("Not signed in"); ok {
		t.Fatal("a native dream still asks for a separate sign-in")
	}
	if len(stage.argv) != 0 {
		t.Fatalf("starting the dream opened a setup terminal: %v", stage.argv)
	}
}

// the bar rests on the phase, and inside Building on the pieces finished, never on the manager's estimate
func TestTheDreamBarFollowsThePhase(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1400))
	for route, caption := range map[string]string{
		"dreams/dream-receipts": "Phase 6 of 7: Presenting",
		"dreams/dream-esp32":    "Phase 4 of 7: Building, 1 of 3 pieces done",
	} {
		stage.open(route)
		if _, ok := stage.named(caption); !ok {
			t.Errorf("%s does not say %q; there is %v", route, caption, stage.names())
		}
	}
}

// a newly started dream has not yet finished a task
func TestAFreshDreamHasFinishedNothing(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1400))
	stage.open("dreams")
	stage.click("New dream")
	stage.click("Your dream")
	stage.typeText("Make a plant watering reminder.")
	stage.click("Start")
	stage.await("the manager works", func() bool {
		_, ok := stage.named("Working")
		return ok
	})
	if _, ok := stage.named("No task has finished this month yet."); !ok {
		t.Fatalf("a fresh dream counts a finished task; there is %v", stage.names())
	}
}

func TestThePresentationNamesWhoBuiltIt(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 2400))
	stage.open("dreams/dream-receipts")
	for _, name := range []string{"Who built it", "Developer"} {
		if _, ok := stage.named(name); !ok {
			t.Errorf("the presentation does not show %q; there is %v", name, stage.names())
		}
	}
}
