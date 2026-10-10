package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

type niriImportMapper struct {
	desktop  map[string]any
	niri     map[string]any
	items    []niriImportItem
	losses   []niriImportLoss
	binds    []niriImportBind
	conflict []niriImportConflict
	preserve []string
}

func mapNiriImport(nodes []*kdlNode) niriImportScan {
	m := &niriImportMapper{desktop: map[string]any{}, niri: map[string]any{}}
	for _, node := range nodes {
		m.mapTop(node)
	}
	patch := map[string]any{"desktop": m.desktop, "wm": map[string]any{wm.ProviderNiri: m.niri}}
	return niriImportScan{
		Items: m.items, Conflicts: m.conflict, Losses: m.losses,
		Patch: patch, Binds: m.binds, Preserved: strings.Join(m.preserve, "\n\n"),
	}
}

func (m *niriImportMapper) mapTop(node *kdlNode) {
	switch node.Name {
	case "input":
		m.mapInput(node)
	case "layout":
		m.mapLayout(node)
	case "animations":
		m.mapAnimations(node)
	case "environment":
		m.mapEnvironment(node)
	case "spawn-at-startup", "spawn-sh-at-startup":
		m.mapSpawn(node)
	case "cursor":
		m.mapCursor(node)
	case "binds":
		m.mapBinds(node)
	case "window-rule":
		m.mapWindowRule(node)
	case "layer-rule":
		m.mapLayerRule(node)
	case "output":
		m.keep(node, "output", "Display pins have no neutral on-disk store, so this output block stays in the user include.")
	case "prefer-no-csd":
		setNested(m.niri, true, "preferNoCsd")
		m.item(node, "setting", true)
	case "screenshot-path":
		if v, ok := node.Arg(0); ok {
			setNested(m.niri, v.String(), "screenshotPath")
			m.item(node, "setting", true)
		} else {
			m.keep(node, "setting", "A screenshot path without a value cannot be mapped.")
		}
	case "hotkey-overlay":
		m.mapHotkeyOverlay(node)
	case "gestures":
		m.mapGestures(node)
	case "overview":
		m.mapOverview(node)
	case "recent-windows":
		off := node.Child("off") != nil
		setNested(m.niri, !off, "recentWindows")
		m.item(node, "setting", true)
	case "clipboard":
		if node.Child("disable-primary") != nil {
			setNested(m.desktop, false, "input", "middleClickPaste")
			m.item(node, "setting", true)
		} else {
			m.keep(node, "setting", "This clipboard block has no Ryoku setting.")
		}
	default:
		m.keep(node, "raw", fmt.Sprintf("Unknown top-level node %q is preserved.", node.Name))
	}
}

func (m *niriImportMapper) mapInput(node *kdlNode) {
	for _, child := range node.Children {
		switch child.Name {
		case "keyboard":
			m.mapKeyboard(child)
		case "touchpad":
			m.mapPointer(child, true)
		case "mouse":
			m.mapPointer(child, false)
		case "focus-follows-mouse":
			setNested(m.desktop, 1, "input", "followMouse")
			if v, ok := child.Props["max-scroll-amount"]; ok {
				s := strings.TrimSuffix(v.String(), "%")
				if n, err := strconv.Atoi(s); err == nil {
					setNested(m.niri, n, "focusFollowsMouseScroll")
				}
			}
		case "warp-mouse-to-focus":
			mode := "separate"
			if v, ok := child.Props["mode"]; ok {
				mode = v.String()
			}
			setNested(m.niri, mode, "warpMouseToFocus")
		case "workspace-auto-back-and-forth":
			setNested(m.niri, true, "workspaceAutoBackAndForth")
		case "disable-power-key-handling":
			setNested(m.niri, true, "disablePowerKey")
		case "mod-key":
			setFirstArg(m.niri, child, "modKey")
		case "mod-key-nested":
			setFirstArg(m.niri, child, "modKeyNested")
		default:
			m.keepWrapped(node.Name, child, "setting", fmt.Sprintf("input.%s has no Ryoku setting.", child.Name))
		}
	}
	m.item(node, "setting", true)
}

