//go:build linux

package palette

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const watchMask = syscall.IN_CLOSE_WRITE | syscall.IN_MOVED_TO | syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF

// Watch follows palette file replacement and calls fn when the resolved
// palette changes. Only Follow reads those files, so an app watches while
// it follows the wallpaper.
func Watch(ctx context.Context, mode func() Mode, fn func(Palette)) error {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	targets := []string{palettePath(), themeSettingsPath(), shellSettingsPath()}
	watches := make(map[int]string)
	if err := armWatches(fd, watches, targets); err != nil {
		return err
	}

	events := make(chan struct{}, 1)
	failures := make(chan error, 1)
	go readWatchEvents(ctx, fd, watches, targets, events, failures)
	current := Load(mode())
	var timer *time.Timer
	var deadline <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-failures:
			if ctx.Err() != nil || errors.Is(err, syscall.EBADF) {
				return nil
			}
			return err
		case <-events:
			if timer == nil {
				timer = time.NewTimer(150 * time.Millisecond)
			} else if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(150 * time.Millisecond)
			deadline = timer.C
		case <-deadline:
			deadline = nil
			next := Load(mode())
			if !equalPalette(current, next) {
				current = next
				fn(next)
			}
		}
	}
}

func readWatchEvents(ctx context.Context, fd int, watches map[int]string, targets []string, events chan<- struct{}, failures chan<- error) {
	buffer := make([]byte, 16*1024)
	for {
		n, err := syscall.Read(fd, buffer)
		if err != nil {
			select {
			case failures <- err:
			case <-ctx.Done():
			}
			return
		}
		changed := false
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
				changed = true
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
					changed = true
					break
				}
			}
		}
		if changed {
			if err := armWatches(fd, watches, targets); err != nil {
				select {
				case failures <- err:
				case <-ctx.Done():
				}
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

func equalPalette(a, b Palette) bool {
	return a.Dark == b.Dark && a.Source == b.Source && maps.Equal(a.Roles, b.Roles)
}
