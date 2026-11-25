package strand

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

var update = flag.Bool("update", false, "rewrite strand/testdata/trajectories")

const (
	frame = time.Second / 60
	every = 4
	// pointEvery thins stroke points in the goldens; the last point is
	// always kept.
	pointEvery = 8
)

type goldenStroke struct {
	N     int          `json:"n"`
	Width float64      `json:"width"`
	Alpha float64      `json:"alpha"`
	Pts   []spec.Point `json:"pts"`
}

type goldenFrame struct {
	I       int            `json:"i"`
	FPS     int            `json:"fps"`
	Redraw  bool           `json:"redraw"`
	Stop    bool           `json:"stop"`
	Seq     uint64         `json:"seq"`
	Phase   Phase          `json:"phase"`
	U       float64        `json:"u"`
	Tip     spec.Point     `json:"tip"`
	Loosen  float64        `json:"loosen"`
	Bbox    Rect           `json:"bbox"`
	Char    *CharPose      `json:"char,omitempty"`
	Arrived bool           `json:"arrived,omitempty"`
	Skipped string         `json:"skipped,omitempty"`
	Strokes []goldenStroke `json:"strokes"`
}

type golden struct {
	Scenario string        `json:"scenario"`
	FrameS   float64       `json:"frameS"`
	Every    int           `json:"every"`
	Env      Env           `json:"env"`
	Frames   []goldenFrame `json:"frames"`
}

type recorder struct {
	e      *Engine
	env    Env
	i      int
	frames []goldenFrame
}

func thin(st Stroke) goldenStroke {
	g := goldenStroke{N: len(st.Pts), Width: st.Width, Alpha: st.Alpha}
	for i, p := range st.Pts {
		if i%pointEvery == 0 || i == len(st.Pts)-1 {
			g.Pts = append(g.Pts, p)
		}
	}
	return g
}

func (r *recorder) run(n int) {
	for range n {
		tk := r.e.Step(frame, r.env)
		if r.i%every == 0 || tk.Stop || tk.Frame.Arrived || tk.Frame.Phase != r.last() {
			f := tk.Frame
			gf := goldenFrame{I: r.i, FPS: tk.FPS, Redraw: tk.Redraw, Stop: tk.Stop, Seq: f.Seq, Phase: f.Phase, U: f.U, Tip: f.Tip,
				Loosen: f.Loosen, Bbox: f.Bbox, Char: f.Char, Arrived: f.Arrived, Skipped: f.Skipped, Strokes: []goldenStroke{}}
			for _, st := range f.Strokes {
				gf.Strokes = append(gf.Strokes, thin(st))
			}
			r.frames = append(r.frames, gf)
		}
		r.i++
	}
}

func (r *recorder) last() Phase {
	if len(r.frames) == 0 {
		return ""
	}
	return r.frames[len(r.frames)-1].Phase
}

