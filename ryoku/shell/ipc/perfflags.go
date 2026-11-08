package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// parsePerfFlag reads a boolean opt-in from a performance.json body. Anything
// malformed, absent, or the wrong type is false, so an optimisation stays off
// unless the user clearly turned it on.
func parsePerfFlag(b []byte, key string) bool {
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

// ryokuConfigDir is ~/.config/ryoku (honouring XDG_CONFIG_HOME), the dir the
// Ryoku Settings sections write. Empty only when the home dir is unknowable.
func ryokuConfigDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "ryoku")
}

// perfPath is ~/.config/ryoku/performance.json, the file the Performance section
// in Ryoku Settings writes. Empty only when the home dir is unknowable.
func perfPath() string {
	dir := ryokuConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "performance.json")
}

// perfFlag reads one opt-in out of performance.json. A missing file is off.
func perfFlag(key string) bool {
	b, err := os.ReadFile(perfPath())
	if err != nil {
		return false
	}
	return parsePerfFlag(b, key)
}

// perfFlagDefault is perfFlag with an explicit default, for optimisations that
// ship on by default (the user opts out). A missing file, missing key, or wrong
// type all yield def.
func perfFlagDefault(key string, def bool) bool {
	b, err := os.ReadFile(perfPath())
	if err != nil {
		return def
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return def
	}
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}

// watchPerformanceFlags wakes the saver policy when the Hub replaces its file.
// The slow path uses the same interval as the power-state reconciliation.
func (d *daemon) watchPerformanceFlags() {
	path := perfPath()
	if path == "" {
		return
	}
	last, lastErr := os.ReadFile(path)
	lastExists := lastErr == nil
	check := func() {
		body, err := os.ReadFile(path)
		exists := err == nil
		if exists == lastExists && (!exists || bytes.Equal(body, last)) {
			return
		}
		last, lastExists = body, exists
		d.kickSaverExtras()
	}

	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err == nil {
		if _, watchErr := unix.InotifyAddWatch(fd, filepath.Dir(path),
			unix.IN_CLOSE_WRITE|unix.IN_CREATE|unix.IN_MOVED_TO|unix.IN_DELETE|unix.IN_MOVED_FROM); watchErr != nil {
			unix.Close(fd)
			fd = -1
		}
	}
	if err != nil {
		fd = -1
	}
	if fd >= 0 {
		defer unix.Close(fd)
	}
	buf := make([]byte, 4096)
	for {
		if fd >= 0 {
			pfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN | unix.POLLERR}}
			_, _ = unix.Poll(pfds, int(autoProfileReconcile/time.Millisecond))
			for pfds[0].Revents != 0 {
				if _, err := unix.Read(fd, buf); err != nil {
					break
				}
			}
		} else {
			timer := time.NewTimer(autoProfileReconcile)
			select {
			case <-d.quit:
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		select {
		case <-d.quit:
			return
		default:
		}
		check()
	}
}
