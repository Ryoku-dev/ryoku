package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// plugin.go is the `ryoku plugin` CLI: install a shell plugin from a git repo
// or a local folder, remove it, list what is installed, or validate a local
// plugin tree (export and share live in plugin_share.go). Install stages a copy,
// validates the manifest against the shared rules (relative entry points, a
// known host set, no symlinks, an id that is neither a reserved built-in widget
// nor already installed), then installs it through ryostore's transaction into
// ~/.local/share/ryoku/plugins/<id>. It never executes anything from the plugin.

// knownHosts is the set a manifest may declare (docs/plugins.md). A manifest
// naming any other host is rejected.
var knownHosts = map[string]bool{"framePopout": true, "desktopWidget": true, "topbarGlyph": true, "sidebarCard": true}

// pluginIDRe is the id grammar the shell's discover.sh enforces: lowercase
// alphanumerics and dashes, not starting with a dash.
var pluginIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// builtinWidgetIDs is the reserved built-in widget id set (contract 1). It is
// the fallback when the catalogue file cannot be found, so a manifest can never
// claim a built-in id even before the catalogue ships.
var builtinWidgetIDs = []string{
	"launcher", "workspaces", "status", "memory", "cpu", "volume", "ai", "clock",
	"media", "quick", "network", "battery", "brightness", "power", "bluetooth",
	"cputemp", "gpu", "storage", "layout",
}

// manifest is the validated subset of a plugin manifest the CLI reasons about.
type manifest struct {
	ID      string
	Name    string
	Version string
	Hosts   []string
}

// rawManifest mirrors the on-disk manifest.json shape the CLI reads.
type rawManifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Hosts       []string `json:"hosts"`
	EntryPoints struct {
		Main    string `json:"main"`
		Content string `json:"content"`
	} `json:"entryPoints"`
}

func cmdPlugin(args []string) error {
	if len(args) < 1 {
		return pluginUsage()
	}
	switch args[0] {
	case "new":
		return cmdPluginNew(args[1:])
	case "add":
		return cmdPluginAdd(args[1:])
	case "remove":
		return cmdPluginRemove(args[1:])
	case "list":
		return cmdPluginList(args[1:])
	case "validate":
		return cmdPluginValidate(args[1:])
	case "export":
		return cmdPluginExport(args[1:])
	case "share":
		return cmdPluginShare(args[1:])
	case "-h", "--help", "help":
		return pluginUsage()
	}
	return fmt.Errorf(i18n.T("unknown plugin command %q (use: new, add, remove, list, validate, export, share)"), args[0])
}

func pluginUsage() error {
	fmt.Print(i18n.T("Usage: ryoku plugin <command>\n\n  new <id> [--bar|--desktop|--popout|--sidebar] [--name N] [--author \"N <m>\"] [--to <dir>]\n                                     scaffold a new plugin folder and git-init it\n                                     (--bar is the default host; adds a panel)\n  add <git-url|dir> [--bar] [--yes] [--allow-findings]\n                                     fetch, validate, audit, and install a plugin;\n                                     --bar puts it on the QS Bar;\n                                     --allow-findings installs despite blocking audit findings\n  remove <id>                        uninstall a plugin and drop its placement\n  list [--json]                      installed plugins (--json adds capabilities)\n  validate <dir> [--allow <rule,..>] validate manifest and run the R1-R11 audit\n  export <id> [--to <path>]          write a deterministic .tar.gz\n  share <id>                         export and open the Ryostore PR flow\n"))
	return nil
}

// pluginsInstallRoot is where `add` installs: ~/.local/share/ryoku/plugins.
func pluginsInstallRoot() string {
	return filepath.Join(sys.Xdg("XDG_DATA_HOME", ".local/share"), "ryoku", "plugins")
}

