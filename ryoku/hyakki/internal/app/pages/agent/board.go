package agent

import (
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the board is about now, older sessions live in the sidebar
const recentLimit = 5

type board struct {
	header    look.PaneHeader
	scroll    look.Scroller
	sheet     look.Sheet
	rows      int
	reveal    look.Reveal
	figures   [3]look.CountUp
	composer  chat.Composer
	place     placement
	more      kit.Plaque
	temporary env.Temporary
	starting  bool
	tablets   map[string]*boardTablet
	ready     env.AgentReady
	first     boardStart
}

type boardTablet struct {
	tablet kit.Tablet
	open   kit.Plaque
}

type boardItem struct {
	title   string
	count   int
	session protocol.SessionInfo
	kind    int
}

const (
	itemLead = iota
	itemReady
	itemPlaces
	itemSection
	itemSession
	itemFolio
)

func (b *board) focus(e *env.Env) {
	b.reveal.Reset()
	b.place.prime(e)
	b.composer.Focus()
}

func (b *board) layout(gtx layout.Context, e *env.Env, page *Page) layout.Dimensions {
	snapshot := e.Snapshot()
	sessions := groupSessions(snapshot)
	items := b.items(sessions, b.ready.Ready(e))
	b.rows = len(items)
	running := len(sessions[groupWorking]) > 0
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			content := look.PaneHeaderContent{Title: i18n.T("Agents"), Scrolled: b.scroll.Position.First > 0 || b.scroll.Position.Offset > 0}
			content.Actions = []look.HeaderAction{b.temporary.Action()}
			if b.temporary.On {
				content.Facts = []look.HeaderFact{env.TemporaryMark()}
			}
			if running {
				content.Menu = []kit.MenuItem{{Icon: look.Icon.Stop, Label: i18n.T("Stop every agent")}}
			}
			dimensions := b.header.Layout(gtx, e.Theme, e.Overlays, content)
			if _, ok := b.header.ActionClicked(); ok {
				b.temporary.On = !b.temporary.On
				b.composer.Focus()
			}
			if _, ok := b.header.MenuClicked(); ok {
				for _, info := range sessions[groupWorking] {
					stopSession(e, info.ID)
				}
			}
			return dimensions
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return b.sheet.Layout(gtx, e.Theme, look.PageFor(protocol.TabAgent), func(gtx layout.Context) layout.Dimensions {
				return b.scroll.Layout(gtx, e.Theme, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
						return b.reveal.Group(gtx, e.Theme, index, func(gtx layout.Context) layout.Dimensions {
							return b.layoutItem(gtx, e, page, snapshot, sessions, items[index], index == len(items)-1)
						})
					})
				})
			})
		}),
	)
}

func (b *board) items(sessions [groupCount][]protocol.SessionInfo, ready bool) []boardItem {
	items := []boardItem{{kind: itemLead}}
	if !ready {
		items = append(items, boardItem{kind: itemReady})
	}
	items = append(items, boardItem{kind: itemPlaces})
	add := func(title string, list []protocol.SessionInfo) {
		if len(list) == 0 {
			return
		}
		items = append(items, boardItem{kind: itemSection, title: title, count: len(list)})
		for _, info := range list {
			items = append(items, boardItem{kind: itemSession, session: info})
		}
	}
	add(groupNeeds.label(), sessions[groupNeeds])
	add(groupWorking.label(), sessions[groupWorking])
	recent := append(append([]protocol.SessionInfo(nil), sessions[groupDone]...), sessions[groupIdle]...)
	add(i18n.T("Recently finished"), recent[:min(len(recent), recentLimit)])
	return append(items, boardItem{kind: itemFolio})
}

func (b *board) layoutItem(
	gtx layout.Context,
	e *env.Env,
	page *Page,
	snapshot state.Snapshot,
	sessions [groupCount][]protocol.SessionInfo,
	item boardItem,
	last bool,
) layout.Dimensions {
	th := e.Theme
	switch item.kind {
	case itemLead:
		return layout.Inset{Top: th.Sp.S5, Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.layoutLead(gtx, e, page, snapshot, sessions)
		})
	case itemReady:
		return layout.Inset{Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.ready.Layout(gtx, e)
		})
	case itemPlaces:
		return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.first.layoutPlaces(gtx, e, snapshot, b)
		})
	case itemFolio:
		return layout.Inset{Top: th.Sp.S5, Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.sheet.Colophon(gtx, th, look.PageFor(protocol.TabAgent))
		})
	case itemSection:
		return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S2}.Layout(gtx, look.SectionHead(th, look.Head{Title: item.title, Count: item.count}))
	default:
		bottom := th.Sp.S1
		if last {
			bottom = th.Sp.S5
		}
		return layout.Inset{Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.layoutSession(gtx, e, snapshot, item.session)
		})
	}
}

func (b *board) layoutLead(
	gtx layout.Context,
	e *env.Env,
	page *Page,
	snapshot state.Snapshot,
	sessions [groupCount][]protocol.SessionInfo,
) layout.Dimensions {
	th := e.Theme
	if !b.ready.Ready(e) {
		return look.Monument(gtx, th, env.NoAgentModel(), i18n.T("Get a local model in Settings, under Providers, to start an agent here."))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// the sentence stays in one ink, the rows below carry each agent's state
			return kit.Sentence(gtx, th.Theme, th.T.Sentence, th.R.Fg, []kit.Span{{Text: boardSentence(sessions)}})
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(kit.Text(th.Theme, th.T.Proof, th.R.Fg2, boardProof(gtx.Now, sessions))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.SpecStrip(gtx, th, []look.Spec{
				{Label: i18n.T("Sessions"), Value: kit.Pad2(b.figures[0].Value(gtx, th, len(sessions[groupNeeds])+len(sessions[groupWorking])+len(sessions[groupDone])+len(sessions[groupIdle])))},
				{Label: i18n.T("Need you"), Value: kit.Pad2(b.figures[1].Value(gtx, th, len(sessions[groupNeeds]))), Lit: len(sessions[groupNeeds]) != 0},
				{Label: i18n.T("Working"), Value: kit.Pad2(b.figures[2].Value(gtx, th, len(sessions[groupWorking])))},
			})
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.layoutComposer(gtx, e, page, snapshot) }),
	)
}

