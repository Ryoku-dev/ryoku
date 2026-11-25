package settings

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// Oni's switches are settings; the characters and their load reports are
// the daemon's to read, asked for again whenever the chosen one changes
type oniPanel struct {
	master     kit.Toggle
	courtesy   kit.Toggle
	sounds     kit.Toggle
	inspect    kit.Toggle
	follow     kit.Toggle
	character  look.Select
	size       look.Select
	reduce     look.Select
	disclosure look.Select
	corners    [4]kit.Plaque
	reset      kit.Plaque
	// the way to the Hands page, and a press of it waiting for the next
	// frame: a jump made while the list lays out would be undone by it
	hands   kit.Plaque
	toHands bool

	// a question about the perched window and the picture waiting for a yes
	ask oniAsk
	// talking to Oni, and dictation
	voice     oniVoice
	dictation oniDictation
	// guides, the keep offer and the kept walkthroughs
	guide oniGuide
	// quick commands, their reminders, routines and log
	reflex oniReflex
	// the MCP bridge: its switch, tools and programs
	mcp         oniMCPServe
	writing     oniWriting
	links       [6]kit.Plaque
	destination int
	navigate    bool

	list protocol.OniCharactersResult
	// the character setting the list was read under, and whether a read is out
	listed    bool
	listedFor string
	loading   bool
	problem   string
	busy      bool

	preview oniPreview
}

// leaving Settings forgets the list, what is installed, the dictation
// store, the kept walkthroughs and the quick commands' lists, so coming
// back reads them again
func (o *oniPanel) forget() {
	o.listed = false
	o.voice.forget()
	o.dictation.forget()
	o.guide.forget()
	o.reflex.forget()
	o.mcp.forget()
	o.writing.approvals.stop()
}

func (o *oniPanel) read(e *env.Env, chosen string) {
	o.loading = true
	var result protocol.OniCharactersResult
	e.Call(protocol.MOniCharacters, protocol.Empty{}, &result, func(err error) {
		o.loading, o.listed, o.listedFor = false, true, chosen
		if err != nil {
			o.problem = err.Error()
			return
		}
		o.list, o.problem = result, ""
	})
}

// choose asks the daemon to make id current, or with reset the shipped oni; the settings it changes bring the list back
func (o *oniPanel) choose(e *env.Env, id string, reset bool) {
	o.busy = true
	method, params := protocol.MOniCharacterSelect, any(protocol.OniCharacterSelectParams{ID: id})
	if reset {
		method, params = protocol.MOniCharacterReset, protocol.Empty{}
	}
	e.Call(method, params, nil, func(err error) {
		o.busy = false
		e.Report(err)
	})
}

func characterLabel(c protocol.OniCharacter) string {
	if c.Name != "" {
		return c.Name
	}
	return c.ID
}

// what loading said about a character, short enough for the picker
func characterReport(c protocol.OniCharacter) string {
	switch {
	case !c.Valid:
		return i18n.Tf("Does not load: %s", strings.Join(c.Errors, "; "))
	case len(c.Fallbacks) > 0:
		return i18n.Tf("%d slots fall back", len(c.Fallbacks))
	case c.Shipped:
		return i18n.T("Comes with Hyakki")
	}
	return i18n.T("Every slot loads")
}