// catalogPath resolves the widget catalogue: the packaged base (or
// RYOKU_CONFIG_BASE) first, then a dev checkout via ResolveRepo.
func catalogPath() string {
	rel := filepath.Join("quickshell", "shell", "modules", "bar", "barstyles", "qsbar", "core", "widgets.json")
	if p := filepath.Join(sys.BaseConfigDir(), rel); sys.Exists(p) {
		return p
	}
	if repo := sys.ResolveRepo(); repo != "" {
		if p := filepath.Join(repo, "ryoku", "shell", rel); sys.Exists(p) {
			return p
		}
	}
	return filepath.Join(sys.BaseConfigDir(), rel)
}

// reservedIDs is the set of ids a plugin may not claim: the built-in widgets
// from the catalogue, or the fixed built-in set when the catalogue is absent.
func reservedIDs() map[string]bool {
	out := map[string]bool{}
	if b, err := os.ReadFile(catalogPath()); err == nil {
		var cat []struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(b, &cat) == nil {
			for _, w := range cat {
				if w.ID != "" {
					out[w.ID] = true
				}
			}
		}
	}
	if len(out) == 0 {
		for _, id := range builtinWidgetIDs {
			out[id] = true
		}
	}
	return out
}

// placeTool resolves ryoku-plugins-place: the checkout copy, else the packaged
// name on PATH.
func placeTool() string {
	if repo := sys.ResolveRepo(); repo != "" {
		p := filepath.Join(repo, "ryoku", "shell", "quickshell", "plugins", "ryoku-plugins-place")
		if sys.Exists(p) {
			return p
		}
	}
	return "ryoku-plugins-place"
}

// safeRel reports whether p is a relative path that stays inside its root: no
// absolute paths, no "" and no ".." escape.
func safeRel(p string) bool {
	if p == "" || filepath.IsAbs(p) {
		return false
	}
	clean := filepath.Clean(p)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(os.PathSeparator))
}

// validateManifest reads and checks <dir>/manifest.json against the install
// rules, returning the validated fields. It never runs plugin code.
func validateManifest(dir string, reserved map[string]bool) (manifest, error) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return manifest{}, fmt.Errorf(i18n.T("no manifest.json in %s"), dir)
	}
	var rm rawManifest
	if err := json.Unmarshal(b, &rm); err != nil {
		return manifest{}, fmt.Errorf(i18n.T("manifest.json is not valid JSON: %v"), err)
	}
	if !pluginIDRe.MatchString(rm.ID) {
		return manifest{}, fmt.Errorf(i18n.T("manifest id %q must be lowercase letters, digits and dashes"), rm.ID)
	}
	if reserved[rm.ID] {
		return manifest{}, fmt.Errorf(i18n.T("%q is a reserved built-in widget id"), rm.ID)
	}
	if strings.TrimSpace(rm.Name) == "" {
		return manifest{}, fmt.Errorf(i18n.T("manifest is missing a name"))
	}
	if strings.TrimSpace(rm.Version) == "" {
		return manifest{}, fmt.Errorf(i18n.T("manifest is missing a version"))
	}
	if len(rm.Hosts) == 0 {
		return manifest{}, fmt.Errorf(i18n.T("manifest declares no hosts"))
	}
	for _, h := range rm.Hosts {
		if !knownHosts[h] {
			return manifest{}, fmt.Errorf(i18n.T("unknown host %q (allowed: framePopout, desktopWidget, topbarGlyph, sidebarCard)"), h)
		}
	}
	for label, p := range map[string]string{"entryPoints.main": rm.EntryPoints.Main, "entryPoints.content": rm.EntryPoints.Content} {
		if !safeRel(p) {
			return manifest{}, fmt.Errorf(i18n.T("%s %q must be a relative path with no .."), label, p)
		}
		if !sys.Exists(filepath.Join(dir, p)) {
			return manifest{}, fmt.Errorf(i18n.T("%s %q does not exist in the plugin"), label, p)
		}
	}
	if link, err := firstSymlink(dir); err != nil {
		return manifest{}, err
	} else if link != "" {
		return manifest{}, fmt.Errorf(i18n.T("plugin contains a symlink (%s); symlinks are not allowed"), link)
	}
	return manifest{ID: rm.ID, Name: rm.Name, Version: rm.Version, Hosts: rm.Hosts}, nil
}

