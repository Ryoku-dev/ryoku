package host

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var keymapPattern = regexp.MustCompile(`^[A-Za-z0-9_+.-]+$`)

var excludedZoneinfoFiles = map[string]bool{
	"+VERSION":    true,
	"SECURITY":    true,
	"leapseconds": true,
	"localtime":   true,
	"posixrules":  true,
}

func (a *App) Time(args []string) int {
	if len(args) == 1 && args[0] == "zones" {
		init, err := a.Init()
		if err != nil {
			return a.failf("%v", err)
		}
		var zones []string
		if init == Systemd {
			result := a.query("timedatectl", "list-timezones")
			if result.Code != 0 {
				return ExitFailure
			}
			zones = sortedUnique(strings.Fields(result.Output))
		} else {
			zones, err = zoneNames(a.cfg.ZoneinfoDir)
			if err != nil {
				return a.failf("list time zones: %v", err)
			}
		}
		for _, zone := range zones {
			fmt.Fprintln(a.cfg.Stdout, zone)
		}
		return ExitOK
	}
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

func zoneNames(root string) ([]string, error) {
	if data, err := os.ReadFile(filepath.Join(root, "tzdata.zi")); err == nil {
		zones := map[string]bool{}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "Z" {
				zones[fields[1]] = true
			} else if len(fields) >= 3 && fields[0] == "L" {
				zones[fields[2]] = true
			}
		}
		if len(zones) > 0 {
			return sortedZoneSet(zones), nil
		}
	}

	zones := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if relative == "posix" || relative == "right" {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if excludedZoneinfoFiles[name] || strings.HasSuffix(name, ".tab") || strings.HasSuffix(name, ".zi") {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		var magic [4]byte
		_, readErr := io.ReadFull(file, magic[:])
		closeErr := file.Close()
		if readErr != nil {
			return nil
		}
		if closeErr != nil {
			return closeErr
		}
		if magic == [4]byte{'T', 'Z', 'i', 'f'} {
			zones[filepath.ToSlash(relative)] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sortedZoneSet(zones), nil
}

func sortedZoneSet(zones map[string]bool) []string {
	values := make([]string, 0, len(zones))
	for zone := range zones {
		values = append(values, zone)
	}
	sort.Strings(values)
	return values
}

func sortedUnique(values []string) []string {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return sortedZoneSet(set)
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