func scenarios() map[string]func(r *recorder) {
	far := Rect{X: 1300, Y: 120, W: 90, H: 30}
	win := Rect{X: 200, Y: 150, W: 800, H: 600}
	region := Rect{X: 400, Y: 300, W: 500, H: 300}
	inRegion := Rect{X: 450, Y: 350, W: 100, H: 40}
	sp, err := spec.Load()
	if err != nil {
		panic(err)
	}
	half := Rect{X: 0, Y: 0, W: 960, H: 1080}
	maxed := Rect{X: 0, Y: 32, W: 1920, H: 1048}
	// perchAt seats the character on frame's corner (its anchor at the
	// chin of the seat) and sends the perch-mode target.
	perchAt := func(r *recorder, f Rect, quiet bool) string {
		c := PerchCorner(sp, f, r.env.Anchor, r.env.Centre, r.env.Screen)
		r.env.Anchor = seatAnchor(sp, f, c, r.env.Screen)
		r.e.Target(Target{Seq: 1, Box: f, Frame: &f, Mode: ModePerch, Corner: c, Quiet: quiet})
		return c
	}
	return map[string]func(r *recorder){
		"reach-tap-hold": func(r *recorder) {
			r.e.Target(Target{Seq: 1, Box: control})
			r.run(90)
		},
		"retarget-swing": func(r *recorder) {
			r.e.Target(Target{Seq: 1, Box: control, Quiet: true})
			r.run(30)
			r.e.Target(Target{Seq: 2, Box: far, Quiet: true})
			r.run(50)
		},
		"rewind": func(r *recorder) {
			r.e.Target(Target{Seq: 1, Box: control, Quiet: true})
			r.run(70)
			r.e.Retract(1)
			r.run(40)
		},
		"perch": func(r *recorder) {
			r.e.Target(Target{Seq: 1, Box: win, Frame: &win})
			r.run(90)
			r.e.Retract(1)
			r.run(70)
		},
		"lasso": func(r *recorder) {
			r.e.Target(Target{Seq: 1, Box: control, Mode: ModeLasso})
			r.run(90)
			r.e.Retract(1)
			r.run(30)
		},
		"region": func(r *recorder) {
			r.e.Target(Target{Seq: 1, Box: inRegion, Region: &region, Mode: ModeRegion, Quiet: true})
			r.run(170)
			r.e.Retract(1)
			r.run(30)
		},
		"swing-to-lasso": func(r *recorder) {
			r.e.Target(Target{Seq: 1, Box: control, Quiet: true})
			r.run(60)
			r.e.Target(Target{Seq: 2, Box: far, Mode: ModeLasso, Quiet: true})
			r.run(100)
		},
		"reduced": func(r *recorder) {
			r.env.Reduced = true
			r.e.Target(Target{Seq: 1, Box: control, Mode: ModeLasso})
			r.run(3)
			r.e.Retract(1)
			r.run(3)
		},
		"skip-cover": func(r *recorder) {
			r.e.Target(Target{Seq: 1, Box: Rect{X: 100, Y: 100, W: 1200, H: 700}})
			r.run(3)
		},
		"perch-half": func(r *recorder) {
			perchAt(r, half, false)
			r.run(90)
			r.e.Retract(1)
			r.run(70)
		},
		"perch-max": func(r *recorder) {
			perchAt(r, maxed, false)
			r.run(90)
			r.e.Retract(1)
			r.run(70)
		},
		// The window slides a little (the character jumps with it and the
		// curl is rebuilt), then far (the thread winds home, the character
		// flies, and the thread curls again without a second arrival).
		"perch-follow": func(r *recorder) {
			c := perchAt(r, half, true)
			r.run(80)
			near := Rect{X: 0, Y: 40, W: 960, H: 1040}
			r.env.Anchor = seatAnchor(sp, near, c, r.env.Screen)
			r.e.Reframe(near)
			r.run(8)
			r.e.Retract(1)
			r.run(70)
			moved := Rect{X: 600, Y: 300, W: 900, H: 600}
			r.env.Anchor = seatAnchor(sp, moved, c, r.env.Screen)
			r.e.Reperch(moved)
			r.run(90)
			r.e.Retract(1)
			r.run(70)
		},
	}
}

// seatAnchor is the thread root of a character seated on frame's corner:
// the spec anchor on its normal-size frame at the perch seat.
func seatAnchor(s *spec.OniSpec, frame Rect, corner string, screen Rect) spec.Point {
	side := s.Character.ButtonPx
	p := PerchSeat(s, frame, corner, side, screen)
	return spec.Point{X: p.X + s.Strand.Anchor.X*side, Y: p.Y + s.Strand.Anchor.Y*side}
}

func record(s *spec.OniSpec, name string, play func(r *recorder)) golden {
	r := &recorder{e: New(s), env: baseEnv()}
	r.run(1)
	play(r)
	return golden{Scenario: name, FrameS: frame.Seconds(), Every: every, Env: r.env, Frames: r.frames}
}

func ptsDiff(what string, want, got []spec.Point, tol float64) string {
	if len(want) != len(got) {
		return fmt.Sprintf("%s: %d points, golden %d", what, len(got), len(want))
	}
	for i := range want {
		if !(dist(want[i], got[i]) <= tol) {
			return fmt.Sprintf("%s[%d]: want %v, got %v", what, i, want[i], got[i])
		}
	}
	return ""
}

