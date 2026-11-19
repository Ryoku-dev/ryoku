package localmodels

import (
	"context"
	"encoding/json"
	"errors"
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

// binary is one llama-server Rashin may run, with what --version said.
type binary struct {
	Path    string
	Source  string
	Build   int
	Version string
	Semver  [3]int
	Backend string
	Err     string
}

// meets reports a build that runs models needing min. A binary that names
// only a semantic version passes from v0.8.0 (b11960) on.
func (b binary) meets(min int) bool {
	if b.Err != "" {
		return false
	}
	if b.Build > 0 {
		return b.Build >= min
	}
	if b.Semver != [3]int{} {
		return semverAtLeast(b.Semver, [3]int{0, 8, 0}) && min <= DefaultMinBuild
	}
	return false
}

func semverAtLeast(v, want [3]int) bool {
	for i := range v {
		if v[i] != want[i] {
			return v[i] > want[i]
		}
	}
	return true
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
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano()), true
}

// probe runs path --version once per file version.
func (m *Manager) probe(ctx context.Context, path, source string) binary {
	b := binary{Path: path, Source: source}
	key, ok := probeKey(path)
	if !ok {
		b.Err = i18n.Tf("%s is not there.", path)
		return b
	}
	m.mu.Lock()
	cached, hit := m.bins[key]
	m.mu.Unlock()
	if hit {
		cached.Source = source
		return cached
	}
	out, err := runQuick(ctx, path, 5*time.Second, "--version")
	build, version, semver, parsed := parseVersion(out)
	b.Build, b.Version, b.Semver = build, version, semver
	if err != nil || !parsed {
		if err == nil {
			err = errors.New(i18n.T("it printed no version"))
		}
		b.Err = i18n.Tf("%s does not run: %v", path, err)
		if ctx.Err() != nil {
			return b // not cached: the caller gave up, not the binary
		}
	}
	b.Backend = backendBeside(path)
	m.mu.Lock()
	m.bins[key] = b
	m.mu.Unlock()
	return b
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

// resolution is the llama-server Rashin would run now: State is
// RuntimeStopped when Bin may start, RuntimeTooOld or RuntimeMissing when
// none may. Bin is set for TooOld too, to say which one.
type resolution struct {
	State string
	Bin   binary
	Min   int
}

// resolve picks llama-server: the configured path, else the managed
// runtime, else llama-server on PATH when it is new enough for min. A
// configured binary is used as it is, too old or not, since the user chose
// it.
func (m *Manager) resolve(ctx context.Context, min int) resolution {
	s := m.settings().Providers.Models
	res := resolution{State: protocol.RuntimeMissing, Min: min}
	if s.Binary != "" {
		res.Bin = m.probe(ctx, s.Binary, protocol.RuntimeConfigured)
		res.State = runtimeState(res.Bin, min)
		return res
	}
	if path, _, ok := managedRuntime(); ok {
		b := m.probe(ctx, path, protocol.RuntimeManaged)
		if b.meets(min) {
			return resolution{State: protocol.RuntimeStopped, Bin: b, Min: min}
		}
		res.Bin, res.State = b, runtimeState(b, min)
	}
	if path, err := exec.LookPath("llama-server"); err == nil {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		b := m.probe(ctx, path, protocol.RuntimeSystem)
		if b.meets(min) {
			return resolution{State: protocol.RuntimeStopped, Bin: b, Min: min}
		}
		if res.Bin.Path == "" {
			res.Bin, res.State = b, runtimeState(b, min)
		}
	}
	return res
}

func runtimeState(b binary, min int) string {
	switch {
	case b.meets(min):
		return protocol.RuntimeStopped
	case b.Err != "":
		return protocol.RuntimeMissing
	default:
		return protocol.RuntimeTooOld
	}
}

// offers is what local.runtime_install can put on this computer: the
// upstream Linux x64 builds, not on NixOS, whose loader they do not fit.
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

// runtimeError says why a model cannot start under res.
func runtimeError(res resolution) error {
	switch res.State {
	case protocol.RuntimeTooOld:
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("llama.cpp %s at %s is too old for this model (it needs build %d or newer). Rashin can get its own in Settings > Providers.", res.Bin.Version, res.Bin.Path, res.Min))
	case protocol.RuntimeMissing:
		if res.Bin.Err != "" {
			return protocol.Errorf(protocol.CodeUnavailable, "%s", res.Bin.Err)
		}
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("llama.cpp is not on this computer. Get it in Settings > Providers."))
	}
	return nil
}
