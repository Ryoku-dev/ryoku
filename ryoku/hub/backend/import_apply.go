package main

// Apply a config import: re-scan the source, resolve every keybind conflict per
// the decisions, back up every file to be touched (plus a best-effort snapper
// snapshot), then write. Ingestable Hyprland binds and window rules land in
// desktop.json (the provider regenerates the compositor config from it); a "mine"
// shadow also records an unbind so the imported bind wins over the shipped one.
// Everything else layers into each app's user-include inside a marked,
// idempotent, undoable block. Writes are
// validated (Lua parse) before committing and rolled back from the backup on any
// error.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// pendingWrite is one file apply will replace; luaCheck marks Lua output that
// must parse before anything is committed.
type pendingWrite struct {
	path     string
	content  []byte
	luaCheck bool
}

// conflictChoice is a decoded conflict decision. mode is "ryoku" (default),
// "mine", or "remap"; remap carries the new chord for the remap case.
type conflictChoice struct {
	mode  string
	remap string
}

func backupsDir() string { return filepath.Join(configHome(), "ryoku", "import-backups") }

func applyImport(dec decisions) (applyResult, error) {
	if strings.TrimSpace(dec.Source) == "" {
		return applyResult{}, fmt.Errorf("decisions.source is empty")
	}
	ts := time.Now().Format("20060102-150405")
	res := applyResult{Ts: ts, BackupDir: filepath.Join(backupsDir(), ts)}

	resolve := map[string]conflictChoice{}
	for norm, raw := range dec.Conflicts {
		resolve[norm] = parseChoice(raw)
	}

	var writes []pendingWrite
	var ingBinds []Keybind
	var ingRules []WindowRule
	var ingUnbinds []string
	binds, rules, unbinds := 0, 0, 0
	var unresolved []string
	var providerPatches []map[string]any
	var importedProviders []string
	providerScans := scanProviderImports(dec.Source)

	// Native provider configs are parsed by their provider. The Hub only merges
	// the returned neutral patch and owns the same backup transaction as every
	// other imported app.
	for _, scan := range providerScans {
		if !included(dec, providerAppID(scan.Provider)) {
			continue
		}
		if scan.Error != "" {
			return res, fmt.Errorf("%s", scan.Error)
		}
		patch, importedBinds, importedUnbinds := providerPatchFor(scan, resolve)
		if importPatchNonEmpty(patch) {
			providerPatches = append(providerPatches, patch)
		}
		binds += importedBinds
		unbinds += importedUnbinds
		for _, item := range scan.Items {
			if item.Ingestable && (item.Kind == "windowrule" || item.Kind == "layerrule") {
				rules++
			}
		}
		for _, loss := range scan.Losses {
			unresolved = append(unresolved, loss.Raw+": "+loss.Reason)
		}
		if strings.TrimSpace(scan.Preserved) != "" {
			target, err := providerPreserveTarget(scan.PreservePath)
			if err != nil {
				return res, err
			}
			body := headerComment("//", ts, dec.Source) + ensureTrailingNL(scan.Preserved)
			content := upsertBlock(readFileOr(target), ts, body, "//")
			writes = append(writes, pendingWrite{path: target, content: []byte(content)})
		}
		importedProviders = append(importedProviders, scan.Provider)
	}

	// --- Hyprland: ingest into desktop.json + a user.lua raw block ------------
	if included(dec, "hyprland") {
		if hpath, ok := findConfig(dec.Source, "hypr", "hyprland.conf"); ok {
			b, _ := os.ReadFile(hpath)
			_, hs := parseHyprland(string(b))
			plan := planBinds(hs.Binds, ryokuShipped(), resolve)

			var userLua []string
			for _, kb := range plan.keep {
				if kb.Ingestable {
					ingBinds = append(ingBinds, Keybind{Keys: kb.Combo, Action: kb.Action, Value: kb.Value})
					binds++
					continue
				}
				if lua, ok := translateBind(kb); ok {
					userLua = append(userLua, lua)
				} else {
					userLua = append(userLua, portByHand(kb))
					unresolved = append(unresolved, kb.Raw)
				}
			}
			ingUnbinds = append(ingUnbinds, plan.unbinds...)
			unbinds += len(plan.unbinds)

			for _, r := range hs.Rules {
				ingRules = append(ingRules, r.Rule)
				rules++
			}
			for _, rl := range hs.Raws {
				if rl.Lua != "" {
					userLua = append(userLua, rl.Lua)
				} else {
					userLua = append(userLua, commentPreserve(rl.Raw))
				}
			}
			if len(userLua) > 0 {
				target := filepath.Join(hyprConfigDir(), "user.lua")
				body := headerComment("--", ts, dec.Source) + strings.Join(userLua, "\n") + "\n"
				content := upsertBlock(readFileOr(target), ts, body, "--")
				writes = append(writes, pendingWrite{path: target, content: []byte(content), luaCheck: true})
			}
		}
	}

	// --- layer tiers: append each app's config into its user-include ----------
	if included(dec, "kitty") {
		if p, ok := findConfig(dec.Source, "kitty", "kitty.conf"); ok {
			body := headerComment("#", ts, dec.Source) + ensureTrailingNL(readFileOr(p))
			writes = append(writes, layerWrite("kitty", "user.conf", "#", ts, body))
		}
	}
	if included(dec, "fish") {
		if p, ok := findConfig(dec.Source, "fish", "config.fish"); ok {
			body := headerComment("#", ts, dec.Source) + fishBody(filepath.Dir(p), p)
			writes = append(writes, layerWrite("fish", "user.fish", "#", ts, body))
		}
	}
	if included(dec, "fastfetch") {
		if p, ok := findConfig(dec.Source, "fastfetch", "config.jsonc"); ok {
			// jsonc uses // comments; the marked block stays a valid comment.
			body := headerComment("//", ts, dec.Source) + ensureTrailingNL(readFileOr(p))
			writes = append(writes, layerWrite("fastfetch", "user.jsonc", "//", ts, body))
		}
	}
	// --- drop tier: any other config the user brought -------------------------
	for _, ga := range scanGeneric(dec.Source, providerExcludedDirs(providerScans)) {
		if !included(dec, ga.ID) {
			continue
		}
		var parts []string
		for _, it := range ga.Items {
			parts = append(parts, fmt.Sprintf("# --- %s ---\n", it.Raw)+ensureTrailingNL(readFileOr(filepath.Join(ga.Path, it.Raw))))
		}
		body := headerComment("#", ts, dec.Source) + strings.Join(parts, "")
		writes = append(writes, layerWrite(ga.ID, "user.conf", "#", ts, body))
	}

	// --- persist every typed mapping into desktop.json -----------------------
	ingested := binds > 0 || rules > 0 || unbinds > 0 || len(providerPatches) > 0
	if ingested {
		ns := readJSONMap(desktopStorePath())
		appendDesktopSection(ns, "keybinds", ingBinds)
		appendDesktopSection(ns, "windowRules", ingRules)
		appendDesktopSection(ns, "unbinds", ingUnbinds)
		for _, patch := range providerPatches {
			mergeImportPatch(ns, patch)
		}
		writes = append(writes, pendingWrite{path: desktopStorePath(), content: mustJSON(ns)})
		// Only a live target provider is regenerated. Importing an inactive
		// provider updates its namespace and neutral settings without changing
		// the compositor the user is currently running.
		if len(importedProviders) == 0 {
			writes = append(writes, generatedConfigWrites()...)
		} else {
			for _, provider := range importedProviders {
				if wmImportProviderLive(provider) {
					writes = append(writes, providerGeneratedConfigWrites(provider)...)
				}
			}
		}
	}
	if !ingested {
		for _, provider := range importedProviders {
			if wmImportProviderLive(provider) {
				writes = append(writes, providerGeneratedConfigWrites(provider)...)
			}
		}
	}

	man, err := backup(res.BackupDir, ts, writes)
	if err != nil {
		return res, err
	}
	for _, w := range writes {
		if w.luaCheck && !luaContentOK(w.content) {
			return res, fmt.Errorf("generated Lua for %s failed validation", w.path)
		}
	}
	for _, w := range writes {
		if err := atomicWrite(w.path, w.content, 0o644); err != nil {
			rollback(man)
			return res, fmt.Errorf("write %s: %w (rolled back)", w.path, err)
		}
	}
	if ingested || len(importedProviders) > 0 {
		if len(importedProviders) == 0 {
			_ = applyDesktop()
		} else {
			for _, provider := range importedProviders {
				if err := applyImportedProvider(provider); err != nil {
					rollback(man)
					return res, fmt.Errorf("apply imported provider: %w (rolled back)", err)
				}
			}
		}
	}

	res.FilesWritten = relPaths(writePaths(writes))
	res.BindsIngested, res.RulesIngested, res.Unbinds = binds, rules, unbinds
	res.Unresolved = unresolved
	return res, nil
}

