package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type legacyHyakkiService struct {
	name         string
	activation   bool
	enabled      bool
	enabledState string
	running      bool
	inherited    bool
	localLinks   []legacyHyakkiLink
}

type legacyHyakkiFile struct {
	path   string
	body   []byte
	mode   os.FileMode
	exists bool
}

// Old IDs stay migration inputs, never native launch targets
func legacyHyakkiServices(init InitSystem) []legacyHyakkiService {
	if init == Runit {
		return []legacyHyakkiService{{name: "ryoku-ai-usage"}, {name: "rashind-routines"}, {name: "ryoku-rashin"}, {name: "ryoku-prowl"}, {name: "rashind"}}
	}
	return []legacyHyakkiService{
		{name: "rashind.socket", activation: true},
		{name: "ryoku-ai-usage.timer", activation: true},
		{name: "rashind-routines.timer", activation: true},
		{name: "ryoku-ai-usage.service"}, {name: "rashind-routines.service"},
		{name: "ryoku-rashin.service"}, {name: "ryoku-prowl.service"},
		{name: "rashind.service"},
	}
}

// MigrateHyakki keeps supervisor rollback outside native migration and never rewrites consent
func (a *App) MigrateHyakki(retireBinDir string) error {
	if a.cfg.UID == 0 {
		return fmt.Errorf("run ryoku-host hyakki migrate as the desktop session user, without sudo or pkexec")
	}
	if !a.cfg.Runner.LookPath("hyakki") {
		return fmt.Errorf("install the approved Hyakki package before running ryoku-host hyakki migrate")
	}
	if err := a.validateLegacyHyakkiSocket(); err != nil {
		return err
	}
	if retireBinDir != "" {
		if a.cfg.UID != os.Getuid() || os.Geteuid() != os.Getuid() {
			return fmt.Errorf("development retirement requires the unprivileged desktop user")
		}
		dir, err := legacyHyakkiOwnedDirectory(retireBinDir, a.cfg.UID)
		if err != nil {
			return err
		}
		dir.Close()
	}
	init, err := a.Init()
	if err != nil {
		return err
	}
	guard, err := a.hyakkiMigrationGuard()
	if err != nil {
		return err
	}
	defer guard.Close()
	services := legacyHyakkiServices(init)
	var files []legacyHyakkiFile
	for i := range services {
		s := &services[i]
		s.enabled, s.enabledState, err = a.legacyHyakkiEnabled(s.name)
		if err != nil {
			return err
		}
		code := a.Service([]string{"--user", "is-active", s.name})
		if code != ExitOK && code != ExitAbsent && code != ExitFalse {
			return fmt.Errorf("cannot inspect legacy %s (is-active exited %d); repair the session supervisor before migration", s.name, code)
		}
		s.running = code == ExitOK
		if init == Runit && code == ExitAbsent {
			if _, err := os.Lstat(filepath.Join(a.userServiceDir(), s.name)); !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("legacy runit %s exists but its supervisor cannot be inspected; repair or remove it normally before migration (nothing was changed)", s.name)
			}
		}
		if init == Runit {
			for _, name := range []string{"down", runitUserDisabledMarker} {
				file, err := snapshotLegacyHyakkiFile(filepath.Join(a.userServiceDir(), s.name, name))
				if err != nil {
					return err
				}
				files = append(files, file)
			}
		}
	}
	return a.migrateHyakkiQuiesced(services, files, a.legacyHyakkiReleased, time.Sleep, retireBinDir)
}

func (a *App) legacyHyakkiEnabled(name string) (bool, string, error) {
	code := a.Service([]string{"--user", "is-enabled", name})
	if code == ExitAbsent || code == ExitFalse {
		return false, "", nil
	}
	if code != ExitOK {
		return false, "", fmt.Errorf("cannot inspect legacy %s enabled state", name)
	}
	init, err := a.Init()
	if err != nil {
		return false, "", err
	}
	if init == Runit {
		return true, "", nil
	}
	state, err := a.ServiceProperty("--user", name, "UnitFileState")
	if err != nil {
		return false, "", err
	}
	switch state {
	case "enabled", "enabled-runtime":
		return true, state, nil
	case "static", "indirect", "alias", "generated", "transient":
		return false, state, nil
	default:
		return false, "", fmt.Errorf("cannot safely change legacy %s unit state %q", name, state)
	}
}

