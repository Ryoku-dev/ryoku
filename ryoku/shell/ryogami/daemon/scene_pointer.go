package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// skwd-paper's controller owns the renderer's stdin and has no pointer request,
// so pointer lines go in through /proc, shaped as its Plasma host sends them

const (
	scenePointerInterval = 16 * time.Millisecond
	scenePointerRetry    = time.Second
)

var errScenePointerSink = errors.New("scene pointer sink replaced")

type scenePointerSink struct {
	pid int
	dev uint64
	ino uint64
}

type scenePointerFeed struct {
	mu         sync.Mutex
	sinks      map[string]scenePointerSink
	resolvedAt time.Time
	sent       map[string]time.Time
	last       map[string][2]uint16
	pending    map[string][2]uint16
	timers     map[string]*time.Timer

	workers func() ([]paperWorker, error)
	stdin   func(pid int) string
	now     func() time.Time
}

func newScenePointerFeed(workers func() ([]paperWorker, error)) *scenePointerFeed {
	return &scenePointerFeed{
		sinks:   map[string]scenePointerSink{},
		sent:    map[string]time.Time{},
		last:    map[string][2]uint16{},
		pending: map[string][2]uint16{},
		timers:  map[string]*time.Timer{},
		workers: workers,
		stdin:   func(pid int) string { return fmt.Sprintf("/proc/%d/fd/0", pid) },
		now:     time.Now,
	}
}

func quantizePointer(v float64) uint16 {
	if math.IsNaN(v) || v <= 0 {
		return 0
	}
	if v >= 1 {
		return math.MaxUint16
	}
	return uint16(math.Round(v * math.MaxUint16))
}

func scenePointerLine(pos [2]uint16) []byte {
	return []byte(fmt.Sprintf("{\"pointer\":{\"x\":%d,\"y\":%d,\"buttons\":0}}\n", pos[0], pos[1]))
}

func (f *scenePointerFeed) invalidate() {
	f.mu.Lock()
	f.sinks = map[string]scenePointerSink{}
	f.resolvedAt = time.Time{}
	f.last = map[string][2]uint16{}
	f.mu.Unlock()
}

// the newest position inside an interval is sent when it ends
func (f *scenePointerFeed) move(output string, x, y float64) {
	pos := [2]uint16{quantizePointer(x), quantizePointer(y)}
	f.mu.Lock()
	defer f.mu.Unlock()
	if last, ok := f.last[output]; ok && last == pos {
		delete(f.pending, output)
		return
	}
	if wait := scenePointerInterval - f.now().Sub(f.sent[output]); wait > 0 {
		f.pending[output] = pos
		if f.timers[output] == nil {
			f.timers[output] = time.AfterFunc(wait, func() { f.flush(output) })
		}
		return
	}
	f.sendLocked(output, pos)
}

func (f *scenePointerFeed) flush(output string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.timers, output)
	pos, ok := f.pending[output]
	if !ok {
		return
	}
	delete(f.pending, output)
	f.sendLocked(output, pos)
}

func (f *scenePointerFeed) sendLocked(output string, pos [2]uint16) {
	sink, ok := f.sinkLocked(output)
	if !ok {
		return
	}
	if err := f.write(sink, pos); err != nil {
		for name, s := range f.sinks {
			if s == sink {
				delete(f.sinks, name)
			}
		}
		return
	}
	f.sent[output] = f.now()
	f.last[output] = pos
}

func (f *scenePointerFeed) lookupLocked(output string) (scenePointerSink, bool) {
	if s, ok := f.sinks[output]; ok {
		return s, true
	}
	s, ok := f.sinks["*"]
	return s, ok
}

// a miss re-asks paper once per retry window, not once per motion event
func (f *scenePointerFeed) sinkLocked(output string) (scenePointerSink, bool) {
	if s, ok := f.lookupLocked(output); ok {
		return s, true
	}
	if !f.resolvedAt.IsZero() && f.now().Sub(f.resolvedAt) < scenePointerRetry {
		return scenePointerSink{}, false
	}
	f.resolveLocked()
	return f.lookupLocked(output)
}

func (f *scenePointerFeed) resolveLocked() {
	f.resolvedAt = f.now()
	f.sinks = map[string]scenePointerSink{}
	workers, err := f.workers()
	if err != nil {
		return
	}
	for _, w := range workers {
		if w.Source.Kind != "we" || w.Pid <= 0 {
			continue
		}
		var st syscall.Stat_t
		if syscall.Stat(f.stdin(w.Pid), &st) != nil || st.Mode&syscall.S_IFMT != syscall.S_IFIFO {
			continue
		}
		sink := scenePointerSink{pid: w.Pid, dev: uint64(st.Dev), ino: st.Ino}
		for _, out := range w.Outputs {
			f.sinks[out] = sink
		}
	}
}

// holding the pipe open would hide paper closing stdin, which is how it stops
// the renderer; the inode check keeps a recycled pid from getting the line
func (f *scenePointerFeed) write(sink scenePointerSink, pos [2]uint16) error {
	fd, err := syscall.Open(f.stdin(sink.pid), syscall.O_WRONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		return err
	}
	if uint64(st.Dev) != sink.dev || st.Ino != sink.ino {
		return errScenePointerSink
	}
	_, err = syscall.Write(fd, scenePointerLine(pos))
	return err
}

func (d *daemon) pointerVerb(line string) string {
	f := strings.Fields(line)
	if len(f) != 4 {
		return "err pointer expects <output> <x> <y>"
	}
	x, errX := strconv.ParseFloat(f[2], 64)
	y, errY := strconv.ParseFloat(f[3], 64)
	if errX != nil || errY != nil {
		return "err pointer: bad position"
	}
	if d.scenePointer != nil {
		d.scenePointer.move(f[1], x, y)
	}
	return "ok"
}
