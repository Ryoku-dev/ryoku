package graph

import (
	"fmt"
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/diffview"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func (v *viewState) layoutDetails(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	body := func(gtx layout.Context) layout.Dimensions {
		switch {
		case v.selected.kind == 0:
			return kit.Empty{
				Icon: look.Icon.Graph, Title: i18n.T("Nothing selected"), Body: i18n.T("Choose a commit to read what it changed."),
			}.Layout(gtx, th.Theme)
		case v.detailError != "":
			return kit.Empty{Icon: look.Icon.Warning, Title: i18n.T("The details could not be read"), Body: v.detailError}.Layout(gtx, th.Theme)
		}
		key := fmt.Sprint(v.selected.kind, "\x00", v.selected.hash, "\x00", v.selected.worktree)
		return v.rise.Layout(gtx, th, key, 8, func(gtx layout.Context) layout.Dimensions {
			if v.selected.kind == rowWIP {
				return v.layoutWIPDetails(gtx, e)
			}
			return v.layoutCommitDetails(gtx, e)
		})
	}
	if v.sideDetails {
		return body(gtx)
	}
	v.detailsBack.Kind, v.detailsBack.Small, v.detailsBack.Icon, v.detailsBack.Label = kit.PlaqueQuiet, true, look.Icon.Back, i18n.T("Graph")
	if v.detailsBack.Clicked(gtx) {
		v.detailOpen, v.focusGraph = false, true
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.detailsBack.Layout(gtx, th.Theme)
			})
		}),
		layout.Flexed(1, body),
	)
}

func (v *viewState) layoutDetailFrame(gtx layout.Context, e *env.Env, head layout.Widget, none string) layout.Dimensions {
	th := e.Theme
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S4, Right: th.Sp.S4, Bottom: th.Sp.S3, Left: th.Sp.S4}.Layout(gtx, head)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Rule(gtx, th) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(v.diff.Files) == 0 && !v.diffLoading {
				return kit.Empty{Icon: look.Icon.Diff, Title: i18n.T("No changes"), Body: none}.Layout(gtx, th.Theme)
			}
			return layout.Inset{Top: th.Sp.S2, Right: th.Sp.S2, Left: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if len(v.diff.Files) == 0 {
					return look.SkeletonRows(gtx, th, 4, false)
				}
				// fills the pane so the wheel works anywhere in it
				gtx.Constraints.Min = gtx.Constraints.Max
				v.diffView.Layout(gtx, th, v.diff, diffview.Options{Overlays: e.Overlays, Collapsed: true, FoldOver: 200})
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
	)
}

// from its details once they arrive, from its row until then
func (v *viewState) selectedCommit() protocol.GitCommit {
	if v.detail.Commit.Hash != "" {
		return v.detail.Commit
	}
	for _, candidate := range v.commits {
		if candidate.Hash == v.selected.hash {
			return candidate
		}
	}
	return protocol.GitCommit{Hash: v.selected.hash}
}

func (v *viewState) layoutCommitDetails(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	commit := v.selectedCommit()
	head := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Title, Color: th.R.Fg, MaxLines: 3}.Layout(gtx, th.Theme, commit.Subject)
			}),
		}
		if body := strings.TrimSpace(v.detail.Body); body != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Body, Color: th.R.Fg2, MaxLines: 6}.Layout(gtx, th.Theme, body)
				}),
			)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Facts(gtx, th, v.commitFacts(gtx, e, commit)) }),
		)
		refs := v.detail.Refs
		if len(refs) == 0 {
			refs = v.refsFor(commit.Hash)
		}
		if len(refs) > 0 {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return v.layoutRefChips(gtx, e, refs, v.commitColor(commit.Hash), &v.detailRefTip)
				}),
			)
		}
		if v.compareWith != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(kit.Claim(th.Theme, palette.Attention, i18n.Tf("Compared with %s", v.compareWith))),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	return v.layoutDetailFrame(gtx, e, head, i18n.T("This commit changed no files."))
}

func (v *viewState) commitFacts(gtx layout.Context, e *env.Env, commit protocol.GitCommit) []look.FactRow {
	th := e.Theme
	rows := make([]look.FactRow, 0, 5)
	if commit.Author != "" {
		rows = append(rows, look.FactRow{Label: i18n.T("Author"), Value: commit.Author})
	}
	if committer := strings.TrimSpace(v.detail.Committer); committer != "" && committer != commit.Author {
		rows = append(rows, look.FactRow{Label: i18n.T("Committed by"), Value: committer})
	}
	if !commit.Time.IsZero() {
		rows = append(rows, look.FactRow{Label: i18n.T("Date"), Value: commit.Time.Local().Format("2 Jan 2006, 15:04")})
	}
	v.copyHash.Icon, v.copyHash.Description = look.Icon.Copy, i18n.T("Copy the full hash")
	if v.copyHash.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(commit.Hash))})
		e.Notify(i18n.T("Copied the commit hash"))
	}
	rows = append(rows, look.FactRow{Label: i18n.T("Commit"), Widget: func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(look.TokenChip(th, shortHash(commit.Hash))),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.copyTip.Layout(gtx, th, e.Overlays, v.copyHash.Description, func(gtx layout.Context) layout.Dimensions {
					return v.copyHash.Layout(gtx, th, th.R.Fg3)
				})
			}),
		)
	}})
	if len(commit.Parents) > 0 {
		label := i18n.T("Parent")
		if len(commit.Parents) > 1 {
			label = i18n.T("Parents")
		}
		rows = append(rows, look.FactRow{Label: label, Widget: func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2*len(commit.Parents))
			for _, hash := range commit.Parents {
				button := v.parentButton(hash)
				button.Kind, button.Small, button.Label = kit.PlaqueQuiet, true, shortHash(hash)
				if button.Clicked(gtx) {
					v.selected = selection{kind: rowCommit, hash: hash}
					v.loadCommit(e, hash)
				}
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return button.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
				)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		}})
	}
	return rows
}

