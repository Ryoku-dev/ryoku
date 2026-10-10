package host

import (
	"os"
	"path/filepath"
	"strings"
)

var serviceVerbs = map[string]bool{
	"start": true, "stop": true, "restart": true, "try-restart": true,
	"reload": true, "is-active": true, "is-enabled": true, "enable": true,
	"disable": true, "kill": true, "reset-failed": true, "daemon-reload": true,
}

const runitUserDisabledMarker = ".ryoku-disabled"

func unitName(name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	return name + ".service"
}

func runitName(name string) string {
	name = strings.SplitN(name, ".", 2)[0]
	if name == "bluetooth" {
		return "bluetoothd"
	}
	return name
}

func (a *App) Service(args []string) int {
	user := true
	for len(args) > 0 && (args[0] == "--user" || args[0] == "--system") {
		user = args[0] == "--user"
		args = args[1:]
	}
	if len(args) == 0 {
		return ExitUsage
	}
	verb := args[0]
	args = args[1:]
	if verb == "env" {
		return a.ServiceEnv(args)
	}
	if !serviceVerbs[verb] {
		return ExitUsage
	}
	now := false
	if len(args) > 0 && args[0] == "--now" {
		now, args = true, args[1:]
	}
	if verb == "daemon-reload" {
		if now || len(args) != 0 {
			return ExitUsage
		}
	} else if len(args) == 0 {
		return ExitUsage
	}
	init, err := a.Init()
	if err != nil {
		return a.failf("%v", err)
	}
	if init == Systemd {
		return a.systemdService(user, verb, now, args)
	}
	return a.runitService(user, verb, now, args)
}

func (a *App) systemdService(user bool, verb string, now bool, names []string) int {
	args := make([]string, 0, len(names)+4)
	if user {
		args = append(args, "--user")
	}
	args = append(args, verb)
	if now {
		args = append(args, "--now")
	}
	if verb == "is-active" || verb == "is-enabled" {
		args = append(args, "--quiet")
	}
	for _, name := range names {
		args = append(args, unitName(name))
	}
	result := a.run("systemctl", args...)
	if result.Code == 0 {
		return ExitOK
	}
	if result.Code == 4 || result.Code == 5 {
		return ExitAbsent
	}
	if verb == "is-active" && result.Code == 3 {
		return ExitFalse
	}
	if verb == "is-enabled" && result.Code == 1 {
		return ExitFalse
	}
	return ExitFailure
}

func (a *App) runitService(user bool, verb string, now bool, names []string) int {
	if verb == "daemon-reload" || verb == "reset-failed" {
		return ExitOK
	}
	for _, original := range names {
		name := runitName(original)
		if user {
			path := filepath.Join(a.userServiceDir(), name)
			if !isDir(path) || !isSupervised(path) {
				return ExitAbsent
			}
			code := a.runitUserOne(path, verb, now)
			if code != ExitOK {
				return code
			}
			continue
		}
		definition := filepath.Join(a.cfg.SystemServiceDir, name)
		if !isDir(definition) {
			return ExitAbsent
		}
		code := a.runitSystemOne(name, definition, verb, now)
		if code != ExitOK {
			return code
		}
	}
	return ExitOK
}

func (a *App) runitStatus(path string) (bool, int) {
	result := a.query("sv", "status", path)
	if result.Code != 0 {
		return false, ExitAbsent
	}
	return strings.HasPrefix(result.Output, "run:"), ExitOK
}

func (a *App) runitUserOne(path, verb string, now bool) int {
	switch verb {
	case "is-active":
		running, code := a.runitStatus(path)
		if code != ExitOK {
			return code
		}
		if !running {
			return ExitFalse
		}
		return ExitOK
	case "is-enabled":
		if _, err := os.Stat(filepath.Join(path, runitUserDisabledMarker)); err == nil {
			return ExitFalse
		}
		return ExitOK
	case "enable":
		for _, name := range []string{runitUserDisabledMarker, "down"} {
			if err := os.Remove(filepath.Join(path, name)); err != nil && !os.IsNotExist(err) {
				return ExitFailure
			}
		}
		if !now {
			return ExitOK
		}
		return commandExit(a.run("sv", "up", path))
	case "disable":
		if err := os.WriteFile(filepath.Join(path, "down"), nil, 0o644); err != nil {
			return ExitFailure
		}
		if err := os.WriteFile(filepath.Join(path, runitUserDisabledMarker), nil, 0o644); err != nil {
			return ExitFailure
		}
		if !now {
			return ExitOK
		}
		return commandExit(a.run("sv", "down", path))
	case "try-restart":
		running, code := a.runitStatus(path)
		if code != ExitOK {
			return code
		}
		if !running {
			return ExitOK
		}
		return commandExit(a.run("sv", "restart", path))
	}
	command := map[string]string{"start": "up", "stop": "down", "restart": "restart", "reload": "hup", "kill": "kill"}[verb]
	if command == "" {
		return ExitUsage
	}
	return commandExit(a.run("sv", command, path))
}

func (a *App) runitSystemOne(name, definition, verb string, now bool) int {
	boot := filepath.Join(a.cfg.SystemBootDir, name)
	live := filepath.Join(a.cfg.SystemLiveDir, name)
	switch verb {
	case "is-enabled":
		if _, err := os.Lstat(boot); err != nil {
			return ExitFalse
		}
		return ExitOK
	case "enable":
		if err := ensureLink(definition, boot); err != nil {
			return ExitFailure
		}
		if !now {
			return ExitOK
		}
		if err := ensureLink(definition, live); err != nil {
			return ExitFailure
		}
		return commandExit(a.run("sv", "up", live))
	case "disable":
		if now && isSupervised(live) {
			if code := commandExit(a.run("sv", "down", live)); code != ExitOK {
				return code
			}
		}
		for _, path := range []string{boot, live} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return ExitFailure
			}
		}
		return ExitOK
	}
	if !isSupervised(live) {
		return ExitAbsent
	}
	if verb == "is-active" {
		running, code := a.runitStatus(live)
		if code != ExitOK {
			return code
		}
		if !running {
			return ExitFalse
		}
		return ExitOK
	}
	if verb == "try-restart" {
		running, code := a.runitStatus(live)
		if code != ExitOK {
			return code
		}
		if !running {
			return ExitOK
		}
		return commandExit(a.run("sv", "restart", live))
	}
	command := map[string]string{"start": "up", "stop": "down", "restart": "restart", "reload": "hup", "kill": "kill"}[verb]
	if command == "" {
		return ExitUsage
	}
	return commandExit(a.run("sv", command, live))
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isSupervised(path string) bool {
	if !isDir(path) {
		return false
	}
	info, err := os.Stat(filepath.Join(path, "supervise", "stat"))
	return err == nil && !info.IsDir()
}
func ensureLink(target, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if existing, err := os.Readlink(path); err == nil && existing == target {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(target, path)
}
