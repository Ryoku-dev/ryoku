// Package spec holds Oni's frozen motion reference: the embedded
// oni-spec.json, its strict loader and the pure motion functions every
// renderer shares. release/docs/oni-animation-spec.md carries the same numbers.
package spec

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
)

//go:embed oni-spec.json
var embedded []byte

// Version is the only oni-spec.json schema version this build reads.
const Version = 1

var (
	StateIDs    = []string{"idle", "ready", "listening", "working", "reading", "thinking", "speaking", "error", "sleeping"}
	GestureIDs  = []string{"acknowledge", "done", "error", "poke", "nudge", "wake", "sleep", "arrive", "gulp", "settle"}
	MotifIDs    = []string{"acknowledge", "done", "error", "poke", "nudge", "wake", "sleep", "arrive", "gulp", "settle", "reach", "rewind", "skate", "perch", "bubble", "intro"}
	CueIDs      = []string{"press", "release", "tick", "done-notes", "hands-start", "hands-stop"}
	PaletteIDs  = []string{"idle", "listening", "thinking", "speaking", "done", "error"}
	TierIDs     = []string{"active", "rest", "asleep"}
	ActivityIDs = []string{"asleep", "idle", "breathing", "motion"}
	SizeIDs     = []string{"small", "normal", "big"}
	// KeyframeChannels are the pose parameters a gesture may offset; every
	// gesture's keyframes also carry "at", the progress of each key.
	KeyframeChannels = []string{"scale", "squash", "leanX", "leanY", "unravel", "straighten", "tempo", "bright"}
)

// EventIDs are the moments that play a motif (see Sound.Events).
var EventIDs = []string{
	"gesture-acknowledge", "gesture-done", "gesture-error", "gesture-poke", "gesture-nudge",
	"gesture-wake", "gesture-sleep", "gesture-arrive", "gesture-gulp", "gesture-settle",
	"strand-reach", "strand-lasso", "strand-arrive", "strand-rewind", "strand-perch",
	"bubble-open", "intro-trace", "flight-skate", "flight-land",
}

type Point struct{ X, Y float64 }

// Spring is [stiffness, damping] with mass from Integrator.
type Spring [2]float64

func (s Spring) Stiffness() float64 { return s[0] }
func (s Spring) Damping() float64   { return s[1] }

// Range is [min, max].
type Range [2]float64

// Ease is a CSS cubic-bezier [x1, y1, x2, y2].
type Ease [4]float64

func (e Ease) At(t float64) float64 { return BezierEase(e[0], e[1], e[2], e[3], t) }

type Vec2 [2]float64

// Hover is [amplitude px, rate, breath].
type Hover [3]float64

type OniSpec struct {
	Version       int              `json:"version"`
	Character     Character        `json:"character"`
	Poses         Poses            `json:"poses"`
	Springs       Springs          `json:"springs"`
	States        map[string]State `json:"states"`
	Motion        Motion           `json:"motion"`
	Colour        Colour           `json:"colour"`
	Gestures      Gestures         `json:"gestures"`
	Sleep         Sleep            `json:"sleep"`
	Placement     Placement        `json:"placement"`
	Frames        Frames           `json:"frames"`
	Integrator    Integrator       `json:"integrator"`
	ReducedMotion ReducedMotion    `json:"reducedMotion"`
	LightDesktop  LightDesktop     `json:"lightDesktop"`
	Strand        Strand           `json:"strand"`
	Flight        Flight           `json:"flight"`
	Perch         PerchRules       `json:"perch"`
	Session       Session          `json:"session"`
	Sound         Sound            `json:"sound"`
	Voice         Voice            `json:"voice"`
	Guide         Guide            `json:"guide"`
	Reflex        Reflex           `json:"reflex"`
	Hands         Hands            `json:"hands"`
	Memory        Memory           `json:"memory"`
	Recall        Recall           `json:"recall"`
	Rewrite       Rewrite          `json:"rewrite"`
	Focus         Focus            `json:"focus"`
	Assignment    Assignment       `json:"assignment"`
}

type Character struct {
	ButtonPx     float64       `json:"buttonPx"`
	ShellPx      float64       `json:"shellPx"`
	FramePx      float64       `json:"framePx"`
	RadiusPx     float64       `json:"radiusPx"`
	TubePx       float64       `json:"tubePx"`
	Samples      int           `json:"samples"`
	Supersample  int           `json:"supersample"`
	DPRCap       float64       `json:"dprCap"`
	Trefoil      Trefoil       `json:"trefoil"`
	Wall         float64       `json:"wall"`
	ThickThin    float64       `json:"thickThin"`
	OutlinePx    float64       `json:"outlinePx"`
	RunArc       float64       `json:"runArc"`
	OverlapInk   float64       `json:"overlapInk"`
	OverlapWhite float64       `json:"overlapWhite"`
	Ink          string        `json:"ink"`
	White        string        `json:"white"`
	FallbackS    FallbackFlows `json:"fallbackS"`
}

type Trefoil struct {
	Fov   float64 `json:"fov"`
	Depth float64 `json:"depth"`
}

type FallbackFlows struct {
	FlowA  float64 `json:"flowA"`
	FlowB  float64 `json:"flowB"`
	Glint  float64 `json:"glint"`
	OrbitA float64 `json:"orbitA"`
	OrbitB float64 `json:"orbitB"`
	Core   float64 `json:"core"`
	Halo   float64 `json:"halo"`
}

type Poses struct {
	Flat  FlatPose  `json:"flat"`
	Loose LoosePose `json:"loose"`
}

type FlatPose struct {
	Tilt   float64 `json:"tilt"`
	Squash float64 `json:"squash"`
	Thin   float64 `json:"thin"`
}

type LoosePose struct {
	Lobe           int     `json:"lobe"`
	Toward         Vec2    `json:"toward" oni:"signed"`
	Stem           float64 `json:"stem"`
	Width          float64 `json:"width"`
	TipWidth       float64 `json:"tipWidth"`
	Edge           float64 `json:"edge"`
	RestPx         float64 `json:"restPx"`
	ReachPx        float64 `json:"reachPx"`
	FlightPx       float64 `json:"flightPx"`
	MicPulse       float64 `json:"micPulse"`
	ThreadSpeedPxS float64 `json:"threadSpeedPxS"`
	ThreadMinS     float64 `json:"threadMinS"`
}

type Springs struct {
	Shape      ShapeSprings `json:"shape"`
	Pose       PoseSprings  `json:"pose"`
	PoseRanges PoseRanges   `json:"poseRanges"`
}

type ShapeSprings struct {
	Flat    Spring `json:"flat"`
	Face    Spring `json:"face"`
	Loose   Spring `json:"loose"`
	Tail    Spring `json:"tail"`
	Mirror  Spring `json:"mirror"`
	Aim     Spring `json:"aim"`
	Slack   Spring `json:"slack"`
	Stretch Spring `json:"stretch"`
	LagX    Spring `json:"lagX"`
	LagY    Spring `json:"lagY"`
	Ripple  Spring `json:"ripple"`
}

