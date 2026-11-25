package surface

import (
	"context"
	"encoding/json"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// Renderer is the presenter kind this process opens as.
const Renderer = "native"

// Link is the presenter's connection to the daemon, real or scripted.
type Link interface {
	Open(ctx context.Context) (protocol.OniOpenResult, error)
	// Frames carries the data of every oni event, in order; it closes when
	// the link ends.
	Frames() <-chan json.RawMessage
	Done() <-chan struct{}
	Err() error
	Place(ctx context.Context, p protocol.OniPlaceParams) error
	Interact(ctx context.Context, kind string) error
	Assets(ctx context.Context, id string) (protocol.OniCharacterAssets, error)
	// StrandArrived reports that the strand for seq landed, or with
	// skipped (a feature.SkipReasons value) that it could not be drawn.
	StrandArrived(ctx context.Context, seq uint64, skipped string) error
	// Perch asks the daemon to perch on the window it finds at a point
	// (output-local logical px) or by id; with Hover it only names that
	// window. A refusal is a *protocol.Error whose Reason says why.
	Perch(ctx context.Context, p protocol.OniPerchParams) (protocol.OniPerchResult, error)
	// Detach takes the perch down for reason (one of the presenter's:
	// user, dragged-off, hidden).
	Detach(ctx context.Context, reason string) error
	// Overlay reports the presenter's own pixels on output, output-local
	// logical px, for capture masks; an empty list clears them.
	Overlay(ctx context.Context, output string, rects []protocol.OniRect) error
	// Talk starts, stops or cancels one hold-to-talk press
	// (protocol.OniVoiceActions).
	Talk(ctx context.Context, action string) error
	// Guide sends one of the guide card's actions: next, stop or circle
	// (protocol.OniGuideNext, Stop, Circle). The presenter never starts a
	// guide.
	Guide(ctx context.Context, action string) error
	// Skip lets the keep offer or hint id go (oni.skip). The presenter
	// never keeps: oni.keep is the app's, the CLI's or a spoken yes.
	Skip(ctx context.Context, id string) error
	// Pick reports the pick overlay's one click (protocol.OniPickPoint)
	// or its cancel (protocol.OniPickCancel). The presenter never arms.
	Pick(ctx context.Context, p protocol.OniPickParams) error
	// Hands sends one of the Hands pill's or card's actions: stop or
	// resume (protocol.OniHandsStop, OniHandsResume). The presenter never
	// starts, authorizes or answers a handed-off step.
	Hands(ctx context.Context, action string) error
	// Answer answers the approval card id with one of its options
	// (oni.consent.answer). The daemon takes only approval cards' answers
	// from a presenter.
	Answer(ctx context.Context, id, option string) error
	Close() error
}

// Dialer connects a Link; start lets it launch the daemon.
type Dialer func(ctx context.Context, start bool) (Link, error)

// DaemonDialer dials the real daemon as a passive surface client, which never
// keeps the daemon alive.
func DaemonDialer(socket string) Dialer {
	return func(ctx context.Context, start bool) (Link, error) {
		c, err := client.Dial(ctx, client.Options{Client: protocol.ClientSurface, Socket: socket, Start: start})
		if err != nil {
			return nil, err
		}
		l := &daemonLink{c: c, frames: make(chan json.RawMessage)}
		go l.pump()
		return l, nil
	}
}

type daemonLink struct {
	c      *client.Conn
	frames chan json.RawMessage
}

func (l *daemonLink) pump() {
	defer close(l.frames)
	for ev := range l.c.Events() {
		if ev.Event == protocol.EvtOni {
			l.frames <- ev.Data
		}
	}
}

func (l *daemonLink) Open(ctx context.Context) (protocol.OniOpenResult, error) {
	var r protocol.OniOpenResult
	err := l.c.Call(ctx, protocol.MOniOpen, protocol.OniOpenParams{Renderer: Renderer}, &r)
	return r, err
}

func (l *daemonLink) Frames() <-chan json.RawMessage { return l.frames }
func (l *daemonLink) Done() <-chan struct{}          { return l.c.Done() }
func (l *daemonLink) Err() error                     { return l.c.Err() }
func (l *daemonLink) Close() error                   { return l.c.Close() }

func (l *daemonLink) Place(ctx context.Context, p protocol.OniPlaceParams) error {
	return l.c.Call(ctx, protocol.MOniPlace, p, nil)
}

func (l *daemonLink) Interact(ctx context.Context, kind string) error {
	return l.c.Call(ctx, protocol.MOniInteract, protocol.OniInteractParams{Kind: kind}, nil)
}

func (l *daemonLink) Assets(ctx context.Context, id string) (protocol.OniCharacterAssets, error) {
	var a protocol.OniCharacterAssets
	err := l.c.Call(ctx, protocol.MOniCharacterAssets, protocol.OniCharacterAssetsParams{ID: id}, &a)
	return a, err
}

func (l *daemonLink) StrandArrived(ctx context.Context, seq uint64, skipped string) error {
	return l.c.Call(ctx, protocol.MOniStrandArrived, protocol.OniStrandArrivedParams{Seq: seq, Skipped: skipped}, nil)
}

func (l *daemonLink) Perch(ctx context.Context, p protocol.OniPerchParams) (protocol.OniPerchResult, error) {
	var res protocol.OniPerchResult
	err := l.c.Call(ctx, protocol.MOniPerch, p, &res)
	return res, err
}

func (l *daemonLink) Detach(ctx context.Context, reason string) error {
	return l.c.Call(ctx, protocol.MOniDetach, protocol.OniDetachParams{Reason: reason}, nil)
}

func (l *daemonLink) Overlay(ctx context.Context, output string, rects []protocol.OniRect) error {
	return l.c.Call(ctx, protocol.MOniOverlay, protocol.OniOverlayParams{Output: output, Rects: rects}, nil)
}

func (l *daemonLink) Talk(ctx context.Context, action string) error {
	return l.c.Call(ctx, protocol.MOniTalk, protocol.OniTalkParams{Action: action}, nil)
}

func (l *daemonLink) Guide(ctx context.Context, action string) error {
	return l.c.Call(ctx, protocol.MOniGuide, protocol.OniGuideParams{Action: action}, nil)
}

func (l *daemonLink) Skip(ctx context.Context, id string) error {
	return l.c.Call(ctx, protocol.MOniSkip, protocol.OniKeepParams{ID: id}, nil)
}

func (l *daemonLink) Pick(ctx context.Context, p protocol.OniPickParams) error {
	return l.c.Call(ctx, protocol.MOniPick, p, nil)
}

func (l *daemonLink) Hands(ctx context.Context, action string) error {
	return l.c.Call(ctx, protocol.MOniHands, protocol.OniHandsParams{Action: action}, nil)
}

func (l *daemonLink) Answer(ctx context.Context, id, option string) error {
	return l.c.Call(ctx, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: id, Answer: option}, nil)
}

