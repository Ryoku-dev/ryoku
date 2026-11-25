package settings

import (
	"path/filepath"
	"slices"
	"strconv"
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

// the llama.cpp builds on this computer and the ways to name another: one that was found is used with a
// press, one found nowhere is picked in the system chooser, and a path is typed only where no chooser answers
type localBuilds struct {
	use               map[string]*kit.Plaque
	choose, automatic kit.Plaque
	// the build being switched to, "" for automatic; named says the user gave its path, not a row of the list
	switching bool
	target    string
	named     bool
	typed     bool
	path      kit.TextField
	pathFrom  string
	save      kit.Plaque
	problem   string
}

// whose llama.cpp a build is. One found outside the usual places carries the folder it lies in as its
// source; the path shown with every build says that in full, so the word only says it was found
func engineKind(source string) string {
	switch source {
	case "", protocol.RuntimeConfigured:
		return i18n.T("Your llama.cpp")
	case protocol.RuntimeManaged:
		return i18n.T("Hyakki's llama.cpp")
	case protocol.RuntimeSystem:
		return i18n.T("System llama.cpp")
	case protocol.RuntimeLogin:
		return i18n.T("llama.cpp from your shell")
	}
	return i18n.T("Found llama.cpp")
}

// Hyakki's llama.cpp · b12521 · Graphics card (Vulkan)
func engineWords(source, version string, build int, backend string) string {
	words := []string{engineKind(source)}
	if version == "" && build != 0 {
		version = "b" + strconv.Itoa(build)
	}
	if version != "" {
		words = append(words, version)
	}
	if backend != "" {
		words = append(words, backendWords(backend))
	}
	return strings.Join(words, " · ")
}

// only the daemon says a build cannot run: an old one may still serve one model plainly
func usableBuilds(runtime protocol.LocalRuntime) (count int) {
	for _, build := range runtime.Binaries {
		if build.Broken == "" {
			count++
		}
	}
	return count
}

// Hyakki fetches a llama.cpp of its own while none can run, and offers to beside the user's builds for as
// long as it has none; never on NixOS, where it cannot put one
func offersDownload(runtime protocol.LocalRuntime, nixos bool) bool {
	if nixos {
		return false
	}
	if runtime.State == protocol.RuntimeMissing {
		return true
	}
	managed := slices.ContainsFunc(runtime.Binaries, func(build protocol.LocalRuntimeBinary) bool {
		return build.Source == protocol.RuntimeManaged
	})
	return !managed && len(runtime.Binaries) > 0 && len(runtime.Offer) > 0
}

// which build runs is the daemon's to say, and a settings change announces no runtime
func (b *localBuilds) switchTo(e *env.Env, path string, named bool) {
	b.switching, b.target, b.named, b.problem = true, path, named, ""
	setLocalOption(e, "binary", path, func(err error) {
		if err != nil {
			b.switching, b.problem = false, err.Error()
			return
		}
		e.LocalRefresh(func(err error) {
			b.switching = false
			e.Report(err)
		})
	})
}

func (b *localBuilds) layout(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	runtime := snapshot.Local.Runtime
	binary := snapshot.Settings.Providers.Models.Binary
	b.choose.Kind, b.choose.Small, b.choose.Label, b.choose.Icon = kit.PlaqueTonal, true, i18n.T("Choose llama-server…"), look.Icon.FolderOpen
	b.choose.Busy = b.switching && b.named && !b.typed
	b.automatic.Kind, b.automatic.Small, b.automatic.Label, b.automatic.Icon = kit.PlaqueTonal, true, i18n.T("Choose automatically"), look.Icon.Undo
	b.automatic.Busy = b.switching && !b.named && b.target == ""
	if b.choose.Clicked(gtx) && !b.switching {
		// the chooser opens beside the build the user named before, where the next one is likely to lie
		start := ""
		if filepath.IsAbs(binary) {
			start = filepath.Dir(binary)
		}
		e.ChooseLlamaServer(start, func(path string) { b.switchTo(e, path, true) }, func() {
			b.typed = true
			e.Notify(i18n.T("No file chooser answered. Type the path of llama-server instead."))
		})
	}
	if b.automatic.Clicked(gtx) && binary != "" && !b.switching {
		b.switchTo(e, "", false)
	}
	children := make([]layout.FlexChild, 0, 10)
	if rows := b.rows(gtx, e, runtime); len(rows) > 0 {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2,
				i18n.T("Builds found on this computer. Hyakki uses its own when it has one, then the system's, unless you choose another."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
		)
	}
	children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		row := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.choose.Layout(gtx, th.Theme) })}
		if binary != "" {
			row = append(row, layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.automatic.Layout(gtx, th.Theme) }))
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, row...)
	}))
	if b.typed {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return b.field(gtx, e, binary)
		}))
	}
	if b.problem != "" {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, b.problem)))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func listsBuilds(runtime protocol.LocalRuntime) bool {
	return len(runtime.Binaries) > 0
}