type PoseSprings struct {
	Scale      Spring `json:"scale"`
	Squash     Spring `json:"squash"`
	LeanX      Spring `json:"leanX"`
	LeanY      Spring `json:"leanY"`
	Unravel    Spring `json:"unravel"`
	Straighten Spring `json:"straighten"`
	Tempo      Spring `json:"tempo"`
	Bright     Spring `json:"bright"`
}

type PoseRanges struct {
	Scale   Range `json:"scale"`
	Squash  Range `json:"squash" oni:"signed"`
	Lean    Range `json:"lean" oni:"signed"`
	Unravel Range `json:"unravel"`
	Tempo   Range `json:"tempo"`
	Bright  Range `json:"bright"`
}

// State is one row of the shape table; Spin is in rad/s.
type State struct {
	Flat  float64 `json:"flat"`
	Face  float64 `json:"face"`
	Loose float64 `json:"loose"`
	Spin  float64 `json:"spin"`
	Hover Hover   `json:"hover"`
	Scale float64 `json:"scale"`
	Tempo float64 `json:"tempo"`
}

type Motion struct {
	SpinStop   SpinStop   `json:"spinStop"`
	Stretch    Stretch    `json:"stretch"`
	Lag        Lag        `json:"lag"`
	Ripple     Ripple     `json:"ripple"`
	Whip       Whip       `json:"whip"`
	DragTailAt float64    `json:"dragTailAt"`
	HoverFloat HoverFloat `json:"hoverFloat"`
	Leans      Leans      `json:"leans"`
	Speaking   Speaking   `json:"speaking"`
	Mic        MicLift    `json:"mic"`
}

type SpinStop struct {
	Spring    Spring  `json:"spring"`
	Lead      float64 `json:"lead"`
	WaitFor   float64 `json:"waitFor"`
	WaitEase  float64 `json:"waitEase"`
	Divisions int     `json:"divisions"`
}

type Stretch struct {
	Per float64 `json:"per"`
	Max float64 `json:"max"`
}

type Lag struct {
	Per   float64 `json:"per"`
	Max   float64 `json:"max"`
	Reach float64 `json:"reach"`
}

type Ripple struct {
	Amp          float64 `json:"amp"`
	Waves        float64 `json:"waves"`
	WaveRate     float64 `json:"waveRate"`
	FullSpeedPxS float64 `json:"fullSpeedPxS"`
}

type Whip struct {
	Amp   float64 `json:"amp"`
	Waves float64 `json:"waves"`
}

type HoverFloat struct {
	Secondary      float64 `json:"secondary"`
	SecondaryPhase float64 `json:"secondaryPhase"`
	QuantizePx     float64 `json:"quantizePx"`
}

type Leans struct {
	Reading        float64 `json:"reading"`
	Pointing       float64 `json:"pointing"`
	Listening      float64 `json:"listening"`
	ListeningMinPx float64 `json:"listeningMinPx"`
	FileHover      float64 `json:"fileHover"`
	DragPerSpeed   float64 `json:"dragPerSpeed"`
	DragMax        float64 `json:"dragMax"`
}

type Speaking struct {
	RateA  float64 `json:"rateA"`
	RateB  float64 `json:"rateB"`
	Gain   float64 `json:"gain"`
	Scale  float64 `json:"scale"`
	Squash float64 `json:"squash" oni:"signed"`
	Bright float64 `json:"bright"`
}

type MicLift struct {
	Bright float64 `json:"bright"`
	Band   float64 `json:"band"`
}

type Colour struct {
	Palette     map[string]string `json:"palette"`
	StateColour map[string]string `json:"stateColour"`
	Lap         Lap               `json:"lap"`
	Glow        Glow              `json:"glow"`
	Mic         MicColour         `json:"mic"`
	Beats       Beats             `json:"beats"`
	FocusGuard  FocusGuard        `json:"focusGuard"`
}

type Lap struct {
	DurationS float64 `json:"durationS"`
	Head      float64 `json:"head"`
	HeadWidth float64 `json:"headWidth"`
	Core      float64 `json:"core"`
	ArriveS   float64 `json:"arriveS"`
	LeaveS    float64 `json:"leaveS"`
}

type Glow struct {
	Spread   float64 `json:"spread"`
	Strength float64 `json:"strength"`
	Low      float64 `json:"low"`
	High     float64 `json:"high"`
	PeriodS  float64 `json:"periodS"`
}

type MicColour struct {
	Floor float64 `json:"floor"`
	Width float64 `json:"width"`
	Glow  float64 `json:"glow"`
	Ease  float64 `json:"ease"`
}

type Beats struct {
	Done  Beat `json:"done"`
	Error Beat `json:"error"`
}

type Beat struct {
	Colour string  `json:"colour"`
	HoldS  float64 `json:"holdS"`
}

type FocusGuard struct {
	Active string `json:"active"`
	Done   string `json:"done"`
}

type Gestures struct {
	HandoffMs       float64              `json:"handoffMs"`
	DurationS       map[string]float64   `json:"durationS"`
	ErrorShakeLeanX []float64            `json:"errorShakeLeanX" oni:"signed"`
	Keyframes       map[string]Keyframes `json:"keyframes" oni:"signed"`
}

// Keyframes are one gesture's deltas on the pose: "at" lists the progress
// of each key in [0,1], and each KeyframeChannels entry present lists one
// delta per key, starting and ending at 0.
type Keyframes map[string][]float64

// Sample returns a channel's delta at progress p, easing each segment with
// smoothstep so keys are rest points; absent channels and progress outside
// [0,1] give 0.
func (k Keyframes) Sample(channel string, p float64) float64 {
	at, vals := k["at"], k[channel]
	if channel == "at" || len(vals) == 0 || len(vals) != len(at) || !(p > 0) || p >= 1 {
		return 0
	}
	i := 1
	for i < len(at)-1 && at[i] <= p {
		i++
	}
	span := at[i] - at[i-1]
	if !(span > 0) {
		return vals[i]
	}
	u := (p - at[i-1]) / span
	u = u * u * (3 - 2*u)
	return vals[i-1] + (vals[i]-vals[i-1])*u
}

type Placement struct {
	CornerMarginPx  int                `json:"cornerMarginPx"`
	DragThresholdPx int                `json:"dragThresholdPx"`
	Sizes           map[string]float64 `json:"sizes"`
}

type Sleep struct {
	IdleMs  float64 `json:"idleMs"`
	CommitS float64 `json:"commitS"`
}

// AsleepStopsAfterCommit records a deliberate deviation from the reference:
// once the sleeping pose commits, the renderer paints once and arms no timer.
type Frames struct {
	TiersFps               map[string]int    `json:"tiersFps"`
	DtCapMs                float64           `json:"dtCapMs"`
	AverageHz              Range             `json:"averageHz"`
	SettledStopMs          float64           `json:"settledStopMs"`
	AsleepStopsAfterCommit bool              `json:"asleepStopsAfterCommit"`
	Tier                   map[string]string `json:"tier"`
	BatteryTier            map[string]string `json:"batteryTier"`
}

