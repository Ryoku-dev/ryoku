package keys

import (
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"testing/fstest"
	"time"
)

const (
	codeA = 30
	codeT = 20
)

// recording builds the bytes a keyboard writes: each key event followed by
// a SYN_REPORT, as struct input_event with a running timestamp.
type recording struct {
	b    []byte
	usec int64
}

func (r *recording) event(typ, code uint16, value int32) *recording {
	ev := make([]byte, EventSize)
	r.usec += 8000
	binary.NativeEndian.PutUint64(ev[0:8], uint64(1700000000+r.usec/1e6))
	binary.NativeEndian.PutUint64(ev[8:16], uint64(r.usec%1e6))
	binary.NativeEndian.PutUint16(ev[16:18], typ)
	binary.NativeEndian.PutUint16(ev[18:20], code)
	binary.NativeEndian.PutUint32(ev[20:24], uint32(value))
	r.b = append(r.b, ev...)
	return r
}

func (r *recording) key(code uint16, value int32) *recording {
	// a real keyboard sends EV_MSC MSC_SCAN first, then the key, then SYN
	r.event(0x04, 0x04, 0x70000+int32(code))
	r.event(evKey, code, value)
	return r.event(evSyn, 0, 0)
}

func (r *recording) press(codes ...uint16) *recording {
	for _, c := range codes {
		r.key(c, valueDown)
	}
	return r
}

func (r *recording) release(codes ...uint16) *recording {
	for _, c := range codes {
		r.key(c, valueUp)
	}
	return r
}

func decode(t *testing.T, chord string, b []byte) []Edge {
	t.Helper()
	c, err := ParseChord(chord)
	if err != nil {
		t.Fatal(err)
	}
	return NewDecoder(c).Feed(b)
}

func TestTheChordGoesDownAndUpOnce(t *testing.T) {
	for name, rec := range map[string]*recording{
		"released in reverse":     new(recording).press(keyLeftCtrl, keyLeftAlt).release(keyLeftAlt, keyLeftCtrl),
		"released in press order": new(recording).press(keyLeftCtrl, keyLeftAlt).release(keyLeftCtrl, keyLeftAlt),
		"alt first":               new(recording).press(keyLeftAlt, keyLeftCtrl).release(keyLeftAlt, keyLeftCtrl),
		"right ctrl":              new(recording).press(keyRightCtrl, keyLeftAlt).release(keyRightCtrl, keyLeftAlt),
	} {
		if got := decode(t, "ctrl+alt", rec.b); !slices.Equal(got, []Edge{Down, Up}) {
			t.Errorf("%s: %v, want down then up", name, got)
		}
	}
	// pressing again after letting go is a second talk; re-pressing a key
	// of a chord half let go is not
	rec := new(recording).press(keyLeftCtrl, keyLeftAlt).release(keyLeftAlt).press(keyLeftAlt).release(keyLeftAlt, keyLeftCtrl).
		press(keyLeftCtrl, keyLeftAlt).release(keyLeftCtrl, keyLeftAlt)
	if got := decode(t, "ctrl+alt", rec.b); !slices.Equal(got, []Edge{Down, Up, Down, Up}) {
		t.Fatalf("two presses: %v", got)
	}
}

func TestKeyRepeatIsIgnored(t *testing.T) {
	rec := new(recording).press(keyLeftCtrl, keyLeftAlt)
	for range 30 {
		rec.key(keyLeftAlt, valueRepeat)
		rec.key(keyLeftCtrl, valueRepeat)
	}
	rec.release(keyLeftAlt, keyLeftCtrl)
	if got := decode(t, "ctrl+alt", rec.b); !slices.Equal(got, []Edge{Down, Up}) {
		t.Fatalf("held with repeats: %v", got)
	}
	// a repeat of a key that was never seen going down does not press it
	rec = new(recording).key(keyLeftCtrl, valueRepeat).key(keyLeftAlt, valueRepeat).release(keyLeftAlt, keyLeftCtrl)
	if got := decode(t, "ctrl+alt", rec.b); len(got) != 0 {
		t.Fatalf("repeats alone: %v", got)
	}
}

