package projects

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// body width from which the list and the selected worktree stand side by side
const overviewSplit = 860

type overviewState struct {
	list     look.Scroller
	detail   look.Scroller
	rows     map[string]*worktreeRowState
	fresh    kit.Plaque
	more     look.IconAction
	moreTip  look.Tip
	toolMenu kit.Menu

	create  createWorktree
	move    moveWorktree
	remove  removeWorktree
	prune   pruneWorktrees
	cleanup cleanupWorktrees
}

type worktreeRowState struct {
	tablet   kit.Tablet
	start    kit.Plaque
	session  kit.Plaque
	changes  kit.Plaque
	terminal kit.IconPlaque
	folder   kit.IconPlaque
	more     kit.IconPlaque
	tips     [3]look.Tip
	menu     kit.Menu

	showLog, trust, rerun kit.Plaque
	log                   look.Unfold
	logOpen               bool
	setup                 protocol.SetupResult
	ports                 protocol.PortsResult
	portsLoaded           bool
	portOpen              map[int]*kit.Plaque
	sessions              map[string]*kit.Tablet
}

func (o *overviewState) init() { o.rows = make(map[string]*worktreeRowState) }

func (o *overviewState) row(path string) *worktreeRowState {
	row := o.rows[path]
	if row == nil {
		row = &worktreeRowState{portOpen: make(map[int]*kit.Plaque), sessions: make(map[string]*kit.Tablet)}
		o.rows[path] = row
	}
	return row
}

func (o *overviewState) layout(gtx layout.Context, e *env.Env, page *pageState) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	worktrees := snapshot.Worktrees[page.project]
	selected, ok := page.selectedWorktree(e)
	if !ok {
		return o.layoutEmpty(gtx, e, page)
	}
	page.worktree = selected.Path
	list := func(gtx layout.Context) layout.Dimensions { return o.layoutList(gtx, e, page, worktrees, selected) }
	detail := func(gtx layout.Context) layout.Dimensions { return o.layoutDetail(gtx, e, page, selected) }
	if gtx.Constraints.Max.X < gtx.Dp(overviewSplit) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(260))
				return list(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Rule(gtx, th) }),
			layout.Flexed(1, detail),
		)
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			width := gtx.Dp(380)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			return list(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(max(1, gtx.Dp(1)), gtx.Constraints.Max.Y)
			look.FillRect(gtx, image.Rectangle{Max: size}, th.R.Vein)
			return layout.Dimensions{Size: size}
		}),
		layout.Flexed(1, detail),
	)
}

func (o *overviewState) layoutEmpty(gtx layout.Context, e *env.Env, page *pageState) layout.Dimensions {
	info, _ := page.projectInfo(e)
	body := i18n.T("Its worktrees appear here once the repository has been read.")
	if !info.Git {
		body = i18n.T("This folder has no Git repository, so it has no worktrees, graph or changes. Agents can still work in it.")
	}
	return kit.Empty{Icon: look.Icon.Branch, Title: i18n.T("No worktrees"), Body: body}.Layout(gtx, e.Theme.Theme)
}

func (o *overviewState) layoutList(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	worktrees []protocol.WorktreeInfo,
	selected protocol.WorktreeInfo,
) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	return layout.Inset{Top: th.Sp.S3, Left: th.Sp.S3, Right: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.layoutTools(gtx, e, page) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				o.list.Gap = gtx.Dp(4)
				return o.list.Layout(gtx, th, len(worktrees), func(gtx layout.Context, index int) layout.Dimensions {
					worktree := worktrees[index]
					row := o.row(worktree.Path)
					row.tablet.Pane = th.R.Stone2
					row.start.Kind, row.start.Small, row.start.Label = kit.PlaqueLine, true, i18n.T("New session")
					if row.start.Clicked(gtx) {
						page.newSession(e, worktree.Path)
					}
					if row.tablet.Clicked(gtx) {
						e.NavigateWork(page.project, TabOverview, worktree.Path)
					}
					content := worktreeTablet(th, snapshot, worktree)
					content.Action = &row.start
					return row.tablet.Layout(gtx, th.Theme, content, worktree.Path == selected.Path, false)
				})
			}),
		)
	})
}

