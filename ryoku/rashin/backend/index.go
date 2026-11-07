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
	{"Packages", "native package database", "ryoku-host", "ryoku update"},
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
	b.WriteString("Ryoku Hub page, the wallpaper/theme picker, or the active bar style's settings;\n")
	b.WriteString("see the GUI map below), name that path first with its keybind, then give the\n")
	b.WriteString("command as the headless fallback. You still act through commands (`ryoku`,\n")
	b.WriteString("`ryoku-shell`, `ryoku-hub`, `ryogami`, `ryoku wm act`), never by editing\n")
	b.WriteString("shipped files. The\n")
	b.WriteString("`ryoku` skill is the contract for that: `SKILL.md` (rules and the command\n")
	b.WriteString("catalogue), `gui.md` (the GUI map), `bar.md` (bar styles and the dock), `plugins.md`\n")
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

// guiMapBody renders the GUI-first map: the wallpaper/theme picker, the bar
// style switch and its settings, the app launcher, and every routinely-used
// Ryoku Hub page with its deep link, so an agent names the GUI path before the
// command behind it.
func guiMapBody() string {
	var b strings.Builder
	b.WriteString("## GUI map\n\n")
	b.WriteString("Most desktop changes have a GUI surface; name it first, then the command as\n")
	b.WriteString("the headless fallback. Compositor-gated pages are hidden when the running\n")
	b.WriteString("window manager cannot back them, so a row here may not appear on every box.\n\n")
	b.WriteString("| Want to change | Open | Deep link | Command behind it |\n|---|---|---|---|\n")
	b.WriteString("| Wallpaper or theme | Wallpaper picker (Super+W), or the bar Wallpaper widget | - | `ryogami wallpaper ui` |\n")
	b.WriteString("| Subshell or bar style | Ryoku Hub: Bar Studio | `ryoku-shell hub open bar-studio` | `ryoku-shell barstyle <id>` |\n")
	b.WriteString("| QS Bar layout (while QS Bar is active) | QS Bar Settings | - | `ryoku-shell ipc call qsbar settings \"\"` |\n")
	b.WriteString("| Dock | Stage Editor (Super+Shift+D), Dock section | `ryoku-shell ipc call desktop editSection dock \"\"` | `ryoku-shell dock list` |\n")
	b.WriteString("| Launch an app | App launcher (Super+Space) | - | - |\n")
	for _, s := range guiSections {
		fmt.Fprintf(&b, "| Ryoku Hub: %s | Super+comma | `ryoku-shell hub open %s` | - |\n", s.name, s.key)
	}
	b.WriteString("\nIdle and power timeouts live under Graphics & Power (`gpu`).\n\n")
	return b.String()
}

// ryokuPackages are queried for installed versions in desktop.md.
var ryokuPackages = []string{"ryoku-shell", "ryoku-hub", "ryoku", "ryoku-blobs", "ryoku-desktop", "ryoku-rashin"}

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
		if v := packageVersion(pkg); v != "" {
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

func packagesBody() string {
	var b strings.Builder
	if out, ok := probe(5, "ryoku-host", "pkg", "count"); ok {
		fmt.Fprintf(&b, "## Total installed\n\n%s packages\n\n", strings.TrimSpace(out))
	} else {
		b.WriteString("## Total installed\n\n(native package database unavailable)\n\n")
	}
	if out, ok := probe(8, "ryoku", "status", "--json"); ok {
		var status struct {
			SystemUpdates int `json:"systemUpdates"`
		}
		if json.Unmarshal([]byte(out), &status) == nil {
			fmt.Fprintf(&b, "## Pending system updates\n\n%d packages\n", status.SystemUpdates)
		}
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

func packageVersion(pkg string) string {
	out, ok := probe(5, "ryoku-host", "pkg", "version", pkg)
	if !ok {
		return ""
	}
	return strings.TrimSpace(out)
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
