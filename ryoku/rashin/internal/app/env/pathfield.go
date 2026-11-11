package env

import (
	"os"
	"path/filepath"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/paths"
)

// the path stays editable, so a pasted or typed folder works as well as a chosen one
type PathField struct {
	kit.TextField
	// names the chooser's window
	Title string

	browse kit.IconPlaque
	tip    look.Tip
}

func (f *PathField) Path() string { return strings.TrimSpace(f.Text()) }

func (f *PathField) Layout(gtx layout.Context, e *Env) layout.Dimensions {
	th := e.Theme
	if f.browse.Clicked(gtx) {
		e.ChooseFolder(f.Title, f.start(), f.set, func() {
			e.Notify(i18n.T("No file chooser answered. Type the folder instead."))
		})
	}
	label, help := f.Label, f.Help
	f.Label, f.Help, f.Mono = "", "", true
	defer func() { f.Label, f.Help = label, help }()
	f.browse.Icon, f.browse.Kind, f.browse.Description = look.Icon.FolderOpen, kit.PlaqueTonal, i18n.T("Choose a folder")

	children := make([]layout.FlexChild, 0, 5)
	if label != "" {
		children = append(children,
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, label)),
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
		)
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return f.TextField.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return f.tip.Layout(gtx, th, e.Overlays, f.browse.Description, func(gtx layout.Context) layout.Dimensions {
					return f.browse.Layout(gtx, th.Theme)
				})
			}),
		)
	}))
	if help != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, help)),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (f *PathField) set(path string) {
	f.SetText(path)
	end := f.Editor.Len()
	f.Editor.SetCaret(end, end)
}

// the chooser opens where the field already points, or at home
func (f *PathField) start() string {
	path := f.Path()
	if strings.HasPrefix(path, "~") {
		path = filepath.Join(paths.Home(), strings.TrimPrefix(path, "~"))
	}
	for path != "" && path != "/" && path != "." {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
		path = filepath.Dir(path)
	}
	return paths.Home()
}
