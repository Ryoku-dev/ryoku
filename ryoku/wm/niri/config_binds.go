package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// The niri binds block. niri has no unbind directive and rejects a duplicate
// chord inside one block, so rebinds.kdl cannot subtract from a seeded block the
// way the Hyprland split does: it must be the sole, total source of binds. This
// file translates the shipped Ryoku bind catalogue (ryoku/wm/binds.go) into
// niri's own action vocabulary and folds the store's rebinds, unbinds and custom
// binds into it, resolving every chord collision to exactly one winner (custom
// over rebound default over static default) so the emitted config never carries
// a chord twice. binds reports the full effective legend the cheatsheet and Hub
// draw; apply writes the resolved KDL.

// niriBind is how niri expresses one catalogue entry: the niri action string, or
// a reason when niri has no action for the behaviour (which lists the bind as an
// unhonored legend row rather than dropping it). label and hint replace the
// catalogue copy when niri's mechanic differs from the neutral description. In a
// family action, {n} is the workspace number, substituted per expanded chord.
type niriBind struct {
	action     string
	reason     string // set when action == ""; the Unhonored copy
	label      string // replaces the catalogue Label when niri's mechanic differs
	hint       string // replaces the catalogue Hint when niri's mechanic differs
	noRepeat   bool   // repeat=false
	cooldownMs int    // cooldown-ms=N, for wheel binds
}

func spawnArgs(args ...string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, "spawn")
	for _, a := range args {
		parts = append(parts, kdlStr(a))
	}
	return strings.Join(parts, " ")
}

func spawnSh(cmd string) string { return "spawn-sh " + kdlStr(cmd) }