func (m *niriImportMapper) mapKeyboard(node *kdlNode) {
	for _, child := range node.Children {
		switch child.Name {
		case "xkb":
			for _, x := range child.Children {
				switch x.Name {
				case "layout":
					setFirstArg(m.desktop, x, "input", "kbLayout")
				case "variant":
					setFirstArg(m.desktop, x, "input", "kbVariant")
				case "options":
					setFirstArg(m.desktop, x, "input", "kbOptions")
				default:
					m.keepWrapped("input { keyboard { xkb", x, "setting", fmt.Sprintf("keyboard.xkb.%s is not modelled.", x.Name))
				}
			}
		case "repeat-rate":
			setFirstInt(m.desktop, child, "input", "repeatRate")
		case "repeat-delay":
			setFirstInt(m.desktop, child, "input", "repeatDelay")
		case "numlock":
			setNested(m.desktop, true, "input", "numlockByDefault")
		default:
			m.keepWrapped("input { keyboard", child, "setting", fmt.Sprintf("keyboard.%s is not modelled.", child.Name))
		}
	}
}

func (m *niriImportMapper) mapPointer(node *kdlNode, touchpad bool) {
	for _, child := range node.Children {
		path := []string{"input"}
		switch child.Name {
		case "tap":
			if touchpad {
				setNested(m.desktop, nodeEnabled(child), append(path, "tapToClick")...)
			} else {
				m.keepWrapped("input { mouse", child, "setting", "Mouse tap has no Ryoku setting.")
			}
		case "natural-scroll":
			key := "mouseNaturalScroll"
			if touchpad {
				key = "naturalScroll"
			}
			setNested(m.desktop, nodeEnabled(child), append(path, key)...)
		case "scroll-factor":
			key := "mouseScrollFactor"
			if touchpad {
				key = "touchScrollFactor"
			}
			setFirstFloat(m.desktop, child, append(path, key)...)
		case "dwt":
			if touchpad {
				setNested(m.desktop, nodeEnabled(child), append(path, "disableWhileTyping")...)
			}
		case "drag":
			if touchpad {
				setNested(m.desktop, nodeEnabled(child), append(path, "tapAndDrag")...)
			}
		case "click-method":
			if touchpad {
				v, _ := child.Arg(0)
				setNested(m.desktop, v.String() == "clickfinger", append(path, "clickfinger")...)
			}
		case "middle-emulation":
			setNested(m.desktop, nodeEnabled(child), append(path, "middleEmulation")...)
		case "left-handed":
			setNested(m.desktop, nodeEnabled(child), append(path, "leftHanded")...)
		case "accel-speed":
			setFirstFloat(m.desktop, child, append(path, "sensitivity")...)
		case "accel-profile":
			setFirstArg(m.desktop, child, append(path, "accelProfile")...)
		case "off":
			m.keepWrapped("input { "+node.Name, child, "setting", "A disabled pointer device is preserved because the neutral input store has no device-off field.")
		default:
			m.keepWrapped("input { "+node.Name, child, "setting", fmt.Sprintf("%s.%s is not modelled.", node.Name, child.Name))
		}
	}
}

func (m *niriImportMapper) mapLayout(node *kdlNode) {
	border, ring := node.Child("border"), node.Child("focus-ring")
	if border != nil || ring != nil {
		borderOn := border != nil && blockOn(border)
		ringOn := ring != nil && blockOn(ring)
		frame := "border"
		if borderOn && ringOn {
			frame = "both"
		} else if ringOn {
			frame = "focusRing"
		}
		setNested(m.niri, frame, "frame")
		active := border
		if ringOn {
			active = ring
		}
		if active != nil {
			m.mapFrame(active)
		}
	}
	for _, child := range node.Children {
		switch child.Name {
		case "gaps":
			setFirstInt(m.desktop, child, "appearance", "gapsOut")
		case "background-color":
			setFirstArg(m.niri, child, "backgroundColor")
		case "default-column-width":
			if p := child.Child("proportion"); p != nil {
				setFirstFloat(m.niri, p, "defaultColumnWidth")
			} else {
				m.keepWrapped(node.Name, child, "setting", "A fixed default column width has no proportional Ryoku field.")
			}
		case "preset-column-widths":
			setNested(m.niri, proportions(child), "presetColumnWidths")
		case "preset-window-heights":
			setNested(m.niri, proportions(child), "presetWindowHeights")
		case "default-column-display":
			setFirstArg(m.niri, child, "defaultColumnDisplay")
		case "center-focused-column":
			setFirstArg(m.niri, child, "centerFocusedColumn")
		case "always-center-single-column":
			setNested(m.niri, nodeEnabled(child), "alwaysCenterSingleColumn")
		case "empty-workspace-above-first":
			setNested(m.niri, nodeEnabled(child), "emptyWorkspaceAboveFirst")
		case "struts":
			for _, x := range child.Children {
				if x.Name == "left" || x.Name == "right" || x.Name == "top" || x.Name == "bottom" {
					setFirstInt(m.niri, x, "struts", x.Name)
				} else {
					m.keepWrapped("layout { struts", x, "setting", fmt.Sprintf("struts.%s is not modelled.", x.Name))
				}
			}
		case "shadow":
			m.mapShadow(child)
		case "tab-indicator":
			for _, x := range child.Children {
				switch x.Name {
				case "width":
					setFirstInt(m.niri, x, "tabIndicatorWidth")
				case "hide-when-single-tab":
					setNested(m.niri, nodeEnabled(x), "tabIndicatorHideSingle")
				default:
					m.keepWrapped("layout { tab-indicator", x, "setting", fmt.Sprintf("tab-indicator.%s is not modelled.", x.Name))
				}
			}
		case "insert-hint":
			setNested(m.niri, child.Child("off") == nil, "insertHint")
		case "border", "focus-ring":
		default:
			m.keepWrapped(node.Name, child, "setting", fmt.Sprintf("layout.%s has no Ryoku setting.", child.Name))
		}
	}
	m.item(node, "setting", true)
}

