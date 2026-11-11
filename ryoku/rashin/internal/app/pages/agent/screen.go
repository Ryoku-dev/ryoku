package agent

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// screenPanel shows an agent's own screen live beside its conversation:
// what the agent is doing on it, what it needs from you, and the controls to
// take it over, hand it back or close it. While you hold it, the pointer and
// keyboard over the frame go to the screen.
type screenPanel struct {
	header     look.PaneHeader
	take, stop kit.Plaque

	// the session whose frames the daemon sends while this panel shows it,
	// and when it was last asked to go on
	watching string
	renewed  time.Time
	seq      int64
	loading  bool
	frame    paint.ImageOp
	framed   bool

	// what you do on the frame goes to the screen one action at a time and in
	// order; typing that arrives while one is on its way joins the next
	queue   []protocol.ScreenAction
	sending bool

	// large shows the screen in place of the conversation, for reading it and
	// for working on it; the side panel holds a small one
	large bool

	area    struct{}
	pressed bool
	from    image.Point
	button  pointer.Buttons
	scrolls float32
}

// watch asks the daemon for frames of the screen shown, and stops asking for
// the one no longer shown
func (panel *screenPanel) watch(gtx layout.Context, e *env.Env, id string) {
	if panel.watching != id && panel.watching != "" {
		e.Call(protocol.MScreenControl, protocol.ScreenControlParams{ID: panel.watching, Op: protocol.ScreenUnwatch}, new(protocol.ScreenState), nil)
		panel.seq, panel.framed = 0, false
	}
	// the daemon lets a watch lapse after twenty seconds, so a panel no longer drawn stops its frames
	if panel.watching != id || gtx.Now.Sub(panel.renewed) >= 10*time.Second {
		panel.watching, panel.renewed = id, gtx.Now
		e.Call(protocol.MScreenControl, protocol.ScreenControlParams{ID: id, Op: protocol.ScreenWatch}, new(protocol.ScreenState), nil)
	}
	gtx.Execute(op.InvalidateCmd{At: panel.renewed.Add(10 * time.Second)})
}

// load reads a new frame off the UI goroutine and swaps it in when decoded
func (panel *screenPanel) load(e *env.Env, state protocol.ScreenState) {
	if state.Frame == "" || state.FrameSeq == panel.seq || panel.loading {
		return
	}
	panel.loading = true
	seq, path, id := state.FrameSeq, state.Frame, state.Session
	go func() {
		var decoded image.Image
		if raw, err := os.ReadFile(path); err == nil {
			decoded, _ = png.Decode(bytes.NewReader(raw))
		}
		e.Post(func() {
			panel.loading = false
			if panel.watching != id || decoded == nil {
				return
			}
			panel.frame, panel.framed, panel.seq = paint.NewImageOp(decoded), true, seq
		})
	}()
}

// ScreenView is an agent's screen as its live frame alone, for a page that
// watches it beside other work; while the user holds the screen, what they do
// over the frame goes to it, and the frame takes the keyboard as they take it
type ScreenView struct {
	panel screenPanel
	held  bool
}

// Layout draws the frame at the screen's proportions in the width it is given.
func (view *ScreenView) Layout(gtx layout.Context, e *env.Env, state protocol.ScreenState) layout.Dimensions {
	view.panel.watch(gtx, e, state.Session)
	view.panel.load(e, state)
	yours := state.Holder == protocol.HolderUser
	if yours && !view.held {
		gtx.Execute(key.FocusCmd{Tag: &view.panel.area})
	}
	view.held = yours
	return view.panel.layoutFrame(gtx, e, state, yours)
}