func (o *overviewState) layoutTools(gtx layout.Context, e *env.Env, page *pageState) layout.Dimensions {
	th := e.Theme
	// the pane's one gold action is starting a session in the worktree that is shown
	o.fresh.Kind, o.fresh.Small, o.fresh.Icon, o.fresh.Label = kit.PlaqueLine, true, look.Icon.Add, i18n.T("New worktree")
	if o.fresh.Clicked(gtx) {
		info, _ := page.projectInfo(e)
		o.openNew(e, page, info)
	}
	o.more.Icon, o.more.Description = look.Icon.More, i18n.T("Clean up, prune and repair")
	if o.more.Clicked(gtx) {
		o.openTools(gtx, e, page)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.fresh.Layout(gtx, th.Theme) }),
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return o.moreTip.Layout(gtx, th, e.Overlays, o.more.Description, func(gtx layout.Context) layout.Dimensions {
				return o.more.Layout(gtx, th, th.R.Fg2)
			})
		}),
	)
}

func (o *overviewState) openTools(gtx layout.Context, e *env.Env, page *pageState) {
	items := []kit.MenuItem{
		{Label: i18n.T("Clean up finished worktrees")},
		{Label: i18n.T("Prune records of missing folders")},
		{Label: i18n.T("Repair worktree links")},
	}
	tag := &o.toolMenu
	box := gtx.Dp(28)
	anchor := e.Overlays.AnchorOf(&o.more.Clickable, image.Pt(box, box))
	e.Overlays.Open(tag, anchor, look.Placement{Align: look.End}, func(gtx layout.Context) layout.Dimensions {
		dimensions := o.toolMenu.Layout(gtx, e.Theme.Theme, items)
		if item, ok := o.toolMenu.Clicked(gtx); ok {
			e.Overlays.Close(tag)
			switch item {
			case 0:
				o.cleanup.open(e, page)
			case 1:
				o.prune.open(e, page)
			case 2:
				o.repair(e, page)
			}
		}
		return dimensions
	})
}

func (o *overviewState) repair(e *env.Env, page *pageState) {
	var result protocol.WorktreesResult
	e.Call(protocol.MWorktreesRepair, protocol.ProjectRef{Project: page.project}, &result, func(err error) {
		if err != nil {
			page.fail(err)
			return
		}
		e.Store.SetWorktrees(result)
		e.Notify(i18n.T("Worktree links repaired."))
	})
}

func worktreeTablet(th *look.Theme, snapshot appstate.Snapshot, worktree protocol.WorktreeInfo) kit.TabletContent {
	content := kit.TabletContent{Title: worktree.Name, Facts: worktreeBranch(worktree), FactsMono: true, Progress: -1}
	glyph := look.Icon.Branch
	if worktree.Main {
		glyph = look.Icon.Folder
	}
	content.Mark = func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, look.IconWidget(glyph, 18, th.R.Fg3))
	}
	content.Claim, content.Tone = worktreeClaim(snapshot, worktree)
	if status := worktree.Status; status != nil {
		switch {
		case status.Added+status.Removed > 0:
			content.Proof = fmt.Sprintf("+%d −%d", status.Added, status.Removed)
		case status.AheadBase+status.BehindBase > 0:
			content.Proof = fmt.Sprintf("↑%d ↓%d", status.AheadBase, status.BehindBase)
		}
	}
	return content
}

func worktreeBranch(worktree protocol.WorktreeInfo) string {
	if worktree.Detached {
		return i18n.Tf("detached at %s", shortHash(worktree.Head))
	}
	return worktree.Branch
}

