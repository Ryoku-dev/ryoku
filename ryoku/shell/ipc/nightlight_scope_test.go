package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

func stubNLLookPath(t *testing.T, paths map[string]string) {
	t.Helper()
	old := nlLookPath
	t.Cleanup(func() { nlLookPath = old })
	nlLookPath = func(name string) (string, error) {
		if path, ok := paths[name]; ok {
			return path, nil
		}
		return "", errors.New("not found: " + name)
	}
}

type nlProbeFunc func(context.Context, string, ...string) ([]byte, error)

func stubNLProbe(t *testing.T, probe nlProbeFunc) {
	t.Helper()
	old := nlProbe
	t.Cleanup(func() { nlProbe = old })
	nlProbe = probe
}

func fixedNLProbe(out string, err error) nlProbeFunc {
	return func(context.Context, string, ...string) ([]byte, error) {
		return []byte(out), err
	}
}

func shortenNLProbeTimeout(t *testing.T) {
	t.Helper()
	old := nlProbeTimeout
	t.Cleanup(func() { nlProbeTimeout = old })
	nlProbeTimeout = 20 * time.Millisecond
}

func TestNightlightCommandUsesUserScope(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/1000/bus")
	stubNLLookPath(t, map[string]string{
		"systemd-run": "/usr/bin/systemd-run",
		"systemctl":   "/usr/bin/systemctl",
	})
	stubNLProbe(t, fixedNLProbe("running\n", nil))

	name, args := nightlightCommand("/usr/bin/ryoku-cmd-nightlight", "on", "4000")
	if name != "/usr/bin/systemd-run" {
		t.Fatalf("name = %q, want systemd-run", name)
	}
	want := []string{
		"--user", "--scope", "--collect", "--quiet", "--slice=app.slice", "--",
		"/usr/bin/ryoku-cmd-nightlight", "on", "4000",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
}

func TestNightlightCommandScopesWithoutUserManagerEnv(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	stubNLLookPath(t, map[string]string{
		"systemd-run": "/usr/bin/systemd-run",
		"systemctl":   "/usr/bin/systemctl",
	})
	stubNLProbe(t, fixedNLProbe("running\n", nil))

	name, args := nightlightCommand("/usr/bin/ryoku-cmd-nightlight", "off")
	if name != "/usr/bin/systemd-run" {
		t.Fatalf("name = %q, want systemd-run when the manager answers without session bus env", name)
	}
	want := []string{
		"--user", "--scope", "--collect", "--quiet", "--slice=app.slice", "--",
		"/usr/bin/ryoku-cmd-nightlight", "off",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
}

func TestNightlightCommandFallsBackWithoutSystemdRun(t *testing.T) {
	stubNLLookPath(t, map[string]string{})
	stubNLProbe(t, func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("probe must not run when systemd-run is missing")
		return nil, nil
	})

	name, args := nightlightCommand("/usr/bin/ryoku-cmd-nightlight", "off")
	if name != "/usr/bin/ryoku-cmd-nightlight" {
		t.Fatalf("name = %q, want direct backend", name)
	}
	if !reflect.DeepEqual(args, []string{"off"}) {
		t.Fatalf("args = %q", args)
	}
}

func TestNightlightCommandFallsBackWhenUserManagerProbeErrors(t *testing.T) {
	stubNLLookPath(t, map[string]string{
		"systemd-run": "/usr/bin/systemd-run",
		"systemctl":   "/usr/bin/systemctl",
	})
	stubNLProbe(t, func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "/usr/bin/systemctl" || !reflect.DeepEqual(args, []string{"--user", "is-system-running"}) {
			t.Errorf("probe = %q %q, want systemctl --user is-system-running", name, args)
		}
		// A probe that fails is unavailable even when its output looks healthy.
		return []byte("running\n"), errors.New("exit status 1")
	})

	name, args := nightlightCommand("/usr/bin/ryoku-cmd-nightlight", "off")
	if name != "/usr/bin/ryoku-cmd-nightlight" {
		t.Fatalf("name = %q, want direct backend when the user manager probe errors", name)
	}
	if !reflect.DeepEqual(args, []string{"off"}) {
		t.Fatalf("args = %q", args)
	}
}

