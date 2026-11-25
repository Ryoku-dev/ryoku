package dreams

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
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
	sheet  look.Sheet
	reveal look.Reveal
	// the page shown last, so another dream starts at its top and rises anew
	shown string
	// the rows the scroll list held last frame, for how far the page is read
	rows int

	phases       phaseStrip
	cards        teamsui.NeedCards
	presentation presentationView
	crew         teamsui.CrewLive
	work         teamsui.TaskBoard
	month        teamsui.MonthSpend
	folder       kit.Plaque
	providers    kit.Plaque
}

func (v *dreamView) layout(gtx layout.Context, e *env.Env, p *Page, dream protocol.Team) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	if v.shown != dream.ID {
		v.shown, v.scroll = dream.ID, look.Scroller{}
		v.reveal.Reset()
	}
	main, aside := v.mainParts(gtx, e, p, snapshot, dream), v.asideParts(gtx, e, dream)
	// the parts count on through both columns, and the work under them closes the page
	of := len(main) + len(aside) + 1
	page := look.PartPage(protocol.TabDreams, 1, dream.Name, "")
	return v.sheet.Layout(gtx, th, page, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutHeader(gtx, e, snapshot, dream) }),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				column := gtx
				column.Constraints.Max.X = min(gtx.Constraints.Max.X-2*gtx.Dp(look.ContentInset(e.Class)), gtx.Dp(look.BoardWidth))
				v.work.Team, v.work.Pieces, v.work.Empty = dream.ID, true, i18n.T("The manager has not handed out any work yet.")
				rows, _ := v.work.Rows(column, snapshot)
				v.rows = 3 + rows
				return v.scroll.Layout(gtx, th, v.rows, func(gtx layout.Context, index int) layout.Dimensions {
					return look.Centred(gtx, e.Class, look.BoardWidth, func(gtx layout.Context) layout.Dimensions {
						switch index {
						case 0:
							return layout.Inset{Top: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return look.Split(gtx, th,
									func(gtx layout.Context) layout.Dimensions {
										return v.layoutMain(gtx, e, snapshot, dream, main, of)
									},
									func(gtx layout.Context) layout.Dimensions {
										return layoutParts(gtx, th, &v.reveal, aside, len(main)+1, of, len(main)+1)
									},
								)
							})
						case 1:
							return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S3}.Layout(gtx, look.SectionHead(th, look.Head{Title: i18n.T("Work"), Number: of, Of: of}))
						case v.rows - 1:
							return layout.Inset{Top: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return v.sheet.Colophon(gtx, th, page)
							})
						}
						return v.work.LayoutRow(gtx, e, index-2)
					})
				})
			}),
		)
	})
}