// Loss is what the presenter does when its link ends.
type Loss int

const (
	// LossDormant keeps the last frame on screen and waits, without polling,
	// for a click to redial.
	LossDormant Loss = iota
	// LossRetry redials once without starting the daemon: a socket-activated
	// daemon comes back on its own.
	LossRetry
)

// onLinkLost decides after the link ends. Asleep, the daemon idled out as it
// should, so the surface stays dormant at no cost; awake, one quiet redial
// covers a restart, and a second loss goes dormant too.
func onLinkLost(state string, retried bool) Loss {
	if state == "sleeping" || retried {
		return LossDormant
	}
	return LossRetry
}

// openOutcome is what an oni.open answer means for this process.
type openOutcome int

const (
	openPresent openOutcome = iota
	// openExit: Oni is off or another presenter holds the screen.
	openExit
)

func openResult(r protocol.OniOpenResult) openOutcome {
	if r.Presenter {
		return openPresent
	}
	return openExit
}

// exitFrame reports whether a decoded frame tells the presenter to quit.
func exitFrame(p feature.Payload) bool {
	s, ok := p.(*feature.StateFrame)
	return ok && s.Reason == "oni-off"
}

// hiddenFrame reports whether a decoded state frame unmaps the character.
func hiddenFrame(p feature.Payload) (hidden, isState bool) {
	s, ok := p.(*feature.StateFrame)
	if !ok {
		return false, false
	}
	return s.Reason == "hidden", true
}