func worktreeClaim(snapshot appstate.Snapshot, worktree protocol.WorktreeInfo) (string, palette.Tone) {
	status := worktree.Status
	switch {
	case worktree.Missing:
		return i18n.T("Missing"), palette.Failed
	case status != nil && status.Conflicted > 0:
		return i18n.Tf("%d conflicts", status.Conflicted), palette.Attention
	}
	working := false
	for _, id := range worktree.Sessions {
		switch snapshot.Sessions[id].State {
		case protocol.StateWaiting, protocol.StateFailed:
			return i18n.T("Needs you"), palette.Attention
		case protocol.StateWorking:
			working = true
		}
	}
	switch {
	case working:
		return i18n.T("Working"), palette.Becoming
	case status == nil:
		return "", palette.Idle
	case status.Staged+status.Modified+status.Untracked > 0:
		return i18n.Tf("%d changed", status.Staged+status.Modified+status.Untracked), palette.Idle
	default:
		return i18n.T("Clean"), palette.Idle
	}
}

func (o *overviewState) layoutDetail(gtx layout.Context, e *env.Env, page *pageState, worktree protocol.WorktreeInfo) layout.Dimensions {
	th := e.Theme
	row := o.row(worktree.Path)
	o.loadPorts(e, worktree, row)
	sections := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return o.layoutIdentity(gtx, e, page, worktree, row) },
		func(gtx layout.Context) layout.Dimensions { return look.Facts(gtx, th, worktreeFacts(th, worktree)) },
	}
	if state := setupState(worktree, row); state != protocol.SetupDone && state != protocol.SetupNone && state != "" {
		sections = append(sections, func(gtx layout.Context) layout.Dimensions {
			return o.layoutSetup(gtx, e, page, worktree, row, state)
		})
	}
	if len(worktree.Sessions) > 0 {
		sections = append(sections, func(gtx layout.Context) layout.Dimensions { return o.layoutSessions(gtx, e, worktree, row) })
	}
	if len(row.ports.Ports) > 0 {
		sections = append(sections, func(gtx layout.Context) layout.Dimensions { return o.layoutPorts(gtx, e, row) })
	}
	inset := th.Sp.S5
	if gtx.Constraints.Max.X < gtx.Dp(560) {
		inset = th.Sp.S4
	}
	return o.detail.Layout(gtx, th, len(sections), func(gtx layout.Context, index int) layout.Dimensions {
		top, bottom := th.Sp.S4, th.Sp.S1
		if index == len(sections)-1 {
			bottom = th.Sp.S5
		}
		return layout.Inset{Top: top, Bottom: bottom, Left: inset, Right: inset}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(look.ReadingColumnWidth))
			return sections[index](gtx)
		})
	})
}

func (o *overviewState) layoutIdentity(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	worktree protocol.WorktreeInfo,
	row *worktreeRowState,
) layout.Dimensions {
	th := e.Theme
	o.handleActs(gtx, e, page, worktree, row)
	marks := worktreeMarks(worktree)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, worktree.Name)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(look.TokenChip(th, worktreeBranch(worktree))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Flexed(1, kit.Text(th.Theme, th.T.Caption, th.R.Fg3, marks)),
			)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.layoutActs(gtx, e, row) }),
	)
}

func (o *overviewState) layoutActs(gtx layout.Context, e *env.Env, row *worktreeRowState) layout.Dimensions {
	th := e.Theme
	row.session.Kind, row.session.Icon, row.session.Label = kit.PlaqueGold, look.Icon.Agents, i18n.T("New session here")
	row.changes.Kind, row.changes.Icon, row.changes.Label = kit.PlaqueLine, look.Icon.Diff, i18n.T("Open changes")
	icons := [...]struct {
		plaque *kit.IconPlaque
		tip    string
	}{
		{&row.terminal, i18n.T("Open a terminal here")},
		{&row.folder, i18n.T("Open the folder")},
		{&row.more, i18n.T("Lock, move, set up or remove")},
	}
	row.terminal.Icon, row.folder.Icon, row.more.Icon = look.Icon.Terminal, look.Icon.Folder, look.Icon.More
	widgets := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return row.session.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return row.changes.Layout(gtx, th.Theme) },
	}
	for index, icon := range icons {
		icon.plaque.Kind, icon.plaque.Description = kit.PlaqueQuiet, icon.tip
		widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
			return row.tips[index].Layout(gtx, th, e.Overlays, icon.tip, func(gtx layout.Context) layout.Dimensions {
				return icon.plaque.Layout(gtx, th.Theme)
			})
		})
	}
	return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(widgets), func(gtx layout.Context, index int) layout.Dimensions {
		return widgets[index](gtx)
	})
}