// firstSymlink returns the relative path of the first symlink under dir, or ""
// when there is none.
func firstSymlink(dir string) (string, error) {
	found := ""
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			rel, _ := filepath.Rel(dir, path)
			found = rel
			return filepath.SkipAll
		}
		return nil
	})
	return found, err
}

func cmdPluginAdd(args []string) error {
	url := ""
	bar, yes, allowFindings := false, false, false
	for _, a := range args {
		switch {
		case a == "--bar":
			bar = true
		case a == "--yes" || a == "-y":
			yes = true
		case a == "--allow-findings":
			allowFindings = true
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf(i18n.T("unknown flag %q"), a)
		default:
			if url != "" {
				return fmt.Errorf(i18n.T("give one git url"))
			}
			url = a
		}
	}
	if url == "" {
		return fmt.Errorf(i18n.T("usage: ryoku plugin add <git-url|dir> [--bar] [--yes] [--allow-findings]"))
	}
	// A local folder (a widget written on this desktop, by hand or by an agent)
	// is copied; anything else is a git URL and is cloned.
	local := sys.Exists(filepath.Join(url, "manifest.json"))
	if !local && !sys.Has("git") {
		return fmt.Errorf(i18n.T("git is required to add a plugin from a URL"))
	}

	fmt.Println(sys.Amber(i18n.T("Warning: a plugin runs unsandboxed inside your shell, with your")))
	fmt.Println(sys.Amber(i18n.T("permissions. Only install plugins you trust.")))
	verb := i18n.T("Clone")
	if local {
		verb = i18n.T("Copy")
	}
	if !yes && !confirm(fmt.Sprintf(i18n.T("%s and install %s?"), verb, url)) {
		return fmt.Errorf(i18n.T("aborted"))
	}

	// Stage a copy in a temp dir; ryostore reads the bytes from here and installs
	// them through its supply-chain transaction (receipt + content-hashed view +
	// journal), which is what the shell's discover.sh requires to load it.
	staging, err := os.MkdirTemp("", "ryoku-plugin-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	src := filepath.Join(staging, "src")
	if local {
		if err := copyPluginTree(url, src); err != nil {
			return fmt.Errorf(i18n.T("copy failed: %w"), err)
		}
	} else {
		if err := sys.Run("git", "clone", "--depth", "1", url, src); err != nil {
			return fmt.Errorf(i18n.T("clone failed: %w"), err)
		}
		// Drop the git metadata: it is not part of the plugin and can carry symlinks.
		_ = os.RemoveAll(filepath.Join(src, ".git"))
	}

	m, err := validateManifest(src, reservedIDs())
	if err != nil {
		return err
	}

	// The same static audit `validate` runs. Blocking findings refuse the install
	// unless --allow-findings; warnings are printed and never block.
	res := auditPlugin(src, nil)
	if len(res.Blocking) > 0 || len(res.Warnings) > 0 {
		printAudit(res, false)
	}
	if len(res.Blocking) > 0 && !allowFindings {
		return fmt.Errorf(i18n.T("%d blocking audit finding(s); fix them or re-run with --allow-findings"), len(res.Blocking))
	}
	if sys.Exists(filepath.Join(pluginsInstallRoot(), m.ID)) || pluginHasReceipt(m.ID) {
		return fmt.Errorf(i18n.T("plugin %q is already installed (remove it first)"), m.ID)
	}
	if err := sys.Run(storeTool(), "install", "plugins", m.ID, "--from", src); err != nil {
		return fmt.Errorf(i18n.T("install %q: %w"), m.ID, err)
	}
	fmt.Printf("%s %s (%s)\n", sys.Green(i18n.T("installed")), m.Name, m.Version)

	if bar {
		if err := sys.Run(placeTool(), m.ID, "enabled", "true"); err != nil {
			return fmt.Errorf(i18n.T("enable on bar: %w"), err)
		}
		if err := sys.Run(placeTool(), m.ID, "host", "topbarGlyph"); err != nil {
			return fmt.Errorf(i18n.T("place on bar: %w"), err)
		}
		fmt.Println(sys.Green(i18n.T("enabled")) + i18n.T(" on the bar"))
	}
	return nil
}

func cmdPluginRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf(i18n.T("usage: ryoku plugin remove <id>"))
	}
	id := args[0]
	if !pluginHasReceipt(id) && !sys.Exists(filepath.Join(pluginsInstallRoot(), id)) {
		return fmt.Errorf(i18n.T("plugin %q is not installed"), id)
	}
	// Remove THROUGH ryostore so the receipt, the content-hashed view, and the
	// index entry are torn down together.
	if err := sys.Run(storeTool(), "remove", "plugins", id); err != nil {
		return fmt.Errorf(i18n.T("remove %q: %w"), id, err)
	}
	// Drop its placement so the shell stops loading it (best-effort).
	_ = sys.Run(placeTool(), id, "forget")
	fmt.Printf("%s %s\n", sys.Green(i18n.T("removed")), id)
	return nil
}

