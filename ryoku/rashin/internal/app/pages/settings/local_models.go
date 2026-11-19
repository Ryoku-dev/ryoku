package settings

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

type localModelRow struct {
	act, remove, load, licence kit.Plaque
	tuning                     localTuning
	busy                       bool
	problem                    string
}

func (r *localModelRow) layout(gtx layout.Context, e *env.Env, model protocol.LocalModel, progress protocol.LocalProgress, snapshot state.Snapshot, panel *localPanel) layout.Dimensions {
	th := e.Theme
	return kit.OutlinedSlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		done, total := env.LocalBytes(model)
		if progress.Total > 0 {
			done, total = progress.Done, progress.Total
		}
		r.act.Kind, r.act.Small, r.act.Label = kit.PlaqueTonal, true, modelAction(model)
		r.act.Busy = r.busy
		r.remove.Kind, r.remove.Small, r.remove.Label = kit.PlaqueDangerLine, true, i18n.T("Remove")
		r.load.Kind, r.load.Small, r.load.Label = kit.PlaqueQuiet, true, i18n.T("Load now")
		r.licence.Kind, r.licence.Small, r.licence.Label = kit.PlaqueQuiet, true, i18n.T("Read licence")
		if r.act.Clicked(gtx) && !r.busy {
			switch model.State {
			case protocol.LocalDownloading:
				r.busy = true
				e.LocalPause(model.ID, func(err error) { r.finish(err) })
			case protocol.LocalAvailable, protocol.LocalPaused, protocol.LocalBroken:
				switch {
				case env.LocalFromFile(model):
					// nothing can download the user's own file again: the same file, added once more, repairs it
					e.ChooseModelFile(func(path string) {
						r.busy = true
						e.LocalAdd(protocol.LocalAddParams{Repo: path}, func(_ protocol.LocalModel, err error) {
							r.finish(err)
							panel.asked = false
						})
					}, func() {
						e.Notify(i18n.T("No file chooser answered. Use Add a model and type the path of the .gguf file."))
					})
				case model.State == protocol.LocalAvailable && done == 0:
					panel.requestDownload(model)
				default:
					r.busy = true
					e.LocalDownload(model.ID, func(err error) { r.finish(err) })
				}
			}
		}
		if r.remove.Clicked(gtx) {
			panel.removing = model.ID
			panel.removeConfirm.Dialog.Open = true
		}
		if r.load.Clicked(gtx) {
			r.busy = true
			e.LocalStart(model.ID, func(err error) { r.finish(err) })
		}
		if r.licence.Clicked(gtx) && model.License.URL != "" {
			e.Open(model.License.URL)
		}
		chips := modelChips(model)
		plan, planned := env.LocalPlanFor(snapshot.Local, model)
		planned = planned && model.State == protocol.LocalInstalled
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, kit.Text(th.Theme, th.T.Row, th.R.Fg, model.Name)),
					layout.Rigid(kit.Claim(th.Theme, modelStateTone(model), modelStateWord(model))),
				)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, modelDetail(model))),
		}
		if len(chips) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return (look.Flow{Gap: th.Sp.S1, LineGap: th.Sp.S1}).Layout(gtx, len(chips), func(gtx layout.Context, index int) layout.Dimensions {
					return look.TokenChip(th, chips[index])(gtx)
				})
			}))
		}
		if model.License.Name != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, i18n.Tf("Licence: %s", model.License.Name))))
		}
		if model.Error != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, model.Error)))
		}
		if model.Warning != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, model.Warning)))
		}
		if progress.Phase == protocol.LocalPhaseError || r.problem != "" {
			message := r.problem
			if message == "" {
				message = progress.Error
			}
			if !env.LocalFromFile(model) {
				message = i18n.Tf("Download failed: %s", message)
			}
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, message)))
		}
		if r.busy && env.LocalFromFile(model) {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, i18n.T("Copying the file and checking it"))))
		}
		if model.State == protocol.LocalDownloading || model.State == protocol.LocalPaused {
			caption := i18n.Tf("%s of %s downloaded", env.LocalSize(done), env.LocalSize(total))
			if progress.Phase == protocol.LocalPhaseVerify {
				caption = i18n.T("Checking the downloaded files")
			}
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, caption)), layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				fraction := -1.0
				if total > 0 {
					fraction = float64(done) / float64(total)
				}
				return kit.BladeProgress{Fraction: fraction, Active: model.State == protocol.LocalDownloading}.Layout(gtx, th.Theme)
			}))
		}
		if planned {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(planBlock(th, plan)))
		}
		buttons := []layout.FlexChild{}
		if model.State != protocol.LocalInstalled {
			buttons = append(buttons, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.act.Layout(gtx, th.Theme) }))
		} else if !slicesContains(snapshot.Local.Runtime.Loaded, model.ID) {
			buttons = append(buttons, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.load.Layout(gtx, th.Theme) }))
		}
		if model.License.URL != "" {
			buttons = append(buttons, layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.licence.Layout(gtx, th.Theme) }))
		}
		if model.State != protocol.LocalAvailable {
			buttons = append(buttons, layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.remove.Layout(gtx, th.Theme) }))
		}
		if len(buttons) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, buttons...)
			}))
		}
		if _, set := snapshot.Settings.Providers.Models.Overrides[model.ID]; model.State == protocol.LocalInstalled && (planned || set) {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return r.tuning.layout(gtx, e, model, plan, snapshot)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (r *localModelRow) finish(err error) {
	r.busy = false
	if err != nil {
		r.problem = err.Error()
	} else {
		r.problem = ""
	}
}

func modelAction(model protocol.LocalModel) string {
	switch model.State {
	case protocol.LocalDownloading:
		return i18n.T("Pause")
	case protocol.LocalPaused:
		return i18n.T("Resume")
	case protocol.LocalBroken:
		if env.LocalFromFile(model) {
			return i18n.T("Add the file again")
		}
		return i18n.T("Download again")
	default:
		return i18n.T("Download")
	}
}

// the size on disk, the most the model can read, and where it came from when that was a file; the state
// word beside the name and the Runs with line say the rest
func modelDetail(model protocol.LocalModel) string {
	_, total := env.LocalBytes(model)
	words := []string{env.LocalSize(total)}
	// a model split over several parts is still one model beside its picture reader
	var weights, reader int64
	for _, file := range model.Files {
		if file.Role == protocol.LocalFileMmproj {
			reader += file.Size
		} else {
			weights += file.Size
		}
	}
	if weights > 0 && reader > 0 {
		words[0] += " (" + i18n.Tf("%s %s", i18n.T("model"), env.LocalSize(weights)) + " + " + i18n.Tf("%s %s", i18n.T("picture reader"), env.LocalSize(reader)) + ")"
	}
	if model.Context > 0 {
		words = append(words, i18n.Tf("up to %s context", env.ShortContext(model.Context)))
	}
	if env.LocalFromFile(model) {
		words = append(words, i18n.T("Added from a file"))
	}
	return strings.Join(words, " · ")
}

func modelChips(model protocol.LocalModel) []string {
	chips := []string{}
	if model.Class == protocol.LocalClassLight {
		chips = append(chips, i18n.T("Light"))
	}
	if model.Class == protocol.LocalClassMain {
		chips = append(chips, i18n.T("Main"))
	}
	if model.Params > 0 {
		chips = append(chips, i18n.Tf("%.2fB parameters", float64(model.Params)/1e9))
	}
	if model.Tools {
		chips = append(chips, i18n.T("Tools"))
	}
	if !model.Tools && !model.Catalogue {
		chips = append(chips, i18n.T("No tool calling"))
	}
	if model.Vision {
		chips = append(chips, i18n.T("Pictures"))
	}
	if model.Audio {
		chips = append(chips, i18n.T("Sound"))
	}
	if model.Video {
		chips = append(chips, i18n.T("Video"))
	}
	return chips
}

func slicesContains(ids []string, id string) bool {
	for _, listed := range ids {
		if listed == id {
			return true
		}
	}
	return false
}

func modelStateTone(model protocol.LocalModel) palette.Tone {
	switch model.State {
	case protocol.LocalInstalled:
		return palette.Alive
	case protocol.LocalDownloading:
		return palette.Becoming
	case protocol.LocalBroken:
		return palette.Failed
	case protocol.LocalPaused:
		return palette.Paused
	default:
		return palette.Idle
	}
}

func modelStateWord(model protocol.LocalModel) string {
	switch model.State {
	case protocol.LocalInstalled:
		return i18n.T("On this computer")
	case protocol.LocalDownloading:
		return i18n.T("Downloading")
	case protocol.LocalPaused:
		return i18n.T("Paused")
	case protocol.LocalBroken:
		return i18n.T("Needs repair")
	default:
		return i18n.T("Available")
	}
}
