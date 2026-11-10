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
	"ryoku-rashin/internal/protocol"
)

func (v *viewState) layoutDetails(gtx layout.Context, e *env.Env) layout.Dimensions {
	return kit.Slab(gtx, e.Theme.R.Stone2, e.Theme.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 10, Right: 10, Bottom: 10, Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if !v.sideDetails {
				v.detailsBack.Kind, v.detailsBack.Label, v.detailsBack.Small = kit.PlaqueQuiet, i18n.T("Back"), true
				if v.detailsBack.Clicked(gtx) {
					v.detailOpen = false
					v.focusGraph = true
				}
			}
			body := func(gtx layout.Context) layout.Dimensions {
				if v.selected.kind == 0 {
					return (kit.Empty{
						Title: i18n.T("Commit details"),
						Body:  i18n.T("Select a commit or uncommitted changes row."),
					}).Layout(gtx, e.Theme.Theme)
				}
				if v.detailLoading {
					working := kit.Working{Active: true}
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return working.Layout(gtx, e.Theme.Theme) }),
							layout.Rigid(layout.Spacer{Width: 8}.Layout),
							layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Label, e.Theme.R.Fg2, i18n.T("Loading details"))),
						)
					})
				}
				if v.detailError != "" {
					return (kit.Empty{Title: i18n.T("Details unavailable"), Body: v.detailError}).Layout(gtx, e.Theme.Theme)
				}
				if v.selected.kind == rowWIP {
					return v.layoutWIPDetails(gtx, e)
				}
				return v.layoutCommitDetails(gtx, e)
			}
			if v.sideDetails {
				return body(gtx)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return v.detailsBack.Layout(gtx, e.Theme.Theme)
				}),
				layout.Rigid(layout.Spacer{Height: 6}.Layout),
				layout.Flexed(1, body),
			)
		})
	})
}

func (v *viewState) layoutCommitDetails(gtx layout.Context, e *env.Env) layout.Dimensions {
	commit := v.detail.Commit
	if commit.Hash == "" {
		commit.Hash = v.selected.hash
		for _, candidate := range v.commits {
			if candidate.Hash == commit.Hash {
				commit = candidate
				break
			}
		}
	}
	committer := strings.TrimSpace(v.detail.Committer)
	meta := commit.Author
	if committer != "" && committer != commit.Author {
		meta += " · " + i18n.T("committed by") + " " + committer
	}
	if !commit.Time.IsZero() {
		meta += " · " + commit.Time.Local().Format("2006-01-02 15:04")
	}
	v.copyHash.Kind, v.copyHash.Label, v.copyHash.Small = kit.PlaqueQuiet, i18n.T("Copy"), true
	if v.copyHash.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(commit.Hash))})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Title, e.Theme.R.Fg, commit.Subject)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			body := strings.TrimSpace(v.detail.Body)
			if body == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return (kit.Label{
					Style:    e.Theme.T.Body,
					Color:    e.Theme.R.Fg2,
					MaxLines: 4,
				}).Layout(gtx, e.Theme.Theme, body)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 6}.Layout(gtx, kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg2, meta))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg, commit.Hash)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.copyHash.Layout(gtx, e.Theme.Theme) }),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(commit.Parents) == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg3, i18n.T("Parents"))),
				}
				for _, hash := range commit.Parents {
					hash := hash
					button := v.parentButton(hash)
					button.Kind, button.Label, button.Small = kit.PlaqueQuiet, shortHash(hash), true
					if button.Clicked(gtx) {
						v.selected = selection{kind: rowCommit, hash: hash}
						v.loadCommit(e, hash)
					}
					children = append(children,
						layout.Rigid(layout.Spacer{Width: 3}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return button.Layout(gtx, e.Theme.Theme) }),
					)
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			refs := v.detail.Refs
			if len(refs) == 0 {
				refs = v.refsFor(commit.Hash)
			}
			if len(refs) == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutRefChips(gtx, e, refs, v.commitColor(commit.Hash), &v.detailRefTip)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return v.layoutFileSummary(gtx, e, v.detail.Files)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if v.compareWith == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 5, Bottom: 3}.Layout(gtx, kit.Claim(e.Theme.Theme, palette.Attention, i18n.T("Compared with")+" "+v.compareWith))
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return v.layoutSelectedDiff(gtx, e, i18n.T("Select a file to inspect its diff."))
		}),
	)
}

