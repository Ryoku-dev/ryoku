package settings

import (
	"path/filepath"
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// other programs keep dozens of files; the first few are shown and a link opens the rest
const foundRowsShown = 4

// model files other programs and the user's folders already hold; the daemon looks for them only when
// this list asks, and this list asks only while the manager is open
type localFoundPanel struct {
	asked, loading bool
	// the folders the list was read with, so another set of folders reads it again
	key     string
	models  []protocol.LocalFoundModel
	problem string
	rows    map[string]*localFoundRow
	again   kit.Plaque
	more    kit.Click
	showAll bool
	folders localFolders
}

type localFoundRow struct {
	use     kit.Plaque
	busy    bool
	problem string
}

// the folders the user added for Hyakki to look in, kept in the settings; the system chooser names one,
// and a path is typed only where no chooser answers
type localFolders struct {
	add     kit.Plaque
	remove  map[string]*kit.IconPlaque
	typed   bool
	focus   bool
	path    kit.TextField
	save    kit.Plaque
	busy    bool
	problem string
}

func (f *localFoundPanel) load(e *env.Env) {
	key := strings.Join(e.Snapshot().Settings.Providers.Models.Folders, "\x00")
	if key != f.key {
		f.key, f.asked, f.loading = key, false, false
	}
	if f.asked || f.loading {
		return
	}
	f.asked, f.loading = true, true
	e.LocalFound(func(models []protocol.LocalFoundModel, err error) {
		if key != f.key {
			return
		}
		f.loading = false
		if err != nil {
			f.problem = err.Error()
			return
		}
		f.models, f.problem = models, ""
	})
}

func foundName(found protocol.LocalFoundModel) string {
	if found.Name != "" {
		return found.Name
	}
	return filepath.Base(found.Path)
}

// the program that keeps a found file; one from a folder of the user's is told by the folder it lies in
func foundSource(found protocol.LocalFoundModel) string {
	switch found.Source {
	case protocol.LocalFoundOllama:
		return "Ollama"
	case protocol.LocalFoundLMStudio:
		return "LM Studio"
	case protocol.LocalFoundHuggingFace:
		return i18n.T("Hugging Face cache")
	case protocol.LocalFoundLlamaCpp:
		return i18n.T("llama.cpp cache")
	}
	return env.LocalPlace(filepath.Dir(found.Path))
}

// Ollama · 4.70 GB · Q4_K_M · up to 131K context
func foundDetail(found protocol.LocalFoundModel) string {
	words := []string{foundSource(found), env.LocalSize(found.Size)}
	if found.Quant != "" {
		words = append(words, found.Quant)
	}
	if found.Context > 0 {
		words = append(words, i18n.Tf("up to %s context", env.ShortContext(found.Context)))
	}
	return strings.Join(words, " · ")
}

// the glance a file gets before it is downloaded, for one that is already here: how it fits this computer,
// then what stops Hyakki using it
func foundChips(found protocol.LocalFoundModel) []verdictChip {
	chips := fileChips(protocol.LocalFile{Tools: found.Tools, Context: found.Context, Plan: found.Plan}, false)
	if found.Vision {
		chips = append(chips, verdictChip{i18n.T("Pictures"), palette.Idle})
	}
	return chips
}

// a found file already runs as a model: the scan names it, and one taken since the scan is known by its path
func foundUsed(found protocol.LocalFoundModel, models []protocol.LocalModel) bool {
	return slices.ContainsFunc(models, func(model protocol.LocalModel) bool {
		return model.Path != "" && model.Path == found.Path || found.Model != "" && model.ID == found.Model
	})
}

// the list with one more folder, or why it stays as it is
func foldersWith(folders []string, typed string) ([]string, string) {
	folder, full := env.LocalSource(typed)
	if !full {
		return nil, i18n.T("Enter the full path of a folder.")
	}
	folder = filepath.Clean(folder)
	if slices.Contains(folders, folder) {
		return nil, i18n.T("Hyakki already looks in that folder.")
	}
	return append(slices.Clone(folders), folder), ""
}

// never nil: a merge patch drops a null, and an empty list is what clears the setting
func foldersWithout(folders []string, folder string) []string {
	kept := make([]string, 0, len(folders))
	for _, listed := range folders {
		if listed != folder {
			kept = append(kept, listed)
		}
	}
	return kept
}

func (f *localFoundPanel) layout(gtx layout.Context, e *env.Env, snapshot state.Snapshot, panel *localPanel) layout.Dimensions {
	th := e.Theme
	if f.rows == nil {
		f.rows = make(map[string]*localFoundRow)
	}
	f.again.Kind, f.again.Small, f.again.Label, f.again.Busy = kit.PlaqueQuiet, true, i18n.T("Look again"), f.loading
	if f.again.Clicked(gtx) && !f.loading {
		f.asked = false
		f.load(e)
	}
	if f.more.Clicked(gtx) {
		f.showAll = true
	}
	children := []layout.FlexChild{
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2,
			i18n.T("Model files other programs already keep here. Hyakki runs one from where it is and never moves, changes or deletes it."))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
	}
	switch {
	case len(f.models) > 0:
		shown := len(f.models)
		if !f.showAll {
			shown = min(shown, foundRowsShown)
		}
		for _, found := range f.models[:shown] {
			row := f.rows[found.Path]
			if row == nil {
				row = &localFoundRow{}
				f.rows[found.Path] = row
			}
			used := foundUsed(found, snapshot.Local.Models)
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.layout(gtx, e, found, used, panel)
			}), layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
		}
		if shown < len(f.models) {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Link(gtx, th, &f.more, i18n.Tf("Show all %d models", len(f.models)))
			}), layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
		}
	case f.loading:
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Working{Active: true}.Layout(gtx, th.Theme)
		}), layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
	case f.problem == "":
		children = append(children,
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg, i18n.T("No model files were found. Add the folder where you keep yours."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
	}
	if f.problem != "" {
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, f.problem)), layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return f.folders.layout(gtx, e, snapshot.Settings.Providers.Models.Folders, func(gtx layout.Context) layout.Dimensions {
			return f.again.Layout(gtx, th.Theme)
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (row *localFoundRow) layout(gtx layout.Context, e *env.Env, found protocol.LocalFoundModel, used bool, panel *localPanel) layout.Dimensions {
	th := e.Theme
	fits := found.Plan.Verdict != protocol.LocalVerdictTooBig
	row.use.Kind, row.use.Small, row.use.Label, row.use.Busy = kit.PlaqueTonal, true, i18n.T("Use it"), row.busy
	if row.use.Clicked(gtx) && !row.busy && !used && fits {
		row.busy, row.problem = true, ""
		e.LocalUseFound(found.Path, func(_ protocol.LocalModel, err error) {
			row.busy = false
			if err != nil {
				row.problem = err.Error()
				return
			}
			// the models above are read again, ahead of the local.changed that says the same
			panel.asked = false
		})
	}
	return kit.OutlinedSlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, kit.Text(th.Theme, th.T.Row, th.R.Fg, foundName(found))),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if used {
							return kit.Claim(th.Theme, palette.Alive, i18n.T("In use"))(gtx)
						}
						if !fits {
							gtx = gtx.Disabled()
						}
						return row.use.Layout(gtx, th.Theme)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, foundDetail(found))),
		}
		if chips := foundChips(found); len(chips) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return (look.Flow{Gap: th.Sp.S1, LineGap: th.Sp.S1}).Layout(gtx, len(chips), func(gtx layout.Context, at int) layout.Dimensions {
					return look.Chip(th, nil, chips[at].Word, chips[at].Tone)(gtx)
				})
			}))
		}
		if found.Warning != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, found.Warning)))
		}
		if row.problem != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, row.problem)))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (f *localFolders) store(e *env.Env, folders []string, done func()) {
	f.busy, f.problem = true, ""
	setLocalOption(e, "folders", folders, func(err error) {
		f.busy = false
		if err != nil {
			f.problem = err.Error()
			return
		}
		if done != nil {
			done()
		}
	})
}

