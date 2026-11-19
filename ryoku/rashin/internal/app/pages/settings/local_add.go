package settings

import (
	"image"
	"regexp"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// a repository can hold twenty quantisations; the ranked top is shown and a link opens the rest
const fileRowsShown = 4

// model-00002-of-00003.gguf is the second of three parts of one model
var splitPart = regexp.MustCompile(`(?i)-(\d{5})-of-(\d{5})\.gguf$`)

type localAddPanel struct {
	open            bool
	show, find, add kit.Plaque
	browse          kit.IconPlaque
	source          kit.TextField
	picture         look.Select
	more            kit.Click
	showAll         bool
	rows            []*localFileRow

	// what the result on screen was asked for, so an answer to an older question is dropped
	asked string
	// the source is a file on this computer: no licence to read and nothing to download
	file                     bool
	result                   protocol.LocalSearchResult
	modelFiles, pictureFiles []protocol.LocalFile
	// -1 while no file is picked
	modelIndex, pictureIndex int
	searching, adding        bool
	problem                  string
}

// one file of the ranked list; the whole row takes the press
type localFileRow struct {
	click kit.Click
	hover look.Hover
}

func (a *localAddPanel) clear() {
	a.result, a.modelFiles, a.pictureFiles, a.rows = protocol.LocalSearchResult{}, nil, nil, nil
	a.modelIndex, a.pictureIndex, a.showAll, a.file = -1, 0, false, false
}

// the list is drawn again from nothing after a model went in, ready for the next one
func (a *localAddPanel) added() {
	a.clear()
	a.asked, a.problem, a.open, a.adding = "", "", false, false
	a.source.SetText("")
}

func (a *localAddPanel) search(e *env.Env) {
	source, file := env.LocalSource(a.source.Text())
	switch {
	case source == "":
		a.problem = i18n.T("Enter a repository such as owner/model-GGUF, a Hugging Face link or the path of a .gguf file.")
		return
	case file && !strings.HasSuffix(strings.ToLower(source), ".gguf"):
		a.problem = i18n.T("Choose a model file that ends in .gguf.")
		return
	}
	a.clear()
	a.asked, a.searching, a.problem = source, true, ""
	e.LocalSearch(source, func(result protocol.LocalSearchResult, err error) {
		if a.asked != source {
			return
		}
		a.searching = false
		if err != nil {
			a.problem = err.Error()
			return
		}
		a.take(result, file)
	})
}

// take keeps the model files in the daemon's ranking, shows a split set as the one row of its first part
// with the size of all its parts, and picks what the daemon recommends
func (a *localAddPanel) take(result protocol.LocalSearchResult, file bool) {
	a.clear()
	a.result, a.file = result, file
	// what each set's later parts weigh: every part comes with its own size
	rest := make(map[string]int64)
	for _, listed := range result.Files {
		model := listed.Role == protocol.LocalFileModel && strings.HasSuffix(strings.ToLower(listed.Name), ".gguf")
		set, part := splitSet(listed.Name)
		switch {
		case listed.Role == protocol.LocalFileMmproj:
			a.pictureFiles = append(a.pictureFiles, listed)
		case model && part > 1:
			rest[set] += listed.Size
		case model:
			a.modelFiles = append(a.modelFiles, listed)
		}
	}
	for at := range a.modelFiles {
		if set, part := splitSet(a.modelFiles[at].Name); part == 1 {
			a.modelFiles[at].Size += rest[set]
		}
	}
	a.rows = make([]*localFileRow, len(a.modelFiles))
	for at := range a.rows {
		a.rows[at] = &localFileRow{}
	}
	a.modelIndex = recommendedFile(a.modelFiles, result)
	for at, listed := range a.pictureFiles {
		if result.Mmproj != "" && listed.Name == result.Mmproj {
			a.pictureIndex = at + 1
		}
	}
	// a pick far down the ranking must not hide behind the link
	a.showAll = a.modelIndex >= fileRowsShown
}

// what the parts of one split set share, and which part this is; a whole file is part 0 of itself
func splitSet(name string) (set string, part int) {
	match := splitPart.FindStringSubmatchIndex(name)
	if match == nil {
		return name, 0
	}
	part, _ = strconv.Atoi(name[match[2]:match[3]])
	return name[:match[0]] + " of " + name[match[4]:match[5]], part
}

// the file to preselect: the one the daemon names, else the one it flags, else the one a link pointed at,
// else the only one there is; never one that cannot run here, and -1 leaves the choice open
func recommendedFile(files []protocol.LocalFile, result protocol.LocalSearchResult) int {
	pick := -1
	for at, file := range files {
		switch {
		case result.Recommended != "" && file.Name == result.Recommended:
			pick = at
		case pick < 0 && file.Recommended:
			pick = at
		}
	}
	if pick < 0 {
		for at, file := range files {
			if result.Path != "" && (file.Name == result.Path || strings.HasSuffix(result.Path, "/"+file.Name)) {
				pick = at
			}
		}
	}
	if pick < 0 && len(files) == 1 {
		pick = 0
	}
	if pick >= 0 && fileVerdict(files[pick]) == protocol.LocalVerdictTooBig {
		return -1
	}
	return pick
}

// Q4_K_M reads better than a file name twenty quantisations share; a file with no known quantisation
// keeps its name, without the part counter of a split set
func fileTitle(file protocol.LocalFile) string {
	if file.Quant != "" {
		return file.Quant
	}
	return fileName(file)
}

func fileName(file protocol.LocalFile) string {
	if at := splitPart.FindStringIndex(file.Name); at != nil {
		return file.Name[:at[0]] + ".gguf"
	}
	return file.Name
}

// the quiet line under a quantisation: which file it is and how many parts it comes in
func fileNote(file protocol.LocalFile) string {
	name := ""
	if file.Quant != "" {
		name = fileName(file)
	}
	if file.Parts > 1 {
		return look.Clauses(name, i18n.Tf("%d parts", file.Parts))
	}
	return name
}

// a commit hash is cut to what tells two revisions apart; a branch or a tag keeps its name
func shortRevision(revision string) string {
	if len(revision) != 40 {
		return revision
	}
	for _, r := range revision {
		if !('0' <= r && r <= '9' || 'a' <= r && r <= 'f') {
			return revision
		}
	}
	return revision[:12]
}

func (a *localAddPanel) selected() (protocol.LocalFile, bool) {
	if a.modelIndex < 0 || a.modelIndex >= len(a.modelFiles) {
		return protocol.LocalFile{}, false
	}
	return a.modelFiles[a.modelIndex], true
}

func (a *localAddPanel) update(gtx layout.Context, e *env.Env, panel *localPanel) {
	a.source.Label = i18n.T("Model to add")
	a.source.Placeholder = i18n.T("owner/model-GGUF, a Hugging Face link or a .gguf file")
	a.source.Editor.SingleLine, a.source.Editor.Submit = true, true
	a.browse.Icon, a.browse.Kind, a.browse.Description = look.Icon.FolderOpen, kit.PlaqueTonal, i18n.T("Choose a model file")
	a.find.Kind, a.find.Small, a.find.Label, a.find.Busy = kit.PlaqueTonal, true, i18n.T("Find model files"), a.searching
	a.add.Kind, a.add.Small, a.add.Busy = kit.PlaqueTonal, true, a.adding
	a.add.Label = i18n.T("Add and download")
	if a.file {
		a.add.Label = i18n.T("Add this file")
	}
	submit := false
	for {
		event, ok := a.source.Editor.Update(gtx)
		if !ok {
			break
		}
		switch event.(type) {
		case widget.ChangeEvent:
			// the list was about the text it was asked for, not what is typed since; a path the chooser
			// just put in the field is the text that was asked for
			if source, _ := env.LocalSource(a.source.Text()); source != a.asked {
				a.clear()
				a.asked, a.searching, a.problem = "", false, ""
			}
		case widget.SubmitEvent:
			submit = true
		}
	}
	if a.browse.Clicked(gtx) {
		e.ChooseModelFile(func(path string) {
			a.source.SetText(path)
			a.search(e)
		}, func() {
			e.Notify(i18n.T("No file chooser answered. Type the path of the .gguf file instead."))
		})
	}
	if (a.find.Clicked(gtx) || submit) && !a.searching {
		a.search(e)
	}
	if a.more.Clicked(gtx) {
		a.showAll = true
	}
	for at, row := range a.rows {
		if row.click.Clicked(gtx) && at < len(a.modelFiles) && fileVerdict(a.modelFiles[at]) != protocol.LocalVerdictTooBig {
			a.modelIndex = at
		}
	}
	chosen, picked := a.selected()
	if !a.add.Clicked(gtx) || !picked || a.result.Gated || a.adding {
		return
	}
	if a.file {
		// the file is already the user's, so there is no licence to agree to; the daemon answers once the
		// whole file is copied and checked, with no count of bytes on the way
		source := a.asked
		a.adding, a.problem = true, ""
		e.LocalAdd(protocol.LocalAddParams{Repo: source}, func(_ protocol.LocalModel, err error) {
			a.adding = false
			panel.asked = false
			switch {
			case a.asked != source:
				// the field moved on to something else while the file went in
			case err != nil:
				a.problem = err.Error()
			default:
				a.added()
			}
		})
		return
	}
	params := protocol.LocalAddParams{Repo: a.result.Repo, Revision: a.result.Revision, File: chosen.Name}
	if a.pictureIndex > 0 && a.pictureIndex <= len(a.pictureFiles) {
		params.Mmproj = a.pictureFiles[a.pictureIndex-1].Name
	}
	panel.requestAdd(params, protocol.LocalLicense{
		Name: a.result.License, URL: "https://huggingface.co/" + a.result.Repo,
		Note: i18n.T("Check the model card for its full terms and usage limits."),
	}, chosen.Warning)
}

func (a *localAddPanel) layout(gtx layout.Context, e *env.Env, panel *localPanel) layout.Dimensions {
	th := e.Theme
	a.show.Kind, a.show.Small, a.show.Label, a.show.Icon = kit.PlaqueTonal, true, i18n.T("Add a model"), look.Icon.Add
	if a.show.Clicked(gtx) {
		a.open = !a.open
		if a.open {
			a.source.Focus(gtx)
		}
	}
	if !a.open {
		return a.show.Layout(gtx, th.Theme)
	}
	a.update(gtx, e, panel)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.show.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.source.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.browse.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.find.Layout(gtx, th.Theme) }),
			)
		}),
	}
	gap := layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout)
	switch {
	case a.result.Gated:
		children = append(children, gap, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, i18n.T("This repository is gated. Accept its terms on Hugging Face and use an accessible model repository."))))
	case a.asked != "" && !a.searching && a.problem == "" && len(a.modelFiles) == 0:
		children = append(children, gap, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("No GGUF model file was found there."))))
	case len(a.modelFiles) > 0:
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
		children = append(children, a.results(e)...)
	}
	if a.problem != "" {
		children = append(children, gap, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, a.problem)))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// the ranked files, the picture reader, what the picked file is, and the one act
