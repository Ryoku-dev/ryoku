package feature

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"strings"
	"time"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// Frame kinds on the `oni` event and in the Oni calls.
const (
	KindState         = "state"
	KindCharacter     = "character"
	KindPerch         = "perch"
	KindStrandTarget  = "strand.target"
	KindStrandArrive  = "strand.arrive"
	KindStrandRetract = "strand.retract"
	KindBubble        = "bubble"
	KindNotice        = "notice"
	KindConsent       = "consent"
	KindConsentAnswer = "consent.answer"
	KindVoice         = "voice"
	KindHands         = "hands"
	KindGesture       = "gesture"
	KindPlacement     = "placement"
	KindMotion        = "motion"
	KindLevel         = "level"
	KindGuide         = "guide"
	KindPick          = "pick"
)

var kinds = []string{
	KindState, KindCharacter, KindPerch, KindStrandTarget, KindStrandArrive,
	KindStrandRetract, KindBubble, KindNotice, KindConsent, KindConsentAnswer, KindVoice,
	KindHands, KindGesture, KindPlacement, KindMotion, KindLevel, KindGuide,
	KindPick, KindApproval, KindFocus,
}

// Voice phases, spec.VoicePhaseIDs: the microphone closed, open between
// presses for the wake phrase or open mic, open for a press, the words heard
// being turned into text, and Oni speaking. Only idle is quiet.
const (
	VoiceIdle         = "idle"
	VoiceArmed        = "armed"
	VoiceListening    = "listening"
	VoiceTranscribing = "transcribing"
	VoiceSpeaking     = "speaking"
)

// What opened the microphone: the held chord, a toggle press (hyakki oni
// talk, a compositor bind), the wake phrase, open mic, or a dictation.
const (
	VoiceSourceChord     = "chord"
	VoiceSourceToggle    = "toggle"
	VoiceSourceWake      = "wake"
	VoiceSourceOpen      = "open"
	VoiceSourceDictation = "dictation"
)

// VoiceSources are every voice source.
var VoiceSources = []string{VoiceSourceChord, VoiceSourceToggle, VoiceSourceWake, VoiceSourceOpen, VoiceSourceDictation}

// Kinds returns every frame kind.
func Kinds() []string { return slices.Clone(kinds) }

// Strand modes: point at the target, throw a ring around it, trace a region
// with the pen, or curl on the perched window's own frame. An empty mode
// points.
const (
	StrandModePoint  = "point"
	StrandModeLasso  = "lasso"
	StrandModeRegion = "region"
	// StrandModePerch targets the perched window's own frame: valid only
	// with an attached perch, at x=y=0, w=h=1.
	StrandModePerch = "perch"
)

// StrandModes are every strand mode.
var StrandModes = []string{StrandModePoint, StrandModeLasso, StrandModeRegion, StrandModePerch}

// Why a renderer could not draw a strand target. It reports the target with
// one of these instead of landing it.
const (
	SkipCoversScreen = "covers-screen"
	SkipOffOutput    = "off-output"
	SkipCovered      = "covered"
	SkipHidden       = "hidden"
)

// SkipReasons are every reason a strand target is skipped.
var SkipReasons = []string{SkipCoversScreen, SkipOffOutput, SkipCovered, SkipHidden}

// Why a perch was refused or ended: a closed set the ledger and the tests
// name.
const (
	// PerchClosed: the window is gone, or there is none where Oni was
	// dropped.
	PerchClosed = "closed"
	// PerchUser: the user let go through the chip, the app or the CLI.
	PerchUser = "user"
	// PerchDraggedOff: the user dragged Oni off the window.
	PerchDraggedOff = "dragged-off"
	// PerchOutputGone: the window's output went away.
	PerchOutputGone = "output-gone"
	// PerchHidden: the window, or Oni, went off screen.
	PerchHidden = "hidden"
	// PerchNoGeometry: the desktop does not say where the window is.
	PerchNoGeometry = "no-geometry"
	// PerchOniOff: Oni was turned off.
	PerchOniOff = "oni-off"
	// PerchPresenterLost: nothing draws Oni any more.
	PerchPresenterLost = "presenter-lost"
)

// PerchReasons are every perch and detach reason.
var PerchReasons = []string{PerchClosed, PerchUser, PerchDraggedOff, PerchOutputGone, PerchHidden, PerchNoGeometry, PerchOniOff, PerchPresenterLost}

// Oni calls a later phase will serve, none of them registered yet; every
// call is served now. Settings change through settings.set, so there is no
// Oni settings call.
var reservedCalls = []string{}

// ReservedCalls returns the method names of the Oni calls not yet served.
func ReservedCalls() []string { return slices.Clone(reservedCalls) }

// MaxOverlayRects bounds the presenter's own rects in one oni.overlay.
const MaxOverlayRects = 16

// Limits on a placement frame and a box.
const (
	MaxPlacementOutputs = 16
	maxOutputID         = 64
	maxBoxPx            = 1 << 16
)

// Frame is one Oni frame on the wire: a kind and its payload.
type Frame struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

// Payload is a frame's data. DecodeFrame returns a pointer to one of the
// *Frame types in this file.
type Payload interface {
	Kind() string
	validate() error
}

