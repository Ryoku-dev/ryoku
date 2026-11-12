package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Building a compositor plugin on this box. One builder serves deploy.sh, the
// Plugins page's Rebuild, the doctor's converge on update, and Add from any git
// repository. The recipe is the upstream's hyprpm.toml; the result lands under
// userPluginDir with its .abi and a receipt.

// hyprpmManifest is the part of hyprpm.toml a build needs.
type hyprpmManifest struct {
	Name    string
	Pins    [][]string // [hyprland commit, plugin commit]
	Plugins map[string]hyprpmPlugin
	Order   []string // plugin tables in document order
}

type hyprpmPlugin struct {
	Description string   `toml:"description"`
	Output      string   `toml:"output"`
	Build       []string `toml:"build"`
	// Assets is a Ryoku extension for plugins authored in this repo: a directory
	// the build fills, laid beside the .so as <userPluginDir>/<id>/.
	Assets string `toml:"assets"`
}

func parseHyprpmManifest(text string) (hyprpmManifest, error) {
	var raw map[string]toml.Primitive
	md, err := toml.Decode(text, &raw)
	if err != nil {
		return hyprpmManifest{}, fmt.Errorf("hyprpm.toml: %w", err)
	}
	m := hyprpmManifest{Plugins: map[string]hyprpmPlugin{}}
	for _, k := range md.Keys() {
		if len(k) != 1 {
			continue
		}
		name := k[0]
		prim, ok := raw[name]
		if !ok {
			continue
		}
		if name == "repository" {
			var repo struct {
				Name string     `toml:"name"`
				Pins [][]string `toml:"commit_pins"`
			}
			if err := md.PrimitiveDecode(prim, &repo); err != nil {
				return m, fmt.Errorf("hyprpm.toml [repository]: %w", err)
			}
			m.Name, m.Pins = repo.Name, repo.Pins
			continue
		}
		var p hyprpmPlugin
		if err := md.PrimitiveDecode(prim, &p); err != nil {
			return m, fmt.Errorf("hyprpm.toml [%s]: %w", name, err)
		}
		if p.Output == "" {
			continue
		}
		m.Plugins[name] = p
		m.Order = append(m.Order, name)
	}
	if len(m.Plugins) == 0 {
		return m, fmt.Errorf("hyprpm.toml declares no plugin (no table with an output)")
	}
	return m, nil
}

// pinFor returns the plugin commit the manifest pairs with a Hyprland commit,
// "" when the table has no pin for it (build the default branch, as hyprpm does).
func (m hyprpmManifest) pinFor(hyprlandCommit string) string {
	for _, p := range m.Pins {
		if len(p) == 2 && p[0] == hyprlandCommit {
			return p[1]
		}
	}
	return ""
}

func readManifest(dir string) (hyprpmManifest, error) {
	b, err := os.ReadFile(filepath.Join(dir, "hyprpm.toml"))
	if err != nil {
		return hyprpmManifest{}, fmt.Errorf("no hyprpm.toml in %s: a Hyprland plugin repository declares its builds there", dir)
	}
	return parseHyprpmManifest(string(b))
}

// tipManifest reads hyprpm.toml as the default branch's tip has it: the pin table
// only grows at the tip, and a pinned commit's own copy predates later pins.
func tipManifest(dir string) (hyprpmManifest, error) {
	text := gitOut(dir, "show", "origin/HEAD:hyprpm.toml")
	if text == "" {
		return readManifest(dir)
	}
	return parseHyprpmManifest(text)
}

func srcCacheDir() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "ryoku", "hypr-plugins-src")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "ryoku", "hypr-plugins-src")
}

// repoSlug names a clone dir after its URL: host and path, nothing else.
func repoSlug(url string) string {
	s := strings.TrimSuffix(strings.TrimSpace(url), "/")
	s = strings.TrimSuffix(s, ".git")
	for _, p := range []string{"https://", "http://", "ssh://", "git@", "git://"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.NewReplacer("/", "-", ":", "-", "@", "-").Replace(s)
	return strings.Trim(s, "-")
}

func gitRun(dir string, log io.Writer, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = log, log
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd.Run()
}

func gitOut(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// fetchRepo clones a plugin repository into the source cache or brings an
// existing clone up to date. Full history, since a pin may point well behind tip.
func fetchRepo(url string, log io.Writer) (string, error) {
	slug := repoSlug(url)
	if slug == "" {
		return "", fmt.Errorf("not a git URL: %q", url)
	}
	dir := filepath.Join(srcCacheDir(), slug)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		fmt.Fprintf(log, "fetching %s\n", url)
		if err := gitRun(dir, log, "fetch", "--tags", "--prune", "--quiet", "origin"); err != nil {
			return "", fmt.Errorf("git fetch %s: %w", url, err)
		}
		return dir, nil
	}
	if err := os.MkdirAll(srcCacheDir(), 0o755); err != nil {
		return "", err
	}
	fmt.Fprintf(log, "cloning %s\n", url)
	if err := gitRun("", log, "clone", "--quiet", url, dir); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("git clone %s: %w", url, err)
	}
	return dir, nil
}

