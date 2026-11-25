package keys

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"syscall"
	"testing"
)

func anyDowns(b []byte) int {
	var part []byte
	return anyDecoder{}.feed(&part, b)
}

func TestEveryKeyDownCountsAndRepeatsAndReleasesDoNot(t *testing.T) {
	for name, tc := range map[string]struct {
		rec  *recording
		want int
	}{
		"a letter":                 {new(recording).press(codeA).release(codeA), 1},
		"a modifier alone":         {new(recording).press(keyLeftShift).release(keyLeftShift), 1},
		"a chord":                  {new(recording).press(keyLeftCtrl, keyLeftAlt, codeT).release(codeT, keyLeftAlt, keyLeftCtrl), 3},
		"Esc":                      {new(recording).press(keyEsc).release(keyEsc), 1},
		"a held key repeating":     {new(recording).press(codeA).key(codeA, valueRepeat).key(codeA, valueRepeat).release(codeA), 1},
		"releases only":            {new(recording).release(codeA, keyLeftCtrl), 0},
		"a code past KEY_CNT":      {new(recording).key(keyCount, valueDown), 0},
		"a dropped buffer":         {new(recording).event(evSyn, synDropped, 0), 1},
		"a press after a drop":     {new(recording).event(evSyn, synDropped, 0).press(codeA), 2},
		"scan codes and reports":   {new(recording).event(0x04, 0x04, 0x7001e).event(evSyn, 0, 0), 0},
		"two presses of one key":   {new(recording).press(codeA).release(codeA).press(codeA).release(codeA), 2},
		"a press with others held": {new(recording).press(keyLeftCtrl).press(codeA).release(codeA, keyLeftCtrl), 2},
	} {
		if got := anyDowns(tc.rec.b); got != tc.want {
			t.Errorf("%s: %d presses, want %d", name, got, tc.want)
		}
	}
	// one byte per read, as a slow pipe may deliver them
	var part []byte
	n := 0
	for _, c := range new(recording).press(codeA).release(codeA).press(codeT).b {
		n += anyDecoder{}.feed(&part, []byte{c})
	}
	if n != 2 {
		t.Fatalf("byte-wise reads gave %d presses", n)
	}
}

// The reader's callback and its decoder carry nothing that could name a key.
func TestTheAnyKeyReaderCannotSayWhichKey(t *testing.T) {
	if reflect.TypeOf(anyDecoder{}).NumField() != 0 {
		t.Fatal("the any-key decoder holds state")
	}
	ft := reflect.TypeOf(StartAnyKey).In(1)
	if ft.NumIn() != 0 || ft.NumOut() != 0 {
		t.Fatalf("onKey is %v, want func()", ft)
	}
}

func TestAnyKeySourceReadsEveryKeyboardAndStopClosesThem(t *testing.T) {
	devs := &fakeDevices{}
	host := Host{Sys: twoKeyboards(), Dev: "/dev/input", Open: devs.open}
	keys := make(chan struct{}, 16)
	src, err := StartAnyKey(Options{Host: host}, func() { keys <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer src.Stop()
	if src.Kind() != "evdev" || !slices.Equal(devs.opened, []string{"/dev/input/event3", "/dev/input/event6"}) {
		t.Fatalf("kind %s, opened %q", src.Kind(), devs.opened)
	}
	first, second := devs.pipe(t, "/dev/input/event3"), devs.pipe(t, "/dev/input/event6")
	first.Write(new(recording).press(codeA).key(codeA, valueRepeat).release(codeA).b)
	waitStop(t, keys, "a letter")
	noStop(t, keys, "a repeating letter")
	second.Write(new(recording).press(keyLeftShift).b)
	waitStop(t, keys, "Shift on the second keyboard")
	second.Write(new(recording).release(keyLeftShift).b)
	noStop(t, keys, "a release")

	src.Stop()
	for name, w := range map[string]*os.File{"event3": first, "event6": second} {
		if _, err := w.Write(new(recording).press(codeA).b); !errors.Is(err, syscall.EPIPE) {
			t.Fatalf("%s still open after Stop: write = %v", name, err)
		}
	}
	noStop(t, keys, "a key after Stop")
	src.Stop()
}

func TestAnyKeySourceSaysWhyWhenUnreadable(t *testing.T) {
	devs := &fakeDevices{denied: map[string]bool{"/dev/input/event3": true}}
	var un *Unavailable
	if _, err := StartAnyKey(Options{Host: devs.host()}, func() {}); !errors.As(err, &un) || un.Reason != ReasonPermission {
		t.Fatalf("unreadable keyboard: %v", err)
	}
	if _, err := StartAnyKey(Options{Host: (&fakeDevices{}).host()}, nil); err == nil {
		t.Fatal("an any-key source started with no onKey")
	}
}

func TestTheAnyKeyFIFOIsLabOnlyAndItsOwn(t *testing.T) {
	if AnyKeyFIFOEnv == "HYAKKI_ONI_KEYS_FIFO" || AnyKeyFIFOEnv == "HYAKKI_ONI_STOP_FIFO" {
		t.Fatalf("the any-key FIFO shares %s", AnyKeyFIFOEnv)
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "anykey.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("no mkfifo:", err)
	}
	env := map[string]string{AnyKeyFIFOEnv: fifo, "HYAKKI_ONI_KEYS_FIFO": "/tmp/talk.fifo", "HYAKKI_ONI_STOP_FIFO": "/tmp/stop.fifo"}
	getenv := func(k string) string { return env[k] }
	if o := AnyKeyOptions(getenv); o.FIFO != "" {
		t.Fatalf("outside the lab the FIFO is %q", o.FIFO)
	}
	devs := &fakeDevices{}
	for _, g := range []func(string) string{nil, getenv} {
		if _, err := StartAnyKey(Options{FIFO: fifo, Host: devs.host(), Getenv: g}, func() {}); !errors.Is(err, ErrFIFORefused) {
			t.Fatalf("a FIFO outside the lab: %v", err)
		}
	}
	if len(devs.opened) != 0 {
		t.Fatalf("a refused FIFO fell back to the keyboards: %q", devs.opened)
	}
	env["HYAKKI_ONI_LAB"] = "1"
	opts := AnyKeyOptions(getenv)
	if opts.FIFO != fifo {
		t.Fatalf("in the lab the FIFO is %q, want the any-key one", opts.FIFO)
	}
	keys := make(chan struct{}, 4)
	src, err := StartAnyKey(opts, func() { keys <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer src.Stop()
	if src.Kind() != "fifo" {
		t.Fatalf("kind %s", src.Kind())
	}
	w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.Write(new(recording).press(codeT).key(codeT, valueRepeat).release(codeT).b)
	waitStop(t, keys, "a key from the FIFO")
	noStop(t, keys, "a repeat from the FIFO")
}