// oniBlocks are the section's head and each of its groups, so a frame lays
// out only the groups shown
func (p *Page) oniBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	th := e.Theme
	o := &p.oni
	settings := e.Snapshot().Settings
	oni, defaults := settings.Oni, p.defaults.Oni
	if !o.loading && (!o.listed || o.listedFor != oni.Character) {
		o.read(e, oni.Character)
	}
	active := o.list.Active
	if active == "" {
		active = protocol.OniDefaultCharacter
	}
	o.preview.show(e, active)

	o.master.Value, o.courtesy.Value, o.sounds.Value = oni.Master, oni.Courtesy, oni.Sounds
	master := i18n.T("Oni on the desktop")
	masterDetail := i18n.T("Oni stands at the edge of the screen and shows what Hyakki is doing. Nothing starts at login.")
	if !settings.Enabled {
		masterDetail = i18n.T("Hyakki is off, so Oni stays away until you turn Hyakki on.")
	}
	presence := []look.Setting{
		{Title: master, Detail: masterDetail, Changed: oni.Master != defaults.Master, Reset: func() { set(e, "oni", map[string]any{"master": defaults.Master}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, master, &o.master, func(on bool) { set(e, "oni", map[string]any{"master": on}) })
			}},
	}

	options := make([]look.SelectOption, len(o.list.Characters))
	selected := -1
	for index, c := range o.list.Characters {
		options[index] = look.SelectOption{Label: characterLabel(c), Detail: characterReport(c), Disabled: !c.Valid}
		if c.ID == active {
			selected = index
		}
	}
	o.reset.Kind, o.reset.Small, o.reset.Label = kit.PlaqueTonal, true, i18n.T("Reset to Oni")
	if o.reset.Clicked(gtx) && active != protocol.OniDefaultCharacter && !o.busy {
		o.choose(e, "", true)
	}
	character := []look.Setting{
		{Title: i18n.T("Choose a character"), Detail: i18n.T("Characters live in their own folders. One that does not load is listed with what stopped it and cannot be chosen."),
			Control: func(gtx layout.Context) layout.Dimensions {
				if o.busy || !o.listed {
					gtx = gtx.Disabled()
				}
				o.character.Label = i18n.T("Character")
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				dimensions := o.character.LayoutOptions(gtx, th, e.Overlays, options, selected)
				if index, ok := o.character.Changed(); ok && index >= 0 && index < len(o.list.Characters) {
					if c := o.list.Characters[index]; c.Valid && c.ID != active && !o.busy {
						o.choose(e, c.ID, false)
					}
				}
				return dimensions
			}},
		{Title: i18n.T("Preview"), Below: true, Control: func(gtx layout.Context) layout.Dimensions {
			return o.preview.layout(gtx, e, oni.Size, oniStill(th.Desktop.ReduceMotion, oni.ReduceMotion))
		}},
		{Title: i18n.T("Shipped character"), Detail: i18n.T("Oni comes with Hyakki and always loads."),
			Control: func(gtx layout.Context) layout.Dimensions {
				if active == protocol.OniDefaultCharacter || o.busy {
					gtx = gtx.Disabled()
				}
				return o.reset.Layout(gtx, th.Theme)
			}},
	}
	if o.problem != "" {
		character = append(character, look.Setting{Title: i18n.T("Characters"), Detail: o.problem})
	}
	for _, c := range o.list.Characters {
		if c.ID == active && len(c.Fallbacks) > 0 {
			lines := make([]string, len(c.Fallbacks))
			for index, f := range c.Fallbacks {
				lines[index] = i18n.Tf("%s: %s, drawn as %s", f.Slot, f.Reason, f.Used)
			}
			character = append(character, look.Setting{Title: i18n.T("What falls back"), Detail: strings.Join(lines, "\n")})
		}
	}
	var broken []look.Setting
	for _, c := range o.list.Characters {
		if !c.Valid {
			broken = append(broken, look.Setting{Title: characterLabel(c), Detail: strings.Join(c.Errors, "\n")})
		}
	}

	// Feature controls live on their own sub-pages. Only presence and
	// character resources are read while this landing page is built.

	o.hands.Kind, o.hands.Small, o.hands.Label = kit.PlaqueTonal, true, i18n.T("Open Hands")
	handsDetail := i18n.T("Off. Oni can fill fields and click ordinary controls in one browser tab you authorize; it never submits, sends or pays. Hands has a page of its own.")
	if oni.Hands == protocol.OniHandsBrowser {
		handsDetail = i18n.T("On. Open the browser, authorize a tab and give Hands a goal on its own page.")
	}
	hands := []look.Setting{{Title: i18n.T("Hands in a browser"), Detail: handsDetail,
		Control: func(gtx layout.Context) layout.Dimensions {
			if o.hands.Clicked(gtx) {
				o.toHands = true
				gtx.Execute(op.InvalidateCmd{})
			}
			return look.Named(gtx, i18n.T("Open the Hands settings"), func(gtx layout.Context) layout.Dimensions { return o.hands.Layout(gtx, th.Theme) })
		}}}

	sizes := [...]string{protocol.OniSizeSmall, protocol.OniSizeNormal, protocol.OniSizeBig}
	reduces := [...]string{protocol.OniReduceFollow, protocol.OniReduceOn, protocol.OniReduceOff}
	corners := [...]string{"tl", "tr", "bl", "br"}
	behaviour := []look.Setting{
		{Title: i18n.T("Size"), Detail: i18n.T("How big Oni stands on the desktop. The preview shows each size as drawn."),
			Changed: oni.Size != defaults.Size, Reset: func() { set(e, "oni", map[string]any{"size": defaults.Size}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				o.size.Label = i18n.T("Size")
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				dimensions := o.size.LayoutOptions(gtx, th, e.Overlays, look.Labelled([]string{i18n.T("Small"), i18n.T("Normal"), i18n.T("Big")}), indexOf(sizes[:], oni.Size))
				if index, ok := o.size.Changed(); ok && index >= 0 && index < len(sizes) {
					set(e, "oni", map[string]any{"size": sizes[index]})
				}
				return dimensions
			}},
		{Title: i18n.T("Reduce motion"), Detail: i18n.T("Reduced, Oni holds one still pose for each change instead of moving."),
			Changed: oni.ReduceMotion != defaults.ReduceMotion, Reset: func() { set(e, "oni", map[string]any{"reduceMotion": defaults.ReduceMotion}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				o.reduce.Label = i18n.T("Reduce motion")
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				labels := []string{i18n.T("Follow the desktop"), i18n.T("On"), i18n.T("Off")}
				dimensions := o.reduce.LayoutOptions(gtx, th, e.Overlays, look.Labelled(labels), indexOf(reduces[:], oni.ReduceMotion))
				if index, ok := o.reduce.Changed(); ok && index >= 0 && index < len(reduces) {
					set(e, "oni", map[string]any{"reduceMotion": reduces[index]})
				}
				return dimensions
			}},
		{Title: i18n.T("Step aside for full screen"), Detail: i18n.T("Oni hides while a window fills the screen, such as a game or a video, and comes back after."),
			Changed: oni.Courtesy != defaults.Courtesy, Reset: func() { set(e, "oni", map[string]any{"courtesy": defaults.Courtesy}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Step aside for full screen"), &o.courtesy, func(on bool) { set(e, "oni", map[string]any{"courtesy": on}) })
			}},
		{Title: i18n.T("Sounds"), Detail: i18n.T("Oni plays a short motif when its strand reaches out, lands and comes back. It stays silent while the microphone is live."),
			Changed: oni.Sounds != defaults.Sounds, Reset: func() { set(e, "oni", map[string]any{"sounds": defaults.Sounds}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Sounds"), &o.sounds, func(on bool) { set(e, "oni", map[string]any{"sounds": on}) })
			}},
		{Title: i18n.T("Pin to a corner"), Detail: i18n.T("Oni moves to that corner of the screen it is on. Dragging it frees it again."), Below: true,
			Control: func(gtx layout.Context) layout.Dimensions {
				labels := [...]string{i18n.T("Top left"), i18n.T("Top right"), i18n.T("Bottom left"), i18n.T("Bottom right")}
				pinned := [...]string{i18n.T("Oni is pinned to the top left corner."), i18n.T("Oni is pinned to the top right corner."),
					i18n.T("Oni is pinned to the bottom left corner."), i18n.T("Oni is pinned to the bottom right corner.")}
				children := make([]layout.FlexChild, 0, 2*len(corners))
				for index := range corners {
					plaque := &o.corners[index]
					plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, labels[index]
					if plaque.Clicked(gtx) {
						e.Call(protocol.MOniPlace, protocol.OniPlaceParams{Corner: corners[index]}, nil, func(err error) {
							if err == nil && e.Notify != nil {
								e.Notify(pinned[index])
							}
							e.Report(err)
						})
					}
					if index > 0 {
						children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout))
					}
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) }))
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			}},
	}

	head := func(gtx layout.Context) layout.Dimensions { return sectionHead(gtx, th, sectionOni) }
	groups := []layout.Widget{
		group(th, i18n.T("Presence"), "", presence, nil),
		group(th, i18n.T("Character"), "", character, nil),
	}
	if len(broken) > 0 {
		groups = append(groups, group(th, i18n.T("Characters that do not load"), i18n.T("Fix what each one names in its folder and it is listed again."), broken, nil))
	}
	groups = append(groups,
		group(th, i18n.T("How Oni behaves"), "", behaviour, nil),
		group(th, i18n.T("Oni settings"), "", p.oniPageLinks(e), nil),
		group(th, i18n.T("Hands"), "", hands, nil),
	)
	return spacedBlocks(th, append(blocks, head), groups...)
}