func (panel *screenPanel) layout(gtx layout.Context, e *env.Env, state protocol.ScreenState) layout.Dimensions {
	th := e.Theme
	panel.watch(gtx, e, state.Session)
	panel.load(e, state)
	yours := state.Holder == protocol.HolderUser
	panel.take.Small, panel.stop.Small = true, true
	panel.take.Kind, panel.take.Label = kit.PlaqueTonal, i18n.T("Take control")
	if state.Ask != "" {
		panel.take.Kind = kit.PlaqueGold
	}
	if yours {
		panel.take.Kind, panel.take.Label = kit.PlaqueGold, i18n.T("Hand back")
	}
	panel.stop.Kind, panel.stop.Label = kit.PlaqueQuiet, i18n.T("Close the screen")
	control := func(op string) {
		e.Call(protocol.MScreenControl, protocol.ScreenControlParams{ID: state.Session, Op: op}, new(protocol.ScreenState), e.Report)
	}
	if panel.take.Clicked(gtx) && state.State == protocol.ScreenRunning {
		if yours {
			control(protocol.ScreenRelease)
		} else {
			control(protocol.ScreenTake)
			gtx.Execute(key.FocusCmd{Tag: &panel.area})
		}
	}
	if panel.stop.Clicked(gtx) {
		control(protocol.ScreenStop)
	}
	// the frame takes what the status and the controls under it leave, at the screen's proportions
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return panel.layoutHeader(gtx, e, state) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: th.Sp.S3, Right: th.Sp.S3, Top: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
				return layout.N.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return panel.layoutFrame(gtx, e, state, yours)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: th.Sp.S3, Right: th.Sp.S3, Top: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return panel.layoutStatus(gtx, e, state, yours)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: th.Sp.S3, Right: th.Sp.S3, Top: th.Sp.S3, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return panel.take.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return panel.stop.Layout(gtx, th.Theme) }),
				)
			})
		}),
	)
}

func (panel *screenPanel) layoutHeader(gtx layout.Context, e *env.Env, state protocol.ScreenState) layout.Dimensions {
	content := look.PaneHeaderContent{
		Title: i18n.T("Screen"),
		Actions: []look.HeaderAction{
			{Icon: look.Icon.Fullscreen, Description: i18n.T("Show the screen large")},
			{Icon: look.Icon.Close, Description: i18n.T("Close") + "  Ctrl \\"},
		},
	}
	if panel.large {
		content.Actions = []look.HeaderAction{{Icon: look.Icon.FullscreenExit, Label: i18n.T("Back to the conversation")}}
	}
	switch {
	case state.State == protocol.ScreenStarting:
		content.Claim, content.ClaimTone = i18n.T("Starting"), palette.Becoming
	case state.Holder == protocol.HolderUser:
		content.Claim, content.ClaimTone = i18n.T("You have control"), palette.Attention
	case state.Ask != "":
		content.Claim, content.ClaimTone = i18n.T("Needs you"), palette.Attention
	default:
		content.Claim, content.ClaimTone = i18n.T("The agent has control"), palette.Alive
	}
	if state.Width > 0 {
		content.Facts = []look.HeaderFact{{Text: i18n.Tf("%d × %d", state.Width, state.Height)}}
	}
	dimensions := panel.header.Layout(gtx, e.Theme, e.Overlays, content)
	if index, ok := panel.header.ActionClicked(); ok {
		switch {
		case panel.large:
			panel.large = false
		case index == 0:
			panel.large = true
			e.TogglePanel()
		default:
			e.TogglePanel()
		}
	}
	return dimensions
}

