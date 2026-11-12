package dreams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// a dream's own page: the CEO's view of where it stands, what it asks, its team, its work and what it presented
type dreamView struct {
	header look.PaneHeader
	scroll look.Scroller
	// the page shown last, so another dream starts at its top
	shown string

	cards        teamsui.NeedCards
	presentation presentationView
	signIn       teamsui.SignInActions
	crew         teamsui.CrewLive
	work         teamsui.TaskBoard
	month        teamsui.MonthSpend
	folder       kit.Plaque
}

func (v *dreamView) layout(gtx layout.Context, e *env.Env, p *Page, dream protocol.Team) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	if v.shown != dream.ID {
		v.shown, v.scroll = dream.ID, look.Scroller{}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutHeader(gtx, e, snapshot, dream) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			column := gtx
			column.Constraints.Max.X = min(gtx.Constraints.Max.X-2*gtx.Dp(look.ContentInset(e.Class)), gtx.Dp(look.BoardWidth))
			v.work.Team, v.work.Pieces, v.work.Empty = dream.ID, true, i18n.T("The manager has not handed out any work yet.")
			rows, _ := v.work.Rows(column, snapshot)
			return v.scroll.Layout(gtx, th, 2+rows, func(gtx layout.Context, index int) layout.Dimensions {
				return look.Centred(gtx, e.Class, look.BoardWidth, func(gtx layout.Context) layout.Dimensions {
					switch index {
					case 0:
						return layout.Inset{Top: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return look.Split(gtx, th,
								func(gtx layout.Context) layout.Dimensions { return v.layoutMain(gtx, e, p, snapshot, dream) },
								func(gtx layout.Context) layout.Dimensions { return v.layoutAside(gtx, e, dream) },
							)
						})
					case 1:
						return layout.Inset{Top: th.Sp.S6, Bottom: th.Sp.S2}.Layout(gtx, kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Work")))
					}
					return v.work.LayoutRow(gtx, e, index-2)
				})
			})
		}),
	)
}

func (v *dreamView) layoutHeader(gtx layout.Context, e *env.Env, snapshot state.Snapshot, dream protocol.Team) layout.Dimensions {
	facts := []look.HeaderFact{
		{Text: crewWord(dream)},
		{Text: i18n.Tf("Started %s", look.Ago(gtx.Now, dream.Dream.Started))},
		{Text: paths.Short(dream.Folder), Kind: look.FactLink},
	}
	content := look.PaneHeaderContent{
		Title: dream.Name, Facts: facts,
		Scrolled: v.scroll.Position.First > 0 || v.scroll.Position.Offset > 0,
	}
	content.Claim, content.ClaimTone, _ = standing(snapshot, dream)
	resume := dream.Paused || stalled(snapshot, dream)
	if !finished(dream) {
		action := look.HeaderAction{Icon: look.Icon.Pause, Label: i18n.T("Pause"), Description: i18n.T("Pause the dream: its team stops until you resume it")}
		if resume {
			action = look.HeaderAction{Icon: look.Icon.Play, Label: i18n.T("Resume"), Description: i18n.T("Let the manager go on with the dream")}
		}
		content.Actions = []look.HeaderAction{action}
	}
	dimensions := v.header.Layout(gtx, e.Theme, e.Overlays, content)
	if _, ok := v.header.ActionClicked(); ok && !finished(dream) {
		setPaused(e, dream, !resume)
	}
	if index, ok := v.header.LinkClicked(); ok && index == len(facts)-1 {
		e.OpenFolder(dream.Folder)
	}
	return dimensions
}

// what the CEO reads and answers, top to bottom: the dream, where it stands, its sign-in, its presentation, its questions, its team
func (v *dreamView) layoutMain(gtx layout.Context, e *env.Env, p *Page, snapshot state.Snapshot, dream protocol.Team) layout.Dimensions {
	th := e.Theme
	return stack(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return look.Monument(gtx, th, dream.Name, dream.Dream.Statement)
		},
		gap(th.Sp.S4),
		phaseStrip(th, dream),
		gap(th.Sp.S6),
		standingSection(th, gtx.Now, snapshot, dream),
		v.signInSection(gtx, e, snapshot, dream),
		v.presentation.section(gtx, e, p, dream),
		v.questionsSection(gtx, e, snapshot, dream),
		section(th, i18n.T("Team"), func(gtx layout.Context) layout.Dimensions {
			style := teamsui.FullCrew(th)
			style.Head = teamsui.CrewIdentity(th, style, i18n.T("You"), i18n.T("CEO"), false)
			return teamsui.CrewChart(gtx, th, snapshot, dream.Members, v.crew.Of(dream.ID), style)
		}),
	)
}