// checkoutFor puts the clone on the commit the manifest pins for the Hyprland
// being built against, or the default branch's tip when there is no pin.
func checkoutFor(dir string, m hyprpmManifest, hyprlandCommit string, log io.Writer) (string, error) {
	ref := m.pinFor(hyprlandCommit)
	if ref == "" {
		head := gitOut(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
		if head == "" {
			head = "origin/HEAD"
		}
		ref = head
		fmt.Fprintf(log, "no pin for Hyprland %s in hyprpm.toml; building %s\n", shortCommit(hyprlandCommit), ref)
	} else {
		fmt.Fprintf(log, "pinned to %s for Hyprland %s\n", shortCommit(ref), shortCommit(hyprlandCommit))
	}
	if err := gitRun(dir, log, "checkout", "--quiet", "--force", "--detach", ref); err != nil {
		return "", fmt.Errorf("git checkout %s: %w", ref, err)
	}
	return gitOut(dir, "rev-parse", "HEAD"), nil
}

// checkoutRoot is the repository a dev box deploys from: the flag, then the env,
// then the pointer deploy.sh records.
func checkoutRoot(flag string) string {
	if flag != "" {
		return flag
	}
	if r := os.Getenv("RYOKU_REPO"); r != "" {
		return r
	}
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, _ := os.UserHomeDir()
		state = filepath.Join(home, ".local", "state")
	}
	b, err := os.ReadFile(filepath.Join(state, "ryoku", "repo"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// localSourceDir finds the source of a Ryoku-authored plugin: the checkout when
// this box deploys from one, else the copy ryoku-desktop ships.
func localSourceDir(d pluginDef, checkout string) (string, bool) {
	if checkout != "" {
		p := filepath.Join(checkout, d.Local)
		if _, err := os.Stat(filepath.Join(p, "hyprpm.toml")); err == nil {
			return p, true
		}
	}
	p := filepath.Join(sharedRecipeDir, d.ID)
	if _, err := os.Stat(filepath.Join(p, "hyprpm.toml")); err == nil {
		return p, true
	}
	return "", false
}

// newestMtime walks a source tree for its newest file, so a local plugin whose
// source changed since its last build counts as stale.
func newestMtime(dir string) time.Time {
	var newest time.Time
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() && (fi.Name() == ".git" || fi.Name() == "build" || fi.Name() == "out") {
			return filepath.SkipDir
		}
		if !fi.IsDir() && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
		return nil
	})
	return newest
}

func runBuildSteps(dir string, steps []string, log io.Writer) error {
	if len(steps) == 0 {
		return fmt.Errorf("hyprpm.toml lists no build steps")
	}
	for _, s := range steps {
		fmt.Fprintf(log, "$ %s\n", s)
		cmd := exec.Command("sh", "-c", s)
		cmd.Dir = dir
		cmd.Stdout, cmd.Stderr = log, log
		cmd.Env = append(os.Environ(), fmt.Sprintf("MAKEFLAGS=-j%d", runtime.NumCPU()), fmt.Sprintf("CMAKE_BUILD_PARALLEL_LEVEL=%d", runtime.NumCPU()))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("build step failed: %s", s)
		}
	}
	return nil
}

// pristineSource drops what an earlier build left in a plugin's tree: upstream
// Makefiles track no header dependency, so old objects would just be relinked.
func pristineSource(dir, output string, log io.Writer) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		if err := gitRun(dir, log, "clean", "-ffdxq"); err != nil {
			return fmt.Errorf("git clean %s: %w", dir, err)
		}
		if fileExists(filepath.Join(dir, ".gitmodules")) {
			_ = gitRun(dir, log, "submodule", "foreach", "--quiet", "--recursive", "git", "clean", "-ffdxq")
		}
		return nil
	}
	if output != "" {
		if err := os.Remove(filepath.Join(dir, output)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(p, ".o") {
			return err
		}
		return os.Remove(p)
	})
}

