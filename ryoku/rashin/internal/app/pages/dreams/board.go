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

// every dream as a row under a sentence of how they stand: the ones under way, then the ones done
type board struct {
	header  look.PaneHeader
	scroll  look.Scroller
	sheet   look.Sheet
	reveal  look.Reveal
	tablets map[string]*kit.Tablet
	// the board's figures run up to their values as it opens
	counts [3]look.CountUp
	// the rows the scroll list held last frame, for how far the board is read
	rows int
}

type boardItem struct {
	kind  int
	head  look.Head
	dream protocol.Team
}

const (
	itemLead = iota
	itemHead
	itemDream
	itemFoot
)

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
	for _, dream := range dreams {
		if b.tablet(dream.ID).Clicked(gtx) {
			openDream(e, dream.ID)
		}
	}
	items := boardItems(dreams)
	b.rows = len(items)
	page := look.PageFor(protocol.TabDreams)
	return b.sheet.Layout(gtx, th, page, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				content := look.PaneHeaderContent{
					Title: i18n.T("Dreams"),
					// starting a dream is the board's one gold
					Actions:  []look.HeaderAction{{Icon: look.Icon.Add, Label: i18n.T("New dream"), Gold: true}},
					Scrolled: b.scroll.Position.First > 0 || b.scroll.Position.Offset > 0,
				}
				dimensions := b.header.Layout(gtx, th, e.Overlays, content)
				if _, ok := b.header.ActionClicked(); ok {
					p.New(e)
				}
				return dimensions
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return b.scroll.Layout(gtx, th, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					return look.Centred(gtx, e.Class, look.BoardWidth, func(gtx layout.Context) layout.Dimensions {
						return b.reveal.Group(gtx, th, index, func(gtx layout.Context) layout.Dimensions {
							return b.layoutItem(gtx, e, snapshot, dreams, items[index], page)
						})
					})
				})
			}),
		)
	})
}

// the lead, the dreams under way and the ones done under a ruled head each, and the sheet's foot
func boardItems(dreams []protocol.Team) []boardItem {
	items := []boardItem{{kind: itemLead}}
	done := 0
	for _, dream := range dreams {
		if finished(dream) {
			done++
		}
	}
	groups := 0
	if done < len(dreams) {
		groups++
	}
	if done > 0 {
		groups++
	}
	number := 0
	for index, dream := range dreams {
		// ordered puts the open dreams first, so each group starts where the other ends
		if index == 0 || finished(dream) != finished(dreams[index-1]) {
			number++
			title := i18n.T("Under way")
			if finished(dream) {
				title = i18n.T("Done")
			}
			items = append(items, boardItem{kind: itemHead, head: look.Head{Title: title, Number: number, Of: groups}})
		}
		items = append(items, boardItem{kind: itemDream, dream: dream})
	}
	return append(items, boardItem{kind: itemFoot})
}

func (b *board) layoutItem(gtx layout.Context, e *env.Env, snapshot state.Snapshot, dreams []protocol.Team, item boardItem, page look.SheetPage) layout.Dimensions {
	th := e.Theme
	switch item.kind {
	case itemLead:
		return b.layoutLead(gtx, th, snapshot, dreams)
	case itemHead:
		return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S3}.Layout(gtx, look.SectionHead(th, item.head))
	case itemFoot:
		return layout.Inset{Top: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.sheet.Colophon(gtx, th, page)
		})
	}
	return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return b.layoutRow(gtx, th, snapshot, item.dream)
	})
}

// how the dreams stand in a sentence, what a dream is, and the board's figures
func (b *board) layoutLead(gtx layout.Context, th *look.Theme, snapshot state.Snapshot, dreams []protocol.Team) layout.Dimensions {
	open, needs := standingCounts(snapshot, dreams)
	specs := []look.Spec{
		{Label: i18n.T("Under way"), Value: kit.Pad2(b.counts[0].Value(gtx, th, open))},
		{Label: i18n.T("Need you"), Value: kit.Pad2(b.counts[1].Value(gtx, th, needs)), Lit: needs > 0},
		{Label: i18n.T("Done"), Value: kit.Pad2(b.counts[2].Value(gtx, th, len(dreams)-open))},
	}
	return layout.Inset{Top: th.Sp.S5, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Monument(gtx, th, boardSentence(open, needs, len(dreams)), boardProof(dreams))
			}),
		}
		// a board with no dream has nothing to count
		if len(dreams) > 0 {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SpecStrip(gtx, th, specs) }),
			)
		}
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx, children...)
	})
}

// a dream's row: its name and what its manager last said, its phase and its team, and how far it is along its foot
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
	if !finished(dream) {
		content.Progress, _ = progress(snapshot, dream)
	}
	return b.tablet(dream.ID).Layout(gtx, th.Theme, content, false, false)
}

// how many dreams are under way, and how many of those wait on the user
func standingCounts(snapshot state.Snapshot, dreams []protocol.Team) (open, needs int) {
	for _, dream := range dreams {
		if finished(dream) {
			continue
		}
		open++
		if _, tone, _ := standing(snapshot, dream); tone == palette.Attention {
			needs++
		}
	}
	return open, needs
}

func boardSentence(open, needs, all int) string {
	switch {
	case all == 0:
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
