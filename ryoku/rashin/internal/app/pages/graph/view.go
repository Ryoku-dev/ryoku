package graph

import (
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/gitgraph"
	"ryoku-rashin/internal/protocol"
)

func (v *viewState) layout(gtx layout.Context, e *env.Env, project string) layout.Dimensions {
	th := e.Theme
	if project != v.project {
		v.reset(project)
	}
	if v.refreshRequested {
		v.refreshRequested = false
		v.reload(e)
	}
	if project != "" && !v.loaded && !v.loading {
		v.loadPage(e, true)
	}
	widthDP := int(float32(gtx.Constraints.Max.X) / gtx.Metric.PxPerDp)
	v.sideDetails, widthDP = detailPanePlacement(widthDP)
	v.detailWidth = gtx.Dp(unit.Dp(widthDP))
	v.handleKeys(gtx, e)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S3, Right: th.Sp.S3, Bottom: th.Sp.S2, Left: th.Sp.S3}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions { return v.layoutControls(gtx, e) })
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.layoutContent(gtx, e) }),
	)
}

func (v *viewState) layoutControls(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	labels, scopes, selected := v.scopeOptions()
	search := func(gtx layout.Context) layout.Dimensions {
		v.search.Placeholder, v.search.Hint = i18n.T("Search commits"), "Enter"
		if v.focusSearch {
			v.focusSearch = false
			v.search.Focus(gtx)
		}
		dimensions := v.search.Layout(gtx, th.Theme)
		for {
			raw, ok := v.search.Update(gtx)
			if !ok {
				break
			}
			if submit, ok := raw.(widget.SubmitEvent); ok {
				v.search.SetText(strings.TrimSpace(submit.Text))
				v.reload(e)
			}
		}
		return dimensions
	}
	finder := func(gtx layout.Context) layout.Dimensions {
		v.finder.Placeholder, v.finder.Hint = i18n.T("Go to a branch"), "Enter"
		dimensions := v.finder.Layout(gtx, th.Theme)
		for {
			raw, ok := v.finder.Update(gtx)
			if !ok {
				break
			}
			if submit, ok := raw.(widget.SubmitEvent); ok {
				if !v.findRef(e, submit.Text) {
					v.reload(e)
				}
			}
		}
		return dimensions
	}
	scope := func(gtx layout.Context) layout.Dimensions {
		v.scopeChoice.Form, v.scopeChoice.Icon, v.scopeChoice.Label = look.SelectQuiet, look.Icon.Branch, i18n.T("Show")
		dimensions := v.scopeChoice.Layout(gtx, th, e.Overlays, labels, selected)
		if index, changed := v.scopeChoice.Changed(); changed && index >= 0 && index < len(scopes) {
			option := scopes[index]
			v.scope, v.scopeRef, v.scopeWorktree = option.scope, option.ref, option.worktree
			v.reload(e)
		}
		return dimensions
	}
	finderWidth := min(gtx.Dp(260), max(gtx.Dp(180), gtx.Constraints.Max.X/4))
	children := []layout.FlexChild{
		layout.Rigid(scope),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Flexed(1, search),
	}
	// narrow keeps the commit search only, the scope still reaches every branch and worktree
	if gtx.Constraints.Max.X >= gtx.Dp(640) {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = finderWidth, finderWidth
				return finder(gtx)
			}),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (v *viewState) layoutContent(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	if !v.loaded && v.loading {
		return layout.Inset{Left: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return look.SkeletonRows(gtx, th, 12, false)
		})
	}
	if len(v.entries) == 0 {
		body := i18n.T("Nothing here has been committed yet.")
		switch {
		case v.error != "":
			body = v.error
		case strings.TrimSpace(v.search.Text()) != "":
			body = i18n.Tf("No commit matches “%s”.", strings.TrimSpace(v.search.Text()))
		}
		return kit.Empty{Icon: look.Icon.Graph, Title: i18n.T("No history"), Body: body}.Layout(gtx, th.Theme)
	}
	if v.sideDetails {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.layoutGraph(gtx, e) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(max(1, gtx.Dp(1)), gtx.Constraints.Max.Y)
				look.FillRect(gtx, image.Rectangle{Max: size}, th.R.Vein)
				return layout.Dimensions{Size: size}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				width := min(v.detailWidth, gtx.Constraints.Max.X)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
				return v.layoutDetails(gtx, e)
			}),
		)
	}
	if v.detailOpen {
		return v.layoutDetails(gtx, e)
	}
	return v.layoutGraph(gtx, e)
}

