package localmodels

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestAdoptManagedRuntimeWithoutVersionGate(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHELL", "")
	bin := filepath.Join(paths.LlamaRuntimeDir(), "llama-b12521", "llama-server")
	if err := os.MkdirAll(filepath.Dir(bin), 0o700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \"$1\" in\n--help) echo '--host --port -m --api-key-file --models-preset --models-max --jinja --mmproj --fit -fa' ;;\n--version) echo 'version: 0 (unknown)' ;;\nesac\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	res := m.resolve(context.Background())
	if res.State != protocol.RuntimeStopped || res.Bin.Path != bin || res.Bin.Source != protocol.RuntimeManaged || !res.Bin.Router {
		t.Fatalf("managed runtime = %+v", res)
	}
	if _, err := os.Stat(runtimeRecordPath()); err != nil {
		t.Fatalf("adoption did not write runtime.json: %v", err)
	}
	if res := m.resolve(context.Background()); res.State != protocol.RuntimeStopped {
		t.Fatalf("capable binary rejected by build number: %+v", res)
	}
}

func TestRuntimeSwapWaitsForInferenceLease(t *testing.T) {
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	release := m.Acquire()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if finish, err := m.beginRuntimeSwap(ctx); err == nil {
		finish()
		t.Fatal("swapped an active runtime")
	}
	release()
	finish, err := m.beginRuntimeSwap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan struct{})
	go func() { later := m.Acquire(); later(); close(acquired) }()
	select {
	case <-acquired:
		t.Fatal("new inference entered during runtime swap")
	case <-time.After(20 * time.Millisecond):
	}
	finish()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("runtime swap left inference blocked")
	}
}

func TestModelsProbeDevicesAndCachePerBinary(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	bin := filepath.Join(t.TempDir(), "llama-server")
	marker := filepath.Join(t.TempDir(), "probes")
	script := "#!/bin/sh\ncase \"$1\" in\n--help) echo '--host --port -m --api-key-file --models-preset --models-max --jinja --mmproj --fit -fa' ;;\n--version) echo 'version: 1.0.0-dev (build 12521, commit test)' ;;\n--list-devices) echo probe >> " + marker + "; echo '  Vulkan0: Test GPU (4096 MiB, 2048 MiB free)' ;;\nesac\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings {
		var settings protocol.Settings
		settings.Providers.Models.Binary = bin
		return settings
	}, nil)
	m.Models(context.Background())
	if probes, err := os.ReadFile(marker); err != nil || string(probes) != "probe\n" {
		t.Fatalf("models listing must probe devices to plan installed models: %q, %v", probes, err)
	}
	for range 2 {
		devices := m.Devices(context.Background())
		if len(devices) != 1 || devices[0].ID != "Vulkan0" {
			t.Fatalf("devices = %+v", devices)
		}
	}
	probes, err := os.ReadFile(marker)
	if err != nil || string(probes) != "probe\n" {
		t.Fatalf("device probe cache failed: %q, %v", probes, err)
	}
	if err := os.WriteFile(bin, []byte(script+"# changed\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if devices := m.Devices(context.Background()); len(devices) != 1 {
		t.Fatalf("new binary device probe = %+v", devices)
	}
	probes, err = os.ReadFile(marker)
	if err != nil || string(probes) != "probe\nprobe\n" {
		t.Fatalf("new binary not probed: %q, %v", probes, err)
	}
}

func TestFindRuntimeInHomeLoginPathAndRankGPU(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	build := filepath.Join(home, "llama.cpp", "build", "bin", "llama-server")
	login := filepath.Join(t.TempDir(), "llama-server")
	for _, path := range []string{build, login} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\ncase \"$1\" in\n--help) echo '--host --port -m --api-key-file --alias --models-preset --models-max --jinja --mmproj --fit -fa' ;;\n--version) echo 'version: 0 (unknown)' ;;\nesac\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(build), "libggml-vulkan.so"), []byte("gpu"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	shellCalls := 0
	m.discovery.shell = func(context.Context, string) (string, error) { shellCalls++; return filepath.Dir(login), nil }
	for range 2 {
		res := m.resolve(context.Background())
		if res.State != protocol.RuntimeStopped || res.Bin.Path != build || res.Bin.Backend != protocol.BackendVulkan {
			t.Fatalf("did not prefer GPU build in home: %+v", res)
		}
		if len(res.All) < 2 || res.All[0].Path != build {
			t.Fatalf("not all runtimes exposed: %+v", res.All)
		}
	}
	if shellCalls != 1 {
		t.Fatalf("login shell PATH read %d times", shellCalls)
	}
}

