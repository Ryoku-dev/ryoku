package diffview

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type lineKey struct {
	path string
	hunk int
	line int
}

type fileHeader struct {
	row                                 kit.Click
	fold, stage, unstage, discard, open kit.Plaque
}

type lineControl struct {
	gutter  kit.Click
	hscroll layout.List
}

type commentControl struct {
	resolve kit.Plaque
	remove  kit.Plaque
}

type diffItem struct {
	kind    int
	file    int
	hunk    int
	line    int
	comment int
}

const (
	itemFile = iota
	itemHunk
	itemLine
	itemComment
	itemFolded
	itemTruncated
)

const (
	diffGutterWidth = 76
	diffTextInset   = 84
)

func (v *View) init() {
	if v.folded == nil {
		v.folded = make(map[string]bool)
		v.headers = make(map[string]*fileHeader)
		v.lines = make(map[lineKey]*lineControl)
		v.comments = make(map[string]*commentControl)
	}
}

func (v *View) layout(
	gtx layout.Context,
	th *theme.Theme,
	result protocol.DiffResult,
	options Options,
) layout.Dimensions {
	v.init()
	items, fileIndexes := v.items(result, options)
	if v.scrollPath != "" {
		if index, ok := fileIndexes[v.scrollPath]; ok {
			v.scroll.ScrollTo(index)
		}
		v.scrollPath = ""
	}
	appTheme := &look.Theme{Theme: th}
	return v.scroll.Layout(gtx, appTheme, len(items), func(
		gtx layout.Context,
		index int,
	) layout.Dimensions {
		item := items[index]
		switch item.kind {
		case itemFile:
			return v.layoutFile(gtx, th, result.Files[item.file], options)
		case itemHunk:
			return v.layoutHunk(gtx, th, result.Files[item.file].Hunks[item.hunk])
		case itemLine:
			file := result.Files[item.file]
			line := file.Hunks[item.hunk].Lines[item.line]
			return v.layoutLine(gtx, th, file.Path, item.hunk, item.line, line, options)
		case itemComment:
			return v.layoutComment(gtx, th, options.Comments[item.comment])
		case itemFolded:
			return layout.Inset{
				Top:    4,
				Bottom: 8,
				Left:   diffTextInset,
			}.Layout(
				gtx,
				kit.Text(
					th,
					th.T.Caption,
					th.R.Fg3,
					i18n.T("More lines are folded. Open the file header to show them."),
				),
			)
		default:
			return layout.Inset{Top: 8, Bottom: 8}.Layout(
				gtx,
				kit.Text(
					th,
					th.T.Caption,
					th.R.Amber,
					i18n.T("This diff was truncated."),
				),
			)
		}
	})
}

func (v *View) items(result protocol.DiffResult, options Options) ([]diffItem, map[string]int) {
	items := make([]diffItem, 0)
	files := make(map[string]int, len(result.Files))
	for fi, file := range result.Files {
		files[file.Path] = len(items)
		items = append(items, diffItem{kind: itemFile, file: fi})
		lineCount := 0
		for _, hunk := range file.Hunks {
			lineCount += len(hunk.Lines)
		}
		fold := options.FoldOver > 0 && lineCount > options.FoldOver && !v.folded[file.Path]
		shown := 0
		for hi, hunk := range file.Hunks {
			if fold && shown >= options.FoldOver {
				break
			}
			items = append(items, diffItem{kind: itemHunk, file: fi, hunk: hi})
			for li, line := range hunk.Lines {
				if fold && shown >= options.FoldOver {
					break
				}
				items = append(items, diffItem{kind: itemLine, file: fi, hunk: hi, line: li})
				shown++
				for ci, comment := range options.Comments {
					lineNumber, side := line.New, "new"
					if comment.Side == "old" {
						lineNumber, side = line.Old, "old"
					}
					if comment.Path == file.Path && comment.Line == lineNumber && comment.Side == side {
						items = append(items, diffItem{kind: itemComment, file: fi, hunk: hi, line: li, comment: ci})
					}
				}
			}
		}
		if fold {
			items = append(items, diffItem{kind: itemFolded, file: fi})
		}
	}
	if result.Truncated {
		items = append(items, diffItem{kind: itemTruncated})
	}
	return items, files
}