func frameDiff(want, got goldenFrame, tol float64) string {
	if want.I != got.I || want.FPS != got.FPS || want.Redraw != got.Redraw || want.Stop != got.Stop || want.Seq != got.Seq ||
		want.Phase != got.Phase || want.Arrived != got.Arrived || want.Skipped != got.Skipped {
		return fmt.Sprintf("flags: want i=%d fps=%d redraw=%v stop=%v seq=%d %s arrived=%v skipped=%q, got i=%d fps=%d redraw=%v stop=%v seq=%d %s arrived=%v skipped=%q",
			want.I, want.FPS, want.Redraw, want.Stop, want.Seq, want.Phase, want.Arrived, want.Skipped,
			got.I, got.FPS, got.Redraw, got.Stop, got.Seq, got.Phase, got.Arrived, got.Skipped)
	}
	for _, c := range []struct {
		n    string
		w, g float64
	}{{"u", want.U, got.U}, {"loosen", want.Loosen, got.Loosen}, {"bbox.x", want.Bbox.X, got.Bbox.X}, {"bbox.y", want.Bbox.Y, got.Bbox.Y},
		{"bbox.w", want.Bbox.W, got.Bbox.W}, {"bbox.h", want.Bbox.H, got.Bbox.H}} {
		if !(math.Abs(c.w-c.g) <= tol) {
			return fmt.Sprintf("%s: want %v, got %v", c.n, c.w, c.g)
		}
	}
	if d := ptsDiff("tip", []spec.Point{want.Tip}, []spec.Point{got.Tip}, tol); d != "" {
		return d
	}
	if (want.Char == nil) != (got.Char == nil) {
		return fmt.Sprintf("char: want %v, got %v", want.Char, got.Char)
	}
	if want.Char != nil {
		if d := ptsDiff("char", []spec.Point{want.Char.Pos}, []spec.Point{got.Char.Pos}, tol); d != "" || math.Abs(want.Char.Scale-got.Char.Scale) > tol {
			return "char: " + d
		}
	}
	if len(want.Strokes) != len(got.Strokes) {
		return fmt.Sprintf("%d strokes, golden %d", len(got.Strokes), len(want.Strokes))
	}
	for i := range want.Strokes {
		w, g := want.Strokes[i], got.Strokes[i]
		if w.N != g.N || math.Abs(w.Width-g.Width) > tol || math.Abs(w.Alpha-g.Alpha) > tol {
			return fmt.Sprintf("stroke %d: want n=%d w=%v a=%v, got n=%d w=%v a=%v", i, w.N, w.Width, w.Alpha, g.N, g.Width, g.Alpha)
		}
		if d := ptsDiff(fmt.Sprintf("stroke %d", i), w.Pts, g.Pts, tol); d != "" {
			return d
		}
	}
	return ""
}

