package work

import (
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// how the board's tasks stand, counted over every team's own tasks; the pieces a lead hands out are its work
type standing struct{ working, needs, queued, finished int }

func countStanding(now time.Time, snapshot state.Snapshot) standing {
	var count standing
	for _, task := range snapshot.TeamTasks {
		if task.Parent != "" {
			continue
		}
		switch task.Status {
		case protocol.TaskWorking:
			count.working++
		case protocol.TaskNeedsYou:
			count.needs++
		case protocol.TaskReady:
			count.queued++
		case protocol.TaskDone:
			if task.Ended != nil && now.Sub(*task.Ended) < 7*24*time.Hour {
				count.finished++
			}
		}
	}
	return count
}

// the board's claim says what works, else what waits on the user; the proof says the rest, each a sentence of its own
func boardLead(now time.Time, snapshot state.Snapshot) (claim, proof string) {
	count := countStanding(now, snapshot)
	var sentences []string
	switch {
	case count.working == 1:
		claim = i18n.T("One task is working.")
	case count.working > 1:
		claim = i18n.Tf("%s tasks are working.", look.CountStart(count.working))
	case count.needs == 1:
		claim = i18n.T("One task needs you.")
	case count.needs > 1:
		claim = i18n.Tf("%s tasks need you.", look.CountStart(count.needs))
	default:
		claim = i18n.T("Nothing is working right now.")
	}
	if count.working > 0 {
		switch {
		case count.needs == 1:
			sentences = append(sentences, i18n.T("One needs you."))
		case count.needs > 1:
			sentences = append(sentences, i18n.Tf("%s need you.", look.CountStart(count.needs)))
		}
	}
	switch {
	case count.queued == 1:
		sentences = append(sentences, i18n.T("One waits for room."))
	case count.queued > 1:
		sentences = append(sentences, i18n.Tf("%s wait for room.", look.CountStart(count.queued)))
	}
	switch {
	case count.finished == 1:
		sentences = append(sentences, i18n.T("One finished this week."))
	case count.finished > 1:
		sentences = append(sentences, i18n.Tf("%s finished this week.", look.CountStart(count.finished)))
	}
	if len(sentences) == 0 {
		return claim, i18n.T("Give a team a task and it shows here while it works, then with its report.")
	}
	return claim, strings.Join(sentences, " ")
}
