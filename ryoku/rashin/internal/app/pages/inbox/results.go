package inbox

import (
	"image/color"
	"sort"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// reports already opened that stay on the board under the unread ones
const readReports = 5

// what came back and what works now
func (b *board) layoutResults(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	unread := teamsui.UnreadReports(snapshot)
	reports := append(append([]protocol.TeamTask(nil), unread...), readOnes(snapshot)...)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S2}.Layout(gtx, look.SectionTitle(th, i18n.T("Reports"), len(unread)))
		}),
	}
	if len(reports) == 0 {
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("Reports your teams send back land here."))))
	}
	for _, task := range reports {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return b.layoutReport(gtx, e, snapshot, task)
			})
		}))
	}
	if working := teamsui.Working(snapshot); len(working) > 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S2}.Layout(gtx, look.SectionTitle(th, i18n.T("Working now"), len(working)))
		}))
		for _, task := range working {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return b.layoutWorking(gtx, e, snapshot, task)
				})
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// the newest reports already opened
func readOnes(snapshot state.Snapshot) []protocol.TeamTask {
	var read []protocol.TeamTask
	for _, task := range snapshot.TeamTasks {
		if task.Report != nil && task.Read && task.Parent == "" {
			read = append(read, task)
		}
	}
	sort.Slice(read, func(i, j int) bool { return read[i].Report.At.After(read[j].Report.At) })
	return read[:min(len(read), readReports)]
}

func (b *board) layoutReport(gtx layout.Context, e *env.Env, snapshot state.Snapshot, task protocol.TeamTask) layout.Dimensions {
	th := e.Theme
	item := b.tile("report:" + task.ID)
	if item.clicked(gtx) {
		teamsui.OpenTask(e, task.ID)
	}
	team := teamsui.TeamName(snapshot.Teams, task.Team)
	when := look.Ago(gtx.Now, task.Report.At)
	facts := i18n.Tf("%s · %s", team, when)
	if member := teamsui.MemberOf(snapshot, task.Team, task.Member); member != "" {
		facts = i18n.Tf("%s · %s · %s", team, member, when)
	}
	// an unread report carries the done notch and reads in full ink
	notch, ink := color.NRGBA{}, th.R.Fg2
	if !task.Read {
		notch, ink = th.R.Stone(palette.Alive), th.R.Fg
	}
	return item.layout(gtx, th, notch, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
			layout.Rigid(teamsui.Mark(th, team, 32, false)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Label{Style: th.T.Row, Color: ink, MaxLines: 1}.Layout(gtx, th.Theme, reportTitle(task))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Label{Style: th.T.Caption, Color: th.R.Fg3, MaxLines: 1}.Layout(gtx, th.Theme, facts)
					}),
					layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Label{Style: th.T.Proof, Color: th.R.Fg2, MaxLines: 2}.Layout(gtx, th.Theme, teamsui.Excerpt(task.Report.Body, 3))
					}),
				)
			}),
		)
	})
}

func (b *board) layoutWorking(gtx layout.Context, e *env.Env, snapshot state.Snapshot, task protocol.TeamTask) layout.Dimensions {
	th := e.Theme
	item := b.tile("working:" + task.ID)
	if item.clicked(gtx) {
		teamsui.OpenTask(e, task.ID)
	}
	team := teamsui.TeamName(snapshot.Teams, task.Team)
	word, _, _ := teamsui.TaskStanding(task)
	facts := i18n.Tf("%s · %s", team, word)
	if member := teamsui.MemberOf(snapshot, task.Team, task.Member); member != "" {
		facts = i18n.Tf("%s on %s · %s", member, team, word)
	}
	since := teamsui.Started(task)
	return item.layout(gtx, th, color.NRGBA{}, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Working{Active: true, Size: 32}.Layout(gtx, th.Theme)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Label{Style: th.T.Row, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th.Theme, task.Title)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return kit.Label{Style: th.T.Caption, Color: th.R.Fg3, MaxLines: 1}.Layout(gtx, th.Theme, facts)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, i18n.Tf("started %s", look.Ago(gtx.Now, since)))),
		)
	})
}