func (v *dreamView) layoutHeader(gtx layout.Context, e *env.Env, snapshot state.Snapshot, dream protocol.Team) layout.Dimensions {
	facts := []look.HeaderFact{{Text: crewWord(dream)}}
	if lead, at := teamsui.Lead(dream); at >= 0 {
		facts = append(facts, look.HeaderFact{Text: teamsui.MemberEngine(snapshot, lead).Place()})
	}
	facts = append(facts,
		look.HeaderFact{Text: i18n.Tf("Started %s", look.Ago(gtx.Now, dream.Dream.Started))},
		look.HeaderFact{Text: paths.Short(dream.Folder), Kind: look.FactLink},
	)
	content := look.PaneHeaderContent{
		Title: dream.Name, Facts: facts,
		Scrolled: v.scroll.Position.First > 0 || v.scroll.Position.Offset > 0,
	}
	content.Claim, content.ClaimTone, _ = standing(snapshot, dream)
	resume := dream.Paused || stalled(snapshot, dream)
	if !finished(dream) {
		action := look.HeaderAction{Icon: look.Icon.Pause, Label: i18n.T("Pause")}
		if resume {
			action = look.HeaderAction{Icon: look.Icon.Play, Label: i18n.T("Resume")}
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

// what the CEO reads first: how the dream stands in a sentence over the dream in their own words, then how far it is
func (v *dreamView) layoutMain(gtx layout.Context, e *env.Env, snapshot state.Snapshot, dream protocol.Team, parts []part, of int) layout.Dimensions {
	th := e.Theme
	return stack(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return v.reveal.Group(gtx, th, 0, func(gtx layout.Context) layout.Dimensions {
				return stack(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return look.Monument(gtx, th, heldSentence(gtx.Now, snapshot, dream), dream.Dream.Statement)
					},
					gap(th.Sp.S5),
					func(gtx layout.Context) layout.Dimensions { return v.phases.layout(gtx, th, snapshot, dream) },
					gap(th.Sp.S6),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions { return layoutParts(gtx, th, &v.reveal, parts, 1, of, 1) },
	)
}

// what the CEO reads and answers, top to bottom: where it stands, its presentation, its questions, its team
func (v *dreamView) mainParts(gtx layout.Context, e *env.Env, p *Page, snapshot state.Snapshot, dream protocol.Team) []part {
	th := e.Theme
	parts := []part{{title: i18n.T("Where it stands"), body: v.standingBody(gtx, e, snapshot, dream)}}
	// a presentation waiting for its answer holds the page's one gold, so no question's card takes it
	_, v.cards.Quiet = awaiting(dream)
	if body := v.presentation.body(gtx, e, p, dream); body != nil {
		parts = append(parts, part{title: i18n.T("Presentation"), body: body})
	}
	if body := v.questionsBody(e, snapshot, dream); body != nil {
		parts = append(parts, part{title: i18n.T("Questions for you"), body: body})
	}
	return append(parts, part{title: i18n.T("Team"), body: func(gtx layout.Context) layout.Dimensions {
		style := teamsui.FullCrew(th)
		style.Head = teamsui.CrewIdentity(th, style, i18n.T("You"), i18n.T("CEO"), false)
		return teamsui.CrewChart(gtx, th, snapshot, dream, v.crew.Of(dream.ID), style)
	}})
}

// the manager's own account of where the dream stands, and the way out when nothing can run its model
func (v *dreamView) standingBody(gtx layout.Context, e *env.Env, snapshot state.Snapshot, dream protocol.Team) layout.Widget {
	th := e.Theme
	summary := look.Paragraph(th, th.T.Proof, th.R.Fg, dream.Dream.Summary)
	if dream.Dream.Summary == "" {
		summary = look.Paragraph(th, th.T.Body, th.R.Fg3, i18n.T("The manager has not said where it stands yet."))
	}
	widgets := []layout.Widget{summary}
	if !finished(dream) && managerMissing(snapshot, dream) {
		v.providers.Kind, v.providers.Small, v.providers.Label = kit.PlaqueTonal, true, i18n.T("Open Providers")
		if v.providers.Clicked(gtx) && e.OpenSettings != nil {
			e.OpenSettings("providers")
		}
		lead, _ := teamsui.Lead(dream)
		widgets = append(widgets,
			gap(th.Sp.S3),
			kit.Claim(th.Theme, palette.Failed, i18n.T("Model cannot run")),
			gap(th.Sp.S1),
			look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.Tf("Nothing can run %s now. Start it or add its key in Providers, then resume the dream.", teamsui.MemberEngine(snapshot, lead).Place())),
			gap(th.Sp.S3),
			func(gtx layout.Context) layout.Dimensions { return v.providers.Layout(gtx, th.Theme) },
		)
	}
	return func(gtx layout.Context) layout.Dimensions { return stack(gtx, widgets...) }
}

// what waits on the CEO in this dream; the presentation is answered in its own part
func (v *dreamView) questionsBody(e *env.Env, snapshot state.Snapshot, dream protocol.Team) layout.Widget {
	th := e.Theme
	var needs []teamsui.Need
	for _, need := range teamsui.Needs(snapshot) {
		switch {
		case need.Approval != nil && need.Approval.Team == dream.ID && !presentationCard(dream, *need.Approval):
			needs = append(needs, need)
		case need.Task != nil && need.Task.Team == dream.ID:
			needs = append(needs, need)
		}
	}
	v.cards.Task = dream.Dream.Task
	shown := v.cards.Shown(needs)
	if len(shown) == 0 {
		return nil
	}
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(shown))
		for index, need := range shown {
			bottom := th.Sp.S2
			if index == len(shown)-1 {
				bottom = 0
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return v.cards.Layout(gtx, e, need)
				})
			}))
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// the month in tokens and finished tasks, and the dream's folder
func (v *dreamView) asideParts(gtx layout.Context, e *env.Env, dream protocol.Team) []part {
	th := e.Theme
	v.month.Team = dream.ID
	v.month.Want(e, gtx.Now)
	loaded, spend := v.month.Loaded(), v.month.Spend()
	tokens := teamsui.TokensTrend(gtx.Now, spend, i18n.T("Tokens"))
	done := teamsui.FinishedTrend(gtx.Now, doneSpend(e.Snapshot(), dream, spend), i18n.T("Finished tasks"))
	v.folder.Kind, v.folder.Small, v.folder.Label = kit.PlaqueTonal, true, i18n.T("Open the folder")
	if v.folder.Clicked(gtx) {
		e.OpenFolder(dream.Folder)
	}
	return []part{
		{title: i18n.T("Over time"), body: func(gtx layout.Context) layout.Dimensions {
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
		}},
		{title: i18n.T("Files"), body: func(gtx layout.Context) layout.Dimensions {
			return stack(gtx,
				look.Paragraph(th, th.T.Mono, th.R.Fg2, paths.Short(dream.Folder)),
				gap(th.Sp.S1),
				look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("Everything the team makes stays in this folder.")),
				gap(th.Sp.S3),
				func(gtx layout.Context) layout.Dimensions { return v.folder.Layout(gtx, th.Theme) },
			)
		}},
	}
}

// an unlimited dream says what it spent, a capped one shows its meter
func budgetLine(th *look.Theme, dream protocol.Team) layout.Widget {
	if dream.Budget.MonthTokens == 0 {
		return look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.Tf("No limit. %s tokens this month.", teamsui.Tokens(dream.Stats.MonthTokens)))
	}
	return teamsui.MonthMeter(th, dream.Stats.MonthTokens, dream.Budget.MonthTokens, dream.Budget.WarnPercent)
}