type Integrator struct {
	Mass  float64 `json:"mass"`
	SnapX float64 `json:"snapX"`
	SnapV float64 `json:"snapV"`
}

type ReducedMotion struct {
	Loop              bool `json:"loop"`
	FramesPerEvent    int  `json:"framesPerEvent"`
	ThreadsJumpToHold bool `json:"threadsJumpToHold"`
}

type LightDesktop struct {
	SampleMs     float64  `json:"sampleMs"`
	BoxPx        float64  `json:"boxPx"`
	OnAbove      float64  `json:"onAbove"`
	OffBelow     float64  `json:"offBelow"`
	FadeLuma     Range    `json:"fadeLuma"`
	Treatment    string   `json:"treatment"`
	Alternatives []string `json:"alternatives"`
	Outline      Outline  `json:"outline"`
}

type Outline struct {
	Width        float64 `json:"width"`
	Feather      float64 `json:"feather"`
	FeatherAlpha float64 `json:"featherAlpha"`
	ShadowAlpha  float64 `json:"shadowAlpha"`
	ShadowRadius float64 `json:"shadowRadius"`
}

// Outlined applies the luma hysteresis: once on, the outline stays until
// luma drops below OffBelow; once off, it needs luma above OnAbove.
func (l LightDesktop) Outlined(prev bool, luma float64) bool {
	if prev {
		return !(luma < l.OffBelow)
	}
	return luma > l.OnAbove
}

type Strand struct {
	Stroke          Stroke            `json:"stroke"`
	Anchor          StrandAnchor      `json:"anchor"`
	Accents         map[string]string `json:"accents"`
	BowedPath       BowedPathSpec     `json:"bowedPath"`
	TailLoosen      TailLoosen        `json:"tailLoosen"`
	Reach           ReachSpec         `json:"reach"`
	Tap             Tap               `json:"tap"`
	HoldStopMs      float64           `json:"holdStopMs"`
	RetargetBlendMs float64           `json:"retargetBlendMs"`
	Retarget        RetargetSpec      `json:"retarget"`
	Rewind          RewindSpec        `json:"rewind"`
	SpeedFirst      bool              `json:"speedFirst"`
	PreReachDelayMs float64           `json:"preReachDelayMs"`
	Perch           Perch             `json:"perch"`
	Lasso           Lasso             `json:"lasso"`
	Region          Region            `json:"region"`
	Surface         Surface           `json:"surface"`
	Burst           Burst             `json:"burst"`
	Intro           Intro             `json:"intro"`
	Bubble          Bubble            `json:"bubble"`
}

type Stroke struct {
	WidthPx   float64 `json:"widthPx"`
	OutlinePx float64 `json:"outlinePx"`
	DPRCap    float64 `json:"dprCap"`
	BboxPadPx float64 `json:"bboxPadPx"`
}

// StrandAnchor is where the thread leaves the character: X and Y are
// fractions of the art viewBox (origin top-left). Follow weights how much of
// each pose transform the anchor rides, 0 pinned to the unposed point, 1
// moving with the art.
type StrandAnchor struct {
	X      float64      `json:"x"`
	Y      float64      `json:"y"`
	Follow AnchorFollow `json:"follow"`
}

type AnchorFollow struct {
	Scale  float64 `json:"scale"`
	Squash float64 `json:"squash"`
	Lean   float64 `json:"lean"`
	Lift   float64 `json:"lift"`
	Hover  float64 `json:"hover"`
}

// RetargetSpec is the swing to a new target while the thread is out: u
// restarts at max(ReachFrom, current u) and reaches 1 over
// Strand.RetargetBlendMs with Ease, never going home first.
type RetargetSpec struct {
	ReachFrom float64 `json:"reachFrom"`
	Ease      Ease    `json:"ease"`
}

type BowedPathSpec struct {
	Segments        int     `json:"segments"`
	Points          int     `json:"points"`
	Controls        Range   `json:"controls"`
	BowPerDistance  float64 `json:"bowPerDistance"`
	BowMaxPx        float64 `json:"bowMaxPx"`
	BowScale        float64 `json:"bowScale"`
	SeamBlendPoints int     `json:"seamBlendPoints"`
	TipGapPx        float64 `json:"tipGapPx"`
	TipStopPx       float64 `json:"tipStopPx"`
}

type TailLoosen struct {
	UnravelHold float64 `json:"unravelHold"`
	Depth       float64 `json:"depth"`
	Shrink      float64 `json:"shrink"`
}

type ReachSpec struct {
	DurationS       float64 `json:"durationS"`
	Ease            Ease    `json:"ease"`
	SurfaceSpeedPxS float64 `json:"surfaceSpeedPxS"`
	SurfaceClampS   Range   `json:"surfaceClampS"`
	PerchS          float64 `json:"perchS"`
}

type Tap struct {
	DurationS float64 `json:"durationS"`
	BobPx     float64 `json:"bobPx"`
}

type RewindSpec struct {
	DurationS  float64 `json:"durationS"`
	Ease       Ease    `json:"ease"`
	LassoS     float64 `json:"lassoS"`
	LassoScale float64 `json:"lassoScale"`
}

type Perch struct {
	MinWidthPx  float64 `json:"minWidthPx"`
	MinHeightPx float64 `json:"minHeightPx"`
	CurlS       float64 `json:"curlS"`
	RadiusPx    float64 `json:"radiusPx"`
	Segments    int     `json:"segments"`
	Sweep       float64 `json:"sweep"`
	OffsetPx    float64 `json:"offsetPx"`
	UncoilS     float64 `json:"uncoilS"`
	RecoilS     float64 `json:"recoilS"`
}

type Lasso struct {
	Points       int         `json:"points"`
	GapPx        float64     `json:"gapPx"`
	Power        float64     `json:"power"`
	Turns        float64     `json:"turns"`
	DrawS        float64     `json:"drawS"`
	Ease         Ease        `json:"ease"`
	ThrownPx     float64     `json:"thrownPx"`
	CinchS       float64     `json:"cinchS"`
	Ripple       LassoRipple `json:"ripple"`
	LeadPoints   int         `json:"leadPoints"`
	StartAt      float64     `json:"startAt"`
	LandingGapPx float64     `json:"landingGapPx"`
}

type LassoRipple struct {
	Amp   float64 `json:"amp"`
	Waves float64 `json:"waves"`
	Decay float64 `json:"decay"`
}

