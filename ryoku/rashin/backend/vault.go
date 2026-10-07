package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Vault generated content lives between these markers. Reindex rewrites only
// the fenced region; anything a user or agent writes outside it survives.
const (
	vaultFenceBegin = "<!-- rashin:generated:begin -->"
	vaultFenceEnd   = "<!-- rashin:generated:end -->"
)

// The desktop map's "Bar and dock" section is fenced with its own markers
// inside the generated body, so the bar guide can be located and regenerated on
// its own. They never collide with the outer markers: the " bar" suffix sits
// before the closing "-->", so the outer begin/end are not substrings of these.
const (
	vaultBarFenceBegin = "<!-- rashin:generated:begin bar -->"
	vaultBarFenceEnd   = "<!-- rashin:generated:end bar -->"
)

// generatedFiles are the vault docs Reindex owns end to end. Everything else in
// the vault belongs to the user or an agent.
var generatedFiles = map[string]bool{
	"system.md":     true,
	"desktop.md":    true,
	"packages.md":   true,
	"ryoku-repo.md": true,
	"user.md":       true,
	"habits.md":     true,
	"ownership.md":  true,
	"logs.md":       true,
}

// AgentsTemplate is the vault entry contract. Its fenced body is refreshed on
// every EnsureVault so a template change reaches existing vaults; prose added
// outside the markers survives. It carries no machine-specific paths: the live
// map lives in the regenerated system.md and desktop.md.
const AgentsTemplate = "# Ryoku system vault\n" +
	"\n" +
	"This is the shared knowledge base for every coding agent on this machine\n" +
	"(Arch Linux, the Ryoku desktop, managed by Ryoku). Read it before exploring\n" +
	"the filesystem or guessing where things live.\n" +
	"\n" +
	"## Who you are\n" +
	"\n" +
	"A session opened here is Rashin's Ryoku lane: you are the Needle, the resident\n" +
	"agent of this machine, and every question in this lane is about the machine,\n" +
	"its desktop, or how to work on it. Be direct and technical. A \"how do I\"\n" +
	"question asks for guidance, not for you to change anything; when you do make a\n" +
	"change, say what changed and how to see or undo it. The map in `desktop.md`\n" +
	"names the window manager that runs here (Hyprland or niri): read it, never\n" +
	"assume one.\n" +
	"\n" +
	"## The one rule\n" +
	"\n" +
	"Read `desktop.md` before searching the filesystem. It maps every subsystem to\n" +
	"its config path, the binary that owns it, and how to reload it. Guessing paths\n" +
	"wastes tokens the map already spent.\n" +
	"\n" +
	"To change the desktop, use the `ryoku` skill (linked into your skills dir;\n" +
	"`desktop.md` names its path and carries the GUI map): answer \"how do I\" GUI-\n" +
	"first, act through commands, never edit shipped files. When the ask is a\n" +
	"feature the desktop does not have, climb the ladder in the skill's\n" +
	"`feature.md` before writing code: `ryostore catalog`, then\n" +
	"`ryoku-shell bar catalog`, then `ryoku plugin list`, then build a plugin per\n" +
	"`plugins.md`. Never rebuild what the store or a hidden built-in already ships.\n" +
	"\n" +
	"## What is here\n" +
	"\n" +
	"- `system.md` generated: hardware, kernel, GPU, disks, displays.\n" +
	"- `desktop.md` generated: the Ryoku map (configs, owners, reload commands).\n" +
	"- `packages.md` generated: installed package counts, the explicit set, updates.\n" +
	"- `ryoku-repo.md` generated: the Ryoku source tree map (pre-indexed, ships with the system).\n" +
	"- `user.md` generated: where this user's config diverges from the shipped baseline.\n" +
	"- `habits.md` generated: this user's directories, tool stack, and shell rhythms.\n" +
	"- `ownership.md` generated: who owns each config path and where a change belongs.\n" +
	"- `logs.md` generated: where the logs live and the one command that gathers them.\n" +
	"- `memory/` durable notes agents author and keep across sessions.\n" +
	"- `journal/` dated notes, one file per day named `YYYY-MM-DD.md`.\n" +
	"- `wiki/` shipped, read only: the plain-language guides for someone new to\n" +
	"  Linux, Ryoku, Hyprland in Lua, niri in KDL, Quickshell QML and the Go tools\n" +
	"  (`wiki/README.md` is the index). Cite the matching page when the user is new.\n" +
	"\n" +
	"## Skills, guides and the code index\n" +
	"\n" +
	"- The `ryoku` skill (linked into your skills dir) is how you change this\n" +
	"  desktop. Read the guide that matches the ask: `gui.md` (the GUI map),\n" +
	"  `bar.md` (the bar and dock), `wm.md` (the window-manager seam: `ryoku wm`,\n" +
	"  where each compositor's config is authored and where a user's change goes),\n" +
	"  `plugins.md` and `build.md` (building a widget, a plugin, or a change to the\n" +
	"  Go tools the Ryoku way), `feature.md` (the ladder before writing code), and\n" +
	"  `troubleshoot.md` (break/fix).\n" +
	"- The Ryoku source is indexed by prowl: `prowl search|find|def|outline|\n" +
	"  references` (or the `search_code` tool) answers where code lives, cited to\n" +
	"  file:line, before any grep. `ryoku-repo.md` is its map.\n" +
	"- `ryoku owner <path>` decides where an edit belongs; `ryoku-rashin logs <app>`\n" +
	"  gathers the logs; `ryoku doctor` reports and repairs drift.\n" +
	"\n" +
	"## Rules\n" +
	"\n" +
	"- The generated files are read only. Their content between the\n" +
	"  `rashin:generated` markers is overwritten on every reindex; edits there are\n" +
	"  lost. Write anything durable to `memory/` or `journal/` instead.\n" +
	"- Answer a \"how do I\" desktop question GUI-first: name the Ryoku Hub page,\n" +
	"  shell picker, or QS Bar Settings from desktop.md's GUI map (and the `ryoku`\n" +
	"  skill's `gui.md`) before the command behind it. You still act through commands.\n" +
	"- Changes listed in `user.md` are the user's own; never revert them to\n" +
	"  shipped defaults without being asked.\n" +
	"- This file's generated body is refreshed on every reindex; add your own prose\n" +
	"  outside the `rashin:generated` markers and it is kept. `CLAUDE.md` symlinks here.\n" +
	"\n" +
	"## Before you edit a file\n" +
	"\n" +
	"- Run `ryoku owner <path>` first. It says who writes the file and where a\n" +
	"  change belongs. `ownership.md` is the same rules for this whole machine.\n" +
	"- Never edit a path it calls `ryoku`, `generated`, or `store`; those are\n" +
	"  overwritten on update or owned by one writer. Edit the path it points you to\n" +
	"  instead: a user-override file, a fork under `~/.config/ryoku/user_edits/`, or\n" +
	"  the GUI or command that owns it.\n" +
	"- `user.md` lists the edits this user already made; treat them as their own.\n" +
	"\n" +
	"## When something breaks\n" +
	"\n" +
	"- Gather first, change nothing yet: run `ryoku-rashin logs <app>` and read\n" +
	"  `logs.md` for where each log lives.\n" +
	"- Check the doctor report and the recent package changes (an upgrade right\n" +
	"  before a breakage is the usual cause) before you touch anything.\n" +
	"- Prefer a reversible fix, and ask before anything destructive.\n" +
	"- Record what you found and did in `journal/YYYY-MM-DD.md`.\n"