var commitLiteralRe = regexp.MustCompile(`(?:^|[^0-9a-f])([0-9a-f]{40})(?:[^0-9a-f]|$)`)

// foreignHyprland names the Hyprland commit baked into a plugin .so when it is
// not the given one ("" when it matches or the .so names no commit at all).
func foreignHyprland(so, commit string) string {
	b, err := os.ReadFile(so)
	if err != nil || commit == "" || bytes.Contains(b, []byte(commit)) {
		return ""
	}
	if m := commitLiteralRe.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

// installSo lays a built .so into the user tier with its .abi receipt, to a fresh
// inode: truncating a .so the running compositor has mapped crashes it.
func installSo(built, id string, abi hyprABI) (string, error) {
	_ = exec.Command("strip", "--strip-unneeded", built).Run()
	b, err := os.ReadFile(built)
	if err != nil {
		return "", fmt.Errorf("build produced no %s: %w", filepath.Base(built), err)
	}
	dst := filepath.Join(userPluginDir(), id+".so")
	if err := atomicWrite(dst, b, 0o755); err != nil {
		return "", err
	}
	if err := atomicWrite(abiSidecar(dst), []byte(abi.String()+"\n"), 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

// copyTree replaces dst with a copy of src (.git and build outputs skipped).
func copyTree(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if fi.IsDir() && (fi.Name() == ".git" || (rel != "." && (fi.Name() == "build" || fi.Name() == "out"))) {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, fi.Mode().Perm())
	})
}

// buildJob is one plugin to build in a run.
type buildJob struct {
	def    pluginDef
	reason string
}

type buildResult struct {
	Built   []string          `json:"built"`
	Skipped []string          `json:"skipped"`
	Failed  map[string]string `json:"failed"`
}

// buildPlugins runs a set of jobs, sharing one fetch per repository.
func buildPlugins(jobs []buildJob, checkout string, log io.Writer) buildResult {
	res := buildResult{Built: []string{}, Skipped: []string{}, Failed: map[string]string{}}
	hdr, hdrTag := headersABI()
	if !hdr.ok() {
		for _, j := range jobs {
			res.Failed[j.def.ID] = "Hyprland headers not found (install the hyprland package)"
		}
		return res
	}
	fetched := map[string]string{}
	for _, j := range jobs {
		d := j.def
		fmt.Fprintf(log, "== %s (%s)\n", d.ID, j.reason)
		var dir, commit string
		var err error
		switch {
		case d.Repo != "":
			dir, err = repoDirFor(d.Repo, fetched, log)
			if err != nil {
				res.Failed[d.ID] = err.Error()
				continue
			}
		case d.Local != "":
			src, ok := localSourceDir(d, checkout)
			if !ok {
				res.Failed[d.ID] = "source not found (no checkout and nothing under " + sharedRecipeDir + ")"
				continue
			}
			dir = filepath.Join(srcCacheDir(), "local-"+d.ID)
			if err := copyTree(src, dir); err != nil {
				res.Failed[d.ID] = err.Error()
				continue
			}
			commit = gitOut(src, "rev-parse", "HEAD")
		default:
			res.Failed[d.ID] = "no source to build from"
			continue
		}
		m, err := tipManifest(dir)
		if err != nil {
			res.Failed[d.ID] = err.Error()
			continue
		}
		if d.Repo != "" {
			if commit, err = checkoutFor(dir, m, hdr.Commit, log); err != nil {
				res.Failed[d.ID] = err.Error()
				continue
			}
			if m, err = readManifest(dir); err != nil {
				res.Failed[d.ID] = err.Error()
				continue
			}
		}
		p, ok := m.Plugins[d.Plugin]
		if !ok {
			res.Failed[d.ID] = fmt.Sprintf("hyprpm.toml has no [%s]", d.Plugin)
			continue
		}
		if err := pristineSource(dir, p.Output, log); err != nil {
			res.Failed[d.ID] = err.Error()
			continue
		}
		if err := runBuildSteps(dir, p.Build, log); err != nil {
			res.Failed[d.ID] = err.Error()
			continue
		}
		built := filepath.Join(dir, p.Output)
		if other := foreignHyprland(built, hdr.Commit); other != "" {
			res.Failed[d.ID] = fmt.Sprintf("the build came out for Hyprland %s, not the installed %s; kept the old copy", shortCommit(other), shortCommit(hdr.Commit))
			continue
		}
		so, err := installSo(built, d.ID, hdr)
		if err != nil {
			res.Failed[d.ID] = err.Error()
			continue
		}
		if p.Assets != "" {
			if err := copyTree(filepath.Join(dir, p.Assets), filepath.Join(userPluginDir(), d.ID)); err != nil {
				res.Failed[d.ID] = err.Error()
				continue
			}
		}
		prior, _ := readReceipt(d.ID)
		r := pluginReceipt{
			ID: d.ID, Name: d.Name, Desc: d.Desc, Repo: d.Repo, Local: d.Local, Plugin: d.Plugin,
			Commit: commit, Hyprland: hdrTag, BuiltAt: time.Now().UTC().Format(time.RFC3339),
			Loaded: prior.Loaded, Settings: prior.Settings,
		}
		if r.Desc == "" {
			r.Desc = p.Description
		}
		if _, bundled := bundledByID(d.ID); !bundled {
			r.Settings = detectSettings(so, false, prior.Settings)
		}
		if r.Settings == nil {
			r.Settings = []detectedSetting{}
		}
		if err := writeReceipt(r); err != nil {
			res.Failed[d.ID] = err.Error()
			continue
		}
		fmt.Fprintf(log, "installed %s\n", so)
		res.Built = append(res.Built, d.ID)
	}
	return res
}

func repoDirFor(url string, fetched map[string]string, log io.Writer) (string, error) {
	if dir, ok := fetched[url]; ok {
		return dir, nil
	}
	dir, err := fetchRepo(url, log)
	if err != nil {
		return "", err
	}
	fetched[url] = dir
	return dir, nil
}

// needsBuild says whether a plugin has no copy matching the installed headers. A
// Ryoku-authored plugin is also stale once its source is newer than the build.
func needsBuild(d pluginDef, hdr hyprABI, checkout string) (bool, string) {
	copies := pluginCopies(d.ID)
	for _, c := range copies {
		if c.Tier == "hyprpm" || !c.ABI.ok() || c.ABI != hdr || foreignHyprland(c.Path, hdr.Commit) != "" {
			continue
		}
		if d.Local != "" && c.Tier == "built" {
			if src, ok := localSourceDir(d, checkout); ok {
				if fi, err := os.Stat(c.Path); err == nil && newestMtime(src).After(fi.ModTime()) {
					return true, "source changed"
				}
			}
		}
		return false, ""
	}
	if len(copies) == 0 {
		return true, "not installed"
	}
	return true, "built for another Hyprland"
}

// runPluginRebuild: `plugins rebuild [--all|--stale] [--checkout <dir>] [<id>...]`.
// Log lines go to stderr, the JSON summary to stdout.
func runPluginRebuild(args []string) error {
	var ids []string
	var all, stale bool
	var checkout string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--all":
			all = true
		case "--stale":
			stale = true
		case "--checkout":
			if i+1 >= len(args) {
				return fmt.Errorf("--checkout needs a directory")
			}
			i++
			checkout = args[i]
		default:
			ids = append(ids, args[i])
		}
	}
	if !all && !stale && len(ids) == 0 {
		return fmt.Errorf("plugins rebuild needs --all, --stale or plugin ids")
	}
	if tc := toolchain(); !tc.OK {
		return fmt.Errorf("cannot build plugins here: missing %s", strings.Join(tc.Missing, ", "))
	}
	checkout = checkoutRoot(checkout)
	hdr, _ := headersABI()

	defs := map[string]pluginDef{}
	var order []string
	for _, d := range bundledPlugins {
		defs[d.ID], order = d, append(order, d.ID)
	}
	for _, r := range localReceipts() {
		if _, ok := defs[r.ID]; !ok {
			defs[r.ID], order = receiptDef(r), append(order, r.ID)
		}
	}
	if len(ids) == 0 {
		ids = order
	}

	var jobs []buildJob
	res := buildResult{Built: []string{}, Skipped: []string{}, Failed: map[string]string{}}
	for _, id := range ids {
		d, ok := defs[id]
		if !ok {
			res.Failed[id] = "unknown plugin"
			continue
		}
		if d.Repo == "" && d.Local == "" {
			res.Failed[id] = "no source to build from"
			continue
		}
		reason := "requested"
		if stale && !all {
			need, why := needsBuild(d, hdr, checkout)
			if !need {
				res.Skipped = append(res.Skipped, id)
				continue
			}
			reason = why
		}
		jobs = append(jobs, buildJob{def: d, reason: reason})
	}
	built := buildPlugins(jobs, checkout, os.Stderr)
	res.Built = append(res.Built, built.Built...)
	res.Skipped = append(res.Skipped, built.Skipped...)
	for k, v := range built.Failed {
		res.Failed[k] = v
	}
	if len(res.Built) > 0 {
		// re-emit settings.lua so it names the fresh copies, and swap a running
		// plugin for its fresh build: the old image stays mapped until unloaded.
		o := loadStore(desktopStorePath())
		_ = writeOverlayLua("settings.lua", []byte(genLua(o, borderFollowsPalette(o))))
		loaded := loadedPlugins()
		swapped := false
		for _, id := range res.Built {
			if _, ok := loaded[defs[id].Loaded]; !ok {
				continue
			}
			unloadPluginCopies(id)
			swapped = true
		}
		if swapped && loadEnabledPlugins(o) {
			pushEval(genPluginConfig(o))
		}
	}
	return printJSON(res)
}