func (a *App) migrateHyakkiQuiesced(services []legacyHyakkiService, files []legacyHyakkiFile, released func() (bool, error), sleep func(time.Duration), retireBinDir string) error {
	if err := a.validateLegacyHyakkiSocket(); err != nil {
		return err
	}
	initSystem, initErr := a.Init()
	if initErr != nil {
		return initErr
	}
	// Stopping a worker through systemd could force-kill it after the timeout
	for _, s := range services {
		if s.running && !s.activation && s.name != "rashind.service" {
			return fmt.Errorf("legacy %s is running; finish its work and close/stop it normally, then retry ryoku-host hyakki migrate (nothing was changed)", s.name)
		}
	}
	if busy, err := a.legacyHyakkiForeground(); err != nil {
		return err
	} else if busy {
		return fmt.Errorf("quit the old Rashin window after saving its input, then retry ryoku-host hyakki migrate (nothing was changed)")
	}
	daemonRunning := false
	for _, s := range services {
		if s.name == "rashind.service" && s.running {
			daemonRunning = true
			restart, err := a.ServiceProperty("--user", s.name, "Restart")
			if err != nil || restart != "no" {
				return fmt.Errorf("legacy rashind can be respawned by its supervisor; finish work and stop it normally before retrying ryoku-host hyakki migrate (nothing was changed)")
			}
		}
	}
	if busy, err := legacyHyakkiLockBusy(filepath.Join(a.legacyHyakkiRuntime(), "rashind.lock")); err != nil {
		return err
	} else if busy && !daemonRunning {
		return fmt.Errorf("an unmanaged legacy rashind owns its data; finish its work and let it exit normally before retrying ryoku-host hyakki migrate (nothing was changed)")
	}
	if initSystem == Systemd {
		for i := range services {
			var err error
			services[i].localLinks, err = a.legacyHyakkiLinks(services[i].name)
			if err != nil {
				return err
			}
		}
	}
	var masks []*legacyHyakkiMask
	var sockets []legacyHyakkiSocket
	rollback := func(cause error) error {
		var failures []string
		daemonBusy, daemonErr := legacyHyakkiLockBusy(filepath.Join(a.legacyHyakkiRuntime(), "rashind.lock"))
		if daemonErr != nil {
			failures = append(failures, daemonErr.Error())
		}
		for _, socket := range sockets {
			if daemonErr != nil || daemonBusy {
				if err := socket.restore(); err != nil {
					failures = append(failures, err.Error())
				}
			} else {
				socket.discard()
			}
		}
		for i := len(masks) - 1; i >= 0; i-- {
			if err := masks[i].restore(); err != nil {
				failures = append(failures, err.Error())
			}
		}
		for _, file := range files {
			if err := file.restore(); err != nil {
				failures = append(failures, err.Error())
			}
		}
		// Restore runit markers directly so enable cannot briefly start a down service
		for _, s := range services {
			if s.inherited {
				if err := restoreLegacyHyakkiLinks(s.localLinks); err != nil {
					failures = append(failures, err.Error())
				}
				continue
			}
			if s.enabled && len(files) == 0 {
				code := ExitOK
				if s.enabledState == "enabled-runtime" {
					code = commandExit(a.run("systemctl", a.systemdScopeArgs(true, "enable", "--runtime", s.name)...))
				} else {
					code = a.Service([]string{"--user", "enable", s.name})
				}
				if code != ExitOK {
					failures = append(failures, fmt.Sprintf("enable %s exited %d", s.name, code))
				}
			}
		}
		if len(masks) > 0 && a.Service([]string{"--user", "daemon-reload"}) != ExitOK {
			failures = append(failures, "reload restored legacy user definitions")
		}
		for i := len(services) - 1; i >= 0; i-- {
			s := services[i]
			if s.running && a.Service([]string{"--user", "is-active", s.name}) != ExitOK {
				if code := a.Service([]string{"--user", "start", s.name}); code != ExitOK {
					failures = append(failures, fmt.Sprintf("start %s exited %d", s.name, code))
				}
			}
		}
		for _, s := range services {
			enabled, state, err := a.legacyHyakkiEnabled(s.name)
			if err != nil || enabled != s.enabled || (s.enabled && s.enabledState != "" && state != s.enabledState) {
				failures = append(failures, fmt.Sprintf("%s did not return to its previous enabled state (%v)", s.name, err))
			}
			code := a.Service([]string{"--user", "is-active", s.name})
			if (code != ExitOK && code != ExitFalse && code != ExitAbsent) || (code == ExitOK) != s.running {
				failures = append(failures, fmt.Sprintf("%s did not return to its previous running state", s.name))
			}
		}
		if len(failures) != 0 {
			return fmt.Errorf("%w; legacy service restoration incomplete: %s", cause, strings.Join(failures, "; "))
		}
		return fmt.Errorf("%w; legacy enabled/running service state restored; app consent was not changed", cause)
	}
	// Keep an inherited socket alive until daemon exit so rollback restores the same listener
	var err error
	sockets, err = a.quarantineLegacyHyakkiSockets()
	if err != nil {
		return rollback(err)
	}
	for i := range services {
		s := &services[i]
		if s.enabled {
			code := ExitOK
			if s.enabledState == "enabled-runtime" {
				code = commandExit(a.run("systemctl", a.systemdScopeArgs(true, "disable", "--runtime", s.name)...))
			} else {
				code = a.Service([]string{"--user", "disable", s.name})
			}
			if code != ExitOK {
				return rollback(fmt.Errorf("disable legacy %s exited %d", s.name, code))
			}
			if initSystem == Systemd {
				mask, err := a.maskInheritedHyakkiUnit(s)
				if mask != nil {
					masks = append(masks, mask)
				}
				if err != nil {
					return rollback(err)
				}
			}
		}
		if s.activation && s.name != "rashind.socket" && s.running {
			if code := a.Service([]string{"--user", "stop", s.name}); code != ExitOK {
				return rollback(fmt.Errorf("stop legacy activator %s exited %d", s.name, code))
			}
		}
	}
	// Legacy shutdown cancels work and status omits Idle, so only the daemon can safely choose to exit
	attempts := int(a.legacyHyakkiIdleWait() / (100 * time.Millisecond))
	for attempt := 0; attempt <= attempts; attempt++ {
		ready, err := released()
		if err != nil {
			return rollback(err)
		}
		if ready {
			ownership, acquired, err := a.acquireLegacyHyakkiOwnership()
			if err != nil {
				return rollback(err)
			}
			if !acquired {
				if attempt < attempts {
					sleep(100 * time.Millisecond)
				}
				continue
			}
			fail := func(cause error) error {
				ownership.close()
				return rollback(cause)
			}
			for _, s := range services {
				code := a.Service([]string{"--user", "is-active", s.name})
				if !s.activation && code != ExitAbsent && code != ExitFalse {
					return fail(fmt.Errorf("legacy %s started work or cannot be inspected during quiescing; let it finish normally and retry ryoku-host hyakki migrate", s.name))
				}
			}
			for _, s := range services {
				if s.name == "rashind.socket" {
					code := a.Service([]string{"--user", "is-active", s.name})
					if code == ExitOK {
						if code := a.Service([]string{"--user", "stop", s.name}); code != ExitOK {
							return fail(fmt.Errorf("stop legacy activator %s exited %d", s.name, code))
						}
					} else if code != ExitAbsent && code != ExitFalse {
						return fail(fmt.Errorf("cannot inspect legacy activator %s before stopping it", s.name))
					}
				}
			}
			for _, s := range services {
				enabled, _, err := a.legacyHyakkiEnabled(s.name)
				if err != nil || enabled {
					return fail(fmt.Errorf("legacy %s still has activation enabled or cannot be inspected; disable its custom activation normally and retry ryoku-host hyakki migrate", s.name))
				}
				code := a.Service([]string{"--user", "is-active", s.name})
				if code != ExitAbsent && code != ExitFalse {
					return fail(fmt.Errorf("legacy %s is still active or cannot be inspected; let its owner exit normally before migration", s.name))
				}
				if initSystem == Runit && code == ExitAbsent {
					if _, err := os.Lstat(filepath.Join(a.userServiceDir(), s.name)); !errors.Is(err, os.ErrNotExist) {
						return fail(fmt.Errorf("legacy runit %s became uninspectable during migration; repair its supervisor before retrying", s.name))
					}
				}
			}
			// Native migration acquires these same locks itself. A contender in
			// this handoff must be refused by native migration, not bypassed.
			ownership.close()
			result := a.cfg.Runner.Run(Command{Name: "hyakki", Args: []string{"migrate"}, Stdin: a.cfg.Stdin, Stdout: a.cfg.Stdout, Stderr: a.cfg.Stderr})
			if result.Code != ExitOK {
				return rollback(fmt.Errorf("hyakki migrate exited %d; native migration must complete before legacy retirement", result.Code))
			}
			for _, socket := range sockets {
				socket.discard()
			}
			if retireBinDir != "" {
				if err := a.retireLegacyHyakkiDevelopment(retireBinDir, services); err != nil {
					return fmt.Errorf("Hyakki data migration completed; development retirement refused: %w; old activation was not restored; close the predecessor normally and retry with --retire-dev-binaries", err)
				}
			}
			return nil
		}
		if attempt < attempts {
			sleep(100 * time.Millisecond)
		}
	}
	return rollback(fmt.Errorf("legacy Rashin still owns its data; save and quit the old GUI, finish tasks/downloads/model runners and let rashind exit at its configured idle deadline; close any old API clients or armed routines normally, then retry ryoku-host hyakki migrate; no worker was killed"))
}