// appendDesktopSection appends a typed slice to desktop.<key> in ns, creating the
// array when absent. An empty slice is a no-op.
func appendDesktopSection(ns map[string]any, key string, items any) {
	ib, _ := json.Marshal(items)
	var add []json.RawMessage
	_ = json.Unmarshal(ib, &add)
	if len(add) == 0 {
		return
	}
	d := childMap(ns, "desktop")
	var cur []json.RawMessage
	if raw, ok := d[key]; ok {
		b, _ := json.Marshal(raw)
		_ = json.Unmarshal(b, &cur)
	}
	d[key] = append(cur, add...)
}

// bindPlan is the resolved set of imported binds to apply (remaps already
// folded into Combo/Norm) plus the shipped chords to unbind so a "mine" wins.
type bindPlan struct {
	keep    []importBind
	unbinds []string
}

// planBinds turns the imported binds + conflict decisions into the apply plan.
// It re-derives the same conflict classification scan reports (shipped shadows
// win over self-duplicates, one owner per chord) so the decision keyed by norm
// lands on the right bind.
func planBinds(binds []importBind, shipped map[string]shippedInfo, resolve map[string]conflictChoice) bindPlan {
	kind := map[string]string{}
	firstOf := map[string]int{}
	shippedWinner := map[string]int{}
	dupFirst := map[string]int{}
	dupSecond := map[string]int{}
	for i, b := range binds {
		n := b.Norm
		if n == "" || kind[n] != "" {
			continue
		}
		if _, ok := shipped[n]; ok {
			kind[n] = "shipped"
			shippedWinner[n] = i
			continue
		}
		if fi, ok := firstOf[n]; ok {
			kind[n] = "duplicate"
			dupFirst[n] = fi
			dupSecond[n] = i
			continue
		}
		firstOf[n] = i
	}

	plan := bindPlan{}
	unbound := map[string]bool{}
	for i, b := range binds {
		n := b.Norm
		ch := resolve[n] // absent => zero value => default ("ryoku")
		switch kind[n] {
		case "shipped":
			if i != shippedWinner[n] {
				continue // extra bind on an already-shadowed chord: drop
			}
			switch ch.mode {
			case "mine":
				plan.keep = append(plan.keep, b)
				// Only release the shipped chord when the user's replacement will
				// actually take effect (ingestable, or a translatable dispatcher);
				// unbinding for a comment-only bind would leave a dead key.
				if bindApplies(b) {
					if si, ok := shipped[n]; ok && !unbound[n] {
						plan.unbinds = append(plan.unbinds, si.Combo)
						unbound[n] = true
					}
				}
			case "remap":
				plan.keep = append(plan.keep, remapBind(b, ch.remap))
			}
		case "duplicate":
			switch ch.mode {
			case "mine":
				if i == dupSecond[n] {
					plan.keep = append(plan.keep, b)
				}
			case "remap":
				if i == dupFirst[n] {
					plan.keep = append(plan.keep, b)
				} else if i == dupSecond[n] {
					plan.keep = append(plan.keep, remapBind(b, ch.remap))
				}
			default: // ryoku: keep the first, drop the rest
				if i == dupFirst[n] {
					plan.keep = append(plan.keep, b)
				}
			}
		default:
			plan.keep = append(plan.keep, b)
		}
	}
	return plan
}