// buildFence wraps a generated body in the vault markers, normalising trailing
// whitespace so repeated runs are byte stable.
func buildFence(gen string) string {
	return vaultFenceBegin + "\n" + strings.TrimRight(gen, "\n") + "\n" + vaultFenceEnd
}

// ReplaceFenced rewrites the region between the vault markers with gen. When the
// markers are absent (a first write) it writes the generated body first, then
// the existing document, so hand-authored preamble is preserved below the fence.
func ReplaceFenced(doc, gen string) string {
	fence := buildFence(gen)
	bi := strings.Index(doc, vaultFenceBegin)
	ei := strings.Index(doc, vaultFenceEnd)
	if bi >= 0 && ei > bi {
		before := doc[:bi]
		after := doc[ei+len(vaultFenceEnd):]
		return before + fence + after
	}
	if strings.TrimSpace(doc) == "" {
		return fence + "\n"
	}
	if !strings.HasSuffix(doc, "\n") {
		doc += "\n"
	}
	return fence + "\n\n" + doc
}

// VaultFile is one entry in the vault tree exposed over /api/vault.
type VaultFile struct {
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Mtime     time.Time `json:"mtime"`
	Generated bool      `json:"generated"`
}

// VaultTree walks the vault recursively, returning files (never directories)
// sorted by path. Symlinks (CLAUDE.md) are resolved so size and mtime reflect
// the target; broken links are skipped.
func VaultTree() ([]VaultFile, error) {
	root := VaultDir()
	var out []VaultFile
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		fi, serr := os.Stat(p) // follows symlinks
		if serr != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		rel = filepath.ToSlash(rel)
		out = append(out, VaultFile{
			Path:      rel,
			Size:      fi.Size(),
			Mtime:     fi.ModTime(),
			Generated: generatedFiles[rel] || isWikiPage(rel),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// ReadVaultFile returns a vault file's bytes, guarding against path traversal
// and absolute paths: rel must resolve inside the vault root.
func ReadVaultFile(rel string) ([]byte, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return nil, os.ErrInvalid
	}
	root := VaultDir()
	clean := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	prefix := root + string(os.PathSeparator)
	if clean != root && !strings.HasPrefix(clean, prefix) {
		return nil, os.ErrInvalid
	}
	return os.ReadFile(clean)
}

// VaultStats reports the file count, the newest generated-file mtime (the last
// reindex time), and whether the vault directory exists.
func VaultStats() (files int, lastIndexed time.Time, exists bool) {
	root := VaultDir()
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return 0, time.Time{}, false
	}
	exists = true
	tree, err := VaultTree()
	if err != nil {
		return 0, time.Time{}, true
	}
	files = len(tree)
	for _, f := range tree {
		if f.Generated && f.Mtime.After(lastIndexed) {
			lastIndexed = f.Mtime
		}
	}
	return files, lastIndexed, exists
}

// The AGENTS.md contract shipped before the fence was written once and never
// refreshed, and it grew variants (the habits.md row arrived later). Fencing an
// existing vault would otherwise preserve a stale copy below the markers, so
// every agent on that box reads two contradictory contracts. EnsureVault
// strips the legacy block: from its title to its final line, which is stable
// across the variants. Prose a user or agent added survives.
const (
	legacyAgentsTitle = "# Ryoku system vault"
	legacyAgentsTail  = "- This file (AGENTS.md) is yours to extend. `CLAUDE.md` is a symlink to it."
)

// stripLegacyAgents removes the old write-once contract from a document's
// out-of-fence prose and collapses the blank lines it leaves behind. Only the
// region after the generated fence is scanned: the live template carries the
// same title, so a blind first-match strip would eat the fence itself.
func stripLegacyAgents(doc string) string {
	ei := strings.Index(doc, vaultFenceEnd)
	head, tail := "", doc
	if ei >= 0 {
		head, tail = doc[:ei+len(vaultFenceEnd)], doc[ei+len(vaultFenceEnd):]
	}
	bi := strings.Index(tail, legacyAgentsTitle)
	if bi < 0 {
		return doc
	}
	li := strings.Index(tail[bi:], legacyAgentsTail)
	if li < 0 {
		return doc
	}
	tail = tail[:bi] + tail[bi+li+len(legacyAgentsTail):]
	tail = strings.ReplaceAll(strings.TrimRight(tail, "\n"), "\n\n\n", "\n\n")
	if strings.TrimSpace(tail) == "" {
		return head + "\n"
	}
	return head + "\n" + tail
}

// EnsureVault creates the vault skeleton: the root, memory/ and journal/, the
// AGENTS.md contract, and the CLAUDE.md symlink Claude Code reads.
func EnsureVault() error {
	root := VaultDir()
	for _, d := range []string{root, filepath.Join(root, "memory"), filepath.Join(root, "journal")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	// AGENTS.md must track AgentsTemplate: a box provisioned before a template
	// change would otherwise keep the stale contract forever, since the file was
	// once written only when absent. Fence the template so its body refreshes on
	// every run while prose a user or agent added outside the markers survives.
	agents := filepath.Join(root, "AGENTS.md")
	fenced := ReplaceFenced(stripLegacyAgents(readFileOrEmpty(agents)), AgentsTemplate)
	if err := atomicWrite(agents, []byte(fenced), 0o644); err != nil {
		return err
	}
	claude := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Lstat(claude); os.IsNotExist(err) {
		if err := os.Symlink("AGENTS.md", claude); err != nil {
			return err
		}
	}
	return syncWiki(root)
}

// atomicWrite writes via a temp file in the same directory then renames, so a
// reader never sees a partial file.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
