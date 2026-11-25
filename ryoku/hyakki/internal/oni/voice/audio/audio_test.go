package audio

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

func TestAllowlist(t *testing.T) {
	pw := append([]string{}, pwCatArgs...)
	pa := append([]string{}, parecArgs...)
	with := func(base []string, more ...string) []string { return append(slices.Clone(base), more...) }
	allowed := []struct {
		name string
		args []string
	}{
		{"pw-cat", with(pw, "-")},
		{"pw-cat", with(pw, "--target", "alsa_input.pci-0000_00_1f.3.analog-stereo", "-")},
		{"pw-cat", with(pw, "--target", "42", "-")},
		{"parec", pa},
		{"parec", with(pa, "--device=oni_lab_mic.monitor")},
		{"parec", with(pa, "--device=bluez_input.AA:BB:CC:DD:EE:FF.0")},
		{"pactl", []string{"-f", "json", "list", "sources"}},
		{"pactl", []string{"get-default-source"}},
	}
	for _, c := range allowed {
		if !Allowed(c.name, c.args...) {
			t.Errorf("refused %s %q", c.name, c.args)
		}
	}
	refused := []struct {
		name string
		args []string
	}{
		{"aplay", nil},
		{"aplay", []string{"-D", "hw:0,0", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "raw"}},
		{"arecord", []string{"-D", "hw:0,0"}},
		{"pw-record", []string{"-"}},
		{"pw-play", []string{"-"}},
		{"paplay", nil},
		{"pw-cat", nil},
		{"pw-cat", pw},                        // no stdout argument
		{"pw-cat", with(pw, "/tmp/take.wav")}, // a file
		{"pw-cat", with(pw, "take.raw")},      // a file
		{"pw-cat", with(pw, "--target", "/dev/snd/pcmC0D0c", "-")},
		{"pw-cat", with(pw, "--target", "--playback", "-")},
		{"pw-cat", with(pw, "--target", "", "-")},
		{"pw-cat", with(pw, "--target", "a b", "-")},
		{"pw-cat", with(pw, "-", "extra")},
		{"pw-cat", append([]string{"--playback"}, pw[1:]...)},
		{"pw-cat", with([]string{"--record", "--raw", "--format", "s16", "--rate", "48000", "--channels", "1"}, "-")},
		{"parec", with(pa, "take.raw")},                   // a file
		{"parec", with(pa, "--device=/dev/snd/pcmC0D0c")}, // a device path
		{"parec", with(pa, "--device=")},
		{"parec", with(pa, "--file-format=wav")},
		{"parec", pa[1:]},
		{"pactl", []string{"list", "sources"}},
		{"pactl", []string{"set-default-source", "x"}},
		{"pactl", []string{"load-module", "module-null-sink"}},
		{"pactl", []string{"-f", "json", "list", "sources", "extra"}},
		{"sh", []string{"-c", "pw-cat --record -"}},
	}
	for _, c := range refused {
		if Allowed(c.name, c.args...) {
			t.Errorf("allowed %s %q", c.name, c.args)
		}
	}
}

// spyStream records every argv it is asked to start and serves no audio.
type spyStream struct {
	mu    sync.Mutex
	calls [][]string
}

func (s *spyStream) stream(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
	s.mu.Lock()
	s.calls = append(s.calls, append([]string{name}, args...))
	s.mu.Unlock()
	return io.NopCloser(strings.NewReader("")), nil
}

func haveOnly(names ...string) func(string) bool {
	return func(n string) bool { return slices.Contains(names, n) }
}