func (v *viewState) layoutGraph(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	v.laneMarkerVisible = false
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, &v.keyTag)
	if v.focusGraph {
		v.focusGraph = false
		gtx.Execute(key.FocusCmd{Tag: &v.keyTag})
	}
	area.Pop()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutGraphHeader(gtx, e) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			count := len(v.entries)
			if v.loading && v.loaded {
				count++
			}
			// drawn under the rows but placed from where they ended up this frame so it never trails a scroll
			record := op.Record(gtx.Ops)
			dimensions := v.scroll.Layout(gtx, th, count, func(gtx layout.Context, index int) layout.Dimensions {
				if index == len(v.entries) {
					return v.layoutPageWorking(gtx, e)
				}
				return v.layoutEntry(gtx, e, index)
			})
			rows := record.Stop()
			v.drawSelection(gtx, th, dimensions.Size)
			rows.Add(gtx.Ops)
			if v.next != "" && !v.loading && v.scroll.Position.First+v.scroll.Position.Count >= len(v.entries)-20 {
				v.loadPage(e, false)
			}
			return dimensions
		}),
	)
}

func (v *viewState) drawSelection(gtx layout.Context, th *look.Theme, size image.Point) {
	index := v.selectedIndex()
	if index < 0 {
		return
	}
	height := gtx.Dp(graphRowHeight)
	top := (index-v.scroll.Position.First)*height - v.scroll.Position.Offset
	inset := max(1, gtx.Dp(1))
	target := image.Rect(v.laneColumnWidth(gtx), top+inset, size.X-gtx.Dp(rowEdge), top+height-inset)
	rect := v.slab.Rect(gtx, v.entries[index].id(), target, th.Motion.Select)
	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	kit.FillCut(gtx.Ops, rect, gtx.Dp(th.Cut.Small), th.R.Sel)
	area.Pop()
}

func (v *viewState) layoutGraphHeader(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(graphHeaderHeight))
	columns := v.columns(gtx, size.X)
	head := func(left, right int, label string, end bool) {
		if right <= left {
			return
		}
		cell := gtx
		cell.Constraints = layout.Constraints{Max: image.Pt(right-left, size.Y)}
		text := kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, label)
		x := left
		if end {
			record := op.Record(gtx.Ops)
			x = right - text(cell).Size.X
			record.Stop()
		}
		look.MiddleLeft(cell, x, size.Y, text)
	}
	head(gtx.Dp(13), columns.lanes, i18n.T("Graph"), false)
	head(columns.text, columns.author, i18n.T("Description"), false)
	head(columns.author, columns.date, i18n.T("Author"), false)
	head(columns.date, columns.hash-gtx.Dp(13), i18n.T("Date"), true)
	head(columns.hash, columns.end, i18n.T("Commit"), false)
	line := max(1, gtx.Dp(1))
	look.FillRect(gtx, image.Rect(0, size.Y-line, size.X, size.Y), th.R.Vein)
	return layout.Dimensions{Size: size}
}

func (v *viewState) layoutPageWorking(gtx layout.Context, e *env.Env) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(graphRowHeight))
	gtx.Constraints = layout.Exact(size)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.Working{Active: true, Size: 16}.Layout(gtx, e.Theme.Theme)
	})
}

func detailPanePlacement(width int) (side bool, pane int) {
	if width < 1000 {
		return false, 0
	}
	return true, min(520, max(380, width*36/100))
}

