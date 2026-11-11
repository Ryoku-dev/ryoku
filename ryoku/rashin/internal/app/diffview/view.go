package diffview

import (
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

const (
	numberColumn = 36
	signColumn   = 20
	textInset    = 2*numberColumn + signColumn
	fileRow      = 36
)

type lineKey struct {
	path string
	hunk int
	line int
}

type fileHeader struct {
	row                           kit.Click
	hover                         look.Hover
	press                         anim.Tween
	open, stage, unstage, discard look.IconAction
	tips                          [3]look.Tip
}

type lineControl struct {
	gutter kit.Click
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
	hidden  int
}

const (
	itemFile = iota
	itemHunk
	itemLine
	itemComment
	itemMore
	itemTruncated
)

func (v *View) init() {
	if v.toggled == nil {
		v.toggled = make(map[string]bool)
		v.whole = make(map[string]bool)
		v.headers = make(map[string]*fileHeader)
		v.lines = make(map[lineKey]*lineControl)
		v.comments = make(map[string]*commentControl)
		v.more = make(map[string]*kit.Click)
	}
}

func (v *View) unfolded(path string, options Options) bool {
	return options.Headless || options.Collapsed == v.toggled[path]
}

func (v *View) layout(gtx layout.Context, th *look.Theme, result protocol.DiffResult, options Options) layout.Dimensions {
	v.init()
	if v.scrollPath != "" && !v.unfolded(v.scrollPath, options) {
		v.toggled[v.scrollPath] = !v.toggled[v.scrollPath]
	}
	items, fileIndexes := v.items(result, options)
	if v.scrollPath != "" {
		if index, ok := fileIndexes[v.scrollPath]; ok {
			v.scroll.ScrollTo(index)
		}
		v.scrollPath = ""
	}
	return v.scroll.Layout(gtx, th, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		item := items[index]
		switch item.kind {
		case itemFile:
			return v.layoutFile(gtx, th, result.Files[item.file], options)
		case itemHunk:
			return v.layoutHunk(gtx, th, result.Files[item.file].Hunks[item.hunk])
		case itemLine:
			file := result.Files[item.file]
			return v.layoutLine(gtx, th, file.Path, item, file.Hunks[item.hunk].Lines[item.line], options)
		case itemComment:
			return v.layoutComment(gtx, th, options.Comments[item.comment])
		case itemMore:
			return v.layoutMore(gtx, th, result.Files[item.file].Path, item.hidden)
		default:
			return layout.Inset{Top: 8, Bottom: 8, Left: textInset}.Layout(gtx,
				kit.Text(th.Theme, th.T.Caption, th.R.Amber, i18n.T("This diff is too long to show whole.")))
		}
	})
}

func (v *View) items(result protocol.DiffResult, options Options) ([]diffItem, map[string]int) {
	items := make([]diffItem, 0, 64)
	files := make(map[string]int, len(result.Files))
	for fileIndex, file := range result.Files {
		files[file.Path] = len(items)
		if !options.Headless {
			items = append(items, diffItem{kind: itemFile, file: fileIndex})
		}
		if !v.unfolded(file.Path, options) {
			continue
		}
		total := 0
		for _, hunk := range file.Hunks {
			total += len(hunk.Lines)
		}
		limit := total
		if options.FoldOver > 0 && total > options.FoldOver && !v.whole[file.Path] {
			limit = options.FoldOver
		}
		shown := 0
		for hunkIndex, hunk := range file.Hunks {
			if shown >= limit {
				break
			}
			items = append(items, diffItem{kind: itemHunk, file: fileIndex, hunk: hunkIndex})
			for lineIndex, line := range hunk.Lines {
				if shown >= limit {
					break
				}
				items = append(items, diffItem{kind: itemLine, file: fileIndex, hunk: hunkIndex, line: lineIndex})
				shown++
				for commentIndex, comment := range options.Comments {
					if commentPins(comment, file.Path, line) {
						items = append(items, diffItem{kind: itemComment, comment: commentIndex})
					}
				}
			}
		}
		if limit < total {
			items = append(items, diffItem{kind: itemMore, file: fileIndex, hidden: total - limit})
		}
	}
	if result.Truncated {
		items = append(items, diffItem{kind: itemTruncated})
	}
	return items, files
}

func commentPins(comment protocol.ReviewComment, path string, line protocol.DiffLine) bool {
	if comment.Path != path {
		return false
	}
	if comment.Side == "old" {
		return comment.Line == line.Old
	}
	return comment.Line == line.New
}

