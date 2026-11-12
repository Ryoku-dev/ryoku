package dreams

import (
	"slices"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the phases as words: done in fg2, the current one in fg, the ones to come in fg3
func phaseStrip(th *look.Theme, dream protocol.Team) layout.Widget {
	current := slices.Index(protocol.DreamPhases, dream.Dream.Phase)
	return func(gtx layout.Context) layout.Dimensions {
		return look.Flow{Gap: 21, LineGap: 8}.Layout(gtx, len(protocol.DreamPhases), func(gtx layout.Context, index int) layout.Dimensions {
			ink := th.R.Fg3
			switch {
			case index == current:
				ink = th.R.Fg
			case index < current:
				ink = th.R.Fg2
			}
			return kit.Text(th.Theme, th.T.Label, ink, phaseWord(protocol.DreamPhases[index]))(gtx)
		})
	}
}

// the manager's own account of where the dream stands, how far it truly is, and what the manager is held on
func standingSection(th *look.Theme, now time.Time, snapshot state.Snapshot, dream protocol.Team) layout.Widget {
	summary := look.Paragraph(th, th.T.Proof, th.R.Fg, dream.Dream.Summary)
	if dream.Dream.Summary == "" {
		summary = look.Paragraph(th, th.T.Body, th.R.Fg3, i18n.T("The manager has not said where it stands yet."))
	}
	// the bar rests on the phase and the pieces the daemon saw finish, never on the manager's own estimate
	share, caption := progress(snapshot, dream)
	widgets := []layout.Widget{
		summary,
		gap(th.Sp.S3),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return kit.BladeMeter{Fraction: share}.Layout(gtx, th.Theme)
		},
		gap(th.Sp.S2),
		look.Paragraph(th, th.T.Caption, th.R.Fg2, caption),
	}
	if held := heldSentence(now, snapshot, dream); held != "" {
		widgets = append(widgets, gap(th.Sp.S2), look.Paragraph(th, th.T.Caption, th.R.Fg2, held))
	}
	return section(th, i18n.T("Where it stands"), func(gtx layout.Context) layout.Dimensions {
		return stack(gtx, widgets...)
	})
}

// what the manager is doing or held on, in a sentence
func heldSentence(now time.Time, snapshot state.Snapshot, dream protocol.Team) string {
	task, _ := standingTask(snapshot, dream)
	switch {
	case dream.Dream.Done != nil:
		return i18n.Tf("You accepted it %s.", look.Ago(now, *dream.Dream.Done))
	case finished(dream):
		return i18n.T("You accepted it.")
	case dream.Paused:
		return i18n.T("Paused. Resume lets the team go on.")
	case stalled(snapshot, dream):
		return i18n.T("The manager is not running. Resume starts it again.")
	case dream.Dream.Phase == protocol.DreamPresenting:
		return i18n.T("The manager presented the dream and waits for your answer.")
	case task.Waiting == protocol.WaitingOnYou || task.Status == protocol.TaskNeedsYou:
		return i18n.T("The manager waits for your answer to its question.")
	case task.Waiting == protocol.WaitingOnMembers:
		return i18n.Tf("The manager waits on its team: %s.", working(snapshot, dream))
	case task.Status == protocol.TaskWorking:
		return i18n.T("The manager is at work.")
	}
	return ""
}

// how many of the hires are at work, said whole
func working(snapshot state.Snapshot, dream protocol.Team) string {
	count := 0
	for _, task := range snapshot.TeamTasks {
		if task.Team == dream.ID && task.ID != dream.Dream.Task && task.Status == protocol.TaskWorking {
			count++
		}
	}
	switch count {
	case 0:
		return i18n.T("nobody is at work yet")
	case 1:
		return i18n.T("one hire at work")
	}
	return i18n.Tf("%s hires at work", look.Count(count))
}