func cmdPluginValidate(args []string) error {
	dir := ""
	asJSON := false
	allow := map[string]bool{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			asJSON = true
		case a == "--allow":
			i++
			if i >= len(args) {
				return fmt.Errorf(i18n.T("--allow needs a rule list"))
			}
			addAllowed(allow, args[i])
		case strings.HasPrefix(a, "--allow="):
			addAllowed(allow, strings.TrimPrefix(a, "--allow="))
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf(i18n.T("unknown flag %q"), a)
		default:
			if dir != "" {
				return fmt.Errorf(i18n.T("give one directory"))
			}
			dir = a
		}
	}
	if dir == "" {
		return fmt.Errorf(i18n.T("usage: ryoku plugin validate <dir> [--json] [--allow <rule>,...]"))
	}
	_, mErr := validateManifest(dir, reservedIDs())
	res := auditPlugin(dir, allow)
	printAudit(res, asJSON)
	if mErr != nil {
		return mErr
	}
	if len(res.Blocking) > 0 {
		return fmt.Errorf(i18n.T("%d blocking finding(s); fix them or re-run with --allow"), len(res.Blocking))
	}
	return nil
}

// addAllowed splits a comma-separated rule list into the allow set.
func addAllowed(allow map[string]bool, list string) {
	for _, r := range strings.Split(list, ",") {
		if r = strings.TrimSpace(r); r != "" {
			allow[r] = true
		}
	}
}

