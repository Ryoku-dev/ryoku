package routines

import (
	"sort"
	"time"

	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// how many runs Coming up shows across every routine
const comingLimit = 8

// whether the routine runs by itself now: on, and its team is not paused
func live(snapshot state.Snapshot, routine protocol.Routine) bool {
	if !routine.Enabled {
		return false
	}
	team, ok := teamsui.TeamByID(snapshot.Teams, routine.Team)
	return ok && !team.Paused
}

// one run that is due, for Coming up
type run struct {
	at      time.Time
	routine protocol.Routine
}

// the next runs across every routine that runs by itself now, soonest first
func comingRuns(now time.Time, snapshot state.Snapshot, schedules *teamsui.Schedules, limit int) []run {
	var runs []run
	for _, routine := range snapshot.Routines {
		if !live(snapshot, routine) {
			continue
		}
		schedule := schedules.Parse(routine.Schedule)
		if schedule == nil {
			continue
		}
		at := now
		for range limit {
			at = schedule.Next(at)
			if at.IsZero() {
				break
			}
			runs = append(runs, run{at: at, routine: routine})
		}
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].at.Before(runs[j].at) })
	return runs[:min(len(runs), limit)]
}

// the tasks a routine made, newest first
func routineTasks(snapshot state.Snapshot, id string) []protocol.TeamTask {
	var tasks []protocol.TeamTask
	for _, task := range snapshot.TeamTasks {
		if task.Routine == id && task.Parent == "" {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Created.After(tasks[j].Created) })
	return tasks
}

func routineByID(snapshot state.Snapshot, id string) (protocol.Routine, bool) {
	for _, routine := range snapshot.Routines {
		if routine.ID == id {
			return routine, true
		}
	}
	return protocol.Routine{}, false
}
