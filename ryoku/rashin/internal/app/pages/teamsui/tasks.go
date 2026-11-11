package teamsui

import (
	"sort"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// where a task stands on the Work board and in its list, in the order the list shows them
type Group int

const (
	GroupNeeds Group = iota
	GroupWorking
	GroupQueued
	GroupDone
	GroupHalted
	GroupCount
)

func (g Group) Label() string {
	switch g {
	case GroupNeeds:
		return i18n.T("Needs you")
	case GroupWorking:
		return i18n.T("Working")
	case GroupQueued:
		return i18n.T("Queued")
	case GroupDone:
		return i18n.T("Done")
	}
	return i18n.T("Blocked and stopped")
}

func GroupOf(task protocol.TeamTask) Group {
	switch task.Status {
	case protocol.TaskNeedsYou:
		return GroupNeeds
	case protocol.TaskWorking:
		return GroupWorking
	case protocol.TaskReady:
		return GroupQueued
	case protocol.TaskDone:
		return GroupDone
	}
	return GroupHalted
}

// the top-level tasks by group; pieces a lead handed out stand on their
// task's page, not beside it, and team "" keeps every team's
func GroupTasks(snapshot state.Snapshot, team string) [GroupCount][]protocol.TeamTask {
	var groups [GroupCount][]protocol.TeamTask
	for _, task := range snapshot.TeamTasks {
		if task.Parent != "" || team != "" && task.Team != team {
			continue
		}
		at := GroupOf(task)
		groups[at] = append(groups[at], task)
	}
	// what waits or works keeps its turn, what ended reads newest first
	for _, at := range []Group{GroupNeeds, GroupWorking, GroupQueued} {
		list := groups[at]
		sort.Slice(list, func(i, j int) bool { return Started(list[i]).Before(Started(list[j])) })
	}
	for _, at := range []Group{GroupDone, GroupHalted} {
		list := groups[at]
		sort.Slice(list, func(i, j int) bool { return Ended(list[i]).After(Ended(list[j])) })
	}
	return groups
}

// when the task started working, or was given while it waits
func Started(task protocol.TeamTask) time.Time {
	if task.Started != nil {
		return *task.Started
	}
	return task.Created
}

// when the task ended, or last changed while it has not
func Ended(task protocol.TeamTask) time.Time {
	if task.Ended != nil {
		return *task.Ended
	}
	return task.Updated
}

func Finished(task protocol.TeamTask) bool {
	return task.Status == protocol.TaskDone || task.Status == protocol.TaskBlocked || task.Status == protocol.TaskStopped
}