// pluginRow is one `plugin list` element. source is "store" for a receipt-owned
// install and "dev" for a RYOSTORE_PLUGINS_DIR override.
type pluginRow struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Hosts        []string       `json:"hosts"`
	Dir          string         `json:"dir"`
	Enabled      bool           `json:"enabled"`
	Host         string         `json:"host"`
	Source       string         `json:"source"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
}

func cmdPluginList(args []string) error {
	asJSON := false
	for _, a := range args {
		if a == "--json" {
			asJSON = true
		}
	}
	rows := pluginListRows()
	if asJSON {
		b, _ := json.Marshal(rows)
		fmt.Println(string(b))
		return nil
	}
	if len(rows) == 0 {
		fmt.Println(i18n.T("no plugins installed"))
		return nil
	}
	for _, r := range rows {
		state := i18n.T("off")
		if r.Enabled {
			state = i18n.T("on:") + r.Host
		}
		fmt.Printf("%-20s %-10s %-6s %-12s %s\n", r.ID, r.Version, r.Source, state, strings.Join(r.Hosts, ","))
	}
	return nil
}

// pluginListRows merges the store's receipt-owned installs with any dev override
// dirs and the placement in plugins.json. Dev overrides shadow the store, as the
// shell's discover.sh loads them, so a duplicate id lists once, marked "dev".
func pluginListRows() []pluginRow {
	state := readPluginsState()
	rows := []pluginRow{}
	seen := map[string]bool{}
	add := func(id, dir, version, source string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		m := readManifestMap(filepath.Join(dir, "manifest.json"))
		if version == "" {
			version = manifestString(m, "version")
		}
		st := state[id]
		caps, _ := m["capabilities"].(map[string]any)
		rows = append(rows, pluginRow{
			ID: id, Name: manifestName(m, id), Version: version,
			Hosts: manifestHosts(m), Dir: dir,
			Enabled: st.enabled, Host: st.host, Source: source,
			Capabilities: caps,
		})
	}
	for _, dir := range devPluginDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pdir := filepath.Join(dir, e.Name())
			id := manifestString(readManifestMap(filepath.Join(pdir, "manifest.json")), "id")
			add(id, pdir, "", "dev")
		}
	}
	entries, err := os.ReadDir(storeReceiptDir())
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".json")
			add(id, filepath.Join(pluginsInstallRoot(), id), receiptVersion(id), "store")
		}
	}
	return rows
}

// devPluginDirs are the RYOSTORE_PLUGINS_DIR (legacy RYOKU_PLUGINS_DIR) override
// dirs, colon-separated, mirroring discover.sh.
func devPluginDirs() []string {
	var dirs []string
	pd := os.Getenv("RYOSTORE_PLUGINS_DIR")
	if pd == "" {
		pd = os.Getenv("RYOKU_PLUGINS_DIR")
	}
	if pd != "" {
		for _, d := range strings.Split(pd, ":") {
			if d != "" {
				dirs = append(dirs, d)
			}
		}
	}
	return dirs
}

// storeTool is the ryostore backend, installed on PATH by deploy.sh / the
// package.
func storeTool() string { return "ryostore" }

// storeReceiptDir is where the store writes plugin receipts.
func storeReceiptDir() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "store", "plugins")
}

func pluginHasReceipt(id string) bool {
	return sys.Exists(filepath.Join(storeReceiptDir(), id+".json"))
}

func receiptVersion(id string) string {
	b, err := os.ReadFile(filepath.Join(storeReceiptDir(), id+".json"))
	if err != nil {
		return ""
	}
	var r struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(b, &r) != nil {
		return ""
	}
	return r.Version
}

func readManifestMap(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return map[string]any{}
	}
	return m
}

type pluginState struct {
	enabled bool
	host    string
}

func readPluginsState() map[string]pluginState {
	out := map[string]pluginState{}
	b, err := os.ReadFile(filepath.Join(sys.ConfigHome(), "ryoku", "plugins.json"))
	if err != nil {
		return out
	}
	var raw map[string]any
	if json.Unmarshal(b, &raw) != nil {
		return out
	}
	for id, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		st := pluginState{}
		if e, ok := m["enabled"].(bool); ok {
			st.enabled = e
		}
		if h, ok := m["host"].(string); ok {
			st.host = h
		}
		out[id] = st
	}
	return out
}

func manifestString(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func manifestName(m map[string]any, id string) string {
	if n := manifestString(m, "name"); n != "" {
		return n
	}
	return id
}

func manifestHosts(m map[string]any) []string {
	arr, ok := m["hosts"].([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// confirm asks a y/N question, declining when stdin is not a terminal.
func confirm(prompt string) bool {
	if !sys.StdinIsTTY() {
		return false
	}
	fmt.Printf(i18n.T("%s [y/N] "), prompt)
	var resp string
	_, _ = fmt.Scanln(&resp)
	resp = strings.ToLower(strings.TrimSpace(resp))
	return resp == "y" || resp == "yes"
}