// the list has a row for the build in use, which then says its path
func listsRunning(runtime protocol.LocalRuntime) bool {
	return slices.ContainsFunc(runtime.Binaries, func(build protocol.LocalRuntimeBinary) bool {
		return build.Path == runtime.Path
	})
}

// one row a build: what it is, where it lies, and the press that makes it the one Hyakki runs
func (b *localBuilds) rows(gtx layout.Context, e *env.Env, runtime protocol.LocalRuntime) []look.Setting {
	if !listsBuilds(runtime) {
		return nil
	}
	th := e.Theme
	if b.use == nil {
		b.use = make(map[string]*kit.Plaque)
	}
	rows := make([]look.Setting, 0, len(runtime.Binaries))
	for _, build := range runtime.Binaries {
		use := b.use[build.Path]
		if use == nil {
			use = &kit.Plaque{}
			b.use[build.Path] = use
		}
		use.Kind, use.Small, use.Label = kit.PlaqueTonal, true, i18n.T("Use this llama.cpp")
		use.Busy = b.switching && !b.named && b.target == build.Path
		current := build.Path == runtime.Path
		if use.Clicked(gtx) && !current && build.Broken == "" && !b.switching {
			b.switchTo(e, build.Path, false)
		}
		place := env.LocalPlace(build.Path)
		row := look.Setting{
			Title: engineWords(build.Source, build.Version, build.Build, build.Backend), Detail: place,
			Control: func(gtx layout.Context) layout.Dimensions { return use.Layout(gtx, th.Theme) },
		}
		if !build.Router {
			// a plain server is no worse a choice, but changing models restarts it, which is worth knowing first
			row.Detail = place + " · " + i18n.T("Loads one model at a time")
		}
		switch {
		case current:
			row.Control = kit.Claim(th.Theme, palette.Alive, i18n.T("In use"))
		case build.Broken != "":
			// the row stays, dimmed, so a build the user expected is seen with why it cannot run
			row.Unavailable = place + " · " + build.Broken
		}
		rows = append(rows, row)
	}
	return rows
}

func (b *localBuilds) field(gtx layout.Context, e *env.Env, binary string) layout.Dimensions {
	th := e.Theme
	if binary != b.pathFrom {
		b.path.SetText(binary)
		b.pathFrom = binary
	}
	b.path.Label, b.path.Placeholder, b.path.Mono = i18n.T("Your llama-server binary"), i18n.T("Managed runtime, then llama-server on PATH"), true
	b.path.Editor.SingleLine, b.path.Editor.Submit = true, true
	b.save.Kind, b.save.Small, b.save.Label = kit.PlaqueTonal, true, i18n.T("Use this binary")
	b.save.Busy = b.switching && b.named
	submitted := false
	for {
		event, ok := b.path.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	value, _ := env.LocalSource(b.path.Text())
	changed := value != binary
	if (b.save.Clicked(gtx) || submitted) && changed && !b.switching {
		b.switchTo(e, value, true)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return b.path.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !changed {
				gtx = gtx.Disabled()
			}
			return b.save.Layout(gtx, th.Theme)
		}),
	)
}