// defaultBinds maps each catalogue id to niri's expression of it. Compositor
// behaviours become niri actions; app and shell launches spawn the same commands
// (shell surfaces go through ryoku-shell, whose openSurface bus is compositor
// agnostic); niri's own overview takes the workspace overview. Behaviours niri
// cannot perform (pin, the scratchpad, mouse-button drag, desktop blocks) carry
// a reason so the legend reports them instead of hiding them. An id absent here
// is left off entirely, which would be a bug, so the tests assert full coverage.
func defaultBinds() map[string]niriBind {
	return map[string]niriBind{
		// Windows
		// The iRiS frame can ask before a window closes; only when that is on and
		// iRiS is the bar style does the close go through the shell.
		"window.close":      {action: spawnSh(wm.IrisCloseCheck + " && qs -c shell ipc call closeConfirm trigger || niri msg action close-window"), noRepeat: true},
		"window.fullscreen": {action: "fullscreen-window"},
		// The catalogue names this "Float or tile", which is niri's mechanic too.
		"window.float": {action: "toggle-window-floating"},
		"window.pin":   {reason: "niri has no pin-window action."},
		// The catalogue's resize row is Hyprland's keyboard submap; niri has no
		// such mode. niri's answer to resizing is stepping the column through its
		// preset widths, so the chord drives that and the copy is rewritten to
		// match, which is also what makes the preset-widths setting reachable.
		"window.resize":        {action: "switch-preset-column-width", label: "Step column width", hint: "Cycle the column through its preset widths"},
		"window.presetHeight":  {action: "switch-preset-window-height"},
		"column.tabbed":        {action: "toggle-column-tabbed-display"},
		"column.maximize":      {action: "maximize-column"},
		"column.center":        {action: "center-column"},
		"window.focusPrevious": {action: "focus-window-previous"},

		// Focus
		"focus.left":   {action: "focus-column-left"},
		"focus.right":  {action: "focus-column-right"},
		"focus.up":     {action: "focus-window-up"},
		"focus.down":   {action: "focus-window-down"},
		"column.first": {action: "focus-column-first"},
		"column.last":  {action: "focus-column-last"},

		// Move
		"move.left":         {action: "move-column-left"},
		"move.right":        {action: "move-column-right"},
		"move.up":           {action: "move-window-up"},
		"move.down":         {action: "move-window-down"},
		"column.mergeLeft":  {action: "consume-or-expel-window-left"},
		"column.mergeRight": {action: "consume-or-expel-window-right"},

		// Resize
		"resize.narrower":    {action: `set-column-width "-10%"`},
		"resize.wider":       {action: `set-column-width "+10%"`},
		"resize.shorter":     {action: `set-window-height "-10%"`},
		"resize.taller":      {action: `set-window-height "+10%"`},
		"resize.resetHeight": {action: "reset-window-height"},

		// Workspaces. The families carry {n}, the workspace number, substituted
		// per expanded chord. niri references workspaces by flat index, so
		// Super+Alt+N follows the window there and Super+Shift+N sends it quietly.
		"workspace.focus":                   {action: "focus-workspace {n}"},
		"workspace.moveWindow":              {action: "move-window-to-workspace {n}"},
		"workspace.moveWindowSilent":        {action: "move-window-to-workspace {n} focus=false"},
		"workspace.focus.numpad":            {action: "focus-workspace {n}"},
		"workspace.moveWindow.numpad":       {action: "move-window-to-workspace {n}"},
		"workspace.moveWindowSilent.numpad": {action: "move-window-to-workspace {n} focus=false"},
		"workspace.prev":                    {action: "focus-workspace-up"},
		"workspace.next":                    {action: "focus-workspace-down"},
		"workspace.prevWheel":               {action: "focus-workspace-up", cooldownMs: 150},
		"workspace.nextWheel":               {action: "focus-workspace-down", cooldownMs: 150},
		"workspace.moveWindowPrev":          {action: "move-column-to-workspace-up"},
		"workspace.moveWindowNext":          {action: "move-column-to-workspace-down"},
		"workspace.reorderUp":               {action: "move-workspace-up"},
		"workspace.reorderDown":             {action: "move-workspace-down"},
		"workspace.hideWindow":              {reason: "niri has no scratchpad workspace."},
		"workspace.scratchpad":              {reason: "niri has no scratchpad workspace."},
		"workspace.overview":                {action: "toggle-overview"},
		"workspace.overviewDesktops":        {reason: "niri has no desktop blocks to step through, so this would only open the same overview as SUPER + Tab."},

		// Displays
		"display.focus.left":          {action: "focus-monitor-left"},
		"display.focus.right":         {action: "focus-monitor-right"},
		"display.focus.up":            {action: "focus-monitor-up"},
		"display.focus.down":          {action: "focus-monitor-down"},
		"display.moveWindow.left":     {action: "move-column-to-monitor-left"},
		"display.moveWindow.right":    {action: "move-column-to-monitor-right"},
		"display.moveWindow.up":       {action: "move-column-to-monitor-up"},
		"display.moveWindow.down":     {action: "move-column-to-monitor-down"},
		"display.moveWorkspace.left":  {action: "move-workspace-to-monitor-left"},
		"display.moveWorkspace.right": {action: "move-workspace-to-monitor-right"},
		"display.moveWorkspace.up":    {action: "move-workspace-to-monitor-up"},
		"display.moveWorkspace.down":  {action: "move-workspace-to-monitor-down"},
		"display.cycle":               {action: spawnArgs("ryoku-wm-niri", "act", "output.cycle")},

		// Apps
		"app.terminal": {action: spawnArgs("ryoku-app", "terminal")},
		"app.files":    {action: spawnArgs("ryoku-app", "files")},
		"app.browser":  {action: spawnArgs("ryoku-app", "browser")},
		"app.editor":   {action: spawnArgs("ryoku-app", "editor")},
		"app.notes":    {action: spawnArgs("ryoku-app", "notes")},
		"app.yazi":     {action: spawnArgs("kitty", "-e", "yazi")},
		"app.ryotunes": {action: spawnArgs("ryotunes")},

		// Shell
		"shell.launcher":          {action: spawnArgs("ryoku-shell", "launcher")},
		"shell.ask":               {action: spawnArgs("ryoku-shell", "ask")},
		"shell.rashin":            {action: spawnSh("ryoku-summon Rashin flock -n -o /tmp/rashin-app.lock rashin-app")},
		"shell.cheatsheet":        {action: spawnSh("pkill -x -f 'qs -c keys' 2>/dev/null || " + wm.QmlEnv + " flock -n -o /tmp/ryoku-keys.lock qs -c keys")},
		"shell.lock":              {action: spawnArgs("ryoku-shell", "lock")},
		"shell.quicksettings":     {action: spawnArgs("ryoku-shell", "quicksettings")},
		"shell.wallpaper":         {action: spawnArgs("ryogami", "wallpaper", "ui")},
		"shell.wallpaperRandom":   {action: spawnArgs("ryogami", "wallpaper", "random")},
		"shell.ryovm":             {action: spawnSh("ryoku-summon ryovm " + wm.QmlEnv + " flock -n -o /tmp/ryovm.lock qs -c ryovm")},
		"shell.clipboard":         {action: spawnArgs("ryoku-shell", "clipboard")},
		"shell.visualizer":        {action: spawnArgs("ryoku-shell", "visualizer")},
		"shell.visualizerOverlay": {action: spawnArgs("ryoku-shell", "visualizer-overlay")},
		"shell.visualizerPlace":   {action: spawnArgs("ryoku-shell", "visualizer-place")},
		"shell.voice":             {action: spawnArgs("ryoku-shell", "voice")},
		"shell.settings":          {action: spawnArgs("ryoku-shell", "hub", "open")},
		"shell.screenshot":        {action: spawnSh(wm.QmlEnv + " flock -n -o /tmp/ryoshot.lock qs -c ryoshot")},
		"shell.screenshotPrint":   {action: spawnSh(wm.QmlEnv + " flock -n -o /tmp/ryoshot.lock qs -c ryoshot")},
		"shell.screenshotMonitor": {action: spawnSh(wm.QmlEnv + " flock -n -o /tmp/ryoshot.lock env RYOSHOT_MODE=monitor qs -c ryoshot")},
		"shell.colorPicker":       {action: spawnArgs("hyprpicker", "-a")},
		"shell.restartAudio":      {action: spawnArgs("ryoku-restart-audio")},
		"shell.inhibitShortcuts":  {action: "toggle-keyboard-shortcuts-inhibit"},

		// Media. Locked in the catalogue, so genBinds emits allow-when-locked.
		"media.volumeUp":   {action: spawnArgs("ryoku-volume", "up")},
		"media.volumeDown": {action: spawnArgs("ryoku-volume", "down")},
		"media.mute":       {action: spawnArgs("wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle")},
		"media.play":       {action: spawnArgs("playerctl", "play-pause")},
		"media.next":       {action: spawnArgs("playerctl", "next")},
		"media.prev":       {action: spawnArgs("playerctl", "previous")},

		// Hardware
		"hardware.brightnessUp":   {action: spawnArgs("ryoku-cmd-brightness", "+5")},
		"hardware.brightnessDown": {action: spawnArgs("ryoku-cmd-brightness", "-5")},
		"hardware.touchpadToggle": {action: spawnArgs("ryoku-wm-niri", "act", "input.touchpad", "toggle")},
		"hardware.touchpadOn":     {action: spawnArgs("ryoku-wm-niri", "act", "input.touchpad", "on")},
		"hardware.touchpadOff":    {action: spawnArgs("ryoku-wm-niri", "act", "input.touchpad", "off")},

		// Mouse
		"mouse.move":   {reason: "niri moves windows with Mod and drag natively."},
		"mouse.resize": {reason: "niri resizes windows with Mod and drag natively."},
	}
}