func (v *View) layoutFile(gtx layout.Context, th *look.Theme, file protocol.FileDiff, options Options) layout.Dimensions {
	header := v.headers[file.Path]
	if header == nil {
		header = new(fileHeader)
		v.headers[file.Path] = header
	}
	if header.row.Clicked(gtx) {
		v.toggled[file.Path] = !v.toggled[file.Path]
	}
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(fileRow))
	gtx.Constraints = layout.Exact(size)
	record := op.Record(gtx.Ops)
	_, hover := header.hover.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return header.row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			v.drawFile(gtx, th, header, file, options)
			return layout.Dimensions{Size: size}
		})
	})
	call := record.Stop()
	bounds := image.Rectangle{Max: size}
	cut := gtx.Dp(th.Cut.Small)
	if hover > 0 {
		kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Stone3, hover))
	}
	if press := look.Transition(gtx, &header.press, header.row.Pressed(), th.Motion.Press, th.Motion.Press); press > 0 {
		kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Fg, .08*press))
	}
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

func (v *View) drawFile(gtx layout.Context, th *look.Theme, header *fileHeader, file protocol.FileDiff, options Options) {
	size := gtx.Constraints.Max
	glyph := look.Icon.ChevronRight
	if v.unfolded(file.Path, options) {
		glyph = look.Icon.ExpandMore
	}
	look.PlaceIcon(gtx, glyph, image.Pt(gtx.Dp(8), (size.Y-gtx.Dp(16))/2), 16, th.R.Fg3)
	letter, ink := StatusMark(th, file.Status)
	look.MiddleLeft(gtx, gtx.Dp(32), size.Y, kit.RawText(th.Theme, th.T.Mono, ink, letter))

	right := size.X - gtx.Dp(8)
	counts := gtx
	counts.Constraints = layout.Constraints{Max: image.Pt(size.X/2, size.Y)}
	record := op.Record(gtx.Ops)
	countSize := look.DiffCount(th, file.Added, file.Removed, false)(counts)
	call := record.Stop()
	right -= countSize.Size.X
	offset := op.Offset(image.Pt(right, (size.Y-countSize.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	if options.FileActions && header.hover.Hovered() {
		right = v.drawFileActions(gtx, th, header, file, options, right-gtx.Dp(8))
	}

	path := file.Path
	if file.OldPath != "" && file.OldPath != file.Path {
		path = file.OldPath + " → " + file.Path
	}
	left := gtx.Dp(52)
	room := max(0, right-gtx.Dp(13)-left)
	character := max(1, gtx.Sp(th.T.Mono.Size)*3/5)
	label := gtx
	label.Constraints = layout.Constraints{Max: image.Pt(room, size.Y)}
	look.MiddleLeft(label, left, size.Y, kit.RawText(th.Theme, th.T.Mono, th.R.Fg, look.ElidePath(path, room/character)))
}

func (v *View) drawFileActions(
	gtx layout.Context,
	th *look.Theme,
	header *fileHeader,
	file protocol.FileDiff,
	options Options,
	right int,
) int {
	size := gtx.Constraints.Max
	box := gtx.Dp(28)
	type fileAction struct {
		control *look.IconAction
		icon    *widget.Icon
		tip     string
		kind    ActionKind
	}
	stage := fileAction{&header.stage, look.Icon.Add, i18n.T("Stage"), ActStage}
	if options.Staged {
		stage = fileAction{&header.unstage, look.Icon.Remove, i18n.T("Unstage"), ActUnstage}
	}
	actions := []fileAction{
		{&header.open, look.Icon.OpenInNew, i18n.T("Open in your editor"), ActOpen},
		stage,
		{&header.discard, look.Icon.Undo, i18n.T("Discard changes"), ActDiscard},
	}
	x := right - len(actions)*box
	for index, action := range actions {
		action.control.Icon, action.control.Description = action.icon, action.tip
		if action.control.Clicked(gtx) {
			v.pending = append(v.pending, Action{Kind: action.kind, Path: file.Path})
		}
		at := image.Pt(x+index*box, (size.Y-box)/2)
		look.Place(gtx, image.Rectangle{Min: at, Max: at.Add(image.Pt(box, box))}, func(gtx layout.Context) layout.Dimensions {
			return header.tips[index].Layout(gtx, th, options.Overlays, action.tip, func(gtx layout.Context) layout.Dimensions {
				return action.control.Layout(gtx, th, th.R.Fg2)
			})
		})
	}
	return x
}

func (v *View) layoutHunk(gtx layout.Context, th *look.Theme, hunk protocol.Hunk) layout.Dimensions {
	return layout.Inset{Top: 6, Bottom: 2, Left: textInset}.Layout(gtx, kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, hunk.Header))
}

func (v *View) layoutLine(
	gtx layout.Context,
	th *look.Theme,
	path string,
	item diffItem,
	line protocol.DiffLine,
	options Options,
) layout.Dimensions {
	key := lineKey{path: path, hunk: item.hunk, line: item.line}
	control := v.lines[key]
	if control == nil {
		control = new(lineControl)
		v.lines[key] = control
	}
	if control.gutter.Clicked(gtx) && options.Commentable {
		v.commentPath, v.commentLine, v.commentSide = path, line.New, "new"
		if line.New == 0 {
			v.commentLine, v.commentSide = line.Old, "old"
		}
		v.commentField.SetText("")
		v.commentFocus = true
	}
	width := gtx.Constraints.Max.X
	inset := gtx.Dp(textInset)
	record := op.Record(gtx.Ops)
	body := gtx
	body.Constraints = layout.Constraints{Max: image.Pt(max(0, width-inset-gtx.Dp(8)), gtx.Constraints.Max.Y)}
	ink := th.R.Fg2
	if line.Kind != protocol.LineContext {
		ink = th.R.Fg
	}
	offset := op.Offset(image.Pt(inset, gtx.Dp(2))).Push(gtx.Ops)
	content := kit.Label{Style: th.T.Mono, Color: ink}.Layout(body, th.Theme, strings.ReplaceAll(line.Text, "\t", "    "))
	offset.Pop()
	call := record.Stop()

	height := max(gtx.Sp(th.T.Mono.Line), content.Size.Y) + gtx.Dp(4)
	bounds := image.Rect(0, 0, width, height)
	sign, tint := lineMark(th, line.Kind)
	if tint.A > 0 {
		paint.FillShape(gtx.Ops, tint, clip.Rect(bounds).Op())
	}
	call.Add(gtx.Ops)
	v.drawGutter(gtx, th, control, line, sign, options.Commentable, height)
	dimensions := layout.Dimensions{Size: bounds.Max}
	if !v.commentFieldMatches(path, line) {
		return dimensions
	}
	fieldOffset := op.Offset(image.Pt(0, height)).Push(gtx.Ops)
	field := v.layoutCommentField(gtx, th, path)
	fieldOffset.Pop()
	dimensions.Size.Y += field.Size.Y
	return dimensions
}

func (v *View) drawGutter(
	gtx layout.Context,
	th *look.Theme,
	control *lineControl,
	line protocol.DiffLine,
	sign string,
	commentable bool,
	height int,
) {
	first := gtx.Sp(th.T.Mono.Line) + gtx.Dp(4)
	column := gtx.Dp(numberColumn)
	number := func(x int, value int) {
		if value == 0 {
			return
		}
		cell := gtx
		cell.Constraints = layout.Exact(image.Pt(column-gtx.Dp(6), first))
		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		layout.E.Layout(cell, func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: th.T.MonoSm, Color: th.R.Fg3, MaxLines: 1, Align: text.End}.Layout(gtx, th.Theme, strconv.Itoa(value))
		})
		offset.Pop()
	}
	number(0, line.Old)
	number(column, line.New)
	hovered := commentable && control.gutter.Hovered()
	ink := th.R.Fg3
	switch {
	case hovered:
		sign, ink = "+", th.R.GoldInk
	case line.Kind == protocol.LineAdd:
		ink = th.R.Jade
	case line.Kind == protocol.LineDel:
		ink = th.R.Jasper
	}
	signCell := gtx
	signCell.Constraints = layout.Exact(image.Pt(gtx.Dp(signColumn), first))
	offset := op.Offset(image.Pt(2*column, 0)).Push(gtx.Ops)
	layout.Center.Layout(signCell, kit.RawText(th.Theme, th.T.Mono, ink, sign))
	offset.Pop()
	if !commentable {
		return
	}
	look.Place(gtx, image.Rect(0, 0, gtx.Dp(textInset), height), func(gtx layout.Context) layout.Dimensions {
		return control.gutter.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})
}