// repoInspection is what `add --inspect` reports: the plugins a repository offers.
type repoInspection struct {
	Repo    string              `json:"repo"`
	Name    string              `json:"name"`
	Plugins []inspectedManifest `json:"plugins"`
}

type inspectedManifest struct {
	ID        string `json:"id"`
	Desc      string `json:"desc"`
	Bundled   bool   `json:"bundled"`
	Installed bool   `json:"installed"`
}

// runPluginAdd: `plugins add [--inspect] <git-url> [<plugin>...]`. Builds only;
// the store entry is created when the user enables it through a normal save.
func runPluginAdd(args []string) error {
	inspect := false
	var rest []string
	for _, a := range args {
		if a == "--inspect" {
			inspect = true
		} else {
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		return fmt.Errorf("plugins add needs a git URL")
	}
	url := rest[0]
	if tc := toolchain(); !tc.OK {
		return fmt.Errorf("cannot build plugins here: missing %s", strings.Join(tc.Missing, ", "))
	}
	dir, err := fetchRepo(url, os.Stderr)
	if err != nil {
		return err
	}
	m, err := tipManifest(dir)
	if err != nil {
		return err
	}
	if inspect {
		out := repoInspection{Repo: url, Name: m.Name, Plugins: []inspectedManifest{}}
		for _, id := range m.Order {
			_, bundled := bundledByID(id)
			_, installed := readReceipt(id)
			out.Plugins = append(out.Plugins, inspectedManifest{ID: id, Desc: m.Plugins[id].Description, Bundled: bundled, Installed: installed})
		}
		return printJSON(out)
	}
	want := rest[1:]
	if len(want) == 0 {
		if len(m.Order) != 1 {
			return fmt.Errorf("%s offers %d plugins (%s): name the ones to add", m.Name, len(m.Order), strings.Join(m.Order, ", "))
		}
		want = m.Order
	}
	var jobs []buildJob
	res := buildResult{Built: []string{}, Skipped: []string{}, Failed: map[string]string{}}
	for _, id := range want {
		p, ok := m.Plugins[id]
		if !ok {
			res.Failed[id] = fmt.Sprintf("%s has no plugin named %s", m.Name, id)
			continue
		}
		if _, bundled := bundledByID(id); bundled {
			res.Failed[id] = "already bundled with Ryoku; rebuild it from its own row instead"
			continue
		}
		jobs = append(jobs, buildJob{
			def:    pluginDef{ID: id, Name: id, Desc: p.Description, Repo: url, Plugin: id, Docs: url, Loaded: id},
			reason: "add",
		})
	}
	built := buildPlugins(jobs, "", os.Stderr)
	res.Built = append(res.Built, built.Built...)
	for k, v := range built.Failed {
		res.Failed[k] = v
	}
	return printJSON(res)
}

// runPluginRemove: `plugins remove <id>` drops a locally added plugin's copy from
// the compositor and its files from the user tier. The store entry is the Hub's
// to drop (single writer), so this reports the key rather than writing it.
func runPluginRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("plugins remove needs one plugin id")
	}
	id := args[0]
	if _, bundled := bundledByID(id); bundled {
		return fmt.Errorf("%s is bundled with Ryoku: turn it off instead of removing it", id)
	}
	unloadPluginCopies(id)
	so := filepath.Join(userPluginDir(), id+".so")
	for _, p := range []string{so, abiSidecar(so), receiptPath(id)} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return printJSON(map[string]string{"removed": id, "storeKey": "wm.hyprland.plugins.extra." + id})
}

// sortedExtraIDs gives a stable order for the extra plugins in a store.
func sortedExtraIDs(extra map[string]ExtraPlugin) []string {
	ids := make([]string, 0, len(extra))
	for id := range extra {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
