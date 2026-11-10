package chat

import (
	"path/filepath"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
)

func (state *composerState) processAttach(gtx layout.Context, options ComposerOptions) {
	if options.Disabled || !options.CanAttach {
		return
	}
	if state.attach.Clicked(gtx) {
		if options.Attach != nil {
			options.Attach()
		} else if state.attachOpen {
			state.attachOpen = false
		} else {
			state.showAttachField()
		}
	}
	if !state.attachOpen {
		return
	}
	if state.attachFocus {
		state.attachFocus = false
		state.attachField.Focus(gtx)
	}
	attach := state.add.Clicked(gtx)
	for {
		raw, ok := state.attachField.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := raw.(widget.SubmitEvent); ok {
			attach = true
		}
	}
	if attach {
		state.addAttachment(state.attachField.Text())
		state.attachField.SetText("")
		state.attachOpen = false
		state.wantFocus = true
	}
}

func (state *composerState) showAttachField() {
	state.attachOpen, state.attachFocus = true, true
}

func (state *composerState) addAttachment(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	for _, existing := range state.images {
		if existing == path {
			return
		}
	}
	state.images = append(state.images, path)
}

func (state *composerState) layoutAttachments(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	removed := -1
	dimensions := look.Flow{Gap: 4, LineGap: 4}.Layout(gtx, len(state.images), func(gtx layout.Context, index int) layout.Dimensions {
		path := state.images[index]
		remove := state.remove[path]
		if remove == nil {
			remove = &look.IconAction{Icon: look.Icon.Close, Description: i18n.T("Remove attachment")}
			state.remove[path] = remove
		}
		if remove.Clicked(gtx) {
			removed = index
		}
		return state.layoutChip(gtx, th, "", attachmentName(path), remove, true)
	})
	if removed >= 0 {
		delete(state.remove, state.images[removed])
		state.images = append(state.images[:removed], state.images[removed+1:]...)
	}
	return dimensions
}

func attachmentName(path string) string {
	if name := filepath.Base(path); name != "" && name != "." {
		return name
	}
	return path
}

func (state *composerState) layoutAttachField(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	state.attachField.Placeholder = i18n.T("Path to an image or a file")
	state.add.Label = i18n.T("Attach")
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return state.attachField.Layout(gtx, th) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return state.add.Layout(gtx, th) }),
	)
}