// outBind is one resolved bind, ready for the config writer: the niri chord and
// action apply writes plus the KDL properties niri's parser reads off the node.
type outBind struct {
	chord, action    string
	locked, noRepeat bool
	cooldownMs       int
}

// resolveBinds expands the catalogue into the emitted bind set and folds the
// store's custom binds, rebinds and unbinds into it, resolving every niri chord
// to one winner: a custom bind beats a rebound default beats a static default.
// A family expands to its ten chords; every resolved chord on a number-pad digit
// (a family, a rebind onto the keypad, or a custom bind) also emits its
// NumLock-off twin, so the keypad works either way. It is the single source
// genBinds writes, so the emitted config never carries a chord twice. report
// names each behaviour niri could not honour.
func resolveBinds(s niriStore) ([]outBind, []wm.Unhonored) {
	unbind := map[string]bool{}
	for _, c := range s.Unbinds {
		if c = strings.TrimSpace(c); c != "" {
			unbind[c] = true
		}
	}

	defs := defaultBinds()
	var report []wm.Unhonored
	claimed := map[string]bool{}
	var out []outBind
	// The number pad works whichever way NumLock sits: a resolved bind on a
	// KP_<digit> chord also wants its NumLock-off twin. Twins are held back so
	// every explicit chord claims first, then a twin never displaces a bind a
	// user set on that keysym; among twins the earlier (higher priority)
	// resolution wins, matching the one-winner rule.
	type pendingTwin struct {
		hubChord   string
		action     string
		locked     bool
		noRepeat   bool
		cooldownMs int
	}
	var twins []pendingTwin
	emit := func(hubChord, niriChord, action string, locked, noRepeat bool, cooldownMs int) {
		if niriChord == "" || claimed[niriChord] {
			return
		}
		claimed[niriChord] = true
		out = append(out, outBind{
			chord: niriChord, action: action,
			locked: locked, noRepeat: noRepeat, cooldownMs: cooldownMs,
		})
		if len(wm.NumpadAliases(hubChord)) > 0 {
			twins = append(twins, pendingTwin{hubChord, action, locked, noRepeat, cooldownMs})
		}
	}

	// Priority 1: user custom binds win every chord they take.
	for i, k := range s.Keybinds {
		action, reason := customAction(k)
		if action == "" {
			if reason != "" {
				report = append(report, wm.Unhonored{Key: fmt.Sprintf("desktop.keybinds[%d]", i), Reason: reason})
			}
			continue
		}
		niriChord, ok := toNiriChord(k.Keys)
		if !ok {
			report = append(report, wm.Unhonored{Key: fmt.Sprintf("desktop.keybinds[%d]", i), Reason: fmt.Sprintf("niri cannot bind the chord %q.", k.Keys)})
			continue
		}
		emit(k.Keys, niriChord, action, false, false, 0)
	}

	cat := wm.ShippedBinds()

	// Behaviours niri cannot perform: report each once, unless the user removed it.
	for _, cb := range cat {
		nb := defs[cb.ID]
		if nb.action == "" && nb.reason != "" && !unbind[cb.Chord] {
			report = append(report, wm.Unhonored{Key: fmt.Sprintf("desktop.keybinds (default %s)", cb.Chord), Reason: nb.reason})
		}
	}

	// Priority 2 then 3: rebound defaults claim before static ones, so a rebind
	// onto another default's chord wins and the static default is dropped.
	claimDefaults := func(rebound bool) {
		for _, cb := range cat {
			nb := defs[cb.ID]
			if nb.action == "" {
				continue
			}
			// A family resolves through its family-level rebind first, so one
			// stored entry moves all ten members; the digit stays and only the
			// modifier set changes. A per-member legacy rebind (keyed on the
			// shipped concrete chord) still wins over the family one below.
			famChord, famRebound := cb.Chord, false
			if cb.Family {
				famChord, famRebound = wm.FamilyRebind(cb.Chord, s.KeybindRebinds)
			}
			for idx, chord := range cb.Expand() {
				action := nb.action
				if cb.Family {
					action = strings.Replace(action, "{n}", strconv.Itoa(idx+1), 1)
				}
				if unbind[chord] {
					continue
				}
				to, isRebound := effectiveChord(chord, s.KeybindRebinds)
				if !isRebound && famRebound {
					to = wm.ExpandChord(famChord, idx+1)
					isRebound = true
				}
				if isRebound != rebound {
					continue
				}
				if niriChord, ok := toNiriChord(to); ok {
					emit(to, niriChord, action, cb.Locked, nb.noRepeat, nb.cooldownMs)
				}
			}
		}
	}
	claimDefaults(true)
	claimDefaults(false)

	// Every explicit chord is claimed now, so lay down the NumLock-off twin of
	// each resolved number-pad digit. A twin has no twin of its own, so this pass
	// records nothing further and cannot displace a bind already emitted.
	for _, t := range twins {
		for _, alias := range wm.NumpadAliases(t.hubChord) {
			if aliasChord, ok := toNiriChord(alias); ok {
				emit(alias, aliasChord, t.action, t.locked, t.noRepeat, t.cooldownMs)
			}
		}
	}

	return out, report
}