// The goldens pin every phase's timing and geometry so the later Ryoku
// renderer can check itself against the same frames.
func TestTrajectoryGoldens(t *testing.T) {
	s := mustSpec(t)
	dir := filepath.Join("testdata", "trajectories")
	all := scenarios()
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
			if want.Env != got.Env {
				t.Fatalf("golden env %+v, scenario env %+v", want.Env, got.Env)
			}
			if len(want.Frames) != len(got.Frames) {
				t.Fatalf("%d frames, golden has %d", len(got.Frames), len(want.Frames))
			}
			for i := range want.Frames {
				if d := frameDiff(want.Frames[i], got.Frames[i], 1e-9); d != "" {
					t.Fatalf("frame %d (step %d): %s", i, want.Frames[i].I, d)
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

func TestGoldensLandAndSettle(t *testing.T) {
	s := mustSpec(t)
	for name, play := range scenarios() {
		g := record(s, name, play)
		arrived := 0
		for _, f := range g.Frames {
			if f.Arrived {
				arrived++
			}
		}
		want := 1
		if name == "swing-to-lasso" {
			want = 2
		}
		if arrived != want {
			t.Errorf("%s: %d arrivals, want %d", name, arrived, want)
		}
		last := g.Frames[len(g.Frames)-1]
		switch name {
		case "rewind", "perch", "lasso", "region", "reduced", "skip-cover", "perch-half", "perch-max", "perch-follow":
			if !last.Stop || last.Phase != PhaseHome || len(last.Strokes) != 0 {
				t.Errorf("%s does not end stopped at home: %+v", name, last)
			}
		}
	}
}

// ledgerLine is one strand line of a surface ledger: targets, retracts,
// perch reframes and reperches (box is the new frame) in order, and steps
// carrying their raw dt, the strand Env and, when the strand committed,
// what it drew.
type ledgerLine struct {
	T            float64     `json:"t"`
	Ev           string      `json:"ev"`
	Target       *Target     `json:"target,omitempty"`
	Seq          uint64      `json:"seq,omitempty"`
	Box          *Rect       `json:"box,omitempty"`
	Dt           float64     `json:"dt"`
	Env          *Env        `json:"strandEnv,omitempty"`
	StrandCommit bool        `json:"strandCommit"`
	Strand       *StrandInfo `json:"strand,omitempty"`
}

// StrandInfo is what a committed strand frame records in a ledger.
type StrandInfo struct {
	Phase Phase      `json:"phase"`
	U     float64    `json:"u"`
	Tip   spec.Point `json:"tip"`
	Seq   uint64     `json:"seq"`
	Bbox  Rect       `json:"bbox"`
}

func infoOf(f Frame) StrandInfo {
	return StrandInfo{Phase: f.Phase, U: f.U, Tip: f.Tip, Seq: f.Seq, Bbox: f.Bbox}
}

func infoDiff(want, got StrandInfo, tol float64) string {
	if want.Phase != got.Phase || want.Seq != got.Seq {
		return fmt.Sprintf("phase/seq: want %s/%d, got %s/%d", want.Phase, want.Seq, got.Phase, got.Seq)
	}
	for _, c := range []struct {
		n    string
		w, g float64
	}{{"u", want.U, got.U}, {"tip.x", want.Tip.X, got.Tip.X}, {"tip.y", want.Tip.Y, got.Tip.Y},
		{"bbox.x", want.Bbox.X, got.Bbox.X}, {"bbox.y", want.Bbox.Y, got.Bbox.Y}, {"bbox.w", want.Bbox.W, got.Bbox.W}, {"bbox.h", want.Bbox.H, got.Bbox.H}} {
		if !(math.Abs(c.w-c.g) <= tol) {
			return fmt.Sprintf("%s: want %v, got %v", c.n, c.w, c.g)
		}
	}
	return ""
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
		case "strand-target":
			if l.Target == nil {
				return commits, fmt.Errorf("line %d: strand-target without a target", n)
			}
			e.Target(*l.Target)
		case "strand-retract":
			e.Retract(l.Seq)
		case "strand-reframe", "strand-reperch":
			if l.Box == nil {
				return commits, fmt.Errorf("line %d: %s without a box", n, l.Ev)
			}
			if l.Ev == "strand-reframe" {
				e.Reframe(*l.Box)
			} else {
				e.Reperch(*l.Box)
			}
		case "step":
			if l.Env == nil {
				continue // a character-only step
			}
			tk := e.Step(time.Duration(math.Round(l.Dt*float64(time.Second))), *l.Env)
			if tk.Redraw != l.StrandCommit {
				return commits, fmt.Errorf("line %d (t=%v): ledger strand commit=%v, replay redraw=%v", n, l.T, l.StrandCommit, tk.Redraw)
			}
			if !l.StrandCommit {
				continue
			}
			if l.Strand == nil {
				return commits, fmt.Errorf("line %d: committed strand step without strand info", n)
			}
			if d := infoDiff(*l.Strand, infoOf(tk.Frame), 1e-6); d != "" {
				return commits, fmt.Errorf("line %d (t=%v): %s", n, l.T, d)
			}
			commits++
		}
	}
	if err := sc.Err(); err != nil {
		return commits, err
	}
	if commits == 0 {
		return 0, errors.New("ledger has no committed strand frames")
	}
	return commits, nil
}

// TestLedgerReplay proves the shared engine drove the surface's strand: it
// replays a lab ledger and requires every committed strand frame to match.
func TestLedgerReplay(t *testing.T) {
	path := os.Getenv("HYAKKI_ONI_REPLAY")
	if path == "" {
		t.Skip("set HYAKKI_ONI_REPLAY to a surface ledger to replay its strand")
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
	t.Logf("replayed %d committed strand frames", commits)
}

func TestLedgerReplayDetectsDrift(t *testing.T) {
	s := mustSpec(t)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	e := New(s)
	env := baseEnv()
	at := 0.0
	write := func(l ledgerLine) {
		l.T = at
		if err := enc.Encode(l); err != nil {
			t.Fatal(err)
		}
	}
	stepTo := func(n int) {
		for i := range n {
			env.Anchor.X -= 0.5 * float64(i%3)
			dt := frame + time.Duration(i%5)*time.Microsecond
			tk := e.Step(dt, env)
			at += dt.Seconds()
			ev := env
			l := ledgerLine{Ev: "step", Dt: dt.Seconds(), Env: &ev, StrandCommit: tk.Redraw}
			if tk.Redraw {
				info := infoOf(tk.Frame)
				l.Strand = &info
			}
			write(l)
		}
	}
	stepTo(1)
	tg := Target{Seq: 1, Box: control}
	e.Target(tg)
	write(ledgerLine{Ev: "strand-target", Target: &tg})
	stepTo(40)
	e.Retract(1)
	write(ledgerLine{Ev: "strand-retract", Seq: 1})
	stepTo(40)

	good := buf.String()
	if n, err := replayLedger(s, strings.NewReader(good)); err != nil || n < 60 {
		t.Fatalf("faithful ledger rejected after %d commits: %v", n, err)
	}
	lines := strings.Split(strings.TrimSpace(good), "\n")
	var l ledgerLine
	if err := json.Unmarshal([]byte(lines[20]), &l); err != nil || l.Strand == nil {
		t.Fatalf("line 20 is not a committed strand step: %v", err)
	}
	l.Strand.Tip.X += 1e-4
	bad, _ := json.Marshal(l)
	lines[20] = string(bad)
	if _, err := replayLedger(s, strings.NewReader(strings.Join(lines, "\n"))); err == nil || !strings.Contains(err.Error(), "tip.x") {
		t.Fatalf("a drifted tip replayed clean: %v", err)
	}
	dropped := strings.Split(strings.TrimSpace(good), "\n")
	dropped = append(dropped[:1], dropped[2:]...)
	if _, err := replayLedger(s, strings.NewReader(strings.Join(dropped, "\n"))); err == nil {
		t.Fatal("a ledger missing its target replayed clean")
	}
}