// the live frame, fitted to the panel's width at the screen's proportions
func (panel *screenPanel) layoutFrame(gtx layout.Context, e *env.Env, state protocol.ScreenState, yours bool) layout.Dimensions {
	th := e.Theme
	screenW, screenH := max(state.Width, 1), max(state.Height, 1)
	if state.Width == 0 {
		screenW, screenH = 16, 10
	}
	width := min(gtx.Constraints.Max.X, gtx.Constraints.Max.Y*screenW/screenH)
	size := image.Pt(width, width*screenH/screenW)
	bounds := image.Rectangle{Max: size}
	kit.FillCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Small), th.R.Stone2)
	if panel.framed {
		inner := bounds.Inset(gtx.Dp(2))
		frameSize := panel.frame.Size()
		clipped := clip.Rect(inner).Push(gtx.Ops)
		transform := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(float32(inner.Dx())/float32(max(frameSize.X, 1)), float32(inner.Dy())/float32(max(frameSize.Y, 1)))).Offset(f32.Pt(float32(inner.Min.X), float32(inner.Min.Y)))).Push(gtx.Ops)
		panel.frame.Filter = paint.FilterLinear
		panel.frame.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		transform.Pop()
		clipped.Pop()
	} else {
		message := i18n.T("Waiting for the first frame")
		if state.State == protocol.ScreenStarting {
			message = i18n.T("Starting the agent's screen")
		}
		offset := op.Offset(image.Pt(0, size.Y/2-gtx.Dp(10))).Push(gtx.Ops)
		label := gtx
		label.Constraints = layout.Exact(image.Pt(size.X, gtx.Dp(20)))
		layout.Center.Layout(label, kit.RawText(th.Theme, th.T.Label, th.R.Fg3, message))
		offset.Pop()
	}
	if yours {
		kit.StrokeCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Small), gtx.Dp(2), th.R.Amber)
		panel.input(gtx, e, state, size)
	}
	return layout.Dimensions{Size: size}
}

// input sends what you do over the frame to the screen: a click where you
// let go, a drag from where you pressed, wheel notches, keys and text
func (panel *screenPanel) input(gtx layout.Context, e *env.Env, state protocol.ScreenState, size image.Point) {
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &panel.area)
	key.InputHintOp{Tag: &panel.area, Hint: key.HintAny}.Add(gtx.Ops)
	pointer.CursorCrosshair.Add(gtx.Ops)
	area.Pop()
	at := func(position f32.Point) (int, int) {
		x := int(position.X) * state.Width / max(size.X, 1)
		y := int(position.Y) * state.Height / max(size.Y, 1)
		return min(max(x, 0), state.Width-1), min(max(y, 0), state.Height-1)
	}
	act := func(action protocol.ScreenAction) { panel.send(e, state.Session, action) }
	for {
		raw, ok := gtx.Event(
			pointer.Filter{Target: &panel.area, Kinds: pointer.Press | pointer.Release | pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1000, Max: 1000}},
			key.FocusFilter{Target: &panel.area},
			key.Filter{Focus: &panel.area, Optional: key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper},
		)
		if !ok {
			break
		}
		switch ev := raw.(type) {
		case pointer.Event:
			x, y := at(ev.Position)
			switch ev.Kind {
			case pointer.Press:
				gtx.Execute(key.FocusCmd{Tag: &panel.area})
				panel.pressed, panel.from, panel.button = true, image.Pt(x, y), ev.Buttons
			case pointer.Release:
				if !panel.pressed {
					continue
				}
				panel.pressed = false
				from := panel.from
				switch {
				case panel.button == pointer.ButtonSecondary:
					act(protocol.ScreenAction{Kind: "right", X: x, Y: y})
				case abs(x-from.X)+abs(y-from.Y) > 6:
					act(protocol.ScreenAction{Kind: "drag", X: from.X, Y: from.Y, ToX: x, ToY: y})
				default:
					act(protocol.ScreenAction{Kind: "click", X: x, Y: y})
				}
			case pointer.Scroll:
				panel.scrolls += ev.Scroll.Y
				notches := int(panel.scrolls / 30)
				if notches != 0 {
					panel.scrolls -= float32(notches * 30)
					act(protocol.ScreenAction{Kind: "scroll", X: x, Y: y, DY: notches})
				}
			}
		case key.EditEvent:
			if ev.Text != "" {
				act(protocol.ScreenAction{Kind: "type", Text: ev.Text})
			}
		case key.Event:
			if ev.State != key.Press {
				continue
			}
			if chord := screenChord(ev); chord != "" {
				act(protocol.ScreenAction{Kind: "key", Keys: chord})
			}
		}
	}
}

