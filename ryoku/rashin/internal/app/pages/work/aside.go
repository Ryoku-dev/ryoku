package work

import (
	"fmt"
	"sort"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func (t *taskView) layoutAside(gtx layout.Context, e *env.Env, snapshot state.Snapshot, task protocol.TeamTask) layout.Dimensions {
	th := e.Theme
	word, _, _ := teamsui.TaskStanding(task)
	var rows []layout.Widget
	// the member's screen leads, watched while it works on it
	if screen, ok := screenOf(snapshot, task); ok {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return t.screen.layout(gtx, e, snapshot, task, screen) })
	}
	rows = append(rows,
		kit.AsideFact(th.Theme, i18n.T("Status"), word),
		kit.AsideFact(th.Theme, i18n.T("Team"), teamsui.TeamName(snapshot.Teams, task.Team)),
	)
	if member := teamsui.MemberOf(snapshot, task.Team, task.Member); member != "" {
		rows = append(rows, kit.AsideFact(th.Theme, i18n.T("Member"), member))
	}
	rows = append(rows, engineFacts(th, snapshot, task)...)
	turns := fmt.Sprint(task.Turns)
	if task.MaxTurns > 0 {
		turns = i18n.Tf("%d of %d", task.Turns, task.MaxTurns)
	}
	rows = append(rows,
		kit.AsideFact(th.Theme, i18n.T("Origin"), teamsui.Origin(snapshot.Teams, snapshot.Routines, task)),
		kit.AsideFact(th.Theme, i18n.T("Turns"), turns),
		kit.AsideFact(th.Theme, i18n.T("Tokens"), i18n.Tf("%s in, %s out",
			teamsui.Tokens(int64(task.Usage.Input)), teamsui.Tokens(int64(task.Usage.Output)))),
	)
	if task.Started != nil {
		rows = append(rows, kit.AsideFact(th.Theme, i18n.T("Started"), look.When(gtx.Now, *task.Started)))
	}
	if task.Ended != nil {
		rows = append(rows, kit.AsideFact(th.Theme, i18n.T("Finished"), look.When(gtx.Now, *task.Ended)))
	}
	if pieces := piecesOf(snapshot, task.ID); len(pieces) > 0 {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return t.layoutPieces(gtx, e, snapshot, pieces) })
	}
	if approvals := approvalsOf(snapshot, task.ID); len(approvals) > 0 {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return layoutApprovals(gtx, th, approvals) })
	}
	return kit.Aside(gtx, th.Theme, i18n.T("Details"), rows...)
}

// the pieces the lead handed out, oldest first, as they were given
func piecesOf(snapshot state.Snapshot, parent string) []protocol.TeamTask {
	var pieces []protocol.TeamTask
	for _, task := range snapshot.TeamTasks {
		if task.Parent == parent {
			pieces = append(pieces, task)
		}
	}
	sort.Slice(pieces, func(i, j int) bool { return pieces[i].Created.Before(pieces[j].Created) })
	return pieces
}

func approvalsOf(snapshot state.Snapshot, task string) []protocol.Approval {
	var approvals []protocol.Approval
	for _, approval := range snapshot.Approvals {
		if approval.Task == task {
			approvals = append(approvals, approval)
		}
	}
	return approvals
}

func (t *taskView) layoutPieces(gtx layout.Context, e *env.Env, snapshot state.Snapshot, pieces []protocol.TeamTask) layout.Dimensions {
	th := e.Theme
	if t.links == nil {
		t.links = make(map[string]*kit.Click)
	}
	children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg2, i18n.T("Pieces")))}
	for _, piece := range pieces {
		link := t.links[piece.ID]
		if link == nil {
			link = new(kit.Click)
			t.links[piece.ID] = link
		}
		if link.Clicked(gtx) {
			teamsui.OpenTask(e, piece.ID)
		}
		word, tone, _ := teamsui.TaskStanding(piece)
		ink := th.R.Fg2
		if link.Hovered() {
			ink = th.R.Fg
		}
		facts := word
		if member := teamsui.MemberOf(snapshot, piece.Team, piece.Member); member != "" {
			facts = i18n.Tf("%s · %s", member, word)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return link.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.Flake(gtx, th.Theme, tone, 1, 9) }),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return kit.Label{Style: th.T.Body, Color: ink, MaxLines: 1}.Layout(gtx, th.Theme, piece.Title)
								}),
								layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, facts)),
							)
						}),
					)
				})
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func layoutApprovals(gtx layout.Context, th *look.Theme, approvals []protocol.Approval) layout.Dimensions {
	children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg2, i18n.T("Approvals")))}
	for _, approval := range approvals {
		word, tone := teamsui.ApprovalOutcome(approval)
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Body, Color: th.R.Fg, MaxLines: 2}.Layout(gtx, th.Theme, approval.Title)
			}),
			layout.Rigid(kit.Claim(th.Theme, tone, word)),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