func lineMark(th *look.Theme, kind string) (string, color.NRGBA) {
	switch kind {
	case protocol.LineAdd:
		return "+", palette.WithAlpha(th.R.Jade, .12)
	case protocol.LineDel:
		return "−", palette.WithAlpha(th.R.Jasper, .12)
	default:
		return "", color.NRGBA{}
	}
}

func (v *View) commentFieldMatches(path string, line protocol.DiffLine) bool {
	if v.commentPath != path || v.commentLine == 0 {
		return false
	}
	return v.commentSide == "new" && line.New == v.commentLine || v.commentSide == "old" && line.Old == v.commentLine
}

func (v *View) layoutCommentField(gtx layout.Context, th *look.Theme, path string) layout.Dimensions {
	v.commentField.Placeholder = i18n.T("Write a comment, then Enter")
	v.commentField.Editor.Submit = true
	for {
		event, ok := v.commentField.Editor.Update(gtx)
		if !ok {
			break
		}
		submit, ok := event.(widget.SubmitEvent)
		if !ok {
			continue
		}
		if comment := strings.TrimSpace(submit.Text); comment != "" {
			v.pending = append(v.pending, Action{Kind: ActComment, Path: path, Line: v.commentLine, Side: v.commentSide, Text: comment})
		}
		v.commentPath, v.commentLine, v.commentSide = "", 0, ""
		v.commentField.SetText("")
	}
	return layout.Inset{Top: 4, Right: 8, Bottom: 8, Left: textInset}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		dimensions := v.commentField.Layout(gtx, th.Theme)
		if v.commentFocus {
			v.commentField.Focus(gtx)
			v.commentFocus = false
		}
		return dimensions
	})
}