type Region struct {
	MinControlAreaRatio float64 `json:"minControlAreaRatio"`
	MaxWindowAreaRatio  float64 `json:"maxWindowAreaRatio"`
	PadPx               float64 `json:"padPx"`
	RadiusPx            float64 `json:"radiusPx"`
	StepPx              float64 `json:"stepPx"`
	Overrun             float64 `json:"overrun"`
	Wobble              float64 `json:"wobble"`
	WobbleRate          float64 `json:"wobbleRate"`
	WobbleSeed          int     `json:"wobbleSeed"`
	PenScale            float64 `json:"penScale"`
	AheadPx             float64 `json:"aheadPx"`
	OutsidePx           float64 `json:"outsidePx"`
	SpeedPxS            float64 `json:"speedPxS"`
	ClampS              Range   `json:"clampS"`
	CornerSpeed         float64 `json:"cornerSpeed"`
	CornerSlowPx        float64 `json:"cornerSlowPx"`
	SlowEnds            float64 `json:"slowEnds"`
	LeadPoints          int     `json:"leadPoints"`
	ReelS               float64 `json:"reelS"`
}

type Surface struct {
	OffsetXPx     float64 `json:"offsetXPx"`
	OffsetYPx     float64 `json:"offsetYPx"`
	AnchorInsetPx float64 `json:"anchorInsetPx"`
	MarginPx      float64 `json:"marginPx"`
}

// Burst angle is AnglePi·π radians.
type Burst struct {
	Lines     int     `json:"lines"`
	AnglePi   float64 `json:"anglePi" oni:"signed"`
	Step      float64 `json:"step"`
	FromPx    float64 `json:"fromPx"`
	GrowPx    float64 `json:"growPx"`
	DurationS float64 `json:"durationS"`
	WidthPx   float64 `json:"widthPx"`
}

type Intro struct {
	TraceS          float64 `json:"traceS"`
	OutlineRadiusPx float64 `json:"outlineRadiusPx"`
	OutlineArc      float64 `json:"outlineArc"`
	FadeS           float64 `json:"fadeS"`
}

type Bubble struct {
	Open             Transition `json:"open"`
	Close            Transition `json:"close"`
	TextInS          float64    `json:"textInS"`
	TextRisePx       float64    `json:"textRisePx"`
	TextOutS         float64    `json:"textOutS"`
	MaxWidthPx       float64    `json:"maxWidthPx"`
	BorderPx         float64    `json:"borderPx"`
	RingPx           float64    `json:"ringPx"`
	KickerPx         float64    `json:"kickerPx"`
	KickerTrackingEm float64    `json:"kickerTrackingEm"`
	TextPx           float64    `json:"textPx"`
	HintPx           float64    `json:"hintPx"`
	SolveGutterPx    float64    `json:"solveGutterPx"`
}

// Transition scales from (open) or to (close) Scale over DurationS.
type Transition struct {
	DurationS float64 `json:"durationS"`
	Ease      Ease    `json:"ease"`
	Scale     float64 `json:"scale"`
}

type Flight struct {
	Bow      float64 `json:"bow"`
	BowMaxPx float64 `json:"bowMaxPx"`
	SpeedPxS float64 `json:"speedPxS"`
	ClampS   Range   `json:"clampS"`
	Ease     Ease    `json:"ease"`
}

// PerchRules is the character's side of a perch: where it sits beside the
// window corner the thread curls on (Seat), how it follows a window that
// moves (Reattach), the outline drawn round the window under a drag
// (Highlight) and the chip naming the perched window (Chip).
type PerchRules struct {
	Seat      PerchSeat      `json:"seat"`
	Reattach  PerchReattach  `json:"reattach"`
	Highlight PerchHighlight `json:"highlight"`
	Chip      PerchChip      `json:"chip"`
}

// PerchSeat places the character frame outside the perch corner: its inner
// edge GapPx beyond the window's side, its bottom LiftPx above the top edge.
// An axis that would cross MarginPx of the output flips to the inside of the
// window; the frame is then clamped MarginPx inside the output.
type PerchSeat struct {
	GapPx    float64 `json:"gapPx"`
	LiftPx   float64 `json:"liftPx"`
	MarginPx float64 `json:"marginPx"`
}

// PerchReattach: a seat that moves at most SnapPx jumps with its window and
// keeps its curl; a longer move waits until the window has held still for
// SettleMs, then the character flies and the thread curls again.
type PerchReattach struct {
	SnapPx   float64 `json:"snapPx"`
	SettleMs float64 `json:"settleMs"`
}

// PerchHighlight is the rounded outline PadPx outside a window under a
// drag: a white line WidthPx wide on an ink rim RimPx each side.
type PerchHighlight struct {
	WidthPx  float64 `json:"widthPx"`
	RimPx    float64 `json:"rimPx"`
	PadPx    float64 `json:"padPx"`
	RadiusPx float64 `json:"radiusPx"`
	FadeInS  float64 `json:"fadeInS"`
	FadeOutS float64 `json:"fadeOutS"`
}

// PerchChip is the label naming the perched window, centred GapPx under
// the character frame (above it when below would cross the seat margin).
type PerchChip struct {
	TextPx     float64 `json:"textPx"`
	PadXPx     float64 `json:"padXPx"`
	PadYPx     float64 `json:"padYPx"`
	GapPx      float64 `json:"gapPx"`
	MaxWidthPx float64 `json:"maxWidthPx"`
	RadiusPx   float64 `json:"radiusPx"`
	BorderPx   float64 `json:"borderPx"`
	FadeS      float64 `json:"fadeS"`
}

type Session struct {
	ArrivalGatesAdvance bool    `json:"arrivalGatesAdvance"`
	StaleSeqIgnored     bool    `json:"staleSeqIgnored"`
	SkipCover           Vec2    `json:"skipCover"`
	SkateClearGapPx     float64 `json:"skateClearGapPx"`
	PlanDoneMax         int     `json:"planDoneMax"`
	PlanNextMax         int     `json:"planNextMax"`
	ArrivalTimeoutMs    float64 `json:"arrivalTimeoutMs"`
}

// Sound.Scale lists the pitch classes (0 = C) every motif and cue note uses;
// Events maps each EventIDs moment to the motif it plays.
type Sound struct {
	Synth  Synth             `json:"synth"`
	Scale  []int             `json:"scale"`
	Motifs map[string]Motif  `json:"motifs"`
	Events map[string]string `json:"events"`
	Cues   Cues              `json:"cues"`
	Gating Gating            `json:"gating"`
}

type Synth struct {
	Harmonic  float64 `json:"harmonic"`
	LowpassHz float64 `json:"lowpassHz"`
	AttackS   float64 `json:"attackS"`
	DecayS    float64 `json:"decayS"`
	Sustain   float64 `json:"sustain"`
	ReleaseS  float64 `json:"releaseS"`
	Master    float64 `json:"master"`
	Peak      float64 `json:"peak"`
	DebounceS float64 `json:"debounceS"`
}

// Motif notes are MIDI numbers.
type Motif struct {
	Notes    []int   `json:"notes"`
	SpacingS float64 `json:"spacingS"`
	LengthS  float64 `json:"lengthS"`
	DelayS   float64 `json:"delayS"`
}