func TestRecorderArgvIsAllowedAndChosenByAvailability(t *testing.T) {
	cases := []struct {
		have []string
		node string
		want []string
	}{
		{[]string{"pw-cat", "parec"}, "", append(append([]string{"pw-cat"}, pwCatArgs...), "-")},
		{[]string{"pw-cat"}, "mic.1", append(append([]string{"pw-cat"}, pwCatArgs...), "--target", "mic.1", "-")},
		{[]string{"parec"}, "", append([]string{"parec"}, parecArgs...)},
		{[]string{"parec", "aplay"}, " mic.1 ", append(append([]string{"parec"}, parecArgs...), "--device=mic.1")},
	}
	for _, c := range cases {
		spy := &spyStream{}
		pcm, stop, err := NewRecorderWith(spy.stream, haveOnly(c.have...)).Start(context.Background(), c.node)
		if err != nil {
			t.Fatalf("%v: %v", c.have, err)
		}
		for range pcm {
		}
		if err := stop(); err != nil {
			t.Fatal(err)
		}
		if len(spy.calls) != 1 || !slices.Equal(spy.calls[0], c.want) {
			t.Fatalf("%v %q: started %q, want %q", c.have, c.node, spy.calls, c.want)
		}
		if !Allowed(spy.calls[0][0], spy.calls[0][1:]...) {
			t.Fatalf("started a refused argv %q", spy.calls[0])
		}
	}

	spy := &spyStream{}
	if _, _, err := NewRecorderWith(spy.stream, haveOnly("aplay", "arecord")).Start(context.Background(), ""); !errors.Is(err, ErrNoRecorder) {
		t.Fatalf("no recorder: %v", err)
	}
	for _, node := range []string{"/dev/snd/pcmC0D0c", "-x", "a b", "../x"} {
		if _, _, err := NewRecorderWith(spy.stream, haveOnly("pw-cat", "parec")).Start(context.Background(), node); !errors.Is(err, ErrBadNode) {
			t.Fatalf("node %q: %v", node, err)
		}
	}
	if len(spy.calls) != 0 {
		t.Fatalf("refused starts reached the stream: %q", spy.calls)
	}
}

// pcmBytes is pcm as a recorder writes it.
func pcmBytes(pcm []int16) []byte {
	b := make([]byte, 2*len(pcm))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(s))
	}
	return b
}

// dribble hands on its bytes in reads of the given sizes, cycling.
type dribble struct {
	b     []byte
	sizes []int
	i     int
}

func (d *dribble) Read(p []byte) (int, error) {
	if len(d.b) == 0 {
		return 0, io.EOF
	}
	n := min(d.sizes[d.i%len(d.sizes)], len(p), len(d.b))
	d.i++
	copy(p, d.b[:n])
	d.b = d.b[n:]
	return n, nil
}

