// Package harness discovers ACP agents and drives their sessions.
package harness

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ryoku-i18n"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

type definition struct {
	id         string
	name       string
	commands   [][]string
	installURL string
	// state is what the harness writes under its HOME, relative to it: the
	// only places beside its folder and cache it may write in a sandbox
	state []string
}

var definitions = []definition{
	{id: "claude", name: "Claude Code", commands: [][]string{{"claude-code-acp"}, {"claude-agent-acp"}}, installURL: "https://github.com/zed-industries/claude-agent-acp", state: []string{".claude", ".claude.json"}},
	{id: "codex", name: "Codex", commands: [][]string{{"codex-acp"}}, installURL: "https://github.com/zed-industries/codex-acp", state: []string{".codex"}},
	{id: "opencode", name: "OpenCode", commands: [][]string{{"opencode", "acp"}}, installURL: "https://opencode.ai/docs/acp", state: []string{".local/share/opencode", ".config/opencode", ".local/state/opencode"}},
	{id: "gemini", name: "Gemini CLI", commands: [][]string{{"gemini", "--experimental-acp"}}, installURL: "https://github.com/google-gemini/gemini-cli", state: []string{".gemini"}},
	{id: "qwen", name: "Qwen Code", commands: [][]string{{"qwen", "--experimental-acp"}}, installURL: "https://github.com/QwenLM/qwen-code", state: []string{".qwen"}},
	{id: "hermes", name: "Hermes", commands: [][]string{{"hermes", "acp"}}, installURL: "https://github.com/NousResearch/hermes-agent", state: []string{".hermes"}},
	{id: "omp", name: "Oh My Pi", commands: [][]string{{"omp", "acp"}}, installURL: "https://github.com/can1357/oh-my-pi", state: []string{".omp"}},
	{id: "goose", name: "Goose", commands: [][]string{{"goose", "acp"}}, installURL: "https://block.github.io/goose/docs/guides/acp-clients", state: []string{".config/goose", ".local/share/goose", ".local/state/goose"}},
}

type versionEntry struct {
	mtime   time.Time
	size    int64
	version string
}

// Registry discovers installed harnesses and creates lazy runners.
type Registry struct {
	mu                 sync.Mutex
	versions           map[string]versionEntry
	capabilities       map[string]capabilityEntry
	capabilitiesLoaded bool
	// learned runs after capabilities are recorded
	learned func()
	// probing are the harness ids a probe runs for now
	probing map[string]bool
}

var _ core.Harnesses = (*Registry)(nil)

// New creates an empty registry. Discovery does not run until List or NewAgent.
func New() *Registry {
	return &Registry{versions: make(map[string]versionEntry), probing: make(map[string]bool)}
}

// candidate is one harness as the definitions, the settings and PATH make
// it, before its version and capabilities are read.
type candidate struct {
	id, name string
	install  string
	custom   bool
	// collides marks a custom instance that took a built-in's id
	collides  bool
	installed bool
	path      string
	command   []string
	env       map[string]string
}

// candidates lists the built-in harnesses in table order, then the user's
// instances, looking each up on PATH and running nothing.
func candidates(settings protocol.Settings) []candidate {
	out := make([]candidate, 0, len(definitions)+len(settings.Agent.Instances))
	for _, def := range definitions {
		c := candidate{id: def.id, name: def.name, install: def.installURL}
		if argv, path, ok := resolveDefinition(def); ok {
			c.installed, c.path = true, path
			c.command = append([]string{path}, argv[1:]...)
		}
		out = append(out, c)
	}
	for _, instance := range settings.Agent.Instances {
		c := candidate{id: strings.TrimSpace(instance.ID), name: strings.TrimSpace(instance.Name), custom: true, env: instance.Env}
		if c.name == "" {
			c.name = c.id
		}
		c.collides = builtin(c.id)
		if !c.collides {
			if argv, path, ok := resolveCustom(instance.Command); ok {
				c.installed, c.path, c.command = true, path, argv
			}
		}
		out = append(out, c)
	}
	return out
}

// defaultOf is the harness a session with none of its own runs: the chosen
// one when it is installed, else the first installed in list order.
func defaultOf(list []candidate, chosen string) string {
	first := ""
	for _, c := range list {
		if c.collides || !c.installed {
			continue
		}
		if c.id == chosen {
			return c.id
		}
		if first == "" {
			first = c.id
		}
	}
	return first
}

func builtin(id string) bool {
	for _, def := range definitions {
		if def.id == id {
			return true
		}
	}
	return false
}

// Known reports whether id names a harness on this computer, installed or
// not: a built-in definition or one of the user's instances.
func Known(id string, settings protocol.Settings) bool {
	if id == "" {
		return false
	}
	if builtin(id) {
		return true
	}
	for _, instance := range settings.Agent.Instances {
		if strings.TrimSpace(instance.ID) == id {
			return true
		}
	}
	return false
}

// DefaultID is the harness List marks Default, found without running any
// harness; empty when none is installed.
func DefaultID(settings protocol.Settings) string {
	return defaultOf(candidates(settings), settings.Agent.Harness)
}

