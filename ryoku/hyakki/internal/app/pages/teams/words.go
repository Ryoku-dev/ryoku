package teams

import (
	"slices"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/pages/teamsui"
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

// the models a team's members run on, each once, in the members' order
func teamEngines(snapshot appstate.Snapshot, members []protocol.Member) []teamsui.Engine {
	var engines []teamsui.Engine
	for _, member := range members {
		engine := teamsui.MemberEngine(snapshot, member)
		if !slices.ContainsFunc(engines, func(seen teamsui.Engine) bool { return seen.Line() == engine.Line() }) {
			engines = append(engines, engine)
		}
	}
	return engines
}

// the header's fact of what a team runs on: the one model whole with where it runs, else three names at most, then how many more
func runsFact(engines []teamsui.Engine) string {
	if len(engines) == 1 {
		return i18n.Tf("Runs %s", engines[0].Line())
	}
	// two members on one model with different options are one name here
	var names []string
	for _, engine := range engines {
		if name := engine.Short(); !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return i18n.Tf("Runs %s", names[0])
	case 2:
		return i18n.Tf("Runs %s and %s", names[0], names[1])
	case 3:
		return i18n.Tf("Runs %s, %s and %s", names[0], names[1], names[2])
	}
	return i18n.Tf("Runs %s, %s, %s and %d more", names[0], names[1], names[2], len(names)-3)
}

// a tile says what its team runs on only when that cannot run: a model at rest is not news on every tile
func stalledLine(engines []teamsui.Engine) string {
	var stalled []teamsui.Engine
	for _, engine := range engines {
		if engine.Missing {
			stalled = append(stalled, engine)
		}
	}
	switch len(stalled) {
	case 0:
		return ""
	case 1:
		return i18n.Tf("%s cannot run now", stalled[0].Short())
	}
	return i18n.Tf("%d of its models cannot run now", len(stalled))
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
