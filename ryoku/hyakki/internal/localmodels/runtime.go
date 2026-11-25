package localmodels

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// The llama.cpp build Rashin installs, and its Linux x64 tarballs as the
// release lists them (asset digests from api.github.com, 2026-11-18).
const (
	runtimeBuild       = 12521
	runtimeReleaseBase = "https://github.com/ggml-org/llama.cpp/releases/download"
)

// runtimeAsset is one pinned upstream tarball.
type runtimeAsset struct {
	Name   string
	Size   int64
	SHA256 string
}

var runtimeAssets = map[string]runtimeAsset{
	protocol.BackendVulkan: {Name: "llama-b12521-bin-ubuntu-vulkan-x64.tar.gz", Size: 36925213, SHA256: "0278acdb4b12e24f90002039c011038e824d308b9ae5971433b284b1b3b5fc97"},
	protocol.BackendCPU:    {Name: "llama-b12521-bin-ubuntu-x64.tar.gz", Size: 23852655, SHA256: "ab8343cb39e17a18f183ce0a35543a6c3dcc15fb287f55909dd182cb975e345e"},
}

// runtimeRecord is llama.cpp/runtime.json: the managed runtime in use.
// SHA256 is the tarball's; an adopted folder Rashin did not unpack has none.
type runtimeRecord struct {
	Build     int       `json:"build"`
	Backend   string    `json:"backend"`
	Dir       string    `json:"dir"`
	Asset     string    `json:"asset,omitempty"`
	SHA256    string    `json:"sha256,omitempty"`
	Installed time.Time `json:"installed"`
}

func runtimeRecordPath() string { return filepath.Join(paths.LlamaRuntimeDir(), "runtime.json") }

func buildDir(build int) string { return "llama-b" + strconv.Itoa(build) }

var buildDirPattern = regexp.MustCompile(`^llama-b(\d+)$`)

// binary is one llama-server with capabilities taken from its --help output.
type binary struct {
	Path    string
	Source  string
	Build   int
	Version string
	Semver  [3]int
	Backend string
	Flags   map[string]bool
	Router  bool
	Err     string
}

func (b binary) meets() bool {
	return b.Err == "" && b.Flags["--host"] && b.Flags["--port"] &&
		(b.Flags["-m"] || b.Flags["--model"])
}

var (
	buildPattern     = regexp.MustCompile(`build (\d+)`)
	oldBuildPattern  = regexp.MustCompile(`version: (\d+) \(`)
	semverPattern    = regexp.MustCompile(`version: (\d+)\.(\d+)\.(\d+)`)
	versionLinePrefx = "version:"
)

// parseVersion reads llama-server --version: "version: 1.0.0-dev (build
// 12521, commit 6f311d7ad)" now, "version: 6153 (abc1234)" before 1.0.
func parseVersion(out string) (build int, version string, semver [3]int, ok bool) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, versionLinePrefx) {
			version = strings.TrimSpace(strings.TrimPrefix(line, versionLinePrefx))
			break
		}
	}
	if m := buildPattern.FindStringSubmatch(out); m != nil {
		build, _ = strconv.Atoi(m[1])
	} else if m := oldBuildPattern.FindStringSubmatch(out); m != nil {
		build, _ = strconv.Atoi(m[1])
	}
	if m := semverPattern.FindStringSubmatch(out); m != nil {
		for i := range semver {
			semver[i], _ = strconv.Atoi(m[i+1])
		}
	}
	return build, version, semver, build > 0 || semver != [3]int{}
}

// runQuick runs a short llama-server query (--version, --list-devices) in
// its own process group, killed whole when wait passes.
func runQuick(ctx context.Context, bin string, wait time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = filepath.Dir(bin)
	cmd.Env = childEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// childEnv is rashind's environment for llama.cpp: no proxies (it only ever
// talks to loopback), no LLAMA_ARG_ overrides, and a private LLAMA_CACHE so
// nothing lands in the user's ~/.cache/llama.cpp.
func childEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(key) {
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "LLAMA_CACHE", "LLAMA_API_KEY":
			continue
		}
		if strings.HasPrefix(key, "LLAMA_ARG_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "LLAMA_CACHE="+paths.LlamaCache())
}

// probeKey names a binary file as it is now: a replaced file probes again.
func probeKey(path string) (string, bool) {
	return probeKeyWithStat(path, os.Stat)
}

func probeKeyWithStat(path string, stat func(string) (os.FileInfo, error)) (string, bool) {
	info, err := stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano()), true
}