type Cues struct {
	Partials      []float64      `json:"partials"`
	Amps          []float64      `json:"amps"`
	PitchBendMs   float64        `json:"pitchBendMs"`
	SampleRateHz  float64        `json:"sampleRateHz"`
	Gain          float64        `json:"gain"`
	HandsDoneMinS float64        `json:"handsDoneMinS"`
	Items         map[string]Cue `json:"items"`
}

type Cue struct {
	Notes  []int   `json:"notes"`
	GainDb float64 `json:"gainDb" oni:"signed"`
}

type Gating struct {
	QuietMs            float64 `json:"quietMs"`
	MuteRampMs         float64 `json:"muteRampMs"`
	AckHoldMs          float64 `json:"ackHoldMs"`
	SpeechSampleRateHz float64 `json:"speechSampleRateHz"`
}

// GestureSpec is a gesture resolved with its keyframes and sound motif.
type GestureSpec struct {
	DurationS float64
	Keyframes Keyframes
	Motif     Motif
}

func (s *OniSpec) State(id string) (State, bool) {
	st, ok := s.States[id]
	return st, ok
}

func (s *OniSpec) Gesture(id string) (GestureSpec, bool) {
	d, ok := s.Gestures.DurationS[id]
	if !ok {
		return GestureSpec{}, false
	}
	k, ok := s.Gestures.Keyframes[id]
	if !ok {
		return GestureSpec{}, false
	}
	m, ok := s.Sound.Motifs[id]
	if !ok {
		return GestureSpec{}, false
	}
	return GestureSpec{DurationS: d, Keyframes: k, Motif: m}, true
}

// Size returns the scale factor for a placement size id (see SizeIDs).
func (s *OniSpec) Size(id string) (float64, bool) {
	f, ok := s.Placement.Sizes[id]
	return f, ok
}

func (s *OniSpec) Motif(id string) (Motif, bool) {
	m, ok := s.Sound.Motifs[id]
	return m, ok
}

// slowestLandingMs is the longest a healthy thread takes to land: the
// pre-reach wait, a region pen at its slowest, the flight on to the control,
// the lasso draw and cinch, then the tap. session.arrivalTimeoutMs must
// exceed it so the backstop never cuts off a landing still under way.
func (s *OniSpec) slowestLandingMs() float64 {
	st := s.Strand
	return st.PreReachDelayMs + 1000*(st.Region.ClampS[1]+s.Flight.ClampS[1]+st.Lasso.DrawS+st.Lasso.CinchS+st.Tap.DurationS)
}

// CurlReachPx is how far the perch curl's ink reaches from the window
// corner it wraps: strand.perch.radiusPx plus half the inked stroke
// (strand.stroke.widthPx/2 + outlinePx). perch.seat.gapPx must exceed it.
func (s *OniSpec) CurlReachPx() float64 {
	st := s.Strand
	return st.Perch.RadiusPx + st.Stroke.WidthPx/2 + st.Stroke.OutlinePx
}

// StateColour returns the palette hex a state wears.
func (s *OniSpec) StateColour(id string) (string, bool) {
	p, ok := s.Colour.StateColour[id]
	if !ok {
		return "", false
	}
	hex, ok := s.Colour.Palette[p]
	return hex, ok
}

// TargetFPS returns the frame rate for an activity (see ActivityIDs); battery
// saver drops breathing to the rest tier while motion keeps the full rate.
func (s *OniSpec) TargetFPS(activity string, batterySaver bool) (int, bool) {
	tiers := s.Frames.Tier
	if batterySaver {
		tiers = s.Frames.BatteryTier
	}
	tier, ok := tiers[activity]
	if !ok {
		return 0, false
	}
	fps, ok := s.Frames.TiersFps[tier]
	return fps, ok
}

// Load strictly parses the embedded oni-spec.json.
func Load() (*OniSpec, error) { return Parse(embedded) }

// Parse rejects syntax errors, duplicate or unknown keys, missing fields,
// wrong types or array lengths, non-finite numbers, negative values outside
// fields tagged signed, and semantic violations (see Validate).
func Parse(data []byte) (*OniSpec, error) {
	if err := scanKeys(data); err != nil {
		return nil, fmt.Errorf("oni spec: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("oni spec: %w", err)
	}
	if err := checkShape(reflect.TypeFor[OniSpec](), raw, "", false); err != nil {
		return nil, fmt.Errorf("oni spec: %w", err)
	}
	var s OniSpec
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("oni spec: %w", err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func scanKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var walk func(path string) error
	walk = func(path string) error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		d, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				k, _ := kt.(string)
				p := join(path, k)
				if seen[k] {
					return fmt.Errorf("%s: duplicate key", p)
				}
				seen[k] = true
				if err := walk(p); err != nil {
					return err
				}
			}
		case '[':
			for i := 0; dec.More(); i++ {
				if err := walk(fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
		_, err = dec.Token()
		return err
	}
	if err := walk(""); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing data after the spec object")
	}
	return nil
}

func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name
}