func (b *board) layoutComposer(gtx layout.Context, e *env.Env, page *Page, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	b.more.Kind, b.more.Small, b.more.Label = kit.PlaqueQuiet, true, i18n.T("More options")
	if b.more.Clicked(gtx) {
		page.create.open(e, &b.place)
	}
	home := b.composer.FocusTag()
	b.place.model.Parts, b.place.model.Agents = env.ModelOptions|env.ModelPlan|env.ModelAsking, true
	b.place.model.FollowAgent(e)
	may := b.place.model.Approvals()
	options := chat.ComposerOptions{
		Placeholder: i18n.T("Tell an agent what to do"), CanAttach: true, Disabled: b.starting || !b.place.model.Ready(e),
		Controls: b.place.controls(e, home),
	}
	// the field itself says what a temporary session is, where it always fits
	if b.temporary.On {
		options.Placeholder = env.TemporaryAgentHint()
	}
	e.ComposerDesktop(&b.composer, &options)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	model, _ := b.place.model.Effective()
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := b.composer.Layout(gtx, th, options)
			if submission, ok := b.composer.Submitted(); ok {
				b.start(e, snapshot, submission)
			}
			return dimensions
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// the mode select carries the state, the note beside it stays in one ink
				layout.Flexed(1, kit.Text(th.Theme, th.T.Caption, th.R.Fg3, env.ApprovalsNote(env.AgentApprovals, may))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.more.Layout(gtx, th.Theme) }),
			)
		}),
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return env.ModelLine(gtx, e, model) }),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (b *board) start(e *env.Env, snapshot state.Snapshot, submission chat.Submission) {
	params, ok := b.place.params(e, details{name: worktreeName(submission.Text)})
	if !ok {
		return
	}
	params.Temporary = b.temporary.On
	b.starting = true
	var info protocol.SessionInfo
	e.Call(protocol.MSessionNew, params, &info, func(err error) {
		b.starting = false
		if err != nil {
			b.composer.SetText(submission.Text)
			e.Notify(err.Error())
			return
		}
		// the next session is an ordinary one again
		b.temporary.On = false
		open(e, info.ID)
		e.Submit(info.ID, false, submission)
	})
}

func (b *board) layoutSession(gtx layout.Context, e *env.Env, snapshot state.Snapshot, info protocol.SessionInfo) layout.Dimensions {
	th := e.Theme
	if b.tablets == nil {
		b.tablets = make(map[string]*boardTablet)
	}
	item := b.tablets[info.ID]
	if item == nil {
		item = new(boardTablet)
		b.tablets[info.ID] = item
	}
	item.open.Kind, item.open.Small, item.open.Label = kit.PlaqueTonal, true, i18n.T("Open")
	if item.tablet.Clicked(gtx) || item.tablet.DoubleClicked(gtx) || item.open.Clicked(gtx) {
		open(e, info.ID)
	}
	at := groupOf(snapshot, info)
	content := kit.TabletContent{
		Title: sessionTitle(info), Facts: sessionMatter(snapshot, info), Proof: sessionPlace(snapshot, info),
		Tone: sessionTone(snapshot, info), Action: &item.open,
	}
	switch at {
	case groupNeeds, groupWorking:
		content.Claim = sessionClaim(snapshot, info)
	default:
		content.Claim = look.Ago(gtx.Now, info.Updated)
		if at == groupDone {
			content.Tone = palette.Alive
		}
	}
	if content.Facts == "" {
		content.Facts, content.Proof = content.Proof, env.ModelWords(e, info.Model)
	}
	return item.tablet.Layout(gtx, th.Theme, content, false, false)
}

func boardSentence(sessions [groupCount][]protocol.SessionInfo) string {
	needs, working := len(sessions[groupNeeds]), len(sessions[groupWorking])
	switch {
	case needs == 1:
		return i18n.T("One agent needs you.")
	case needs > 1:
		return i18n.Tf("%s agents need you.", look.CountStart(needs))
	case working == 1:
		return i18n.T("One agent is working.")
	case working > 1:
		return i18n.Tf("%s agents are working.", look.CountStart(working))
	default:
		return i18n.T("No agents are running.")
	}
}

func boardProof(now time.Time, sessions [groupCount][]protocol.SessionInfo) string {
	needs, working, done := len(sessions[groupNeeds]), len(sessions[groupWorking]), len(sessions[groupDone])
	switch {
	case needs > 0 && working > 0:
		return i18n.Tf("%d more working. Answer what is asked to keep them moving.", working)
	case needs > 0:
		return i18n.T("Answer what is asked to keep them moving.")
	case working > 0 && done > 0:
		return i18n.Tf("%d finished since you last looked.", done)
	case working > 0:
		return i18n.T("They will ask here when they need you.")
	case done > 0:
		return i18n.Tf("%d finished since you last looked.", done)
	}
	for _, info := range sessions[groupIdle] {
		return i18n.Tf("The last one finished %s.", look.Ago(now, info.Updated))
	}
	return i18n.T("Pick where it should work and say what to do.")
}
