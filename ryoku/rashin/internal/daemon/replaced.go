package daemon

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// replaced.go lets a daemon make way for a newer build of itself. An update
// installs a new rashind while the old one runs on, for as long as an app is
// connected, and that app then talks to code older than itself: what the
// update fixed stays broken until someone restarts the service by hand. So
// the daemon watches the file it was started from, and once that file is a
// different one and nothing is running, it exits. The next connection starts
// the new program, and clients reconnect by themselves.

// settle is how long the program file has to stay untouched before it counts
// as replaced: an installer writes it in pieces, and a half-written program
// must not be the one that starts next.
var settle = 2 * time.Second

type programWatch struct {
	path    string
	started os.FileInfo
	events  *os.File

	mu       sync.Mutex
	timer    *time.Timer
	replaced bool
}

// watchProgram reports through changed, once, that the file at path is no
// longer the one this process was started from. It costs nothing while the
// file is left alone: the kernel wakes it, nothing polls.
func watchProgram(path string, changed func()) (*programWatch, error) {
	started, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	descriptor, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, err
	}
	// the folder, not the file: a replacement is a new file under the old name
	mask := uint32(syscall.IN_CREATE | syscall.IN_MOVED_TO | syscall.IN_CLOSE_WRITE)
	if _, err := syscall.InotifyAddWatch(descriptor, filepath.Dir(path), mask); err != nil {
		_ = syscall.Close(descriptor)
		return nil, err
	}
	watch := &programWatch{path: path, started: started, events: os.NewFile(uintptr(descriptor), "inotify")}
	go watch.read(filepath.Base(path), changed)
	return watch, nil
}

func (w *programWatch) read(name string, changed func()) {
	buffer := make([]byte, 16*(syscall.SizeofInotifyEvent+syscall.NAME_MAX+1))
	for {
		n, err := w.events.Read(buffer)
		if err != nil {
			return
		}
		for at := 0; at+syscall.SizeofInotifyEvent <= n; {
			length := int(binary.NativeEndian.Uint32(buffer[at+12 : at+16]))
			end := at + syscall.SizeofInotifyEvent + length
			if end > n {
				break
			}
			if eventName(buffer[at+syscall.SizeofInotifyEvent:end]) == name {
				w.touched(changed)
			}
			at = end
		}
	}
}

func eventName(padded []byte) string {
	for index, value := range padded {
		if value == 0 {
			return string(padded[:index])
		}
	}
	return string(padded)
}

func (w *programWatch) touched(changed func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.replaced {
		return
	}
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(settle, func() {
		if !w.another() {
			return
		}
		w.mu.Lock()
		first := !w.replaced
		w.replaced = true
		w.mu.Unlock()
		if first {
			changed()
		}
	})
}

// another reports that a different program now stands at the path. A path
// that is gone or holds no program is an uninstall, not an update.
func (w *programWatch) another() bool {
	now, err := os.Stat(w.path)
	if err != nil || !now.Mode().IsRegular() || now.Mode().Perm()&0o111 == 0 {
		return false
	}
	same := os.SameFile(w.started, now) && now.ModTime().Equal(w.started.ModTime()) && now.Size() == w.started.Size()
	return !same
}

func (w *programWatch) isReplaced() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.replaced
}

func (w *programWatch) close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.mu.Unlock()
	_ = w.events.Close()
}

// retire exits a daemon whose program was replaced, once nothing is running:
// no turn, no agent process, no indexing. A session at rest loses nothing,
// its log is on disk and the new daemon serves it.
func (d *Daemon) retire() {
	d.mu.RLock()
	program := d.program
	d.mu.RUnlock()
	if !program.isReplaced() || d.manager.busy() ||
		d.deps.Indexer != nil && d.deps.Indexer.Busy() || d.deps.Work != nil && d.deps.Work.Busy() {
		return
	}
	d.retireOnce.Do(func() {
		d.warn("rashind was replaced on disk and nothing is running: exiting so the new one starts")
		go d.Shutdown()
	})
}