func (a *localAddPanel) results(e *env.Env) []layout.FlexChild {
	th := e.Theme
	gap := layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout)
	shown := len(a.modelFiles)
	if !a.showAll {
		shown = min(shown, fileRowsShown)
	}
	children := make([]layout.FlexChild, 0, shown+12)
	if len(a.modelFiles) > 1 {
		children = append(children,
			layout.Rigid(kit.RawText(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Files, best for this computer first"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		)
	}
	for at := range shown {
		file, row := a.modelFiles[at], a.rows[at]
		recommended := file.Recommended || a.result.Recommended != "" && file.Name == a.result.Recommended
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return row.layout(gtx, th, file, fileChips(file, recommended), at == a.modelIndex)
		}))
	}
	if shown < len(a.modelFiles) {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Link(gtx, th, &a.more, i18n.Tf("Show all %d files", len(a.modelFiles)))
		}))
	}
	chosen, picked := a.selected()
	if !picked {
		return append(children, gap, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("None of these files can run on this computer. Look for a smaller model or a smaller quantisation."))))
	}
	if len(a.pictureFiles) > 0 {
		pictures := []look.SelectOption{{Label: i18n.T("No picture reader")}}
		for _, file := range a.pictureFiles {
			pictures = append(pictures, look.SelectOption{Label: file.Name, Detail: env.LocalSize(file.Size)})
		}
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			a.picture.Label = i18n.T("Picture reader (optional)")
			gtx.Constraints.Min.X = min(gtx.Dp(selectWidth), gtx.Constraints.Max.X)
			gtx.Constraints.Max.X = gtx.Constraints.Min.X
			dims := a.picture.LayoutOptions(gtx, th, e.Overlays, pictures, a.pictureIndex)
			if at, changed := a.picture.Changed(); changed && at >= 0 && at < len(pictures) {
				a.pictureIndex = at
			}
			return dims
		}))
	}
	facts := make([]string, 0, 3)
	if chosen.Context > 0 {
		facts = append(facts, i18n.Tf("%d context tokens", chosen.Context))
	}
	if chosen.Params > 0 {
		facts = append(facts, i18n.Tf("%.2fB parameters", float64(chosen.Params)/1e9))
	}
	if chosen.Tools {
		facts = append(facts, i18n.T("Tool calling"))
	}
	if len(facts) > 0 {
		children = append(children, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return (look.Flow{Gap: th.Sp.S1, LineGap: th.Sp.S1}).Layout(gtx, len(facts), func(gtx layout.Context, at int) layout.Dimensions {
				return look.TokenChip(th, facts[at])(gtx)
			})
		}))
	}
	if chosen.Warning != "" {
		children = append(children, gap, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, chosen.Warning)))
	}
	if a.file {
		children = append(children, gap, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Rashin adds the file to its models and checks it. Your file stays where it is."))))
		if a.adding {
			children = append(children, gap,
				layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, i18n.T("Copying the file and checking it. A large model takes a few minutes."))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.BladeProgress{Fraction: -1, Active: true}.Layout(gtx, th.Theme)
				}),
			)
		}
	} else {
		source := i18n.Tf("Revision %s", shortRevision(a.result.Revision))
		if a.result.License != "" {
			source = i18n.Tf("Revision %s · Licence %s", shortRevision(a.result.Revision), a.result.License)
		}
		children = append(children, gap, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, source)))
	}
	return append(children, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.add.Layout(gtx, th.Theme) }))
}