func (m *niriImportMapper) mapFrame(node *kdlNode) {
	for _, child := range node.Children {
		switch child.Name {
		case "width":
			setFirstInt(m.desktop, child, "appearance", "borderSize")
		case "active-color":
			setFirstArg(m.desktop, child, "appearance", "activeBorder")
		case "inactive-color":
			setFirstArg(m.desktop, child, "appearance", "inactiveBorder")
		case "urgent-color":
			setFirstArg(m.niri, child, "urgentColor")
		case "active-gradient":
			setNested(m.niri, true, "borderGradient")
			for key, dst := range map[string]string{"from": "gradientFrom", "to": "gradientTo", "relative-to": "gradientRelativeTo"} {
				if v, ok := child.Props[key]; ok {
					setNested(m.niri, v.String(), dst)
				}
			}
			if v, ok := child.Props["angle"]; ok {
				if n, yes := v.Int(); yes {
					setNested(m.niri, n, "gradientAngle")
				}
			}
		case "on", "off":
		default:
			m.keepWrapped("layout { "+node.Name, child, "setting", fmt.Sprintf("%s.%s is not modelled.", node.Name, child.Name))
		}
	}
}

func (m *niriImportMapper) mapShadow(node *kdlNode) {
	setNested(m.desktop, blockOn(node), "appearance", "shadowEnabled")
	for _, child := range node.Children {
		switch child.Name {
		case "softness":
			setFirstInt(m.desktop, child, "appearance", "shadowRange")
		case "spread":
			setFirstInt(m.desktop, child, "appearance", "shadowSpread")
		case "color":
			setFirstArg(m.desktop, child, "appearance", "shadowColor")
		case "offset":
			if v, ok := child.Props["x"]; ok {
				if n, yes := v.Int(); yes {
					setNested(m.desktop, n, "appearance", "shadowOffsetX")
				}
			}
			if v, ok := child.Props["y"]; ok {
				if n, yes := v.Int(); yes {
					setNested(m.desktop, n, "appearance", "shadowOffsetY")
				}
			}
		case "on", "off":
		default:
			m.keepWrapped("layout { shadow", child, "setting", fmt.Sprintf("shadow.%s is not modelled.", child.Name))
		}
	}
}

func (m *niriImportMapper) mapAnimations(node *kdlNode) {
	if node.Child("off") != nil {
		setNested(m.desktop, false, "appearance", "animations")
		m.item(node, "setting", true)
		return
	}
	setNested(m.desktop, true, "appearance", "animations")
	kinds := map[string]string{
		"workspace-switch": "workspaceSwitch", "window-open": "windowOpen", "window-close": "windowClose",
		"horizontal-view-movement": "horizontalViewMovement", "window-movement": "windowMovement",
		"window-resize": "windowResize", "config-notification-open-close": "configNotificationOpenClose",
		"screenshot-ui-open": "screenshotUiOpen", "overview-open-close": "overviewOpenClose",
	}
	for _, child := range node.Children {
		if child.Name == "slowdown" {
			setFirstFloat(m.niri, child, "animationSlowdown")
			continue
		}
		dst, ok := kinds[child.Name]
		if !ok {
			if child.Name != "off" {
				m.keepWrapped(node.Name, child, "setting", fmt.Sprintf("Animation kind %q is not modelled.", child.Name))
			}
			continue
		}
		base := []string{"anim", dst}
		if child.Child("off") != nil {
			setNested(m.niri, "off", append(base, "mode")...)
			continue
		}
		for _, spec := range child.Children {
			switch spec.Name {
			case "spring":
				setNested(m.niri, "spring", append(base, "mode")...)
				setPropFloat(m.niri, spec, "damping-ratio", append(base, "dampingRatio")...)
				setPropInt(m.niri, spec, "stiffness", append(base, "stiffness")...)
				setPropFloat(m.niri, spec, "epsilon", append(base, "epsilon")...)
			case "duration-ms":
				setNested(m.niri, "ease", append(base, "mode")...)
				setFirstInt(m.niri, spec, append(base, "durationMs")...)
			case "curve":
				setNested(m.niri, "ease", append(base, "mode")...)
				setFirstArg(m.niri, spec, append(base, "curve")...)
			case "off":
			default:
				m.keepWrapped("animations { "+child.Name, spec, "setting", fmt.Sprintf("Animation property %q is not modelled.", spec.Name))
			}
		}
	}
	m.item(node, "setting", true)
}