// Match the old daemon's configured idle interval, with its five-second
// shutdown grace. Longer configurations remain a manual close/wait prerequisite.
func (a *App) legacyHyakkiIdleWait() time.Duration {
	minutes := 5
	var settings struct {
		Daemon struct {
			IdleExitMinutes int `json:"idleExitMinutes"`
		} `json:"daemon"`
		Agent struct {
			IdleStopMinutes int `json:"idleStopMinutes"`
		} `json:"agent"`
	}
	dir := a.getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(a.home(), ".config")
	}
	if body, err := os.ReadFile(filepath.Join(dir, "ryoku", "rashin.json")); err == nil && json.Unmarshal(body, &settings) == nil {
		if settings.Daemon.IdleExitMinutes > 0 && settings.Daemon.IdleExitMinutes < minutes {
			minutes = settings.Daemon.IdleExitMinutes
		}
		if settings.Agent.IdleStopMinutes > 0 && settings.Agent.IdleStopMinutes < minutes {
			minutes = settings.Agent.IdleStopMinutes
		}
	}
	return time.Duration(minutes)*time.Minute + 5*time.Second
}

type legacyHyakkiSocket struct {
	original   string
	quarantine string
}

func (a *App) quarantineLegacyHyakkiSockets() ([]legacyHyakkiSocket, error) {
	if err := a.validateLegacyHyakkiSocket(); err != nil {
		return nil, err
	}
	paths := []string{filepath.Join(a.legacyHyakkiRuntime(), "rashin.sock")}
	var moved []legacyHyakkiSocket
	fail := func(cause error) ([]legacyHyakkiSocket, error) {
		for _, socket := range moved {
			cause = errors.Join(cause, socket.restore())
		}
		return nil, cause
	}
	for _, path := range paths {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fail(err)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || info.Mode()&os.ModeSocket == 0 || int(stat.Uid) != a.cfg.UID {
			return fail(fmt.Errorf("unsafe legacy listener %s; close its owner normally before migration", path))
		}
		dir, err := os.MkdirTemp(filepath.Dir(path), ".hyakki-migration-")
		if err != nil {
			return fail(err)
		}
		socket := legacyHyakkiSocket{original: path, quarantine: filepath.Join(dir, "listener")}
		if err := os.Rename(path, socket.quarantine); err != nil {
			_ = os.Remove(dir)
			return fail(err)
		}
		moved = append(moved, socket)
	}
	return moved, nil
}

