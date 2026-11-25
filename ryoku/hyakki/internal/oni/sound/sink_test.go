package sound

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakePlayers puts stand-in player scripts named names on an otherwise
// empty PATH. Each script records its name and arguments, then copies stdin
// to a file; nothing reaches an audio server.
func fakePlayers(t *testing.T, names ...string) (rec string) {
	t.Helper()
	tool := func(name string) string {
		p, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("no %s for the stand-in players: %v", name, err)
		}
		return p
	}
	cat, mv := tool("cat"), tool("mv")
	bin, rec := t.TempDir(), t.TempDir()
	for _, name := range names {
		script := "#!/bin/sh\n" +
			"printf '%s\\n' \"$(basename \"$0\")\" \"$@\" > " + rec + "/args.tmp && " + mv + " " + rec + "/args.tmp " + rec + "/args\n" +
			cat + " > " + rec + "/pcm.tmp && " + mv + " " + rec + "/pcm.tmp " + rec + "/pcm\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Contains(names, "basename") {
		if err := os.Symlink(tool("basename"), filepath.Join(bin, "basename")); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	return rec
}

func waitFile(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			return data
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never appeared", path)
	return nil
}

func tone(n int) []float32 {
	pcm := make([]float32, n)
	for i := range pcm {
		pcm[i] = float32(i%97) / 200
	}
	return pcm
}

func TestPipeSinkNeverUsesALSA(t *testing.T) {
	for _, names := range [][]string{{"aplay"}, nil} {
		t.Run(strings.Join(append([]string{"path"}, names...), "-"), func(t *testing.T) {
			fakePlayers(t, names...)
			if _, err := PipeSink(); err == nil {
				t.Fatalf("PipeSink accepted a PATH holding only %q", names)
			}
		})
	}
}

func TestPipeSinkPrefersPwCatAndSpawnsLazily(t *testing.T) {
	rec := fakePlayers(t, "pw-cat", "paplay", "aplay")
	sink, err := PipeSink()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(rec, "args")); !os.IsNotExist(err) {
		t.Fatal("PipeSink started a player before any sound")
	}
	pcm := tone(RateHz / 20)
	if _, err := sink.Play(pcm, RateHz); err != nil {
		t.Fatal(err)
	}
	got := waitFile(t, filepath.Join(rec, "pcm"))
	if !bytes.Equal(got, encode(nil, pcm)) {
		t.Fatalf("player was fed %d bytes that differ from the %d-sample sound", len(got), len(pcm))
	}
	args := strings.Fields(string(waitFile(t, filepath.Join(rec, "args"))))
	want := []string{"pw-cat", "--playback", "--raw", "--format=f32", "--channels=1", "--rate=" + strconv.Itoa(RateHz)}
	if len(args) < len(want) || !slices.Equal(args[:len(want)], want) || args[len(args)-1] != "-" {
		t.Fatalf("pw-cat args %q", args)
	}
}

func TestPipeSinkFallsBackToPaplay(t *testing.T) {
	rec := fakePlayers(t, "paplay", "aplay")
	sink, err := PipeSink()
	if err != nil {
		t.Fatal(err)
	}
	pcm := tone(RateHz / 50)
	if _, err := sink.Play(pcm, RateHz); err != nil {
		t.Fatal(err)
	}
	if got := waitFile(t, filepath.Join(rec, "pcm")); !bytes.Equal(got, encode(nil, pcm)) {
		t.Fatal("paplay was fed different bytes")
	}
	args := strings.Fields(string(waitFile(t, filepath.Join(rec, "args"))))
	if args[0] != "paplay" || !slices.Contains(args, "--raw") || !slices.Contains(args, "--format=float32ne") ||
		!slices.Contains(args, "--channels=1") || !slices.Contains(args, "--rate="+strconv.Itoa(RateHz)) {
		t.Fatalf("paplay args %q", args)
	}
}

func readRecords(t *testing.T, path string, n int) []FileRecord {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var recs []FileRecord
		if f, err := os.Open(path); err == nil {
			sc := bufio.NewScanner(f)
			sc.Buffer(nil, 1<<24)
			for sc.Scan() {
				var r FileRecord
				if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
					t.Fatal(err)
				}
				recs = append(recs, r)
			}
			f.Close()
		}
		if len(recs) >= n {
			return recs
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never held %d records", path, n)
	return nil
}

func TestFileSinkRecordsWhatWouldPlay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sound.jsonl")
	sink := FileSink(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("FileSink touched its file before any sound")
	}
	pcm := tone(RateHz / 20)
	if _, err := sink.Play(pcm, RateHz); err != nil {
		t.Fatal(err)
	}
	r := readRecords(t, path, 1)[0]
	if r.RateHz != RateHz || r.Samples != len(pcm) || r.Written != len(pcm) || r.CutAt != -1 || r.Error != "" {
		t.Fatalf("record %+v", r)
	}
	if !bytes.Equal(r.PCM, encode(nil, pcm)) {
		t.Fatal("recorded PCM differs from the sound")
	}
}

func TestFileSinkRecordsTheMuteRamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sound.jsonl")
	sink := FileSink(path)
	pcm := make([]float32, RateHz)
	for i := range pcm {
		pcm[i] = 0.25
	}
	cancel, err := sink.Play(pcm, RateHz)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	cancel()
	cancel()
	r := readRecords(t, path, 1)[0]
	ramp := samplesOf(muteRamp(), RateHz)
	if r.CutAt < 0 || r.Written != r.CutAt+ramp || r.Written >= len(pcm) {
		t.Fatalf("cancelled record: cut %d written %d, want cut + %d", r.CutAt, r.Written, ramp)
	}
	got := make([]float32, r.Written)
	for i := range got {
		got[i] = decode(r.PCM[4*i:])
	}
	if got[len(got)-1] <= 0 || got[len(got)-1] > 0.25/float32(ramp/2) {
		t.Fatalf("recording does not end on the ramp's last step: %v", got[len(got)-1])
	}
	for i := 1; i < len(got); i++ {
		if got[i] > got[i-1] {
			t.Fatalf("level rose at %d during the ramp", i)
		}
	}
}

func decode(b []byte) float32 { return math.Float32frombits(binary.NativeEndian.Uint32(b)) }