func checkShape(t reflect.Type, v any, path string, signed bool) error {
	where := path
	if where == "" {
		where = "(root)"
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: want an object", where)
		}
		known := map[string]bool{}
		for i := range t.NumField() {
			f := t.Field(i)
			name := jsonName(f)
			known[name] = true
			fv, ok := obj[name]
			if !ok {
				return fmt.Errorf("%s: missing key", join(path, name))
			}
			if err := checkShape(f.Type, fv, join(path, name), signed || f.Tag.Get("oni") == "signed"); err != nil {
				return err
			}
		}
		for _, k := range sortedKeys(obj) {
			if !known[k] {
				return fmt.Errorf("%s: unknown key", join(path, k))
			}
		}
	case reflect.Map:
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: want an object", where)
		}
		for _, k := range sortedKeys(obj) {
			if err := checkShape(t.Elem(), obj[k], join(path, k), signed); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		arr, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s: want an array", where)
		}
		if t.Kind() == reflect.Array && len(arr) != t.Len() {
			return fmt.Errorf("%s: want %d elements, got %d", where, t.Len(), len(arr))
		}
		for i, e := range arr {
			if err := checkShape(t.Elem(), e, fmt.Sprintf("%s[%d]", path, i), signed); err != nil {
				return err
			}
		}
	case reflect.Float64, reflect.Int:
		n, ok := v.(json.Number)
		if !ok {
			return fmt.Errorf("%s: want a number", where)
		}
		f, err := n.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("%s: number %s is not finite", where, n)
		}
		if t.Kind() == reflect.Int {
			if _, err := n.Int64(); err != nil {
				return fmt.Errorf("%s: want an integer, got %s", where, n)
			}
		}
		if f < 0 && !signed {
			return fmt.Errorf("%s: negative value %s", where, n)
		}
	case reflect.String:
		if _, ok := v.(string); !ok {
			return fmt.Errorf("%s: want a string", where)
		}
	case reflect.Bool:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s: want a boolean", where)
		}
	default:
		return fmt.Errorf("%s: unsupported schema kind %s", where, t.Kind())
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func sameSet[V any](path string, m map[string]V, want []string) error {
	var missing, extra []string
	for _, id := range want {
		if _, ok := m[id]; !ok {
			missing = append(missing, id)
		}
	}
	for _, k := range sortedKeys(m) {
		if !slices.Contains(want, k) {
			extra = append(extra, k)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		return fmt.Errorf("%s: missing %v, unknown %v", path, missing, extra)
	}
	return nil
}

var hexColour = regexp.MustCompile(`^#[0-9A-F]{6}$`)

var (
	springType = reflect.TypeFor[Spring]()
	rangeType  = reflect.TypeFor[Range]()
	easeType   = reflect.TypeFor[Ease]()
)

func walkValues(v reflect.Value, path string, fn func(string, reflect.Value)) {
	fn(path, v)
	switch v.Kind() {
	case reflect.Struct:
		for i := range v.NumField() {
			walkValues(v.Field(i), join(path, jsonName(v.Type().Field(i))), fn)
		}
	case reflect.Map:
		keys := make([]string, 0, v.Len())
		for _, k := range v.MapKeys() {
			keys = append(keys, k.String())
		}
		slices.Sort(keys)
		for _, k := range keys {
			walkValues(v.MapIndex(reflect.ValueOf(k)), join(path, k), fn)
		}
	}
}

// Validate checks the relations a renderer relies on beyond the schema.
func (s *OniSpec) Validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	check := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if s.Version != Version {
		bad("version: unsupported %d (want %d)", s.Version, Version)
	}
	check(sameSet("states", s.States, StateIDs))
	check(sameSet("colour.palette", s.Colour.Palette, PaletteIDs))
	check(sameSet("colour.stateColour", s.Colour.StateColour, StateIDs))
	check(sameSet("gestures.durationS", s.Gestures.DurationS, GestureIDs))
	check(sameSet("gestures.keyframes", s.Gestures.Keyframes, GestureIDs))
	check(sameSet("placement.sizes", s.Placement.Sizes, SizeIDs))
	check(sameSet("sound.motifs", s.Sound.Motifs, MotifIDs))
	check(sameSet("sound.cues.items", s.Sound.Cues.Items, CueIDs))
	check(sameSet("frames.tiersFps", s.Frames.TiersFps, TierIDs))
	check(sameSet("frames.tier", s.Frames.Tier, ActivityIDs))
	check(sameSet("frames.batteryTier", s.Frames.BatteryTier, ActivityIDs))

	walkValues(reflect.ValueOf(*s), "", func(path string, v reflect.Value) {
		switch v.Type() {
		case springType:
			sp := v.Interface().(Spring)
			if !(sp.Stiffness() > 0 && sp.Damping() > 0) {
				bad("%s: spring needs positive stiffness and damping", path)
			}
		case rangeType:
			r := v.Interface().(Range)
			if r[0] > r[1] {
				bad("%s: range min exceeds max", path)
			}
		case easeType:
			e := v.Interface().(Ease)
			if e[0] < 0 || e[0] > 1 || e[2] < 0 || e[2] > 1 {
				bad("%s: ease x controls must lie in [0,1]", path)
			}
		}
	})
	for _, c := range []struct{ path, v string }{
		{"character.ink", s.Character.Ink}, {"character.white", s.Character.White},
		{"colour.focusGuard.active", s.Colour.FocusGuard.Active}, {"colour.focusGuard.done", s.Colour.FocusGuard.Done},
	} {
		if !hexColour.MatchString(c.v) {
			bad("%s: colour %q is not #RRGGBB", c.path, c.v)
		}
	}
	for _, id := range sortedKeys(s.Colour.Palette) {
		if !hexColour.MatchString(s.Colour.Palette[id]) {
			bad("colour.palette.%s: colour is not #RRGGBB", id)
		}
	}
	for _, id := range sortedKeys(s.Strand.Accents) {
		if !hexColour.MatchString(s.Strand.Accents[id]) {
			bad("strand.accents.%s: colour is not #RRGGBB", id)
		}
	}
	for _, id := range sortedKeys(s.Colour.StateColour) {
		if _, ok := s.Colour.Palette[s.Colour.StateColour[id]]; !ok {
			bad("colour.stateColour.%s: unknown palette entry %q", id, s.Colour.StateColour[id])
		}
	}
	for _, b := range []struct {
		name string
		beat Beat
	}{{"done", s.Colour.Beats.Done}, {"error", s.Colour.Beats.Error}} {
		if _, ok := s.Colour.Palette[b.beat.Colour]; !ok {
			bad("colour.beats.%s.colour: unknown palette entry %q", b.name, b.beat.Colour)
		}
	}

	pr := s.Springs.PoseRanges
	for _, id := range sortedKeys(s.States) {
		st := s.States[id]
		p := "states." + id
		for _, w := range []float64{st.Flat, st.Face, st.Loose} {
			if w < 0 || w > 1 {
				bad("%s: shape weight outside [0,1]", p)
			}
		}
		if math.Abs(st.Flat+st.Face+st.Loose-1) > 1e-9 {
			bad("%s: shape weights must sum to 1", p)
		}
		if st.Scale < pr.Scale[0] || st.Scale > pr.Scale[1] {
			bad("%s.scale: outside springs.poseRanges.scale", p)
		}
		if st.Tempo < pr.Tempo[0] || st.Tempo > pr.Tempo[1] {
			bad("%s.tempo: outside springs.poseRanges.tempo", p)
		}
		if !(st.Hover[0] > 0 && st.Hover[1] > 0) {
			bad("%s.hover: amplitude and rate must be positive", p)
		}
	}

	for _, id := range sortedKeys(s.Gestures.DurationS) {
		if !(s.Gestures.DurationS[id] > 0) {
			bad("gestures.durationS.%s: must be positive", id)
		}
	}
	for _, id := range sortedKeys(s.Gestures.Keyframes) {
		checkKeyframes(&errs, "gestures.keyframes."+id, s.Gestures.Keyframes[id])
	}
	if shake := s.Gestures.Keyframes["error"]["leanX"]; len(shake) != len(s.Gestures.ErrorShakeLeanX)+2 || (len(shake) > 2 && !slices.Equal(shake[1:len(shake)-1], s.Gestures.ErrorShakeLeanX)) {
		bad("gestures.keyframes.error.leanX: interior keys must equal gestures.errorShakeLeanX")
	}
	sz := s.Placement.Sizes
	if !(sz["small"] > 0 && sz["small"] < sz["normal"] && sz["normal"] < sz["big"]) {
		bad("placement.sizes: want 0 < small < normal < big")
	}
	if s.Placement.DragThresholdPx < 1 {
		bad("placement.dragThresholdPx: must be at least 1")
	}
	for _, id := range sortedKeys(s.Sound.Motifs) {
		m := s.Sound.Motifs[id]
		checkNotes(&errs, "sound.motifs."+id+".notes", m.Notes)
		if !(m.LengthS > 0) {
			bad("sound.motifs.%s.lengthS: must be positive", id)
		}
	}
	for _, id := range sortedKeys(s.Sound.Cues.Items) {
		checkNotes(&errs, "sound.cues.items."+id+".notes", s.Sound.Cues.Items[id].Notes)
	}
	if len(s.Sound.Cues.Partials) == 0 || len(s.Sound.Cues.Partials) != len(s.Sound.Cues.Amps) {
		bad("sound.cues: partials and amps must be non-empty and the same length")
	}
	if len(s.Sound.Scale) == 0 {
		bad("sound.scale: empty")
	}
	inScale := map[int]bool{}
	for i, pc := range s.Sound.Scale {
		if pc > 11 {
			bad("sound.scale[%d]: pitch class %d outside 0-11", i, pc)
		}
		if inScale[pc] {
			bad("sound.scale[%d]: pitch class %d listed twice", i, pc)
		}
		inScale[pc] = true
	}
	offScale := func(path string, notes []int) {
		for _, n := range notes {
			if n >= 0 && !inScale[n%12] {
				bad("%s: MIDI note %d is outside sound.scale", path, n)
			}
		}
	}
	for _, id := range sortedKeys(s.Sound.Motifs) {
		offScale("sound.motifs."+id+".notes", s.Sound.Motifs[id].Notes)
	}
	for _, id := range sortedKeys(s.Sound.Cues.Items) {
		offScale("sound.cues.items."+id+".notes", s.Sound.Cues.Items[id].Notes)
	}
	check(sameSet("sound.events", s.Sound.Events, EventIDs))
	played := map[string]bool{}
	for _, id := range sortedKeys(s.Sound.Events) {
		m := s.Sound.Events[id]
		if !slices.Contains(MotifIDs, m) {
			bad("sound.events.%s: unknown motif %q", id, m)
		}
		played[m] = true
	}
	for _, id := range MotifIDs {
		if _, ok := s.Sound.Motifs[id]; ok && !played[id] {
			bad("sound.motifs.%s: no sound.events entry plays it", id)
		}
	}

	fps := s.Frames.TiersFps
	if !(fps["asleep"] > 0 && fps["asleep"] <= fps["rest"] && fps["rest"] <= fps["active"]) {
		bad("frames.tiersFps: want 0 < asleep <= rest <= active")
	}
	for _, m := range []struct {
		path string
		t    map[string]string
	}{{"frames.tier", s.Frames.Tier}, {"frames.batteryTier", s.Frames.BatteryTier}} {
		for _, a := range sortedKeys(m.t) {
			if !slices.Contains(TierIDs, m.t[a]) {
				bad("%s.%s: unknown tier %q", m.path, a, m.t[a])
			}
		}
	}
	for _, a := range ActivityIDs {
		if fps[s.Frames.BatteryTier[a]] > fps[s.Frames.Tier[a]] {
			bad("frames.batteryTier.%s: battery saver may not raise the frame rate", a)
		}
	}
	if !(s.Frames.DtCapMs > 0) {
		bad("frames.dtCapMs: must be positive")
	}
	if !(s.Integrator.Mass > 0 && s.Integrator.SnapX > 0 && s.Integrator.SnapV > 0) {
		bad("integrator: mass and snap thresholds must be positive")
	}

	bp := s.Strand.BowedPath
	if bp.Segments < 1 || bp.Points != bp.Segments+1 {
		bad("strand.bowedPath: points must equal segments+1")
	}
	if !(bp.Controls[0] > 0 && bp.Controls[0] < bp.Controls[1] && bp.Controls[1] < 1) {
		bad("strand.bowedPath.controls: want 0 < first < second < 1")
	}
	if bp.SeamBlendPoints >= bp.Points {
		bad("strand.bowedPath.seamBlendPoints: must be fewer than points")
	}
	for _, d := range []struct {
		path string
		v    float64
	}{
		{"strand.reach.durationS", s.Strand.Reach.DurationS},
		{"strand.rewind.durationS", s.Strand.Rewind.DurationS},
		{"strand.tap.durationS", s.Strand.Tap.DurationS},
		{"strand.reach.surfaceSpeedPxS", s.Strand.Reach.SurfaceSpeedPxS},
		{"strand.lasso.power", s.Strand.Lasso.Power},
		{"flight.speedPxS", s.Flight.SpeedPxS},
		{"perch.seat.gapPx", s.Perch.Seat.GapPx},
		{"perch.reattach.snapPx", s.Perch.Reattach.SnapPx},
		{"perch.reattach.settleMs", s.Perch.Reattach.SettleMs},
		{"perch.highlight.widthPx", s.Perch.Highlight.WidthPx},
		{"perch.chip.textPx", s.Perch.Chip.TextPx},
	} {
		if !(d.v > 0) {
			bad("%s: must be positive", d.path)
		}
	}
	if s.Strand.Lasso.Points < 3 {
		bad("strand.lasso.points: need at least 3")
	}
	an := s.Strand.Anchor
	if an.X > 1 || an.Y > 1 {
		bad("strand.anchor: x and y must lie in [0,1]")
	}
	for _, w := range []struct {
		name string
		v    float64
	}{{"scale", an.Follow.Scale}, {"squash", an.Follow.Squash}, {"lean", an.Follow.Lean}, {"lift", an.Follow.Lift}, {"hover", an.Follow.Hover}} {
		if w.v > 1 {
			bad("strand.anchor.follow.%s: weight must lie in [0,1]", w.name)
		}
	}
	if rf := s.Strand.Retarget.ReachFrom; !(rf > 0 && rf < 1) {
		bad("strand.retarget.reachFrom: want 0 < reachFrom < 1")
	}
	if !(s.Strand.RetargetBlendMs > 0) {
		bad("strand.retargetBlendMs: must be positive")
	}
	if int64(s.Strand.Region.WobbleSeed) > math.MaxUint32 {
		bad("strand.region.wobbleSeed: must fit in 32 bits")
	}
	if slowest := s.slowestLandingMs(); !(s.Session.ArrivalTimeoutMs > slowest) {
		bad("session.arrivalTimeoutMs: must exceed the slowest landing (%g ms)", slowest)
	}
	// The curl reaches strand.perch.radiusPx from the corner plus half the
	// inked stroke; a seat any closer would sit on its own thread.
	if clear := s.CurlReachPx(); !(s.Perch.Seat.GapPx > clear) {
		bad("perch.seat.gapPx: must clear the perch curl (more than %g px)", clear)
	}
	if hl := s.Perch.Highlight; hl.PadPx < hl.WidthPx/2+hl.RimPx {
		bad("perch.highlight.padPx: the outline must stay outside the window (at least %g px)", hl.WidthPx/2+hl.RimPx)
	}
	if ch := s.Perch.Chip; !(ch.MaxWidthPx > 2*ch.PadXPx+ch.TextPx) {
		bad("perch.chip.maxWidthPx: leaves no room for text inside the padding")
	}

	ld := s.LightDesktop
	if !(0 <= ld.OffBelow && ld.OffBelow < ld.OnAbove && ld.OnAbove <= 1) {
		bad("lightDesktop: want 0 <= offBelow < onAbove <= 1")
	}
	if ld.Treatment == "" || slices.Contains(ld.Alternatives, ld.Treatment) {
		bad("lightDesktop.treatment: must be set and not listed as an alternative")
	}
	if s.ReducedMotion.Loop || s.ReducedMotion.FramesPerEvent < 1 {
		bad("reducedMotion: must not loop and must paint at least one frame per event")
	}
	s.checkVoice(bad)
	s.checkGuide(bad)
	s.checkReflex(bad)
	s.checkHands(bad)
	s.checkPhase16(bad)

	if len(errs) > 0 {
		return fmt.Errorf("oni spec: %w", errors.Join(errs...))
	}
	return nil
}