func (row *localFileRow) layout(gtx layout.Context, th *look.Theme, file protocol.LocalFile, chips []verdictChip, selected bool) layout.Dimensions {
	enabled := fileVerdict(file) != protocol.LocalVerdictTooBig
	title, quiet := look.RowInk(th, selected)
	if !enabled {
		title, quiet = th.R.Fg3, th.R.Fg3
	}
	width := gtx.Constraints.Max.X
	record := op.Record(gtx.Ops)
	inner := gtx
	inner.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
	content := layout.Inset{Top: th.Sp.S2, Right: th.Sp.S3, Bottom: th.Sp.S2, Left: th.Sp.S3}.Layout(inner, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, kit.RawText(th.Theme, th.T.Row, title, fileTitle(file))),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(kit.RawText(th.Theme, th.T.Detail, quiet, env.LocalSize(file.Size))),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// the mark's place is kept, so picking a row moves nothing in it
						if !selected {
							return layout.Spacer{Width: 18, Height: 18}.Layout(gtx)
						}
						return look.IconWidget(look.Icon.Check, 18, title)(gtx)
					}),
				)
			}),
		}
		if note := fileNote(file); note != "" {
			children = append(children, layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, quiet, note)))
		}
		if len(chips) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return (look.Flow{Gap: th.Sp.S1, LineGap: th.Sp.S1}).Layout(gtx, len(chips), func(gtx layout.Context, at int) layout.Dimensions {
					return look.Chip(th, nil, chips[at].Word, chips[at].Tone)(gtx)
				})
			}))
		}
		if file.Plan.Context > 0 && enabled {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, quiet, i18n.Tf("Would run with: %s", planWords(file.Plan)))))
		}
		if file.Plan.Reason != "" && (fileVerdict(file) == protocol.LocalVerdictTooBig || fileVerdict(file) == protocol.LocalVerdictUnsuitable || fileVerdict(file) == protocol.LocalVerdictSplit) {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, file.Plan.Reason)))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := record.Stop()
	size := image.Pt(width, content.Size.Y)
	press := gtx
	press.Constraints = layout.Exact(size)
	if !enabled {
		press = press.Disabled()
	}
	_, hover := row.hover.Layout(press, th, func(gtx layout.Context) layout.Dimensions {
		return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: size} })
	})
	look.RowWash(gtx, th, image.Rectangle{Max: size}, hover, 0, selected)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}
