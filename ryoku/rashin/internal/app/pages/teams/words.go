package teams

import (
	"slices"
	"strings"
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

// the engines a team's members run on, each once, in the members' order
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

// each harness the engines run on, once
func engineHarnesses(engines []teamsui.Engine) []string {
	var names []string
	for _, engine := range engines {
		if !slices.Contains(names, engine.Harness) {
			names = append(names, engine.Harness)
		}
	}
	return names
}

// a team tile's line under its crew: the one engine, else how many and on what
func enginesCaption(snapshot appstate.Snapshot, members []protocol.Member) string {
	engines := teamEngines(snapshot, members)
	switch len(engines) {
	case 0:
		return ""
	case 1:
		return engines[0].Line()
	}
	names := engineHarnesses(engines)
	// engines on one harness differ by model, so the models tell them apart
	if len(names) == 1 {
		names = names[:0]
		for _, engine := range engines {
			names = append(names, engine.Short())
		}
	}
	return i18n.Tf("%d engines: %s", len(engines), strings.Join(names, ", "))
}

// the header's fact of what a team runs on: the one engine whole, else three harnesses at most, then how many more
func runsFact(snapshot appstate.Snapshot, members []protocol.Member) string {
	engines := teamEngines(snapshot, members)
	if len(engines) == 1 {
		return i18n.Tf("Runs %s", engines[0].Line())
	}
	names := engineHarnesses(engines)
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