// genBinds renders rebinds.kdl and returns the binds it could not honour. The
// block is always well formed, empty body included, because a missing include is
// a hard config error that would cost the user their session.
func genBinds(s niriStore) (string, []wm.Unhonored) {
	out, report := resolveBinds(s)

	var b strings.Builder
	b.WriteString("binds {\n")
	for _, o := range out {
		var props []string
		if o.locked {
			props = append(props, "allow-when-locked=true")
		}
		if o.noRepeat {
			props = append(props, "repeat=false")
		}
		if o.cooldownMs > 0 {
			props = append(props, fmt.Sprintf("cooldown-ms=%d", o.cooldownMs))
		}
		head := o.chord
		if len(props) > 0 {
			head += " " + strings.Join(props, " ")
		}
		fmt.Fprintf(&b, "    %s { %s; }\n", head, o.action)
	}
	b.WriteString("}\n")
	return b.String(), report
}

// bindRows is the full effective legend the binds verb prints: every catalogue
// row in catalogue order (its chord resolved against the user's rebinds, its
// copy overridden where niri's mechanic differs, and marked unhonored where niri
// cannot perform it), then niri's own binds outside the catalogue, then the
// user's custom binds. One list, so the cheatsheet and the Hub read the legend
// from the seam rather than parsing niri's config.
func bindRows(s niriStore) []wm.BindRow {
	defs := defaultBinds()
	rows := make([]wm.BindRow, 0, 128)

	for _, cb := range wm.ShippedBinds() {
		nb := defs[cb.ID]
		label, hint := cb.Label, cb.Hint
		if nb.label != "" {
			label = nb.label
		}
		if nb.hint != "" {
			hint = nb.hint
		}
		// A family keeps its {n} and resolves through its family-level rebind, so
		// the row carries the effective {n} chord and DisplayKeys renders the range.
		// A plain bind takes the user's rebind when set.
		eff, _ := effectiveChord(cb.Chord, s.KeybindRebinds)
		if cb.Family {
			eff, _ = wm.FamilyRebind(cb.Chord, s.KeybindRebinds)
		}
		row := wm.BindRow{
			ID:         cb.ID,
			Category:   cb.Category,
			Label:      label,
			Hint:       hint,
			Keys:       wm.DisplayKeys(eff),
			Default:    cb.Chord,
			Chord:      eff,
			Kind:       cb.Kind,
			Rebindable: rebindable(cb),
			Locked:     cb.Locked,
		}
		if nb.action == "" {
			row.Unhonored = nb.reason
		}
		rows = append(rows, row)
	}

	// niri exclusives would follow here, titled with the compositor's name. Every
	// niri behaviour Ryoku binds now carries a catalogue id, so there are none;
	// the section stays a home for a future niri-only bind.

	// Custom binds from desktop.keybinds, in store order. A degenerate bind with
	// no command is dropped, mirroring resolveBinds; the rest list, marked
	// unhonored when niri cannot express the action or the chord.
	for i, k := range s.Keybinds {
		action, reason := customAction(k)
		if action == "" && reason == "" {
			continue
		}
		chord := strings.TrimSpace(k.Keys)
		row := wm.BindRow{
			ID:         fmt.Sprintf("custom.%d", i),
			Category:   "Custom",
			Label:      customLabel(k),
			Keys:       wm.DisplayKeys(chord),
			Default:    chord,
			Chord:      chord,
			Kind:       wm.BindCustom,
			Rebindable: false,
		}
		switch {
		case action == "":
			row.Unhonored = reason
		default:
			if _, ok := toNiriChord(chord); !ok {
				row.Unhonored = fmt.Sprintf("niri cannot bind the chord %q.", k.Keys)
			}
		}
		rows = append(rows, row)
	}

	return rows
}

