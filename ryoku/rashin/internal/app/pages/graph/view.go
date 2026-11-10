package graph

import (
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/gitgraph"
	"ryoku-rashin/internal/protocol"
)

func (v *viewState) layout(gtx layout.Context, e *env.Env, project string) layout.Dimensions {
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

	body := func(gtx layout.Context) layout.Dimensions {
		return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 13, Right: 13, Bottom: 13, Left: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutControls(gtx, e) }),
					layout.Rigid(layout.Spacer{Height: 8}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.layoutContent(gtx, e) }),
				)
			})
		})
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(body),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions { return v.layoutOperationDialog(gtx, e) }),
	)
}

func (v *viewState) layoutControls(gtx layout.Context, e *env.Env) layout.Dimensions {
	labels, scopes, selected := v.scopeOptions()
	height := gtx.Dp(40)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	search := func(gtx layout.Context) layout.Dimensions {
		v.search.Placeholder, v.search.Hint = i18n.T("Search commits"), i18n.T("Enter")
		if v.focusSearch {
			v.focusSearch = false
			v.search.Focus(gtx)
		}
		dimensions := v.search.Layout(gtx, e.Theme.Theme)
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
		v.finder.Placeholder, v.finder.Hint = i18n.T("Find ref or worktree"), i18n.T("Enter")
		dimensions := v.finder.Layout(gtx, e.Theme.Theme)
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
		dimensions := v.scopeChoice.Layout(gtx, e.Theme, e.Overlays, labels, selected)
		if index, changed := v.scopeChoice.Changed(); changed && index >= 0 && index < len(scopes) {
			option := scopes[index]
			v.scope, v.scopeRef, v.scopeWorktree = option.scope, option.ref, option.worktree
			v.reload(e)
		}
		return dimensions
	}
	scopeWidth := min(gtx.Dp(220), max(gtx.Dp(160), gtx.Constraints.Max.X/4))
	finderWidth := min(gtx.Dp(260), max(gtx.Dp(180), gtx.Constraints.Max.X/3))
	fixed := func(width int, body layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			return body(gtx)
		}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(fixed(scopeWidth, scope)),
		layout.Rigid(layout.Spacer{Width: 6}.Layout),
		layout.Flexed(1, search),
		layout.Rigid(layout.Spacer{Width: 6}.Layout),
		layout.Rigid(fixed(finderWidth, finder)),
	)
}

func (v *viewState) layoutContent(gtx layout.Context, e *env.Env) layout.Dimensions {
	if !v.loaded && v.loading {
		working := kit.Working{Active: true}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return working.Layout(gtx, e.Theme.Theme) }),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Label, e.Theme.R.Fg2, i18n.T("Loading history"))),
			)
		})
	}
	if len(v.entries) == 0 {
		body := i18n.T("This scope has no commits.")
		if v.error != "" {
			body = v.error
		}
		return (kit.Empty{Title: i18n.T("No history"), Body: body}).Layout(gtx, e.Theme.Theme)
	}
	if v.sideDetails {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.layoutGraph(gtx, e) }),
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
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
	v.laneMarkerVisible = false
	return kit.Slab(gtx, e.Theme.R.Stone1, e.Theme.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
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
				dimensions := v.scroll.Layout(gtx, e.Theme, count, func(gtx layout.Context, index int) layout.Dimensions {
					if index == len(v.entries) {
						return v.layoutPageWorking(gtx, e)
					}
					return v.layoutEntry(gtx, e, index)
				})
				if v.next != "" && !v.loading && v.scroll.Position.First+v.scroll.Position.Count >= len(v.entries)-20 {
					v.loadPage(e, false)
				}
				return dimensions
			}),
		)
	})
}

func (v *viewState) layoutGraphHeader(gtx layout.Context, e *env.Env) layout.Dimensions {
	height := gtx.Dp(graphHeaderHeight)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	laneWidth := v.laneColumnWidth(gtx)
	authorWidth, dateWidth, hashWidth := commitColumnWidths(gtx)
	paint.FillShape(gtx.Ops, e.Theme.R.Stone2, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())
	line := max(1, gtx.Dp(1))
	paint.FillShape(gtx.Ops, e.Theme.R.Vein, clip.Rect(image.Rect(0, height-line, gtx.Constraints.Max.X, height)).Op())
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return headerCell(gtx, e, laneWidth, i18n.T("Graph"))
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return headerCell(gtx, e, gtx.Constraints.Max.X, i18n.T("Description"))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return headerCell(gtx, e, authorWidth, i18n.T("Author"))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return headerCell(gtx, e, dateWidth, i18n.T("Date"))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return headerCell(gtx, e, hashWidth, i18n.T("Commit"))
		}),
	)
}

