package agent

import (
	"image"
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
	composer  chat.Composer
	place     placement
	approvals env.Approvals
	more      kit.Plaque
	temporary env.Temporary
	starting  bool
	tablets   map[string]*boardTablet
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
	itemInstall
	itemPlaces
	itemSection
	itemSession
)

func (b *board) focus(e *env.Env) {
	b.place.prime(e)
	b.composer.Focus()
}

func (b *board) layout(gtx layout.Context, e *env.Env, page *Page) layout.Dimensions {
	snapshot := e.Snapshot()
	sessions := groupSessions(snapshot)
	items := b.items(snapshot, sessions)
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
			return b.scroll.Layout(gtx, e.Theme, len(items), func(gtx layout.Context, index int) layout.Dimensions {
				return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
					return b.layoutItem(gtx, e, page, snapshot, sessions, items[index], index == len(items)-1)
				})
			})
		}),
	)
}

func (b *board) items(snapshot state.Snapshot, sessions [groupCount][]protocol.SessionInfo) []boardItem {
	items := []boardItem{{kind: itemLead}}
	if len(env.InstalledHarnesses(snapshot)) == 0 {
		// goes first: without a harness nothing below it can be continued either
		items = append(items, boardItem{kind: itemInstall})
	}
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
	// a board with nothing to report says where an agent could start instead of standing empty
	if len(items) == 1 && len(env.InstalledHarnesses(snapshot)) > 0 {
		items = append(items, boardItem{kind: itemPlaces})
	}
	return items
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
	case itemInstall:
		return layout.Inset{Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.first.layoutInstall(gtx, e, snapshot)
		})
	case itemPlaces:
		return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.first.layoutPlaces(gtx, e, snapshot, b)
		})
	case itemSection:
		return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S2}.Layout(gtx, look.SectionTitle(th, item.title, item.count))
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
	// with no harness there is nothing to tell an agent yet, so the composer waits for one
	if len(env.InstalledHarnesses(snapshot)) == 0 {
		return look.Monument(gtx, th, i18n.T("Agents need a harness."), i18n.T("Install one and you can start agents from here."))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// the sentence stays in one ink, the rows below carry each agent's state
			return kit.Sentence(gtx, th.Theme, th.T.Sentence, th.R.Fg, []kit.Span{{Text: boardSentence(sessions)}})
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(kit.Text(th.Theme, th.T.Proof, th.R.Fg2, boardProof(gtx.Now, sessions))),
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
	may := b.approvals.Value("", snapshot.Settings.Agent.Approvals)
	options := chat.ComposerOptions{
		Placeholder: i18n.T("Tell an agent what to do"), CanAttach: true, Disabled: b.starting,
		Controls: append(b.place.controls(e, home), func(gtx layout.Context) layout.Dimensions {
			return b.approvals.Layout(gtx, e, env.AgentApprovals, "", snapshot.Settings.Agent.Approvals, home)
		}),
	}
	if value, changed := b.approvals.Changed(); changed {
		e.SetApprovals(env.AgentApprovals, value)
	}
	// the field itself says what a temporary session is, where it always fits
	if b.temporary.On {
		options.Placeholder = env.TemporaryAgentHint()
	}
	e.ComposerDesktop(&b.composer, &options)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	harnesses := env.InstalledHarnesses(snapshot)
	picked := b.place.picked(snapshot, harnesses)
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
	// what the session will run with, the model picked here over the harness's own, so the choice is made knowing it
	if facts := env.StartFacts(b.place.model.reader.Info(snapshot, picked.ID), e.AgentModels[picked.ID]); len(facts) > 0 {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return env.LayoutFacts(gtx, th, facts) }),
		)
	}
	children = append(children, b.place.model.noteRows(th)...)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (b *board) start(e *env.Env, snapshot state.Snapshot, submission chat.Submission) {
	params, ok := b.place.params(snapshot, e.AgentModels, details{name: worktreeName(submission.Text)})
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
	item.tablet.Pane = th.R.Stone2
	item.open.Kind, item.open.Small, item.open.Label = kit.PlaqueTonal, true, i18n.T("Open")
	if item.tablet.Clicked(gtx) || item.tablet.DoubleClicked(gtx) || item.open.Clicked(gtx) {
		open(e, info.ID)
	}
	at := groupOf(snapshot, info)
	content := kit.TabletContent{
		Title: sessionTitle(info), Facts: sessionMatter(snapshot, info), Proof: sessionPlace(snapshot, info),
		Tone: sessionTone(snapshot, info), Action: &item.open, Mark: harnessMark(th, harnessName(snapshot, info.Harness)),
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
		content.Facts, content.Proof = content.Proof, harnessName(snapshot, info.Harness)
	}
	return item.tablet.Layout(gtx, th.Theme, content, false, false)
}

func harnessMark(th *look.Theme, name string) layout.Widget {
	letters := []rune(name)
	if len(letters) > 2 {
		letters = letters[:2]
	}
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Min
		kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), th.R.Field)
		return layout.Center.Layout(gtx, kit.RawText(th.Theme, th.T.Label, th.R.Fg2, string(letters)))
	}
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

func approvalsNote(snapshot state.Snapshot, noHarness bool) string {
	if noHarness {
		return i18n.T("No harness is installed, so nothing can start yet.")
	}
	return env.ApprovalsNote(env.AgentApprovals, snapshot.Settings.Agent.Approvals)
}