// oniLookingRows are the screen ask's settings, the question about the
// perched window and what it came to
func (p *Page) oniLookingRows(e *env.Env, oni, defaults protocol.OniSettings) []look.Setting {
	th := e.Theme
	o := &p.oni
	o.inspect.Value, o.follow.Value = oni.Inspect, oni.PerchFollowsFocus
	inspect := i18n.T("Let Oni look at a window")
	follow := i18n.T("Follow the focused window")
	disclosures := [...]string{protocol.OniDisclosureSession, protocol.OniDisclosureAlways}
	unavailable := ""
	switch {
	case !protocol.EffectiveOni(e.Snapshot().Settings).Master:
		unavailable = i18n.T("Oni is off, so it cannot look at a window.")
	case !oni.Inspect:
		unavailable = i18n.T("Turn on Let Oni look at a window first.")
	}
	rows := []look.Setting{
		{Title: inspect, Detail: i18n.T("When you ask, Oni takes a picture of only the window it is perched on. You see the picture and where it goes before anything is sent."),
			Changed: oni.Inspect != defaults.Inspect, Reset: func() { set(e, "oni", map[string]any{"inspect": defaults.Inspect}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, inspect, &o.inspect, func(on bool) { set(e, "oni", map[string]any{"inspect": on}) })
			}},
		{Title: i18n.T("Ask before sending"), Detail: i18n.T("Whether Oni shows the picture and its destination once per session or before every picture."),
			Changed: oni.Disclosure != defaults.Disclosure, Reset: func() { set(e, "oni", map[string]any{"disclosure": defaults.Disclosure}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				o.disclosure.Label = i18n.T("Ask before sending")
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				options := []look.SelectOption{
					{Label: i18n.T("Once per session"), Detail: i18n.T("Oni asks before the first picture goes to a destination, then not again until it leaves the desktop.")},
					{Label: i18n.T("Every time"), Detail: i18n.T("Oni asks before every picture.")},
				}
				dimensions := o.disclosure.LayoutOptions(gtx, th, e.Overlays, options, indexOf(disclosures[:], oni.Disclosure))
				if index, ok := o.disclosure.Changed(); ok && index >= 0 && index < len(disclosures) {
					set(e, "oni", map[string]any{"disclosure": disclosures[index]})
				}
				return dimensions
			}},
		{Title: follow, Detail: i18n.T("While perched, Oni moves to whichever window you focus. Off, it stays on the window you put it on."),
			Changed: oni.PerchFollowsFocus != defaults.PerchFollowsFocus, Reset: func() { set(e, "oni", map[string]any{"perchFollowsFocus": defaults.PerchFollowsFocus}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, follow, &o.follow, func(on bool) { set(e, "oni", map[string]any{"perchFollowsFocus": on}) })
			}},
		{Title: i18n.T("Ask about the perched window"), Detail: i18n.T("Oni answers in words, or points at the part of the window you asked about."),
			Unavailable: unavailable, Below: true,
			Control: func(gtx layout.Context) layout.Dimensions { return o.ask.layoutEntry(gtx, e) }},
	}
	if row, ok := o.ask.result(); ok {
		rows = append(rows, row)
	}
	return rows
}

// whether the preview holds still: the app's own reduced motion always wins,
// since a moving preview in a reduced app would be the one thing that moves;
// otherwise Oni's setting, which can follow the desktop
func oniStill(desktop bool, setting string) bool {
	switch setting {
	case protocol.OniReduceOn:
		return true
	case protocol.OniReduceOff:
		return anim.Reduced()
	}
	return anim.Reduced() || desktop
}

func indexOf(values []string, value string) int {
	for index, each := range values {
		if each == value {
			return index
		}
	}
	return 0
}