func TestNightlightCommandScopesWhenManagerIsDegraded(t *testing.T) {
	stubNLLookPath(t, map[string]string{
		"systemd-run": "/usr/bin/systemd-run",
		"systemctl":   "/usr/bin/systemctl",
	})
	stubNLProbe(t, func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "/usr/bin/systemctl" || !reflect.DeepEqual(args, []string{"--user", "is-system-running"}) {
			t.Errorf("probe = %q %q, want systemctl --user is-system-running", name, args)
		}
		// is-system-running reports the state on stdout and exits 1 when
		// degraded; that manager can still host a scope.
		return []byte("degraded\n"), &exec.ExitError{}
	})

	name, args := nightlightCommand("/usr/bin/ryoku-cmd-nightlight", "off")
	if name != "/usr/bin/systemd-run" {
		t.Fatalf("name = %q, want systemd-run for a degraded but reachable manager", name)
	}
	want := []string{
		"--user", "--scope", "--collect", "--quiet", "--slice=app.slice", "--",
		"/usr/bin/ryoku-cmd-nightlight", "off",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
}

func TestNightlightCommandFallsBackWhenUserManagerIsUnavailable(t *testing.T) {
	stubNLLookPath(t, map[string]string{
		"systemd-run": "/usr/bin/systemd-run",
		"systemctl":   "/usr/bin/systemctl",
	})
	stubNLProbe(t, fixedNLProbe("Failed to connect to bus: No such file or directory\n", errors.New("exit status 1")))

	name, args := nightlightCommand("/usr/bin/ryoku-cmd-nightlight", "off")
	if name != "/usr/bin/ryoku-cmd-nightlight" {
		t.Fatalf("name = %q, want direct backend when the user manager is unreachable", name)
	}
	if !reflect.DeepEqual(args, []string{"off"}) {
		t.Fatalf("args = %q", args)
	}
}

func TestNightlightCommandFallsBackWhenUserManagerIsStopping(t *testing.T) {
	stubNLLookPath(t, map[string]string{
		"systemd-run": "/usr/bin/systemd-run",
		"systemctl":   "/usr/bin/systemctl",
	})
	stubNLProbe(t, fixedNLProbe("stopping\n", nil))

	name, args := nightlightCommand("/usr/bin/ryoku-cmd-nightlight", "off")
	if name != "/usr/bin/ryoku-cmd-nightlight" {
		t.Fatalf("name = %q, want direct backend when the user manager is not running", name)
	}
	if !reflect.DeepEqual(args, []string{"off"}) {
		t.Fatalf("args = %q", args)
	}
}

func TestNightlightCommandFallsBackWhenUserManagerProbeTimesOut(t *testing.T) {
	stubNLLookPath(t, map[string]string{
		"systemd-run": "/usr/bin/systemd-run",
		"systemctl":   "/usr/bin/systemctl",
	})
	shortenNLProbeTimeout(t)
	timedOut := false
	stubNLProbe(t, func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return nil, errors.New("probe context has no deadline")
		}
		if left := time.Until(deadline); left <= 0 || left > nlProbeTimeout {
			return nil, fmt.Errorf("probe deadline %v from now, want within %v", left, nlProbeTimeout)
		}
		<-ctx.Done()
		timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
		return nil, ctx.Err()
	})

	name, args := nightlightCommand("/usr/bin/ryoku-cmd-nightlight", "off")
	if name != "/usr/bin/ryoku-cmd-nightlight" {
		t.Fatalf("name = %q, want direct backend when the user manager probe times out", name)
	}
	if !reflect.DeepEqual(args, []string{"off"}) {
		t.Fatalf("args = %q", args)
	}
	if !timedOut {
		t.Fatal("probe was not cancelled by a context deadline")
	}
}

func TestNightlightCommandFallsBackWithoutSystemctl(t *testing.T) {
	stubNLLookPath(t, map[string]string{"systemd-run": "/usr/bin/systemd-run"})
	stubNLProbe(t, func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("probe must not run when systemctl is missing")
		return nil, nil
	})

	name, args := nightlightCommand("/usr/bin/ryoku-cmd-nightlight", "off")
	if name != "/usr/bin/ryoku-cmd-nightlight" {
		t.Fatalf("name = %q, want direct backend when systemctl cannot be found", name)
	}
	if !reflect.DeepEqual(args, []string{"off"}) {
		t.Fatalf("args = %q", args)
	}
}