func (v *viewState) layoutWIPDetails(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	path := v.selected.worktree
	worktree, _ := v.worktree(path)
	status := v.status.Status
	staged, modified, untracked, conflicted := status.Staged, status.Modified, status.Untracked, status.Conflicted
	agent := ""
	for _, row := range v.wip {
		if row.Worktree == path {
			staged, modified, untracked, conflicted = row.Staged, row.Modified, row.Untracked, row.Conflicted
			agent = strings.TrimSpace(row.Agent)
			break
		}
	}
	counts := strings.Join(nonZeroCounts(staged, modified, untracked, conflicted), ", ")
	facts := []look.FactRow{{Label: i18n.T("Worktree"), Value: v.worktreeLabel(path)}}
	if v.status.Branch != "" {
		facts = append(facts, look.FactRow{Label: i18n.T("Branch"), Widget: look.TokenChip(th, v.status.Branch)})
	}
	facts = append(facts, look.FactRow{Label: i18n.T("Folder"), Value: paths.Short(path), Mono: true})
	if agent != "" {
		facts = append(facts, look.FactRow{Label: i18n.T("Agent"), Widget: kit.Claim(th.Theme, agentTone(agent), agentLabel(agent))})
	}
	head := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Uncommitted changes"))),
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, counts)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Facts(gtx, th, facts) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutWIPActions(gtx, e, worktree) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutConflict(gtx, e) }),
		)
	}
	return v.layoutDetailFrame(gtx, e, head, i18n.T("Everything in this worktree is committed."))
}

func (v *viewState) layoutWIPActions(gtx layout.Context, e *env.Env, worktree protocol.WorktreeInfo) layout.Dimensions {
	th := e.Theme
	type act struct {
		button *kit.Plaque
		action Action
	}
	target := Action{Project: v.project, Worktree: v.selected.worktree}
	plaque := func(id, label string, kind kit.PlaqueKind) *kit.Plaque {
		button := v.button(id)
		button.Kind, button.Small, button.Label = kind, true, label
		return button
	}
	acts := []act{
		{plaque("open-changes", i18n.T("Open changes"), kit.PlaqueTonal), target.as(ActOpenChanges)},
		{plaque("new-session", i18n.T("New session"), kit.PlaqueTonal), target.as(ActNewSession)},
	}
	if count := len(worktree.Sessions); count > 0 {
		session := target.as(ActOpenSession)
		session.Session = worktree.Sessions[count-1]
		acts = append(acts, act{plaque("open-session", i18n.T("Open session"), kit.PlaqueTonal), session})
	}
	acts = append(acts, act{plaque("open-worktree", i18n.T("Open worktree"), kit.PlaqueTonal), target.as(ActOpenWorktree)})
	for _, item := range acts {
		if item.button.Clicked(gtx) {
			v.actions = append(v.actions, item.action)
		}
	}
	return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(acts), func(gtx layout.Context, index int) layout.Dimensions {
		return acts[index].button.Layout(gtx, th.Theme)
	})
}

func (action Action) as(kind ActionKind) Action {
	action.Kind = kind
	return action
}

func (v *viewState) layoutConflict(gtx layout.Context, e *env.Env) layout.Dimensions {
	if len(v.opResult.Conflicts) == 0 || v.selected.worktree == "" || v.selected.worktree != v.conflictWorktree {
		return layout.Dimensions{}
	}
	th := e.Theme
	proceed := v.conflictButton("continue", i18n.T("Continue"), kit.PlaqueTonal)
	abort := v.conflictButton("abort", i18n.T("Abort"), kit.PlaqueDangerLine)
	changes := v.conflictButton("conflict-changes", i18n.T("Open changes"), kit.PlaqueTonal)
	resolve := v.conflictButton("resolve-agent", i18n.T("Resolve with an agent"), kit.PlaqueTonal)
	if abort.Clicked(gtx) {
		v.runConflictAction(e, protocol.MGitAbort)
	}
	if proceed.Clicked(gtx) {
		v.runConflictAction(e, protocol.MGitContinue)
	}
	target := Action{Project: v.project, Worktree: v.selected.worktree}
	if changes.Clicked(gtx) {
		v.actions = append(v.actions, target.as(ActOpenChanges))
	}
	if resolve.Clicked(gtx) {
		v.actions = append(v.actions, target.as(ActNewSession))
	}
	buttons := [...]*kit.Plaque{proceed, abort, changes, resolve}
	body := i18n.Tf("%s stopped on conflicts in %s.", i18n.T(v.opResult.Operation), strings.Join(v.opResult.Conflicts, ", "))
	return layout.Inset{Top: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Claim(th.Theme, palette.Attention, i18n.T("Conflicts"))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg, body)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(buttons), func(gtx layout.Context, index int) layout.Dimensions {
						return buttons[index].Layout(gtx, th.Theme)
					})
				}),
			)
		})
	})
}

func (v *viewState) conflictButton(id, label string, kind kit.PlaqueKind) *kit.Plaque {
	button := v.conflictButtons[id]
	if button == nil {
		button = new(kit.Plaque)
		v.conflictButtons[id] = button
	}
	button.Kind, button.Label, button.Small = kind, label, true
	return button
}

func (v *viewState) commitColor(hash string) int {
	for index, commit := range v.commits {
		if commit.Hash == hash && index < len(v.commitLayouts) {
			return v.commitLayouts[index].Color
		}
	}
	return 0
}
