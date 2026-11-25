package settings

import (
	"image"
	"testing/fstest"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/render/desktoplook"
	rendergio "ryoku-rashin/internal/oni/render/gio"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// the character drawn in Settings, by the same motion engine and renderer the desktop surface runs
type oniPreview struct {
	spec    *spec.OniSpec
	problem string

	// the character asked for, and what drawing it found
	id        string
	request   uint64
	character *render.Character
	failed    string

	engine *motion.Engine
	state  string
	// the pose shown and when it was taken, so the next frame steps from it
	tick  motion.Tick
	stamp time.Time
	still bool
	// a still preview is posed once for each change
	posed bool

	states look.Select
}

// show asks for id's character unless it is already the one drawn or on its way
func (v *oniPreview) show(e *env.Env, id string) {
	if v.spec == nil && v.problem == "" {
		loaded, err := spec.Load()
		if err != nil {
			v.problem = i18n.Tf("The Oni motion spec does not load: %v.", err)
			return
		}
		v.spec, v.engine, v.state = loaded, motion.New(loaded), spec.StateIDs[0]
	}
	if v.spec == nil || id == "" || id == v.id {
		return
	}
	v.id, v.character, v.failed = id, nil, ""
	v.request++
	request := v.request
	// the shipped oni is built into the program; any other comes from the daemon that validated it
	if id == profile.ShippedID {
		v.drawn(profile.Default(), request)
		return
	}
	var assets protocol.OniCharacterAssets
	e.Call(protocol.MOniCharacterAssets, protocol.OniCharacterAssetsParams{ID: id}, &assets, func(err error) {
		if request != v.request {
			return
		}
		if err != nil {
			v.failed = err.Error()
			return
		}
		files := make(fstest.MapFS, len(assets.Files))
		for name, data := range assets.Files {
			files[assets.ID+"/"+name] = &fstest.MapFile{Data: data}
		}
		loaded, _ := profile.Load(files, assets.ID)
		v.drawn(loaded, request)
	})
}

func (v *oniPreview) drawn(loaded profile.Loaded, request uint64) {
	if request != v.request {
		return
	}
	if !loaded.Valid {
		v.failed = i18n.T("This character does not load.")
		return
	}
	character, err := render.Load(loaded.Profile, loaded.Files, v.spec)
	if err != nil {
		v.failed = err.Error()
		return
	}
	v.character = character
	v.restart()
}

// a new character or state starts its motion over from rest
func (v *oniPreview) restart() {
	v.engine.Reset()
	v.engine.SetState(v.state, "")
	v.stamp, v.posed = time.Time{}, false
}

// stateOptions are the states the preview can show, in the spec's order
func stateOptions() []look.SelectOption {
	names := map[string]string{
		"idle": i18n.T("Idle"), "ready": i18n.T("Ready"), "listening": i18n.T("Listening"),
		"working": i18n.T("Working"), "reading": i18n.T("Reading"), "thinking": i18n.T("Thinking"),
		"speaking": i18n.T("Speaking"), "error": i18n.T("Something went wrong"), "sleeping": i18n.T("Asleep"),
	}
	options := make([]look.SelectOption, len(spec.StateIDs))
	for index, id := range spec.StateIDs {
		options[index].Label = names[id]
		if options[index].Label == "" {
			options[index].Label = id
		}
	}
	return options
}

// how tall the character's sprite stands in the preview at the normal size, and how much of its box it fills
const (
	oniPreviewSprite unit.Dp = 180
	oniPreviewFill           = 0.6
)

// layout draws the character at size id, moving unless still, under the
// desktop's colours. It is drawn larger than the desktop draws it, so a
// character a few dozen pixels across still reads in Settings.
func (v *oniPreview) layout(gtx layout.Context, e *env.Env, size string, still bool) layout.Dimensions {
	th := e.Theme
	if v.problem != "" {
		return look.Paragraph(th, th.T.Body, th.R.Fg2, v.problem)(gtx)
	}
	if v.spec == nil {
		return layout.Dimensions{}
	}
	if still != v.still {
		v.still = still
		v.restart()
	}
	selected := 0
	for index, id := range spec.StateIDs {
		if id == v.state {
			selected = index
		}
	}
	scale, ok := v.spec.Size(size)
	if !ok {
		size, scale = protocol.OniSizeNormal, 1
	}
	normal, ok := v.spec.Size(protocol.OniSizeNormal)
	if !ok || normal <= 0 {
		normal = 1
	}
	caption := map[string]string{
		protocol.OniSizeSmall:  i18n.T("Shown larger than on the desktop, where Oni stands at the small size."),
		protocol.OniSizeNormal: i18n.T("Shown larger than on the desktop, where Oni stands at the normal size."),
		protocol.OniSizeBig:    i18n.T("Shown larger than on the desktop, where Oni stands at the big size."),
	}[size]
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			v.states.Form, v.states.Label = look.SelectQuiet, i18n.T("Preview state")
			dimensions := v.states.LayoutOptions(gtx, th, e.Overlays, stateOptions(), selected)
			if index, ok := v.states.Changed(); ok && index >= 0 && index < len(spec.StateIDs) && spec.StateIDs[index] != v.state {
				v.state = spec.StateIDs[index]
				v.restart()
			}
			return dimensions
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			width := gtx.Constraints.Max.X
			// the sprite grows with the size as on the desktop, never wider than the column lets it fill
			sprite := min(float64(gtx.Dp(oniPreviewSprite))*scale/normal, float64(width)*oniPreviewFill)
			zoom := sprite / v.spec.Character.ButtonPx
			box := image.Pt(width, int(sprite/oniPreviewFill))
			// the box keeps its height while the character loads or when it cannot be drawn
			switch {
			case v.failed != "":
				look.Place(gtx, image.Rectangle{Max: box}, look.Paragraph(th, th.T.Body, th.R.Fg2, v.failed))
				return layout.Dimensions{Size: box}
			case v.character == nil:
				look.Place(gtx, image.Rectangle{Max: box}, look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("Drawing the character…")))
				return layout.Dimensions{Size: box}
			}
			v.step(gtx)
			// the frame holds every pose and glow around the sprite; the box shows its middle
			side := v.character.Extent(zoom).X
			at := image.Pt((box.X-side)/2, (box.Y-side)/2)
			defer clip.Rect{Max: box}.Push(gtx.Ops).Pop()
			look.Place(gtx, image.Rectangle{Min: at, Max: at.Add(image.Pt(side, side))}, func(gtx layout.Context) layout.Dimensions {
				return rendergio.Layout(gtx, v.character, v.tick.Pose, desktoplook.FromDesktop(th.Desktop), float32(side))
			})
			return layout.Dimensions{Size: box}
		}),
		layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, caption)),
	)
}

// step moves the pose on by the time since the last frame, and asks for the next frame only while the motion is not done
func (v *oniPreview) step(gtx layout.Context) {
	if v.still {
		if !v.posed {
			v.tick, v.posed = v.engine.Step(0, motion.Input{Reduced: true}), true
		}
		return
	}
	var dt time.Duration
	if !v.stamp.IsZero() {
		dt = gtx.Now.Sub(v.stamp)
	}
	v.stamp = gtx.Now
	v.tick = v.engine.Step(dt, motion.Input{})
	if !v.tick.Stop {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(v.tick.Next)})
	}
}