func (v *viewState) handleKeys(gtx layout.Context, e *env.Env) {
	// the keys below never arrived without this, gio only gives focus to a tag that asks for it
	filters := []event.Filter{
		key.FocusFilter{Target: &v.keyTag},
		key.Filter{Focus: &v.keyTag, Name: key.NameUpArrow},
		key.Filter{Focus: &v.keyTag, Name: key.NameDownArrow},
		key.Filter{Focus: &v.keyTag, Name: key.NamePageUp},
		key.Filter{Focus: &v.keyTag, Name: key.NamePageDown},
		key.Filter{Focus: &v.keyTag, Name: key.NameHome},
		key.Filter{Focus: &v.keyTag, Name: key.NameEnd},
		key.Filter{Focus: &v.keyTag, Name: "/"},
		key.Filter{Focus: &v.keyTag, Name: key.NameReturn},
	}
	for {
		raw, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		keyEvent, ok := raw.(key.Event)
		if !ok || keyEvent.State != key.Press {
			continue
		}
		if keyEvent.Name == "/" {
			v.focusSearch = true
			gtx.Execute(op.InvalidateCmd{})
			continue
		}
		if len(v.entries) == 0 {
			continue
		}
		if keyEvent.Name == key.NameReturn {
			if !v.sideDetails && v.selectedIndex() >= 0 {
				v.detailOpen = true
				gtx.Execute(op.InvalidateCmd{})
			}
			continue
		}
		index := v.selectedIndex()
		if index < 0 {
			index = 0
		}
		switch keyEvent.Name {
		case key.NameUpArrow:
			index--
		case key.NameDownArrow:
			index++
		case key.NamePageUp:
			index -= max(1, v.scroll.Position.Count-1)
		case key.NamePageDown:
			index += max(1, v.scroll.Position.Count-1)
		case key.NameHome:
			index = 0
		case key.NameEnd:
			index = len(v.entries) - 1
		}
		index = min(max(index, 0), len(v.entries)-1)
		v.reveal(index)
		v.selectEntry(e, v.entries[index])
	}
}

// scrolls only as far as it takes, so the list holds still while the selection walks what's on screen
func (v *viewState) reveal(index int) {
	first, count := v.scroll.Position.First, v.scroll.Position.Count
	switch {
	case index <= first:
		v.scroll.ScrollTo(index)
	case index >= first+count-1:
		v.scroll.ScrollTo(max(0, index-max(1, count-2)))
	}
}

func (v *viewState) selectedIndex() int {
	for index, entry := range v.entries {
		if entry.kind == v.selected.kind && ((entry.kind == rowCommit && entry.commit.Hash == v.selected.hash) || (entry.kind == rowWIP && entry.wip.Worktree == v.selected.worktree)) {
			return index
		}
	}
	return -1
}

func (v *viewState) reload(e *env.Env) {
	v.generation++
	v.loading = false
	v.loaded = false
	v.next = ""
	v.error = ""
	v.commits = nil
	v.commitLayouts = nil
	v.entries = nil
	v.refs = nil
	v.wip = nil
	v.layoutState = gitgraph.New()
	v.scroll.ScrollTo(0)
	v.loadPage(e, true)
}

func (v *viewState) loadPage(e *env.Env, first bool) {
	if v.loading || v.project == "" || (!first && v.next == "") {
		return
	}
	v.loading = true
	generation := v.generation
	cursor := ""
	if !first {
		cursor = v.next
	}
	params := protocol.GitLogParams{
		Project: v.project, Scope: v.scope, Ref: v.scopeRef, Worktree: v.scopeWorktree,
		Search: strings.TrimSpace(v.search.Text()), Limit: pageLimit, Cursor: cursor,
	}
	var result protocol.GitLogResult
	e.Call(protocol.MGitLog, params, &result, func(err error) {
		if generation != v.generation {
			return
		}
		v.loading = false
		v.loaded = true
		if err != nil {
			v.error = err.Error()
			return
		}
		v.applyPage(result, first)
		if first {
			v.loadWorktrees(e, generation)
			if v.selected.kind == rowCommit && v.selected.hash != "" {
				v.loadCommit(e, v.selected.hash)
			} else if v.selected.kind == rowWIP && v.selected.worktree != "" {
				v.loadWIP(e, v.selected.worktree)
			} else if len(v.entries) > 0 {
				v.selectEntry(e, v.entries[0])
			}
		}
	})
}

