package teamsui

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// one ledger entry as a sentence: who did what to what, in plain words
func ActivitySentence(entry protocol.Activity) string {
	actor, target := ActorName(entry.Actor), entry.Target
	var words string
	switch entry.Action {
	case "team.create":
		words = i18n.Tf("%s created the team %s", actor, target)
	case "team.update":
		words = i18n.Tf("%s changed the team %s", actor, target)
	case "team.pause":
		words = i18n.Tf("%s paused %s", actor, target)
	case "team.resume":
		words = i18n.Tf("%s let %s work again", actor, target)
	case "team.delete":
		words = i18n.Tf("%s deleted the team %s", actor, target)
	case "team.export":
		words = i18n.Tf("%s exported %s", actor, target)
	case "team.import":
		words = i18n.Tf("%s imported the team %s", actor, target)
	case "task.create":
		words = i18n.Tf("%s added the task %s", actor, target)
	case "task.start":
		words = i18n.Tf("%s started working on %s", actor, target)
	case "task.report":
		words = i18n.Tf("%s reported on %s", actor, target)
	case "task.block":
		words = i18n.Tf("%s could not finish %s", actor, target)
	case "task.stop":
		words = i18n.Tf("%s stopped %s", actor, target)
	case "task.retry":
		words = i18n.Tf("%s ran %s again", actor, target)
	case "task.delegate":
		words = i18n.Tf("%s handed out %s", actor, target)
	case "approval.ask":
		words = i18n.Tf("%s asked you: %s", actor, target)
	case "approval.approve":
		words = i18n.Tf("%s approved %s", actor, target)
	case "approval.choose":
		words = i18n.Tf("%s chose “%s” for %s", actor, entry.Choice, target)
	case "approval.deny":
		words = i18n.Tf("%s said no to %s", actor, target)
	case "approval.expire":
		words = i18n.Tf("Nobody answered in time: %s", target)
	case "credential.use":
		words = i18n.Tf("%s sent %s", actor, target)
	case "credential.refuse":
		words = i18n.Tf("%s was not allowed to send %s", actor, target)
	case "credential.request":
		words = i18n.Tf("%s asked for the credential %s", actor, target)
	case "credential.add":
		words = i18n.Tf("%s added %s to the vault", actor, target)
	case "credential.rotate":
		words = i18n.Tf("%s replaced the value of %s", actor, target)
	case "credential.remove":
		words = i18n.Tf("%s removed %s from the vault", actor, target)
	case "mail.send":
		words = i18n.Tf("%s sent the mail %s", actor, target)
	case "mail.refuse":
		words = i18n.Tf("%s was not allowed to send the mail %s", actor, target)
	case "routine.create":
		words = i18n.Tf("%s created the routine %s", actor, target)
	case "routine.update":
		words = i18n.Tf("%s changed the routine %s", actor, target)
	case "routine.fire":
		words = i18n.Tf("%s started a run: %s", actor, target)
	case "routine.skip":
		words = i18n.Tf("%s skipped a run of %s", actor, target)
	case "routine.off":
		words = i18n.Tf("%s turned off the routine %s", actor, target)
	case "routine.delete":
		words = i18n.Tf("%s deleted the routine %s", actor, target)
	case "budget.warn":
		words = i18n.Tf("%s is close to its spending limit", target)
	case "budget.stop":
		words = i18n.Tf("%s reached its spending limit and paused", target)
	default:
		// an action the app doesn't know yet keeps its own name
		words = strings.Join([]string{actor, entry.Action, target}, " ")
	}
	return strings.TrimRight(strings.TrimSpace(words), ":")
}

// "you" is how the ledger names the user
func ActorName(actor string) string {
	if actor == "" || actor == protocol.InitiatorYou {
		return i18n.T("You")
	}
	return actor
}

// who started it, in a few words, for the mark's accessible name
func InitiatorWord(initiator string) string {
	switch initiator {
	case protocol.InitiatorYou:
		return i18n.T("Started by you")
	case protocol.InitiatorRoutine:
		return i18n.T("Started by a routine")
	case protocol.InitiatorAgent:
		return i18n.T("Started by an agent")
	}
	return i18n.T("Started by Rashin")
}

// a small mark for who started it: you, a routine, an agent or Rashin itself
func InitiatorMark(th *look.Theme, initiator string) layout.Widget {
	switch initiator {
	case protocol.InitiatorYou:
		return look.IconWidget(look.Icon.Member, 16, th.R.Fg3)
	case protocol.InitiatorRoutine:
		return look.IconWidget(look.Icon.Routine, 16, th.R.Fg3)
	case protocol.InitiatorAgent:
		return look.IconWidget(look.Icon.Agents, 16, th.R.Fg3)
	}
	return look.RyokuMark(14, th.R.Fg3)
}

// one ledger entry as a row: when, in a column of whenWidth, who started it,
// the sentence and its detail, then whether anyone watched and whether it failed
func LedgerRow(th *look.Theme, when string, whenWidth unit.Dp, entry protocol.Activity, detail string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			look.Cell(gtx.Dp(whenWidth), kit.Text(th.Theme, th.T.Caption, th.R.Fg3, when)),
			look.Cell(gtx.Dp(28), func(gtx layout.Context) layout.Dimensions {
				return look.Named(gtx, InitiatorWord(entry.Initiator), InitiatorMark(th, entry.Initiator))
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				lines := []layout.FlexChild{layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg, ActivitySentence(entry)))}
				if detail != "" {
					lines = append(lines,
						layout.Rigid(layout.Spacer{Height: 2}.Layout),
						layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, detail)),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lines...)
			}),
		}
		if entry.Unwatched {
			children = append(children,
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Nobody watching"))),
			)
		}
		// a step that worked needs no word, only a failure is a state
		if !entry.OK {
			children = append(children,
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(kit.Claim(th.Theme, palette.Failed, i18n.T("Failed"))),
			)
		}
		return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...)
		})
	}
}
