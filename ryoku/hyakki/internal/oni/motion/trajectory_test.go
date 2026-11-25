package motion

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

var update = flag.Bool("update", false, "rewrite motion/testdata/trajectories")

type goldenFrame struct {
	I      int  `json:"i"`
	FPS    int  `json:"fps"`
	Redraw bool `json:"redraw"`
	Stop   bool `json:"stop"`
	Pose   Pose `json:"pose"`
}

type golden struct {
	Scenario string        `json:"scenario"`
	FrameS   float64       `json:"frameS"`
	Every    int           `json:"every"`
	Frames   []goldenFrame `json:"frames"`
}

const every = 4

type recorder struct {
	e      *Engine
	i      int
	frames []goldenFrame
}

func (r *recorder) run(n int, in Input) {
	for range n {
		tk := r.e.Step(frame, in)
		if r.i%every == 0 || tk.Stop {
			r.frames = append(r.frames, goldenFrame{I: r.i, FPS: tk.FPS, Redraw: tk.Redraw, Stop: tk.Stop, Pose: tk.Pose})
		}
		r.i++
	}
}

func scenarios(s *spec.OniSpec) map[string]func(r *recorder) {
	out := map[string]func(r *recorder){
		"handoff-poke-done": func(r *recorder) {
			r.e.Play("poke")
			r.run(6, Input{})
			r.e.Play("done")
			r.run(60, Input{})
		},
		"drag": func(r *recorder) {
			r.run(30, Input{Drag: &Drag{Velocity: spec.Point{X: 900, Y: -300}}})
			r.run(50, Input{})
		},
		"listening-pointer-mic": func(r *recorder) {
			r.e.SetState("listening", "")
			r.run(80, Input{Pointer: &spec.Point{X: 40, Y: -30}, Mic: 0.6})
		},
		"speaking-speech": func(r *recorder) {
			r.e.SetState("speaking", "")
			r.run(80, Input{Speech: 0.8})
		},
		"sleep-commit": func(r *recorder) {
			r.e.Play("sleep")
			r.e.SetState("sleeping", "")
			r.run(130, Input{})
		},
		"thinking-spin-stop": func(r *recorder) {
			r.e.SetState("thinking", "")
			r.run(50, Input{})
			r.e.SetState("ready", "")
			r.run(90, Input{})
		},
	}
	for _, id := range spec.StateIDs {
		if id == "idle" || id == "sleeping" {
			continue
		}
		out["state-"+id] = func(r *recorder) {
			r.e.SetState(id, "")
			r.run(72, Input{})
		}
	}
	for _, id := range spec.GestureIDs {
		g, _ := s.Gesture(id)
		n := int(math.Ceil((g.DurationS + 0.25) * 60))
		out["gesture-"+id] = func(r *recorder) {
			r.e.Play(id)
			r.run(n, Input{})
		}
	}
	return out
}

func record(s *spec.OniSpec, name string, play func(r *recorder)) golden {
	r := &recorder{e: New(s)}
	r.run(1, Input{})
	play(r)
	return golden{Scenario: name, FrameS: frame.Seconds(), Every: every, Frames: r.frames}
}

func poseDiff(want, got Pose, tol float64) string {
	if want.State != got.State || want.Gesture != got.Gesture || want.Lap.From != got.Lap.From || want.Lap.To != got.Lap.To {
		return fmt.Sprintf("state/gesture/lap ids %q %q %q→%q, got %q %q %q→%q",
			want.State, want.Gesture, want.Lap.From, want.Lap.To, got.State, got.Gesture, got.Lap.From, got.Lap.To)
	}
	w := []float64{want.Flat, want.Face, want.Loose, want.Spin, want.Scale, want.Squash, want.LeanX, want.LeanY, want.Unravel, want.Straighten, want.Tempo, want.Bright, want.HoverY, want.Lap.Progress, want.Lap.Glow}
	g := []float64{got.Flat, got.Face, got.Loose, got.Spin, got.Scale, got.Squash, got.LeanX, got.LeanY, got.Unravel, got.Straighten, got.Tempo, got.Bright, got.HoverY, got.Lap.Progress, got.Lap.Glow}
	names := []string{"flat", "face", "loose", "spin", "scale", "squash", "leanX", "leanY", "unravel", "straighten", "tempo", "bright", "hoverY", "lap.progress", "lap.glow"}
	for i := range w {
		if !(math.Abs(w[i]-g[i]) <= tol) {
			return fmt.Sprintf("%s: want %v, got %v", names[i], w[i], g[i])
		}
	}
	return ""
}

