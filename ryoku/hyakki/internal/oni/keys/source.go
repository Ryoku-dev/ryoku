package keys

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	i18n "ryoku-i18n"
)

// Reason says why the key source cannot read the keyboards. Settings shows
// Message and offers the compositor bind instead.
type Reason string

const (
	// ReasonNoKeyboard: no input device has a keyboard's keys.
	ReasonNoKeyboard Reason = "no-keyboard"
	// ReasonPermission: keyboards exist but none could be opened; the
	// session has no uaccess on them and Hyakki ships no udev rule.
	ReasonPermission Reason = "permission"
	// ReasonUnsupported: this system has no evdev.
	ReasonUnsupported Reason = "unsupported"
)

// Message is the sentence Settings shows for r.
func (r Reason) Message() string {
	switch r {
	case ReasonNoKeyboard:
		return i18n.T("Oni found no keyboard to listen to for the talk chord. Bind hyakki oni talk start and stop in your desktop instead.")
	case ReasonPermission:
		return i18n.T("Oni may not read your keyboard, so the talk chord cannot work here. Bind hyakki oni talk start on press and hyakki oni talk stop on release in your desktop instead.")
	case ReasonUnsupported:
		return i18n.T("This system cannot read keys for the talk chord. Bind hyakki oni talk start and stop in your desktop instead.")
	}
	return ""
}

// Unavailable is the error Start returns when no keyboard can be read.
type Unavailable struct{ Reason Reason }

func (u *Unavailable) Error() string { return "keys: keyboards unavailable: " + string(u.Reason) }

// ErrFIFORefused: a FIFO key source was asked for outside the lab.
var ErrFIFORefused = errors.New("keys: a FIFO key source needs HYAKKI_ONI_LAB=1")

// Host is where keyboards are found and opened.
type Host struct {
	// Sys is the sysfs root; keyboards are the class/input/event* nodes
	// whose capabilities have a keyboard's keys.
	Sys fs.FS
	// Dev is the directory of the event nodes, /dev/input.
	Dev string
	// Open opens an event node for reading.
	Open func(name string) (io.ReadCloser, error)
	// watch: follow keyboards plugged in later (inotify on Dev).
	watch bool
}

// System is the running system's keyboards.
func System() Host {
	return Host{Sys: os.DirFS("/sys"), Dev: "/dev/input", watch: true,
		Open: func(name string) (io.ReadCloser, error) { return os.Open(name) }}
}

// Status is what Probe found: how many keyboards and, when none can be
// read, why.
type Status struct {
	Reason    Reason `json:"reason,omitempty"`
	Keyboards int    `json:"keyboards"`
}

// Probe tells whether the talk chord can be read here. It opens each
// keyboard node and closes it at once, reading nothing.
func Probe(h Host) Status {
	if !supported {
		return Status{Reason: ReasonUnsupported}
	}
	names := keyboards(h.Sys)
	st := Status{Keyboards: len(names)}
	if len(names) == 0 {
		st.Reason = ReasonNoKeyboard
		return st
	}
	for _, name := range names {
		if r, err := h.Open(filepath.Join(h.Dev, name)); err == nil {
			r.Close()
			return st
		}
	}
	st.Reason = ReasonPermission
	return st
}

// keyboards lists the event nodes whose capabilities have a keyboard's keys:
// key events with autorepeat, and Ctrl, the letters and Space.
func keyboards(sys fs.FS) []string {
	if sys == nil {
		return nil
	}
	entries, err := fs.ReadDir(sys, "class/input")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "event") && isKeyboard(sys, e.Name()) {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out
}

func isKeyboard(sys fs.FS, node string) bool {
	caps := path.Join("class/input", node, "device/capabilities")
	ev, ok := capBits(sys, path.Join(caps, "ev"))
	if !ok || !hasBit(ev, evKey) || !hasBit(ev, 0x14) { // EV_REP
		return false
	}
	key, ok := capBits(sys, path.Join(caps, "key"))
	if !ok {
		return false
	}
	for _, code := range []int{keyLeftCtrl, 30, 44, keySpace} { // KEY_A, KEY_Z
		if !hasBit(key, code) {
			return false
		}
	}
	return true
}

// capBits reads a sysfs capability bitmap: hex longs, most significant
// first, returned least significant first.
func capBits(sys fs.FS, name string) ([]uint64, bool) {
	raw, err := fs.ReadFile(sys, name)
	if err != nil {
		return nil, false
	}
	fields := strings.Fields(string(raw))
	words := make([]uint64, len(fields))
	for i, f := range fields {
		w, err := strconv.ParseUint(f, 16, 64)
		if err != nil {
			return nil, false
		}
		words[len(fields)-1-i] = w
	}
	return words, len(words) > 0
}

func hasBit(words []uint64, bit int) bool {
	return bit/64 < len(words) && words[bit/64]&(1<<(bit%64)) != 0
}

// Options configure Start, StartStop and StartAnyKey.
type Options struct {
	// Chord is the talk chord, as Settings stores it. StartStop and
	// StartAnyKey do not use it.
	Chord string
	// Host is where the keyboards are; the zero Host is System().
	Host Host
	// FIFO, lab only, reads the struct input_event bytes from a named
	// pipe instead of the keyboards.
	FIFO string
	// Getenv reads HYAKKI_ONI_LAB, which a FIFO needs.
	Getenv func(string) string
}

