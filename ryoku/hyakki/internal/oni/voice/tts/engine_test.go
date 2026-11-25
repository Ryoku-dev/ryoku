package tts

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// wav16 encodes mono 16-bit PCM.
func wav16(rate int, s []int16) []byte {
	b := []byte("RIFF")
	b = binary.LittleEndian.AppendUint32(b, uint32(36+2*len(s)))
	b = append(b, "WAVEfmt "...)
	b = binary.LittleEndian.AppendUint32(b, 16)
	b = binary.LittleEndian.AppendUint16(b, 1)
	b = binary.LittleEndian.AppendUint16(b, 1)
	b = binary.LittleEndian.AppendUint32(b, uint32(rate))
	b = binary.LittleEndian.AppendUint32(b, uint32(rate*2))
	b = binary.LittleEndian.AppendUint16(b, 2)
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(2*len(s)))
	for _, v := range s {
		b = binary.LittleEndian.AppendUint16(b, uint16(v))
	}
	return b
}

func toneWAV(rate int, secs float64) []byte {
	s := make([]int16, int(float64(rate)*secs))
	for i := range s {
		s[i] = int16(8000 * math.Sin(2*math.Pi*220*float64(i)/float64(rate)))
	}
	return wav16(rate, s)
}

func rms(pcm []float32) float64 {
	var sum float64
	for _, v := range pcm {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum / float64(max(len(pcm), 1)))
}

// call is one command a fake runner saw.
type call struct {
	argv  []string
	stdin string
}

func fakeRunner(out []byte, calls *[]call) Runner {
	return func(_ context.Context, argv []string, stdin []byte) ([]byte, error) {
		*calls = append(*calls, call{slices.Clone(argv), string(stdin)})
		return out, nil
	}
}