func (v *View) layoutFile(
	gtx layout.Context,
	th *theme.Theme,
	file protocol.FileDiff,
	options Options,
) layout.Dimensions {
	header := v.headers[file.Path]
	if header == nil {
		header = new(fileHeader)
		v.headers[file.Path] = header
	}
	if header.row.Clicked(gtx) || header.fold.Clicked(gtx) {
		v.folded[file.Path] = !v.folded[file.Path]
	}
	if header.stage.Clicked(gtx) {
		v.pending = append(v.pending, Action{Kind: ActStage, Path: file.Path})
	}
	if header.unstage.Clicked(gtx) {
		v.pending = append(v.pending, Action{Kind: ActUnstage, Path: file.Path})
	}
	if header.discard.Clicked(gtx) {
		v.pending = append(v.pending, Action{Kind: ActDiscard, Path: file.Path})
	}
	if header.open.Clicked(gtx) {
		v.pending = append(v.pending, Action{Kind: ActOpen, Path: file.Path})
	}
	path := file.Path
	if file.OldPath != "" && file.OldPath != file.Path {
		path = file.OldPath + " → " + file.Path
	}
	counts := fmt.Sprintf("+%d -%d", file.Added, file.Removed)
	header.fold.Kind, header.fold.Small = kit.PlaqueQuiet, true
	if v.folded[file.Path] {
		header.fold.Label = i18n.T("Unfold")
	} else {
		header.fold.Label = i18n.T("Fold")
	}
	header.stage.Kind, header.stage.Label, header.stage.Small =
		kit.PlaqueQuiet, i18n.T("Stage"), true
	header.unstage.Kind, header.unstage.Label, header.unstage.Small =
		kit.PlaqueQuiet, i18n.T("Unstage"), true
	header.discard.Kind, header.discard.Label, header.discard.Small =
		kit.PlaqueDangerLine, i18n.T("Discard"), true
	header.open.Kind, header.open.Label, header.open.Small =
		kit.PlaqueQuiet, i18n.T("Open"), true
	return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(
		gtx layout.Context,
	) layout.Dimensions {
		return header.row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			background := th.R.Stone2
			if header.row.Hovered() {
				background = palette.Mix(background, th.R.Fg, .06)
			}
			return kit.Slab(gtx, background, th.Cut.Small, func(
				gtx layout.Context,
			) layout.Dimensions {
				return layout.Inset{Top: 5, Right: 8, Bottom: 5, Left: 8}.Layout(
					gtx,
					func(gtx layout.Context) layout.Dimensions {
						children := []layout.FlexChild{
							layout.Rigid(kit.Text(
								th,
								th.T.MonoSm,
								th.R.Fg3,
								statusLetter(file.Status),
							)),
							layout.Rigid(layout.Spacer{Width: 8}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								label := kit.Label{
									Style:    th.T.MonoSm,
									Color:    th.R.Fg,
									MaxLines: 1,
								}
								return label.Layout(gtx, th, path)
							}),
							layout.Rigid(layout.Spacer{Width: 8}.Layout),
							layout.Rigid(kit.Text(th, th.T.MonoSm, th.R.Fg3, counts)),
						}
						if header.row.Hovered() {
							children = append(
								children,
								layout.Rigid(layout.Spacer{Width: 4}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return header.fold.Layout(gtx, th)
								}),
							)
							if options.FileActions {
								children = append(
									children,
									layout.Rigid(layout.Spacer{Width: 4}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return header.open.Layout(gtx, th)
									}),
									layout.Rigid(layout.Spacer{Width: 4}.Layout),
								)
								if options.Staged {
									children = append(
										children,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return header.unstage.Layout(gtx, th)
										}),
									)
								} else {
									children = append(
										children,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return header.stage.Layout(gtx, th)
										}),
									)
								}
								children = append(
									children,
									layout.Rigid(layout.Spacer{Width: 4}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return header.discard.Layout(gtx, th)
									}),
								)
							}
						}
						return layout.Flex{
							Axis:      layout.Horizontal,
							Alignment: layout.Middle,
						}.Layout(gtx, children...)
					},
				)
			})
		})
	})
}

