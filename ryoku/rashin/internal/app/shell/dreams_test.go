package shell

import (
	"image"

	"gioui.org/f32"
	"gioui.org/io/pointer"
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

// "Use the setup of" opens the setup for the new dream, and once it is done the manager starts without Resume
func TestADreamStartsOnceItsSetupIsCopied(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1400))
	stage.open("dreams")
	stage.click("New dream")
	stage.click("Your dream")
	statement := "Make a tea timer that rings on my phone."
	stage.typeText(statement)
	stage.scrollDialog("Your dream")
	stage.click("Sign in when it starts")
	stage.click("Use the setup of Mail desk")
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
	stage.await("the setup terminal opens with Mail desk's config", func() bool {
		home, _ := stage.harness("codex").Teams.HomeOf(made.ID)
		return home.Opening && len(stage.argv) > 0
	})
	argv := stage.argv[len(stage.argv)-1]
	home, _ := stage.harness("codex").Teams.HomeOf(made.ID)
	at := slices.Index(argv, home.Home)
	if at < 0 || at+1 >= len(argv) || argv[len(argv)-1] != "sh" {
		t.Fatalf("the setup was started as %q", argv)
	}
	if _, ok := stage.named("Not signed in"); !ok {
		t.Fatalf("the header does not say the dream waits for its sign-in; there is %v", stage.names())
	}
	// the terminal's command ends well, as the setup shell exiting 0 does
	stage.shell.environment().Call(protocol.MHarnessesTeamsDone, protocol.HarnessTeamsDoneParams{Token: argv[at+1]}, new(protocol.HarnessTeamsDoneResult), func(error) {})
	stage.await("the manager starts by itself", func() bool {
		dream := dreamOf(stage, made.ID)
		task := stage.runtime.store.Snapshot().TeamTasks[dream.Dream.Task]
		return task.Status == protocol.TaskWorking
	})
	if _, ok := stage.named("Not signed in"); ok {
		t.Fatal("the header still says Not signed in after the setup finished")
	}
	if _, ok := stage.named("Resume"); ok {
		t.Fatal("the page asks for Resume although the manager started")
	}
}

// a dream blocked on a sign-in that has since been made reads as starting, never as not signed in
func TestTheDreamHeaderFollowsTheSignIn(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1400))
	stage.open("dreams")
	stage.click("New dream")
	stage.click("Your dream")
	stage.typeText("Make a plant watering reminder.")
	stage.click("Start")
	stage.await("the dream waits for its sign-in", func() bool {
		_, ok := stage.named("Not signed in")
		return ok
	})
	stage.click("Sign Codex in for Make a plant watering reminder.")
	argv := stage.argv[len(stage.argv)-1]
	token := argv[slices.Index(argv, "codex")-1]
	stage.shell.environment().Call(protocol.MHarnessesTeamsDone, protocol.HarnessTeamsDoneParams{Token: token}, new(protocol.HarnessTeamsDoneResult), func(error) {})
	stage.await("the header leaves Not signed in", func() bool {
		_, ok := stage.named("Not signed in")
		return !ok
	})
}

// scroll wheels the dialog whose field is named by field down to its foot
func (s *stage) scrollDialog(field string) {
	s.t.Helper()
	node, ok := s.named(field)
	if !ok {
		s.t.Fatalf("nothing on screen is called %q", field)
	}
	at := centre(node.Desc.Bounds)
	position := f32.Pt(float32(at.X), float32(at.Y))
	for range 8 {
		s.queue(pointer.Event{Kind: pointer.Scroll, Position: position, Scroll: f32.Pt(0, 200)})
	}
	s.settle()
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

// a dream that stopped on its sign-in and went on as the same task has finished nothing yet
func TestAFreshDreamHasFinishedNothing(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1400))
	stage.open("dreams")
	stage.click("New dream")
	stage.click("Your dream")
	stage.typeText("Make a plant watering reminder.")
	stage.click("Start")
	stage.await("the dream waits for its sign-in", func() bool {
		_, ok := stage.named("Not signed in")
		return ok
	})
	stage.click("Sign Codex in for Make a plant watering reminder.")
	argv := stage.argv[len(stage.argv)-1]
	stage.shell.environment().Call(protocol.MHarnessesTeamsDone, protocol.HarnessTeamsDoneParams{Token: argv[slices.Index(argv, "codex")-1]}, new(protocol.HarnessTeamsDoneResult), func(error) {})
	stage.await("the manager works", func() bool {
		_, ok := stage.named("Working")
		return ok
	})
	if _, ok := stage.named("No task has finished this month yet."); !ok {
		t.Fatalf("a fresh dream counts a finished task; there is %v", stage.names())
	}
}

// under the presentation, who built it and on what, from the tasks' own records
func TestThePresentationNamesWhoBuiltIt(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 2400))
	stage.open("dreams/dream-receipts")
	for _, name := range []string{"Who built it", "Developer", "Codex", "gpt-5.6", "Claude Code", "sonnet"} {
		if _, ok := stage.named(name); !ok {
			t.Errorf("the presentation does not show %q; there is %v", name, stage.names())
		}
	}
}