func (m *niriImportMapper) mapEnvironment(node *kdlNode) {
	for _, child := range node.Children {
		v, ok := child.Arg(0)
		if !ok {
			m.keepWrapped(node.Name, child, "env", fmt.Sprintf("Environment variable %q has no value.", child.Name))
			continue
		}
		switch child.Name {
		case "BROWSER":
			setNested(m.desktop, v.String(), "apps", "browser")
		case "TERMINAL":
			setNested(m.desktop, v.String(), "apps", "terminal")
		default:
			appendNested(m.desktop, map[string]any{"key": child.Name, "value": v.String()}, "env")
		}
	}
	m.item(node, "env", true)
}

func (m *niriImportMapper) mapSpawn(node *kdlNode) {
	if len(node.Args) == 0 {
		m.keep(node, "exec", "A startup command without arguments cannot be mapped.")
		return
	}
	command := node.Args[0].String()
	if node.Name == "spawn-at-startup" {
		parts := make([]string, 0, len(node.Args))
		for _, arg := range node.Args {
			parts = append(parts, shellQuote(arg.String()))
		}
		command = strings.Join(parts, " ")
	}
	appendNested(m.desktop, map[string]any{"command": command}, "autostart")
	m.item(node, "exec", true)
}

func (m *niriImportMapper) mapCursor(node *kdlNode) {
	for _, child := range node.Children {
		switch child.Name {
		case "xcursor-theme":
			setFirstArg(m.desktop, child, "cursor", "theme")
		case "xcursor-size":
			setFirstInt(m.desktop, child, "cursor", "size")
		case "hide-after-inactive-ms":
			if v, ok := child.Arg(0); ok {
				if n, yes := v.Int(); yes {
					setNested(m.desktop, (n+999)/1000, "cursor", "inactiveTimeout")
					if n%1000 != 0 {
						m.keepWrapped(node.Name, child, "setting", "Sub-second cursor timeout precision stays in the native user include.")
					}
				}
			}
		case "hide-when-typing":
			setNested(m.desktop, nodeEnabled(child), "cursor", "hideOnKeyPress")
		default:
			m.keepWrapped(node.Name, child, "setting", fmt.Sprintf("cursor.%s is not modelled.", child.Name))
		}
	}
	m.item(node, "setting", true)
}

func (m *niriImportMapper) mapBinds(node *kdlNode) {
	shipped := niriImportShipped()
	seen := map[string]bool{}
	for _, bind := range node.Children {
		combo := niriDisplayChord(bind.Name)
		norm := niriImportNorm(combo)
		action := firstAction(bind)
		if action == nil {
			m.keepWrappedDisabled("binds", bind, "bind", "A bind without an action is preserved but disabled.")
			continue
		}
		canonical := canonicalKDLAction(action)
		semantic := canonical
		mapped := niriImportBind{Norm: norm, Combo: combo}
		desc := action.Name
		def, known := shipped.byAction[canonical]
		if !known {
			if id := importedShippedID(action); id != "" {
				def, known = shipped.byID[id]
				if known {
					semantic = def.action
				}
			}
		}
		if known {
			mapped.RebindFrom = def.chord
			desc = def.desc
			if niriImportNorm(def.chord) == norm {
				mapped.RebindFrom = ""
			}
		} else if store, ok := importedCustomBind(combo, action); ok {
			mapped.Store = store
			desc = customLabel(Keybind{Keys: combo, Action: fmt.Sprint(store["action"]), Value: fmt.Sprint(store["value"])})
		} else {
			m.keepWrappedDisabled("binds", bind, "bind", fmt.Sprintf("The action %q has no neutral keybind mapping, so the preserved bind is disabled.", action.Name))
			continue
		}
		if len(bind.Props) > 0 {
			m.loss(bind.Raw, "Bind metadata such as lock, repeat, or cooldown is not stored; the chord and action are imported.")
		}
		if owner, ok := shipped.byChord[norm]; ok && owner.action != semantic {
			mapped.Conflict = true
			mapped.ShadowsShipped = true
			c := niriImportConflict{Combo: combo, Norm: norm, Kind: "shipped"}
			c.Ryoku.Action, c.Ryoku.Desc = owner.id, owner.desc
			c.Mine.Raw, c.Mine.Desc = bind.Raw, desc
			m.conflict = append(m.conflict, c)
		} else if seen[norm] {
			mapped.Conflict = true
			c := niriImportConflict{Combo: combo, Norm: norm, Kind: "duplicate"}
			c.Ryoku.Action, c.Ryoku.Desc = "", "Earlier imported bind"
			c.Mine.Raw, c.Mine.Desc = bind.Raw, desc
			m.conflict = append(m.conflict, c)
		}
		seen[norm] = true
		m.binds = append(m.binds, mapped)
		m.items = append(m.items, niriImportItem{Kind: "bind", Raw: bind.Raw, Combo: combo, Dispatcher: action.Name, Ingestable: true})
	}
}