func checkKeyframes(errs *[]error, path string, k Keyframes) {
	bad := func(format string, args ...any) { *errs = append(*errs, fmt.Errorf(path+format, args...)) }
	at := k["at"]
	if len(at) < 2 || at[0] != 0 || at[len(at)-1] != 1 {
		bad(".at: want at least two keys from 0 to 1")
	}
	for i := 1; i < len(at); i++ {
		if !(at[i] > at[i-1]) {
			bad(".at: keys must strictly increase")
			break
		}
	}
	if len(k) < 2 {
		bad(": no channels")
	}
	for _, ch := range sortedKeys(k) {
		if ch == "at" {
			continue
		}
		if !slices.Contains(KeyframeChannels, ch) {
			bad(".%s: unknown channel", ch)
			continue
		}
		v := k[ch]
		if len(v) != len(at) {
			bad(".%s: want %d keys, got %d", ch, len(at), len(v))
			continue
		}
		if v[0] != 0 || v[len(v)-1] != 0 {
			bad(".%s: must start and end at 0", ch)
		}
	}
}

func checkNotes(errs *[]error, path string, notes []int) {
	if len(notes) == 0 {
		*errs = append(*errs, fmt.Errorf("%s: empty", path))
	}
	for _, n := range notes {
		if n < 0 || n > 127 {
			*errs = append(*errs, fmt.Errorf("%s: MIDI note %d out of range", path, n))
		}
	}
}