func (v *View) layoutHunk(
	gtx layout.Context,
	th *theme.Theme,
	hunk protocol.Hunk,
) layout.Dimensions {
	return layout.Inset{
		Top:    4,
		Bottom: 4,
		Left:   diffTextInset,
	}.Layout(gtx, kit.Text(th, th.T.MonoSm, th.R.Lapis, hunk.Header))
}

func (v *View) layoutLine(
	gtx layout.Context,
	th *theme.Theme,
	path string,
	hunkIndex int,
	lineIndex int,
	line protocol.DiffLine,
	options Options,
) layout.Dimensions {
	key := lineKey{path: path, hunk: hunkIndex, line: lineIndex}
	control := v.lines[key]
	if control == nil {
		control = new(lineControl)
		control.hscroll.Axis = layout.Horizontal
		v.lines[key] = control
	}
	if control.gutter.Clicked(gtx) && options.Commentable {
		v.commentPath = path
		v.commentLine, v.commentSide = line.New, "new"
		if line.New == 0 {
			v.commentLine, v.commentSide = line.Old, "old"
		}
		v.commentField.SetText("")
		v.commentFocus = true
	}
	background, ink := th.R.Stone1, th.R.Fg2
	switch line.Kind {
	case protocol.LineAdd:
		background, ink = palette.Mix(th.R.Stone1, th.R.Jade, .13), th.R.Fg
	case protocol.LineDel:
		background, ink = palette.Mix(th.R.Stone1, th.R.Jasper, .12), th.R.Fg
	}
	lineWidget := func(gtx layout.Context) layout.Dimensions {
		width := gtx.Constraints.Max.X
		if !options.Wrap {
			characterWidth := max(1, gtx.Sp(th.T.MonoSm.Size)*3/5)
			width = gtx.Dp(diffTextInset) + len([]rune(line.Text))*characterWidth
		}
		if width <= 0 {
			width = 1
		}
		height := max(gtx.Dp(28), gtx.Sp(th.T.MonoSm.Line)+gtx.Dp(8))
		paint.FillShape(gtx.Ops, background, clip.Rect(image.Rect(0, 0, width, height)).Op())
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gutterWidth := gtx.Dp(diffGutterWidth)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = gutterWidth, gutterWidth
				marker := "  "
				hovered := options.Commentable && control.gutter.Hovered()
				if hovered {
					marker = "+ "
				}
				numbers := fmt.Sprintf("%4s %4s", number(line.Old), number(line.New))
				return control.gutter.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					color := th.R.Fg3
					if hovered {
						color = th.R.GoldInk
					}
					return layout.Center.Layout(
						gtx,
						kit.Text(th, th.T.MonoSm, color, marker+numbers),
					)
				})
			}),
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				prefix := line.Kind
				if prefix == protocol.LineContext {
					prefix = " "
				}
				label := kit.Label{Style: th.T.MonoSm, Color: ink}
				if !options.Wrap {
					label.MaxLines = 1
				}
				return label.Layout(gtx, th, prefix+line.Text)
			}),
		)
	}
	lineChild := func(gtx layout.Context) layout.Dimensions {
		if options.Wrap {
			return lineWidget(gtx)
		}
		return control.hscroll.Layout(gtx, 1, func(
			gtx layout.Context,
			_ int,
		) layout.Dimensions {
			return lineWidget(gtx)
		})
	}
	fieldChild := func(gtx layout.Context) layout.Dimensions {
		if !v.commentFieldMatches(path, line) {
			return layout.Dimensions{}
		}
		v.commentField.Label = i18n.T("Review comment")
		v.commentField.Placeholder = i18n.T("Write a comment and press Enter")
		v.commentField.Editor.Submit = true
		for {
			event, ok := v.commentField.Editor.Update(gtx)
			if !ok {
				break
			}
			if submit, ok := event.(widget.SubmitEvent); ok {
				text := strings.TrimSpace(submit.Text)
				if text != "" {
					v.pending = append(v.pending, Action{
						Kind: ActComment,
						Path: path,
						Line: v.commentLine,
						Side: v.commentSide,
						Text: text,
					})
				}
				v.commentPath, v.commentLine, v.commentSide = "", 0, ""
				v.commentField.SetText("")
			}
		}
		return layout.Inset{
			Top:    4,
			Right:  10,
			Bottom: 7,
			Left:   diffTextInset,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			dimensions := v.commentField.Layout(gtx, th)
			if v.commentFocus {
				v.commentField.Focus(gtx)
				v.commentFocus = false
			}
			return dimensions
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(lineChild),
		layout.Rigid(fieldChild),
	)
}

