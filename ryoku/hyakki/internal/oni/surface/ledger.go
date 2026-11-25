package surface

import (
	"encoding/json"
	"image"
	"io"
	"os"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
)

// Ledger event kinds. "commit" is exactly one wl_surface.commit carrying new
// pixels (part "strand", "chip" or "highlight" for those surfaces, none for
// the sprite); "state", "gesture", "reset", "strand-target",
// "strand-retract", "strand-reframe", "strand-reperch" and "step" are every
// engine call, in order, so a ledger replays through the motion and strand
// engines.
const (
	EvStart   = "start"
	EvFrame   = "frame"
	EvState   = "state"
	EvGesture = "gesture"
	EvReset   = "reset"
	EvStep    = "step"
	EvCommit  = "commit"
	EvArm     = "arm"
	EvStop    = "stop"
	EvHide    = "hide"
	EvShow    = "show"
	EvOutput  = "output"
	EvDrag    = "drag"
	EvDrop    = "drop"
	EvPlace   = "place"
	EvDormant = "dormant"
	EvRedial  = "redial"
	EvExit    = "exit"

	EvStrandTarget  = "strand-target"
	EvStrandRetract = "strand-retract"
	EvStrandArrive  = "strand-arrive"
	EvStrandSkip    = "strand-skip"
	EvBubble        = "bubble"
	EvSound         = "sound"
	EvStrandReframe = "strand-reframe"
	EvStrandReperch = "strand-reperch"

	EvPerch     = "perch"
	EvDetach    = "detach"
	EvHighlight = "highlight"
	EvOverlay   = "overlay"

	// EvKeys is the talk-chord reader starting (reason evdev or fifo),
	// stopping, or unable to start (a keys.Reason, fifo-refused or error);
	// EvChord is one chord edge (event down, up or cancel), never a key;
	// EvMicMark is the mic mark coming on or going off (reason on, off).
	EvKeys    = "keys"
	EvChord   = "chord"
	EvMicMark = "micmark"

	// EvGuide is a guide frame the card took (its seq, phase, step and
	// actions, never its words), or the card let go (phase idle);
	// EvCardPress is one pill press sent to the daemon (reason the
	// action); EvPick is the pick overlay's surfaces changing (reason
	// armed, point, cancel, daemon or lost, outputs the ones mapped
	// after), never where the pointer is.
	EvGuide     = "guide"
	EvCardPress = "card-press"
	EvPick      = "pick"

	// EvHands is a Hands frame taken: its phase and step, and as reason
	// the pause while paused or the decision's reason once inactive (and end,
	// the way the task ended), never a page's words. EvHandsPress is one pill or card press sent to the
	// daemon (reason the action: stop, resume or not-now).
	EvHands      = "hands"
	EvHandsPress = "hands-press"
	// EvApproval is an approval frame taken: its id as event, reason open
	// or closed, and its scope as kind, never its title. EvApprovalPress
	// is one of its buttons sent as oni.consent.answer (event the id,
	// reason the option).
	EvApproval      = "approval"
	EvApprovalPress = "approval-press"
)

// Surface parts on commit and hide lines; the sprite has none.
const (
	PartStrand    = "strand"
	PartChip      = "chip"
	PartHighlight = "highlight"
	PartCard      = "card"
	// PartPill and PartHandoff are Hands' pill and handoff card, and
	// PartApproval an approval card in the handoff card's place; PartGlow
	// the glow's strips on one output, a commit line carrying them as Rects
	// and the strength they were painted at as Level.
	PartPill     = "pill"
	PartHandoff  = "handoff"
	PartApproval = "approval"
	PartGlow     = "glow"
)