// rebindable reports whether the Hub may let a user record a new chord over this
// bind. A workspace family is rebindable as a unit: the Hub records one chord and
// the store keeps the {n} placeholder, so all ten members move together. A media
// or hardware chord rides a dedicated key, so it stays fixed.
func rebindable(cb wm.CatalogBind) bool {
	for _, tok := range strings.Split(cb.Chord, " + ") {
		if strings.HasPrefix(tok, "mouse") || strings.HasPrefix(tok, "XF86") {
			return false
		}
	}
	return true
}

// runBinds prints the effective bind legend as a JSON array of wm.BindRow, read
// through the store so the chords reflect what the session actually emits.
func runBinds(args []string) error {
	storePath := ""
	if len(args) > 0 {
		storePath = args[0]
	}
	rows := bindRows(loadStore(storePath))
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// effectiveChord resolves a default's emitted chord: the user's rebind when set,
// otherwise the shipped chord. The bool reports whether a rebind applied.
func effectiveChord(def string, rebinds map[string]string) (string, bool) {
	if v, ok := rebinds[def]; ok {
		if t := strings.TrimSpace(v); t != "" {
			return t, true
		}
	}
	return def, false
}

// customAction maps a store keybind action onto its niri action. exec runs the
// value through the shell; the window actions take none; the workspace actions
// take a flat index, which niri references directly. An unknown action (a submap
// or a plugin toggle from another compositor) yields a reason so the bind is
// reported, not dropped. An empty exec is degenerate and dropped silently.
func customAction(k Keybind) (action, reason string) {
	switch k.Action {
	case "exec", "":
		if strings.TrimSpace(k.Value) == "" {
			return "", ""
		}
		return spawnSh(k.Value), ""
	case "close":
		return "close-window", ""
	case "fullscreen":
		return "fullscreen-window", ""
	case "togglefloating":
		return "toggle-window-floating", ""
	case "workspace":
		if n, ok := workspaceIndex(k.Value); ok {
			return fmt.Sprintf("focus-workspace %d", n), ""
		}
		return "", fmt.Sprintf("niri needs a workspace number, not %q.", k.Value)
	case "movetoworkspace":
		if n, ok := workspaceIndex(k.Value); ok {
			return fmt.Sprintf("move-window-to-workspace %d", n), ""
		}
		return "", fmt.Sprintf("niri needs a workspace number, not %q.", k.Value)
	case "movetoworkspacesilent":
		if n, ok := workspaceIndex(k.Value); ok {
			return fmt.Sprintf("move-window-to-workspace %d focus=false", n), ""
		}
		return "", fmt.Sprintf("niri needs a workspace number, not %q.", k.Value)
	}
	return "", fmt.Sprintf("niri has no bind action for %q.", k.Action)
}

// customLabel is the human text the legend shows for a store keybind, derived
// from its action so the row reads like the shipped legend rather than a raw
// dispatcher name. It stands independent of customAction: a bind niri cannot
// honour still reads its intent, with the reason carried in Unhonored.
func customLabel(k Keybind) string {
	switch k.Action {
	case "exec", "":
		if v := strings.TrimSpace(k.Value); v != "" {
			return "Run: " + v
		}
		return "Run command"
	case "close":
		return "Close window"
	case "fullscreen":
		return "Fullscreen"
	case "togglefloating":
		return "Float or tile"
	case "workspace":
		return "Focus workspace " + strings.TrimSpace(k.Value)
	case "movetoworkspace":
		return "Send to workspace " + strings.TrimSpace(k.Value)
	case "movetoworkspacesilent":
		return "Send to workspace " + strings.TrimSpace(k.Value) + " quietly"
	}
	return k.Action
}

// workspaceIndex parses a custom bind's workspace value as a positive flat index,
// the only shape niri's focus-workspace and move-window-to-workspace accept here.
func workspaceIndex(v string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// toNiriChord rewrites a Hub display chord ("SUPER + SHIFT + Left") as a niri
// chord ("Super+Shift+Left"). ok is false for a chord niri cannot bind, such as a
// pointer button, so the caller reports it rather than emitting a broken line.
func toNiriChord(chord string) (string, bool) {
	parts := strings.Split(chord, "+")
	if len(parts) == 0 {
		return "", false
	}
	tokens := make([]string, 0, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return "", false
		}
		if i < len(parts)-1 {
			m, ok := niriMod(p)
			if !ok {
				return "", false
			}
			tokens = append(tokens, m)
			continue
		}
		k, ok := niriKey(p)
		if !ok {
			return "", false
		}
		tokens = append(tokens, k)
	}
	return strings.Join(tokens, "+"), true
}

func niriMod(tok string) (string, bool) {
	switch strings.ToUpper(tok) {
	case "SUPER", "SUPERKEY", "MOD", "WIN", "META", "LOGO":
		return "Super", true
	case "CTRL", "CONTROL":
		return "Ctrl", true
	case "ALT":
		return "Alt", true
	case "SHIFT":
		return "Shift", true
	}
	return "", false
}

// niriKey normalises a key token to its XKB name. A lone letter is upper-cased;
// the scroll pseudo-keys become niri's wheel names; the page-up/down catalogue
// names take their XKB spelling. A number-pad keysym, whether the NumLock-on
// digit (KP_1..KP_0) or its NumLock-off twin (KP_End..KP_Insert and the arrows),
// and the punctuation names niri's xkb parser accepts pass through unchanged. A
// pointer button has no key name, so it fails.
func niriKey(tok string) (string, bool) {
	switch tok {
	case "mouse_up":
		return "WheelScrollUp", true
	case "mouse_down":
		return "WheelScrollDown", true
	case "Prior":
		return "Page_Up", true
	case "Next":
		return "Page_Down", true
	}
	if strings.HasPrefix(tok, "mouse:") || strings.HasPrefix(tok, "mouse") {
		return "", false
	}
	if len(tok) == 1 {
		r := tok[0]
		if r >= 'a' && r <= 'z' {
			return strings.ToUpper(tok), true
		}
	}
	return tok, true
}
