package window

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDesk stands in for every compositor command: each allowed argv maps to
// a recorded fixture file, and every call is logged so tests can see exactly
// what a provider asked for. Nothing real is ever started.
type fakeDesk struct {
	mu      sync.Mutex
	files   map[string]string
	streams map[string]string
	calls   [][]string
}

func newDesk(files map[string]string) *fakeDesk {
	return &fakeDesk{files: files, streams: map[string]string{}}
}

func argvKey(name string, args []string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

func (d *fakeDesk) set(key, file string) {
	d.mu.Lock()
	d.files[key] = file
	d.mu.Unlock()
}

func (d *fakeDesk) record(name string, args []string) {
	d.mu.Lock()
	d.calls = append(d.calls, append([]string{name}, args...))
	d.mu.Unlock()
}

func (d *fakeDesk) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.calls)
}

func (d *fakeDesk) run(_ context.Context, name string, args ...string) ([]byte, error) {
	d.record(name, args)
	if name == "hyprctl" && len(args) == 2 && args[0] == "--batch" {
		return d.batch(args[1])
	}
	d.mu.Lock()
	file, ok := d.files[argvKey(name, args)]
	d.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("fake desk: no answer for %q", argvKey(name, args))
	}
	return os.ReadFile(filepath.Join("testdata", file))
}

// batch answers a Hyprland batch the way Hyprland does: each `j/VERB`
// answered as `hyprctl -j VERB` would be, joined by Hyprland's batch
// delimiter, as one read.
func (d *fakeDesk) batch(cmds string) ([]byte, error) {
	var parts [][]byte
	for _, c := range strings.Split(cmds, ";") {
		verb, ok := strings.CutPrefix(c, "j/")
		d.mu.Lock()
		file, known := d.files[argvKey("hyprctl", []string{"-j", verb})]
		d.mu.Unlock()
		if !ok || !known {
			return nil, fmt.Errorf("fake desk: no batch answer for %q", c)
		}
		b, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			return nil, err
		}
		parts = append(parts, b)
	}
	return bytes.Join(parts, []byte("\n\n\n")), nil
}

func (d *fakeDesk) stream(_ context.Context, name string, args ...string) (io.ReadCloser, error) {
	d.record(name, args)
	file, ok := d.streams[argvKey(name, args)]
	if !ok {
		return nil, fmt.Errorf("fake desk: no stream for %q", argvKey(name, args))
	}
	return os.Open(filepath.Join("testdata", file))
}

// collect runs Watch to the end of a recorded stream and returns every
// snapshot it delivered.
func collect(t *testing.T, p Provider) []Snapshot {
	t.Helper()
	var got []Snapshot
	err := p.Watch(context.Background(), func(s Snapshot) { got = append(got, s) })
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("Watch ended with %v, want the recorded stream's end", err)
	}
	return got
}

func byID(t *testing.T, s Snapshot, id ID) Window {
	t.Helper()
	w, ok := find(s, id)
	if !ok {
		t.Fatalf("window %s missing from %+v", id, s.Windows)
	}
	return w
}

func wantBox(t *testing.T, w Window, want Box) {
	t.Helper()
	if w.Box == nil {
		t.Fatalf("window %s has no box (%s), want %+v", w.ID, w.Unplaced, want)
	}
	if *w.Box != want {
		t.Fatalf("window %s box %+v, want %+v", w.ID, *w.Box, want)
	}
}

func wantUnplaced(t *testing.T, w Window, reason string) {
	t.Helper()
	if w.Box != nil {
		t.Fatalf("window %s has box %+v, want none", w.ID, *w.Box)
	}
	if w.Unplaced != reason {
		t.Fatalf("window %s unplaced %q, want %q", w.ID, w.Unplaced, reason)
	}
}

func kinds(cs []Change) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Kind)
	}
	return out
}

func within(t *testing.T, d time.Duration, c <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(d):
		t.Fatalf("timed out waiting for %s", what)
	}
}
