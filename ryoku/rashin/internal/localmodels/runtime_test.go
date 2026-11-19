package localmodels

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestAdoptManagedRuntimeAndGateBuild(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	bin := filepath.Join(paths.LlamaRuntimeDir(), "llama-b12521", "llama-server")
	if err := os.MkdirAll(filepath.Dir(bin), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf 'version: 1.0.0-dev (build 12521, commit test)\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	res := m.resolve(context.Background(), DefaultMinBuild)
	if res.State != protocol.RuntimeStopped || res.Bin.Path != bin || res.Bin.Source != protocol.RuntimeManaged || res.Bin.Build != runtimeBuild {
		t.Fatalf("managed runtime = %+v", res)
	}
	if _, err := os.Stat(runtimeRecordPath()); err != nil {
		t.Fatalf("adoption did not write runtime.json: %v", err)
	}
	if res := m.resolve(context.Background(), runtimeBuild+1); res.State != protocol.RuntimeTooOld {
		t.Fatalf("unsupported newer model = %+v", res)
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
	script := "#!/bin/sh\ncase \"$1\" in\n--version) echo 'version: 1.0.0-dev (build 12521, commit test)' ;;\n--list-devices) echo probe >> " + marker + "; echo '  Vulkan0: Test GPU (4096 MiB, 2048 MiB free)' ;;\nesac\n"
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