func (v *viewState) layoutWIPDetails(gtx layout.Context, e *env.Env) layout.Dimensions {
	path := v.selected.worktree
	worktree, _ := v.worktree(path)
	name := v.worktreeLabel(path)
	status := v.status.Status
	staged, modified, untracked, conflicted := status.Staged, status.Modified, status.Untracked, status.Conflicted
	for _, row := range v.wip {
		if row.Worktree == path {
			staged, modified, untracked, conflicted = row.Staged, row.Modified, row.Untracked, row.Conflicted
			break
		}
	}
	counts := strings.Join(nonZeroCounts(staged, modified, untracked, conflicted), " · ")
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Title, e.Theme.R.Fg, i18n.T("Uncommitted changes")+" · "+name)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			meta := strings.Trim(strings.Join([]string{v.status.Branch, path}, " · "), " ·")
			return layout.Inset{Top: 5}.Layout(gtx, kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, meta))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if counts == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 5}.Layout(gtx, kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, counts))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutWIPActions(gtx, e, worktree) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutConflict(gtx, e) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutFileSummary(gtx, e, v.status.Files) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return v.layoutSelectedDiff(gtx, e, i18n.T("Select a file to inspect its diff."))
		}),
	)
}

func (v *viewState) layoutWIPActions(gtx layout.Context, e *env.Env, worktree protocol.WorktreeInfo) layout.Dimensions {
	open := v.button("open-worktree")
	changes := v.button("open-changes")
	newSession := v.button("new-session")
	open.Kind, open.Label, open.Small = kit.PlaqueQuiet, i18n.T("Open worktree"), true
	changes.Kind, changes.Label, changes.Small = kit.PlaqueLine, i18n.T("Open changes"), true
	newSession.Kind, newSession.Label, newSession.Small = kit.PlaqueQuiet, i18n.T("New session"), true
	if open.Clicked(gtx) {
		v.actions = append(v.actions, Action{Kind: ActOpenWorktree, Project: v.project, Worktree: v.selected.worktree})
	}
	if changes.Clicked(gtx) {
		v.actions = append(v.actions, Action{Kind: ActOpenChanges, Project: v.project, Worktree: v.selected.worktree})
	}
	if newSession.Clicked(gtx) {
		v.actions = append(v.actions, Action{Kind: ActNewSession, Project: v.project, Worktree: v.selected.worktree})
	}
	var sessionButton *kit.Plaque
	if len(worktree.Sessions) > 0 {
		session := worktree.Sessions[len(worktree.Sessions)-1]
		sessionButton = v.button("open-session:" + session)
		sessionButton.Kind, sessionButton.Label, sessionButton.Small = kit.PlaqueQuiet, i18n.T("Open session"), true
		if sessionButton.Clicked(gtx) {
			v.actions = append(v.actions, Action{Kind: ActOpenSession, Project: v.project, Worktree: v.selected.worktree, Session: session})
		}
	}
	return layout.Inset{Top: 8, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return changes.Layout(gtx, e.Theme.Theme) }),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return open.Layout(gtx, e.Theme.Theme) }),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions { return newSession.Layout(gtx, e.Theme.Theme) })}
				if sessionButton != nil {
					children = append(children,
						layout.Rigid(layout.Spacer{Width: 6}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return sessionButton.Layout(gtx, e.Theme.Theme) }),
					)
				}
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
			}),
		)
	})
}

