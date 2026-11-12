package dreams

import (
	"slices"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a phase in one word, as the strip and the list say it
func phaseWord(phase string) string {
	switch phase {
	case protocol.DreamInterviewing:
		return i18n.T("Interviewing")
	case protocol.DreamDesigning:
		return i18n.T("Designing")
	case protocol.DreamPlanning:
		return i18n.T("Planning")
	case protocol.DreamBuilding:
		return i18n.T("Building")
	case protocol.DreamTesting:
		return i18n.T("Testing")
	case protocol.DreamPresenting:
		return i18n.T("Presenting")
	case protocol.DreamDone:
		return i18n.T("Done")
	}
	return phase
}

// the manager and how many it hired, as the list and the header count the team
func crewWord(dream protocol.Team) string {
	switch hires := len(dream.Members) - 1; hires {
	case 0:
		return i18n.T("The manager alone")
	case 1:
		return i18n.T("The manager and one hire")
	default:
		return i18n.Tf("The manager and %d hires", hires)
	}
}

func finished(dream protocol.Team) bool {
	return dream.Dream.Done != nil || dream.Dream.Phase == protocol.DreamDone
}

// the manager's standing task, which the dream is worked in
func standingTask(snapshot state.Snapshot, dream protocol.Team) (protocol.TeamTask, bool) {
	task, ok := snapshot.TeamTasks[dream.Dream.Task]
	return task, ok && dream.Dream.Task != ""
}

// the standing task is not running and nothing keeps it so on purpose, so Resume starts it again
func stalled(snapshot state.Snapshot, dream protocol.Team) bool {
	if dream.Paused || finished(dream) || dream.Dream.Phase == protocol.DreamPresenting {
		return false
	}
	task, ok := standingTask(snapshot, dream)
	if ok && task.Status == protocol.TaskBlocked && task.SignIn != "" {
		// blocked on a sign-in: its own section offers it, and the manager starts by itself once it is done
		return false
	}
	return !ok || task.Status == protocol.TaskBlocked || task.Status == protocol.TaskStopped || task.Status == protocol.TaskDone
}

// whether the dream's home of a harness is signed in now, which a task blocked earlier does not know
func signedIn(snapshot state.Snapshot, dream protocol.Team, harness string) bool {
	for _, info := range snapshot.Harnesses {
		if info.ID == harness {
			home, ok := info.Teams.HomeOf(dream.ID)
			return ok && home.SignedIn != nil
		}
	}
	return false
}

// how a dream stands in a word, its tone, and whether its manager is at work
func standing(snapshot state.Snapshot, dream protocol.Team) (word string, tone palette.Tone, working bool) {
	task, ok := standingTask(snapshot, dream)
	switch {
	case finished(dream):
		return i18n.T("Done"), palette.Alive, false
	case dream.Paused:
		return i18n.T("Paused"), palette.Paused, false
	case dream.Dream.Phase == protocol.DreamPresenting && dream.Dream.Presentation != nil && dream.Dream.Presentation.Verdict == "":
		return i18n.T("Ready for you"), palette.Attention, false
	case ok && task.Status == protocol.TaskBlocked && task.SignIn != "" && !signedIn(snapshot, dream, task.SignIn):
		return i18n.T("Not signed in"), palette.Attention, false
	case ok && task.Status == protocol.TaskBlocked && task.SignIn != "":
		// the daemon starts the manager by itself once the harness is signed in for the dream
		return i18n.T("Starting"), palette.Becoming, true
	case ok && task.Status == protocol.TaskBlocked:
		return i18n.T("Blocked"), palette.Failed, false
	case stalled(snapshot, dream):
		return i18n.T("Not running"), palette.Paused, false
	case task.Status == protocol.TaskNeedsYou || task.Waiting == protocol.WaitingOnYou:
		return i18n.T("Needs you"), palette.Attention, false
	case task.Waiting == protocol.WaitingOnMembers:
		return i18n.T("Waiting on the team"), palette.Becoming, true
	case task.Status == protocol.TaskWorking:
		return i18n.T("Working"), palette.Becoming, true
	}
	return i18n.T("Queued"), palette.Idle, false
}

// open dreams first, the newest on top, then the ones that are done
func ordered(dreams []protocol.Team) []protocol.Team {
	sorted := slices.Clone(dreams)
	slices.SortStableFunc(sorted, func(a, b protocol.Team) int {
		if finished(a) != finished(b) {
			if finished(a) {
				return 1
			}
			return -1
		}
		return b.Dream.Started.Compare(a.Dream.Started)
	})
	return sorted
}

// the presentation the user has not answered yet
func awaiting(dream protocol.Team) (*protocol.DreamPresentation, bool) {
	presentation := dream.Dream.Presentation
	return presentation, presentation != nil && presentation.Verdict == "" && !finished(dream)
}

// the Inbox card that carries the presentation, which the dream's page answers in its own section;
// the daemon asks it with exactly these two options on the standing task
func presentationCard(dream protocol.Team, approval protocol.Approval) bool {
	_, open := awaiting(dream)
	return open && approval.Team == dream.ID && approval.Task == dream.Dream.Task &&
		slices.Equal(approval.Options, []string{i18n.T("Accept"), i18n.T("Send notes")})
}