func (a *App) validateLegacyHyakkiSocket() error {
	canonical := filepath.Join(a.legacyHyakkiRuntime(), "rashin.sock")
	if override := a.getenv("RASHIN_SOCKET"); override != "" && override != canonical {
		return fmt.Errorf("cannot correlate RASHIN_SOCKET %q with predecessor ownership locks; unset it and close its owner normally before migration (nothing was changed)", override)
	}
	return nil
}

type legacyHyakkiOwnership []*os.File

func (locks legacyHyakkiOwnership) close() {
	for _, lock := range locks {
		_ = lock.Close()
	}
}

// Open every component without following links. Only the selected directory
// must be user-owned; system ancestors such as /home and /tmp are expected.
func legacyHyakkiOwnedDirectory(path string, uid int) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("legacy migration requires a clean absolute directory: %s", path)
	}
	fd, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		next, err := syscall.Openat(fd, part, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
		_ = syscall.Close(fd)
		if err != nil {
			return nil, fmt.Errorf("inspect legacy migration directory %s: %w", path, err)
		}
		fd = next
	}
	f := os.NewFile(uintptr(fd), path)
	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		f.Close()
		return nil, err
	}
	if uid >= 0 && (int(stat.Uid) != uid || stat.Mode&0o022 != 0) {
		f.Close()
		return nil, fmt.Errorf("unsafe owned legacy migration directory %s", path)
	}
	return f, nil
}

