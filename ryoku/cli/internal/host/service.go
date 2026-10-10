package host

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var serviceVerbs = map[string]bool{
	"start": true, "stop": true, "restart": true, "try-restart": true,
	"reload": true, "is-active": true, "is-enabled": true, "enable": true,
	"disable": true, "kill": true, "reset-failed": true, "daemon-reload": true,
}

const runitUserDisabledMarker = ".ryoku-disabled"

const (
	runitTAI64Offset   = uint64(0x400000000000000a)
	runitRestartWindow = 10 * time.Second
	runitSampleDelay   = 2 * time.Second
)

type ServiceFailure struct {
	Scope string
	Name  string
}

type runitServiceSource struct {
	scope string
	dir   string
}

type runitSuperviseState struct {
	running bool
	down    bool
	wantUp  bool
	changed [12]byte
	change  time.Time
}

type runitRestartCandidate struct {
	failure ServiceFailure
	path    string
	state   runitSuperviseState
}

func (a *App) FailedServices() ([]ServiceFailure, error) {
	return runitFailedServicesIn(
		[]runitServiceSource{
			{scope: "--system", dir: a.cfg.SystemLiveDir},
			{scope: "--user", dir: a.userServiceDir()},
		},
		time.Now,
		time.Sleep,
	)
}

func runitFailedServicesIn(sources []runitServiceSource, now func() time.Time, sleep func(time.Duration)) ([]ServiceFailure, error) {
	var failed []ServiceFailure
	var candidates []runitRestartCandidate
	observedAt := now()
	for _, source := range sources {
		entries, err := os.ReadDir(source.dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read service directory %s: %w", source.dir, err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			path := filepath.Join(source.dir, entry.Name())
			info, err := os.Stat(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("inspect service %s: %w", path, err)
			}
			if !info.IsDir() {
				continue
			}
			if source.scope == "--user" {
				if _, err := os.Stat(filepath.Join(path, runitUserDisabledMarker)); err == nil {
					continue
				} else if !os.IsNotExist(err) {
					return nil, fmt.Errorf("inspect service %s: %w", path, err)
				}
			}
			state, err := readRunitSuperviseState(path)
			if err != nil {
				if os.IsNotExist(err) && fileExists(filepath.Join(path, "down")) {
					continue
				}
				return nil, fmt.Errorf("read supervise state for %s: %w", path, err)
			}
			if !state.wantUp {
				continue
			}
			failure := ServiceFailure{Scope: source.scope, Name: entry.Name()}
			if state.down {
				failed = append(failed, failure)
				continue
			}
			if !state.running {
				continue
			}
			age := observedAt.Sub(state.change)
			if age >= -time.Second && age <= runitRestartWindow {
				candidates = append(candidates, runitRestartCandidate{failure: failure, path: path, state: state})
			}
		}
	}
	if len(candidates) == 0 {
		return failed, nil
	}
	sleep(runitSampleDelay)
	for _, candidate := range candidates {
		state, err := readRunitSuperviseState(candidate.path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("resample supervise state for %s: %w", candidate.path, err)
		}
		if !state.wantUp {
			continue
		}
		if state.down || (state.running && state.changed != candidate.state.changed) {
			failed = append(failed, candidate.failure)
		}
	}
	return failed, nil
}

func readRunitSuperviseState(path string) (runitSuperviseState, error) {
	status, err := os.ReadFile(filepath.Join(path, "supervise", "status"))
	if err != nil {
		return runitSuperviseState{}, err
	}
	if len(status) < 20 {
		return runitSuperviseState{}, fmt.Errorf("short supervise/status")
	}
	var changed [12]byte
	copy(changed[:], status[:12])
	seconds := binary.BigEndian.Uint64(status[:8])
	nanoseconds := binary.BigEndian.Uint32(status[8:12])
	if seconds < runitTAI64Offset || nanoseconds >= uint32(time.Second) {
		return runitSuperviseState{}, fmt.Errorf("invalid supervise timestamp")
	}
	wantUp := false
	switch status[17] {
	case 'u':
		wantUp = true
	case 'd':
	default:
		return runitSuperviseState{}, fmt.Errorf("invalid supervise want state")
	}
	running, down := false, false
	switch status[19] {
	case 0:
		down = true
	case 1:
		running = true
	case 2:
	default:
		return runitSuperviseState{}, fmt.Errorf("invalid supervise service state")
	}
	return runitSuperviseState{
		running: running,
		down:    down,
		wantUp:  wantUp,
		changed: changed,
		change:  time.Unix(int64(seconds-runitTAI64Offset), int64(nanoseconds)),
	}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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
