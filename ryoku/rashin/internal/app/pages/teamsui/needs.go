package teamsui

import (
	"sort"
	"time"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// one thing that needs the user: a pending approval, or a task that stopped
// short and waits for them to look
type Need struct {
	Approval *protocol.Approval
	Task     *protocol.TeamTask
	At       time.Time
}

// the task a need is about: the approval's task, or the task that waits
func (need Need) taskID() string {
	if need.Approval != nil {
		return need.Approval.Task
	}
	if need.Task != nil {
		return need.Task.ID
	}
	return ""
}

// what needs the user across every team, oldest first so nothing waits behind
// what came later
func Needs(snapshot state.Snapshot) []Need {
	var needs []Need
	waiting := make(map[string]bool)
	for i := range snapshot.Approvals {
		approval := snapshot.Approvals[i]
		if approval.Status != protocol.ApprovalPending {
			continue
		}
		needs = append(needs, Need{Approval: &approval, At: approval.Created})
		waiting[approval.Task] = true
	}
	for id := range snapshot.TeamTasks {
		task := snapshot.TeamTasks[id]
		if task.Status != protocol.TaskBlocked || waiting[task.ID] {
			continue
		}
		at := task.Updated
		needs = append(needs, Need{Task: &task, At: at})
	}
	sort.Slice(needs, func(i, j int) bool { return needs[i].At.Before(needs[j].At) })
	return needs
}

// reports the user has not opened yet, newest first
func UnreadReports(snapshot state.Snapshot) []protocol.TeamTask {
	var reports []protocol.TeamTask
	for _, task := range snapshot.TeamTasks {
		if task.Report != nil && !task.Read && task.Parent == "" {
			reports = append(reports, task)
		}
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Report.At.After(reports[j].Report.At) })
	return reports
}

// tasks at work now, longest running first
func Working(snapshot state.Snapshot) []protocol.TeamTask {
	var working []protocol.TeamTask
	for _, task := range snapshot.TeamTasks {
		if task.Status == protocol.TaskWorking {
			working = append(working, task)
		}
	}
	sort.Slice(working, func(i, j int) bool { return Started(working[i]).Before(Started(working[j])) })
	return working
}

// the newest report a team handed back, of every team when team is ""; the
// pieces' reports went to their lead
func NewestReport(snapshot state.Snapshot, team string) (protocol.TeamTask, bool) {
	var newest protocol.TeamTask
	found := false
	for _, task := range snapshot.TeamTasks {
		if task.Report == nil || task.Parent != "" || team != "" && task.Team != team {
			continue
		}
		if !found || task.Report.At.After(newest.Report.At) {
			newest, found = task, true
		}
	}
	return newest, found
}

// routines that will run, soonest first: on, scheduled, and of a team that
// is not paused; of every team when team is ""
func Upcoming(snapshot state.Snapshot, team string) []protocol.Routine {
	paused := make(map[string]bool, len(snapshot.Teams))
	for _, each := range snapshot.Teams {
		paused[each.ID] = each.Paused
	}
	var runs []protocol.Routine
	for _, routine := range snapshot.Routines {
		if routine.Enabled && routine.NextRun != nil && !paused[routine.Team] && (team == "" || routine.Team == team) {
			runs = append(runs, routine)
		}
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].NextRun.Before(*runs[j].NextRun) })
	return runs
}