func TestCaptureDecodesAcrossOddReads(t *testing.T) {
	want := make([]int16, 5000)
	for i := range want {
		want[i] = int16(i*37 - 20000)
	}
	want[0], want[1] = math.MinInt16, math.MaxInt16
	stream := func(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
		return io.NopCloser(&dribble{b: pcmBytes(want), sizes: []int{1, 3, 7, 4000, 2}}), nil
	}
	pcm, stop, err := NewRecorderWith(stream, haveOnly("pw-cat")).Start(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	var got []int16
	for c := range pcm {
		if len(c) > chunkBytes/2 {
			t.Fatalf("chunk of %d samples", len(c))
		}
		got = append(got, c...)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("decoded %d samples, mismatch", len(got))
	}
}

// pipeProc is a recorder that writes nothing until it is closed.
type pipeProc struct {
	r      *io.PipeReader
	w      *io.PipeWriter
	closes atomic.Int32
}

func (p *pipeProc) Read(b []byte) (int, error) { return p.r.Read(b) }
func (p *pipeProc) Close() error {
	p.closes.Add(1)
	_ = p.w.Close()
	return p.r.Close()
}

func TestCaptureStopsOnCancelAndStop(t *testing.T) {
	for _, viaCancel := range []bool{true, false} {
		r, w := io.Pipe()
		proc := &pipeProc{r: r, w: w}
		stream := func(ctx context.Context, name string, args ...string) (io.ReadCloser, error) { return proc, nil }
		ctx, cancel := context.WithCancel(context.Background())
		pcm, stop, err := NewRecorderWith(stream, haveOnly("parec")).Start(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		go func() { _, _ = w.Write(pcmBytes([]int16{1, 2, 3})) }()
		if got := <-pcm; !slices.Equal(got, []int16{1, 2, 3}) {
			t.Fatalf("first chunk %v", got)
		}
		if viaCancel {
			cancel()
			for range pcm {
			}
			if proc.closes.Load() != 1 {
				t.Fatalf("cancel closed the recorder %d times", proc.closes.Load())
			}
		}
		for range 2 {
			if err := stop(); err != nil {
				t.Fatal(err)
			}
		}
		if _, open := <-pcm; open {
			t.Fatal("channel still open after stop")
		}
		if n := proc.closes.Load(); n != 1 {
			t.Fatalf("recorder closed %d times", n)
		}
		cancel()
	}
}

func TestCaptureRefusesDoneContext(t *testing.T) {
	spy := &spyStream{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := NewRecorderWith(spy.stream, haveOnly("pw-cat")).Start(ctx, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("start after cancel: %v", err)
	}
	if len(spy.calls) != 0 {
		t.Fatal("started a recorder for a done context")
	}
}

// entries lists a directory's names.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

// stagedRecorder puts a stand-in pw-cat on PATH that writes body's output
// and then waits, and points TMPDIR and XDG_RUNTIME_DIR at empty dirs. The
// stand-in is ours: no sound server is reached. It returns a Recorder that
// reports each child's pid, and the two dirs.
func stagedRecorder(t *testing.T, body string) (*Recorder, func() int, []string) {
	t.Helper()
	bin := t.TempDir()
	tmp, run := t.TempDir(), t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(bin, "pw-cat"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMPDIR", tmp)
	t.Setenv("XDG_RUNTIME_DIR", run)
	var mu sync.Mutex
	pid := 0
	exec := ExecStream(nil)
	stream := func(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
		rc, err := exec(ctx, name, args...)
		if err == nil {
			mu.Lock()
			pid = rc.(*execReader).cmd.Process.Pid
			mu.Unlock()
		}
		return rc, err
	}
	return NewRecorderWith(stream, onPath), func() int { mu.Lock(); defer mu.Unlock(); return pid }, []string{tmp, run}
}

func reaped(pid int) bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }

func TestCaptureKillsAndReapsTheChild(t *testing.T) {
	// Two samples, 1 and 2, then wait far longer than the test.
	const body = `printf '\001\000\002\000'; exec sleep 600`
	for _, viaCancel := range []bool{false, true} {
		rec, pid, dirs := stagedRecorder(t, body)
		ctx, cancel := context.WithCancel(context.Background())
		pcm, stop, err := rec.Start(ctx, "lab.mic")
		if err != nil {
			t.Fatal(err)
		}
		if got := <-pcm; !slices.Equal(got, []int16{1, 2}) {
			t.Fatalf("first chunk %v", got)
		}
		if reaped(pid()) {
			t.Fatal("child gone before stop")
		}
		if viaCancel {
			cancel()
			for range pcm {
			}
			if !reaped(pid()) {
				t.Fatalf("child %d not reaped after cancel", pid())
			}
		}
		if err := stop(); err != nil {
			t.Fatalf("stop: %v", err)
		}
		if !reaped(pid()) {
			t.Fatalf("child %d not reaped after stop", pid())
		}
		cancel()
		for _, d := range dirs {
			if names := entries(t, d); len(names) != 0 {
				t.Fatalf("capture left %q in %s", names, d)
			}
		}
	}
}

func TestCaptureReportsARecorderThatFails(t *testing.T) {
	rec, pid, _ := stagedRecorder(t, `echo 'no such target' >&2; exit 3`)
	pcm, stop, err := rec.Start(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for range pcm {
	}
	if err := stop(); err == nil {
		t.Fatal("a recorder that exited 3 stopped cleanly")
	}
	if !reaped(pid()) {
		t.Fatal("failed child not reaped")
	}
}

const sourcesJSON = `[
 {"index":1,"name":"alsa_output.pci-0000_00_1f.3.analog-stereo.monitor","description":"Monitor of Built-in Audio","monitor_of_sink":"alsa_output.pci-0000_00_1f.3.analog-stereo","properties":{"device.class":"monitor"}},
 {"index":2,"name":"alsa_input.pci-0000_00_1f.3.analog-stereo","description":"Built-in Audio Analog Stereo","monitor_of_sink":"n/a","properties":{"device.class":"sound","device.api":"alsa"}},
 {"index":3,"name":"oni_lab_mic.monitor","description":"","monitor_source":"oni_lab_mic","properties":{}},
 {"index":4,"name":"bluez_input.AA:BB:CC:DD:EE:FF.0","description":"Headset","monitor_of_sink":null,"properties":{"device.bus":"bluetooth","node.latency":3}},
 {"index":5,"name":"null.sink.monitor","description":"Null","properties":{"device.class":"monitor"}},
 {"index":6,"name":"/dev/snd/pcmC0D0c","description":"Raw"},
 {"index":7,"name":"","description":"Nameless"}
]`

// fakeRun serves canned pactl output and records each call.
type fakeRun struct {
	out   map[string]string
	calls [][]string
}

func (f *fakeRun) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	out, ok := f.out[strings.Join(append([]string{name}, args...), " ")]
	if !ok {
		return nil, errors.New("unexpected command")
	}
	return []byte(out), nil
}

func TestSources(t *testing.T) {
	f := &fakeRun{out: map[string]string{
		"pactl -f json list sources": sourcesJSON,
		"pactl get-default-source":   "alsa_input.pci-0000_00_1f.3.analog-stereo\n",
	}}
	ctx := context.Background()
	mics, err := Sources(ctx, f.run)
	if err != nil {
		t.Fatal(err)
	}
	wantMics := []Source{
		{Name: "alsa_input.pci-0000_00_1f.3.analog-stereo", Desc: "Built-in Audio Analog Stereo"},
		{Name: "bluez_input.AA:BB:CC:DD:EE:FF.0", Desc: "Headset"},
	}
	if !slices.Equal(mics, wantMics) {
		t.Fatalf("mics %+v", mics)
	}
	all, err := AllSources(ctx, f.run)
	if err != nil {
		t.Fatal(err)
	}
	wantAll := []Source{
		{Name: "alsa_output.pci-0000_00_1f.3.analog-stereo.monitor", Desc: "Monitor of Built-in Audio", Monitor: true},
		wantMics[0],
		{Name: "oni_lab_mic.monitor", Desc: "oni_lab_mic.monitor", Monitor: true},
		wantMics[1],
		{Name: "null.sink.monitor", Desc: "Null", Monitor: true},
	}
	if !slices.Equal(all, wantAll) {
		t.Fatalf("all %+v", all)
	}
	def, err := DefaultSource(ctx, f.run)
	if err != nil || def != "alsa_input.pci-0000_00_1f.3.analog-stereo" {
		t.Fatalf("default %q %v", def, err)
	}
	for _, c := range f.calls {
		if !Allowed(c[0], c[1:]...) {
			t.Fatalf("ran a refused argv %q", c)
		}
	}

	bad := &fakeRun{out: map[string]string{"pactl -f json list sources": "Connection failure"}}
	if _, err := Sources(ctx, bad.run); err == nil {
		t.Fatal("garbage parsed as a source list")
	}
	odd := &fakeRun{out: map[string]string{"pactl get-default-source": "/dev/snd/pcmC0D0c\n"}}
	if def, err := DefaultSource(ctx, odd.run); err != nil || def != "" {
		t.Fatalf("default %q %v", def, err)
	}
	if _, err := Sources(ctx, nil); err == nil {
		t.Fatal("no runner listed sources")
	}
}

func loadSpec(t *testing.T) *spec.OniSpec {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

// square is n samples of a square wave at ±amp: its RMS is amp.
func square(n int, amp int16) []int16 {
	pcm := make([]int16, n)
	for i := range pcm {
		if i%2 == 0 {
			pcm[i] = amp
		} else {
			pcm[i] = -amp
		}
	}
	return pcm
}

func TestMeterLevels(t *testing.T) {
	sp := loadSpec(t)
	m := NewMeter(sp)
	// 15 Hz at most over 10 ms frames: 7 frames of 160 samples.
	if m.Window != 1120 {
		t.Fatalf("window %d samples", m.Window)
	}
	if hz := float64(SampleRateHz) / float64(m.Window); hz > sp.Voice.Levels.MaxHz {
		t.Fatalf("%v levels a second", hz)
	}
	w := m.Window
	var pcm []int16
	pcm = append(pcm, make([]int16, w)...)  // silence
	pcm = append(pcm, square(w, 32767)...)  // full scale
	pcm = append(pcm, square(w, 3277)...)   // -20 dBFS
	pcm = append(pcm, square(w, 16384)...)  // -6.02 dBFS
	pcm = append(pcm, square(w, 16)...)     // -66 dBFS, under the floor
	pcm = append(pcm, square(w, 33)...)     // -59.9 dBFS, just over it
	pcm = append(pcm, square(w/2, 1000)...) // half a window: not yet a level
	want := []float64{0, 1, 2.0 / 3, 1 - 20*math.Log10(2)/60, 0, (60 + 20*math.Log10(33.0/32768)) / 60}

	check := func(got []float64) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%d levels, want %d: %v", len(got), len(want), got)
		}
		for i := range want {
			if math.Abs(got[i]-want[i]) > 1e-3 {
				t.Fatalf("level %d = %v, want %v", i, got[i], want[i])
			}
		}
	}
	check(NewMeter(sp).Push(pcm))

	// The same capture in uneven chunks gives the same levels.
	m = NewMeter(sp)
	var got []float64
	for rest, i := pcm, 0; len(rest) > 0; i++ {
		n := min([]int{1, 333, 1119, 2500, 7}[i%5], len(rest))
		got = append(got, m.Push(rest[:n])...)
		rest = rest[n:]
	}
	check(got)
	// The half window completes with the next push.
	if l := m.Push(square(w/2, 1000)); len(l) != 1 || math.Abs(l[0]-(60+20*math.Log10(1000.0/32768))/60) > 1e-3 {
		t.Fatalf("carried window %v", l)
	}
	if Level(math.NaN()) != 0 || Level(-1) != 0 || Level(1e9) != 1 {
		t.Fatal("level out of [0, 1]")
	}
}

func TestMicTestMeasuresAndKeepsNothing(t *testing.T) {
	sp := loadSpec(t)
	tmp, run := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("XDG_RUNTIME_DIR", run)

	w := NewMeter(sp).Window
	pcm := append(square(w, 3277), square(w, 32767)...)
	pcm = append(pcm, square(w/3, 1)...)
	spy := &spyStream{}
	stream := func(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
		_, _ = spy.stream(ctx, name, args...)
		return io.NopCloser(&dribble{b: pcmBytes(pcm), sizes: []int{640}}), nil
	}
	levels, err := Test(context.Background(), NewRecorderWith(stream, haveOnly("pw-cat")), "lab.mic", time.Minute, NewMeter(sp))
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 2 || math.Abs(levels[0]-2.0/3) > 1e-3 || math.Abs(levels[1]-1) > 1e-3 {
		t.Fatalf("levels %v", levels)
	}
	if !Allowed(spy.calls[0][0], spy.calls[0][1:]...) {
		t.Fatalf("meter ran %q", spy.calls[0])
	}

	// A recorder that ends before one window gives no levels.
	empty := func(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("\x01\x00")), nil
	}
	if _, err := Test(context.Background(), NewRecorderWith(empty, haveOnly("parec")), "", time.Minute, NewMeter(sp)); !errors.Is(err, ErrNoAudio) {
		t.Fatalf("short capture: %v", err)
	}

	// The duration ends a recorder that never stops on its own.
	r, wr := io.Pipe()
	proc := &pipeProc{r: r, w: wr}
	go func() {
		for {
			if _, err := wr.Write(pcmBytes(square(w, 3277))); err != nil {
				return
			}
		}
	}()
	held := func(ctx context.Context, name string, args ...string) (io.ReadCloser, error) { return proc, nil }
	levels, err = Test(context.Background(), NewRecorderWith(held, haveOnly("pw-cat")), "", 50*time.Millisecond, NewMeter(sp))
	if err != nil || len(levels) == 0 {
		t.Fatalf("held: %d levels, %v", len(levels), err)
	}
	if proc.closes.Load() != 1 {
		t.Fatalf("held recorder closed %d times", proc.closes.Load())
	}

	for _, d := range []string{tmp, run} {
		if names := entries(t, d); len(names) != 0 {
			t.Fatalf("meter left %q in %s", names, d)
		}
	}
}