// These are the predecessor's actual flock inodes, not the host helper's
// serial guard. Missing locks are created so a newly starting owner contends.
func (a *App) acquireLegacyHyakkiOwnership() (legacyHyakkiOwnership, bool, error) {
	runtime := a.legacyHyakkiRuntime()
	parent, err := legacyHyakkiOwnedDirectory(filepath.Dir(runtime), -1)
	if err != nil {
		return nil, false, err
	}
	defer parent.Close()
	if err := syscall.Mkdirat(int(parent.Fd()), filepath.Base(runtime), 0o700); err != nil && !errors.Is(err, syscall.EEXIST) {
		return nil, false, err
	}
	dir, err := legacyHyakkiOwnedDirectory(runtime, a.cfg.UID)
	if err != nil {
		return nil, false, err
	}
	defer dir.Close()
	var locks legacyHyakkiOwnership
	for _, name := range []string{"app.lock", "rashind.lock", "config.lock"} {
		fd, err := syscall.Openat(int(dir.Fd()), name, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
		if err != nil {
			locks.close()
			return nil, false, fmt.Errorf("acquire legacy ownership %s: %w", name, err)
		}
		f := os.NewFile(uintptr(fd), name)
		locks = append(locks, f)
		var stat syscall.Stat_t
		if err := syscall.Fstat(fd, &stat); err != nil {
			locks.close()
			return nil, false, err
		}
		if stat.Mode&syscall.S_IFMT != syscall.S_IFREG || int(stat.Uid) != a.cfg.UID {
			locks.close()
			return nil, false, fmt.Errorf("unsafe legacy ownership lock %s", name)
		}
		if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			locks.close()
			if errors.Is(err, syscall.EWOULDBLOCK) {
				return nil, false, nil
			}
			return nil, false, err
		}
	}
	return locks, true, nil
}

func (a *App) retireLegacyHyakkiDevelopment(binDir string, services []legacyHyakkiService) error {
	if a.cfg.UID == 0 || a.cfg.UID != os.Getuid() || os.Geteuid() != os.Getuid() {
		return fmt.Errorf("development retirement requires the unprivileged desktop user")
	}
	if !filepath.IsAbs(binDir) || filepath.Clean(binDir) != binDir {
		return fmt.Errorf("development retirement requires a clean absolute owned directory")
	}
	locks, acquired, err := a.acquireLegacyHyakkiOwnership()
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("a predecessor restarted and owns its data; its development files were preserved")
	}
	defer locks.close()
	init, err := a.Init()
	if err != nil {
		return err
	}
	for _, s := range services {
		enabled, _, err := a.legacyHyakkiEnabled(s.name)
		if err != nil || enabled {
			return fmt.Errorf("legacy %s has activation enabled or cannot be inspected; development files were preserved", s.name)
		}
		code := a.Service([]string{"--user", "is-active", s.name})
		if code != ExitAbsent && code != ExitFalse {
			return fmt.Errorf("legacy %s restarted or cannot be inspected; development files were preserved", s.name)
		}
		if init == Runit && code == ExitAbsent {
			if _, err := os.Lstat(filepath.Join(a.userServiceDir(), s.name)); !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("legacy runit %s exists but cannot be inspected; development files were preserved", s.name)
			}
		}
	}
	dataDir := a.getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(a.home(), ".local", "share")
	}
	paths := []string{}
	for _, name := range []string{"rashind", "rashin", "rashin-app", "ryoku-rashin", "ryoku-ai-usage"} {
		paths = append(paths, filepath.Join(binDir, name))
	}
	for _, name := range []string{"rashin-app", "dev.ryoku.rashin"} {
		paths = append(paths, filepath.Join(dataDir, "applications", name+".desktop"))
		paths = append(paths, filepath.Join(dataDir, "icons", "hicolor", "scalable", "apps", name+".svg"))
	}
	// Pin all parent directories and validate every target before removing any.
	// Unlinkat removes only these names, never recursively or through a symlink.
	type target struct {
		dir  *os.File
		name string
	}
	var targets []target
	defer func() {
		for _, target := range targets {
			target.dir.Close()
		}
	}()
	for _, path := range paths {
		dir, err := legacyHyakkiOwnedDirectory(filepath.Dir(path), a.cfg.UID)
		if errors.Is(err, os.ErrNotExist) && filepath.Dir(path) != binDir {
			continue
		}
		if err != nil {
			return err
		}
		name := filepath.Base(path)
		fd, err := unix.Openat(int(dir.Fd()), name, unix.O_PATH|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if errors.Is(err, os.ErrNotExist) {
			dir.Close()
			continue
		}
		if err != nil {
			dir.Close()
			return fmt.Errorf("inspect development file %s: %w", path, err)
		}
		var stat syscall.Stat_t
		err = syscall.Fstat(fd, &stat)
		if err != nil || int(stat.Uid) != a.cfg.UID {
			_ = syscall.Close(fd)
			dir.Close()
			return fmt.Errorf("unsafe development file %s", path)
		}
		valid := stat.Mode&syscall.S_IFMT == syscall.S_IFREG
		if stat.Mode&syscall.S_IFMT == syscall.S_IFLNK && filepath.Dir(path) == binDir && name == "rashin" {
			var link [64]byte
			n, linkErr := unix.Readlinkat(fd, "", link[:])
			valid = linkErr == nil && string(link[:n]) == "ryoku-rashin"
		}
		_ = syscall.Close(fd)
		if !valid {
			dir.Close()
			return fmt.Errorf("unsafe development file %s", path)
		}
		targets = append(targets, target{dir: dir, name: name})
	}
	for _, target := range targets {
		if err := syscall.Unlinkat(int(target.dir.Fd()), target.name); err != nil {
			return fmt.Errorf("retire development file %s: %w", target.name, err)
		}
	}
	return nil
}