func (v *viewState) layoutConflict(gtx layout.Context, e *env.Env) layout.Dimensions {
	if len(v.opResult.Conflicts) == 0 || v.selected.worktree == "" || v.selected.worktree != v.conflictWorktree {
		return layout.Dimensions{}
	}
	abort := v.conflictButton("abort", i18n.T("Abort"), kit.PlaqueDangerLine)
	continueButton := v.conflictButton("continue", i18n.T("Continue"), kit.PlaqueLine)
	changes := v.conflictButton("conflict-changes", i18n.T("Open changes"), kit.PlaqueQuiet)
	resolve := v.conflictButton("resolve-agent", i18n.T("Resolve with agent"), kit.PlaqueQuiet)
	if abort.Clicked(gtx) {
		v.runConflictAction(e, protocol.MGitAbort)
	}
	if continueButton.Clicked(gtx) {
		v.runConflictAction(e, protocol.MGitContinue)
	}
	if changes.Clicked(gtx) {
		v.actions = append(v.actions, Action{Kind: ActOpenChanges, Project: v.project, Worktree: v.selected.worktree})
	}
	if resolve.Clicked(gtx) {
		v.actions = append(v.actions, Action{Kind: ActNewSession, Project: v.project, Worktree: v.selected.worktree})
	}
	body := i18n.Tf("%s stopped with conflicts in %s.", i18n.T(v.opResult.Operation), strings.Join(v.opResult.Conflicts, ", "))
	return layout.Inset{Top: 8, Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.SlabContent(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Claim(e.Theme.Theme, palette.Attention, i18n.T("Conflicts"))),
				layout.Rigid(layout.Spacer{Height: 6}.Layout),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Body, e.Theme.R.Fg, body)),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return abort.Layout(gtx, e.Theme.Theme) }),
								layout.Rigid(layout.Spacer{Width: 6}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return continueButton.Layout(gtx, e.Theme.Theme) }),
							)
						}),
						layout.Rigid(layout.Spacer{Height: 6}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return changes.Layout(gtx, e.Theme.Theme) }),
								layout.Rigid(layout.Spacer{Width: 6}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return resolve.Layout(gtx, e.Theme.Theme) }),
							)
						}),
					)
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

func (v *viewState) layoutFileSummary(gtx layout.Context, e *env.Env, files []protocol.FileChange) layout.Dimensions {
	if len(files) == 0 {
		return layout.Dimensions{}
	}
	return layout.Inset{Top: 7, Bottom: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Group, e.Theme.R.Fg3, i18n.Tf("Files · %d", len(files)))),
			layout.Rigid(layout.Spacer{Height: 3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				rowHeight := gtx.Dp(24)
				gtx.Constraints.Min.Y = 0
				gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, rowHeight*min(5, len(files)))
				return v.fileScroll.Layout(gtx, e.Theme, len(files), func(gtx layout.Context, index int) layout.Dimensions {
					return v.layoutFileRow(gtx, e, files[index])
				})
			}),
		)
	})
}

func (v *viewState) layoutFileRow(gtx layout.Context, e *env.Env, file protocol.FileChange) layout.Dimensions {
	control := v.fileRows[file.Path]
	if control == nil {
		control = new(kit.Click)
		v.fileRows[file.Path] = control
	}
	if control.Clicked(gtx) {
		v.selectedFile = file.Path
	}
	height := gtx.Dp(24)
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	return control.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		background := e.Theme.R.Stone2
		if file.Path == v.selectedFile {
			background = e.Theme.R.Stone3
		} else if control.Hovered() {
			background = palette.Mix(e.Theme.R.Stone2, e.Theme.R.Fg, .06)
		}
		return kit.Slab(gtx, background, e.Theme.Cut.Token, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: 6, Left: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(18), gtx.Dp(18)
						ink := e.Theme.R.Fg3
						if file.Status == protocol.ChangeConflict {
							ink = e.Theme.R.Jasper
						}
						return kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, ink, file.Status)(gtx)
					}),
					layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg, file.Path)),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, fmt.Sprintf("+%d -%d", file.Added, file.Removed))),
				)
			})
		})
	})
}

func (v *viewState) layoutSelectedDiff(gtx layout.Context, e *env.Env, empty string) layout.Dimensions {
	if v.selectedFile == "" {
		return (kit.Empty{Title: i18n.T("No file selected"), Body: empty}).Layout(gtx, e.Theme.Theme)
	}
	for _, file := range v.diff.Files {
		if file.Path == v.selectedFile {
			return v.diffView.Layout(gtx, e.Theme.Theme, protocol.DiffResult{
				Files:     []protocol.FileDiff{file},
				Truncated: v.diff.Truncated,
			}, diffview.Options{FoldOver: 300})
		}
	}
	return (kit.Empty{
		Title: i18n.T("No text diff"),
		Body:  i18n.T("This file has no text diff to show."),
	}).Layout(gtx, e.Theme.Theme)
}

func (v *viewState) commitColor(hash string) int {
	for index, commit := range v.commits {
		if commit.Hash == hash && index < len(v.commitLayouts) {
			return v.commitLayouts[index].Color
		}
	}
	return 0
}