func (o *overviewState) handleActs(gtx layout.Context, e *env.Env, page *pageState, worktree protocol.WorktreeInfo, row *worktreeRowState) {
	if row.session.Clicked(gtx) {
		page.newSession(e, worktree.Path)
	}
	if row.changes.Clicked(gtx) {
		e.NavigateWork(page.project, TabChanges, worktree.Path)
	}
	if row.terminal.Clicked(gtx) {
		e.OpenTerminal(worktree.Path)
	}
	if row.folder.Clicked(gtx) {
		e.Open(worktree.Path)
	}
	if row.more.Clicked(gtx) {
		o.openWorktreeMenu(gtx, e, page, worktree, row)
	}
}

func (o *overviewState) openWorktreeMenu(gtx layout.Context, e *env.Env, page *pageState, worktree protocol.WorktreeInfo, row *worktreeRowState) {
	lock := i18n.T("Lock")
	if worktree.Locked {
		lock = i18n.T("Unlock")
	}
	items := []kit.MenuItem{
		{Icon: look.Icon.Lock, Label: lock},
		{Icon: look.Icon.Folder, Label: i18n.T("Move")},
		{Icon: look.Icon.Play, Label: i18n.T("Run setup")},
		{Divider: true},
		{Icon: look.Icon.Delete, Label: i18n.T("Remove"), Danger: true, Disabled: worktree.Main},
	}
	tag := &row.menu
	size := gtx.Dp(40)
	anchor := e.Overlays.AnchorOf(&row.more.Clickable, image.Pt(size, size))
	e.Overlays.Open(tag, anchor, look.Placement{}, func(gtx layout.Context) layout.Dimensions {
		dimensions := row.menu.Layout(gtx, e.Theme.Theme, items)
		item, ok := row.menu.Clicked(gtx)
		if !ok {
			return dimensions
		}
		e.Overlays.Close(tag)
		switch item {
		case 0:
			params := protocol.WorktreeLockParams{Path: worktree.Path, Lock: !worktree.Locked, Reason: i18n.T("Locked in Rashin")}
			e.Call(protocol.MWorktreesLock, params, new(protocol.WorktreeInfo), func(err error) {
				page.fail(err)
				e.LoadWorktrees(page.project, nil)
			})
		case 1:
			o.move.open(worktree)
		case 2:
			o.runSetup(e, page, worktree, row, true)
		case 4:
			o.remove.open(e, page, worktree)
		}
		return dimensions
	})
}

func worktreeMarks(worktree protocol.WorktreeInfo) string {
	marks := make([]string, 0, 4)
	if worktree.Main {
		marks = append(marks, i18n.T("The project's own folder"))
	}
	if worktree.Locked {
		marks = append(marks, i18n.T("Locked"))
	}
	if worktree.Missing {
		marks = append(marks, i18n.T("Its folder is missing"))
	}
	if worktree.Prunable {
		marks = append(marks, i18n.T("Can be pruned"))
	}
	return strings.Join(marks, ", ")
}

func worktreeFacts(th *look.Theme, worktree protocol.WorktreeInfo) []look.FactRow {
	rows := make([]look.FactRow, 0, 6)
	if status := worktree.Status; status != nil {
		rows = append(rows, look.FactRow{Label: i18n.T("Working tree"), Value: workingTreeWords(status)})
		if status.Added+status.Removed > 0 {
			rows = append(rows, look.FactRow{Label: i18n.T("Lines"), Widget: look.DiffCount(th, status.Added, status.Removed, false)})
		}
		if status.Upstream != "" {
			value := fmt.Sprintf("%s  ↑%d ↓%d", status.Upstream, status.Ahead, status.Behind)
			rows = append(rows, look.FactRow{Label: i18n.T("Upstream"), Value: value, Mono: true})
		}
		if worktree.Base != "" {
			value := fmt.Sprintf("%s  ↑%d ↓%d", worktree.Base, status.AheadBase, status.BehindBase)
			rows = append(rows, look.FactRow{Label: i18n.T("Base"), Value: value, Mono: true})
		}
	}
	rows = append(rows,
		look.FactRow{Label: i18n.T("Head"), Value: shortHash(worktree.Head), Mono: true},
		look.FactRow{Label: i18n.T("Folder"), Value: paths.Short(worktree.Path), Mono: true},
	)
	return rows
}