type importShippedBind struct {
	id, chord, action, desc string
}

type importShippedIndex struct {
	byAction map[string]importShippedBind
	byChord  map[string]importShippedBind
	byID     map[string]importShippedBind
}

func niriImportShipped() importShippedIndex {
	idx := importShippedIndex{
		byAction: map[string]importShippedBind{},
		byChord:  map[string]importShippedBind{},
		byID:     map[string]importShippedBind{},
	}
	defs := defaultBinds()
	for _, cat := range wm.ShippedBinds() {
		native := defs[cat.ID]
		if native.action == "" {
			continue
		}
		for i, chord := range cat.Expand() {
			action := native.action
			if cat.Family {
				action = strings.Replace(action, "{n}", strconv.Itoa(i+1), 1)
			}
			canonical := canonicalActionText(action)
			row := importShippedBind{id: cat.ID, chord: chord, action: canonical, desc: cat.Label}
			idx.byAction[canonical] = row
			idx.byChord[niriImportNorm(chord)] = row
			idx.byID[cat.ID] = row
		}
	}
	return idx
}

func importedShippedID(action *kdlNode) string {
	switch action.Name {
	case "close-window":
		return "window.close"
	case "fullscreen-window":
		return "window.fullscreen"
	case "toggle-window-floating":
		return "window.float"
	default:
		return ""
	}
}

func firstAction(bind *kdlNode) *kdlNode {
	for _, child := range bind.Children {
		return child
	}
	return nil
}

func canonicalActionText(action string) string {
	nodes, err := parseKDL("<action>", action+"\n")
	if err != nil || len(nodes) == 0 {
		return strings.Join(strings.Fields(action), " ")
	}
	return canonicalKDLAction(nodes[0])
}

func canonicalKDLAction(node *kdlNode) string {
	var b strings.Builder
	b.WriteString(node.Name)
	for _, arg := range node.Args {
		b.WriteByte('\x00')
		b.WriteString(arg.String())
	}
	keys := make([]string, 0, len(node.Props))
	for key := range node.Props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		b.WriteByte('\x00')
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(node.Props[key].String())
	}
	return b.String()
}

func importedCustomBind(combo string, action *kdlNode) (map[string]any, bool) {
	store := map[string]any{"keys": combo, "release": false}
	switch action.Name {
	case "spawn-sh":
		v, ok := action.Arg(0)
		if !ok {
			return nil, false
		}
		store["action"], store["value"] = "exec", v.String()
	case "spawn":
		if len(action.Args) == 0 {
			return nil, false
		}
		parts := make([]string, 0, len(action.Args))
		for _, arg := range action.Args {
			parts = append(parts, shellQuote(arg.String()))
		}
		store["action"], store["value"] = "exec", strings.Join(parts, " ")
	case "close-window":
		store["action"], store["value"] = "close", ""
	case "fullscreen-window":
		store["action"], store["value"] = "fullscreen", ""
	case "toggle-window-floating":
		store["action"], store["value"] = "togglefloating", ""
	case "focus-workspace":
		v, ok := action.Arg(0)
		if !ok {
			return nil, false
		}
		store["action"], store["value"] = "workspace", v.String()
	case "move-window-to-workspace":
		v, ok := action.Arg(0)
		if !ok {
			return nil, false
		}
		kind := "movetoworkspace"
		if focus, ok := action.Props["focus"]; ok {
			if on, yes := focus.Bool(); yes && !on {
				kind = "movetoworkspacesilent"
			}
		}
		store["action"], store["value"] = kind, v.String()
	default:
		return nil, false
	}
	return store, true
}