// The goldens pin spring convergence, the state and gesture tables and the
// tiers so the later QML port can check itself against the same frames.
func TestTrajectoryGoldens(t *testing.T) {
	s := mustSpec(t)
	dir := filepath.Join("testdata", "trajectories")
	all := scenarios(s)
	if *update {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, play := range all {
		t.Run(name, func(t *testing.T) {
			got := record(s, name, play)
			path := filepath.Join(dir, name+".json")
			if *update {
				data, err := json.MarshalIndent(got, "", " ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden (run with -update): %v", err)
			}
			var want golden
			if err := json.Unmarshal(data, &want); err != nil {
				t.Fatal(err)
			}
			if len(want.Frames) != len(got.Frames) {
				t.Fatalf("%d frames, golden has %d", len(got.Frames), len(want.Frames))
			}
			for i, wf := range want.Frames {
				gf := got.Frames[i]
				if wf.I != gf.I || wf.FPS != gf.FPS || wf.Redraw != gf.Redraw || wf.Stop != gf.Stop {
					t.Fatalf("frame %d: golden i=%d fps=%d redraw=%v stop=%v, got i=%d fps=%d redraw=%v stop=%v",
						i, wf.I, wf.FPS, wf.Redraw, wf.Stop, gf.I, gf.FPS, gf.Redraw, gf.Stop)
				}
				if d := poseDiff(wf.Pose, gf.Pose, 1e-9); d != "" {
					t.Fatalf("frame %d (step %d): %s", i, wf.I, d)
				}
			}
		})
	}
	if !*update {
		files, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if _, ok := all[strings.TrimSuffix(filepath.Base(f), ".json")]; !ok {
				t.Errorf("stale golden %s has no scenario", f)
			}
		}
	}
}

func TestGoldensSettleAndCommitSleep(t *testing.T) {
	s := mustSpec(t)
	g := record(s, "sleep-commit", scenarios(s)["sleep-commit"])
	last := g.Frames[len(g.Frames)-1]
	if !last.Stop || last.Pose.State != "sleeping" {
		t.Fatalf("sleep scenario does not end in a stopped sleeping pose: %+v", last)
	}
	for _, name := range []string{"state-listening", "state-thinking", "gesture-wake"} {
		g := record(s, name, scenarios(s)[name])
		f := g.Frames[len(g.Frames)-1].Pose
		st, _ := s.State(f.State)
		if math.Abs(f.Flat-st.Flat) > 1e-3 || math.Abs(f.Face-st.Face) > 1e-3 || math.Abs(f.Loose-st.Loose) > 1e-3 || f.Gesture != "" {
			t.Errorf("%s did not converge on its state shape: %+v", name, f)
		}
	}
}

// ledgerLine is one line of the surface's HYAKKI_ONI_LEDGER: every engine
// call in order; steps carry their raw dt in seconds, the input and, when
// committed, the drawn pose.
type ledgerLine struct {
	T       float64 `json:"t"`
	Ev      string  `json:"ev"`
	State   string  `json:"state"`
	Reason  string  `json:"reason"`
	Gesture string  `json:"gesture"`
	Dt      float64 `json:"dt"`
	Input   *Input  `json:"input"`
	Commit  bool    `json:"commit"`
	Pose    *Pose   `json:"pose"`
}