// Name is how a harness id reads: its definition's name, else its instance's
// name or id, else the id itself.
func Name(id string, settings protocol.Settings) string {
	for _, def := range definitions {
		if def.id == id {
			return def.name
		}
	}
	for _, instance := range settings.Agent.Instances {
		if strings.TrimSpace(instance.ID) != id {
			continue
		}
		if name := strings.TrimSpace(instance.Name); name != "" {
			return name
		}
		return id
	}
	return id
}

func (r *Registry) List(ctx context.Context, refresh bool) ([]protocol.HarnessInfo, error) {
	settings, err := config.Load()
	if err != nil {
		return nil, err
	}
	found := candidates(settings)
	chosen := defaultOf(found, settings.Agent.Harness)
	out := make([]protocol.HarnessInfo, 0, len(found))
	for _, c := range found {
		info := protocol.HarnessInfo{ID: c.id, Name: c.name, Install: c.install, Custom: c.custom, Default: !c.collides && c.id == chosen && chosen != ""}
		switch {
		case c.collides:
			info.Install = i18n.T("This custom harness id conflicts with a built-in harness. Choose a different id.")
		case c.installed:
			info.Installed = true
			info.Path = c.path
			info.Command = c.command
			info.Version = r.version(ctx, c.path, refresh)
			info.Capabilities = r.capabilitiesFor(c.id, c.path)
			if !c.custom {
				info.Setup = setupOf(c.id)
			}
		case c.custom:
			info.Install = i18n.T("The custom harness command is not executable. Check its command and PATH.")
		}
		out = append(out, info)
	}
	return out, nil
}

func (r *Registry) NewAgent(_ context.Context, opts core.HarnessOptions) (core.Runner, error) {
	settings, err := config.Load()
	if err != nil {
		return nil, err
	}
	found := candidates(settings)
	want := strings.TrimSpace(opts.Harness)
	if want == "" {
		want = defaultOf(found, settings.Agent.Harness)
	}
	for _, c := range found {
		if c.id != want || c.collides {
			continue
		}
		if !c.installed {
			return nil, errors.New(i18n.T("The selected agent harness is not installed. Install it and try again."))
		}
		id, path := c.id, c.path
		options := []runnerOption{withCapabilityReporter(func(capabilities protocol.HarnessCapabilities) {
			r.recordCapabilities(id, path, capabilities)
		})}
		if c.custom {
			options = append(options, withEnvironment(c.env))
		}
		if opts.Sandbox != nil {
			spec, sandboxErr := sandboxFor(*opts.Sandbox, stateOf(c.id, c.command), c.env)
			if sandboxErr != nil {
				return nil, sandboxErr
			}
			options = append(options, withSandbox(spec))
		}
		return newRunner(opts, c.command, options...), nil
	}
	if want == "" {
		return nil, errors.New(i18n.T("No supported agent harness is installed. Install one and try again."))
	}
	return nil, errors.New(i18n.T("The selected agent harness is not supported. Choose a harness from the list."))
}

func resolveDefinition(def definition) ([]string, string, bool) {
	for _, candidate := range def.commands {
		path, err := lookPath(def.id, candidate[0])
		if err != nil {
			continue
		}
		return candidate, path, true
	}
	return nil, "", false
}

func resolveCustom(command []string) ([]string, string, bool) {
	if len(command) == 0 {
		return nil, "", false
	}
	binary := strings.TrimSpace(command[0])
	if binary == "" || (!filepath.IsAbs(binary) && strings.ContainsRune(binary, filepath.Separator)) {
		return nil, "", false
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, "", false
	}
	path, err = absolutePath(path)
	if err != nil {
		return nil, "", false
	}
	argv := append([]string(nil), command...)
	argv[0] = path
	return argv, path, true
}

func lookPath(id, binary string) (string, error) {
	path, err := exec.LookPath(binary)
	if err == nil {
		return absolutePath(path)
	}
	if id != "hermes" {
		return "", err
	}
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return "", err
	}
	for _, dir := range []string{filepath.Join(home, ".hermes", "bin"), filepath.Join(home, ".local", "bin")} {
		candidate := filepath.Join(dir, binary)
		if st, statErr := os.Stat(candidate); statErr == nil && st.Mode().IsRegular() && st.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", err
}

func absolutePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Abs(path)
}

func (r *Registry) version(ctx context.Context, path string, _ bool) string {
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	r.mu.Lock()
	cached, ok := r.versions[path]
	r.mu.Unlock()
	if ok && cached.mtime.Equal(st.ModTime()) && cached.size == st.Size() {
		return cached.version
	}
	versionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, _ := exec.CommandContext(versionCtx, path, "--version").CombinedOutput()
	version := strings.TrimSpace(string(output))
	if cut := strings.IndexByte(version, '\n'); cut >= 0 {
		version = strings.TrimSpace(version[:cut])
	}
	r.mu.Lock()
	r.versions[path] = versionEntry{mtime: st.ModTime(), size: st.Size(), version: version}
	r.mu.Unlock()
	return version
}
