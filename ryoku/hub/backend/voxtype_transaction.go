package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type voxtypeFileSnapshot struct {
	path   string
	exists bool
	mode   os.FileMode
	data   []byte
	link   string
}

type voxtypeUserServiceSnapshot struct {
	name              string
	tracked           bool
	definitionPresent bool
	servicePresent    bool
	enabled           bool
	active            bool
}

type voxtypeSetSnapshot struct {
	files    []voxtypeFileSnapshot
	services []voxtypeUserServiceSnapshot
}

// withVoxtypeMutationLock keeps selection, service startup and model removal
// from taking snapshots or rolling back across another Hub process' changes.
func withVoxtypeMutationLock(fn func() error) error {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find the user state directory: %w", err)
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	stateHome, err := filepath.Abs(stateHome)
	if err != nil {
		return fmt.Errorf("resolve the user state directory: %w", err)
	}
	path := filepath.Join(stateHome, "ryoku", "locks", "voxtype.lock")
	if err := validateVoxtypeLockPath(stateHome, path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create the dictation lock directory: %w", err)
	}
	if err := validateVoxtypeLockPath(stateHome, path); err != nil {
		return err
	}
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("open the dictation operation lock: %w", err)
	}
	lock := os.NewFile(uintptr(fd), path)
	defer lock.Close()
	info, err := lock.Stat()
	if err != nil {
		return fmt.Errorf("inspect the dictation operation lock: %w", err)
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || !ok || owner.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("dictation operation lock is not a user-owned regular file")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock dictation changes: %w", err)
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	return fn()
}

func validateVoxtypeLockPath(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("dictation lock is outside the configured state directory")
	}
	for current := path; current != root; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect dictation lock path %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink in dictation lock path: %s", current)
		}
	}
	return nil
}

func snapshotVoxtypeSetState() (*voxtypeSetSnapshot, error) {
	snapshot := &voxtypeSetSnapshot{}
	for _, path := range []string{
		voxtypeConfigPath(), voxtypeOpenAIKeyPath(),
		reduxDependencyDropinPath(), phononDependencyDropinPath(),
		voxtypeUnitPath(), voxtypeResilienceDropinPath(),
	} {
		file, err := snapshotVoxtypeFile(path)
		if err != nil {
			return nil, err
		}
		snapshot.files = append(snapshot.files, file)
	}
	initSystem := hostInitName()
	for _, name := range []string{"phonon-2", "voxtype-redux", "voxtype"} {
		service, err := snapshotVoxtypeUserService(initSystem, name)
		if err != nil {
			return nil, err
		}
		snapshot.services = append(snapshot.services, service)
	}
	return snapshot, nil
}

func snapshotVoxtypeFile(path string) (voxtypeFileSnapshot, error) {
	snapshot := voxtypeFileSnapshot{path: path}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, fmt.Errorf("inspect %s before changing dictation settings: %w", path, err)
	}
	snapshot.exists = true
	snapshot.mode = info.Mode()
	if info.Mode()&os.ModeSymlink != 0 {
		snapshot.link, err = os.Readlink(path)
	} else if info.Mode().IsRegular() {
		snapshot.data, err = os.ReadFile(path)
	}
	if err != nil {
		return snapshot, fmt.Errorf("read %s before changing dictation settings: %w", path, err)
	}
	return snapshot, nil
}

func (s voxtypeFileSnapshot) restore() (bool, error) {
	current, err := os.Lstat(s.path)
	if !s.exists {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("inspect new dictation file %s during rollback: %w", s.path, err)
		}
		if current.IsDir() {
			return false, fmt.Errorf("refusing to remove directory created at dictation file path %s", s.path)
		}
		if err := os.Remove(s.path); err != nil {
			return false, fmt.Errorf("remove new dictation file %s: %w", s.path, err)
		}
		return true, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
		return false, fmt.Errorf("inspect dictation file %s during rollback: %w", s.path, err)
	}
	if s.mode&os.ModeSymlink != 0 {
		if err == nil && current.Mode()&os.ModeSymlink != 0 {
			link, readErr := os.Readlink(s.path)
			if readErr == nil && link == s.link {
				return false, nil
			}
		}
		if err == nil {
			if current.IsDir() {
				return false, fmt.Errorf("refusing to replace directory at dictation path %s", s.path)
			}
			if removeErr := os.Remove(s.path); removeErr != nil {
				return false, fmt.Errorf("remove changed dictation path %s: %w", s.path, removeErr)
			}
		}
		if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
			return false, fmt.Errorf("restore parent of dictation path %s: %w", s.path, err)
		}
		if err := os.Symlink(s.link, s.path); err != nil {
			return false, fmt.Errorf("restore dictation symlink %s: %w", s.path, err)
		}
		return true, nil
	}
	if !s.mode.IsRegular() {
		return false, nil
	}
	if err == nil && current.Mode().IsRegular() {
		data, readErr := os.ReadFile(s.path)
		if readErr == nil && bytes.Equal(data, s.data) && current.Mode().Perm() == s.mode.Perm() {
			return false, nil
		}
	}
	if err := atomicWrite(s.path, s.data, s.mode.Perm()); err != nil {
		return false, fmt.Errorf("restore dictation file %s: %w", s.path, err)
	}
	return true, nil
}

