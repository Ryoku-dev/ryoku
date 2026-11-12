// Package teamsui is what every Teams page shares: how a task, a team or a
// routine stands, a team's mark, token counts, lookups, the list of what
// needs the user and the cards that answer it, the task board, the routine
// tablet and the ledger's words.
package teamsui

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// a state worth a word: the Working flakes while an agent is at work, else the tone's flake
func Claim(th *look.Theme, word string, tone palette.Tone, working bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if word == "" {
			return layout.Dimensions{}
		}
		if !working {
			return kit.Claim(th.Theme, tone, word)(gtx)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Stone(tone), word)),
		)
	}
}

// how an approval ended, or that it still waits
func ApprovalOutcome(approval protocol.Approval) (string, palette.Tone) {
	switch approval.Status {
	case protocol.ApprovalPending:
		return i18n.T("Waiting for you"), palette.Attention
	case protocol.ApprovalExpired:
		return i18n.T("Nobody answered"), palette.Paused
	case protocol.ApprovalDenied:
		return i18n.T("Denied"), palette.Paused
	}
	switch approval.Decision {
	case protocol.DecideAlways:
		return i18n.T("Always allowed"), palette.Alive
	case protocol.DecideProvide:
		return i18n.T("Added to the vault"), palette.Alive
	case protocol.DecideChoose:
		return choseLine(approval.Options, approval.Choice), palette.Alive
	}
	return i18n.T("Approved once"), palette.Alive
}

// a task's state in one word, its tone, and whether its agent is at work
func TaskStanding(task protocol.TeamTask) (word string, tone palette.Tone, working bool) {
	switch task.Status {
	case protocol.TaskReady:
		return i18n.T("Queued"), palette.Idle, false
	case protocol.TaskWorking:
		if task.Waiting == protocol.WaitingOnMembers {
			return i18n.T("Waiting on members"), palette.Becoming, true
		}
		return i18n.T("Working"), palette.Becoming, true
	case protocol.TaskNeedsYou:
		return i18n.T("Needs you"), palette.Attention, false
	case protocol.TaskDone:
		return i18n.T("Done"), palette.Alive, false
	case protocol.TaskBlocked:
		return i18n.T("Blocked"), palette.Failed, false
	case protocol.TaskStopped:
		return i18n.T("Stopped"), palette.Paused, false
	}
	return task.Status, palette.Idle, false
}

// a team's state when it has one worth a word: paused, needs you, working
func TeamStanding(team protocol.Team) (word string, tone palette.Tone, working bool) {
	switch {
	case team.Paused && team.PauseReason == protocol.PausedByBudget:
		return i18n.T("Paused at its limit"), palette.Attention, false
	case team.Paused:
		return i18n.T("Paused"), palette.Paused, false
	case team.Stats.NeedsYou > 0:
		return i18n.T("Needs you"), palette.Attention, false
	case team.Stats.Working > 0:
		return i18n.T("Working"), palette.Becoming, true
	}
	return "", palette.Idle, false
}

// a routine's state: off, failing, or on with nothing to say
func RoutineStanding(routine protocol.Routine) (word string, tone palette.Tone) {
	switch {
	case !routine.Enabled && routine.Failures >= protocol.RoutineFailuresMax:
		return i18n.T("Off after failing"), palette.Failed
	case !routine.Enabled:
		return i18n.T("Off"), palette.Paused
	case routine.LastStatus == protocol.TaskBlocked || routine.LastStatus == protocol.TaskStopped:
		return i18n.T("Last run failed"), palette.Attention
	}
	return "", palette.Idle
}

// an approval's kind in one word
func ApprovalWord(kind string) string {
	switch kind {
	case protocol.ApprovalMail:
		return i18n.T("Mail")
	case protocol.ApprovalCredential:
		return i18n.T("Credential")
	case protocol.ApprovalRequest:
		return i18n.T("Credential request")
	case protocol.ApprovalBudget:
		return i18n.T("Spending limit")
	case protocol.ApprovalTeam:
		return i18n.T("Team proposal")
	case protocol.ApprovalRestore:
		return i18n.T("Notice")
	}
	return i18n.T("Decision")
}

// the task's origin as a fact line: who started it
func Origin(snapshotTeams []protocol.Team, routines []protocol.Routine, task protocol.TeamTask) string {
	switch task.Origin {
	case protocol.TaskFromRoutine:
		for _, routine := range routines {
			if routine.ID == task.Routine {
				return i18n.Tf("From the routine %s", routine.Title)
			}
		}
		return i18n.T("From a routine")
	case protocol.TaskFromLead:
		if team, ok := TeamByID(snapshotTeams, task.Team); ok {
			for _, member := range team.Members {
				if member.Lead {
					return i18n.Tf("Handed over by %s", member.Name)
				}
			}
		}
		return i18n.T("Handed over by the lead")
	}
	return i18n.T("Asked by you")
}
