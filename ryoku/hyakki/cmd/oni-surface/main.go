// Command oni-surface draws Oni on the desktop: a layer-shell surface the
// size of the character, driven by the oni daemon. `hyakki oni show` and the
// app start it; it never starts at login on its own.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"syscall"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/oni/surface"
	"ryoku-rashin/internal/oni/surface/launch"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	switch {
	case len(args) == 1 && args[0] == "--version":
		fmt.Printf("Oni surface %s\n", version)
		return 0
	case len(args) == 1 && args[0] == "--globals":
		g, err := surface.ListGlobals()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		out, _ := json.Marshal(g)
		fmt.Println(string(out))
		return 0
	case len(args) > 0:
		fmt.Fprintln(os.Stderr, i18n.T("Usage: oni-surface [--version | --globals]"))
		return 2
	}

	lab, err := surface.LabFromEnv(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	lock, held, err := launch.Acquire()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !held {
		return 0
	}
	defer lock.Release()

	dial := surface.DaemonDialer("")
	if lab.Script != "" {
		f, err := os.Open(lab.Script)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		entries, err := surface.LoadScript(f)
		f.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		dial = surface.ScriptDialer(entries, lab.Arrivals)
	}
	var ledger *surface.Ledger
	if lab.On {
		if ledger, err = surface.OpenLedger(lab.Ledger, os.Getenv("WAYLAND_DISPLAY")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer ledger.Close()
	}
	if lab.Profile != "" {
		stopProfile, err := startProfile(lab.Profile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer stopProfile()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = surface.Run(ctx, surface.Options{Lab: lab, Dial: dial, Ledger: ledger})
	switch {
	case err == nil:
		return 0
	case errors.Is(err, client.ErrDisabled), errors.Is(err, surface.ErrPresenterRefused):
		fmt.Fprintln(os.Stderr, err)
		return 0
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

// startProfile records a CPU profile of the run into dir; the returned stop
// ends it and adds the run's allocations. Lab only (HYAKKI_ONI_PROFILE).
func startProfile(dir string) (func(), error) {
	cpu, err := os.Create(filepath.Join(dir, "cpu.pprof"))
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(cpu); err != nil {
		cpu.Close()
		return nil, err
	}
	return func() {
		pprof.StopCPUProfile()
		cpu.Close()
		if f, err := os.Create(filepath.Join(dir, "allocs.pprof")); err == nil {
			pprof.Lookup("allocs").WriteTo(f, 0)
			f.Close()
		}
	}, nil
}