func replayLedger(s *spec.OniSpec, r io.Reader) (commits int, err error) {
	e := New(s)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for n := 1; sc.Scan(); n++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var l ledgerLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			return commits, fmt.Errorf("line %d: %w", n, err)
		}
		switch l.Ev {
		case "state":
			e.SetState(l.State, l.Reason)
		case "gesture":
			e.Play(l.Gesture)
		case "reset":
			e.Reset()
		case "step":
			var in Input
			if l.Input != nil {
				in = *l.Input
			}
			tk := e.Step(time.Duration(math.Round(l.Dt*float64(time.Second))), in)
			if tk.Redraw != l.Commit {
				return commits, fmt.Errorf("line %d (t=%v): ledger commit=%v, replay redraw=%v", n, l.T, l.Commit, tk.Redraw)
			}
			if !l.Commit {
				continue
			}
			if l.Pose == nil {
				return commits, fmt.Errorf("line %d: committed step without a pose", n)
			}
			if d := poseDiff(*l.Pose, tk.Pose, 1e-6); d != "" {
				return commits, fmt.Errorf("line %d (t=%v): %s", n, l.T, d)
			}
			commits++
		}
	}
	if err := sc.Err(); err != nil {
		return commits, err
	}
	if commits == 0 {
		return 0, errors.New("ledger has no committed frames")
	}
	return commits, nil
}

// TestLedgerReplay proves the shared engine drove the surface's pixels: it
// replays a lab ledger and requires every committed pose to match.
func TestLedgerReplay(t *testing.T) {
	path := os.Getenv("HYAKKI_ONI_REPLAY")
	if path == "" {
		t.Skip("set HYAKKI_ONI_REPLAY to a surface ledger to replay it")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	commits, err := replayLedger(mustSpec(t), f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("replayed %d committed frames", commits)
}

func TestLedgerReplayDetectsDrift(t *testing.T) {
	s := mustSpec(t)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	e := New(s)
	at := 0.0
	write := func(l ledgerLine) {
		l.T = at
		if err := enc.Encode(l); err != nil {
			t.Fatal(err)
		}
	}
	step := func(dt time.Duration, in Input) {
		tk := e.Step(dt, in)
		at += dt.Seconds()
		l := ledgerLine{Ev: "step", Dt: dt.Seconds(), Input: &in, Commit: tk.Redraw}
		if tk.Redraw {
			l.Pose = &tk.Pose
		}
		write(l)
	}
	step(0, Input{})
	e.SetState("listening", "")
	write(ledgerLine{Ev: "state", State: "listening"})
	for i := range 40 {
		step(frame+time.Duration(i)*time.Microsecond, Input{Pointer: &spec.Point{X: float64(i), Y: -10}, Mic: 0.3})
	}
	e.Play("poke")
	write(ledgerLine{Ev: "gesture", Gesture: "poke"})
	for range 30 {
		step(frame, Input{})
	}

	good := buf.String()
	if n, err := replayLedger(s, strings.NewReader(good)); err != nil || n < 60 {
		t.Fatalf("faithful ledger rejected after %d commits: %v", n, err)
	}
	lines := strings.Split(strings.TrimSpace(good), "\n")
	var l ledgerLine
	if err := json.Unmarshal([]byte(lines[20]), &l); err != nil || l.Pose == nil {
		t.Fatalf("line 20 is not a committed step: %v", err)
	}
	l.Pose.LeanX += 1e-4
	bad, _ := json.Marshal(l)
	lines[20] = string(bad)
	if _, err := replayLedger(s, strings.NewReader(strings.Join(lines, "\n"))); err == nil || !strings.Contains(err.Error(), "leanX") {
		t.Fatalf("a drifted pose replayed clean: %v", err)
	}
	dropped := strings.Split(strings.TrimSpace(good), "\n")
	dropped = append(dropped[:1], dropped[2:]...)
	if _, err := replayLedger(s, strings.NewReader(strings.Join(dropped, "\n"))); err == nil {
		t.Fatal("a ledger missing its state event replayed clean")
	}
}
