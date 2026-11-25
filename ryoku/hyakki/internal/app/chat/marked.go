package chat

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

// the one stretch of the transcript that is marked, so marking in one place lets go of the last
type marked struct {
	key   string
	tag   event.Tag
	text  string
	clear func()
	// the keyboard has been seen on it, so not seeing it there means it went elsewhere
	held bool
}

// text stays marked only while it has the keyboard, marks left behind would look copyable and not be
func (state *viewState) hold(gtx layout.Context, key string, tag event.Tag, changed, marking, focused bool, text func() string, clear func()) {
	switch {
	case changed && marking:
		if state.marked.key != key && state.marked.clear != nil {
			state.marked.clear()
			// the text that lost its marks may already be drawn this frame
			gtx.Execute(op.InvalidateCmd{})
		}
		// read now: the lines it was read from may be gone by the time it is wanted
		state.marked = marked{key: key, tag: tag, text: text(), clear: clear, held: focused}
	case state.marked.key != key:
	case !marking:
		state.marked = marked{}
	case focused:
		state.marked.held = true
	case state.marked.held && !state.menuOpen():
		clear()
		state.marked = marked{}
		gtx.Execute(op.InvalidateCmd{})
	}
}

// a menu takes the keyboard too, and may be the one offering to copy what is marked
func (state *viewState) menuOpen() bool {
	if state.overlays == nil {
		return false
	}
	_, open := state.overlays.Top()
	return open
}

func (state *viewState) markedText() string { return state.marked.text }

// escape lets go of what is marked and hands the keyboard back
func (state *viewState) releaseMarked(gtx layout.Context) {
	if state.marked.tag == nil {
		return
	}
	for {
		pressed, ok := gtx.Event(key.Filter{Focus: state.marked.tag, Name: key.NameEscape})
		if !ok {
			return
		}
		if pressed, ok := pressed.(key.Event); !ok || pressed.State != key.Press {
			continue
		}
		state.marked.clear()
		state.marked = marked{}
		state.act(Action{Kind: ActRelease})
	}
}

// text takes the keyboard on a press, one that marked nothing has to give it back
func (state *viewState) watchPresses(gtx layout.Context) {
	if state.settling > 0 {
		state.settling--
	}
	for {
		pointed, ok := gtx.Event(pointer.Filter{Target: &state.presses, Kinds: pointer.Press | pointer.Release})
		if !ok {
			return
		}
		if pointed, ok := pointed.(pointer.Event); ok {
			state.settling = 0
			if pointed.Kind == pointer.Release {
				// the keyboard can arrive a frame after the press that asked for it
				state.settling = 3
			}
		}
	}
}
