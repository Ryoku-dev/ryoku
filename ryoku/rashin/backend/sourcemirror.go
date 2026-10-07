package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// sourcemirror.go keeps a read-only copy of the live desktop config under the
// vault, indexed with prowl, so the prowl MCP server and search_code
// answer on a packaged box with no source checkout, not only on a maintainer's
// machine. Everything here is best effort: a missing prowl or a copy
// error degrades the feature and never fails the reindex.

// sourceMirrorMaxFileSize caps a mirrored file; anything larger (a wallpaper, a
// prebuilt binary) is skipped, since the index only wants source and config.
const sourceMirrorMaxFileSize = 2 << 20 // 2 MiB

const sourceMirrorReadme = "# Rashin config mirror\n" +
	"\n" +
	"This is a READ-ONLY copy of the live desktop config (`~/.config/quickshell`, the\n" +
	"active window manager's config, and `~/.config/ryoku/*.json`), kept only so `prowl`\n" +
	"can index the config on a box with no source checkout. It is rebuilt on every\n" +
	"Rashin reindex; edits here are overwritten and never reach the desktop. Edit\n" +
	"the real files (see `desktop.md`), never this mirror.\n"

// sourceMirrorDir is the vault's read-only config mirror and its prowl index.
func sourceMirrorDir() string {
	return filepath.Join(VaultDir(), "source")
}

// mirrorInput is one live config tree to mirror. glob, when set, restricts the
// copy to top-level files matching it (the ryoku dir holds more than its JSON
// stores, so only `*.json` is mirrored from it).
type mirrorInput struct {
	src  string
	dst  string
	glob string
}

func sourceMirrorInputs() []mirrorInput {
	cfg := configHome()
	inputs := []mirrorInput{
		{src: filepath.Join(cfg, "quickshell"), dst: "quickshell"},
	}
	if dir := compositorConfigDir(); dir != "" {
		inputs = append(inputs, mirrorInput{src: filepath.Join(cfg, dir), dst: dir})
	}
	inputs = append(inputs, mirrorInput{src: filepath.Join(cfg, "ryoku"), dst: "ryoku", glob: "*.json"})
	return inputs
}

// RefreshSourceMirror rebuilds the config mirror and refreshes its prowl index.
// It is a no-op when prowl is not installed, and it never returns an error
// that would fail the reindex.
func RefreshSourceMirror() error {
	if _, ok := findProwl(); !ok {
		return nil
	}
	root := sourceMirrorDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil
	}
	for _, in := range sourceMirrorInputs() {
		if !dirExists(in.src) {
			continue
		}
		mirrorTree(in, filepath.Join(root, in.dst))
	}
	_ = atomicWrite(filepath.Join(root, "README.md"), []byte(sourceMirrorReadme), 0o644)
	indexSourceMirror(root)
	return nil
}

// mirrorTree copies one input into dst: a glob input takes only its matching
// top-level files, a whole-tree input prefers rsync and falls back to a Go copy.
func mirrorTree(in mirrorInput, dst string) {
	if in.glob != "" {
		matches, _ := filepath.Glob(filepath.Join(in.src, in.glob))
		for _, m := range matches {
			copyMirrorFile(m, filepath.Join(dst, filepath.Base(m)))
		}
		return
	}
	if rsyncMirror(in.src, dst) {
		return
	}
	goCopyTree(in.src, dst)
}

// rsyncMirror mirrors src into dst with rsync when it is on PATH: no symlinks
// (no -l), the size cap, and --delete so a removed file leaves the mirror.
// Returns false when rsync is absent or fails, so the caller uses the Go copy.
func rsyncMirror(src, dst string) bool {
	rsync, err := exec.LookPath("rsync")
	if err != nil {
		return false
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sep := string(os.PathSeparator)
	cmd := exec.CommandContext(ctx, rsync, "-rt", "--max-size=2m", "--delete",
		src+sep, dst+sep)
	return cmd.Run() == nil
}

// goCopyTree is the rsync-less fallback: walk src, copy regular files under the
// size cap, and skip every symlink (WalkDir never descends one).
func goCopyTree(src, dst string) {
	_ = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			_ = os.MkdirAll(target, 0o755)
			return nil
		}
		copyMirrorFile(p, target)
		return nil
	})
}

// copyMirrorFile copies one regular file under the size cap, skipping symlinks,
// directories, and anything too large.
func copyMirrorFile(src, dst string) {
	fi, err := os.Lstat(src)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > sourceMirrorMaxFileSize {
		return
	}
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return
	}
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, in)
}

// indexSourceMirror sets up and refreshes prowl inside the mirror under one
// 120s budget. init installs Prowl's AGENTS.md block, MCP config, and skills
// into the mirror (integrations agents, agent-skills, claude, omp) and builds
// the .prowl code index. Best effort and logged: a missing prowl or a slow init
// never fails the reindex.
func indexSourceMirror(root string) {
	bin, ok := findProwl()
	if !ok {
		return
	}
	fmt.Fprintln(os.Stderr, "ryoku-rashin: indexing the config mirror with prowl")
	deadline := time.Now().Add(120 * time.Second)
	runProwlAt(root, bin, deadline, "init", "--yes", "--no-input", "--integrations", "agents,agent-skills,claude,omp")
}

func runProwlAt(dir, bin string, deadline time.Time, args ...string) {
	timeout := time.Until(deadline)
	if timeout <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	_ = cmd.Run()
}
