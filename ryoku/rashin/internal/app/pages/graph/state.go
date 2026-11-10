package graph

import (
	"sort"
	"strings"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/diffview"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/gitgraph"
	"ryoku-rashin/internal/protocol"
)

const (
	pageLimit      = 200
	collapsedLanes = 24
)

type rowKind uint8

const (
	rowCommit rowKind = iota + 1
	rowWIP
)

type graphEntry struct {
	kind   rowKind
	commit protocol.GitCommit
	layout gitgraph.Row
	wip    protocol.WipRow
}

func (entry graphEntry) id() string {
	if entry.kind == rowWIP {
		return "wip:" + entry.wip.Worktree
	}
	return "commit:" + entry.commit.Hash
}

type rowState struct {
	input   rowInput
	menu    kit.Menu
	menuTag struct{}
	expand  kit.Click
	refTip  look.Tip
	paths   graphPathCache
}

type selection struct {
	kind     rowKind
	hash     string
	worktree string
}

type viewState struct {
	project          string
	layoutState      *gitgraph.Layout
	commits          []protocol.GitCommit
	commitLayouts    []gitgraph.Row
	entries          []graphEntry
	refs             []protocol.GitRef
	wip              []protocol.WipRow
	worktrees        []protocol.WorktreeInfo
	next             string
	loading          bool
	loaded           bool
	generation       uint64
	refreshRequested bool
	error            string
	rows             map[string]*rowState
	scroll           look.Scroller
	fileScroll       look.Scroller
	search           kit.Search
	finder           kit.Search

	scopeChoice       look.Choice
	scope             string
	scopeRef          string
	scopeWorktree     string
	focusSearch       bool
	keyTag            struct{}
	focusGraph        bool
	selected          selection
	expandedLanes     bool
	laneMarkerVisible bool
	sideDetails       bool
	detailWidth       int
	detailOpen        bool

	detail        protocol.GitCommitDetail
	status        protocol.GitStatusResult
	diff          protocol.DiffResult
	compareWith   string
	selectedFile  string
	detailLoading bool
	detailError   string
	detailToken   uint64
	diffView      diffview.View
	fileRows      map[string]*kit.Click
	detailRefTip  look.Tip
	copyHash      kit.Plaque
	detailsBack   kit.Plaque
	parentButtons map[string]*kit.Plaque
	detailButtons map[string]*kit.Plaque

	actions          []Action
	operation        pendingOperation
	opResult         protocol.GitOpResult
	conflictWorktree string
	conflictButtons  map[string]*kit.Plaque
}

func (v *viewState) reset(project string) {
	v.generation++
	v.project = project
	v.layoutState = gitgraph.New()
	v.commits = nil
	v.commitLayouts = nil
	v.entries = nil
	v.refs = nil
	v.wip = nil
	v.worktrees = nil
	v.next = ""
	v.loading = false
	v.loaded = false
	v.error = ""
	v.rows = make(map[string]*rowState)
	v.scope = protocol.ScopeAll
	v.scopeRef = ""
	v.scopeWorktree = ""
	v.selected = selection{}
	v.focusGraph = true
	v.expandedLanes = false
	v.laneMarkerVisible = false
	v.sideDetails = false
	v.detailWidth = 0
	v.detailOpen = false
	v.detail = protocol.GitCommitDetail{}
	v.status = protocol.GitStatusResult{}
	v.diff = protocol.DiffResult{}
	v.compareWith = ""
	v.selectedFile = ""
	v.detailLoading = false
	v.detailError = ""
	v.fileRows = make(map[string]*kit.Click)
	v.parentButtons = make(map[string]*kit.Plaque)
	v.detailButtons = make(map[string]*kit.Plaque)
	v.opResult = protocol.GitOpResult{}
	v.conflictWorktree = ""
	v.actions = nil
	v.search.SetText("")
	v.finder.SetText("")
	v.search.Editor.SingleLine = true
	v.search.Editor.Submit = true
	v.finder.Editor.SingleLine = true
	v.finder.Editor.Submit = true
}

func (v *viewState) row(id string) *rowState {
	row := v.rows[id]
	if row == nil {
		row = new(rowState)
		v.rows[id] = row
	}
	return row
}

