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
}

var definitions = []definition{
	{id: "claude", name: "Claude Code", commands: [][]string{{"claude-code-acp"}, {"claude-agent-acp"}}, installURL: "https://github.com/zed-industries/claude-agent-acp"},
	{id: "codex", name: "Codex", commands: [][]string{{"codex-acp"}}, installURL: "https://github.com/zed-industries/codex-acp"},
	{id: "opencode", name: "OpenCode", commands: [][]string{{"opencode", "acp"}}, installURL: "https://opencode.ai/docs/acp"},
	{id: "gemini", name: "Gemini CLI", commands: [][]string{{"gemini", "--experimental-acp"}}, installURL: "https://github.com/google-gemini/gemini-cli"},
	{id: "qwen", name: "Qwen Code", commands: [][]string{{"qwen", "--experimental-acp"}}, installURL: "https://github.com/QwenLM/qwen-code"},
	{id: "hermes", name: "Hermes", commands: [][]string{{"hermes", "acp"}}, installURL: "https://github.com/NousResearch/hermes-agent"},
	{id: "omp", name: "Oh My Pi", commands: [][]string{{"omp", "acp"}}, installURL: "https://github.com/can1357/oh-my-pi"},
	{id: "goose", name: "Goose", commands: [][]string{{"goose", "acp"}}, installURL: "https://block.github.io/goose/docs/guides/acp-clients"},
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
}

var _ core.Harnesses = (*Registry)(nil)

// New creates an empty registry. Discovery does not run until List or NewAgent.
func New() *Registry {
	return &Registry{versions: make(map[string]versionEntry)}
}

func (r *Registry) List(ctx context.Context, refresh bool) ([]protocol.HarnessInfo, error) {
	settings, err := config.Load()
	if err != nil {
		return nil, err
	}
	out := make([]protocol.HarnessInfo, 0, len(definitions)+len(settings.Agent.Instances))
	builtins := make(map[string]struct{}, len(definitions))
	for _, def := range definitions {
		builtins[def.id] = struct{}{}
		info := protocol.HarnessInfo{ID: def.id, Name: def.name, Install: def.installURL}
		argv, path, ok := resolveDefinition(def)
		if ok {
			info.Installed = true
			info.Path = path
			info.Command = append([]string{path}, argv[1:]...)
			info.Version = r.version(ctx, path, refresh)
			info.Capabilities = r.capabilitiesFor(path)
			info.Setup = setupOf(def.id)
		}
		out = append(out, info)
	}
	for _, instance := range settings.Agent.Instances {
		id := strings.TrimSpace(instance.ID)
		name := strings.TrimSpace(instance.Name)
		if name == "" {
			name = id
		}
		info := protocol.HarnessInfo{ID: id, Name: name, Custom: true}
		if _, collides := builtins[id]; collides {
			info.Install = i18n.T("This custom harness id conflicts with a built-in harness. Choose a different id.")
			out = append(out, info)
			continue
		}
		argv, path, ok := resolveCustom(instance.Command)
		if ok {
			info.Installed = true
			info.Path = path
			info.Command = argv
			info.Version = r.version(ctx, path, refresh)
			info.Capabilities = r.capabilitiesFor(path)
		} else {
			info.Install = i18n.T("The custom harness command is not executable. Check its command and PATH.")
		}
		out = append(out, info)
	}
	chosen := settings.Agent.Harness
	if !installed(out, chosen) {
		chosen = ""
		for _, info := range out {
			if info.Installed {
				chosen = info.ID
				break
			}
		}
	}
	for i := range out {
		out[i].Default = out[i].ID == chosen && chosen != ""
	}
	return out, nil
}

func (r *Registry) NewAgent(ctx context.Context, opts core.HarnessOptions) (core.Runner, error) {
	list, err := r.List(ctx, false)
	if err != nil {
		return nil, err
	}
	want := strings.TrimSpace(opts.Harness)
	if want == "" {
		for _, info := range list {
			if info.Default {
				want = info.ID
				break
			}
		}
	}
	for _, info := range list {
		if info.ID != want {
			continue
		}
		if !info.Installed {
			return nil, errors.New(i18n.T("The selected agent harness is not installed. Install it and try again."))
		}
		options := []runnerOption{withCapabilityReporter(func(capabilities protocol.HarnessCapabilities) {
			r.recordCapabilities(info.Path, capabilities)
		})}
		if info.Custom {
			settings, loadErr := config.Load()
			if loadErr != nil {
				return nil, loadErr
			}
			for _, instance := range settings.Agent.Instances {
				if strings.TrimSpace(instance.ID) == want {
					options = append(options, withEnvironment(instance.Env))
					break
				}
			}
		}
		return newRunner(opts, info.Command, options...), nil
	}
	if want == "" {
		return nil, errors.New(i18n.T("No supported agent harness is installed. Install one and try again."))
	}
	return nil, errors.New(i18n.T("The selected agent harness is not supported. Choose a harness from the list."))
}

func installed(list []protocol.HarnessInfo, id string) bool {
	if id == "" {
		return false
	}
	for _, info := range list {
		if info.ID == id {
			return info.Installed
		}
	}
	return false
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