func workingTreeWords(status *protocol.WorktreeStatus) string {
	parts := make([]string, 0, 5)
	add := func(count int, format string) {
		if count > 0 {
			parts = append(parts, i18n.Tf(format, count))
		}
	}
	add(status.Conflicted, "%d in conflict")
	add(status.Staged, "%d staged")
	add(status.Modified, "%d modified")
	add(status.Untracked, "%d untracked")
	if status.Operation != "" {
		parts = append(parts, i18n.Tf("%s in progress", status.Operation))
	}
	if len(parts) == 0 {
		return i18n.T("Clean")
	}
	return strings.Join(parts, ", ")
}

func sectionTitle(th *look.Theme, title string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: th.Sp.S2}.Layout(gtx, kit.Text(th.Theme, th.T.Title, th.R.Fg, title))
	}
}

func (o *overviewState) layoutSessions(gtx layout.Context, e *env.Env, worktree protocol.WorktreeInfo, row *worktreeRowState) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	children := []layout.FlexChild{layout.Rigid(sectionTitle(th, i18n.T("Sessions")))}
	for _, id := range worktree.Sessions {
		info, ok := snapshot.Sessions[id]
		if !ok {
			continue
		}
		tablet := row.sessions[id]
		if tablet == nil {
			tablet = new(kit.Tablet)
			row.sessions[id] = tablet
		}
		tablet.Pane = th.R.Stone2
		if tablet.Clicked(gtx) {
			e.Navigate(protocol.TabAgent, id)
		}
		content := kit.TabletContent{Title: info.Title, Facts: info.Activity, Progress: -1}
		if content.Facts == "" {
			content.Facts = info.Preview
		}
		content.Mark = func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, look.IconWidget(look.Icon.Agents, 18, th.R.Fg3))
		}
		content.Claim, content.Tone = sessionClaim(info)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return tablet.Layout(gtx, th.Theme, content, false, false)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func sessionClaim(info protocol.SessionInfo) (string, palette.Tone) {
	switch info.State {
	case protocol.StateWorking:
		return i18n.T("Working"), palette.Becoming
	case protocol.StateWaiting:
		return i18n.T("Needs you"), palette.Attention
	case protocol.StateFailed:
		return i18n.T("Failed"), palette.Failed
	case protocol.StateStopped:
		return i18n.T("Stopped"), palette.Idle
	default:
		if info.Unread {
			return i18n.T("Done"), palette.Alive
		}
		return "", palette.Idle
	}
}

func (o *overviewState) loadPorts(e *env.Env, worktree protocol.WorktreeInfo, row *worktreeRowState) {
	if row.portsLoaded {
		return
	}
	row.portsLoaded = true
	e.Call(protocol.MWorktreesPorts, protocol.WorktreeRef{Path: worktree.Path}, &row.ports, func(error) {})
}

func (o *overviewState) layoutPorts(gtx layout.Context, e *env.Env, row *worktreeRowState) layout.Dimensions {
	th := e.Theme
	ports := row.ports.Ports
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(sectionTitle(th, i18n.T("Listening"))),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(ports), func(gtx layout.Context, index int) layout.Dimensions {
				port := ports[index]
				plaque := row.portOpen[port.Port]
				if plaque == nil {
					plaque = new(kit.Plaque)
					row.portOpen[port.Port] = plaque
				}
				plaque.Kind, plaque.Small, plaque.Icon = kit.PlaqueQuiet, true, look.Icon.OpenInNew
				plaque.Label = fmt.Sprintf("%d  %s", port.Port, port.Command)
				if plaque.Clicked(gtx) {
					e.Open(port.URL)
				}
				return plaque.Layout(gtx, th.Theme)
			})
		}),
	)
}

