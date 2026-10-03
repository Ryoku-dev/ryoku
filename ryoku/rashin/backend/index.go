package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	wm "ryoku-wm"
)

// Reindex regenerates the machine-owned vault docs. Every probe is best
// effort, so a missing tool degrades one section rather than failing the index.
func Reindex() error {
	if err := EnsureVault(); err != nil {
		return err
	}
	docs := []struct {
		name, header, body string
	}{
		{"system.md", systemHeader, systemBody()},
		{"desktop.md", desktopHeader, desktopBody()},
		{"packages.md", packagesHeader, packagesBody()},
	}
	for _, d := range docs {
		if err := writeVaultDoc(d.name, d.header, d.body); err != nil {
			return err
		}
	}
	if err := writeRepoVaultDoc(); err != nil {
		return err
	}
	if err := writeUserVaultDoc(); err != nil {
		return err
	}
	if err := WriteHabits(); err != nil {
		return err
	}
	if err := writeOwnershipVaultDoc(); err != nil {
		return err
	}
	if err := writeLogsMapVaultDoc(); err != nil {
		return err
	}
	// Best effort, bounded, and never fatal: mirror the live config under the
	// vault and refresh its prowl index so search_code works off a checkout.
	_ = RefreshSourceMirror()
	return nil
}

// ReindexUser refreshes only the user-owned changes layer; cheap enough for
// the watcher to run whenever the live config drifts.
func ReindexUser() error {
	if err := EnsureVault(); err != nil {
		return err
	}
	return writeUserVaultDoc()
}

const systemHeader = "# System\n" +
	"\n" +
	"Hardware, kernel, GPU, disks, and displays. Generated: the content between the\n" +
	"markers is overwritten on every reindex."

const desktopHeader = "# Ryoku desktop map\n" +
	"\n" +
	"Where every subsystem's config lives, the binary that owns it, and how to\n" +
	"reload it. Read this before searching the filesystem. Generated: the content\n" +
	"between the markers is overwritten on every reindex."

const packagesHeader = "# Packages\n" +
	"\n" +
	"Installed package counts, the explicit set, and pending updates. Generated:\n" +
	"the content between the markers is overwritten on every reindex."

// writeVaultDoc lays down a generated doc as header + fenced body on first
// write, and thereafter rewrites only the fenced body so a user's out-of-fence
// edits (including the header) survive.
func writeVaultDoc(name, header, body string) error {
	path := filepath.Join(VaultDir(), name)
	existing := readFileOrEmpty(path)
	var doc string
	if existing == "" {
		doc = header + "\n\n" + buildFence(body) + "\n"
	} else {
		doc = ReplaceFenced(existing, body)
	}
	return atomicWrite(path, []byte(doc), 0o644)
}