// Source reads the keyboards, or the lab FIFO, while it runs, and reports
// chord edges, or for StartStop each Esc press and for StartAnyKey each key
// press. Its readers sleep in the poller: no timer, no poll.
type Source struct {
	edge func(Edge)
	dec  *Decoder
	// onPress and presses are set for StartStop and StartAnyKey instead of
	// edge and dec
	onPress func()
	presses pressDecoder
	// report keeps onPress calls one at a time
	report sync.Mutex
	kind   string
	host   Host

	mu      sync.Mutex
	open    map[string]io.Closer
	stopped bool
	watch   io.Closer
}

// Kind is what the source reads: "evdev" or "fifo".
func (s *Source) Kind() string { return s.kind }

// Start reads keys for opts.Chord and calls edge, from the source's own
// goroutines one at a time, for every chord edge. It returns *Unavailable
// when no keyboard can be read, and ErrFIFORefused for a FIFO outside the
// lab.
func Start(opts Options, edge func(Edge)) (*Source, error) {
	chord, err := ParseChord(opts.Chord)
	if err != nil {
		return nil, err
	}
	return start(opts, &Source{edge: edge, dec: NewDecoder(chord)})
}

// start opens what opts names for s, whose decoder is set.
func start(opts Options, s *Source) (*Source, error) {
	s.open = map[string]io.Closer{}
	if opts.FIFO != "" {
		if opts.Getenv == nil || opts.Getenv("HYAKKI_ONI_LAB") != "1" {
			return nil, ErrFIFORefused
		}
		s.kind = "fifo"
		r, err := openFIFO(opts.FIFO)
		if err != nil {
			return nil, err
		}
		s.open[opts.FIFO] = r
		go s.readFIFO(opts.FIFO, r)
		return s, nil
	}
	s.kind = "evdev"
	s.host = opts.Host
	if s.host.Open == nil {
		s.host = System()
	}
	if !supported {
		return nil, &Unavailable{Reason: ReasonUnsupported}
	}
	names := keyboards(s.host.Sys)
	if len(names) == 0 {
		return nil, &Unavailable{Reason: ReasonNoKeyboard}
	}
	for _, name := range names {
		s.add(name)
	}
	if len(s.open) == 0 {
		return nil, &Unavailable{Reason: ReasonPermission}
	}
	if s.host.watch {
		if w, err := watchDir(s.host.Dev, s.plugged); err == nil {
			s.watch = w
		}
	}
	return s, nil
}

// Stop closes every device; no chord edge is reported after it returns.
// Stop does not wait for onPress, which may call it: a press read just
// before Stop may still reach onPress as Stop returns, and none read after
// does.
func (s *Source) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	for _, c := range s.open {
		c.Close()
	}
	s.open = nil
	if s.watch != nil {
		s.watch.Close()
	}
}

// add opens keyboard node name unless it is open already.
func (s *Source) add(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.open[name] != nil {
		return
	}
	r, err := s.host.Open(filepath.Join(s.host.Dev, name))
	if err != nil {
		return
	}
	s.open[name] = r
	go s.readDevice(name, r)
}

// plugged is called for a node created or given access in Dev.
func (s *Source) plugged(name string) {
	if strings.HasPrefix(name, "event") && isKeyboard(s.host.Sys, name) {
		s.add(name)
	}
}

func (s *Source) readDevice(name string, r io.ReadCloser) {
	var part []byte
	buf := make([]byte, 64*EventSize)
	for {
		n, err := r.Read(buf)
		if n > 0 && !s.feed(&part, buf[:n]) {
			return
		}
		if err != nil {
			// unplugged or closed by Stop
			s.mu.Lock()
			if s.open[name] == r {
				delete(s.open, name)
			}
			s.mu.Unlock()
			r.Close()
			return
		}
	}
}

// readFIFO reads the lab pipe, opening it again should its writers all go.
func (s *Source) readFIFO(name string, r io.ReadCloser) {
	var part []byte
	buf := make([]byte, 64*EventSize)
	for {
		n, err := r.Read(buf)
		if n > 0 && !s.feed(&part, buf[:n]) {
			return
		}
		if err == nil {
			continue
		}
		r.Close()
		s.mu.Lock()
		stopped := s.stopped
		s.mu.Unlock()
		if stopped || !errors.Is(err, io.EOF) {
			return
		}
		next, oerr := openFIFO(name)
		if oerr != nil {
			return
		}
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			next.Close()
			return
		}
		s.open[name], r, part = next, next, part[:0]
		s.mu.Unlock()
	}
}

// feed decodes b, a stream's next bytes, and reports the edges; false once
// the source is stopped.
func (s *Source) feed(part *[]byte, b []byte) bool {
	if s.presses != nil {
		return s.feedPress(part, b)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return false
	}
	s.dec.feed(part, b, s.edge)
	return true
}

// openFIFO opens the lab pipe for reading and writing, so it never reads
// end-of-file while the harness reopens its end, and refuses anything that
// is not a named pipe.
func openFIFO(name string) (io.ReadCloser, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeNamedPipe == 0 {
		return nil, fmt.Errorf("keys: %s is not a named pipe", name)
	}
	return os.OpenFile(name, os.O_RDWR, 0)
}
