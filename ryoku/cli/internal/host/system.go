package host

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var keymapPattern = regexp.MustCompile(`^[A-Za-z0-9_+.-]+$`)

func (a *App) Time(args []string) int {
	if len(args) == 1 && args[0] == "zone" {
		init, err := a.Init()
		if err != nil {
			return a.failf("%v", err)
		}
		if init == Systemd {
			return commandExit(a.run("timedatectl", "show", "-p", "Timezone", "--value"))
		}
		target, err := os.Readlink(a.cfg.LocaltimePath)
		if err != nil {
			return ExitFailure
		}
		zone, err := filepath.Rel(a.cfg.ZoneinfoDir, func() string {
			if filepath.IsAbs(target) {
				return target
			}
			return filepath.Clean(filepath.Join(filepath.Dir(a.cfg.LocaltimePath), target))
		}())
		if err != nil || zone == ".." || strings.HasPrefix(zone, ".."+string(filepath.Separator)) {
			return ExitFailure
		}
		fmt.Fprintln(a.cfg.Stdout, filepath.ToSlash(zone))
		return ExitOK
	}
	if len(args) == 2 && args[0] == "set-zone" {
		zone := args[1]
		init, err := a.Init()
		if err != nil {
			return a.failf("%v", err)
		}
		if init == Systemd {
			return commandExit(a.run("timedatectl", "set-timezone", zone))
		}
		if !validZone(a.cfg.ZoneinfoDir, zone) {
			return ExitUsage
		}
		if a.cfg.UID != 0 {
			return commandExit(a.run("pkexec", "ryoku-host", "time", "set-zone", zone))
		}
		target := filepath.Join(a.cfg.ZoneinfoDir, filepath.FromSlash(zone))
		if err := os.Remove(a.cfg.LocaltimePath); err != nil && !os.IsNotExist(err) {
			return ExitFailure
		}
		if err := os.Symlink(target, a.cfg.LocaltimePath); err != nil {
			return ExitFailure
		}
		return ExitOK
	}
	return ExitUsage
}

func validZone(root, zone string) bool {
	if zone == "" || filepath.IsAbs(zone) || strings.Contains(zone, "..") || strings.ContainsRune(zone, '\x00') {
		return false
	}
	path := filepath.Join(root, filepath.FromSlash(zone))
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (a *App) Keymap(args []string) int {
	if len(args) < 2 || args[0] != "set" || len(args) > 4 {
		return ExitUsage
	}
	layout := args[1]
	variant, options := "", ""
	if len(args) > 2 {
		variant = args[2]
	}
	if len(args) > 3 {
		options = args[3]
	}
	if !keymapPattern.MatchString(layout) {
		return ExitUsage
	}
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	if init == Systemd {
		return commandExit(a.run("localectl", "set-x11-keymap", layout, "", variant, options))
	}
	if a.cfg.UID != 0 {
		pkargs := []string{"ryoku-host", "keymap", "set", layout}
		if len(args) > 2 {
			pkargs = append(pkargs, variant)
		}
		if len(args) > 3 {
			pkargs = append(pkargs, options)
		}
		return commandExit(a.run("pkexec", pkargs...))
	}
	if err := writeKeymap(a.cfg.RCConfPath, layout); err != nil {
		return a.failf("set the console keymap: %v", err)
	}
	return ExitOK
}

func writeKeymap(path, keymap string) error {
	if !keymapPattern.MatchString(keymap) {
		return fmt.Errorf("invalid console keymap %q", keymap)
	}
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(string(body), "\n")
	entry, found := fmt.Sprintf("KEYMAP=%q", keymap), false
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "KEYMAP=") {
			if !found {
				lines[index], found = entry, true
			} else {
				lines[index] = ""
			}
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines[len(lines)-1] = entry
			lines = append(lines, "")
		} else {
			lines = append(lines, entry)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