func (v *viewState) applyPage(result protocol.GitLogResult, first bool) {
	if first {
		v.layoutState = gitgraph.New()
		v.commits = nil
		v.commitLayouts = nil
		v.refs = append([]protocol.GitRef(nil), result.Refs...)
		v.wip = append([]protocol.WipRow(nil), result.Wip...)
	}
	rows := v.layoutState.Append(result.Commits)
	v.commits = append(v.commits, result.Commits...)
	v.commitLayouts = append(v.commitLayouts, rows...)
	v.next = result.Next
	v.rebuildEntries()
}

func (v *viewState) loadWorktrees(e *env.Env, generation uint64) {
	var result protocol.WorktreesResult
	e.Call(protocol.MWorktreesList, protocol.ProjectRef{Project: v.project}, &result, func(err error) {
		if err == nil && generation == v.generation {
			v.worktrees = append([]protocol.WorktreeInfo(nil), result.Worktrees...)
		}
	})
}

func (v *viewState) selectEntry(e *env.Env, entry graphEntry) {
	if entry.kind == rowWIP {
		v.selected = selection{kind: rowWIP, hash: entry.wip.Head, worktree: entry.wip.Worktree}
		if e != nil {
			v.loadWIP(e, entry.wip.Worktree)
		}
		return
	}
	v.selected = selection{kind: rowCommit, hash: entry.commit.Hash}
	if e != nil {
		v.loadCommit(e, entry.commit.Hash)
	}
}

func (v *viewState) findRef(e *env.Env, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for index, entry := range v.entries {
		matched := false
		if entry.kind == rowWIP {
			matched = strings.Contains(strings.ToLower(entry.wip.Worktree), query) || strings.Contains(strings.ToLower(entry.wip.Branch), query)
		} else {
			for _, ref := range v.refsFor(entry.commit.Hash) {
				matched = matched || strings.Contains(strings.ToLower(ref.Name), query)
			}
		}
		if matched {
			v.scroll.ScrollTo(index)
			v.selectEntry(e, entry)
			return true
		}
	}
	for _, ref := range v.refs {
		if strings.Contains(strings.ToLower(ref.Name), query) {
			v.scope, v.scopeRef, v.scopeWorktree = protocol.ScopeBranch, ref.Name, ""
			return false
		}
	}
	v.error = i18n.T("No matching ref or worktree")
	return true
}

// returns the token the answers must still hold when they arrive
func (v *viewState) clearDetails() uint64 {
	v.detailToken++
	v.diffLoading = true
	v.detailError = ""
	v.detail = protocol.GitCommitDetail{}
	v.status = protocol.GitStatusResult{}
	v.diff = protocol.DiffResult{}
	v.compareWith = ""
	return v.detailToken
}

func (v *viewState) loadCommit(e *env.Env, hash string) {
	token := v.clearDetails()
	current := func() bool { return token == v.detailToken && v.selected.kind == rowCommit && v.selected.hash == hash }
	var detail protocol.GitCommitDetail
	e.Call(protocol.MGitShow, protocol.GitCommitParams{Project: v.project, Hash: hash}, &detail, func(err error) {
		if !current() {
			return
		}
		if err != nil {
			v.detailError = err.Error()
			return
		}
		v.detail = detail
	})
	var diff protocol.DiffResult
	params := protocol.DiffParams{Project: v.project, Mode: protocol.DiffCommit, Commit: hash, Context: 3}
	e.Call(protocol.MGitDiff, params, &diff, func(err error) {
		if !current() {
			return
		}
		v.diffLoading = false
		if err == nil {
			v.diff = diff
		}
	})
}

func (v *viewState) loadWIP(e *env.Env, path string) {
	token := v.clearDetails()
	current := func() bool { return token == v.detailToken && v.selected.kind == rowWIP && v.selected.worktree == path }
	var status protocol.GitStatusResult
	e.Call(protocol.MGitStatus, protocol.WorktreeRef{Path: path}, &status, func(err error) {
		if !current() {
			return
		}
		if err != nil {
			v.detailError = err.Error()
			return
		}
		v.status = status
	})
	var diff protocol.DiffResult
	e.Call(protocol.MGitDiff, protocol.DiffParams{Worktree: path, Mode: protocol.DiffWorking, Context: 3}, &diff, func(err error) {
		if !current() {
			return
		}
		v.diffLoading = false
		if err == nil {
			v.diff = diff
		}
	})
}
