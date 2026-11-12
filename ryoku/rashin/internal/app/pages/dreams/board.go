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
	"ryoku-rashin/internal/protocol"
)

// every dream as a row under a sentence of how they stand, with the one gold that starts another
type board struct {
	header  look.PaneHeader
	scroll  look.Scroller
	create  kit.Plaque
	tablets map[string]*kit.Tablet
}

func (b *board) tablet(id string) *kit.Tablet {
	if b.tablets == nil {
		b.tablets = make(map[string]*kit.Tablet)
	}
	tablet := b.tablets[id]
	if tablet == nil {
		tablet = new(kit.Tablet)
		b.tablets[id] = tablet
	}
	return tablet
}

func (b *board) layout(gtx layout.Context, e *env.Env, p *Page) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	dreams := ordered(teamsui.Dreams(snapshot.Teams))
	b.create.Kind, b.create.Label = kit.PlaqueGold, i18n.T("New dream")
	if b.create.Clicked(gtx) {
		p.New(e)
	}
	for _, dream := range dreams {
		if b.tablet(dream.ID).Clicked(gtx) {
			openDream(e, dream.ID)
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			content := look.PaneHeaderContent{Title: i18n.T("Dreams"), Scrolled: b.scroll.Position.First > 0 || b.scroll.Position.Offset > 0}
			return b.header.Layout(gtx, th, e.Overlays, content)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return b.scroll.Layout(gtx, th, 1+len(dreams), func(gtx layout.Context, index int) layout.Dimensions {
				return look.Centred(gtx, e.Class, look.BoardWidth, func(gtx layout.Context) layout.Dimensions {
					if index == 0 {
						return b.layoutLead(gtx, e, snapshot, dreams)
					}
					return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return b.layoutRow(gtx, th, snapshot, dreams[index-1])
					})
				})
			})
		}),
	)
}

// how the dreams stand in a sentence, what a dream is, and New dream
func (b *board) layoutLead(gtx layout.Context, e *env.Env, snapshot state.Snapshot, dreams []protocol.Team) layout.Dimensions {
	th := e.Theme
	return layout.Inset{Top: th.Sp.S6, Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Monument(gtx, th, boardSentence(snapshot, dreams), boardProof(dreams))
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.create.Layout(gtx, th.Theme) }),
		)
	})
}

// a dream's row: its name and what its manager last said, its phase and its team
func (b *board) layoutRow(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, dream protocol.Team) layout.Dimensions {
	_, tone, _ := standing(snapshot, dream)
	facts := dream.Dream.Summary
	if facts == "" {
		facts = dream.Dream.Statement
	}
	content := kit.TabletContent{
		Mark:  teamsui.Mark(th, dream.Name, 32, false),
		Title: dream.Name, Facts: facts, Claim: phaseWord(dream.Dream.Phase), Proof: crewWord(dream), Tone: tone, Progress: -1,
	}
	if tone != palette.Attention {
		content.Tone = palette.Idle
	}
	return b.tablet(dream.ID).Layout(gtx, th.Theme, content, false, false)
}

func boardSentence(snapshot state.Snapshot, dreams []protocol.Team) string {
	open, needs := 0, 0
	for _, dream := range dreams {
		if finished(dream) {
			continue
		}
		open++
		if _, tone, _ := standing(snapshot, dream); tone == palette.Attention {
			needs++
		}
	}
	switch {
	case len(dreams) == 0:
		return i18n.T("No dream yet.")
	case open == 0:
		return i18n.T("Every dream is done.")
	case open == 1 && needs == 1:
		return i18n.T("One dream is under way and needs you.")
	case open == 1:
		return i18n.T("One dream is under way.")
	case needs == 0:
		return i18n.Tf("%s dreams are under way.", look.CountStart(open))
	case needs == 1:
		return i18n.Tf("%s dreams are under way and one needs you.", look.CountStart(open))
	}
	return i18n.Tf("%s dreams are under way and %s need you.", look.CountStart(open), look.Count(needs))
}

func boardProof(dreams []protocol.Team) string {
	if len(dreams) == 0 {
		return i18n.T("A dream is something you want made, worked on until it is built, tested and accepted. You are its CEO: a manager asks you about the design, hires the team it needs and presents the result to you.")
	}
	return i18n.T("Dreams run without asking for confirmation. Their managers ask you only about the design and the task.")
}
