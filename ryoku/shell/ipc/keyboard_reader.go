package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

func keyboardEventPaths(sysRoot, devRoot string) ([]string, error) {
	entries, err := os.ReadDir(sysRoot)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, 2)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "event") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(sysRoot, name, "device", "capabilities", "key"))
		if err != nil || !isKeyboardCapabilities(string(raw)) {
			continue
		}
		path := filepath.Join(devRoot, name)
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func readKeyDevice(runCtx, deviceCtx context.Context, path string, readerID uint64, messages chan<- keyDeviceMessage) {
	// Nonblocking opens let os.File's poller interrupt Read on Close, including evdev.
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		sendKeyDeviceMessage(runCtx, messages, keyDeviceMessage{path: path, readerID: readerID, timestampUS: monotonicTimestampUS(), err: err})
		return
	}
	defer file.Close()
	if err := configureKeyEventClock(file); err != nil {
		sendKeyDeviceMessage(runCtx, messages, keyDeviceMessage{path: path, readerID: readerID, timestampUS: monotonicTimestampUS(), err: err})
		return
	}
	sendKeyDeviceMessage(runCtx, messages, keyDeviceMessage{path: path, readerID: readerID, opened: true, timestampUS: monotonicTimestampUS()})
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-deviceCtx.Done():
			_ = file.Close()
		case <-stopped:
		}
	}()
	var raw [linuxInputEventSize]byte
	for {
		if _, err := io.ReadFull(file, raw[:]); err != nil {
			message := keyDeviceMessage{path: path, readerID: readerID, timestampUS: monotonicTimestampUS()}
			if deviceCtx.Err() == nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				message.err = err
			}
			sendKeyDeviceMessage(runCtx, messages, message)
			return
		}
		if event, ok := decodeLinuxInputEvent(raw[:]); ok {
			sendKeyDeviceMessage(runCtx, messages, keyDeviceMessage{path: path, readerID: readerID, hasEvent: true, event: event, timestampUS: event.timestampUS})
		}
	}
}

// Watch the nearest existing ancestor, then rebuild after directory replacement.
// This also covers atomic saves and queue overflow without a reconciliation ticker.
func watchDirectoryChanges(ctx context.Context, directory string, changed chan<- struct{}) {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return
	}
	defer unix.Close(fd)
	stopFD, stop, err := quitEventFD(ctx.Done())
	if err != nil {
		return
	}
	defer func() { stop(); unix.Close(stopFD) }()
	const mask = unix.IN_CREATE | unix.IN_DELETE | unix.IN_MOVED_TO | unix.IN_MOVED_FROM | unix.IN_CLOSE_WRITE | unix.IN_ATTRIB | unix.IN_DELETE_SELF | unix.IN_MOVE_SELF
	wd := -1
	rebuild := func() {
		if wd >= 0 {
			_, _ = unix.InotifyRmWatch(fd, uint32(wd))
			wd = -1
		}
		path := directory
		for {
			if next, err := unix.InotifyAddWatch(fd, path, mask); err == nil {
				wd = next
				return
			}
			parent := filepath.Dir(path)
			if parent == path {
				return
			}
			path = parent
		}
	}
	rebuild()
	select {
	case changed <- struct{}{}:
	default:
	}
	var buf [8192]byte
	for {
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}, {Fd: int32(stopFD), Events: unix.POLLIN}}
		if _, err := unix.Poll(poll, -1); err != nil {
			if err == unix.EINTR {
				continue
			}
			return
		}
		if poll[1].Revents != 0 {
			return
		}
		dirty, lost := false, false
		for {
			n, err := unix.Read(fd, buf[:])
			if err != nil {
				break
			}
			for _, event := range decodeFileEvents(buf[:n]) {
				if event.mask&unix.IN_IGNORED != 0 && int(event.watch) != wd {
					continue
				}
				dirty = true
				if event.mask&(unix.IN_IGNORED|unix.IN_MOVE_SELF|unix.IN_DELETE_SELF|unix.IN_CREATE|unix.IN_MOVED_TO) != 0 {
					lost = true
				}
			}
		}
		if lost {
			rebuild()
		}
		if dirty {
			select {
			case changed <- struct{}{}:
			default:
			}
		}
	}
}

func runKeyboardReader(ctx context.Context, sysRoot, devRoot string, deliver func(keyDeviceMessage, int)) {
	messages := make(chan keyDeviceMessage, 80)
	changes := make(chan struct{}, 1)
	watchCtx, stopWatch := context.WithCancel(ctx)
	var workers sync.WaitGroup
	workers.Add(1)
	go func() { defer workers.Done(); watchDirectoryChanges(watchCtx, devRoot, changes) }()
	active := map[string]keyDeviceReader{}
	var serial uint64
	var healthy int
	refresh := func() {
		paths, err := keyboardEventPaths(sysRoot, devRoot)
		if err != nil {
			deliver(keyDeviceMessage{err: err}, healthy)
			return
		}
		present := make(map[string]bool, len(paths))
		for _, path := range paths {
			present[path] = true
			if _, found := active[path]; found {
				continue
			}
			serial++
			deviceCtx, cancel := context.WithCancel(ctx)
			active[path] = keyDeviceReader{id: serial, cancel: cancel}
			workers.Add(1)
			go func(path string, id uint64) { defer workers.Done(); readKeyDevice(ctx, deviceCtx, path, id, messages) }(path, serial)
		}
		for path, reader := range active {
			if !present[path] {
				reader.cancel()
			}
		}
		if len(paths) == 0 {
			deliver(keyDeviceMessage{}, 0)
		}
	}
	pending := make([]keyDeviceMessage, 0, 8)
	var timer *time.Timer
	var ready <-chan time.Time
	defer func() {
		stopWatch()
		if timer != nil {
			timer.Stop()
		}
		for _, reader := range active {
			reader.cancel()
		}
		workers.Wait()
	}()
	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-changes:
			refresh()
		case message := <-messages:
			reader, exists := active[message.path]
			if !exists || reader.id != message.readerID {
				continue
			}
			pending = insertOrderedKeyMessage(pending, message)
			if ready == nil {
				if timer == nil {
					timer = time.NewTimer(keyEventReorderDelay)
				} else {
					timer.Reset(keyEventReorderDelay)
				}
				ready = timer.C
			}
		case <-ready:
			pending = drainKeyDeviceBatch(messages, active, pending)
			for _, message := range pending {
				reader, exists := active[message.path]
				if !exists || reader.id != message.readerID {
					continue
				}
				if message.opened {
					if !reader.opened {
						healthy++
					}
					reader.opened = true
					active[message.path] = reader
				} else if !message.hasEvent {
					if reader.opened {
						healthy--
					}
					reader.cancel()
					delete(active, message.path)
				}
				deliver(message, healthy)
			}
			pending = pending[:0]
			ready = nil
		}
	}
}
