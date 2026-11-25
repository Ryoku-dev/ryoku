package keys

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/fstest"
	"time"
)

func escDowns(d *escDecoder, b []byte) int {
	var part []byte
	return d.feed(&part, b)
}

func TestEscDownStopsOnceAndRepeatsDoNot(t *testing.T) {
	d := new(escDecoder)
	r := new(recording).press(keyEsc).key(keyEsc, valueRepeat).key(keyEsc, valueRepeat).key(keyEsc, valueDown)
	if n := escDowns(d, r.b); n != 1 {
		t.Fatalf("a held, repeating Esc stopped %d times", n)
	}
	if n := escDowns(d, new(recording).release(keyEsc).press(keyEsc).release(keyEsc).b); n != 1 {
		t.Fatalf("a second press stopped %d times", n)
	}
	held := new(recording).press(keyLeftCtrl, keyLeftShift, codeA).press(keyEsc).release(keyEsc, codeA, keyLeftShift, keyLeftCtrl)
	if n := escDowns(d, held.b); n != 1 {
		t.Fatalf("Esc with other keys held stopped %d times", n)
	}
	others := new(recording).press(keyEsc).press(keyLeftCtrl, codeT).release(codeT, keyLeftCtrl).key(keyEsc, valueRepeat).release(keyEsc)
	if n := escDowns(d, others.b); n != 1 {
		t.Fatalf("keys pressed while Esc was held stopped %d times", n)
	}
	if n := escDowns(d, new(recording).press(keyLeftCtrl, keyLeftAlt, codeA, keySpace).release(codeA, keySpace, keyLeftAlt, keyLeftCtrl).b); n != 0 {
		t.Fatalf("keys without Esc stopped %d times", n)
	}
}

func TestEscAcrossReadsKeyboardsAndDroppedBuffers(t *testing.T) {
	// one byte per read, as a slow pipe may deliver them
	d := new(escDecoder)
	var part []byte
	n := 0
	for _, c := range new(recording).press(keyEsc).release(keyEsc).press(keyEsc).release(keyEsc).b {
		n += d.feed(&part, []byte{c})
	}
	if n != 2 {
		t.Fatalf("byte-wise reads stopped %d times", n)
	}
	// Esc held on one keyboard and pressed on another is one press
	var a, b []byte
	d = new(escDecoder)
	n = d.feed(&a, new(recording).press(keyEsc).b) + d.feed(&b, new(recording).press(keyEsc).b)
	if n != 1 {
		t.Fatalf("Esc on two keyboards stopped %d times", n)
	}
	// an overrun forgets the held Esc: the next press counts, a repeat
	// still does not
	r := new(recording).event(evSyn, synDropped, 0).key(keyEsc, valueRepeat)
	if n := d.feed(&a, r.b); n != 0 {
		t.Fatalf("a repeat after an overrun stopped %d times", n)
	}
	if n := d.feed(&a, new(recording).release(keyEsc).press(keyEsc).b); n != 1 {
		t.Fatalf("a press after an overrun stopped %d times", n)
	}
}

// twoKeyboards is fakeSys with a second keyboard on event6.
func twoKeyboards() fstest.MapFS {
	sys := fakeSys()
	for _, name := range []string{"ev", "key"} {
		sys["class/input/event6/device/capabilities/"+name] = sys["class/input/event3/device/capabilities/"+name]
	}
	return sys
}

func (f *fakeDevices) pipe(t *testing.T, name string) *os.File {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	w := f.pipes[name]
	if w == nil {
		t.Fatalf("%s was never opened", name)
	}
	return w
}

func waitStop(t *testing.T, stops <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-stops:
	case <-time.After(5 * time.Second):
		t.Fatalf("no stop for %s", what)
	}
}