// the month in tokens and finished tasks, and the dream's folder
func (v *dreamView) layoutAside(gtx layout.Context, e *env.Env, dream protocol.Team) layout.Dimensions {
	th := e.Theme
	v.month.Team = dream.ID
	v.month.Want(e, gtx.Now)
	loaded, spend := v.month.Loaded(), v.month.Spend()
	tokens := teamsui.TokensTrend(gtx.Now, spend, i18n.T("Tokens"))
	done := teamsui.FinishedTrend(gtx.Now, spend, i18n.T("Finished tasks"))
	v.folder.Kind, v.folder.Small, v.folder.Label = kit.PlaqueTonal, true, i18n.T("Open the folder")
	if v.folder.Clicked(gtx) {
		e.OpenFolder(dream.Folder)
	}
	return stack(gtx,
		section(th, i18n.T("Over time"), func(gtx layout.Context) layout.Dimensions {
			return stack(gtx,
				budgetLine(th, dream),
				gap(th.Sp.S5),
				func(gtx layout.Context) layout.Dimensions {
					return look.HoldUntil(gtx, loaded, func(gtx layout.Context) layout.Dimensions { return tokens.Layout(gtx, th) })
				},
				func(gtx layout.Context) layout.Dimensions {
					return look.HoldUntil(gtx, loaded, func(gtx layout.Context) layout.Dimensions { return done.Layout(gtx, th) })
				},
			)
		}),
		section(th, i18n.T("Files"), func(gtx layout.Context) layout.Dimensions {
			return stack(gtx,
				look.Paragraph(th, th.T.Mono, th.R.Fg2, paths.Short(dream.Folder)),
				gap(th.Sp.S1),
				look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("Everything the team makes stays in this folder.")),
				gap(th.Sp.S3),
				func(gtx layout.Context) layout.Dimensions { return v.folder.Layout(gtx, th.Theme) },
			)
		}),
	)
}

// an unlimited dream says what it spent, a capped one shows its meter
func budgetLine(th *look.Theme, dream protocol.Team) layout.Widget {
	if dream.Budget.MonthTokens == 0 {
		return look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.Tf("No limit. %s tokens this month.", teamsui.Tokens(dream.Stats.MonthTokens)))
	}
	return teamsui.MonthMeter(th, dream.Stats.MonthTokens, dream.Budget.MonthTokens, dream.Budget.WarnPercent)
}

// the harness the manager runs on signs in for each dream on its own, from scratch or from a team's setup
func (v *dreamView) signInSection(gtx layout.Context, e *env.Env, snapshot state.Snapshot, dream protocol.Team) layout.Widget {
	th := e.Theme
	lead, at := teamsui.Lead(dream.Members)
	if at < 0 || finished(dream) {
		return nil
	}
	harness, ok := teamsui.MemberHarness(snapshot, lead)
	if !ok || !harness.Installed || harness.Teams == nil {
		return nil
	}
	task, running := standingTask(snapshot, dream)
	waiting := running && task.Status == protocol.TaskBlocked && task.SignIn != ""
	if teamsui.SignedIn(harness, dream.ID) {
		if !waiting {
			return nil
		}
		return section(th, i18n.T("Sign-in"), look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("Signed in. The manager is starting.")))
	}
	return section(th, i18n.T("Sign-in"), func(gtx layout.Context) layout.Dimensions {
		return stack(gtx,
			look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.Tf("%s runs the manager and signs in for each dream on its own. Sign it in, or start from the setup of a team that is signed in.", harness.Name)),
			gap(th.Sp.S3),
			func(gtx layout.Context) layout.Dimensions {
				return v.signIn.Row(gtx, e, harness, dream, harness.Name, gtx.Now)
			},
		)
	})
}

// what waits on the CEO in this dream, the presentation aside, and the manager's sign-in, which has its own section
func (v *dreamView) questionsSection(gtx layout.Context, e *env.Env, snapshot state.Snapshot, dream protocol.Team) layout.Widget {
	th := e.Theme
	var needs []teamsui.Need
	for _, need := range teamsui.Needs(snapshot) {
		switch {
		case need.Approval != nil && need.Approval.Team == dream.ID && !presentationCard(dream, *need.Approval):
			needs = append(needs, need)
		case need.Task != nil && need.Task.Team == dream.ID && need.Task.SignIn == "":
			needs = append(needs, need)
		}
	}
	v.cards.Task = dream.Dream.Task
	shown := v.cards.Shown(needs)
	if len(shown) == 0 {
		return nil
	}
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(look.SectionTitle(th, i18n.T("Questions for you"), v.cards.Open(shown))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		}
		for _, need := range shown {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return v.cards.Layout(gtx, e, need)
				})
			}))
		}
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
