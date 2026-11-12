package dreams

import (
	"slices"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// where each phase stands on the bar: building runs from its start to testing by the pieces finished
var phaseShare = map[string]float64{
	protocol.DreamInterviewing: 0.05,
	protocol.DreamDesigning:    0.15,
	protocol.DreamPlanning:     0.25,
	protocol.DreamBuilding:     0.35,
	protocol.DreamTesting:      0.80,
	protocol.DreamPresenting:   0.95,
	protocol.DreamDone:         1,
}

// the pieces the manager handed out this round: under its standing task, since the last notes
func roundPieces(snapshot state.Snapshot, dream protocol.Team) []protocol.TeamTask {
	var since time.Time
	if presentation := dream.Dream.Presentation; presentation != nil && presentation.Verdict == protocol.DreamNotes {
		since = presentation.At
	}
	var pieces []protocol.TeamTask
	for _, task := range snapshot.TeamTasks {
		if task.Team == dream.ID && task.Parent != "" && task.Parent == dream.Dream.Task && !task.Created.Before(since) {
			pieces = append(pieces, task)
		}
	}
	slices.SortFunc(pieces, func(a, b protocol.TeamTask) int { return a.Created.Compare(b.Created) })
	return pieces
}

// how far the dream is, from the phase it is in and, while building, the pieces finished of those handed out;
// the caption says what the share rests on
func progress(snapshot state.Snapshot, dream protocol.Team) (share float64, caption string) {
	phase := dream.Dream.Phase
	if finished(dream) {
		phase = protocol.DreamDone
	}
	share = phaseShare[phase]
	position := slices.Index(protocol.DreamPhases, phase) + 1
	caption = i18n.Tf("Phase %d of %d: %s", position, len(protocol.DreamPhases), phaseWord(phase))
	if phase != protocol.DreamBuilding {
		return share, caption
	}
	pieces := roundPieces(snapshot, dream)
	if len(pieces) == 0 {
		return share, i18n.Tf("Phase %d of %d: %s, no piece handed out yet", position, len(protocol.DreamPhases), phaseWord(phase))
	}
	done := 0
	for _, piece := range pieces {
		if piece.Status == protocol.TaskDone {
			done++
		}
	}
	share += (phaseShare[protocol.DreamTesting] - share) * float64(done) / float64(len(pieces))
	return share, i18n.Tf("Phase %d of %d: %s, %d of %d pieces done", position, len(protocol.DreamPhases), phaseWord(phase), done, len(pieces))
}

// the month's spend with its finished tasks counted from the tasks that really ended done,
// so an attempt that stopped on a sign-in and went on as the same task never counts
func doneSpend(snapshot state.Snapshot, dream protocol.Team, spend protocol.TeamSpend) protocol.TeamSpend {
	days := slices.Clone(spend.Days)
	for index := range days {
		days[index].Tasks, days[index].Routines = 0, 0
	}
	for _, task := range snapshot.TeamTasks {
		if task.Team != dream.ID || task.Status != protocol.TaskDone || task.Ended == nil {
			continue
		}
		day := task.Ended.In(time.Local).Format("2006-01-02")
		if index := slices.IndexFunc(days, func(each protocol.DaySpend) bool { return each.Day == day }); index >= 0 {
			days[index].Tasks++
		}
	}
	spend.Days = days
	return spend
}

// one member's work this round as the daemon recorded it: the engine each of its tasks really ran on
type builder struct {
	member  protocol.Member
	tasks   []protocol.TeamTask
	engines []ranOn
}

// a harness and the model id it was given, as the task recorded them
type ranOn struct{ harness, model string }

// everyone who worked this round, the manager first, each with the harness and model of the tasks they ran
func builders(snapshot state.Snapshot, dream protocol.Team) []builder {
	tasks := roundPieces(snapshot, dream)
	if task, ok := standingTask(snapshot, dream); ok {
		tasks = append([]protocol.TeamTask{task}, tasks...)
	}
	var list []builder
	for _, task := range tasks {
		if task.Started == nil {
			continue
		}
		at := slices.IndexFunc(list, func(each builder) bool { return each.member.ID == task.Member })
		if at < 0 {
			member, ok := memberOf(dream, task.Member)
			if !ok {
				member = protocol.Member{ID: task.Member, Name: i18n.T("A released member")}
			}
			list = append(list, builder{member: member})
			at = len(list) - 1
		}
		list[at].tasks = append(list[at].tasks, task)
		harness, model, _ := teamsui.TaskEngineIDs(snapshot, task)
		if engine := (ranOn{harness, model}); !slices.Contains(list[at].engines, engine) {
			list[at].engines = append(list[at].engines, engine)
		}
	}
	return list
}

func memberOf(dream protocol.Team, id string) (protocol.Member, bool) {
	for _, member := range dream.Members {
		if member.ID == id {
			return member, true
		}
	}
	return protocol.Member{}, false
}
