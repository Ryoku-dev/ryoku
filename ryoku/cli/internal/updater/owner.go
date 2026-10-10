package updater

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// `ryoku owner <path>` answers the one question a coding agent (or a curious
// user) has before touching a config file: who writes this, and where does my
// change belong so an update does not eat it. It reuses the materialize
// contract wholesale (the manifest, the seeds, the overlay, forkable, and the
// window-manager seam) so its verdict always matches what an update actually
// does, rather than a second copy of the rules that could drift.

// ownership is one path's verdict. edit is the path an agent should actually
// write to: the same path for a file it may edit in place, an overlay fork or a
// user-override file for a shipped one, or "" when the change has to go through
// a command or the GUI.
type ownership struct {
	Path   string `json:"path"`
	Class  string `json:"class"`
	Writer string `json:"writer"`
	Advice string `json:"advice"`
	Edit   string `json:"edit"`
}

// shellUserOverrides are the dedicated user files Ryoku reads but never ships,
// outside a provider's own tree: the terminal and shell escape hatches.
var shellUserOverrides = []string{
	"kitty/user.conf",
	"fish/user.fish",
	"bash/user.bash",
	"zsh/user.zsh",
}

// shellOverrideForTree maps a Ryoku-owned tree to the user file that shadows it,
// so a change to a shipped file in that tree is pointed at the file the update
// never overwrites instead of the overlay.
var shellOverrideForTree = map[string]string{
	"kitty": "kitty/user.conf",
	"fish":  "fish/user.fish",
	"bash":  "bash/user.bash",
	"zsh":   "zsh/user.zsh",
}

// ownerEnv is the machine's ownership facts, gathered once so classifying many
// paths does not re-read the manifest or re-ask the seam each time.
type ownerEnv struct {
	configHome      string
	editsDir        string
	base            string
	baseExists      bool
	manifest        map[string]bool   // rels the last materialize recorded as shipped
	generated       map[string]bool   // provider-authored config, minus its overlay copies
	userOverride    map[string]bool   // rels that are dedicated user-override files
	overrideForTree map[string]string // top-level tree -> the user file that shadows it
}

func newOwnerEnv() ownerEnv {
	e := ownerEnv{
		configHome:      sys.ConfigHome(),
		editsDir:        sys.UserEditsDir(),
		base:            sys.BaseConfigDir(),
		manifest:        map[string]bool{},
		generated:       map[string]bool{},
		userOverride:    map[string]bool{},
		overrideForTree: map[string]string{},
	}
	if info, err := os.Stat(e.base); err == nil && info.IsDir() {
		e.baseExists = true
	}
	for _, rel := range readManifest(materializeStatePath()) {
		e.manifest[rel] = true
	}
	for _, name := range wm.Providers() {
		for _, rel := range wm.GeneratedConfig(name) {
			// The overlay copies the provider keeps are forks, not generated
			// state a user should edit through the Hub; only the live generated
			// files count here.
			if strings.HasPrefix(rel, "ryoku/user_edits/") {
				continue
			}
			e.generated[rel] = true
		}
		for _, rel := range wm.ConfigUserOwned(name) {
			e.userOverride[rel] = true
		}
		if dir := wm.ConfigDir(name); dir != "" {
			if files := wm.ConfigFiles(name); len(files) > 0 {
				e.overrideForTree[dir] = files[0]
			}
		}
	}
	for _, rel := range shellUserOverrides {
		e.userOverride[rel] = true
	}
	for dir, rel := range shellOverrideForTree {
		e.overrideForTree[dir] = rel
	}
	return e
}

// resolve turns a user argument into the absolute path it names. Absolute and
// ~/... paths are taken as given; a bare relative path is read against ~/.config,
// where the overwhelming majority of the paths a user asks about live.
func (e ownerEnv) resolve(arg string) string {
	switch {
	case arg == "~":
		return sys.Home()
	case strings.HasPrefix(arg, "~/"):
		return filepath.Join(sys.Home(), arg[2:])
	case filepath.IsAbs(arg):
		return filepath.Clean(arg)
	default:
		return filepath.Join(e.configHome, arg)
	}
}

