package wm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A daemon can construct its Client before the compositor's provider is
// answering. Caps must not cache that failure for the process lifetime: the
// first probe errors, and the next one, once the provider is up, succeeds and
// carries the real capabilities. This is the latent fault behind a session that
// had no night light for its whole life because it asked caps one beat too early.
func TestCapsReprobesAfterFailedProbe(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "count")

	// The fake provider fails its first caps invocation and succeeds after, keyed
	// on a counter file so each exec of the script advances the state.
	prov := filepath.Join(dir, "ryoku-wm-testwm")
	script := strings.NewReplacer("@COUNT@", counter).Replace(`#!/usr/bin/env bash
set -u
count="@COUNT@"
n=$(cat "$count" 2>/dev/null || echo 0)
printf '%s\n' "$((n + 1))" >"$count"
[[ "${1:-}" == caps ]] || exit 0
if (( n == 0 )); then
  echo "provider still coming up" >&2
  exit 1
fi
printf '%s' '{"name":"testwm","supports":["nightLight"],"nightLightProcess":"gammastep"}'
`)
	if err := os.WriteFile(prov, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// The fake must win over any packaged provider on PATH; RYOKU_WM forces
	// detection to it without needing that compositor's session.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RYOKU_WM", "testwm")

	c := Open()

	if _, err := c.Caps(); err == nil {
		t.Fatal("first caps probe should fail while the provider is not answering")
	}

	caps, err := c.Caps()
	if err != nil {
		t.Fatalf("second caps probe should succeed once the provider answers: %v", err)
	}
	if caps.NightLightProcess != "gammastep" {
		t.Fatalf("reprobe did not pick up the real caps: %+v", caps)
	}
	if !c.Can(CapNightLight) {
		t.Fatal("night light capability should read true after a successful reprobe")
	}
}

func TestActContextPropagatesTransientCapsFailure(t *testing.T) {
	dir := t.TempDir()
	prov := filepath.Join(dir, "ryoku-wm-testwm")
	if err := os.WriteFile(prov, []byte("#!/bin/sh\necho 'provider retraining' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := OpenNamed("testwm").ActContext(context.Background(), ActionOutputPower, "on")
	if err == nil {
		t.Fatal("transient caps failure was accepted as an action success")
	}
	if errors.Is(err, ErrUnsupported) {
		t.Fatalf("transient caps failure became permanent unsupported: %v", err)
	}
}

func TestActContextTimeoutKillsProviderProcessGroup(t *testing.T) {
	dir := t.TempDir()
	childPath := filepath.Join(dir, "child.pid")
	prov := filepath.Join(dir, "ryoku-wm-testwm")
	script := strings.NewReplacer("@CHILD@", childPath).Replace(`#!/usr/bin/env bash
if [[ "${1:-}" == caps ]]; then
  printf '%s' '{"name":"testwm","supports":["outputPower"]}'
  exit 0
fi
sleep 30 &
printf '%s\n' "$!" >"@CHILD@"
wait
`)
	if err := os.WriteFile(prov, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := OpenNamed("testwm").ActContext(ctx, ActionOutputPower, "on")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hung provider action returned %v, want deadline exceeded", err)
	}
	raw, readErr := os.ReadFile(childPath)
	if readErr != nil {
		t.Fatalf("provider never started its IPC child: %v", readErr)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if convErr != nil {
		t.Fatal(convErr)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		stat, statErr := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
		if os.IsNotExist(statErr) {
			break
		}
		fields := strings.Fields(string(stat))
		if statErr == nil && len(fields) > 2 && fields[2] == "Z" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("provider IPC child %d survived context cancellation", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