func noStop(t *testing.T, stops <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-stops:
		t.Fatalf("a stop for %s", what)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestStopSourceReadsEveryKeyboardAndStopClosesThem(t *testing.T) {
	devs := &fakeDevices{}
	host := Host{Sys: twoKeyboards(), Dev: "/dev/input", Open: devs.open}
	stops := make(chan struct{}, 8)
	src, err := StartStop(Options{Host: host}, func() { stops <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer src.Stop()
	if src.Kind() != "evdev" || !slices.Equal(devs.opened, []string{"/dev/input/event3", "/dev/input/event6"}) {
		t.Fatalf("kind %s, opened %q", src.Kind(), devs.opened)
	}
	first, second := devs.pipe(t, "/dev/input/event3"), devs.pipe(t, "/dev/input/event6")
	first.Write(new(recording).press(keyEsc).key(keyEsc, valueRepeat).key(keyEsc, valueRepeat).release(keyEsc).b)
	waitStop(t, stops, "Esc")
	noStop(t, stops, "a repeating Esc")
	second.Write(new(recording).press(keyLeftCtrl, codeA).press(keyEsc).b)
	waitStop(t, stops, "Esc with Ctrl+A held, on the second keyboard")
	first.Write(new(recording).press(keyLeftCtrl, keyLeftAlt).press(codeT).b)
	noStop(t, stops, "keys without Esc")

	src.Stop()
	for name, w := range map[string]*os.File{"event3": first, "event6": second} {
		if _, err := w.Write(new(recording).press(keyEsc).b); !errors.Is(err, syscall.EPIPE) {
			t.Fatalf("%s still open after Stop: write = %v", name, err)
		}
	}
	noStop(t, stops, "Esc after Stop")
	src.Stop() // twice is harmless
}

func TestOnStopMayStopTheSource(t *testing.T) {
	devs := &fakeDevices{}
	stopped := make(chan struct{}, 4)
	var (
		mu  sync.Mutex
		src *Source
		err error
	)
	mu.Lock()
	src, err = StartStop(Options{Host: devs.host()}, func() {
		mu.Lock()
		s := src
		mu.Unlock()
		s.Stop()
		stopped <- struct{}{}
	})
	mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	w := devs.pipe(t, "/dev/input/event3")
	// two presses in one read: the first stops the source, the second is
	// not reported
	w.Write(new(recording).press(keyEsc).release(keyEsc).press(keyEsc).b)
	waitStop(t, stopped, "Esc")
	noStop(t, stopped, "the press after Stop")
	if _, err := w.Write(new(recording).press(keyEsc).b); !errors.Is(err, syscall.EPIPE) {
		t.Fatalf("the keyboard is open after onStop stopped the source: %v", err)
	}
}

func TestStopSourceSaysWhyWhenUnreadable(t *testing.T) {
	devs := &fakeDevices{denied: map[string]bool{"/dev/input/event3": true}}
	var un *Unavailable
	if _, err := StartStop(Options{Host: devs.host()}, func() {}); !errors.As(err, &un) || un.Reason != ReasonPermission {
		t.Fatalf("unreadable keyboard: %v", err)
	}
	none := Host{Sys: fstest.MapFS{}, Dev: "/dev/input", Open: devs.open}
	if _, err := StartStop(Options{Host: none}, func() {}); !errors.As(err, &un) || un.Reason != ReasonNoKeyboard {
		t.Fatalf("no keyboard: %v", err)
	}
	if _, err := StartStop(Options{Host: (&fakeDevices{}).host()}, nil); err == nil {
		t.Fatal("a stop source started with no onStop")
	}
}

func TestTheStopFIFOIsLabOnly(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "keys.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("no mkfifo:", err)
	}
	devs := &fakeDevices{}
	for _, getenv := range []func(string) string{nil, func(string) string { return "" }, func(string) string { return "0" }} {
		if _, err := StartStop(Options{FIFO: fifo, Host: devs.host(), Getenv: getenv}, func() {}); !errors.Is(err, ErrFIFORefused) {
			t.Fatalf("a FIFO outside the lab: %v", err)
		}
	}
	if len(devs.opened) != 0 {
		t.Fatalf("a refused FIFO fell back to the keyboards: %q", devs.opened)
	}
	lab := func(k string) string {
		if k == "HYAKKI_ONI_LAB" {
			return "1"
		}
		return ""
	}
	plain := filepath.Join(dir, "plain")
	os.WriteFile(plain, nil, 0o600)
	if _, err := StartStop(Options{FIFO: plain, Getenv: lab}, func() {}); err == nil {
		t.Fatal("a regular file served as the FIFO")
	}
	stops := make(chan struct{}, 4)
	src, err := StartStop(Options{FIFO: fifo, Getenv: lab}, func() { stops <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer src.Stop()
	if src.Kind() != "fifo" {
		t.Fatalf("kind %s", src.Kind())
	}
	for range 2 {
		w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(new(recording).press(keyEsc).key(keyEsc, valueRepeat).release(keyEsc).b)
		w.Close()
		waitStop(t, stops, "Esc from the FIFO")
	}
	noStop(t, stops, "a repeat from the FIFO")
}

// countingFS counts every sysfs file opened.
type countingFS struct {
	fs.FS
	opens atomic.Int64
}

func (c *countingFS) Open(name string) (fs.File, error) {
	c.opens.Add(1)
	return c.FS.Open(name)
}

// readerStates returns the state of every goroutine reading a keyboard.
func readerStates() []string {
	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]
	var states []string
	for _, g := range strings.Split(string(buf), "\n\n") {
		if !strings.Contains(g, "oni/keys.(*Source).readDevice") {
			continue
		}
		head, _, _ := strings.Cut(g, "\n")
		_, state, _ := strings.Cut(head, "[")
		state, _, _ = strings.Cut(state, "]")
		states = append(states, state)
	}
	return states
}

func TestAnIdleStopSourceOnlySleepsInThePoller(t *testing.T) {
	devs := &fakeDevices{}
	sys := &countingFS{FS: twoKeyboards()}
	src, err := StartStop(Options{Host: Host{Sys: sys, Dev: "/dev/input", Open: devs.open}}, func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Stop()
	// both readers block in a read the poller wakes, not a sleep, a select
	// or a timer
	deadline := time.Now().Add(5 * time.Second)
	for {
		states := readerStates()
		if len(states) == 2 && states[0] == "IO wait" && states[1] == "IO wait" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("keyboard readers are %q, want two in IO wait", states)
		}
		time.Sleep(time.Millisecond)
	}
	devs.mu.Lock()
	opened := len(devs.opened)
	devs.mu.Unlock()
	sysOpens := sys.opens.Load()
	time.Sleep(200 * time.Millisecond)
	devs.mu.Lock()
	reopened := len(devs.opened)
	devs.mu.Unlock()
	if reopened != opened || sys.opens.Load() != sysOpens {
		t.Fatalf("an idle source looked again: devices %d→%d, sysfs %d→%d", opened, reopened, sysOpens, sys.opens.Load())
	}
	if states := readerStates(); len(states) != 2 || states[0] != "IO wait" || states[1] != "IO wait" {
		t.Fatalf("idle keyboard readers are %q", states)
	}
}