// probe caches help and version probes against the resolved file's identity.
func (m *Manager) probe(ctx context.Context, path, source string) binary {
	b := binary{Path: path, Source: source}
	key, ok := probeKeyWithStat(path, m.discovery.stat)
	if !ok {
		b.Err = i18n.Tf("%s is not a readable llama-server binary.", path)
		return b
	}
	m.mu.Lock()
	cached, hit := m.bins[key]
	m.mu.Unlock()
	if hit {
		cached.Source = source
		return cached
	}
	help, err := runQuick(ctx, path, 5*time.Second, "--help")
	if err != nil {
		b.Err = i18n.Tf("%s could not run: %s", path, probeFailure(help, err))
	} else {
		b.Flags = helpFlags(help)
		b.Router = b.Flags["--models-preset"]
		if !b.meets() {
			b.Err = i18n.Tf("%s does not support llama-server model, host and port options.", path)
		}
		if !b.Flags["--api-key-file"] && !b.Flags["--api-key"] {
			b.Err = i18n.Tf("%s cannot require an API key for Hyakki's local server.", path)
		}
	}
	if b.Err == "" {
		out, err := runQuick(ctx, path, 3*time.Second, "--version")
		b.Build, b.Version, b.Semver, _ = parseVersion(out)
		if b.Version == "" {
			b.Version = strings.TrimSpace(out)
		}
		if err != nil && b.Version == "" {
			b.Version = i18n.T("Version not reported")
		}
	}
	b.Backend = backendBeside(path)
	if b.Err == "" && b.Backend == "" && b.Flags["--list-devices"] {
		if devices, err := runQuick(ctx, path, 3*time.Second, "--list-devices"); err == nil {
			for _, dev := range parseDevices(devices) {
				if strings.HasPrefix(strings.ToLower(dev.ID), "cuda") {
					b.Backend = protocol.BackendCUDA
					break
				}
				if strings.HasPrefix(strings.ToLower(dev.ID), "vulkan") {
					b.Backend = protocol.BackendVulkan
				}
			}
		}
	}
	if ctx.Err() != nil {
		return b
	}
	m.mu.Lock()
	m.bins[key] = b
	m.mu.Unlock()
	return b
}

func probeFailure(out string, err error) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return err.Error()
	}
	if len(out) > 400 {
		out = out[:400]
	}
	return out
}

var helpFlagPattern = regexp.MustCompile(`^--?[A-Za-z][A-Za-z0-9-]*$`)

func helpFlags(out string) map[string]bool {
	flags := make(map[string]bool)
	for _, word := range strings.Fields(out) {
		word = strings.Trim(word, "[](),:;|")
		flag, _, _ := strings.Cut(word, "=")
		if helpFlagPattern.MatchString(flag) {
			flags[flag] = true
		}
	}
	// older builds take -fa as a bare switch; only a valued one accepts auto
	if strings.Contains(out, "--flash-attn [on|off|auto]") {
		flags[flashAttnValued] = true
	}
	return flags
}

// flashAttnValued marks a binary whose --flash-attn takes on, off or auto.
const flashAttnValued = "flash-attn=valued"

// flashAttnAuto reports whether flash-attn = auto may be passed: unknown
// flags (no probe yet) keep the current upstream default.
func flashAttnAuto(flags map[string]bool) bool {
	return flags == nil || flags[flashAttnValued]
}

// backendBeside reads the GPU backend from the ggml modules next to a
// binary: upstream tarballs carry them there. "" when none is.
func backendBeside(bin string) string {
	dir := filepath.Dir(bin)
	for _, try := range []struct{ lib, backend string }{
		{"libggml-cuda.so", protocol.BackendCUDA},
		{"libggml-vulkan.so", protocol.BackendVulkan},
	} {
		if _, err := os.Stat(filepath.Join(dir, try.lib)); err == nil {
			return try.backend
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "libggml-cpu-x64.so")); err == nil {
		return protocol.BackendCPU
	}
	return ""
}