func snapshotVoxtypeUserService(initSystem, name string) (voxtypeUserServiceSnapshot, error) {
	snapshot := voxtypeUserServiceSnapshot{name: name}
	path := ""
	switch initSystem {
	case "systemd":
		path = filepath.Join(configHome(), "systemd", "user", name+".service")
		snapshot.definitionPresent = fileExists(path)
	case "runit":
		path = filepath.Join(runitUserServicePath(name), "run")
		snapshot.definitionPresent = fileExists(path)
		if !snapshot.definitionPresent {
			return snapshot, nil
		}
	default:
		return snapshot, nil
	}
	var err error
	snapshot.enabled, err = queryVoxtypeServiceState("is-enabled", name)
	if err != nil {
		if serviceStateIsAbsent(err) {
			// A packaged systemd unit can be known to the user manager without a
			// user-local definition. Conversely, an installed Voxtype binary with
			// no known unit may create one during selection, so keep a cleanup
			// snapshot for that case too.
			if initSystem == "systemd" && name == "voxtype" && onPath("voxtype") {
				snapshot.tracked = true
			}
			return snapshot, nil
		}
		return snapshot, err
	}
	snapshot.tracked = true
	snapshot.servicePresent = true
	snapshot.active, err = queryVoxtypeServiceState("is-active", name)
	if err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func queryVoxtypeServiceState(verb, name string) (bool, error) {
	err := exec.Command("ryoku-host", "svc", "--user", verb, name).Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 {
		return false, nil
	}
	return false, fmt.Errorf("query %s state for %s: %w", verb, name, err)
}

func (s *voxtypeSetSnapshot) restore() error {
	var rollbackErrors []error
	for i := len(s.services) - 1; i >= 0; i-- {
		snapshot := &s.services[i]
		if !snapshot.tracked || snapshot.definitionPresent || snapshot.servicePresent ||
			snapshot.name != "voxtype" || hostInitName() != "systemd" {
			continue
		}
		if fileExists(voxtypeUnitPath()) {
			if err := userService("disable", "--now", snapshot.name); err != nil && !serviceStateIsAbsent(err) {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("disable newly created Voxtype service during rollback: %w", err))
			}
		}
		snapshot.tracked = false
	}
	changed := false
	for i := len(s.files) - 1; i >= 0; i-- {
		fileChanged, err := s.files[i].restore()
		changed = changed || fileChanged
		if err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if changed && hostInitName() == "systemd" {
		if err := userService("daemon-reload"); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("reload restored Voxtype dependencies: %w", err))
		}
	}
	for _, snapshot := range s.services {
		if !snapshot.tracked {
			continue
		}
		if err := snapshot.restoreEnabledState(); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	for _, snapshot := range s.services {
		if !snapshot.tracked {
			continue
		}
		if err := snapshot.restoreActiveState(true); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	return errors.Join(rollbackErrors...)
}

func serviceStateIsAbsent(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 4
}

func (s voxtypeUserServiceSnapshot) restoreEnabledState() error {
	if !s.tracked {
		return nil
	}
	current, err := queryVoxtypeServiceState("is-enabled", s.name)
	if err != nil {
		return err
	}
	if current == s.enabled {
		return nil
	}
	action := "disable"
	if s.enabled {
		action = "enable"
	}
	if err := userService(action, s.name); err != nil {
		return fmt.Errorf("restore %s enabled state: %w", s.name, err)
	}
	return nil
}

func (s voxtypeUserServiceSnapshot) restoreActiveState(restartVoxtype bool) error {
	if !s.tracked {
		return nil
	}
	current, err := queryVoxtypeServiceState("is-active", s.name)
	if err != nil {
		return err
	}
	if restartVoxtype && s.name == "voxtype" && s.active {
		action := "restart"
		if !current {
			action = "start"
		}
		if err := userService(action, s.name); err != nil {
			return fmt.Errorf("restore Voxtype with the previous settings: %w", err)
		}
		return nil
	}
	if current == s.active {
		return nil
	}
	action := "stop"
	if s.active {
		action = "start"
	}
	if err := userService(action, s.name); err != nil {
		return fmt.Errorf("restore %s active state: %w", s.name, err)
	}
	return nil
}
