package tool

import "ryoku-rashin/internal/protocol"

// Mode is what a runner is doing, and with it which tools it may offer. The
// engine's mode profiles (engine.ModeProfile) add the prompt and caps.
type Mode string

const (
	// ModeAskRead is an Ask in Read only: it reads the machine to answer and
	// changes nothing.
	ModeAskRead Mode = "ask-read"
	// ModeAskAct is an Ask in Ask first or YOLO: it may run commands and
	// write files the user owns, under the session's approvals.
	ModeAskAct Mode = "ask-act"
	// ModeAgent is an Agent session working a folder under its approvals.
	ModeAgent Mode = "agent"
	// ModePlan is an Agent session with Plan first on, until the user
	// approves its plan: read-only tools and the todo list.
	ModePlan Mode = "plan"
	// ModeTeam is a team task's session, its tools run in the task's
	// sandbox.
	ModeTeam Mode = "team"
	// ModeDreamManager is the lead of a dream: it hires, delegates and
	// presents.
	ModeDreamManager Mode = "dream-manager"
	// ModeSubagent is a child run of the task tool, inside its parent's
	// turn.
	ModeSubagent Mode = "subagent"
)

// Modes lists every mode in a fixed order.
var Modes = []Mode{ModeAskRead, ModeAskAct, ModeAgent, ModePlan, ModeTeam, ModeDreamManager, ModeSubagent}

// Class is how much a model can carry: it picks the tool set, the tool
// descriptions and schemas, and the prompt template.
type Class string

const (
	// ClassCompact is for light models: at most MaxCompactTools schemas,
	// one-line descriptions, find/replace edits, a system prompt of at most
	// 3 KB written to the small-model rules, temperature at most 0.3.
	ClassCompact Class = "compact"
	// ClassFull is every tool of the mode, hashline anchored edits and the
	// full prompt.
	ClassFull Class = "full"
)

// MaxCompactTools caps the tool schemas a compact profile offers.
const MaxCompactTools = 8

// ClassFor picks the class for a model: compact when its catalogue class is
// light (light), when it has at most protocol.LightParams parameters, or when
// it is a local model whose size is unknown (params 0); full otherwise. A
// remote model of unknown size is full: the free remote routes are large.
func ClassFor(params int64, light, local bool) Class {
	switch {
	case light:
		return ClassCompact
	case params > 0 && params <= protocol.LightParams:
		return ClassCompact
	case params == 0 && local:
		return ClassCompact
	default:
		return ClassFull
	}
}

// ClassForModel is ClassFor on a models.list entry.
func ClassForModel(m protocol.ModelInfo) Class {
	return ClassFor(m.Params, m.Light, m.Tier == protocol.TierLocal)
}

// Profile is what a tool is told about the run it serves.
type Profile struct {
	Mode  Mode
	Class Class
	// ReadOnly offers only tools whose ReadOnly is true: Ask in Read only,
	// Plan first, and a subagent whose parent may not write.
	ReadOnly bool
	// Window is the context window of the model in use, in tokens; 0 when
	// unknown, which Limits treats as DefaultWindow.
	Window int
}

// Compact reports the compact class.
func (p Profile) Compact() bool { return p.Class == ClassCompact }

// Limits are the output caps for this profile.
func (p Profile) Limits() Limits { return LimitsFor(p.Window, p.Class) }

// DefaultWindow is assumed for a model whose window is unknown: the smallest
// a local model is run with.
const DefaultWindow = 16384

// Limits caps what one tool result may put in front of the model, scaled to
// the window so four to six results fit beside the system prompt.
type Limits struct {
	// Told caps Result.Text in bytes; the engine cuts anything longer as a
	// last resort, so a tool should shape its own output below it.
	Told int
	// Shown caps Result.Shown in runes, for the tool line.
	Shown int
	// Head and Tail are the bytes a tool keeps from the start and end of an
	// output longer than Told, with a note of what was left out between.
	Head, Tail int
	// TailLines is how many last lines of command output to keep when only
	// the end matters.
	TailLines int
	// ReadLines is how many lines read returns without an explicit range.
	ReadLines int
	// LineBytes caps one shown line; longer lines are cut with a marker.
	LineBytes int
	// GrepFiles and GrepPerFile cap grep's files and matches per file.
	GrepFiles, GrepPerFile int
	// Paths caps find_files and glob results; Entries caps ls.
	Paths, Entries int
}

// ShownRunes caps a tool line's output in the app.
const ShownRunes = 6000

// LimitsFor scales the caps to a window of tokens. Told is about 6% of the
// window (four bytes a token), between 4 KiB and 50 KiB: about 7.7 KiB at
// 32K. The other caps grow with Told from their values at 8 KiB.
func LimitsFor(window int, class Class) Limits {
	if window <= 0 {
		window = DefaultWindow
	}
	told := clampInt(window*6/100*4, 4<<10, 50<<10)
	scale := func(base, lo, hi int) int { return clampInt(base*told/(8<<10), lo, hi) }
	l := Limits{
		Told:        told,
		Shown:       ShownRunes,
		Head:        told * 3 / 8,
		Tail:        told * 3 / 8,
		TailLines:   scale(120, 60, 400),
		ReadLines:   scale(150, 100, 300),
		LineBytes:   512,
		GrepFiles:   scale(10, 10, 50),
		GrepPerFile: scale(5, 5, 20),
		Paths:       scale(50, 50, 200),
		Entries:     scale(100, 100, 256),
	}
	if class == ClassCompact {
		l.LineBytes = 300
		l.GrepPerFile = 5
	}
	return l
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