func (f *localFolders) take(e *env.Env, folders []string, typed string) {
	next, problem := foldersWith(folders, typed)
	if problem != "" {
		f.problem = problem
		return
	}
	// the field stays where no chooser answers, ready for the next folder
	f.store(e, next, func() { f.path.SetText("") })
}

// beside is the act that shares the row of Add a folder
func (f *localFolders) layout(gtx layout.Context, e *env.Env, folders []string, beside layout.Widget) layout.Dimensions {
	th := e.Theme
	if f.remove == nil {
		f.remove = make(map[string]*kit.IconPlaque)
	}
	f.add.Kind, f.add.Small, f.add.Label, f.add.Icon, f.add.Busy = kit.PlaqueTonal, true, i18n.T("Add a folder"), look.Icon.FolderOpen, f.busy && !f.typed
	if f.add.Clicked(gtx) && !f.busy {
		// the answer comes frames later, so the list it adds to is read then
		e.ChooseFolder(i18n.T("Choose a folder with model files"), "", func(path string) {
			f.take(e, e.Snapshot().Settings.Providers.Models.Folders, path)
		}, func() {
			f.typed, f.focus = true, true
			e.Notify(i18n.T("No file chooser answered. Type the folder instead."))
		})
	}
	children := make([]layout.FlexChild, 0, 2*len(folders)+6)
	if len(folders) > 0 {
		children = append(children,
			layout.Rigid(kit.RawText(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Hyakki also looks in"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		)
	}
	for _, folder := range folders {
		remove := f.remove[folder]
		if remove == nil {
			remove = &kit.IconPlaque{Icon: look.Icon.Remove, Kind: kit.PlaqueQuiet, Small: true}
			f.remove[folder] = remove
		}
		place := env.LocalPlace(folder)
		remove.Description = i18n.Tf("Stop looking in %s", place)
		if remove.Clicked(gtx) && !f.busy {
			f.store(e, foldersWithout(folders, folder), nil)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, look.Paragraph(th, th.T.MonoSm, th.R.Fg, place)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return remove.Layout(gtx, th.Theme) }),
			)
		}))
	}
	if len(folders) > 0 {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.add.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(beside),
		)
	}))
	if f.typed {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.field(gtx, e, folders)
		}))
	}
	if f.problem != "" {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, f.problem)))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (f *localFolders) field(gtx layout.Context, e *env.Env, folders []string) layout.Dimensions {
	th := e.Theme
	if f.focus {
		f.focus = false
		f.path.Focus(gtx)
	}
	f.path.Label, f.path.Placeholder, f.path.Mono = i18n.T("Folder to look in"), "~/Models", true
	f.path.Editor.SingleLine, f.path.Editor.Submit = true, true
	f.save.Kind, f.save.Small, f.save.Label, f.save.Busy = kit.PlaqueTonal, true, i18n.T("Add this folder"), f.busy
	submitted := false
	for {
		event, ok := f.path.Editor.Update(gtx)
		if !ok {
			break
		}
		switch event.(type) {
		case widget.ChangeEvent:
			// a refusal is about the text it was given
			f.problem = ""
		case widget.SubmitEvent:
			submitted = true
		}
	}
	filled := strings.TrimSpace(f.path.Text()) != ""
	if (f.save.Clicked(gtx) || submitted) && filled && !f.busy {
		f.take(e, folders, f.path.Text())
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return f.path.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !filled {
				gtx = gtx.Disabled()
			}
			return f.save.Layout(gtx, th.Theme)
		}),
	)
}