func (v *View) commentFieldMatches(path string, line protocol.DiffLine) bool {
	if v.commentPath != path || v.commentLine == 0 {
		return false
	}
	return v.commentSide == "new" && line.New == v.commentLine ||
		v.commentSide == "old" && line.Old == v.commentLine
}

func (v *View) layoutComment(
	gtx layout.Context,
	th *theme.Theme,
	comment protocol.ReviewComment,
) layout.Dimensions {
	control := v.comments[comment.ID]
	if control == nil {
		control = new(commentControl)
		v.comments[comment.ID] = control
	}
	if control.resolve.Clicked(gtx) {
		v.pending = append(v.pending, Action{
			Kind: ActResolve,
			Path: comment.Path,
			Line: comment.Line,
			Side: comment.Side,
			ID:   comment.ID,
		})
	}
	if control.remove.Clicked(gtx) {
		v.pending = append(v.pending, Action{
			Kind: ActDelete,
			Path: comment.Path,
			Line: comment.Line,
			Side: comment.Side,
			ID:   comment.ID,
		})
	}
	if comment.Resolved {
		control.resolve.Label = i18n.T("Reopen")
	} else {
		control.resolve.Label = i18n.T("Resolve")
	}
	control.resolve.Kind, control.resolve.Small = kit.PlaqueLine, true
	control.remove.Kind, control.remove.Label, control.remove.Small =
		kit.PlaqueDangerLine, i18n.T("Delete"), true
	return layout.Inset{
		Top:    3,
		Right:  10,
		Bottom: 7,
		Left:   diffTextInset,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(
			gtx,
			th.R.Field(palette.Attention),
			th.Cut.Small,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 7, Right: 8, Bottom: 7, Left: 8}.Layout(
					gtx,
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis:      layout.Horizontal,
							Alignment: layout.Middle,
						}.Layout(gtx,
							layout.Flexed(1, kit.Text(
								th,
								th.T.Body,
								th.R.Fg,
								comment.Text,
							)),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return control.resolve.Layout(gtx, th)
							}),
							layout.Rigid(layout.Spacer{Width: 4}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return control.remove.Layout(gtx, th)
							}),
						)
					},
				)
			},
		)
	})
}

func number(value int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprint(value)
}

func statusLetter(status string) string {
	switch status {
	case protocol.ChangeAdded:
		return "A"
	case protocol.ChangeDeleted:
		return "D"
	case protocol.ChangeRenamed:
		return "R"
	case protocol.ChangeCopied:
		return "C"
	case protocol.ChangeType:
		return "T"
	case protocol.ChangeConflict:
		return "!"
	case protocol.ChangeUntracked:
		return "?"
	default:
		return "M"
	}
}