func setupState(worktree protocol.WorktreeInfo, row *worktreeRowState) string {
	if row.setup.State != "" {
		return row.setup.State
	}
	return worktree.Setup
}

func (o *overviewState) runSetup(e *env.Env, page *pageState, worktree protocol.WorktreeInfo, row *worktreeRowState, rerun bool) {
	if rerun {
		row.setup.State = protocol.SetupRunning
	}
	params := protocol.WorktreeSetupParams{Path: worktree.Path, Rerun: rerun}
	e.Call(protocol.MWorktreesSetup, params, &row.setup, func(err error) {
		page.fail(err)
		row.logOpen = row.setup.State == protocol.SetupFailed || row.setup.State == protocol.SetupUntrusted
		e.LoadWorktrees(page.project, nil)
	})
}

func (o *overviewState) layoutSetup(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	worktree protocol.WorktreeInfo,
	row *worktreeRowState,
	state string,
) layout.Dimensions {
	th := e.Theme
	if row.showLog.Clicked(gtx) {
		row.logOpen = !row.logOpen
		if row.setup.State == "" {
			o.runSetup(e, page, worktree, row, false)
		}
	}
	if row.rerun.Clicked(gtx) {
		o.runSetup(e, page, worktree, row, true)
	}
	if row.trust.Clicked(gtx) {
		params := protocol.TrustParams{Path: worktree.Path, Hash: row.setup.Hash}
		e.Call(protocol.MWorktreesTrust, params, &row.setup, func(err error) {
			page.fail(err)
			e.LoadWorktrees(page.project, nil)
		})
	}
	tone, word, actions := palette.Becoming, i18n.T("Setting up"), []*kit.Plaque(nil)
	row.showLog.Kind, row.showLog.Small, row.showLog.Label = kit.PlaqueQuiet, true, i18n.T("Show log")
	if row.logOpen {
		row.showLog.Label = i18n.T("Hide log")
	}
	row.rerun.Kind, row.rerun.Small, row.rerun.Label = kit.PlaqueLine, true, i18n.T("Run again")
	row.trust.Kind, row.trust.Small, row.trust.Label = kit.PlaqueGold, true, i18n.T("Trust and run")
	switch state {
	case protocol.SetupFailed:
		tone, word, actions = palette.Failed, i18n.T("Setup failed"), []*kit.Plaque{&row.rerun, &row.showLog}
	case protocol.SetupUntrusted:
		tone, word = palette.Attention, i18n.T("Setup waits for you")
		row.showLog.Label = i18n.T("Review the script")
		actions = []*kit.Plaque{&row.showLog}
		if row.setup.Hash != "" {
			actions = []*kit.Plaque{&row.trust, &row.showLog}
		}
	}
	record := op.Record(gtx.Ops)
	dimensions := layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if state == protocol.SetupRunning {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
						}),
						layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
						layout.Rigid(kit.Text(th.Theme, th.T.Label, th.R.Fg2, word)),
					)
				}
				return kit.Claim(th.Theme, tone, word)(gtx)
			}),
		}
		if len(actions) > 0 {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(actions), func(gtx layout.Context, index int) layout.Dimensions {
						return actions[index].Layout(gtx, th.Theme)
					})
				}),
			)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return row.log.Layout(gtx, th, row.logOpen, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S3}.Layout(gtx, look.Paragraph(th, th.T.Mono, th.R.Fg2, setupLog(row.setup)))
			})
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), th.R.Field(tone))
	call.Add(gtx.Ops)
	return dimensions
}

func setupLog(result protocol.SetupResult) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{result.Command, result.Script, result.Log} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return i18n.T("Nothing was written yet.")
	}
	return strings.Join(parts, "\n\n")
}