// executable puts an empty program named name in a new folder.
func executable(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOrder(t *testing.T) {
	cases := []struct {
		setting, profile string
		want             []string
	}{
		{"", "system", []string{System}},
		{"", "clips", []string{Clips, System}},
		{"", "kokoro", []string{Kokoro, System}},
		{"kokoro", "clips", []string{Kokoro, Clips, System}},
		{"clips", "clips", []string{Clips, System}},
		{"system", "kokoro", []string{System, Kokoro}},
		{"loud", "", []string{System}},
	}
	for _, c := range cases {
		if got := Order(c.setting, c.profile); !slices.Equal(got, c.want) {
			t.Fatalf("Order(%q, %q) = %q, want %q", c.setting, c.profile, got, c.want)
		}
	}
	sys, kok := &fakeEngine{name: System}, &fakeEngine{name: Kokoro}
	got := Pick(Order("", Clips), map[string]Engine{System: sys, Kokoro: kok})
	if len(got) != 1 || got[0] != sys {
		t.Fatalf("Pick skipped nothing or picked kokoro: %v", got)
	}
}

func TestSystemRunsEspeakOnStdoutOnly(t *testing.T) {
	var calls []call
	bin := executable(t, "espeak-ng")
	for _, c := range []struct {
		speed float64
		wpm   string
	}{{0, "175"}, {1, "175"}, {2, "350"}, {0.5, "88"}} {
		e, err := NewSystem(SystemOptions{Path: bin, Voice: "en-gb", Speed: c.speed, Run: fakeRunner(toneWAV(22050, 0.5), &calls)})
		if err != nil {
			t.Fatal(err)
		}
		pcm, rate, err := e.Render(context.Background(), "  -rf Hello there. ")
		if err != nil || rate != SpeechRateHz() || len(pcm) != SpeechRateHz()/2 {
			t.Fatalf("speed %v: %d samples at %d, %v", c.speed, len(pcm), rate, err)
		}
		last := calls[len(calls)-1]
		want := []string{bin, "--stdout", "--stdin", "-b", "1", "-s", c.wpm, "-v", "en-gb"}
		if !slices.Equal(last.argv, want) || last.stdin != "-rf Hello there." {
			t.Fatalf("speed %v: argv %q stdin %q", c.speed, last.argv, last.stdin)
		}
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := NewSystem(SystemOptions{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no espeak-ng: %v", err)
	}
}

// The system voice is the real espeak-ng writing to stdout: no device, no
// file. Its speech lasts between 35 and 140 ms per character of text at
// speed 1 (espeak-ng's 175 words a minute is about 70 ms per character),
// grows with the text and shrinks with speed.
func TestSystemSpeechIsAudibleAndPaced(t *testing.T) {
	if _, err := exec.LookPath("espeak-ng"); err != nil {
		t.Skip("espeak-ng is not installed")
	}
	const minPerChar, maxPerChar = 0.035, 0.140
	render := func(speed float64, line string) []float32 {
		t.Helper()
		e, err := NewSystem(SystemOptions{Speed: speed})
		if err != nil {
			t.Fatal(err)
		}
		pcm, rate, err := e.Render(context.Background(), line)
		if err != nil || rate != SpeechRateHz() {
			t.Fatalf("render %q: rate %d, %v", line, rate, err)
		}
		return pcm
	}
	secs := func(pcm []float32) float64 { return float64(len(pcm)) / float64(SpeechRateHz()) }
	short := "Hello there."
	long := "The quick brown fox jumps over the lazy dog, then runs far away into the quiet forest."
	var durations []float64
	for _, line := range []string{short, long} {
		pcm := render(1, line)
		if r := rms(pcm); r < 0.02 {
			t.Fatalf("%q is near silent: rms %.4f", line, r)
		}
		per := secs(pcm) / float64(utf8.RuneCountInString(line))
		t.Logf("%q: %.2f s, %.1f ms per character, rms %.3f", line, secs(pcm), per*1000, rms(pcm))
		if per < minPerChar || per > maxPerChar {
			t.Fatalf("%q lasts %.3f s: %.1f ms per character, want %v-%v", line, secs(pcm), per*1000, minPerChar*1000, maxPerChar*1000)
		}
		durations = append(durations, secs(pcm))
	}
	if durations[1] < 3*durations[0] {
		t.Fatalf("a 7x longer line lasts %.2f s against %.2f s", durations[1], durations[0])
	}
	if fast := secs(render(2, long)); fast > 0.75*durations[1] {
		t.Fatalf("speed 2 lasts %.2f s against %.2f s at speed 1", fast, durations[1])
	}
}

func kokoroPack(t *testing.T, lexicon bool) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{kokoroModel, kokoroVoices, kokoroTokens} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, kokoroData), 0o755); err != nil {
		t.Fatal(err)
	}
	if lexicon {
		if err := os.WriteFile(filepath.Join(dir, "lexicon-us-en.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// No sherpa-onnx is installed on the build machine, so Kokoro is proven
// against a stand-in runner only: arguments, pack checks and reading the
// WAV past the program's log.
func TestKokoroArgumentsAndPack(t *testing.T) {
	bin := executable(t, KokoroBinary)
	dir := kokoroPack(t, true)
	var calls []call
	out := append([]byte("loading model\n"), toneWAV(24000, 0.25)...)
	e, err := FindKokoro(KokoroOptions{Binary: bin, Dir: dir, Speaker: 25, Speed: 1.25, Run: fakeRunner(out, &calls)})
	if err != nil {
		t.Fatal(err)
	}
	pcm, rate, err := e.Render(context.Background(), "--help me")
	if err != nil || rate != SpeechRateHz() || len(pcm) != SpeechRateHz()/4 || rms(pcm) < 0.1 {
		t.Fatalf("render: %d samples at %d, %v", len(pcm), rate, err)
	}
	want := []string{
		bin,
		"--kokoro-model=" + filepath.Join(dir, "model.onnx"),
		"--kokoro-voices=" + filepath.Join(dir, "voices.bin"),
		"--kokoro-tokens=" + filepath.Join(dir, "tokens.txt"),
		"--kokoro-data-dir=" + filepath.Join(dir, "espeak-ng-data"),
		"--kokoro-length-scale=0.800",
		"--num-threads=4",
		"--sid=25",
		"--output-filename=/dev/stdout",
		"--kokoro-lexicon=" + filepath.Join(dir, "lexicon-us-en.txt"),
		"help me",
	}
	if !slices.Equal(calls[0].argv, want) || calls[0].stdin != "" {
		t.Fatalf("argv\n got %q\nwant %q", calls[0].argv, want)
	}
	if _, _, err := (&KokoroEngine{bin: bin, dir: dir, speed: 1, run: fakeRunner([]byte("no audio"), &calls)}).Render(context.Background(), "hi"); err == nil {
		t.Fatal("output without a WAV rendered")
	}
	for name, f := range map[string]string{"model": kokoroModel, "voices": kokoroVoices, "tokens": kokoroTokens, "data": kokoroData} {
		dir := kokoroPack(t, false)
		if err := os.RemoveAll(filepath.Join(dir, f)); err != nil {
			t.Fatal(err)
		}
		if _, err := FindKokoro(KokoroOptions{Binary: bin, Dir: dir}); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("pack without %s: %v", name, err)
		}
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := FindKokoro(KokoroOptions{Dir: dir}); !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), KokoroBinary) {
		t.Fatalf("no sherpa-onnx: %v", err)
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if _, err := FindKokoro(KokoroOptions{Binary: bin}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no pack installed: %v", err)
	}
}