func headerCell(gtx layout.Context, e *env.Env, width int, label string) layout.Dimensions {
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
	return layout.Inset{Right: 8, Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, kit.Text(e.Theme.Theme, e.Theme.T.Group, e.Theme.R.Fg3, label))
	})
}

func (v *viewState) layoutPageWorking(gtx layout.Context, e *env.Env) layout.Dimensions {
	height := gtx.Dp(graphRowHeight)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	working := kit.Working{Active: true, Size: 7}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return working.Layout(gtx, e.Theme.Theme) }),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg2, i18n.T("Loading more commits"))),
		)
	})
}

func detailPanePlacement(width int) (side bool, pane int) {
	if width < 1000 {
		return false, 0
	}
	return true, min(560, max(380, width*40/100))
}

func (v *viewState) handleKeys(gtx layout.Context, e *env.Env) {
	filters := []event.Filter{
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
		v.scroll.ScrollTo(index)
		v.selectEntry(e, v.entries[index])
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

func (v *viewState) loadCommit(e *env.Env, hash string) {
	v.detailToken++
	token := v.detailToken
	v.detailLoading = true
	v.detailError = ""
	v.detail = protocol.GitCommitDetail{}
	v.status = protocol.GitStatusResult{}
	v.diff = protocol.DiffResult{}
	v.compareWith = ""
	v.selectedFile = ""
	v.fileScroll.ScrollTo(0)
	var detail protocol.GitCommitDetail
	e.Call(protocol.MGitShow, protocol.GitCommitParams{Project: v.project, Hash: hash}, &detail, func(err error) {
		if token != v.detailToken || v.selected.kind != rowCommit || v.selected.hash != hash {
			return
		}
		v.detailLoading = false
		if err != nil {
			v.detailError = err.Error()
			return
		}
		v.detail = detail
		if len(detail.Files) != 0 {
			v.selectedFile = detail.Files[0].Path
		}
	})
	var diff protocol.DiffResult
	e.Call(protocol.MGitDiff, protocol.DiffParams{Project: v.project, Mode: protocol.DiffCommit, Commit: hash, Context: 3}, &diff, func(err error) {
		if token == v.detailToken && err == nil && v.selected.kind == rowCommit && v.selected.hash == hash {
			v.diff = diff
			if v.selectedFile == "" && len(diff.Files) != 0 {
				v.selectedFile = diff.Files[0].Path
			}
		}
	})
}

func (v *viewState) loadWIP(e *env.Env, path string) {
	v.detailToken++
	token := v.detailToken
	v.detailLoading = true
	v.detailError = ""
	v.detail = protocol.GitCommitDetail{}
	v.status = protocol.GitStatusResult{}
	v.diff = protocol.DiffResult{}
	v.compareWith = ""
	v.selectedFile = ""
	v.fileScroll.ScrollTo(0)
	var status protocol.GitStatusResult
	e.Call(protocol.MGitStatus, protocol.WorktreeRef{Path: path}, &status, func(err error) {
		if token != v.detailToken || v.selected.kind != rowWIP || v.selected.worktree != path {
			return
		}
		v.detailLoading = false
		if err != nil {
			v.detailError = err.Error()
			return
		}
		v.status = status
		if len(status.Files) != 0 {
			v.selectedFile = status.Files[0].Path
		}
	})
	var diff protocol.DiffResult
	e.Call(protocol.MGitDiff, protocol.DiffParams{Worktree: path, Mode: protocol.DiffWorking, Context: 3}, &diff, func(err error) {
		if token == v.detailToken && err == nil && v.selected.kind == rowWIP && v.selected.worktree == path {
			v.diff = diff
			if v.selectedFile == "" && len(diff.Files) != 0 {
				v.selectedFile = diff.Files[0].Path
			}
		}
	})
}