func (m *niriImportMapper) mapWindowRule(node *kdlNode) {
	class, title := "", ""
	var props []*kdlNode
	unknown := false
	for _, child := range node.Children {
		if child.Name == "match" {
			for key, value := range child.Props {
				switch key {
				case "app-id":
					class = value.String()
				case "title":
					title = value.String()
				default:
					unknown = true
				}
			}
			continue
		}
		props = append(props, child)
	}
	add := func(action, value string) {
		appendNested(m.desktop, map[string]any{"class": class, "title": title, "action": action, "value": value}, "windowRules")
	}
	minW, minH, maxW, maxH := 0, 0, 0, 0
	for _, prop := range props {
		v, _ := prop.Arg(0)
		switch prop.Name {
		case "open-floating":
			if on, ok := v.Bool(); ok {
				if on {
					add("float", "")
				} else {
					add("tile", "")
				}
			} else {
				unknown = true
			}
		case "open-fullscreen":
			if on, _ := v.Bool(); on {
				add("fullscreen", "")
			} else {
				unknown = true
			}
		case "open-maximized":
			if on, _ := v.Bool(); on {
				add("maximize", "")
			} else {
				unknown = true
			}
		case "opacity":
			add("opacity", v.String())
		case "open-on-workspace":
			add("workspace", v.String())
		case "geometry-corner-radius":
			if n, ok := v.Int(); ok && n == 0 {
				add("norounding", "")
			} else {
				unknown = true
			}
		case "scroll-factor":
			add("scrollfactor", v.String())
		case "tiled-state":
			if on, _ := v.Bool(); on {
				add("tiledstate", "")
			} else {
				unknown = true
			}
		case "baba-is-float":
			if on, _ := v.Bool(); on {
				add("babaisfloat", "")
			} else {
				unknown = true
			}
		case "block-out-from":
			add("blockout", v.String())
		case "min-width":
			minW, _ = v.Int()
		case "min-height":
			minH, _ = v.Int()
		case "max-width":
			maxW, _ = v.Int()
		case "max-height":
			maxH, _ = v.Int()
		case "default-column-width":
			if p := prop.Child("proportion"); p != nil {
				if x, ok := p.Arg(0); ok {
					add("columnwidth", x.String())
				}
			} else {
				unknown = true
			}
		case "border":
			if prop.Child("off") != nil {
				add("noborder", "")
			} else {
				unknown = true
			}
		case "shadow":
			if prop.Child("off") != nil {
				add("noshadow", "")
			} else {
				unknown = true
			}
		case "background-effect":
			mapped := false
			for _, effect := range prop.Children {
				switch effect.Name {
				case "blur":
					if nodeEnabled(effect) {
						add("blur", "")
					} else {
						add("noblur", "")
					}
					mapped = true
				case "xray":
					if nodeEnabled(effect) {
						add("xray", "")
					}
					mapped = true
				}
			}
			unknown = unknown || !mapped
		default:
			unknown = true
		}
	}
	if minW > 0 || minH > 0 {
		add("minsize", fmt.Sprintf("%dx%d", minW, minH))
	}
	if maxW > 0 || maxH > 0 {
		add("maxsize", fmt.Sprintf("%dx%d", maxW, maxH))
	}
	if unknown {
		m.keep(node, "windowrule", "Unsupported match or rule properties are preserved with the mapped parts.")
	} else {
		m.item(node, "windowrule", true)
	}
}

func (m *niriImportMapper) mapLayerRule(node *kdlNode) {
	rule := map[string]any{"opacity": -1.0, "cornerRadius": -1, "blur": "inherit", "shadow": "inherit", "blockOut": false, "babaIsFloat": false}
	unknown := false
	for _, child := range node.Children {
		v, _ := child.Arg(0)
		switch child.Name {
		case "match":
			if ns, ok := child.Props["namespace"]; ok && len(child.Props) == 1 {
				rule["namespace"] = ns.String()
			} else {
				unknown = true
			}
		case "opacity":
			if f, ok := v.Float(); ok {
				rule["opacity"] = f
			} else {
				unknown = true
			}
		case "geometry-corner-radius":
			if n, ok := v.Int(); ok {
				rule["cornerRadius"] = n
			} else {
				unknown = true
			}
		case "block-out-from":
			rule["blockOut"] = true
		case "baba-is-float":
			if on, ok := v.Bool(); ok {
				rule["babaIsFloat"] = on
			} else {
				unknown = true
			}
		case "shadow":
			if child.Child("on") != nil {
				rule["shadow"] = "on"
			} else if child.Child("off") != nil {
				rule["shadow"] = "off"
			} else {
				unknown = true
			}
		case "background-effect":
			if blur := child.Child("blur"); blur != nil {
				if on, ok := blur.Arg(0); ok {
					if b, yes := on.Bool(); yes && b {
						rule["blur"] = "on"
					} else {
						rule["blur"] = "off"
					}
				}
			} else {
				unknown = true
			}
		default:
			unknown = true
		}
	}
	if strings.TrimSpace(fmt.Sprint(rule["namespace"])) == "" {
		unknown = true
	} else {
		appendNested(m.niri, rule, "layerRules")
	}
	if unknown {
		m.keep(node, "layerrule", "Unsupported layer-rule fields are preserved with the mapped parts.")
	} else {
		m.item(node, "layerrule", true)
	}
}