func systemBody() string {
	var b strings.Builder
	model, cores := cpuModelCores()
	total, _ := memTotalUsed()

	b.WriteString("## CPU\n\n")
	fmt.Fprintf(&b, "- Model: %s\n", orUnknown(model))
	fmt.Fprintf(&b, "- Logical cores: %s\n\n", orUnknownInt(cores))

	b.WriteString("## Memory\n\n")
	if total > 0 {
		fmt.Fprintf(&b, "- Total: %s\n\n", humanBytes(total))
	} else {
		b.WriteString("- Total: unknown\n\n")
	}

	b.WriteString("## Kernel\n\n")
	fmt.Fprintf(&b, "- Release: %s\n\n", orUnknown(kernelRelease()))

	b.WriteString("## GPU\n\n")
	if gpus := gpuDescribe(); len(gpus) > 0 {
		for _, g := range gpus {
			fmt.Fprintf(&b, "- %s\n", g)
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- (none detected)\n\n")
	}

	b.WriteString("## Displays\n\n")
	if mons := monitorRows(); len(mons) > 0 {
		for _, m := range mons {
			fmt.Fprintf(&b, "- %s\n", m)
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- (none detected)\n\n")
	}

	b.WriteString("## Disks\n\n")
	if rows := diskRows(); len(rows) > 0 {
		b.WriteString("| Device | Mount | Size | Used |\n|---|---|---|---|\n")
		for _, r := range rows {
			b.WriteString(r + "\n")
		}
	} else {
		b.WriteString("(no block devices detected)\n")
	}
	return b.String()
}

// desktopMapRow is one verified line of the Ryoku map.
type desktopMapRow struct {
	subsystem, path, owner, reload string
}

// desktopMap is distilled from docs/structure.md, docs/cli.md, the hub and shell
// READMEs, and the shell IPC surface. Every path, owner, and reload command is
// verified against the repo. The shell exposes no per-component restart verb, so
// shell surfaces reload via `ryoku reload`.
var desktopMap = []desktopMapRow{
	{"Shell surfaces (pill, sidebar, ryoshot, widgets, launcher, hub)", "~/.config/quickshell/", "ryoku-shell daemon", "ryoku reload"},
	{"Terminal", "~/.config/kitty/", "kitty", "relaunch kitty"},
	{"Shell + prompt", "~/.config/{fish,bash,zsh}/, ~/.config/starship.toml", "fish, bash, or zsh; starship", "open a new shell"},
	{"Editor", "~/.config/nvim/", "Neovim (LazyVim)", "relaunch nvim"},
	{"File manager", "~/.config/yazi/", "yazi", "relaunch yazi"},
	{"Ryoku CLI", "binary `ryoku`; state ~/.local/state/ryoku/", "ryoku", "ryoku update | rollback | status | materialize | reload"},
	{"Hub state", "~/.config/ryoku/hub.toml", "ryoku-hub (Ryoku Settings)", "written by Ryoku Settings; no reload"},
	{"Theme + colour source", "~/.config/ryoku/theme.json", "ryoku-hub", "ryogami wallpaper repaint"},
	{"Palette", "~/.cache/ryoku/colors.json", "ryoku-shell daemon (matugen)", "ryogami wallpaper repaint"},
	{"Packages", "pacman + yay database", "pacman, yay", "ryoku update"},
	{"System vault", "~/.local/share/ryoku/rashin/", "ryoku-rashin", "ryoku-rashin index"},
}

// actingBody is the first thing desktop.md says: how an agent changes this
// desktop. It names the shipped `ryoku` skill with its resolved path, so an
// agent that only followed the vault pointer (and never loaded the skill by
// itself) still reaches the plugin and command contracts instead of reading
// shell source.
func actingBody() string {
	var b strings.Builder
	b.WriteString("## Acting on this desktop\n\n")
	b.WriteString("Answer a \"how do I\" question GUI-first: when the change has a GUI path (a\n")
	b.WriteString("Ryoku Hub page, the wallpaper/theme picker, or QS Bar Settings; see the GUI\n")
	b.WriteString("map below), name that path first with its keybind, then give the command as\n")
	b.WriteString("the headless fallback. You still act through commands (`ryoku`, `ryoku-shell`,\n")
	b.WriteString("`ryoku-hub`, `ryogami`, `ryoku wm act`), never by editing shipped files. The\n")
	b.WriteString("`ryoku` skill is the contract for that: `SKILL.md` (rules and the command\n")
	b.WriteString("catalogue), `gui.md` (the GUI map), `bar.md` (the QS Bar model), `plugins.md`\n")
	b.WriteString("(how a new widget is written and installed), `feature.md` (the ladder:\n")
	b.WriteString("check the store, then the machine, then build).\n")
	if d := skillSourceDir(); d != "" {
		fmt.Fprintf(&b, "Read them at `%s/` (also linked into each agent's skills dir as `ryoku`).\n", d)
	} else {
		b.WriteString("It is linked into each agent's skills dir as `ryoku` by `ryoku-rashin wire`.\n")
	}
	b.WriteString("\n")
	return b.String()
}

// guiSections is the Ryoku Hub page list the GUI map advertises, in rail order.
// Only pages a user routinely reaches for are listed; each is deep-linked with
// `ryoku-shell hub open <key>`. Compositor-gated pages are hidden at runtime
// when the active window manager cannot back them, so a listed row may not
// appear on a given box.
var guiSections = []struct{ name, key string }{
	{"General", "global"},
	{"Updates", "updates"},
	{"Displays", "displays"},
	{"Connections", "connections"},
	{"Input", "input"},
	{"Graphics & Power", "gpu"},
	{"Animations", "animations"},
	{"Lockscreen", "lockscreen"},
	{"Window Manager", "windowmanager"},
	{"Plugins", "plugins"},
	{"Bar Studio", "bar-studio"},
	{"Desktop", "desktop"},
	{"Widgets", "widgets"},
	{"App Launcher", "launcher"},
	{"Keybinds", "keybinds"},
	{"Performance", "performance"},
	{"Session", "session"},
	{"Recording", "recording"},
	{"Dictation", "dictation"},
	{"Add-ons", "addons"},
	{"Rashin", "rashin"},
}

// guiMapBody renders the GUI-first map: the wallpaper/theme picker, QS Bar
// Settings, the app launcher, and every routinely-used Ryoku Hub page with its
// deep link, so an agent names the GUI path before the command behind it.
func guiMapBody() string {
	var b strings.Builder
	b.WriteString("## GUI map\n\n")
	b.WriteString("Most desktop changes have a GUI surface; name it first, then the command as\n")
	b.WriteString("the headless fallback. Compositor-gated pages are hidden when the running\n")
	b.WriteString("window manager cannot back them, so a row here may not appear on every box.\n\n")
	b.WriteString("| Want to change | Open | Deep link | Command behind it |\n|---|---|---|---|\n")
	b.WriteString("| Wallpaper or theme | Wallpaper picker (Super+W), or the bar Wallpaper widget | - | `ryogami wallpaper ui` |\n")
	b.WriteString("| Bar or dock layout | QS Bar Settings | - | `ryoku-shell bar settings` |\n")
	b.WriteString("| Launch an app | App launcher (Super+Space) | - | - |\n")
	for _, s := range guiSections {
		fmt.Fprintf(&b, "| Ryoku Hub: %s | Super+comma | `ryoku-shell hub open %s` | - |\n", s.name, s.key)
	}
	b.WriteString("\nIdle and power timeouts live under Graphics & Power (`gpu`).\n\n")
	return b.String()
}

// ryokuPackages are queried for installed versions in desktop.md.
var ryokuPackages = []string{"ryoku-shell", "ryoku-hub", "ryoku", "ryoku-blobs", "ryoku-sidebarfx", "ryoku-desktop", "ryoku-rashin"}

func desktopBody() string {
	var b strings.Builder
	b.WriteString(actingBody())
	b.WriteString(guiMapBody())
	b.WriteString("## Subsystem map\n\n")
	b.WriteString("| Subsystem | Config path | Owner | Reload |\n|---|---|---|---|\n")
	if d := wm.Detect(); d.Name != "" {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			d.Name+" (window manager)",
			mdCell("~/.config/"+wm.ConfigDir(d.Name)+"/"),
			d.Name+" config",
			mdCell("ryoku wm act config.reload"))
	} else {
		b.WriteString("| Window manager | (no provider installed) | - | - |\n")
	}
	for _, r := range desktopMap {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.subsystem, mdCell(r.path), r.owner, mdCell(r.reload))
	}
	b.WriteString("\n## Ryoku package versions\n\n")
	any := false
	for _, pkg := range ryokuPackages {
		if v := pacmanVersion(pkg); v != "" {
			fmt.Fprintf(&b, "- %s %s\n", pkg, v)
			any = true
		} else {
			fmt.Fprintf(&b, "- %s (not installed)\n", pkg)
		}
	}
	if !any {
		b.WriteString("\n(no Ryoku packages installed; likely a dev checkout)\n")
	}
	b.WriteString(barSectionBody())
	return b.String()
}