var builtin = sync.OnceValue(func() *OniSpec {
	s, err := Load()
	if err != nil {
		panic(err)
	}
	return s
})

// BezierEase evaluates CSS cubic-bezier(x1,y1,x2,y2) at progress t,
// clamping t to [0,1]; NaN progress counts as not started.
func BezierEase(x1, y1, x2, y2, t float64) float64 {
	if !(t > 0) {
		return 0
	}
	if t >= 1 {
		return 1
	}
	x1 = math.Min(1, math.Max(0, x1))
	x2 = math.Min(1, math.Max(0, x2))
	bez := func(p1, p2, s float64) float64 {
		u := 1 - s
		return 3*u*u*s*p1 + 3*u*s*s*p2 + s*s*s
	}
	dbez := func(p1, p2, s float64) float64 {
		u := 1 - s
		return 3*u*u*p1 + 6*u*s*(p2-p1) + 3*s*s*(1-p2)
	}
	const eps = 1e-12
	s := t
	for range 8 {
		e := bez(x1, x2, s) - t
		if math.Abs(e) < eps {
			return bez(y1, y2, s)
		}
		d := dbez(x1, x2, s)
		if math.Abs(d) < 1e-9 {
			break
		}
		s -= e / d
		if s < 0 || s > 1 {
			break
		}
	}
	lo, hi := 0.0, 1.0
	s = t
	for range 100 {
		x := bez(x1, x2, s)
		if math.Abs(x-t) < eps {
			break
		}
		if x < t {
			lo = s
		} else {
			hi = s
		}
		s = (lo + hi) / 2
	}
	return bez(y1, y2, s)
}

// Reach is the strand's extension u(t) from home for t seconds into the
// guide reach; it reaches 1 at the reach duration.
func Reach(t float64) float64 {
	r := builtin().Strand.Reach
	return r.Ease.At(t / r.DurationS)
}

// Rewind is the strand's remaining extension u(t) for t seconds into a
// full rewind; it reaches 0 at the rewind duration.
func Rewind(t float64) float64 {
	r := builtin().Strand.Rewind
	return 1 - r.Ease.At(t/r.DurationS)
}

// SurfaceReachS is the length-based reach duration for a surface thread.
func SurfaceReachS(totalPx float64) float64 {
	r := builtin().Strand.Reach
	return math.Min(r.SurfaceClampS[1], math.Max(r.SurfaceClampS[0], totalPx/r.SurfaceSpeedPxS))
}

// BowedPath samples the strand's cubic from a to b, both controls pushed
// from the chord toward centre; it returns the spec's point count, uniform
// in t, with the exact endpoints.
func BowedPath(a, b, centre Point) []Point {
	bp := builtin().Strand.BowedPath
	dx, dy := b.X-a.X, b.Y-a.Y
	bow := math.Min(bp.BowMaxPx, math.Hypot(dx, dy)*bp.BowPerDistance) * bp.BowScale
	ox, oy := centre.X-(a.X+dx/2), centre.Y-(a.Y+dy/2)
	if n := math.Hypot(ox, oy); n > 0 {
		ox, oy = ox/n*bow, oy/n*bow
	} else {
		ox, oy = 0, 0
	}
	c1 := Point{a.X + dx*bp.Controls[0] + ox, a.Y + dy*bp.Controls[0] + oy}
	c2 := Point{a.X + dx*bp.Controls[1] + ox, a.Y + dy*bp.Controls[1] + oy}
	pts := make([]Point, bp.Points)
	for i := range pts {
		t := float64(i) / float64(bp.Segments)
		u := 1 - t
		w0, w1, w2, w3 := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		pts[i] = Point{
			X: w0*a.X + w1*c1.X + w2*c2.X + w3*b.X,
			Y: w0*a.Y + w1*c1.Y + w2*c2.Y + w3*b.Y,
		}
	}
	pts[0], pts[len(pts)-1] = a, b
	return pts
}

// SpringStep advances one spring by dt seconds with semi-implicit Euler,
// capping dt at the spec's frame cap and snapping to target once both
// offset and velocity fall under the snap thresholds.
func SpringStep(x, v, target float64, s Spring, dt float64) (float64, float64) {
	sp := builtin()
	if !(dt > 0) {
		return x, v
	}
	// A non-finite state would poison every later frame; land instead.
	if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(v) || math.IsInf(v, 0) {
		return target, 0
	}
	dt = math.Min(dt, sp.Frames.DtCapMs/1000)
	a := (s.Stiffness()*(target-x) - s.Damping()*v) / sp.Integrator.Mass
	v += a * dt
	x += v * dt
	if math.Abs(target-x) < sp.Integrator.SnapX && math.Abs(v) < sp.Integrator.SnapV {
		return target, 0
	}
	return x, v
}
