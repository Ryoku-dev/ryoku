package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
)

func TestSupervisedWorkerStopsItsRunningChildAfterAtomicDisable(t *testing.T) {
	for _, mode := range []struct {
		name string
		once bool
	}{{"supervised", false}, {"once", true}} {
		t.Run(mode.name, func(t *testing.T) {
			root := workerFixture(t)
			pidFile := filepath.Join(root, "worker.pid")
			t.Setenv("HYAKKI_TEST_WORKER_PID", pidFile)
			program := filepath.Join(root, "collector")
			if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf '%s' \"$$\" > \"$HYAKKI_TEST_WORKER_PID\"\ntrap 'exit 0' TERM INT\nwhile :; do sleep 1; done\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			done := make(chan error, 1)
			stopped := make(chan struct{})
			go func() {
				defer close(stopped)
				done <- superviseWorker(ctx, workerSpec{name: "usage", program: program, period: time.Hour, once: mode.once})
			}()
			defer func() { cancel(); <-stopped }()
			var pid int
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if raw, err := os.ReadFile(pidFile); err == nil {
					pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
					if err == nil && pid > 0 {
						break
					}
				}
				select {
				case err := <-done:
					t.Fatalf("worker stopped before its collector ran: %v", err)
				default:
				}
				time.Sleep(5 * time.Millisecond)
			}
			if pid == 0 {
				t.Fatal("collector did not start")
			}
			if err := config.SetEnabled(false); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("disabled worker kept running")
			}
			if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("disabled collector survived worker shutdown: %v", err)
			}
		})
	}
}

func workerFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for variable, directory := range map[string]string{
		"HOME": "home", "XDG_CONFIG_HOME": "config", "XDG_STATE_HOME": "state",
		"XDG_DATA_HOME": "data", "XDG_CACHE_HOME": "cache", "XDG_RUNTIME_DIR": "runtime",
	} {
		t.Setenv(variable, filepath.Join(root, directory))
	}
	if err := config.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestOneShotUsageReportsFailureWithoutRetryAndReleasesItsLease(t *testing.T) {
	root := workerFixture(t)
	runs := filepath.Join(root, "runs")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := superviseWorker(ctx, workerSpec{
		name: "usage", once: true, period: time.Hour, program: "/bin/sh",
		args: []string{"-c", "printf 'run\\n' >> \"$1\"; exit 23", "collector", runs},
	})
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 23 {
		t.Fatalf("the failed one-shot collector was not reported: %v", err)
	}
	data, err := os.ReadFile(runs)
	if err != nil || string(data) != "run\n" {
		t.Fatalf("the one-shot collector retried or failed to run: %q, %v", data, err)
	}
	lease, err := workerLock("usage-job")
	if err != nil {
		t.Fatalf("the finished collector retained its lease: %v", err)
	}
	lease.Close()
}

func TestManualUsageRefreshSharesTheCollectorLeaseButNotTheSchedulerWait(t *testing.T) {
	root := workerFixture(t)
	scheduler, err := workerLock("usage")
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	job, err := workerLock("usage-job")
	if err != nil {
		t.Fatal(err)
	}
	defer job.Close()
	started := filepath.Join(root, "collected")
	spec := workerSpec{
		name: "usage", once: true, period: time.Hour, program: "/bin/sh",
		args: []string{"-c", "printf collected > \"$1\"", "collector", started},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := superviseWorker(ctx, spec); !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("overlapping usage collection was not refused: %v", err)
	}
	if _, err := os.Stat(started); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("another collector ran while the shared lease was held: %v", err)
	}
	job.Close()
	if err := superviseWorker(ctx, spec); err != nil {
		t.Fatalf("manual refresh could not run beside an idle scheduler: %v", err)
	}
	data, err := os.ReadFile(started)
	if err != nil || string(data) != "collected" {
		t.Fatalf("manual refresh waited for the scheduler instead of collecting: %q, %v", data, err)
	}
}