// relUnderConfig gives the slash path of named relative to ~/.config, and whether
// it is under it at all.
func (e ownerEnv) relUnderConfig(named string) (string, bool) {
	rel, err := filepath.Rel(e.configHome, named)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// isRyokuOwned reports whether rel is a file the package ships and an update
// overwrites: recorded in the manifest, present in the base tree, or under the
// quickshell tree, which materialize converges as wholly Ryoku-owned even when
// the shipped baseline is not on disk (a dev box).
func (e ownerEnv) isRyokuOwned(rel string) bool {
	if rel == "quickshell" || strings.HasPrefix(rel, "quickshell/") {
		return true
	}
	if e.manifest[rel] {
		return true
	}
	return e.baseExists && sys.Exists(filepath.Join(e.base, rel))
}

// isStore reports whether rel is a Ryoku tool's JSON/TOML store directly under
// ~/.config/ryoku (shell.json, hub.toml, theme.json). Nested files (the overlay,
// the i18n user catalog) are not stores.
func isStore(rel string) bool {
	tail, ok := strings.CutPrefix(rel, "ryoku/")
	if !ok || strings.Contains(tail, "/") {
		return false
	}
	return strings.HasSuffix(tail, ".json") || strings.HasSuffix(tail, ".toml")
}

// storeWriter names a store's writer only where the code proves one. shell.json
// has a single writer (ryoku-shell, per the ryoku skill); the rest are left as a
// generic Ryoku tool so the answer never claims a writer it cannot back.
func storeWriter(rel string) string {
	if rel == "ryoku/shell.json" {
		return i18n.T("ryoku-shell (its sole writer)")
	}
	return i18n.T("a Ryoku tool; change it through its GUI or command")
}

// packageOwner returns the native package that owns an absolute path, or "".
func packageOwner(path string) string {
	owner, err := host.Default().PackageOwner(path)
	if err != nil {
		return ""
	}
	return owner
}

// classify is the whole verdict for one argument. The order is the contract's:
// the first class that matches wins.
func (e ownerEnv) classify(arg string) ownership {
	named := e.resolve(arg)
	o := ownership{Path: named}

	// overlay: the user's own fork tree, which wins over Ryoku's copy on update.
	if named == e.editsDir || strings.HasPrefix(named, e.editsDir+string(os.PathSeparator)) {
		o.Class = "overlay"
		o.Writer = i18n.T("you, through the user_edits overlay")
		o.Advice = i18n.T("This is your overlay copy; it wins over Ryoku's file at the mirrored ~/.config path on every update. Edit it here.")
		o.Edit = named
		return o
	}

	rel, underConfig := e.relUnderConfig(named)

	// forked: a shipped file the user copied into the overlay to fully own.
	if underConfig && forkable(rel) && e.isRyokuOwned(rel) {
		fork := filepath.Join(e.editsDir, filepath.FromSlash(rel))
		if sys.Exists(fork) {
			o.Class = "forked"
			o.Writer = i18n.T("you; your overlay fork wins over the shipped copy")
			o.Advice = i18n.Tf("You have forked this. Edit the overlay copy at %s; `ryoku reset %s` drops the fork and takes Ryoku's version again.", fork, rel)
			o.Edit = fork
			return o
		}
	}

	// generated: authored by a provider from the neutral store, or by the Hub.
	if underConfig && e.generated[rel] {
		o.Class = "generated"
		o.Writer = i18n.T("Ryoku Settings or the compositor provider")
		o.Advice = i18n.T("Change it in Ryoku Hub or the owning command, never by hand: the provider rewrites it from its store.")
		return o
	}

	// store: a Ryoku tool's JSON/TOML store under ~/.config/ryoku.
	if underConfig && isStore(rel) {
		o.Class = "store"
		o.Writer = storeWriter(rel)
		o.Advice = i18n.T("A Ryoku tool's store. Change it through its Ryoku Hub page or the owning command, not by hand.")
		return o
	}

	// seed: shipped once on a fresh install, then the machine's own.
	if underConfig && isSeed(rel) {
		o.Class = "seed"
		o.Writer = i18n.T("seeded once by Ryoku, then yours")
		o.Advice = i18n.T("Edit it in place. Ryoku seeds it once on a fresh install and never overwrites it again.")
		o.Edit = named
		return o
	}

	// user-override: a dedicated user file Ryoku reads but never ships.
	if underConfig && e.userOverride[rel] {
		o.Class = "user-override"
		o.Writer = i18n.T("you")
		o.Advice = i18n.T("Your own override file. Ryoku reads it but never ships or overwrites it, so this is where a tweak belongs.")
		o.Edit = named
		return o
	}

	// ryoku: shipped by the package and overwritten on every update.
	if underConfig && e.isRyokuOwned(rel) {
		o.Class = "ryoku"
		o.Writer = i18n.T("the ryoku-desktop package, through materialize")
		if rel == "quickshell" || strings.HasPrefix(rel, "quickshell/") {
			o.Advice = i18n.T("Ryoku ships this and overwrites it on every update. The shell tree cannot be forked (a fork breaks the shell); change it through ryoku-shell, Ryoku Hub, or a plugin (`ryoku plugin`).")
			return o
		}
		top := rel
		if i := strings.IndexByte(rel, '/'); i >= 0 {
			top = rel[:i]
		}
		if ov, ok := e.overrideForTree[top]; ok {
			o.Edit = filepath.Join(e.configHome, filepath.FromSlash(ov))
			o.Advice = i18n.Tf("Ryoku ships this and overwrites it on every update. Put your change in %s instead; Ryoku reads it but never overwrites it.", o.Edit)
		} else {
			o.Edit = filepath.Join(e.editsDir, filepath.FromSlash(rel))
			o.Advice = i18n.Tf("Ryoku ships this and overwrites it on every update. Fork it into the overlay at %s to keep a change; `ryoku reset %s` undoes the fork.", o.Edit, rel)
		}
		return o
	}

	// user: under ~/.config, never shipped.
	if underConfig {
		o.Class = "user"
		o.Writer = i18n.T("you, or another app")
		o.Advice = i18n.T("Your own file under ~/.config. Ryoku does not manage it.")
		o.Edit = named
		return o
	}

	// package: outside ~/.config, owned by a distribution package.
	if pkg := packageOwner(named); pkg != "" {
		o.Class = "package"
		o.Writer = i18n.Tf("the %s package", pkg)
		o.Advice = i18n.T("Package upgrades overwrite it. Use the program's documented override (for example an /etc drop-in) or ask before touching it.")
		return o
	}

	// unmanaged: outside ~/.config and owned by nothing Ryoku knows.
	o.Class = "unmanaged"
	o.Writer = i18n.T("unknown")
	o.Advice = i18n.T("Ryoku does not manage this path.")
	o.Edit = named
	return o
}

// Owner classifies config paths for an agent or a user: `ryoku owner <path>...`
// prints one block per path, `--json` prints the machine-readable array, and
// `--map` prints the whole-machine ownership summary the Rashin vault carries.
func Owner(args []string) error {
	jsonOut, mapOut := false, false
	var paths []string
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--map":
			mapOut = true
		case "-h", "--help":
			ownerUsage()
			return nil
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf(i18n.T("unknown argument: %s (try --help)"), a)
			}
			paths = append(paths, a)
		}
	}

	e := newOwnerEnv()
	if mapOut {
		fmt.Print(e.ownershipMap())
		return nil
	}
	if len(paths) == 0 {
		ownerUsage()
		return nil
	}

	verdicts := make([]ownership, 0, len(paths))
	for _, p := range paths {
		verdicts = append(verdicts, e.classify(p))
	}
	if jsonOut {
		b, err := json.MarshalIndent(verdicts, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	for i, v := range verdicts {
		if i > 0 {
			fmt.Println()
		}
		printOwnership(v)
	}
	return nil
}

func printOwnership(o ownership) {
	fmt.Println(sys.Bold(o.Path))
	if resolved, err := filepath.EvalSymlinks(o.Path); err == nil && resolved != o.Path {
		fmt.Printf(i18n.T("  resolves to %s\n"), resolved)
	}
	fmt.Printf(i18n.T("  class:   %s\n"), o.Class)
	fmt.Printf(i18n.T("  writer:  %s\n"), o.Writer)
	fmt.Printf(i18n.T("  advice:  %s\n"), o.Advice)
	if o.Edit != "" {
		fmt.Printf(i18n.T("  edit:    %s\n"), o.Edit)
	} else {
		fmt.Println(i18n.T("  edit:    go through the command or GUI above, not a file"))
	}
}

func ownerUsage() {
	fmt.Print(i18n.T("Usage: ryoku owner <path>... [--json]\n" +
		"       ryoku owner --map\n\n" +
		"  Classify config paths: who writes each one, and where your change\n" +
		"  belongs so an update does not overwrite it.\n\n" +
		"  <path>   absolute, ~/..., or relative to ~/.config\n" +
		"  --json   print the verdicts as a JSON array\n" +
		"  --map    print a markdown ownership summary for this machine\n"))
}

// ownershipMap renders the whole-machine ownership summary for the Rashin vault:
// the rule in plain words, then the overlay, the user-override files, the
// generated files, the tool stores, the seeds, and the Ryoku-owned trees as a
// compact table. Grounded in this box's manifest, seam and overlay, so a coding
// agent reading it in the vault sees where THIS machine's edits belong.
func (e ownerEnv) ownershipMap() string {
	var b strings.Builder
	b.WriteString(i18n.T("## Where a config change belongs\n\n"))
	b.WriteString(i18n.T("Ryoku ships your desktop as packages, and every `ryoku update` re-lays the\n" +
		"shipped files, so a change written straight into one is overwritten. Ask\n" +
		"`ryoku owner <path>` before editing anything under ~/.config. In short:\n\n"))
	b.WriteString(i18n.T("- Edit a shipped Ryoku file only through its overlay fork (`ryoku reset` undoes it).\n" +
		"- Generated files belong to Ryoku Hub or a command, never a hand edit.\n" +
		"- Seeds and your own override files are edited in place and survive updates.\n" +
		"- The shell tree (quickshell) cannot be forked; change it through ryoku-shell, the Hub, or a plugin.\n\n"))

	b.WriteString(i18n.T("## The overlay\n\n"))
	b.WriteString(i18n.Tf("Your fork tree lives at `%s`. A regular file here wins over Ryoku's copy at\nthe mirrored ~/.config path on every update.\n\n", e.editsDir))
	// The provider's generated settings also live in the overlay, out of sight;
	// they are Ryoku Hub's, listed under Generated files, not the user's forks.
	var forks []string
	if rels, _ := sys.UserEditFiles(); len(rels) > 0 {
		for _, rel := range rels {
			if !e.generated[rel] {
				forks = append(forks, rel)
			}
		}
	}
	if len(forks) > 0 {
		b.WriteString(i18n.T("Current forks on this machine:\n\n"))
		for _, rel := range forks {
			b.WriteString("- `" + rel + "`\n")
		}
		b.WriteString("\n")
	} else {
		b.WriteString(i18n.T("No forks yet.\n\n"))
	}

	b.WriteString(i18n.T("## Your override files\n\n"))
	b.WriteString(i18n.T("Dedicated files Ryoku reads but never ships, so a tweak here always survives:\n\n"))
	for _, ov := range e.overrideFileList() {
		state := i18n.T("absent")
		if sys.PathPresent(filepath.Join(e.configHome, filepath.FromSlash(ov.rel))) {
			state = i18n.T("present")
		}
		b.WriteString(fmt.Sprintf("- `%s` (%s) %s\n", ov.rel, state, ov.note))
	}
	b.WriteString("\n")

	if gen := sortedKeys(e.generated); len(gen) > 0 {
		b.WriteString(i18n.T("## Generated files\n\n"))
		b.WriteString(i18n.T("Written by Ryoku Hub or the compositor provider from a store. Change them in\nthe Hub, never by hand:\n\n"))
		for _, rel := range gen {
			b.WriteString("- `" + rel + "`\n")
		}
		b.WriteString("\n")
	}

	if stores := e.storeList(); len(stores) > 0 {
		b.WriteString(i18n.T("## Tool stores\n\n"))
		b.WriteString(i18n.T("A Ryoku tool's own JSON/TOML state under ~/.config/ryoku, each with one\nwriter. Change them through their GUI or command, never by hand:\n\n"))
		var others []string
		for _, s := range stores {
			if s.writer == storeWriter("") {
				others = append(others, "`"+s.rel+"`")
				continue
			}
			b.WriteString(fmt.Sprintf("- `%s` written by %s\n", s.rel, s.writer))
		}
		if len(others) > 0 {
			b.WriteString(i18n.T("- written by their own Ryoku tool: ") + strings.Join(others, ", ") + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(i18n.T("## Seeds\n\n"))
	b.WriteString(i18n.T("Shipped once on a fresh install, then yours. Edit in place; an update never\noverwrites them:\n\n"))
	for _, rel := range e.seedList() {
		b.WriteString("- `" + rel + "`\n")
	}
	b.WriteString(i18n.T("- `nvim/` (the whole LazyVim tree)\n\n"))

	b.WriteString(i18n.T("## Ryoku-owned trees\n\n"))
	if !e.baseExists {
		b.WriteString(i18n.Tf("The shipped baseline at `%s` was not found (a dev box), so this list is drawn\nfrom the update manifest and the shell tree only.\n\n", e.base))
	}
	b.WriteString(i18n.T("Shipped by the package and overwritten on every update. Never edit these; put\nthe change where the last column says.\n\n"))
	b.WriteString(i18n.T("| Tree | Files | Change goes to |\n|---|---|---|\n"))
	for _, g := range e.ryokuTreeGroups() {
		b.WriteString(fmt.Sprintf("| `%s` | %d | %s |\n", g.name, g.count, e.changeTarget(g.name)))
	}
	b.WriteString("\n")
	return b.String()
}

type overrideFile struct {
	rel  string
	note string
}

// overrideFileList is the user-override files worth naming: each provider's own
// hand-edit config (asked of the seam, so no compositor is named in this file)
// and the terminal and shell escape hatches.
func (e ownerEnv) overrideFileList() []overrideFile {
	var out []overrideFile
	seen := map[string]bool{}
	for _, name := range wm.Providers() {
		for _, rel := range wm.ConfigFiles(name) {
			if seen[rel] || !strings.Contains(filepath.Base(rel), ".") {
				continue // skip a directory escape hatch like <wm>/modules
			}
			seen[rel] = true
			out = append(out, overrideFile{rel, i18n.T("your own compositor config, read but never shipped")})
		}
	}
	notes := map[string]string{
		"kitty/user.conf": i18n.T("kitty terminal tweaks, included last"),
		"fish/user.fish":  i18n.T("your fish additions"),
		"bash/user.bash":  i18n.T("your bash additions"),
		"zsh/user.zsh":    i18n.T("your zsh additions"),
	}
	for _, rel := range shellUserOverrides {
		if seen[rel] {
			continue
		}
		out = append(out, overrideFile{rel, notes[rel]})
	}
	return out
}

type storeEntry struct {
	rel    string
	writer string
}

// storeList is the tool stores actually present under ~/.config/ryoku on this
// machine, each with its writer.
func (e ownerEnv) storeList() []storeEntry {
	entries, err := os.ReadDir(filepath.Join(e.configHome, "ryoku"))
	if err != nil {
		return nil
	}
	var out []storeEntry
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		rel := "ryoku/" + ent.Name()
		if isStore(rel) {
			out = append(out, storeEntry{rel, storeWriter(rel)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out
}

// seedList is the per-path seeds (the whole nvim tree is noted separately), sorted.
func (e ownerEnv) seedList() []string {
	return sortedKeys(generatedSeed)
}

type treeGroup struct {
	name  string
	count int
}

// ryokuTreeGroups groups this machine's Ryoku-owned files by their top-level
// tree, so the map summarises them in a few rows instead of listing every
// shipped file. Seeds, generated files, stores and user overrides are excluded:
// those are not the files an update overwrites.
func (e ownerEnv) ryokuTreeGroups() []treeGroup {
	set := map[string]bool{}
	for rel := range e.manifest {
		set[rel] = true
	}
	if e.baseExists {
		if rels, err := walkRel(e.base); err == nil {
			for _, rel := range rels {
				set[rel] = true
			}
		}
	}
	if rels, err := walkRel(filepath.Join(e.configHome, "quickshell")); err == nil {
		for _, rel := range rels {
			set["quickshell/"+rel] = true
		}
	}
	counts := map[string]int{}
	for rel := range set {
		if isSeed(rel) || e.generated[rel] || isStore(rel) || e.userOverride[rel] {
			continue
		}
		top := rel
		if i := strings.IndexByte(rel, '/'); i >= 0 {
			top = rel[:i] + "/"
		}
		counts[top]++
	}
	out := make([]treeGroup, 0, len(counts))
	for name, n := range counts {
		out = append(out, treeGroup{name, n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// changeTarget is where a change to a file in tree belongs: the shell tree points
// at ryoku-shell, a tree with a user-override file points there, and any other
// shipped tree forks through the overlay.
func (e ownerEnv) changeTarget(tree string) string {
	top := strings.TrimSuffix(tree, "/")
	if top == "quickshell" {
		return i18n.T("ryoku-shell, Ryoku Hub, or `ryoku plugin`")
	}
	if ov, ok := e.overrideForTree[top]; ok {
		return "`~/.config/" + ov + "`"
	}
	return i18n.T("a fork in the overlay")
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
