package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const runtimeReadyTimeout = 30 * time.Second

func nextRuntimeBackoff(delay time.Duration) time.Duration {
	return capDur(delay*2, 30*time.Second)
}

type desktopRun struct {
	runtime     bool
	item        *shellItem
	ready       string
	intentional atomic.Bool
	failed      atomic.Bool
}

func (d *daemon) desktopSelection() *shellItem {
	index := d.currentShellIndex()
	if index == nil {
		return nil
	}
	item := index.item(index.Active)
	if item == nil || item.Runtime.Start == "" {
		return nil
	}
	return item
}

func runtimeEnv(item *shellItem, ready string) []string {
	env := shellItemEnvFrom(qsEnv(), item)
	filtered := env[:0]
	for _, value := range env {
		if strings.HasPrefix(value, "RYOKU_RUNTIME_MANAGED=") || strings.HasPrefix(value, "RYOKU_RUNTIME_READY_FILE=") {
			continue
		}
		filtered = append(filtered, value)
	}
	return append(filtered, "RYOKU_RUNTIME_MANAGED=1", "RYOKU_RUNTIME_READY_FILE="+ready)
}

func (d *daemon) installRuntime(item *shellItem, env []string) error {
	if item.Runtime.Install == "" {
		return nil
	}
	key := shellItemKey(item) + ":" + item.Runtime.Install
	d.runtimeInstallMu.Lock()
	if d.runtimeInstalled == nil {
		d.runtimeInstalled = map[string]bool{}
	}
	if d.runtimeInstalled[key] {
		d.runtimeInstallMu.Unlock()
		return nil
	}
	d.runtimeInstallMu.Unlock()
	if err := runItemScript(item.Runtime.Install, env); err != nil {
		return err
	}
	d.runtimeInstallMu.Lock()
	d.runtimeInstalled[key] = true
	d.runtimeInstallMu.Unlock()
	return nil
}

func (d *daemon) desktopCommand() (*exec.Cmd, *desktopRun, error) {
	item := d.desktopSelection()
	run := &desktopRun{item: item, runtime: item != nil}
	if item == nil {
		cmd := exec.Command("qs", qsSelect("shell")...)
		cmd.Env = d.shellQsEnv()
		return cmd, run, nil
	}
	env := shellItemEnv(item)
	if item.Runtime.Check != "" {
		if err := runItemScript(item.Runtime.Check, env); err != nil {
			run.runtime = false
			cmd := exec.Command("qs", qsSelect("shell")...)
			cmd.Env = d.shellQsEnv()
			return cmd, run, nil
		}
	}
	if err := d.installRuntime(item, env); err != nil {
		return nil, run, fmt.Errorf("runtime install for %s: %w", item.ID, err)
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return nil, run, fmt.Errorf("shell runtime requires XDG_RUNTIME_DIR")
	}
	file, err := os.CreateTemp(dir, "ryoku-shell-runtime-ready-")
	if err != nil {
		return nil, run, err
	}
	run.ready = file.Name()
	if err := file.Close(); err != nil {
		os.Remove(run.ready)
		return nil, run, err
	}
	cmd := exec.Command(item.Runtime.Start)
	cmd.Env = runtimeEnv(item, run.ready)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, run, nil
}

func runtimeReady(path string, pid int) bool {
	body, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(body)) == strconv.Itoa(pid)
}

func (d *daemon) setRuntimeRun(run *desktopRun) {
	d.runtimeRunMu.Lock()
	d.runtimeRun = run
	runtimeIPCReadyFile.Store(run.ready)
	d.runtimeRunMu.Unlock()
}

func (d *daemon) clearRuntimeRun(run *desktopRun) {
	if run == nil {
		return
	}
	d.runtimeRunMu.Lock()
	if d.runtimeRun == run {
		d.runtimeRun = nil
		runtimeIPCReadyFile.Store("")
	}
	d.runtimeRunMu.Unlock()
}

func (d *daemon) markRuntimeRunIntentional() {
	d.runtimeRunMu.Lock()
	if d.runtimeRun != nil {
		d.runtimeRun.intentional.Store(true)
	}
	d.runtimeRunMu.Unlock()
}

func terminateDesktop(cmd *exec.Cmd, group bool, exited <-chan struct{}) {
	signal := func(sig syscall.Signal) {
		if group {
			_ = syscall.Kill(-cmd.Process.Pid, sig)
		} else {
			_ = cmd.Process.Signal(sig)
		}
	}
	signal(syscall.SIGTERM)
	go func() {
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
			signal(syscall.SIGKILL)
		}
	}()
}

func (d *daemon) watchDesktop(cmd *exec.Cmd, run *desktopRun, exited <-chan struct{}, timeout time.Duration) {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	deadline := time.Now().Add(timeout)
	ready := !run.runtime
	for {
		select {
		case <-exited:
			return
		case <-d.quit:
			run.intentional.Store(true)
			terminateDesktop(cmd, run.runtime, exited)
			return
		case <-tick.C:
			if !ready {
				ready = runtimeReady(run.ready, cmd.Process.Pid)
				if ready {
					go recordBootOK(exited)
				}
				if !ready && time.Now().After(deadline) {
					run.failed.Store(true)
					terminateDesktop(cmd, true, exited)
					return
				}
			}
		}
	}
}

func (d *daemon) restoreAfterDesktop(run *desktopRun, cmd *exec.Cmd) {
	if !run.runtime {
		return
	}
	defer os.Remove(run.ready)
	cleanupRuntimeWindows(run.ready)
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	select {
	case <-d.quit:
		return
	default:
	}
	if run.intentional.Load() {
		return
	}
	d.fallbackRuntime(run.item, "shell runtime exited or failed readiness")
}

func (d *daemon) fallbackRuntime(failed *shellItem, reason string) {
	index := d.currentShellIndex()
	if index == nil || failed == nil || index.Active != failed.ID || index.Fallback == "" || index.Fallback == failed.ID {
		return
	}
	if d.settings == nil {
		return
	}
	value, _ := json.Marshal(index.Fallback)
	if err := d.settings.patch("barStyle", value); err != nil {
		fmt.Fprintf(os.Stderr, "shell runtime fallback could not persist: %v\n", err)
		return
	}
	if err := d.syncShellsNow(false); err != nil {
		fmt.Fprintf(os.Stderr, "shell runtime fallback could not sync: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "%s; restoring %s\n", reason, index.Fallback)
}

func ownedRuntimeWindow(argv []string, environ, token string) bool {
	if len(argv) < 1 || token == "" {
		return false
	}
	exe := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(argv[0]), "."), "-wrapped")
	if exe != "qs" && exe != "quickshell" {
		return false
	}
	for _, value := range strings.Split(environ, "\x00") {
		if value == "RYOKU_RUNTIME_READY_FILE="+token {
			return true
		}
	}
	return false
}

func cleanupRuntimeWindows(token string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		dir := filepath.Join("/proc", entry.Name())
		args, err := os.ReadFile(filepath.Join(dir, "cmdline"))
		if err != nil {
			continue
		}
		env, err := os.ReadFile(filepath.Join(dir, "environ"))
		if err != nil {
			continue
		}
		argv := strings.Split(strings.TrimRight(string(args), "\x00"), "\x00")
		if ownedRuntimeWindow(argv, string(env), token) {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	}
}