func remapBind(b importBind, combo string) importBind {
	b.Combo = combo
	b.Norm = normCombo(combo)
	return b
}

// parseChoice decodes a decisions.conflicts value: a bare string ("ryoku" /
// "mine") or a { "remap": "<combo>" } object.
func parseChoice(raw json.RawMessage) conflictChoice {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return conflictChoice{mode: s}
	}
	var obj struct {
		Remap string `json:"remap"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Remap != "" {
		return conflictChoice{mode: "remap", remap: obj.Remap}
	}
	return conflictChoice{}
}

// --- backup + rollback -------------------------------------------------------

func backup(dir, ts string, writes []pendingWrite) (importManifest, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return importManifest{}, err
	}
	snapshotBestEffort(ts)
	man := importManifest{Ts: ts}
	for i, w := range writes {
		mf := manifestFile{Path: w.path}
		if data, err := os.ReadFile(w.path); err == nil {
			bp := filepath.Join(dir, fmt.Sprintf("%03d-%s", i, filepath.Base(w.path)))
			if err := os.WriteFile(bp, data, 0o644); err != nil {
				return man, err
			}
			mf.Backup = bp
		}
		man.Files = append(man.Files, mf)
	}
	mb, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return man, err
	}
	return man, os.WriteFile(filepath.Join(dir, "manifest.json"), mb, 0o644)
}

func rollback(man importManifest) {
	for _, f := range man.Files {
		if f.Backup != "" {
			if data, err := os.ReadFile(f.Backup); err == nil {
				_ = atomicWrite(f.Path, data, 0o644)
			}
			continue
		}
		_ = os.Remove(f.Path) // file did not exist before the import
	}
}

// snapshotBestEffort takes a snapper snapshot when this host provides the
// snapshot stack and snapper is installed. It is a belt-and-suspenders layer
// over the per-file backup, so failures are ignored.
func snapshotBestEffort(ts string) {
	if snapshotsUnavailable() {
		return
	}
	if _, err := exec.LookPath("snapper"); err != nil {
		return
	}
	// -c root + -c number so it uses the root config and prunes under NUMBER_LIMIT
	// like update snapshots, instead of piling up untagged forever.
	_ = exec.Command("snapper", "-c", "root", "create", "-c", "number", "-d", "ryoku config import "+ts).Run()
	_ = exec.Command("snapper", "-c", "root", "cleanup", "number").Run()
}

// --- write helpers -----------------------------------------------------------

func layerWrite(app, name, prefix, ts, body string) pendingWrite {
	target := filepath.Join(configHome(), app, name)
	return pendingWrite{path: target, content: []byte(upsertBlock(readFileOr(target), ts, body, prefix))}
}

// upsertBlock replaces any prior ryoku-import block (idempotent re-import) then
// appends the new one, so a re-scan overwrites rather than stacks.
func upsertBlock(existing, ts, body, prefix string) string {
	cleaned := removeImportBlock(existing, prefix)
	block := prefix + " >>> ryoku-import " + ts + " >>>\n" + body + prefix + " <<< ryoku-import <<<\n"
	if strings.TrimSpace(cleaned) == "" {
		return block
	}
	if !strings.HasSuffix(cleaned, "\n") {
		cleaned += "\n"
	}
	return cleaned + block
}

func removeImportBlock(s, prefix string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, prefix+" >>> ryoku-import ") {
			inBlock = true
			continue
		}
		if inBlock && strings.HasPrefix(t, prefix+" <<< ryoku-import") {
			inBlock = false
			continue
		}
		if !inBlock {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func headerComment(prefix, ts, source string) string {
	return prefix + " imported " + ts + " from " + source + "\n"
}

// commentPreserve keeps a raw Hyprland line in user.lua as a valid Lua comment,
// so an untranslated setting is never lost and never breaks the file.
func commentPreserve(raw string) string { return "-- ryoku-import: " + raw }

// bindApplies reports whether an imported bind will actually take effect:
// ingestable (a GUI Keybind) or a dispatcher translateBind can render.
func bindApplies(b importBind) bool {
	if b.Ingestable {
		return true
	}
	_, ok := translateBind(b)
	return ok
}

// portByHand keeps an untranslatable bind as a valid Lua comment, naming the
// dispatcher the user must port manually; it does not take effect.
func portByHand(b importBind) string {
	return fmt.Sprintf("-- ryoku-import (port by hand: %s): %s", b.Dispatcher, b.Raw)
}

// fishBody concatenates config.fish and every functions/ + conf.d/ file into one
// user.fish body; fish sources it late, so the appended functions and snippets
// take effect on top of the shipped config.
func fishBody(dir, configPath string) string {
	parts := []string{"# --- config.fish ---\n" + ensureTrailingNL(readFileOr(configPath))}
	for _, sub := range []string{"functions", "conf.d"} {
		entries, _ := os.ReadDir(filepath.Join(dir, sub))
		names := []string{}
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			parts = append(parts, fmt.Sprintf("# --- %s/%s ---\n", sub, name)+ensureTrailingNL(readFileOr(filepath.Join(dir, sub, name))))
		}
	}
	return strings.Join(parts, "")
}

func included(dec decisions, id string) bool {
	a, ok := dec.Apps[id]
	return ok && a.Include
}

func readFileOr(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func ensureTrailingNL(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func writePaths(writes []pendingWrite) []string {
	out := make([]string, 0, len(writes))
	for _, w := range writes {
		out = append(out, w.path)
	}
	return out
}

// relPaths reports absolute paths relative to the config home, deduped and
// sorted, for the JSON summaries.
func relPaths(paths []string) []string {
	base := configHome()
	set := map[string]bool{}
	for _, p := range paths {
		r, err := filepath.Rel(base, p)
		if err != nil {
			r = p
		}
		set[r] = true
	}
	out := make([]string, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// luaContentOK is the write-time syntax gate, mirroring ryoku doctor: luac -p
// when available, else non-empty with balanced () and {} (a torn write's
// failure mode).
func luaContentOK(b []byte) bool {
	if _, err := exec.LookPath("luac"); err == nil {
		f, err := os.CreateTemp("", "ryoku-import-*.lua")
		if err == nil {
			name := f.Name()
			_, _ = f.Write(b)
			_ = f.Close()
			defer os.Remove(name)
			return exec.Command("luac", "-p", name).Run() == nil
		}
	}
	s := string(b)
	return strings.TrimSpace(s) != "" && luaBalanced(s, '(', ')') && luaBalanced(s, '{', '}')
}

func luaBalanced(s string, open, shut rune) bool {
	depth := 0
	for _, r := range s {
		switch r {
		case open:
			depth++
		case shut:
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}
