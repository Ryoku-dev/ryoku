package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ryoku-rashin/internal/ask"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/daemon"
	"ryoku-rashin/internal/gateway"
	"ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/index"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/teams"
	"ryoku-rashin/internal/workspace"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: rashind [--version]")
		os.Exit(1)
	}
	if err := run(); err != nil {
		logStartup(err)
		os.Exit(1)
	}
}
func run() error {
	// a core dump, or a same-user read of /proc/<pid>/mem, would lift the
	// vault's key and every value the daemon opens
	if _, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_DUMPABLE, 0, 0); errno != 0 {
		return fmt.Errorf("prctl PR_SET_DUMPABLE: %w", errno)
	}
	initial, err := config.Load()
	if err != nil {
		return err
	}
	if !initial.Enabled {
		return nil
	}
	lock, acquired, err := daemon.AcquireLock()
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	owned := true
	defer func() {
		if owned {
			_ = lock.Close()
		}
	}()
	gw, err := gateway.Open(paths.StateDir())
	if err != nil {
		return err
	}
	events := daemon.NewBroadcaster()
	settings := func() protocol.Settings {
		s, err := config.Load()
		if err != nil {
			return config.Defaults()
		}
		return s
	}
	idx, err := index.Open(events, settings)
	if err != nil {
		gw.Close()
		return err
	}
	work, err := workspace.Open(events, gw, settings)
	if err != nil {
		idx.Close()
		gw.Close()
		return err
	}
	crew, err := teams.Open(events, settings)
	if err != nil {
		work.Close()
		idx.Close()
		gw.Close()
		return err
	}
	deps := daemon.Deps{Gateway: gw, Asker: ask.New(gw, idx), Harnesses: harness.New(), Indexer: idx, Work: work, Teams: crew, Version: version, Events: events, Lock: lock}
	// read now: once the file is replaced the kernel no longer names it
	if program, programErr := os.Executable(); programErr == nil {
		deps.Program = program
	}
	d := daemon.New(deps)
	gw.OnChange(d.ProvidersChanged)
	owned = false
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	return d.Run(ctx)
}

func logStartup(err error) {
	_ = os.MkdirAll(filepath.Dir(paths.DaemonLog()), 0o700)
	if stat, statErr := os.Stat(paths.DaemonLog()); statErr == nil && stat.Size() >= 1<<20 {
		_ = os.Remove(paths.DaemonLog() + ".1")
		_ = os.Rename(paths.DaemonLog(), paths.DaemonLog()+".1")
	}
	f, openErr := os.OpenFile(paths.DaemonLog(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if openErr != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s startup: %v\n", time.Now().Format(time.RFC3339), err)
}