// Entry is one ledger line. Pose and Input hold motion values as they
// marshal. A step that stepped the strand engine carries StrandEnv and
// StrandCommit (the strand's Redraw), plus Strand when it redrew.
type Entry struct {
	T       float64  `json:"t"`
	Ev      string   `json:"ev"`
	State   string   `json:"state,omitempty"`
	Gesture string   `json:"gesture,omitempty"`
	Reason  string   `json:"reason,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	DT      *float64 `json:"dt,omitempty"`
	Input   any      `json:"input,omitempty"`
	Commit  *bool    `json:"commit,omitempty"`
	Pose    any      `json:"pose,omitempty"`
	Rect    *[4]int  `json:"rect,omitempty"`
	Output  string   `json:"output,omitempty"`
	Scale   float64  `json:"scale,omitempty"`
	FPS     int      `json:"fps,omitempty"`
	Seq     uint64   `json:"seq,omitempty"`
	FX      *float64 `json:"fx,omitempty"`
	FY      *float64 `json:"fy,omitempty"`
	Part    string   `json:"part,omitempty"`

	Strand       *StrandInfo    `json:"strand,omitempty"`
	StrandEnv    *strand.Env    `json:"strandEnv,omitempty"`
	StrandCommit *bool          `json:"strandCommit,omitempty"`
	Target       *strand.Target `json:"target,omitempty"`
	Box          *strand.Rect   `json:"box,omitempty"`
	Char         *CharInfo      `json:"char,omitempty"`
	Skipped      string         `json:"skipped,omitempty"`
	Event        string         `json:"event,omitempty"`
	Motif        string         `json:"motif,omitempty"`
	Perch        *PerchInfo     `json:"perch,omitempty"`
	Rects        []strand.Rect  `json:"rects,omitempty"`
	// Phase and Source are a voice frame's, never its words; Mark is a
	// commit carrying the mic mark.
	Phase  string `json:"phase,omitempty"`
	Source string `json:"source,omitempty"`
	Mark   bool   `json:"mark,omitempty"`
	// Step and Actions are a guide frame's; Outputs the pick surfaces
	// mapped.
	Step    int      `json:"step,omitempty"`
	Actions []string `json:"actions,omitempty"`
	Outputs []string `json:"outputs,omitempty"`
	// Level is the glow's strength on a glow commit; Buttons the actions
	// of a pill or handoff card commit, output-local logical px.
	Level   *float64     `json:"level,omitempty"`
	Buttons []ButtonInfo `json:"buttons,omitempty"`
	// End is how a Hands task ended, on the hands line that ended it;
	// Scope where a running task works, on its hands lines, with Output
	// the picked window's output for a live window.
	End   string `json:"end,omitempty"`
	Scope string `json:"scope,omitempty"`

	Display string `json:"display"`
}

// StrandInfo is what a strand frame drew, in output-local logical px.
type StrandInfo struct {
	Phase strand.Phase `json:"phase"`
	U     float64      `json:"u"`
	Tip   spec.Point   `json:"tip"`
	Seq   uint64       `json:"seq"`
	Bbox  strand.Rect  `json:"bbox"`
}

func strandInfo(f strand.Frame) *StrandInfo {
	return &StrandInfo{Phase: f.Phase, U: f.U, Tip: f.Tip, Seq: f.Seq, Bbox: f.Bbox}
}

// ButtonInfo is one action on Hands' pill or handoff card and where it is,
// output-local logical px.
type ButtonInfo struct {
	Action string  `json:"action"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
}

func rectInfo(r image.Rectangle) strand.Rect {
	return strand.Rect{X: float64(r.Min.X), Y: float64(r.Min.Y), W: float64(r.Dx()), H: float64(r.Dy())}
}

// CharInfo is the character's centre (output-local logical px) and scale
// while a region pen carries it.
type CharInfo struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Scale float64 `json:"scale"`
}

// PerchInfo is the window a perch line names: its id, the top corner the
// character sits by, and its frame, output-local logical px.
type PerchInfo struct {
	Window string      `json:"window"`
	Corner string      `json:"corner"`
	Box    strand.Rect `json:"box"`
}

// Ledger writes one JSON line per event. A nil *Ledger records nothing.
type Ledger struct {
	mu      sync.Mutex
	w       io.Writer
	c       io.Closer
	start   time.Time
	now     func() time.Time
	display string
}

// OpenLedger appends to path. An empty path is a nil ledger.
func OpenLedger(path, display string) (*Ledger, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	l := newLedger(f, display, time.Now)
	l.c = f
	return l, nil
}

func newLedger(w io.Writer, display string, now func() time.Time) *Ledger {
	return &Ledger{w: w, now: now, start: now(), display: display}
}

// Add stamps e with the time since the ledger opened and the display, then
// writes it as one line.
func (l *Ledger) Add(e Entry) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	e.T = l.now().Sub(l.start).Seconds()
	e.Display = l.display
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	l.w.Write(append(b, '\n'))
}

// Close closes the file.
func (l *Ledger) Close() error {
	if l == nil || l.c == nil {
		return nil
	}
	return l.c.Close()
}

func rectOf(r image.Rectangle) *[4]int {
	return &[4]int{r.Min.X, r.Min.Y, r.Dx(), r.Dy()}
}