func (m *niriImportMapper) mapHotkeyOverlay(node *kdlNode) {
	for _, child := range node.Children {
		switch child.Name {
		case "skip-at-startup":
			setNested(m.niri, nodeEnabled(child), "hotkeyOverlaySkip")
		case "hide-not-bound":
			setNested(m.niri, nodeEnabled(child), "hotkeyOverlayHideNotBound")
		default:
			m.keepWrapped(node.Name, child, "setting", fmt.Sprintf("hotkey-overlay.%s is not modelled.", child.Name))
		}
	}
	m.item(node, "setting", true)
}

func (m *niriImportMapper) mapGestures(node *kdlNode) {
	for _, child := range node.Children {
		switch child.Name {
		case "hot-corners":
			setNested(m.niri, child.Child("off") == nil, "hotCorners")
		case "dnd-edge-view-scroll":
			setChildIntProps(m.niri, child, map[string]string{"trigger-width": "dndEdgeViewScrollTriggerWidth", "delay-ms": "dndEdgeViewScrollDelayMs", "max-speed": "dndEdgeViewScrollMaxSpeed"})
		case "dnd-edge-workspace-switch":
			setChildIntProps(m.niri, child, map[string]string{"trigger-height": "dndEdgeWorkspaceTriggerHeight", "delay-ms": "dndEdgeWorkspaceDelayMs", "max-speed": "dndEdgeWorkspaceMaxSpeed"})
		default:
			m.keepWrapped(node.Name, child, "setting", fmt.Sprintf("gestures.%s is not modelled.", child.Name))
		}
	}
	m.item(node, "setting", true)
}

func (m *niriImportMapper) mapOverview(node *kdlNode) {
	for _, child := range node.Children {
		switch child.Name {
		case "zoom":
			setFirstFloat(m.niri, child, "overviewZoom")
		case "backdrop-color":
			setFirstArg(m.niri, child, "backdropColor")
		case "workspace-shadow":
			setNested(m.niri, blockOn(child), "workspaceShadow")
			setChildIntProps(m.niri, child, map[string]string{"softness": "workspaceShadowSoftness", "spread": "workspaceShadowSpread"})
			if off := child.Child("offset"); off != nil {
				if v, ok := off.Props["y"]; ok {
					if n, yes := v.Int(); yes {
						setNested(m.niri, n, "workspaceShadowOffsetY")
					}
				}
			}
			if color := child.Child("color"); color != nil {
				setFirstArg(m.niri, color, "workspaceShadowColor")
			}
		default:
			m.keepWrapped(node.Name, child, "setting", fmt.Sprintf("overview.%s is not modelled.", child.Name))
		}
	}
	m.item(node, "setting", true)
}

func (m *niriImportMapper) item(node *kdlNode, kind string, ingestable bool) {
	m.items = append(m.items, niriImportItem{Kind: kind, Raw: node.Raw, Ingestable: ingestable})
}

func (m *niriImportMapper) loss(raw, reason string) {
	m.losses = append(m.losses, niriImportLoss{Raw: raw, Reason: reason})
}

func (m *niriImportMapper) keep(node *kdlNode, kind, reason string) {
	m.item(node, kind, false)
	m.loss(node.Raw, reason)
	m.preserve = append(m.preserve, node.Raw)
}

