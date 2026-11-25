// Package launch starts and stops oni-surface for the command line and the
// app. It never links the surface's display code, so any program may use it.
package launch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
)

// Binary is the surface program's name.
const Binary = "oni-surface"

// ErrNoDisplay means there is no Wayland session to show Oni in.
var ErrNoDisplay = errors.New(i18n.T("Oni needs a Wayland session: WAYLAND_DISPLAY is not set."))

// LockPath is the single-instance lock the running surface holds; it
// carries the holder's pid.
func LockPath() string { return filepath.Join(paths.RuntimeDir(), "oni-surface.lock") }

// LogPath is where a launched surface writes.
func LogPath() string { return filepath.Join(paths.StateDir(), "oni-surface.log") }

// Path finds oni-surface: HYAKKI_SURFACE, beside this executable, then PATH.
func Path() (string, error) {
	if p := os.Getenv("HYAKKI_SURFACE"); p != "" {
		return p, nil
	}
	if self, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(self), Binary)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return exec.LookPath(Binary)
}

// Lock is the held single-instance lock.
type Lock struct{ f *os.File }

// Acquire takes the lock for this process. held is false, with no error,
// when another surface already runs.
func Acquire() (lock *Lock, held bool, err error) {
	if err := paths.EnsureDir(paths.RuntimeDir()); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(LockPath(), os.O_CREATE|os.O_RDWR|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if err := f.Truncate(0); err == nil {
		f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
	}
	return &Lock{f: f}, true, nil
}

// Release drops the lock.
func (l *Lock) Release() {
	if l == nil || l.f == nil {
		return
	}
	l.f.Close()
	l.f = nil
}

// holder is the pid of the running surface, or 0 when none holds the lock.
func holder() (int, error) {
	f, err := os.Open(LockPath())
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err == nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		return 0, nil
	} else if !errors.Is(err, syscall.EWOULDBLOCK) {
		return 0, err
	}
	buf := make([]byte, 32)
	n, _ := f.ReadAt(buf, 0)
	pid, err := strconv.Atoi(strings.TrimSpace(string(buf[:n])))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("%s", i18n.Tf("The Oni surface lock names no process: %s.", LockPath()))
	}
	return pid, nil
}

// Running reports whether a surface holds the lock.
func Running() bool {
	pid, err := holder()
	return err == nil && pid > 0
}

// Launch starts oni-surface detached, unless one already runs.
func Launch(ctx context.Context) error {
	if pid, err := holder(); err != nil {
		return err
	} else if pid > 0 {
		return nil
	}
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return ErrNoDisplay
	}
	bin, err := Path()
	if err != nil {
		return err
	}
	if err := paths.EnsureDir(paths.StateDir()); err != nil {
		return err
	}
	logf, err := os.OpenFile(LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer devnull.Close()
	cmd := exec.Command(bin)
	cmd.Dir = paths.Home()
	cmd.Stdin = devnull
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// Stop asks the running surface to quit and waits until it has, or ctx ends.
// No surface running is not an error.
func Stop(ctx context.Context) error {
	pid, err := holder()
	if err != nil || pid == 0 {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		if p, err := holder(); err != nil || p == 0 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