// ---- the QS Bar catalogue ---------------------------------------------------

// barWidget mirrors one entry of the qsbar widget catalogue (widgets.json) and
// the plugin rows `ryoku-shell bar catalog --json` adds. Only id and label are
// guaranteed; a reader tolerates a widget that omits visKey or settings.
type barWidget struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	VisKey   string       `json:"visKey"`
	Kind     string       `json:"kind"`
	Hosts    []string     `json:"hosts"`
	Settings []barSetting `json:"settings"`
}

type barSetting struct {
	Key string `json:"key"`
}

// barCatalogFile resolves the shipped widgets.json: the live config first, then
// the packaged base, then the dev checkout the deploy recorded. Returns "" when
// none exists.
func barCatalogFile() string {
	const rel = "quickshell/shell/modules/bar/barstyles/qsbar/core/widgets.json"
	cands := []string{
		filepath.Join(configHome(), rel),
		filepath.Join("/usr/share/ryoku/config", rel),
	}
	if repo := recordedCheckout(); repo != "" {
		cands = append(cands, filepath.Join(repo, "ryoku", "shell", rel))
	}
	for _, p := range cands {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// loadBuiltinWidgets reads the built-in widget catalogue. It stays the source of
// the visibility key the daemon's JSON omits, so built-ins always come from the
// file. Returns (nil, "") when the file is absent or unparseable.
func loadBuiltinWidgets() ([]barWidget, string) {
	path := barCatalogFile()
	if path == "" {
		return nil, ""
	}
	var ws []barWidget
	if json.Unmarshal([]byte(readFileOrEmpty(path)), &ws) != nil {
		return nil, ""
	}
	return ws, path
}

// loadPluginWidgets asks the daemon for installed bar-capable plugins. Without a
// running daemon (or its bar CLI) it returns nil, so the section degrades to
// built-ins only. The query is read-only.
func loadPluginWidgets() []barWidget {
	out, ok := probe(5, "ryoku-shell", "bar", "catalog", "--json")
	if !ok {
		return nil
	}
	var all []barWidget
	if json.Unmarshal([]byte(out), &all) != nil {
		return nil
	}
	var plugins []barWidget
	for _, w := range all {
		if w.Kind == "plugin" {
			plugins = append(plugins, w)
		}
	}
	return plugins
}

// settingKeys joins a catalogue widget's setting keys for the id table.
func settingKeys(rows []barSetting) string {
	keys := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Key != "" {
			keys = append(keys, r.Key)
		}
	}
	if len(keys) == 0 {
		return "(none)"
	}
	return strings.Join(keys, ", ")
}

// barSectionBody renders the desktop map's fenced "Bar and dock" section:
// built-ins from widgets.json (authoritative for the visibility key) and, when
// the daemon answers, the installed bar plugins. It is fenced with its own
// markers so the guide can be located and regenerated on its own.
func barSectionBody() string {
	var b strings.Builder
	b.WriteString("\n## Bar and dock\n\n")
	b.WriteString(vaultBarFenceBegin + "\n")
	b.WriteString("The default bar is QS Bar (`qsbar`). Its layout is data in\n")
	b.WriteString("`~/.config/ryoku/shell.json` under `qsbar.layout`:\n\n")
	b.WriteString("```json\n")
	b.WriteString("\"qsbar\": { \"layout\": { \"version\": 1,\n")
	b.WriteString("  \"left\": [ids], \"center\": [ids], \"right\": [ids] } }\n")
	b.WriteString("```\n\n")
	b.WriteString("An entry is a widget id (a built-in below, or an installed plugin's id).\n")
	b.WriteString("Placement and visibility are separate: a built-in renders when\n")
	b.WriteString("`qsbar.widgets[<key>]` is true (a blank key is always shown); a plugin when\n")
	b.WriteString("`~/.config/ryoku/plugins.json` has it `enabled` with `host: \"topbarGlyph\"`.\n")
	b.WriteString("The shell daemon is the only writer of `shell.json`; change the bar with\n")
	b.WriteString("`ryoku-shell bar ...`, never by editing the file.\n\n")

	builtins, src := loadBuiltinWidgets()
	b.WriteString("### Built-in widgets\n\n")
	if len(builtins) == 0 {
		b.WriteString("(widget catalogue not found; deploy or install the shell to populate it)\n\n")
	} else {
		b.WriteString("| id | label | visibility key | settings keys |\n|---|---|---|---|\n")
		for _, w := range builtins {
			vis := w.VisKey
			if vis == "" {
				vis = "(always)"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", w.ID, mdCell(w.Label), mdCell(vis), mdCell(settingKeys(w.Settings)))
		}
		fmt.Fprintf(&b, "\nSource: `%s`.\n\n", tildeAbbrev(src))
	}

	if plugins := loadPluginWidgets(); len(plugins) > 0 {
		b.WriteString("### Installed bar plugins\n\n")
		b.WriteString("| id | label | hosts | settings keys |\n|---|---|---|---|\n")
		for _, w := range plugins {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", w.ID, mdCell(w.Label), mdCell(strings.Join(w.Hosts, ", ")), mdCell(settingKeys(w.Settings)))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Commands\n\n")
	b.WriteString("```\n")
	b.WriteString("bar list [--json] | catalog [--json]\n")
	b.WriteString("bar move <id> --section left|center|right [--index N | --before <id> | --after <id>]\n")
	b.WriteString("bar show <id> | hide <id> | set <id> <key> <value>\n")
	b.WriteString("bar position top|bottom | form full|fit|dock|notch|islands | defaults | settings [route]\n")
	b.WriteString("dock show|hide | edge auto|top|bottom|left|right | autohide on|off | pin <app>|unpin <app> | list [--json]\n")
	b.WriteString("```\n\n")
	b.WriteString("Run these through `ryoku-shell` (e.g. `ryoku-shell bar move clock --section right`).\n")
	b.WriteString("Installed plugins live in `~/.local/share/ryoku/plugins/<id>/`; their bar\n")
	b.WriteString("placement and settings persist in `~/.config/ryoku/plugins.json`.\n\n")
	b.WriteString("### Adding a widget\n\n")
	b.WriteString("A widget that is not in the table above is a plugin, never an edit to the\n")
	b.WriteString("shipped QML under `~/.config/quickshell/` or `/usr/share/ryoku/`. Read the\n")
	b.WriteString("`ryoku` skill's `plugins.md` for the contract (manifest.json, service/Main.qml,\n")
	b.WriteString("content/Widget.qml, host `topbarGlyph`), write it in its own directory, then:\n\n")
	b.WriteString("```\n")
	b.WriteString("ryoku plugin validate <dir>\n")
	b.WriteString("ryoku plugin add <dir-or-git-url> --bar --yes\n")
	b.WriteString("```\n\n")
	b.WriteString("It lands on the bar and under QS Bar Settings > Community (`ryoku-shell bar\n")
	b.WriteString("settings community`); `ryoku plugin remove <id>` takes it off again, and\n")
	b.WriteString("`ryoku plugin share <id>` (only when the user asks to publish) opens the\n")
	b.WriteString("Ryostore pull request that lists it for everyone.\n")
	b.WriteString(vaultBarFenceEnd + "\n")
	return b.String()
}

func packagesBody() string {
	var b strings.Builder
	if out, ok := probe(5, "pacman", "-Qq"); ok {
		fmt.Fprintf(&b, "## Total installed\n\n%d packages\n\n", len(nonEmptyLines(out)))
	} else {
		b.WriteString("## Total installed\n\n(pacman unavailable)\n\n")
	}
	if out, ok := probe(5, "pacman", "-Qqe"); ok {
		lines := nonEmptyLines(out)
		b.WriteString("## Explicitly installed\n\n")
		fmt.Fprintf(&b, "<details><summary>%d explicit packages</summary>\n\n", len(lines))
		for _, l := range lines {
			fmt.Fprintf(&b, "- %s\n", l)
		}
		b.WriteString("\n</details>\n\n")
	}
	// checkupdates exits non-zero (2) when nothing is pending; skip the section
	// on any error or a missing tool rather than reporting a misleading zero.
	if out, ok := probe(5, "checkupdates"); ok {
		fmt.Fprintf(&b, "## Pending updates\n\n%d packages\n", len(nonEmptyLines(out)))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func gpuDescribe() []string {
	if _, err := exec.LookPath("ryoku-gpu-detect"); err == nil {
		if out, ok := probe(5, "ryoku-gpu-detect"); ok {
			if lines := nonEmptyLines(out); len(lines) > 0 {
				return lines
			}
		}
	}
	out, ok := probe(5, "lspci")
	if !ok {
		return nil
	}
	var gpus []string
	for _, l := range nonEmptyLines(out) {
		ll := strings.ToLower(l)
		if strings.Contains(ll, "vga compatible controller") ||
			strings.Contains(ll, "3d controller") ||
			strings.Contains(ll, "display controller") {
			gpus = append(gpus, strings.TrimSpace(l))
		}
	}
	return gpus
}

func monitorRows() []string {
	outs := monitorOutputs()
	if len(outs) == 0 {
		return nil
	}
	var rows []string
	for _, o := range outs {
		rows = append(rows, fmt.Sprintf("%s: %dx%d, scale %g", o.Name, o.Width, o.Height, o.Scale))
	}
	return rows
}

// lsblkNode mirrors `lsblk -J` with nullable string fields, tolerating any that
// the running lsblk omits.
type lsblkNode struct {
	Name       string      `json:"name"`
	Mountpoint *string     `json:"mountpoint"`
	Size       *string     `json:"size"`
	Fsused     *string     `json:"fsused"`
	Children   []lsblkNode `json:"children"`
}

func diskRows() []string {
	out, ok := probe(5, "lsblk", "-J", "-o", "NAME,MOUNTPOINT,SIZE,FSUSED")
	if !ok {
		return nil
	}
	var doc struct {
		BlockDevices []lsblkNode `json:"blockdevices"`
	}
	if json.Unmarshal([]byte(out), &doc) != nil {
		return nil
	}
	var rows []string
	var walk func(n lsblkNode)
	walk = func(n lsblkNode) {
		if n.Mountpoint != nil && *n.Mountpoint != "" {
			rows = append(rows, fmt.Sprintf("| %s | %s | %s | %s |",
				n.Name, *n.Mountpoint, orDash(n.Size), orDash(n.Fsused)))
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, d := range doc.BlockDevices {
		walk(d)
	}
	return rows
}

func pacmanVersion(pkg string) string {
	out, ok := probe(5, "pacman", "-Q", pkg)
	if !ok {
		return ""
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) >= 2 {
		return fields[1]
	}
	return ""
}

// probe runs a command with a timeout, returning its stdout and whether it
// succeeded. It is the single best-effort exec path for the indexer.
func probe(seconds int, name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func orUnknownInt(n int) string {
	if n <= 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d", n)
}

func orDash(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}

// mdCell escapes the pipe so a path or command never breaks the table.
func mdCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