func TestLoginShellUsesFishPathList(t *testing.T) {
	shell := filepath.Join(t.TempDir(), "fish")
	script := "#!/bin/sh\n[ \"$2\" = 'string join : $PATH' ] || exit 2\nprintf '/first/bin:/second/bin'\n"
	if err := os.WriteFile(shell, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := loginPath(context.Background(), shell)
	if err != nil || got != "/first/bin:/second/bin" {
		t.Fatalf("fish login PATH = %q, %v", got, err)
	}
}

func TestPresetRouterWithoutModelsMaxStillRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llama-server")
	script := "#!/bin/sh\ncase \"$1\" in\n--help) echo '--host --port -m --api-key-file --models-preset --jinja' ;;\n--version) echo 'version: 0 (unknown)' ;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	got := m.probe(context.Background(), path, protocol.RuntimeConfigured)
	if !got.meets() || !got.Router || got.Build != 0 {
		t.Fatalf("preset router was rejected for missing models-max: %+v", got)
	}
	presets := renderPresetsFor([]presetModel{{ID: "test", Model: "model.gguf"}}, got.Flags)
	if strings.Contains(presets, "stop-timeout") || strings.Contains(presets, "parallel =") {
		t.Fatalf("unsupported options were written to router presets: %s", presets)
	}
}

func TestBrokenBinaryIsFoundAndShowsLoaderError(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SHELL", "")
	bin := filepath.Join(t.TempDir(), "llama-server")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'error while loading shared libraries: libggml.so: cannot open shared object file' >&2\nexit 127\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings {
		var s protocol.Settings
		s.Providers.Models.Binary = bin
		return s
	}, nil)
	res := m.resolve(context.Background())
	if res.State != protocol.RuntimeFailed || len(res.All) == 0 || !strings.Contains(res.Bin.Err, "libggml.so") {
		t.Fatalf("found broken runtime incorrectly reported: %+v", res)
	}
	if err := runtimeError(res); err == nil || !strings.Contains(err.Error(), bin) || !strings.Contains(err.Error(), "libggml.so") {
		t.Fatalf("missing tried binary and loader message: %v", err)
	}
}

func TestPlainServerDropsUnsupportedFlagsButKeepsModelAndKey(t *testing.T) {
	b := binary{Flags: helpFlags("--host --port -m --api-key-file --alias --ctx-size --mmproj --n-gpu-layers -fa")}
	plan := presetModel{ID: "test", Model: "/data/test.gguf", Mmproj: "/data/mmproj.gguf", Context: 8192,
		PlanArgs: map[string]string{"fit": "on", "flash-attn": "auto", "n-gpu-layers": "all", "threads": "4"}}
	got := strings.Join(plainArgs(b, plan, 9876, "/tmp/key", "secret"), " ")
	for _, required := range []string{"--host 127.0.0.1", "--port 9876", "--api-key-file /tmp/key", "-m /data/test.gguf",
		"--alias test", "--mmproj /data/mmproj.gguf", "--ctx-size 8192", "--n-gpu-layers 999"} {
		if !strings.Contains(got, required) {
			t.Fatalf("plain server missing %q in %q", required, got)
		}
	}
	// a bare -fa switch would read the next word as a stray argument
	for _, absent := range []string{"--models-preset", "--fit ", "--threads", "--jinja", "-fa"} {
		if strings.Contains(got, absent) {
			t.Fatalf("plain server used unsupported %q in %q", absent, got)
		}
	}
}

func TestFlashAttentionAutoOnlyWhereTheFlagTakesAValue(t *testing.T) {
	plan := presetModel{ID: "test", Model: "/data/test.gguf", PlanArgs: map[string]string{"flash-attn": "auto"}}
	valued := binary{Flags: helpFlags("--host --port -m --api-key-file\n-fa,   --flash-attn [on|off|auto]  set Flash Attention use")}
	if got := strings.Join(plainArgs(valued, plan, 1, "/k", "s"), " "); !strings.Contains(got, "--flash-attn auto") {
		t.Fatalf("valued flash attention missing in %q", got)
	}
	if ini := renderPresetsFor([]presetModel{plan}, valued.Flags); !strings.Contains(ini, "flash-attn = auto") {
		t.Fatalf("valued router presets lack flash-attn:\n%s", ini)
	}
	bare := helpFlags("--host --port --models-preset -m --api-key-file\n-fa,   --flash-attn            enable Flash Attention (default: disabled)")
	if ini := renderPresetsFor([]presetModel{plan}, bare); strings.Contains(ini, "flash-attn") {
		t.Fatalf("an old router got a value for a bare switch:\n%s", ini)
	}
}