func TestAltGrNeverCounts(t *testing.T) {
	for name, rec := range map[string]*recording{
		"altgr as lctrl+ralt": new(recording).press(keyLeftCtrl, keyRightAlt).release(keyRightAlt, keyLeftCtrl),
		"ralt then lctrl":     new(recording).press(keyRightAlt, keyLeftCtrl).release(keyLeftCtrl, keyRightAlt),
		"altgr then lalt":     new(recording).press(keyLeftCtrl, keyRightAlt, keyLeftAlt).release(keyLeftAlt, keyRightAlt, keyLeftCtrl),
		"altgr typing":        new(recording).press(keyLeftCtrl, keyRightAlt, codeA).release(codeA, keyRightAlt, keyLeftCtrl),
	} {
		if got := decode(t, "ctrl+alt", rec.b); len(got) != 0 {
			t.Errorf("%s: %v, want nothing", name, got)
		}
	}
	// AltGr struck while the chord is down spoils the press
	rec := new(recording).press(keyLeftCtrl, keyLeftAlt, keyRightAlt).release(keyRightAlt, keyLeftAlt, keyLeftCtrl)
	if got := decode(t, "ctrl+alt", rec.b); !slices.Equal(got, []Edge{Down, Cancel}) {
		t.Fatalf("altgr during the chord: %v", got)
	}
}

func TestPartialAndStrayChordsAreNotTalks(t *testing.T) {
	for name, rec := range map[string]*recording{
		"ctrl only":       new(recording).press(keyLeftCtrl).release(keyLeftCtrl),
		"alt only":        new(recording).press(keyLeftAlt).release(keyLeftAlt),
		"a letter":        new(recording).press(codeA).release(codeA),
		"shift extra":     new(recording).press(keyLeftShift, keyLeftCtrl, keyLeftAlt).release(keyLeftShift, keyLeftAlt, keyLeftCtrl),
		"letter held":     new(recording).press(codeA, keyLeftCtrl, keyLeftAlt).release(codeA, keyLeftAlt, keyLeftCtrl),
		"ctrl+t then alt": new(recording).press(keyLeftCtrl, codeT).release(codeT).press(keyLeftAlt).release(keyLeftAlt, keyLeftCtrl),
	} {
		if got := decode(t, "ctrl+alt", rec.b); len(got) != 0 {
			t.Errorf("%s: %v, want nothing", name, got)
		}
	}
	// ctrl+alt+t is a shortcut: the talk that began is cancelled, and
	// letting go of t with the chord still held does not start another
	rec := new(recording).press(keyLeftCtrl, keyLeftAlt, codeT).release(codeT).release(keyLeftAlt, keyLeftCtrl)
	if got := decode(t, "ctrl+alt", rec.b); !slices.Equal(got, []Edge{Down, Cancel}) {
		t.Fatalf("ctrl+alt+t: %v", got)
	}
	// a chord with a key goes down only with that key
	rec = new(recording).press(keyLeftAlt, keyLeftMeta).press(codeT).release(codeT, keyLeftMeta, keyLeftAlt)
	if got := decode(t, "alt+super+t", rec.b); !slices.Equal(got, []Edge{Down, Up}) {
		t.Fatalf("alt+super+t: %v", got)
	}
	rec = new(recording).press(keyLeftAlt, keyLeftMeta).release(keyLeftMeta, keyLeftAlt)
	if got := decode(t, "alt+super+t", rec.b); len(got) != 0 {
		t.Fatalf("alt+super without t: %v", got)
	}
}

