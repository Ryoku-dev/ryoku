package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// nest runs niri inside a headless weston for a program that needs a screen
// of its own. The session lives in dir, which becomes its runtime dir, so its
// sockets never meet the user's; niri reads a config written here, so the
// user's own never loads. Animations are off and a window takes the whole
// output, which is what a program that drives apps by sight wants.

const nestParent = "wayland-parent"

const nestConfig = `input {
    keyboard {
        xkb {
            layout "us"
        }
    }
}
output "winit" {
    scale 1.0
}
layout {
    gaps 0
    center-focused-column "never"
    default-column-width { proportion 1.0; }
    focus-ring { off; }
    border { off; }
}
prefer-no-csd
animations {
    off
}
hotkey-overlay {
    skip-at-startup
}
`

func runNest(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: nest <dir> <width>x<height>")
	}
	dir := args[0]
	width, height, err := nestSize(args[1])
	if err != nil {
		return err
	}
	if !filepath.IsAbs(dir) {
		return errors.New("nest: the session dir must be absolute")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	config := filepath.Join(dir, "niri.kdl")
	if err := os.WriteFile(config, []byte(nestConfig), 0o600); err != nil {
		return err
	}
	env := nestEnv(dir)
	weston := exec.Command("weston", "--backend=headless", "--shell=kiosk", "--idle-time=0",
		"--width="+strconv.Itoa(width), "--height="+strconv.Itoa(height), "--socket="+nestParent)
	weston.Env = env
	weston.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := weston.Start(); err != nil {
		return fmt.Errorf("nest: start weston: %w", err)
	}
	niri := exec.Command("niri", "-c", config)
	niri.Env = append(append([]string(nil), env...), "WAYLAND_DISPLAY="+nestParent)
	niri.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stop := func() {
		for _, command := range []*exec.Cmd{niri, weston} {
			if command.Process != nil {
				_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
			}
		}
	}
	exited := make(chan error, 2)
	go func() { exited <- fmt.Errorf("nest: weston exited: %v", weston.Wait()) }()
	if err := nestWait(func() bool { return nestSocket(dir, nestParent) }, exited); err != nil {
		stop()
		return err
	}
	if err := niri.Start(); err != nil {
		stop()
		return fmt.Errorf("nest: start niri: %w", err)
	}
	go func() { exited <- fmt.Errorf("nest: niri exited: %v", niri.Wait()) }()
	display := ""
	if err := nestWait(func() bool {
		display = nestDisplay(dir)
		return display != "" && nestSocket(dir, display)
	}, exited); err != nil {
		stop()
		return err
	}
	if err := json.NewEncoder(stdout).Encode(map[string]string{"display": display}); err != nil {
		stop()
		return err
	}
	_ = stdout.Flush()
	closed := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		close(closed)
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	select {
	case <-closed:
	case <-signals:
	case err = <-exited:
	}
	stop()
	deadline := time.After(3 * time.Second)
	for range 2 {
		select {
		case <-exited:
		case <-deadline:
			for _, command := range []*exec.Cmd{niri, weston} {
				if command.Process != nil {
					_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
				}
			}
			return err
		}
	}
	return err
}

func nestSize(value string) (int, int, error) {
	w, h, ok := strings.Cut(value, "x")
	width, widthErr := strconv.Atoi(w)
	height, heightErr := strconv.Atoi(h)
	if !ok || widthErr != nil || heightErr != nil || width < 320 || height < 240 || width > 7680 || height > 4320 {
		return 0, 0, fmt.Errorf("nest: size %q is not <width>x<height>", value)
	}
	return width, height, nil
}

// nestEnv is the caller's environment pointed at dir: none of the user's
// display, compositor or X11 handles survive into the nested session
func nestEnv(dir string) []string {
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "WAYLAND_DISPLAY", "WAYLAND_SOCKET", "NIRI_SOCKET", "DISPLAY", "XDG_RUNTIME_DIR", "HYPRLAND_INSTANCE_SIGNATURE", "SWAYSOCK":
			continue
		}
		env = append(env, entry)
	}
	return append(env, "XDG_RUNTIME_DIR="+dir)
}

func nestSocket(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && info.Mode()&os.ModeSocket != 0
}

// nestDisplay reads the Wayland socket niri took from the name of its IPC
// socket, niri.<display>.<pid>.sock
func nestDisplay(dir string) string {
	socket := liveSocketFor(dir, "")
	if socket == "" {
		return ""
	}
	name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(socket), "niri."), ".sock")
	display, _, ok := strings.Cut(name, ".")
	if !ok || display == nestParent {
		return ""
	}
	return display
}

func nestWait(ready func() bool, exited <-chan error) error {
	deadline := time.Now().Add(20 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			return errors.New("nest: the nested session did not come up in 20 s")
		}
		select {
		case err := <-exited:
			return err
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil
}