// managedRuntime is the managed llama-server, adopting a llama-b<build>
// folder already unpacked under llama.cpp when runtime.json is missing.
func managedRuntime() (path string, rec runtimeRecord, ok bool) {
	root := paths.LlamaRuntimeDir()
	if data, err := os.ReadFile(runtimeRecordPath()); err == nil && json.Unmarshal(data, &rec) == nil && buildDirPattern.MatchString(rec.Dir) {
		bin := filepath.Join(root, rec.Dir, "llama-server")
		if info, err := os.Stat(bin); err == nil && info.Mode().IsRegular() {
			return bin, rec, true
		}
	}
	dirs := installedBuilds()
	if len(dirs) == 0 {
		return "", runtimeRecord{}, false
	}
	build := dirs[0]
	bin := filepath.Join(root, buildDir(build), "llama-server")
	rec = runtimeRecord{Build: build, Backend: backendBeside(bin), Dir: buildDir(build), Installed: time.Now().UTC()}
	if data, err := json.MarshalIndent(rec, "", "  "); err == nil {
		_ = writeAtomic(runtimeRecordPath(), append(data, '\n'), 0o600)
	}
	return bin, rec, true
}

// installedBuilds are the llama-b<build> folders holding a llama-server,
// newest first.
func installedBuilds() []int {
	entries, err := os.ReadDir(paths.LlamaRuntimeDir())
	if err != nil {
		return nil
	}
	var builds []int
	for _, e := range entries {
		m := buildDirPattern.FindStringSubmatch(e.Name())
		if m == nil || !e.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(paths.LlamaRuntimeDir(), e.Name(), "llama-server")); err != nil || !info.Mode().IsRegular() {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		builds = append(builds, n)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(builds)))
	return builds
}

// resolution retains every binary for UI and actionable failure messages.
type resolution struct {
	State string
	Bin   binary
	All   []binary
}

func (m *Manager) resolve(ctx context.Context) resolution {
	all := m.discover(ctx)
	res := resolution{State: protocol.RuntimeMissing, All: all}
	for _, b := range all {
		if b.Source == protocol.RuntimeConfigured {
			res.Bin, res.State = b, runtimeState(b)
			return res
		}
		if b.meets() {
			res.Bin, res.State = b, protocol.RuntimeStopped
			return res
		}
		if res.Bin.Path == "" {
			res.Bin, res.State = b, runtimeState(b)
		}
	}
	return res
}

func runtimeState(b binary) string {
	if b.meets() {
		return protocol.RuntimeStopped
	}
	if b.Path == "" {
		return protocol.RuntimeMissing
	}
	return protocol.RuntimeFailed
}

// offers are upstream Linux x64 builds, not NixOS binaries with incompatible loaders.
func offers() []string {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || nixOS() {
		return nil
	}
	return []string{protocol.BackendVulkan, protocol.BackendCPU}
}

func nixOS() bool {
	_, err := os.Stat("/etc/NIXOS")
	return err == nil
}

func runtimeError(res resolution) error {
	if res.State == protocol.RuntimeStopped || res.State == protocol.RuntimeReady {
		return nil
	}
	if len(res.All) == 0 {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("No llama-server was found. Choose a binary or get Hyakki's own in Settings > Providers > Local models."))
	}
	reasons := make([]string, 0, len(res.All))
	for _, b := range res.All {
		reason := b.Err
		if reason == "" {
			reason = i18n.T("Available, but another binary was selected.")
		}
		reasons = append(reasons, b.Path+": "+reason)
	}
	if res.Bin.Source == protocol.RuntimeConfigured {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The chosen llama-server cannot run. Tried: %s", strings.Join(reasons, "; ")))
	}
	return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("No usable llama-server was found. Tried: %s", strings.Join(reasons, "; ")))
}