func TestEventsSplitAcrossReadsAndDroppedBuffers(t *testing.T) {
	c, _ := ParseChord("ctrl+alt")
	d := NewDecoder(c)
	b := new(recording).press(keyLeftCtrl, keyLeftAlt).release(keyLeftAlt, keyLeftCtrl).b
	var got []Edge
	for i := 0; i < len(b); i += 7 {
		got = append(got, d.Feed(b[i:min(i+7, len(b))])...)
	}
	if !slices.Equal(got, []Edge{Down, Up}) {
		t.Fatalf("seven bytes at a time: %v", got)
	}
	// SYN_DROPPED while the chord is down: what is held is unknown, so the
	// press is cancelled and the stray releases that follow do nothing
	rec := new(recording).press(keyLeftCtrl, keyLeftAlt).event(evSyn, synDropped, 0).release(keyLeftAlt, keyLeftCtrl).
		press(keyLeftCtrl, keyLeftAlt).release(keyLeftAlt, keyLeftCtrl)
	if got := NewDecoder(c).Feed(rec.b); !slices.Equal(got, []Edge{Down, Cancel, Down, Up}) {
		t.Fatalf("dropped: %v", got)
	}
}

func TestRebindValidation(t *testing.T) {
	for chord, want := range map[string]string{
		"ctrl+alt":       "ctrl+alt",
		"Alt + Ctrl":     "ctrl+alt",
		"super+shift+f9": "shift+super+f9",
		"ctrl+space":     "ctrl+space",
		"alt+super+t":    "alt+super+t",
		"ctrl+alt+0":     "ctrl+alt+0",
		"super+f24":      "super+f24",
	} {
		c, err := ParseChord(chord)
		if err != nil || c.String() != want {
			t.Errorf("ParseChord(%q) = %q, %v; want %q", chord, c, err, want)
		}
	}
	for _, chord := range []string{"", "a", "ctrl", "shift+a", "ctrl+a+b", "altgr+a", "ctrl+alt+shift+super+a", "ctrl+f25", "ctrl+f0", "ctrl+é", "ctrl+ctrl", "ctrl+tab"} {
		if c, err := ParseChord(chord); err == nil {
			t.Errorf("ParseChord(%q) = %q, want a refusal", chord, c)
		}
	}
	// every function key reaches its evdev code
	for n, code := range map[int]uint16{1: 59, 10: 68, 11: 87, 12: 88, 13: 183, 24: 194} {
		if got := fCode(n); got != code {
			t.Errorf("F%d = %d, want %d", n, got, code)
		}
	}
}

// fakeSys is a sysfs with a keyboard on event3, a mouse on event4 and a
// power button on event5.
func fakeSys() fstest.MapFS {
	caps := func(ev, key string) map[string]string { return map[string]string{"ev": ev, "key": key} }
	nodes := map[string]map[string]string{
		// EV_SYN|EV_KEY|EV_MSC|EV_LED|EV_REP and a full keyboard
		"event3": caps("120013\n", "402000000 3803078f800d001 feffffdfffefffff fffffffffffffffe\n"),
		"event4": caps("17\n", "1f0000 0 0 0 0\n"),
		"event5": caps("3\n", "10000000000000 0\n"),
	}
	fsys := fstest.MapFS{}
	for node, files := range nodes {
		for name, body := range files {
			fsys["class/input/"+node+"/device/capabilities/"+name] = &fstest.MapFile{Data: []byte(body)}
		}
	}
	fsys["class/input/mouse0/device/name"] = &fstest.MapFile{Data: []byte("mouse\n")}
	return fsys
}

// fakeDevices opens pipes in place of event nodes, refusing those in denied.
type fakeDevices struct {
	mu     sync.Mutex
	opened []string
	denied map[string]bool
	pipes  map[string]*os.File
}

func (f *fakeDevices) host() Host {
	return Host{Sys: fakeSys(), Dev: "/dev/input", Open: f.open}
}

func (f *fakeDevices) open(name string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, name)
	if f.denied[name] {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	if f.pipes == nil {
		f.pipes = map[string]*os.File{}
	}
	f.pipes[name] = w
	return r, nil
}

