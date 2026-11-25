package localmodels

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/protocol"
)

type discoveryOptions struct {
	home     func() string
	getenv   func(string) string
	glob     func(string) ([]string, error)
	realpath func(string) (string, error)
	stat     func(string) (os.FileInfo, error)
	shell    func(context.Context, string) (string, error)
	loginMu  sync.Mutex
	loginKey string
	login    string
}

func defaultDiscoveryOptions() discoveryOptions {
	return discoveryOptions{home: func() string { home, _ := os.UserHomeDir(); return home },
		getenv: os.Getenv, glob: filepath.Glob, realpath: filepath.EvalSymlinks, stat: os.Stat, shell: loginPath}
}

func loginPath(ctx context.Context, shell string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	pathCommand := `printf %s "$PATH"`
	if filepath.Base(shell) == "fish" {
		pathCommand = "string join : $PATH"
	}
	out, err := exec.CommandContext(ctx, shell, "-lc", pathCommand).Output()
	return string(out), err
}

type candidate struct{ path, source string }

func (d *discoveryOptions) loginPATH(ctx context.Context) string {
	shell := d.getenv("SHELL")
	if shell == "" {
		return ""
	}
	d.loginMu.Lock()
	defer d.loginMu.Unlock()
	if d.loginKey == shell {
		return d.login
	}
	d.loginKey = shell
	d.login, _ = d.shell(ctx, shell)
	return d.login
}

func executableInPATH(value string) []string {
	var paths []string
	for _, dir := range filepath.SplitList(value) {
		if dir == "" {
			continue
		}
		paths = append(paths, filepath.Join(dir, "llama-server"))
	}
	return paths
}

func (m *Manager) candidates(ctx context.Context) []candidate {
	d := &m.discovery
	var all []candidate
	appendCandidate := func(path, source string) {
		if path != "" {
			all = append(all, candidate{path: path, source: source})
		}
	}
	appendCandidate(m.settings().Providers.Models.Binary, protocol.RuntimeConfigured)
	if path, _, ok := managedRuntime(); ok {
		appendCandidate(path, protocol.RuntimeManaged)
	}
	for _, path := range executableInPATH(d.getenv("PATH")) {
		appendCandidate(path, protocol.RuntimeSystem)
	}
	for _, path := range executableInPATH(d.loginPATH(ctx)) {
		appendCandidate(path, protocol.RuntimeLogin)
	}
	home := d.home()
	user := d.getenv("USER")
	if user == "" {
		user = filepath.Base(home)
	}
	known := []string{
		filepath.Join(home, ".local/bin"),
		filepath.Join(home, ".linuxbrew/bin"),
		"/home/linuxbrew/.linuxbrew/bin",
		filepath.Join(home, ".nix-profile/bin"),
		"/run/current-system/sw/bin",
		filepath.Join("/etc/profiles/per-user", user, "bin"),
		"/usr/local/bin", "/usr/bin",
	}
	for _, pattern := range []string{
		filepath.Join(home, "llama.cpp/build*/bin"),
		filepath.Join(home, "llama.cpp*/build*/bin"),
		filepath.Join(home, "*/llama.cpp/build*/bin"),
		filepath.Join(home, "src/llama.cpp/build*/bin"),
		filepath.Join(home, ".local/share/*/llama.cpp*"),
		filepath.Join(home, ".local/share/*/llama.cpp*/bin"),
		filepath.Join(home, ".local/share/*/llama.cpp*/build*/bin"),
		"/opt/*/bin",
	} {
		matches, _ := d.glob(pattern)
		known = append(known, matches...)
	}
	for _, dir := range known {
		appendCandidate(filepath.Join(dir, "llama-server"), protocol.RuntimeFound)
	}
	seen := make(map[string]bool, len(all))
	out := make([]candidate, 0, len(all))
	for _, item := range all {
		if !filepath.IsAbs(item.path) {
			if abs, err := filepath.Abs(item.path); err == nil {
				item.path = abs
			}
		}
		canonical, err := d.realpath(item.path)
		if err != nil && item.source != protocol.RuntimeConfigured {
			continue
		}
		if err == nil {
			item.path = canonical
		}
		if !seen[item.path] {
			seen[item.path] = true
			out = append(out, item)
		}
	}
	return out
}

func (m *Manager) discover(ctx context.Context) []binary {
	all := m.candidates(ctx)
	found := make([]binary, 0, len(all))
	for _, item := range all {
		if ctx.Err() != nil {
			break
		}
		found = append(found, m.probe(ctx, item.path, item.source))
	}
	// A user's explicit choice wins; otherwise prefer the managed runtime,
	// then router-capable GPU builds, CPU routers, and finally plain servers.
	sort.SliceStable(found, func(i, j int) bool { return binaryRank(found[i]) > binaryRank(found[j]) })
	return found
}

func binaryRank(b binary) int {
	if b.Source == protocol.RuntimeConfigured {
		return 100
	}
	if b.Err != "" {
		return -1
	}
	if b.Source == protocol.RuntimeManaged {
		return 90
	}
	if b.Router && b.Backend != "" && b.Backend != protocol.BackendCPU {
		return 80
	}
	if b.Router {
		return 70
	}
	if b.Backend != "" && b.Backend != protocol.BackendCPU {
		return 60
	}
	return 50
}

func runtimeBinaries(all []binary) []protocol.LocalRuntimeBinary {
	out := make([]protocol.LocalRuntimeBinary, 0, len(all))
	for _, b := range all {
		caps := make([]string, 0, len(b.Flags))
		for flag := range b.Flags {
			caps = append(caps, flag)
		}
		sort.Strings(caps)
		out = append(out, protocol.LocalRuntimeBinary{Path: b.Path, Source: binaryLabel(b), Version: b.Version,
			Build: b.Build, Backend: b.Backend, Router: b.Router, Capabilities: caps, Broken: b.Err})
	}
	return out
}

func binaryLabel(b binary) string {
	if b.Source != protocol.RuntimeFound {
		return b.Source
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(b.Path, home+string(filepath.Separator)) {
		return "~/" + strings.TrimPrefix(filepath.Dir(b.Path), home+string(filepath.Separator))
	}
	return filepath.Dir(b.Path)
}
