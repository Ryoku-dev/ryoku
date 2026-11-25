package shell

import (
	"context"
	"image"
	"slices"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

func (s *stage) memories(params protocol.MemoryListParams) []protocol.Memory {
	s.t.Helper()
	var result protocol.MemoryListResult
	if err := s.runtime.client.Call(context.Background(), protocol.MMemoryList, params, &result); err != nil {
		s.t.Fatal(err)
	}
	return result.Memories
}

func (s *stage) memoryStatus(id string) string {
	s.t.Helper()
	var memory protocol.Memory
	if err := s.runtime.client.Call(context.Background(), protocol.MMemoryGet, protocol.MemoryRefParams{ID: id}, &memory); err != nil {
		return ""
	}
	return memory.Status
}

func (s *stage) team(id string) protocol.Team {
	for _, team := range s.runtime.store.Snapshot().Teams {
		if team.ID == id {
			return team
		}
	}
	return protocol.Team{}
}

func hasText(memories []protocol.Memory, text string) bool {
	return slices.ContainsFunc(memories, func(memory protocol.Memory) bool { return memory.Text == text })
}

// a team's Overview shows what it remembers and previews its brief; Open memory leads to the Memory page on the
// team's scope, where a memory is added and forgotten
func TestATeamsMemoryIsAddedAndForgottenFromItsOverview(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 3000))
	stage.open("teams/team-repo")
	stage.await("the team's memory section arrives", func() bool {
		_, ok := stage.named("Repo watch remembers three things. It last learned from the run on 6 Nov.")
		return ok
	})
	stage.click("Preview the brief")
	stage.await("the brief opens beside the Overview", func() bool {
		_, ok := stage.named("What the next run gets")
		return ok
	})
	stage.click("Close")
	scope := protocol.MemoryTeamPrefix + "team-repo"
	stage.click("Open memory")
	if current := stage.shell.router.Current(); current.Tab != protocol.TabMemory || current.MemoryScope != scope {
		t.Fatalf("Open memory went to %+v, want the Memory page on %s", current, scope)
	}
	stage.click("Add memory")
	const fact = "Releases are tagged on Thursdays"
	stage.typeText(fact)
	stage.click("Save")
	stage.await("the daemon remembers it in the team's scope", func() bool {
		return hasText(stage.memories(protocol.MemoryListParams{Scope: scope}), fact)
	})
	stage.click(fact)
	// the pointer rests on the row, so its acts stand in place of its claim
	stage.click("Forget")
	stage.await("the daemon forgets it", func() bool {
		return !hasText(stage.memories(protocol.MemoryListParams{Scope: scope}), fact)
	})
}

// the team's Settings tab saves Carry the last report through the team update
func TestCarryTheLastReportTurnsOffInATeamsSettings(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1000))
	stage.open("teams/team-repo/settings")
	if !stage.team("team-repo").TeamMemory().CarryReport {
		t.Fatal("the demo team starts without carrying its report")
	}
	stage.reveal("Carry the last report")
	stage.click("Carry the last report")
	stage.await("the daemon stores the team's memory settings", func() bool {
		memory := stage.team("team-repo").TeamMemory()
		return !memory.CarryReport && memory.Remember && memory.ReadsYou
	})
}

// a memory a team suggested waits in the Inbox as a card; Keep keeps it, and a card of several keeps them all
func TestASuggestedMemoryIsKeptFromTheInbox(t *testing.T) {
	// every need card stands at once, so the window is as tall as the Inbox gets
	stage := newStage(t, image.Pt(1440, 7000))
	stage.open("inbox")
	stage.await("the suggestions arrive as cards", func() bool {
		_, ok := stage.named("3 suggestions from Find this week's coverage of Ryoku")
		return ok
	})
	for _, name := range []string{"Most duplicate install issues trace back to the mirror list on fresh ISOs", "Edit and keep", "Keep all", "Reads Phoronix most mornings"} {
		if _, ok := stage.named(name); !ok {
			t.Fatalf("the Inbox shows no %q; on screen: %v", name, stage.names())
		}
	}
	// three cards wait for the five suggestions: one task gave three, another one, and an import one
	if count, want := stage.shell.router.Inbox, len(teamsui.Needs(stage.runtime.store.Snapshot()))+3; count != want {
		t.Fatalf("the Inbox count is %d, want %d with the three suggestion cards", count, want)
	}
	stage.click("Keep all")
	stage.await("the daemon keeps every suggestion of the card", func() bool {
		return stage.memoryStatus("m1e4a7b5") == protocol.MemoryActive &&
			stage.memoryStatus("m2f8c1d6") == protocol.MemoryActive &&
			stage.memoryStatus("m3a5e9f7") == protocol.MemoryActive
	})
	stage.await("the card folds to what was kept", func() bool {
		_, ok := stage.named("Kept three memories.")
		return ok
	})
	if stage.memoryStatus("m4b2d6a8") != protocol.MemorySuggested {
		t.Fatal("keeping one card's suggestions kept another card's too")
	}
	stage.click("Keep")
	stage.await("the daemon keeps the oldest single suggestion", func() bool {
		return stage.memoryStatus("m4b2d6a8") == protocol.MemoryActive
	})
}

// a project set to Kept here is saved at once, without the dialog's Save
func TestAProjectIsSetToKeepItsMemory(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1200))
	stage.open("projects/hyperion/overview")
	stage.click("More")
	stage.click("Project settings")
	field, ok := stage.named("Base")
	if !ok {
		t.Fatalf("the settings dialog shows no Base field; on screen: %v", stage.names())
	}
	// the group stands at the foot of the dialog's scrolling body
	at := field.Desc.Bounds.Min.Add(image.Pt(20, 10))
	for range 6 {
		if _, ok := stage.named("Kept here"); ok {
			break
		}
		stage.queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(at.X), float32(at.Y))},
			pointer.Event{Kind: pointer.Scroll, Position: f32.Pt(float32(at.X), float32(at.Y)), Scroll: f32.Pt(0, 400)})
		stage.settle()
	}
	stage.click("Kept here")
	stage.await("the daemon stores the project's memory", func() bool {
		for _, project := range stage.runtime.store.Snapshot().Projects {
			if project.Name == "hyperion" {
				return project.Settings.Memory == protocol.ProjectMemoryKept
			}
		}
		return false
	})
}