func TestOnlyKeyboardsAreFoundAndOpened(t *testing.T) {
	if got := keyboards(fakeSys()); !slices.Equal(got, []string{"event3"}) {
		t.Fatalf("keyboards = %q", got)
	}
	devs := &fakeDevices{}
	if st := Probe(devs.host()); st != (Status{Keyboards: 1}) {
		t.Fatalf("probe = %+v", st)
	}
	edges := make(chan Edge, 4)
	src, err := Start(Options{Chord: "ctrl+alt", Host: devs.host()}, func(e Edge) { edges <- e })
	if err != nil {
		t.Fatal(err)
	}
	defer src.Stop()
	if src.Kind() != "evdev" || !slices.Equal(devs.opened, []string{"/dev/input/event3", "/dev/input/event3"}) {
		t.Fatalf("kind %s, opened %q", src.Kind(), devs.opened)
	}
	devs.mu.Lock()
	w := devs.pipes["/dev/input/event3"]
	devs.mu.Unlock()
	w.Write(new(recording).press(keyLeftCtrl, keyLeftAlt).release(keyLeftAlt, keyLeftCtrl).b)
	for _, want := range []Edge{Down, Up} {
		select {
		case got := <-edges:
			if got != want {
				t.Fatalf("edge %v, want %v", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no %v edge", want)
		}
	}
	src.Stop()
	w.Write(new(recording).press(keyLeftCtrl, keyLeftAlt).b)
	select {
	case e := <-edges:
		t.Fatalf("an edge after Stop: %v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestUnreadableKeyboardsSayWhy(t *testing.T) {
	devs := &fakeDevices{denied: map[string]bool{"/dev/input/event3": true}}
	st := Probe(devs.host())
	if st.Reason != ReasonPermission || st.Keyboards != 1 || st.Reason.Message() == "" {
		t.Fatalf("probe = %+v", st)
	}
	_, err := Start(Options{Chord: "ctrl+alt", Host: devs.host()}, func(Edge) {})
	var un *Unavailable
	if !errors.As(err, &un) || un.Reason != ReasonPermission {
		t.Fatalf("start = %v", err)
	}
	none := Host{Sys: fstest.MapFS{}, Dev: "/dev/input", Open: devs.open}
	if st := Probe(none); st.Reason != ReasonNoKeyboard || !strings.Contains(st.Reason.Message(), "hyakki oni talk") {
		t.Fatalf("no keyboard: %+v", st)
	}
	if _, err := Start(Options{Chord: "ctrl+alt", Host: none}, func(Edge) {}); !errors.As(err, &un) || un.Reason != ReasonNoKeyboard {
		t.Fatalf("start with no keyboard = %v", err)
	}
	if _, err := Start(Options{Chord: "shift+a", Host: devs.host()}, func(Edge) {}); err == nil || errors.As(err, &un) {
		t.Fatalf("a bad chord started: %v", err)
	}
}

func TestTheFIFOSourceIsLabOnly(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "chord.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("no mkfifo:", err)
	}
	devs := &fakeDevices{}
	for _, getenv := range []func(string) string{nil, func(string) string { return "" }, func(string) string { return "0" }} {
		if _, err := Start(Options{Chord: "ctrl+alt", FIFO: fifo, Host: devs.host(), Getenv: getenv}, func(Edge) {}); !errors.Is(err, ErrFIFORefused) {
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
	if _, err := Start(Options{Chord: "ctrl+alt", FIFO: plain, Getenv: lab}, func(Edge) {}); err == nil {
		t.Fatal("a regular file served as the FIFO")
	}
	edges := make(chan Edge, 8)
	src, err := Start(Options{Chord: "ctrl+alt", FIFO: fifo, Getenv: lab}, func(e Edge) { edges <- e })
	if err != nil {
		t.Fatal(err)
	}
	defer src.Stop()
	if src.Kind() != "fifo" {
		t.Fatalf("kind %s", src.Kind())
	}
	// the harness opens and closes its end per chord: the source keeps
	// reading across writers
	for range 2 {
		w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(new(recording).press(keyLeftCtrl, keyRightAlt).release(keyRightAlt, keyLeftCtrl).
			press(keyLeftCtrl, keyLeftAlt).release(keyLeftAlt, keyLeftCtrl).b)
		w.Close()
		for _, want := range []Edge{Down, Up} {
			select {
			case got := <-edges:
				if got != want {
					t.Fatalf("edge %v, want %v", got, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("no %v edge from the FIFO", want)
			}
		}
	}
}
