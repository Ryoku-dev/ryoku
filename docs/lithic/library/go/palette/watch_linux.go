//go:build linux

package palette

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const watchMask = syscall.IN_CLOSE_WRITE | syscall.IN_MOVED_TO | syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF

// settle coalesces a burst of file events (an atomic replace is a create, a write and a
// rename) into one reading.
const settle = 150 * time.Millisecond

// Watch follows palette file replacement and calls fn when the resolved
// palette changes. Only Follow reads those files, so an app watches while
// it follows the wallpaper. WatchDesktop follows everything the desktop asks for.
func Watch(ctx context.Context, mode func() Mode, fn func(Palette)) error {
	var current Palette
	return watchFiles(ctx, []string{palettePath(), themeSettingsPath(), shellSettingsPath()},
		func() { current = Load(mode()) },
		func() {
			next := Load(mode())
			if !equalPalette(current, next) {
				current = next
				fn(next)
			}
		})
}

// WatchDesktop follows the desktop's look and calls fn with a new reading whenever it
// differs from the last one (Desktop.Equal), so the daemon's many shell.json writes that
// do not touch the look call nothing. It watches shell.json, theme.json and matugen.json in
// the ryoku config directory and colors.json in the ryoku cache directory, always: on
// Ryoku's colours, light or dark still follows colors.json's surface under matugen's smart
// and sun modes. Events are coalesced over 150 ms, so an atomic replace calls fn once.
//
// It reads the desktop once when its watches are armed and compares later readings with
// that one; it does not call fn for it. Read the first look with ReadDesktop before
// starting the watch. fn runs on the watching goroutine; hand the result to the window's
// goroutine (build a theme with theme.NewFor and invalidate the window). WatchDesktop
// blocks until ctx ends, then returns nil. At rest it costs one blocked read.
func WatchDesktop(ctx context.Context, fn func(Desktop), opts ...Option) error {
	c := newReadConfig(opts)
	var current Desktop
	return watchFiles(ctx, c.targets(),
		func() {
			current = readDesktop(c)
			if c.armed != nil {
				c.armed()
			}
		},
		func() {
			next := readDesktop(c)
			if !next.Equal(current) {
				current = next
				fn(next)
			}
		})
}

// watchFiles watches the parent directories of targets (re-arming when a directory
// appears or goes), calls armed once the watches are in place and changed once per
// settled burst of events that touch a target.
func watchFiles(ctx context.Context, targets []string, armed, changed func()) error {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return err
	}
	// a pollable file, so closing it wakes the reader instead of leaving it blocked on a
	// descriptor number the process may reuse
	file := os.NewFile(uintptr(fd), "inotify")
	defer file.Close()

	conn, err := file.SyscallConn()
	if err != nil {
		return err
	}
	// arm (re)places the directory watches; Control holds the descriptor open meanwhile
	watches := make(map[int]string)
	arm := func() error {
		var armErr error
		if err := conn.Control(func(fd uintptr) { armErr = armWatches(int(fd), watches, targets) }); err != nil {
			return err
		}
		return armErr
	}
	if err := arm(); err != nil {
		return err
	}

	events := make(chan struct{}, 1)
	failures := make(chan error, 1)
	go readWatchEvents(ctx, file, arm, watches, targets, events, failures)
	armed()
	timer := time.NewTimer(settle)
	timer.Stop()
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-failures:
			if ctx.Err() != nil || errors.Is(err, os.ErrClosed) {
				return nil
			}
			return err
		case <-events:
			timer.Reset(settle)
		case <-timer.C:
			changed()
		}
	}
}

func readWatchEvents(ctx context.Context, file *os.File, arm func() error, watches map[int]string, targets []string, events chan<- struct{}, failures chan<- error) {
	buffer := make([]byte, 16*1024)
	fail := func(err error) {
		select {
		case failures <- err:
		case <-ctx.Done():
		}
	}
	for {
		n, err := file.Read(buffer)
		if err != nil {
			fail(err)
			return
		}
		touched := false
		for offset := 0; offset+syscall.SizeofInotifyEvent <= n; {
			event := (*syscall.InotifyEvent)(unsafe.Pointer(&buffer[offset]))
			name := ""
			if event.Len != 0 {
				raw := buffer[offset+syscall.SizeofInotifyEvent : offset+syscall.SizeofInotifyEvent+int(event.Len)]
				for len(raw) > 0 && raw[len(raw)-1] == 0 {
					raw = raw[:len(raw)-1]
				}
				name = string(raw)
			}
			offset += syscall.SizeofInotifyEvent + int(event.Len)
			dir, ok := watches[int(event.Wd)]
			if !ok {
				continue
			}
			if event.Mask&(syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF|syscall.IN_IGNORED) != 0 {
				delete(watches, int(event.Wd))
				touched = true
				continue
			}
			for _, target := range targets {
				relative, err := filepath.Rel(dir, target)
				if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) {
					continue
				}
				first := relative
				if separator := firstSeparator(relative); separator >= 0 {
					first = relative[:separator]
				}
				if name == first {
					touched = true
					break
				}
			}
		}
		if touched {
			if err := arm(); err != nil {
				fail(err)
				return
			}
			select {
			case events <- struct{}{}:
			default:
			}
		}
	}
}

func armWatches(fd int, watches map[int]string, targets []string) error {
	desired := make(map[string]bool)
	for _, target := range targets {
		desired[nearestDirectory(filepath.Dir(target))] = true
	}
	for descriptor, dir := range watches {
		if desired[dir] {
			delete(desired, dir)
			continue
		}
		_, _ = syscall.InotifyRmWatch(fd, uint32(descriptor))
		delete(watches, descriptor)
	}
	for dir := range desired {
		descriptor, err := syscall.InotifyAddWatch(fd, dir, watchMask)
		if err != nil {
			return err
		}
		watches[descriptor] = dir
	}
	return nil
}

func nearestDirectory(path string) string {
	for {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return parent
		}
		path = parent
	}
}

func firstSeparator(path string) int {
	for index := range len(path) {
		if os.IsPathSeparator(path[index]) {
			return index
		}
	}
	return -1
}
