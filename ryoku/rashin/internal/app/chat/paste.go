package chat

import (
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
)

// gio only reads text off the clipboard, so a paste also lets the page look for an image there
func (state *composerState) processPaste(gtx layout.Context, options ComposerOptions) {
	if options.Disabled || options.PasteImage == nil || !options.CanAttach {
		return
	}
	for {
		raw, ok := gtx.Event(key.Filter{Focus: &state.editor, Name: "V", Required: key.ModShortcut})
		if !ok {
			return
		}
		if pressed, ok := raw.(key.Event); ok && pressed.State == key.Press {
			gtx.Execute(clipboard.ReadCmd{Tag: &state.editor})
			options.PasteImage()
		}
	}
}