func (v *View) layoutComment(gtx layout.Context, th *look.Theme, comment protocol.ReviewComment) layout.Dimensions {
	control := v.comments[comment.ID]
	if control == nil {
		control = new(commentControl)
		v.comments[comment.ID] = control
	}
	target := Action{Path: comment.Path, Line: comment.Line, Side: comment.Side, ID: comment.ID}
	if control.resolve.Clicked(gtx) {
		target.Kind = ActResolve
		v.pending = append(v.pending, target)
	}
	if control.remove.Clicked(gtx) {
		target.Kind = ActDelete
		v.pending = append(v.pending, target)
	}
	control.resolve.Kind, control.resolve.Small, control.resolve.Label = kit.PlaqueTonal, true, i18n.T("Resolve")
	plane, ink := th.R.MineralField(palette.Attention), th.R.Fg
	if comment.Resolved {
		control.resolve.Label = i18n.T("Reopen")
		plane, ink = th.R.Stone2, th.R.Fg3
	}
	control.remove.Kind, control.remove.Small, control.remove.Label = kit.PlaqueQuiet, true, i18n.T("Delete")
	return layout.Inset{Top: 4, Right: 8, Bottom: 8, Left: textInset}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		record := op.Record(gtx.Ops)
		dimensions := layout.Inset{Top: 8, Right: 8, Bottom: 8, Left: 13}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, look.Paragraph(th, th.T.Body, ink, comment.Text)),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return control.resolve.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: 4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return control.remove.Layout(gtx, th.Theme) }),
			)
		})
		call := record.Stop()
		kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Small), plane)
		call.Add(gtx.Ops)
		return dimensions
	})
}

func (v *View) layoutMore(gtx layout.Context, th *look.Theme, path string, hidden int) layout.Dimensions {
	click := v.more[path]
	if click == nil {
		click = new(kit.Click)
		v.more[path] = click
	}
	if click.Clicked(gtx) {
		v.whole[path] = true
	}
	return layout.Inset{Top: 4, Bottom: 8, Left: textInset}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			ink := th.R.Fg3
			if click.Hovered() {
				ink = th.R.Fg
			}
			return kit.Text(th.Theme, th.T.Caption, ink, i18n.Tf("Show %d more lines", hidden))(gtx)
		})
	})
}

func StatusMark(th *look.Theme, status string) (string, color.NRGBA) {
	switch status {
	case protocol.ChangeAdded:
		return "A", th.R.Jade
	case protocol.ChangeDeleted:
		return "D", th.R.Jasper
	case protocol.ChangeRenamed:
		return "R", th.R.Lapis
	case protocol.ChangeCopied:
		return "C", th.R.Lapis
	case protocol.ChangeType:
		return "T", th.R.Fg2
	case protocol.ChangeConflict:
		return "!", th.R.Jasper
	case protocol.ChangeUntracked:
		return "?", th.R.Fg3
	default:
		return "M", th.R.Amber
	}
}
