package env

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/desktop"
)

// answers arrive on the frame goroutine, a headless render has no desktop so its hooks are nil
type Desktop struct {
	Choose func(request desktop.ChooseRequest, done func(paths []string, err error))
	// done is not called when the clipboard holds no image
	ClipboardImage func(done func(path string))
	// done may be nil
	Launch func(request desktop.LaunchRequest, done func(error))
}

// chosen is not called when the reader backs out, unavailable when there is no chooser so the caller can offer a typed path
func (e *Env) ChooseFolder(title, start string, chosen func(path string), unavailable func()) {
	e.choose(desktop.ChooseRequest{Title: title, Folder: true, Start: start}, func(paths []string) { chosen(paths[0]) }, unavailable)
}

func (e *Env) ChooseFiles(title string, chosen func(paths []string), unavailable func()) {
	e.choose(desktop.ChooseRequest{Title: title, Multiple: true}, chosen, unavailable)
}

func (e *Env) choose(request desktop.ChooseRequest, chosen func(paths []string), unavailable func()) {
	if e.Desktop.Choose == nil {
		unavailable()
		return
	}
	e.Desktop.Choose(request, func(paths []string, err error) {
		switch {
		case err != nil:
			unavailable()
		case len(paths) > 0:
			chosen(paths)
		}
	})
}

// attach goes to the system chooser and a paste can bring in an image
func (e *Env) ComposerDesktop(composer *chat.Composer, options *chat.ComposerOptions) {
	attach := func(paths []string) {
		for _, path := range paths {
			composer.AddAttachment(path)
		}
		composer.Focus()
	}
	options.Attach = func() { e.ChooseFiles(i18n.T("Attach files"), attach, composer.ShowAttachField) }
	if e.Desktop.ClipboardImage != nil {
		options.PasteImage = func() { e.Desktop.ClipboardImage(composer.AddAttachment) }
	}
}