func (m *niriImportMapper) keepWrapped(parent string, child *kdlNode, kind, reason string) {
	open := strings.TrimSpace(parent)
	if strings.Contains(open, "{") {
		parts := strings.Split(open, "{")
		var b strings.Builder
		for i, part := range parts {
			if strings.TrimSpace(part) == "" {
				continue
			}
			b.WriteString(strings.Repeat("    ", i))
			b.WriteString(strings.TrimSpace(part))
			b.WriteString(" {\n")
		}
		b.WriteString(strings.Repeat("    ", len(parts)))
		b.WriteString(strings.ReplaceAll(child.Raw, "\n", "\n"+strings.Repeat("    ", len(parts))))
		b.WriteByte('\n')
		for i := len(parts) - 1; i >= 0; i-- {
			b.WriteString(strings.Repeat("    ", i))
			b.WriteString("}\n")
		}
		m.preserve = append(m.preserve, strings.TrimSpace(b.String()))
	} else {
		m.preserve = append(m.preserve, open+" {\n    "+strings.ReplaceAll(child.Raw, "\n", "\n    ")+"\n}")
	}
	m.items = append(m.items, niriImportItem{Kind: kind, Raw: child.Raw, Ingestable: false})
	m.loss(child.Raw, reason)
}

func (m *niriImportMapper) keepWrappedDisabled(parent string, child *kdlNode, kind, reason string) {
	before := len(m.preserve)
	m.keepWrapped(parent, child, kind, reason)
	if len(m.preserve) > before {
		m.preserve[len(m.preserve)-1] = "/- " + m.preserve[len(m.preserve)-1]
	}
}

func setNested(root map[string]any, value any, path ...string) {
	if len(path) == 0 {
		return
	}
	cur := root
	for _, key := range path[:len(path)-1] {
		next, ok := cur[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[key] = next
		}
		cur = next
	}
	cur[path[len(path)-1]] = value
}

func appendNested(root map[string]any, value any, path ...string) {
	if len(path) == 0 {
		return
	}
	cur := root
	for _, key := range path[:len(path)-1] {
		next, ok := cur[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[key] = next
		}
		cur = next
	}
	leaf := path[len(path)-1]
	list, _ := cur[leaf].([]any)
	cur[leaf] = append(list, value)
}

func setFirstArg(root map[string]any, node *kdlNode, path ...string) {
	if v, ok := node.Arg(0); ok {
		setNested(root, v.String(), path...)
	}
}
func setFirstInt(root map[string]any, node *kdlNode, path ...string) {
	if v, ok := node.Arg(0); ok {
		if n, yes := v.Int(); yes {
			setNested(root, n, path...)
		}
	}
}
func setFirstFloat(root map[string]any, node *kdlNode, path ...string) {
	if v, ok := node.Arg(0); ok {
		if n, yes := v.Float(); yes {
			setNested(root, n, path...)
		}
	}
}
func setPropInt(root map[string]any, node *kdlNode, prop string, path ...string) {
	if v, ok := node.Props[prop]; ok {
		if n, yes := v.Int(); yes {
			setNested(root, n, path...)
		}
	}
}
func setPropFloat(root map[string]any, node *kdlNode, prop string, path ...string) {
	if v, ok := node.Props[prop]; ok {
		if n, yes := v.Float(); yes {
			setNested(root, n, path...)
		}
	}
}

func setChildIntProps(root map[string]any, node *kdlNode, names map[string]string) {
	for _, child := range node.Children {
		if dst, ok := names[child.Name]; ok {
			setFirstInt(root, child, dst)
		}
	}
}

func nodeEnabled(node *kdlNode) bool {
	if v, ok := node.Arg(0); ok {
		if b, yes := v.Bool(); yes {
			return b
		}
	}
	return node.Child("off") == nil
}

func blockOn(node *kdlNode) bool {
	if node.Child("off") != nil {
		return false
	}
	return true
}

func proportions(node *kdlNode) string {
	var out []string
	for _, child := range node.Children {
		if child.Name == "proportion" {
			if v, ok := child.Arg(0); ok {
				out = append(out, v.String())
			}
		}
	}
	return strings.Join(out, ", ")
}

func niriDisplayChord(chord string) string {
	parts := strings.Split(chord, "+")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		switch strings.ToLower(part) {
		case "mod", "super":
			parts[i] = "SUPER"
		case "ctrl", "control":
			parts[i] = "CTRL"
		case "alt":
			parts[i] = "ALT"
		case "shift":
			parts[i] = "SHIFT"
		default:
			parts[i] = part
		}
	}
	return strings.Join(parts, " + ")
}

func niriImportNorm(combo string) string {
	var parts []string
	for _, part := range strings.Split(combo, "+") {
		part = strings.ToLower(strings.TrimSpace(part))
		switch part {
		case "mod", "meta", "win":
			part = "super"
		case "control":
			part = "ctrl"
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "+")
}

func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`;&|()<>*?[]{}!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