func (v *viewState) button(id string) *kit.Plaque {
	button := v.detailButtons[id]
	if button == nil {
		button = new(kit.Plaque)
		v.detailButtons[id] = button
	}
	return button
}

func (v *viewState) parentButton(hash string) *kit.Plaque {
	button := v.parentButtons[hash]
	if button == nil {
		button = new(kit.Plaque)
		v.parentButtons[hash] = button
	}
	return button
}

func (v *viewState) rebuildEntries() {
	wipByHead := make(map[string][]protocol.WipRow)
	for _, row := range v.wip {
		target := v.wipTarget(row)
		wipByHead[target] = append(wipByHead[target], row)
	}
	entries := make([]graphEntry, 0, len(v.commits)+len(v.wip))
	for index, commit := range v.commits {
		row := v.commitLayouts[index]
		for _, wip := range wipByHead[commit.Hash] {
			entries = append(entries, graphEntry{kind: rowWIP, layout: row, wip: wip})
		}
		entries = append(entries, graphEntry{kind: rowCommit, commit: commit, layout: row})
	}
	v.entries = entries
}
func (v *viewState) wipTarget(row protocol.WipRow) string {
	for _, commit := range v.commits {
		if commit.Hash == row.Head || (row.Head != "" && strings.HasPrefix(commit.Hash, row.Head)) {
			return commit.Hash
		}
	}
	for _, ref := range v.refs {
		if (row.Branch != "" && ref.Name == row.Branch) || (row.Worktree != "" && ref.Worktree == row.Worktree) {
			return ref.Target
		}
	}
	return row.Head
}

func (v *viewState) refsFor(hash string) []protocol.GitRef {
	var refs []protocol.GitRef
	for _, ref := range v.refs {
		if ref.Target == hash {
			refs = append(refs, ref)
		}
	}
	return refs
}
func (v *viewState) checkedOutAt(hash string) (string, bool) {
	for _, ref := range v.refs {
		if ref.Target == hash && ref.Worktree != "" {
			return ref.Worktree, true
		}
	}
	return "", false
}

func (v *viewState) scopeOptions() (labels []string, scopes []selectionScope, selected int) {
	labels = append(labels, i18n.T("All refs"))
	scopes = append(scopes, selectionScope{scope: protocol.ScopeAll})
	seen := make(map[string]bool)
	for _, ref := range v.refs {
		if ref.Kind != protocol.RefBranch || seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		labels = append(labels, i18n.T("Branch")+": "+ref.Name)
		scopes = append(scopes, selectionScope{scope: protocol.ScopeBranch, ref: ref.Name})
	}
	worktrees := append([]protocol.WorktreeInfo(nil), v.worktrees...)
	sort.SliceStable(worktrees, func(i, j int) bool { return worktrees[i].Name < worktrees[j].Name })
	for _, worktree := range worktrees {
		name := worktree.Name
		if name == "" {
			name = worktree.Path
		}
		labels = append(labels, i18n.T("Worktree")+": "+name)
		scopes = append(scopes, selectionScope{scope: protocol.ScopeWorktree, worktree: worktree.Path})
	}
	for index, option := range scopes {
		if option.scope == v.scope && option.ref == v.scopeRef && option.worktree == v.scopeWorktree {
			selected = index
			break
		}
	}
	return labels, scopes, selected
}

type selectionScope struct {
	scope    string
	ref      string
	worktree string
}

func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func (v *viewState) worktree(path string) (protocol.WorktreeInfo, bool) {
	for _, worktree := range v.worktrees {
		if worktree.Path == path {
			return worktree, true
		}
	}
	return protocol.WorktreeInfo{}, false
}

func worktreeDirty(worktree protocol.WorktreeInfo) bool {
	status := worktree.Status
	return status != nil && (status.Staged != 0 || status.Modified != 0 || status.Untracked != 0 || status.Conflicted != 0)
}

func (v *viewState) worktreeNames() ([]string, []protocol.WorktreeInfo) {
	values := append([]protocol.WorktreeInfo(nil), v.worktrees...)
	sort.SliceStable(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	labels := make([]string, len(values))
	for index, value := range values {
		labels[index] = value.Name
		if labels[index] == "" {
			labels[index] = value.Path
		}
		if value.Branch != "" {
			labels[index] += " · " + value.Branch
		}
	}
	return labels, values
}
