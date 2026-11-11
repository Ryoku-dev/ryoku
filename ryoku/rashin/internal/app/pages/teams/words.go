package teams

import (
	"time"

	"ryoku-i18n"

	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func membersWord(count int) string {
	if count == 1 {
		return i18n.T("1 member")
	}
	return i18n.Tf("%d members", count)
}

func tasksWord(count int) string {
	if count == 1 {
		return i18n.T("1 task")
	}
	return i18n.Tf("%d tasks", count)
}

// a member with no harness of its own runs on the one Settings names for agents
func harnessName(snapshot appstate.Snapshot, id string) string {
	if id == "" {
		return i18n.T("Default harness")
	}
	for _, harness := range snapshot.Harnesses {
		if harness.ID == id {
			return harness.Name
		}
	}
	return id
}

func modelName(model string) string {
	if model == "" {
		return i18n.T("its default model")
	}
	return model
}

// top-level tasks a team was given in the last seven days
func tasksThisWeek(snapshot appstate.Snapshot, team string, now time.Time) int {
	count := 0
	for _, task := range snapshot.TeamTasks {
		if task.Team == team && task.Parent == "" && now.Sub(task.Created) < 7*24*time.Hour {
			count++
		}
	}
	return count
}

func approvalsSentence(value string) string {
	switch value {
	case protocol.ApprovalsAuto:
		return i18n.T("Changes files without asking.")
	case protocol.ApprovalsAsk:
		return i18n.T("Asks before it changes files.")
	}
	return i18n.T("Only reads and reports. It changes no file.")
}