func (panel *screenPanel) send(e *env.Env, id string, action protocol.ScreenAction) {
	if last := len(panel.queue) - 1; action.Kind == "type" && last >= 0 && panel.queue[last].Kind == "type" {
		panel.queue[last].Text += action.Text
	} else {
		panel.queue = append(panel.queue, action)
	}
	panel.flush(e, id)
}

func (panel *screenPanel) flush(e *env.Env, id string) {
	if panel.sending || len(panel.queue) == 0 {
		return
	}
	next := panel.queue[0]
	panel.queue = panel.queue[1:]
	panel.sending = true
	e.Call(protocol.MScreenAct, protocol.ScreenActParams{ID: id, By: protocol.HolderUser, Action: next}, new(protocol.ScreenActResult), func(err error) {
		panel.sending = false
		if err != nil {
			panel.queue = nil
			e.Report(err)
			return
		}
		panel.flush(e, id)
	})
}

// screenChord names a key press the way the screen takes it, leaving plain
// characters to arrive as typed text
func screenChord(ev key.Event) string {
	names := map[key.Name]string{
		key.NameReturn: "Return", key.NameEnter: "Return", key.NameEscape: "Escape", key.NameTab: "Tab",
		key.NameDeleteBackward: "BackSpace", key.NameDeleteForward: "Delete", key.NameHome: "Home", key.NameEnd: "End",
		key.NamePageUp: "Page_Up", key.NamePageDown: "Page_Down", key.NameUpArrow: "Up", key.NameDownArrow: "Down",
		key.NameLeftArrow: "Left", key.NameRightArrow: "Right", key.NameSpace: "space",
	}
	name, special := names[ev.Name]
	if !special {
		if strings.HasPrefix(string(ev.Name), "F") && len(ev.Name) > 1 {
			name, special = string(ev.Name), true
		} else {
			name = strings.ToLower(string(ev.Name))
		}
	}
	var modifiers []string
	if ev.Modifiers.Contain(key.ModCtrl) {
		modifiers = append(modifiers, "ctrl")
	}
	if ev.Modifiers.Contain(key.ModAlt) {
		modifiers = append(modifiers, "alt")
	}
	if ev.Modifiers.Contain(key.ModSuper) {
		modifiers = append(modifiers, "super")
	}
	if len(modifiers) == 0 && !special {
		return ""
	}
	if ev.Modifiers.Contain(key.ModShift) && (special || len(modifiers) > 0) {
		modifiers = append(modifiers, "shift")
	}
	return strings.Join(append(modifiers, name), "+")
}

// the agent's latest action, or what it needs from you while it waits
func (panel *screenPanel) layoutStatus(gtx layout.Context, e *env.Env, state protocol.ScreenState, yours bool) layout.Dimensions {
	th := e.Theme
	switch {
	case state.Ask != "" && !yours:
		return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Claim(th.Theme, palette.Attention, i18n.T("The agent needs you"))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Body, Color: th.R.Fg}.Layout(gtx, th.Theme, state.Ask)
				}),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(kit.RawText(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Take control, do it on the screen, then hand it back."))),
			)
		})
	case yours:
		text := i18n.T("Your clicks, scrolls and keys go to the screen. The agent waits until you hand it back.")
		if state.Ask != "" {
			text = state.Ask
		}
		return kit.Label{Style: th.T.Detail, Color: th.R.Fg2}.Layout(gtx, th.Theme, text)
	case state.Action != "":
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(look.IconWidget(look.Icon.Screen, 16, th.R.Fg3)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Flexed(1, kit.RawText(th.Theme, th.T.Detail, th.R.Fg2, state.Action)),
		)
	}
	return kit.Label{Style: th.T.Caption, Color: th.R.Fg3}.Layout(gtx, th.Theme, i18n.T("Apps the agent opens here stay off your desktop."))
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