// StateFrame is the character's state, one of spec.StateIDs. Reason is a
// short code such as oni-off when Oni is hidden.
type StateFrame struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// CharacterFrame names the character in use and the slots that fell back.
type CharacterFrame struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Fallbacks []protocol.OniFallback `json:"fallbacks,omitempty"`
}

// Box is a rectangle in output-local logical pixels: X, Y its top-left
// corner, W, H its size.
type Box struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// PerchFrame says which window Oni is perched on, or that it is not. Box is
// the perched window's frame, present exactly when Attached.
type PerchFrame struct {
	Seq      uint64 `json:"seq"`
	Attached bool   `json:"attached"`
	Window   string `json:"window,omitempty"`
	App      string `json:"app,omitempty"`
	Title    string `json:"title,omitempty"`
	Corner   string `json:"corner,omitempty"`
	Output   string `json:"output,omitempty"`
	Box      *Box   `json:"box,omitempty"`
}

// StrandTargetFrame sends the strand to a box given in fractions of the
// perched window's Box, or of the character's output when Oni is not
// perched. Mode is one of StrandModes (empty points); Region, the area a
// region pen traces, is present exactly in region mode. Quiet plays no
// sound for this target.
type StrandTargetFrame struct {
	Seq    uint64  `json:"seq"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
	Label  string  `json:"label,omitempty"`
	Mode   string  `json:"mode,omitempty"`
	Region *Box    `json:"region,omitempty"`
	Quiet  bool    `json:"quiet,omitempty"`
}

// StrandArriveFrame is the renderer's report that the strand with Seq
// landed, or with Skipped (one of SkipReasons) that it could not be drawn.
// A stale Seq advances nothing.
type StrandArriveFrame struct {
	Seq     uint64 `json:"seq"`
	Skipped string `json:"skipped,omitempty"`
}

// StrandRetractFrame rewinds the strand that went out for Seq.
type StrandRetractFrame struct {
	Seq uint64 `json:"seq"`
}

// BubbleFrame is one line in the bubble at the strand's tip.
type BubbleFrame struct {
	Seq  uint64 `json:"seq"`
	Tone string `json:"tone"`
	Text string `json:"text"`
}

// NoticeFrame is a program's plain notice beside Oni, without a strand.
type NoticeFrame struct {
	Text string `json:"text"`
}

// ConsentFrame asks the user before a feature acts. Destination is local or
// hosted; a hosted one names its Route.
type ConsentFrame struct {
	ID          string `json:"id"`
	Feature     string `json:"feature"`
	Class       string `json:"class"`
	Destination string `json:"destination"`
	Route       string `json:"route,omitempty"`
	Question    string `json:"question"`
	ExpiresMs   int64  `json:"expiresMs"`
}

// ConsentAnswerFrame answers the consent question ID.
type ConsentAnswerFrame struct {
	ID     string `json:"id"`
	Answer string `json:"answer"`
}

// VoiceFrame is the voice pipeline's phase and what opened the microphone.
// Text is the partial heard or the line spoken. A dictation's words never
// ride on it: they go only into the window dictated to, so its frames
// carry no text.
type VoiceFrame struct {
	Phase  string `json:"phase"`
	Text   string `json:"text,omitempty"`
	Source string `json:"source,omitempty"`
}

// LevelFrame is how loud the microphone and Oni's speech are, each in
// [0, 1]. The daemon sends it at most spec voice.levels.maxHz times a
// second and only while the voice phase is listening or speaking; it is
// never in a snapshot.
type LevelFrame struct {
	Mic    float64 `json:"mic"`
	Speech float64 `json:"speech"`
}

// HandsFrame reports Hands. Stage is the stored hands stage: off, browser,
// app or desktop. While no task runs it is inactive, and Reason is the
// stage's row's decision: oni-off, feature-off, or allowed when Hands may
// work but has nothing to do. An active frame is a running task in Scope,
// one of the stage's scopes (protocol.OniHandsStageScopes): Phase one of
// protocol.OniHandsPhases, Step of Max, and Pause, one of the scope's
// pauses (protocol.OniHandsScopePauses), only while paused. A task in a
// picked window also names the window's App id and the Output it is on,
// codes and never its title. The frame that ends a task is inactive, and
// every indication goes with it; End, one of protocol.OniHandsEnds, says
// how it ended, and only that frame carries it.
type HandsFrame struct {
	Stage  string `json:"stage"`
	Active bool   `json:"active"`
	Reason string `json:"reason"`
	Scope  string `json:"scope,omitempty"`
	Phase  string `json:"phase,omitempty"`
	Step   int    `json:"step,omitempty"`
	Max    int    `json:"max,omitempty"`
	Pause  string `json:"pause,omitempty"`
	App    string `json:"app,omitempty"`
	Output string `json:"output,omitempty"`
	End    string `json:"end,omitempty"`
}

// GestureFrame plays one of spec.GestureIDs once.
type GestureFrame struct {
	Seq uint64 `json:"seq"`
	ID  string `json:"id"`
}

// PlacementFrame is where Oni stands: pinned to Corner, or with no corner at
// the saved fractions of the output it is on. Size is one of
// protocol.OniSizes.
type PlacementFrame struct {
	Seq     uint64            `json:"seq"`
	Corner  string            `json:"corner"`
	Size    string            `json:"size"`
	Outputs []PlacementOutput `json:"outputs"`
	Active  string            `json:"active,omitempty"`
}

// PlacementOutput is Oni's saved spot on one output, as fractions of it.
type PlacementOutput struct {
	ID string  `json:"id"`
	FX float64 `json:"fx"`
	FY float64 `json:"fy"`
}

// MotionFrame is how much Oni may move: Reduce is one of
// protocol.OniReduceModes, Courtesy steps Oni aside for covering and
// fullscreen windows. Sounds lets the renderer play Oni's motifs. Chord is
// the hold-to-talk chord a presenter with its own key reader reads, in
// protocol.NormalizeOniChord's form, and empty while voice is off: nothing
// reads keys then.
type MotionFrame struct {
	Reduce       string `json:"reduce"`
	BatterySaver bool   `json:"batterySaver"`
	Courtesy     bool   `json:"courtesy"`
	Sounds       bool   `json:"sounds"`
	Chord        string `json:"chord,omitempty"`
}

type emptyCall struct{}

// the served calls decode into protocol's params through these, which share
// their fields and add the checks
type (
	openCall          protocol.OniOpenParams
	placeCall         protocol.OniPlaceParams
	interactCall      protocol.OniInteractParams
	assetsCall        protocol.OniCharacterAssetsParams
	arrivedCall       protocol.OniStrandArrivedParams
	perchCall         protocol.OniPerchParams
	detachCall        protocol.OniDetachParams
	overlayCall       protocol.OniOverlayParams
	askCall           protocol.OniAskParams
	consentAnswerCall protocol.OniConsentAnswerParams
	talkCall          protocol.OniTalkParams
	speakCall         protocol.OniSpeakParams
	dictateCall       protocol.OniDictateParams
	voiceTestCall     protocol.OniVoiceTestParams
	guideCall         protocol.OniGuideParams
	keepCall          protocol.OniKeepParams
	pickCall          protocol.OniPickParams
	remindersCall     protocol.OniRemindersParams
	routinesCall      protocol.OniRoutinesParams
	handsCall         protocol.OniHandsParams
)

func (StateFrame) Kind() string         { return KindState }
func (CharacterFrame) Kind() string     { return KindCharacter }
func (PerchFrame) Kind() string         { return KindPerch }
func (StrandTargetFrame) Kind() string  { return KindStrandTarget }
func (StrandArriveFrame) Kind() string  { return KindStrandArrive }
func (StrandRetractFrame) Kind() string { return KindStrandRetract }
func (BubbleFrame) Kind() string        { return KindBubble }
func (NoticeFrame) Kind() string        { return KindNotice }
func (ConsentFrame) Kind() string       { return KindConsent }
func (ConsentAnswerFrame) Kind() string { return KindConsentAnswer }
func (VoiceFrame) Kind() string         { return KindVoice }
func (HandsFrame) Kind() string         { return KindHands }
func (GestureFrame) Kind() string       { return KindGesture }
func (PlacementFrame) Kind() string     { return KindPlacement }
func (MotionFrame) Kind() string        { return KindMotion }
func (LevelFrame) Kind() string         { return KindLevel }
func (GuideFrame) Kind() string         { return KindGuide }
func (PickFrame) Kind() string          { return KindPick }

func newPayload(kind string) (Payload, bool) {
	switch kind {
	case KindState:
		return &StateFrame{}, true
	case KindCharacter:
		return &CharacterFrame{}, true
	case KindPerch:
		return &PerchFrame{}, true
	case KindStrandTarget:
		return &StrandTargetFrame{}, true
	case KindStrandArrive:
		return &StrandArriveFrame{}, true
	case KindStrandRetract:
		return &StrandRetractFrame{}, true
	case KindBubble:
		return &BubbleFrame{}, true
	case KindNotice:
		return &NoticeFrame{}, true
	case KindConsent:
		return &ConsentFrame{}, true
	case KindConsentAnswer:
		return &ConsentAnswerFrame{}, true
	case KindVoice:
		return &VoiceFrame{}, true
	case KindHands:
		return &HandsFrame{}, true
	case KindGesture:
		return &GestureFrame{}, true
	case KindPlacement:
		return &PlacementFrame{}, true
	case KindMotion:
		return &MotionFrame{}, true
	case KindLevel:
		return &LevelFrame{}, true
	case KindGuide:
		return &GuideFrame{}, true
	case KindPick:
		return &PickFrame{}, true
	case KindApproval:
		return &ApprovalFrame{}, true
	case KindFocus:
		return &FocusFrame{}, true
	}
	return nil, false
}

// DecodeFrame decodes one frame strictly: exact field names, every field
// without omitempty present, no unknown or duplicate keys, no nulls, no
// trailing data, and values the contract allows.
func DecodeFrame(b []byte) (Payload, error) {
	var f Frame
	if err := decodeStrict(b, &f); err != nil {
		return nil, fmt.Errorf("oni frame: %w", err)
	}
	p, ok := newPayload(f.Kind)
	if !ok {
		return nil, fmt.Errorf("oni frame: unknown kind %q", f.Kind)
	}
	if err := decodeStrict(f.Data, p); err != nil {
		return nil, fmt.Errorf("oni %s frame: %w", f.Kind, err)
	}
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("oni %s frame: %w", f.Kind, err)
	}
	return p, nil
}

// EncodeFrame encodes a valid payload as a frame.
func EncodeFrame(p Payload) ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("oni %s frame: %w", p.Kind(), err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Frame{Kind: p.Kind(), Data: data})
}

// DecodeCall decodes the params of an Oni call as strictly as DecodeFrame.
// Absent params read as {}. The served calls with params decode to
// protocol's: *protocol.OniOpenParams, *protocol.OniPlaceParams,
// *protocol.OniInteractParams, *protocol.OniCharacterAssetsParams,
// *protocol.OniStrandArrivedParams, *protocol.OniPerchParams,
// *protocol.OniDetachParams, *protocol.OniOverlayParams,
// *protocol.OniAskParams, *protocol.OniConsentAnswerParams,
// *protocol.OniTalkParams, *protocol.OniSpeakParams,
// *protocol.OniDictateParams, *protocol.OniVoiceTestParams,
// *protocol.OniGuideParams, *protocol.OniKeepParams (oni.keep and
// oni.skip), *protocol.OniPickParams, *protocol.OniRemindersParams,
// *protocol.OniRoutinesParams, *protocol.OniHandsParams,
// *protocol.OniRewriteParams, *protocol.OniRecallParams,
// *protocol.OniRecallImportParams, *protocol.OniFocusParams and
// *protocol.OniAssignmentParams; oni.reflex.log, oni.hands.log and
// oni.assignment.log take {}.
func DecodeCall(method string, params json.RawMessage) (any, error) {
	var v interface{ validate() error }
	switch method {
	case protocol.MOniClose, protocol.MOniInspect:
		v = &emptyCall{}
	case protocol.MOniGuide:
		v = &guideCall{}
	case protocol.MOniKeep, protocol.MOniSkip:
		v = &keepCall{}
	case protocol.MOniPick:
		v = &pickCall{}
	case protocol.MOniOpen:
		v = &openCall{}
	case protocol.MOniPlace:
		v = &placeCall{}
	case protocol.MOniInteract:
		v = &interactCall{}
	case protocol.MOniCharacterAssets:
		v = &assetsCall{}
	case protocol.MOniPerch:
		v = &perchCall{}
	case protocol.MOniDetach:
		v = &detachCall{}
	case protocol.MOniOverlay:
		v = &overlayCall{}
	case protocol.MOniAsk:
		v = &askCall{}
	case protocol.MOniConsentAnswer:
		v = &consentAnswerCall{}
	case protocol.MOniStrandArrived:
		v = &arrivedCall{}
	case protocol.MOniTalk:
		v = &talkCall{}
	case protocol.MOniSpeak:
		v = &speakCall{}
	case protocol.MOniDictate:
		v = &dictateCall{}
	case protocol.MOniVoiceTest:
		v = &voiceTestCall{}
	case protocol.MOniReminders:
		v = &remindersCall{}
	case protocol.MOniRoutines:
		v = &routinesCall{}
	case protocol.MOniReflexLog, protocol.MOniHandsLog, protocol.MOniAssignmentLog:
		v = &emptyCall{}
	case protocol.MOniHands:
		v = &handsCall{}
	case protocol.MOniRewrite:
		v = &rewriteCall{}
	case protocol.MOniRecall:
		v = &recallCall{}
	case protocol.MOniRecallImport:
		v = &recallImportCall{}
	case protocol.MOniFocus:
		v = &focusCall{}
	case protocol.MOniAssignment:
		v = &assignmentCall{}
	default:
		return nil, fmt.Errorf("%q is not an Oni call", method)
	}
	if len(bytes.TrimSpace(params)) == 0 {
		params = json.RawMessage("{}")
	}
	if err := decodeStrict(params, v); err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	if err := v.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	switch c := v.(type) {
	case *openCall:
		return (*protocol.OniOpenParams)(c), nil
	case *placeCall:
		return (*protocol.OniPlaceParams)(c), nil
	case *interactCall:
		return (*protocol.OniInteractParams)(c), nil
	case *assetsCall:
		return (*protocol.OniCharacterAssetsParams)(c), nil
	case *arrivedCall:
		return (*protocol.OniStrandArrivedParams)(c), nil
	case *perchCall:
		return (*protocol.OniPerchParams)(c), nil
	case *detachCall:
		return (*protocol.OniDetachParams)(c), nil
	case *overlayCall:
		return (*protocol.OniOverlayParams)(c), nil
	case *askCall:
		return (*protocol.OniAskParams)(c), nil
	case *consentAnswerCall:
		return (*protocol.OniConsentAnswerParams)(c), nil
	case *talkCall:
		return (*protocol.OniTalkParams)(c), nil
	case *speakCall:
		return (*protocol.OniSpeakParams)(c), nil
	case *dictateCall:
		return (*protocol.OniDictateParams)(c), nil
	case *voiceTestCall:
		return (*protocol.OniVoiceTestParams)(c), nil
	case *guideCall:
		return (*protocol.OniGuideParams)(c), nil
	case *keepCall:
		return (*protocol.OniKeepParams)(c), nil
	case *pickCall:
		return (*protocol.OniPickParams)(c), nil
	case *remindersCall:
		return (*protocol.OniRemindersParams)(c), nil
	case *routinesCall:
		return (*protocol.OniRoutinesParams)(c), nil
	case *handsCall:
		return (*protocol.OniHandsParams)(c), nil
	case *rewriteCall:
		return (*protocol.OniRewriteParams)(c), nil
	case *recallCall:
		return (*protocol.OniRecallParams)(c), nil
	case *recallImportCall:
		return (*protocol.OniRecallImportParams)(c), nil
	case *focusCall:
		return (*protocol.OniFocusParams)(c), nil
	case *assignmentCall:
		return (*protocol.OniAssignmentParams)(c), nil
	}
	return v, nil
}

func (f StateFrame) validate() error {
	if !slices.Contains(spec.StateIDs, f.State) {
		return fmt.Errorf("unknown state %q", f.State)
	}
	if f.Reason != "" && !validCode(f.Reason, 32) {
		return errors.New("reason is not a short code")
	}
	return nil
}

func (f CharacterFrame) validate() error {
	if !protocol.ValidOniCharacterID(f.ID) {
		return fmt.Errorf("character id %q is invalid", f.ID)
	}
	if !protocol.ValidOniText(f.Name, 64, false) {
		return errors.New("character name is empty, too long or carries control characters")
	}
	if len(f.Fallbacks) > 64 {
		return errors.New("too many fallbacks")
	}
	for _, fb := range f.Fallbacks {
		if !protocol.ValidOniText(fb.Slot, 64, false) || !protocol.ValidOniText(fb.Reason, 200, false) || !protocol.ValidOniText(fb.Used, 120, false) {
			return errors.New("a fallback needs a slot, a reason and what was used")
		}
	}
	return nil
}

func (f PerchFrame) validate() error {
	if f.Seq == 0 {
		return errors.New("seq starts at 1")
	}
	if !f.Attached {
		if f.Window != "" || f.App != "" || f.Title != "" || f.Corner != "" || f.Output != "" || f.Box != nil {
			return errors.New("a detached perch names no window")
		}
		return nil
	}
	if !validToken(f.Window, 64) {
		return errors.New("window is not a seam window id")
	}
	if f.App != "" && !protocol.ValidOniText(f.App, 128, false) {
		return errors.New("app is too long or carries control characters")
	}
	if f.Title != "" && !protocol.ValidOniText(f.Title, 256, false) {
		return errors.New("title is too long or carries control characters")
	}
	if f.Corner != "top-left" && f.Corner != "top-right" {
		return fmt.Errorf("corner %q is not top-left or top-right", f.Corner)
	}
	if !protocol.ValidOniText(f.Output, 64, false) {
		return errors.New("output is missing")
	}
	if f.Box == nil {
		return errors.New("an attached perch carries the window's box")
	}
	return f.Box.validate()
}

func (f StrandTargetFrame) validate() error {
	if f.Seq == 0 {
		return errors.New("seq starts at 1")
	}
	for _, v := range []float64{f.X, f.Y, f.W, f.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return errors.New("box coordinates are fractions of the window or output")
		}
	}
	if f.X+f.W > 1 || f.Y+f.H > 1 {
		return errors.New("box leaves the window")
	}
	if f.Label != "" && !protocol.ValidOniText(f.Label, 120, false) {
		return errors.New("label is too long or carries control characters")
	}
	switch f.Mode {
	case "", StrandModePoint, StrandModeLasso:
		if f.Region != nil {
			return fmt.Errorf("a %s target has no region", f.mode())
		}
	case StrandModeRegion:
		if f.Region == nil {
			return errors.New("a region target carries its region")
		}
		return f.Region.validate()
	case StrandModePerch:
		if f.Region != nil {
			return errors.New("a perch target has no region")
		}
		if f.X != 0 || f.Y != 0 || f.W != 1 || f.H != 1 {
			return errors.New("a perch target is the whole perched window: x=y=0, w=h=1")
		}
	default:
		return fmt.Errorf("mode %q is not point, lasso, region or perch", f.Mode)
	}
	return nil
}

func (f StrandTargetFrame) mode() string {
	if f.Mode == "" {
		return StrandModePoint
	}
	return f.Mode
}

func (f StrandArriveFrame) validate() error { return validArrival(f.Seq, f.Skipped) }

func (c arrivedCall) validate() error { return validArrival(c.Seq, c.Skipped) }

func validArrival(seq uint64, skipped string) error {
	if seq == 0 {
		return errors.New("seq starts at 1")
	}
	if skipped != "" && !slices.Contains(SkipReasons, skipped) {
		return fmt.Errorf("skipped %q is not covers-screen, off-output, covered or hidden", skipped)
	}
	return nil
}

func (f StrandRetractFrame) validate() error {
	if f.Seq == 0 {
		return errors.New("seq starts at 1")
	}
	return nil
}

// a box is finite, has a size and stays within reach of any output
func (b Box) validate() error {
	for _, v := range []float64{b.X, b.Y, b.W, b.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > maxBoxPx {
			return errors.New("a box is finite logical pixels")
		}
	}
	if b.W <= 0 || b.H <= 0 {
		return errors.New("a box has a width and a height")
	}
	return nil
}

func (f BubbleFrame) validate() error {
	if f.Seq == 0 {
		return errors.New("seq starts at 1")
	}
	switch f.Tone {
	case "say", "answer", "warn":
	default:
		return fmt.Errorf("unknown tone %q", f.Tone)
	}
	if !protocol.ValidOniText(f.Text, 600, true) {
		return errors.New("text is empty, too long or carries control characters")
	}
	return nil
}

func (f NoticeFrame) validate() error {
	if !protocol.ValidOniText(f.Text, 600, true) {
		return errors.New("notice text is empty, too long or carries control characters")
	}
	return nil
}

func (f ConsentFrame) validate() error {
	if !validCode(f.ID, 64) {
		return errors.New("consent id is not a short code")
	}
	feat, ok := Lookup(f.Feature)
	if !ok {
		return fmt.Errorf("unknown feature %q", f.Feature)
	}
	if feat.Stage == StageUnavailable {
		return fmt.Errorf("feature %q is unavailable in this build", f.Feature)
	}
	if f.Class != feat.Consent {
		return fmt.Errorf("feature %q asks for %s consent, not %q", f.Feature, feat.Consent, f.Class)
	}
	switch f.Destination {
	case "local":
	case "hosted":
		if feat.Egress == EgressLocal {
			return fmt.Errorf("feature %q keeps everything on this computer", f.Feature)
		}
		if f.Route == "" {
			return errors.New("a hosted destination names its route")
		}
	default:
		return fmt.Errorf("destination %q is not local or hosted", f.Destination)
	}
	if f.Route != "" && !protocol.ValidOniText(f.Route, 128, false) {
		return errors.New("route is too long or carries control characters")
	}
	if !protocol.ValidOniText(f.Question, 400, true) {
		return errors.New("question is empty, too long or carries control characters")
	}
	if f.ExpiresMs < 1 || f.ExpiresMs > ConsentTTL.Milliseconds() {
		return fmt.Errorf("a consent question expires within %d ms", ConsentTTL.Milliseconds())
	}
	switch f.Feature {
	case GuideKeep:
		return validLocalOffer(f, KeepOfferTTL())
	case GuideHints:
		return validLocalOffer(f, HintOfferTTL())
	case Routines:
		return validLocalOffer(f, RoutineOfferTTL())
	case Decisions:
		return validDecisionCard(f)
	case HandsBrowser, HandsApp, HandsDesktop:
		return validHandoffCard(f)
	case MemoryForget, FeedGuard, Recall, Rewrite, Assignment:
		return validPhase16Card(f)
	case Reflex, Reminders, Notes:
		return fmt.Errorf("%s announces what it does and asks nothing", f.Feature)
	}
	return nil
}

func (f ConsentAnswerFrame) validate() error {
	if !validCode(f.ID, 64) {
		return errors.New("consent id is not a short code")
	}
	if f.Answer != "yes" && f.Answer != "no" {
		return fmt.Errorf("answer %q is not yes or no", f.Answer)
	}
	return nil
}

func (f VoiceFrame) validate() error {
	if f.Source != "" && !slices.Contains(VoiceSources, f.Source) {
		return fmt.Errorf("unknown voice source %q", f.Source)
	}
	dictation := f.Source == VoiceSourceDictation
	switch f.Phase {
	case VoiceIdle:
		if f.Text != "" || f.Source != "" {
			return errors.New("the idle phase carries no text and no source")
		}
	case VoiceArmed:
		if f.Source != VoiceSourceWake && f.Source != VoiceSourceOpen {
			return fmt.Errorf("the armed phase is the wake phrase or open mic, not %q", f.Source)
		}
		if f.Text != "" {
			return errors.New("the armed phase carries no text")
		}
	case VoiceListening:
		if f.Text != "" {
			return errors.New("the listening phase carries no text")
		}
	case VoiceTranscribing:
		if dictation {
			if f.Text != "" {
				return errors.New("a dictation's words never ride on a frame")
			}
			return nil
		}
		if !protocol.ValidOniText(f.Text, 600, true) {
			return errors.New("text is empty, too long or carries control characters")
		}
	case VoiceSpeaking:
		if dictation {
			return errors.New("a dictation is never spoken")
		}
		if !protocol.ValidOniText(f.Text, 600, true) {
			return errors.New("text is empty, too long or carries control characters")
		}
	default:
		return fmt.Errorf("unknown voice phase %q", f.Phase)
	}
	return nil
}

func (f LevelFrame) validate() error {
	if !fraction(f.Mic) || !fraction(f.Speech) {
		return errors.New("a level is a number in [0, 1]")
	}
	return nil
}

func (c talkCall) validate() error {
	if !slices.Contains(protocol.OniVoiceActions, c.Action) {
		return fmt.Errorf("action %q is not start, stop or cancel", c.Action)
	}
	return nil
}

func (c speakCall) validate() error {
	if c.Action != protocol.OniVoiceStop {
		return fmt.Errorf("action %q is not stop", c.Action)
	}
	return nil
}

func (c dictateCall) validate() error {
	if !slices.Contains(protocol.OniVoiceActions, c.Action) {
		return fmt.Errorf("action %q is not start, stop or cancel", c.Action)
	}
	if c.Mode == "" {
		return nil
	}
	if c.Action != protocol.OniVoiceStart {
		return errors.New("only a start names a mode")
	}
	if !slices.Contains(protocol.OniDictateModes, c.Mode) {
		return fmt.Errorf("mode %q is not exact, clean or polished", c.Mode)
	}
	return nil
}

func (c voiceTestCall) validate() error {
	if c.Mic != "" && !protocol.ValidOniMicNode(c.Mic) {
		return errors.New("mic is not a source node name")
	}
	if c.Ms != 0 && (c.Ms < protocol.OniVoiceTestMinMs || c.Ms > protocol.OniVoiceTestMaxMs) {
		return fmt.Errorf("a mic test runs %d to %d ms", protocol.OniVoiceTestMinMs, protocol.OniVoiceTestMaxMs)
	}
	return nil
}

// a perch names a window, or, from the presenter's drop, an output and a
// point on it the daemon hit-tests
func (c perchCall) validate() error {
	switch {
	case c.Window != "" && c.Output != "":
		return errors.New("a perch names a window or a point on an output, not both")
	case c.Window != "":
		if !validToken(c.Window, 64) {
			return errors.New("window is not a seam window id")
		}
		if c.X != 0 || c.Y != 0 || c.Hover {
			return errors.New("a perch on a named window carries no point")
		}
		return nil
	case c.Output != "":
		if !validOutput(c.Output) {
			return errors.New("the output id is too long or carries control characters")
		}
		for _, v := range []float64{c.X, c.Y} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > maxBoxPx {
				return errors.New("a point is finite logical pixels on its output")
			}
		}
		return nil
	}
	return errors.New("a perch names a window or a point on an output")
}

func (c detachCall) validate() error {
	switch c.Reason {
	case "", PerchUser, PerchDraggedOff, PerchHidden:
		return nil
	}
	return fmt.Errorf("detach reason %q is not user, dragged-off or hidden", c.Reason)
}

func (c overlayCall) validate() error {
	if !validOutput(c.Output) {
		return errors.New("the output id is empty, too long or carries control characters")
	}
	if len(c.Rects) > MaxOverlayRects {
		return fmt.Errorf("more than %d overlay rects", MaxOverlayRects)
	}
	for _, r := range c.Rects {
		if err := (Box{X: r.X, Y: r.Y, W: r.W, H: r.H}).validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c askCall) validate() error {
	if !protocol.ValidOniText(c.Text, 2000, true) {
		return errors.New("text is empty, too long or carries control characters")
	}
	return nil
}

// a card whose id starts h- is never answered here: a hint is kept or
// skipped, and a Hands handoff ends only when the user presses the control
// themselves or stops Hands
func (c consentAnswerCall) validate() error {
	if strings.HasPrefix(c.ID, HandsCardPrefix) {
		return fmt.Errorf("card %q is not answered with yes or no: a hint is kept or skipped, and a Hands handoff ends when the user presses the control or stops Hands", c.ID)
	}
	// an approval card is answered with one of its buttons
	if strings.HasPrefix(c.ID, ApprovalHandsPrefix) || strings.HasPrefix(c.ID, ApprovalPermissionPrefix) {
		if !validCode(c.ID, 64) || !slices.Contains(ApprovalOptions, c.Answer) {
			return fmt.Errorf("an approval card is answered with one of %v", ApprovalOptions)
		}
		return nil
	}
	return ConsentAnswerFrame{ID: c.ID, Answer: c.Answer}.validate()
}

func (emptyCall) validate() error { return nil }

func (f GestureFrame) validate() error {
	if f.Seq == 0 {
		return errors.New("seq starts at 1")
	}
	if !slices.Contains(spec.GestureIDs, f.ID) {
		return fmt.Errorf("unknown gesture %q", f.ID)
	}
	return nil
}

func (f PlacementFrame) validate() error {
	if f.Seq == 0 {
		return errors.New("seq starts at 1")
	}
	if f.Corner != "" && !slices.Contains(protocol.OniCorners, f.Corner) {
		return fmt.Errorf("corner %q is not tl, tr, bl, br or empty", f.Corner)
	}
	if !slices.Contains(protocol.OniSizes, f.Size) {
		return fmt.Errorf("size %q is not small, normal or big", f.Size)
	}
	if len(f.Outputs) > MaxPlacementOutputs {
		return fmt.Errorf("more than %d outputs", MaxPlacementOutputs)
	}
	seen := map[string]bool{}
	for _, o := range f.Outputs {
		if !validOutput(o.ID) {
			return errors.New("an output id is empty, too long or carries control characters")
		}
		if seen[o.ID] {
			return fmt.Errorf("output %q is listed twice", o.ID)
		}
		seen[o.ID] = true
		if !fraction(o.FX) || !fraction(o.FY) {
			return fmt.Errorf("output %q: a spot is fractions of the output", o.ID)
		}
	}
	if f.Active != "" && !validOutput(f.Active) {
		return errors.New("the active output id is too long or carries control characters")
	}
	return nil
}

func (f MotionFrame) validate() error {
	if !slices.Contains(protocol.OniReduceModes, f.Reduce) {
		return fmt.Errorf("reduce %q is not follow, on or off", f.Reduce)
	}
	if f.Chord != "" {
		if canon, ok := protocol.NormalizeOniChord(f.Chord); !ok || canon != f.Chord {
			return fmt.Errorf("chord %q is not a canonical talk chord", f.Chord)
		}
	}
	return nil
}

func (c openCall) validate() error {
	switch c.Renderer {
	case protocol.OniRendererNative, protocol.OniRendererRyoku, protocol.OniRendererObserver:
		return nil
	}
	return fmt.Errorf("renderer %q is not native, ryoku or observer", c.Renderer)
}

func (c placeCall) validate() error {
	if c.Corner != "" && !slices.Contains(protocol.OniCorners, c.Corner) {
		return fmt.Errorf("corner %q is not tl, tr, bl, br or empty", c.Corner)
	}
	if !fraction(c.FX) || !fraction(c.FY) {
		return errors.New("a spot is fractions of the output")
	}
	if c.Output == "" {
		if c.Corner == "" {
			return errors.New("a free spot names its output")
		}
		return nil
	}
	if !validOutput(c.Output) {
		return errors.New("the output id is too long or carries control characters")
	}
	return nil
}

func (c interactCall) validate() error {
	switch c.Kind {
	case protocol.OniInteractPoke, protocol.OniInteractWake, protocol.OniInteractDrag, protocol.OniInteractDrop:
		return nil
	}
	return fmt.Errorf("interaction %q is not poke, wake, drag or drop", c.Kind)
}

func (c assetsCall) validate() error {
	if !protocol.ValidOniCharacterID(c.ID) {
		return fmt.Errorf("character id %q is invalid", c.ID)
	}
	return nil
}

// a compositor's output name, such as DP-1 or HEADLESS-2
func validOutput(id string) bool { return protocol.ValidOniText(id, maxOutputID, false) }

func fraction(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }

// a short lowercase code: letters, digits and hyphens
func validCode(s string, limit int) bool {
	if s == "" || len(s) > limit {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// a seam window id: niri's numbers, Hyprland's 0x addresses
func validToken(s string, limit int) bool {
	if s == "" || len(s) > limit {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && !strings.ContainsRune(":._-", rune(c)) {
			return false
		}
	}
	return true
}

var (
	rawMessageType = reflect.TypeFor[json.RawMessage]()
	// a time decodes from its RFC 3339 string; json.Unmarshal checks it
	timeType = reflect.TypeFor[time.Time]()
)

func decodeStrict(b []byte, v any) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return errors.New("not a JSON object")
	}
	if err := noDuplicateKeys(b); err != nil {
		return err
	}
	if err := checkKeys(b, reflect.TypeOf(v)); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing data after the object")
	}
	return nil
}

// encoding/json keeps the last of two equal keys; a frame with both is
// ambiguous and refused
func noDuplicateKeys(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				key, _ := kt.(string)
				if seen[key] {
					return fmt.Errorf("duplicate key %q", key)
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
		}
		_, err = dec.Token()
		return err
	}
	return walk()
}

type wireField struct {
	name     string
	typ      reflect.Type
	optional bool
}

// encoding/json matches keys case-insensitively and treats absent fields as
// zero; the wire contract wants exact names and every required field
func checkKeys(raw json.RawMessage, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == rawMessageType {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("null value")
	}
	if t == timeType {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return err
		}
		var fields []wireField
		for i := range t.NumField() {
			sf := t.Field(i)
			name, opts, _ := strings.Cut(sf.Tag.Get("json"), ",")
			if !sf.IsExported() || name == "-" {
				continue
			}
			if name == "" {
				name = sf.Name
			}
			fields = append(fields, wireField{name: name, typ: sf.Type, optional: strings.Contains(opts, "omitempty") || strings.Contains(opts, "omitzero")})
		}
		for key := range obj {
			if !slices.ContainsFunc(fields, func(f wireField) bool { return f.name == key }) {
				return fmt.Errorf("unknown field %q", key)
			}
		}
		for _, f := range fields {
			v, ok := obj[f.name]
			if !ok {
				if f.optional {
					continue
				}
				return fmt.Errorf("missing field %q", f.name)
			}
			if err := checkKeys(v, f.typ); err != nil {
				return fmt.Errorf("%s: %w", f.name, err)
			}
		}
	case reflect.Slice:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return err
		}
		for i, item := range items {
			if err := checkKeys(item, t.Elem()); err != nil {
				return fmt.Errorf("[%d]: %w", i, err)
			}
		}
	}
	return nil
}