func (s legacyHyakkiSocket) restore() error {
	// A live daemon must get its original listener inode back
	if _, err := os.Lstat(s.quarantine); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, err := os.Lstat(s.original); err == nil {
		return fmt.Errorf("legacy listener restoration incomplete: %s already exists; previous socket retained at %s", s.original, s.quarantine)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(s.quarantine, s.original); err != nil {
		return err
	}
	return os.Remove(filepath.Dir(s.quarantine))
}

func (s legacyHyakkiSocket) discard() {
	_ = os.Remove(s.quarantine)
	_ = os.Remove(filepath.Dir(s.quarantine))
}

func (a *App) legacyHyakkiRuntime() string {
	if dir := a.getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "rashin")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("rashin-%d", a.cfg.UID))
}

func legacyHyakkiLockBusy(path string) (bool, error) {
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect legacy lock %s: %w", path, err)
	}
	defer syscall.Close(fd)
	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		return false, err
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFREG || int(stat.Uid) != os.Getuid() {
		return false, fmt.Errorf("unsafe legacy ownership lock %s", path)
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func (a *App) legacyHyakkiForeground() (bool, error) {
	return legacyHyakkiLockBusy(filepath.Join(a.legacyHyakkiRuntime(), "app.lock"))
}

func (a *App) legacyHyakkiReleased() (bool, error) {
	for _, name := range []string{"app.lock", "rashind.lock", "config.lock"} {
		busy, err := legacyHyakkiLockBusy(filepath.Join(a.legacyHyakkiRuntime(), name))
		if err != nil || busy {
			return false, err
		}
	}
	return true, nil
}

func (a *App) hyakkiMigrationGuard() (*os.File, error) {
	dir := filepath.Join(a.runtimeDir(), "ryoku-hyakki-migration")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode().Perm()&0o077 != 0 || int(stat.Uid) != a.cfg.UID {
		return nil, fmt.Errorf("unsafe Hyakki migration runtime directory")
	}
	fd, err := syscall.Open(filepath.Join(dir, "operation.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "Hyakki migration guard")
	info, err = f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	stat, ok = info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || int(stat.Uid) != a.cfg.UID {
		f.Close()
		return nil, fmt.Errorf("unsafe Hyakki migration guard")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another session migration is running; wait for it to finish")
	}
	return f, nil
}

func snapshotLegacyHyakkiFile(path string) (legacyHyakkiFile, error) {
	f := legacyHyakkiFile{path: path}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if !info.Mode().IsRegular() {
		return f, fmt.Errorf("unsafe runit service marker %s", path)
	}
	f.body, err = os.ReadFile(path)
	f.mode, f.exists = info.Mode().Perm(), true
	return f, err
}

func (f legacyHyakkiFile) restore() error {
	if !f.exists {
		if err := os.Remove(f.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.WriteFile(f.path, f.body, f.mode); err != nil {
		return err
	}
	return os.Chmod(f.path, f.mode)
}
