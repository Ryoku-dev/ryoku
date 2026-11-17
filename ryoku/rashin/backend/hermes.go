package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// HermesInfo is the resident agent's install and wiring state for the dashboard.
type HermesInfo struct {
	Installed  bool   `json:"installed"`
	Version    string `json:"version"`
	Configured bool   `json:"configured"`
	Wired      bool   `json:"wired"`
	// SkillWired is true when ~/.hermes/skills/ryoku links the shipped skill.
	SkillWired bool   `json:"skillWired"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
}

// hermesMemory is the file the vault pointer block is wired into.
func hermesMemory() string {
	return filepath.Join(home(), ".hermes", "memories", "MEMORY.md")
}

func hermesConfig() string {
	return filepath.Join(home(), ".hermes", "config.yaml")
}

// FindHermes resolves the hermes binary: PATH first, then the two locations its
// installer uses.
func FindHermes() (string, bool) {
	if p, err := exec.LookPath("hermes"); err == nil {
		return p, true
	}
	for _, cand := range []string{
		filepath.Join(home(), ".hermes", "bin", "hermes"),
		filepath.Join(home(), ".local", "bin", "hermes"),
	} {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand, true
		}
	}
	return "", false
}

// HermesStatus reports install, version, config, wiring, and the chosen model,
// best effort: a missing or slow hermes never blocks the caller.
func HermesStatus() HermesInfo {
	info := HermesInfo{}
	bin, ok := FindHermes()
	info.Installed = ok
	if ok {
		info.Version = hermesVersion(bin)
	}
	info.Configured = hermesOnboarded()
	info.Wired = fileHasBlock(hermesMemory())
	info.SkillWired = isOurSkillLink(filepath.Join(home(), ".hermes", "skills", "ryoku"))
	info.Provider, info.Model, _ = hermesModel()
	if picked := modelPick("hermes"); picked != "" {
		if provider, model, ok := strings.Cut(picked, ":"); ok && provider != "" && model != "" {
			info.Provider = provider
			info.Model = model
		}
	}
	return info
}

// hermesOnboarded reports whether the user finished hermes's own onboarding.
// The installer lays a template config.yaml with an EMPTY `model:` line, so
// file existence alone is a false positive; a chosen model is the artifact of
// the user actually completing `hermes setup`. Two config shapes exist:
// a scalar (`model: openrouter/x`) and a mapping (`model:` with indented
// `provider:`/`default:` keys).
func hermesOnboarded() bool {
	_, _, ok := hermesModel()
	return ok
}

// hermesModel reads the chosen provider and model from config.yaml without a
// yaml dependency: the model block is flat enough for line parsing.
func hermesModel() (provider, model string, ok bool) {
	b, err := os.ReadFile(hermesConfig())
	if err != nil {
		return "", "", false
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), "model:")
		if !found || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if v := strings.TrimSpace(rest); v != "" {
			return "", strings.Trim(v, `"'`), true // scalar form
		}
		for _, sub := range lines[i+1:] {
			if strings.TrimSpace(sub) == "" {
				continue
			}
			if !strings.HasPrefix(sub, " ") && !strings.HasPrefix(sub, "\t") {
				break // mapping block ended
			}
			t := strings.TrimSpace(sub)
			if v, is := strings.CutPrefix(t, "provider:"); is {
				provider = strings.Trim(strings.TrimSpace(v), `"'`)
			}
			if v, is := strings.CutPrefix(t, "default:"); is {
				model = strings.Trim(strings.TrimSpace(v), `"'`)
			}
		}
		return provider, model, provider != "" || model != ""
	}
	return "", "", false
}

func hermesVersion(bin string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(firstLine(string(out)))
	// Real output: "Hermes Agent v0.18.0 (2026.7.1) · upstream 88d1d620".
	// Pick the first token that looks like a version number.
	for _, f := range strings.Fields(line) {
		v := strings.TrimPrefix(f, "v")
		if len(v) > 0 && v[0] >= '0' && v[0] <= '9' && strings.Contains(v, ".") {
			return v
		}
	}
	return line
}

// renameHermesSession retitles through the hermes CLI, which owns state.db;
// Rashin only ever reads it.
func renameHermesSession(id, title string) error {
	bin, ok := FindHermes()
	if !ok {
		return errors.New("hermes is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "sessions", "rename", "--", id, title).CombinedOutput()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	// hermes exits 0 for a missing session or a rejected title, so read what it printed
	if err == nil && strings.HasPrefix(last, "Session '") && strings.Contains(last, "' renamed to: ") {
		return nil
	}
	if last == "" && err != nil {
		last = err.Error()
	}
	if last == "" {
		last = "hermes did not rename the session"
	}
	return errors.New(last)
}

// WireHermesMemory upserts the pointer block into Hermes's MEMORY.md, creating
// memories/ only when ~/.hermes already exists. It never installs Hermes.
func WireHermesMemory() error {
	if !dirExists(filepath.Join(home(), ".hermes")) {
		return errors.New("hermes not installed")
	}
	file := hermesMemory()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	doc := readFileOrEmpty(file)
	return atomicWrite(file, []byte(upsertBlock(doc)), 0o644)
}
