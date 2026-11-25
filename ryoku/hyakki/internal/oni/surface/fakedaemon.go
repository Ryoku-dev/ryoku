package surface

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// ScriptEntry is one line of a lab frame script: a frame to send AtMs after
// oni.open, or EOF to end the link as a daemon exit would.
type ScriptEntry struct {
	AtMs  int64           `json:"atMs"`
	Frame json.RawMessage `json:"frame,omitempty"`
	EOF   bool            `json:"eof,omitempty"`
}

// LoadScript reads a script as a JSON array or as JSON lines. Every frame is
// decoded strictly, so a script can only send what the daemon could.
func LoadScript(r io.Reader) ([]ScriptEntry, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSpace(data)
	var entries []ScriptEntry
	if len(data) > 0 && data[0] == '[' {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&entries); err != nil {
			return nil, fmt.Errorf("oni script: %w", err)
		}
	} else {
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for n := 1; sc.Scan(); n++ {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var e ScriptEntry
			dec := json.NewDecoder(bytes.NewReader(line))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&e); err != nil {
				return nil, fmt.Errorf("oni script line %d: %w", n, err)
			}
			entries = append(entries, e)
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	for i, e := range entries {
		if e.AtMs < 0 {
			return nil, fmt.Errorf("oni script entry %d: atMs is negative", i+1)
		}
		if e.EOF == (len(e.Frame) > 0) {
			return nil, fmt.Errorf("oni script entry %d: needs exactly one of frame or eof", i+1)
		}
		if e.EOF {
			continue
		}
		if _, err := feature.DecodeFrame(e.Frame); err != nil {
			return nil, fmt.Errorf("oni script entry %d: %w", i+1, err)
		}
	}
	sort.SliceStable(entries, func(a, b int) bool { return entries[a].AtMs < entries[b].AtMs })
	return entries, nil
}

// ScriptDialer serves the presenter from a script in-process. Each dial is a
// fresh daemon that replays the script from its own oni.open. A non-empty
// arrivals path records every oni.strandArrived call there (Arrival lines).
func ScriptDialer(entries []ScriptEntry, arrivals string) Dialer {
	return func(ctx context.Context, start bool) (Link, error) {
		d := newFakeDaemon(entries)
		d.arrivals = arrivals
		return d, nil
	}
}

// Arrival is one oni.strandArrived call the scripted daemon served.
// Advanced is true only for the seq it was waiting on, as the daemon would
// advance it: a stale, repeated or retracted seq advances nothing.
type Arrival struct {
	T        float64 `json:"t"`
	Seq      uint64  `json:"seq"`
	Skipped  string  `json:"skipped,omitempty"`
	Advanced bool    `json:"advanced"`
}

type fakeDaemon struct {
	entries  []ScriptEntry
	arrivals string

	mu      sync.Mutex
	state   string
	place   feature.PlacementFrame
	gseq    uint64
	pending uint64 // the strand seq awaiting arrival, 0 for none
	landed  []Arrival
	queue   []json.RawMessage
	signal  chan struct{}
	frames  chan json.RawMessage
	done    chan struct{}
	err     error
	timers  []*time.Timer
	opened  bool
	calls   []string
	stopped bool

	// perchAt answers oni.perch; nil refuses every drop as an empty
	// desktop would (reason closed) and finds no window under a hover.
	perchAt  func(p protocol.OniPerchParams) (protocol.OniPerchResult, error)
	perches  []protocol.OniPerchParams
	detaches []string
	overlays []protocol.OniOverlayParams
	talks    []string
	guides   []string
	skips    []string
	picks    []protocol.OniPickParams
	hands    []string
	answers  []protocol.OniConsentAnswerParams
}

func newFakeDaemon(entries []ScriptEntry) *fakeDaemon {
	d := &fakeDaemon{
		entries: entries,
		state:   "idle",
		place:   feature.PlacementFrame{Seq: 1, Corner: "br", Size: "normal", Outputs: []feature.PlacementOutput{}},
		signal:  make(chan struct{}, 1),
		frames:  make(chan json.RawMessage),
		done:    make(chan struct{}),
	}
	go d.pump()
	return d
}

func (d *fakeDaemon) pump() {
	defer close(d.frames)
	for {
		d.mu.Lock()
		if len(d.queue) == 0 {
			d.mu.Unlock()
			select {
			case <-d.signal:
				continue
			case <-d.done:
				return
			}
		}
		f := d.queue[0]
		d.queue = d.queue[1:]
		d.mu.Unlock()
		select {
		case d.frames <- f:
		case <-d.done:
			return
		}
	}
}

func (d *fakeDaemon) snapshot() []json.RawMessage {
	loaded := profile.Default()
	out := make([]json.RawMessage, 0, 5)
	for _, p := range []feature.Payload{
		&feature.StateFrame{State: d.state},
		&feature.CharacterFrame{ID: loaded.Profile.ID, Name: loaded.Profile.Name},
		&feature.MotionFrame{Reduce: "follow", Courtesy: true},
		&d.place,
		&feature.VoiceFrame{Phase: "idle"},
	} {
		b, err := feature.EncodeFrame(p)
		if err != nil {
			panic(err)
		}
		out = append(out, b)
	}
	return out
}

func (d *fakeDaemon) Open(ctx context.Context) (protocol.OniOpenResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.opened {
		return protocol.OniOpenResult{}, errors.New("oni.open: already open")
	}
	d.opened = true
	d.calls = append(d.calls, protocol.MOniOpen)
	snap := d.snapshot()
	for _, e := range d.entries {
		d.timers = append(d.timers, time.AfterFunc(time.Duration(e.AtMs)*time.Millisecond, func() { d.play(e) }))
	}
	return protocol.OniOpenResult{Presenter: true, Frames: snap}, nil
}

func (d *fakeDaemon) play(e ScriptEntry) {
	if e.EOF {
		d.end(io.EOF)
		return
	}
	p, err := feature.DecodeFrame(e.Frame)
	if err != nil {
		return
	}
	d.mu.Lock()
	switch f := p.(type) {
	case *feature.StateFrame:
		d.state = f.State
	case *feature.PlacementFrame:
		d.place = *f
	case *feature.StrandTargetFrame:
		d.pending = f.Seq
	case *feature.StrandRetractFrame:
		if d.pending == f.Seq {
			d.pending = 0
		}
	}
	d.mu.Unlock()
	d.send(e.Frame)
}

func (d *fakeDaemon) send(frame json.RawMessage) {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	d.queue = append(d.queue, frame)
	d.mu.Unlock()
	select {
	case d.signal <- struct{}{}:
	default:
	}
}

func (d *fakeDaemon) emit(p feature.Payload) {
	b, err := feature.EncodeFrame(p)
	if err != nil {
		return
	}
	d.send(b)
}

func (d *fakeDaemon) end(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	d.stopped = true
	d.err = err
	for _, t := range d.timers {
		t.Stop()
	}
	close(d.done)
}

func (d *fakeDaemon) Frames() <-chan json.RawMessage { return d.frames }
func (d *fakeDaemon) Done() <-chan struct{}          { return d.done }

func (d *fakeDaemon) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

func (d *fakeDaemon) Close() error {
	d.end(io.EOF)
	return nil
}

func (d *fakeDaemon) live(method string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return fmt.Errorf("%s: %w", method, io.EOF)
	}
	d.calls = append(d.calls, method)
	return nil
}

func (d *fakeDaemon) Place(ctx context.Context, p protocol.OniPlaceParams) error {
	if err := d.live(protocol.MOniPlace); err != nil {
		return err
	}
	d.mu.Lock()
	d.place.Seq++
	d.place.Corner = p.Corner
	d.place.Active = p.Output
	outs := d.place.Outputs[:0:0]
	for _, o := range d.place.Outputs {
		if o.ID != p.Output {
			outs = append(outs, o)
		}
	}
	if p.Corner == "" {
		outs = append(outs, feature.PlacementOutput{ID: p.Output, FX: p.FX, FY: p.FY})
	}
	d.place.Outputs = outs
	frame := d.place
	d.mu.Unlock()
	d.emit(&frame)
	return nil
}

func (d *fakeDaemon) Interact(ctx context.Context, kind string) error {
	if err := d.live(protocol.MOniInteract); err != nil {
		return err
	}
	d.mu.Lock()
	wake := kind == "wake" && d.state == "sleeping"
	if wake {
		d.state = "idle"
		d.gseq++
	}
	seq := d.gseq
	d.mu.Unlock()
	if wake {
		d.emit(&feature.GestureFrame{Seq: seq, ID: "wake"})
		d.emit(&feature.StateFrame{State: "idle"})
	}
	return nil
}

func (d *fakeDaemon) Assets(ctx context.Context, id string) (protocol.OniCharacterAssets, error) {
	if err := d.live(protocol.MOniCharacterAssets); err != nil {
		return protocol.OniCharacterAssets{}, err
	}
	var loaded profile.Loaded
	if id == profile.ShippedID {
		loaded = profile.Default()
	} else {
		loaded, _ = profile.Load(os.DirFS(paths.CharactersDir()), id)
	}
	if !loaded.Valid {
		return protocol.OniCharacterAssets{}, fmt.Errorf("oni.character.assets: %q is not a valid character", id)
	}
	return protocol.OniCharacterAssets{ID: id, Files: loaded.Files}, nil
}

func (d *fakeDaemon) StrandArrived(ctx context.Context, seq uint64, skipped string) error {
	if err := d.live(protocol.MOniStrandArrived); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	a := Arrival{Seq: seq, Skipped: skipped, Advanced: seq != 0 && seq == d.pending}
	a.T = float64(time.Now().UnixNano()) / 1e9
	if a.Advanced {
		d.pending = 0
	}
	d.landed = append(d.landed, a)
	if d.arrivals == "" {
		return nil
	}
	line, err := json.Marshal(a)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(d.arrivals, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func (d *fakeDaemon) Perch(ctx context.Context, p protocol.OniPerchParams) (protocol.OniPerchResult, error) {
	if err := d.live(protocol.MOniPerch); err != nil {
		return protocol.OniPerchResult{}, err
	}
	d.mu.Lock()
	d.perches = append(d.perches, p)
	at := d.perchAt
	d.mu.Unlock()
	if at != nil {
		return at(p)
	}
	if p.Hover {
		return protocol.OniPerchResult{Reason: "closed"}, nil
	}
	return protocol.OniPerchResult{}, &protocol.Error{Code: protocol.CodeNotFound, Message: "There is no window there.", Reason: "closed"}
}

func (d *fakeDaemon) Detach(ctx context.Context, reason string) error {
	if err := d.live(protocol.MOniDetach); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.detaches = append(d.detaches, reason)
	return nil
}

func (d *fakeDaemon) Overlay(ctx context.Context, output string, rects []protocol.OniRect) error {
	if err := d.live(protocol.MOniOverlay); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.overlays = append(d.overlays, protocol.OniOverlayParams{Output: output, Rects: append([]protocol.OniRect(nil), rects...)})
	return nil
}

// Talk records the press; the scripted daemon has no microphone.
func (d *fakeDaemon) Talk(ctx context.Context, action string) error {
	if err := d.live(protocol.MOniTalk); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.talks = append(d.talks, action)
	return nil
}

// Guide records the card's action; the scripted daemon runs no guide.
func (d *fakeDaemon) Guide(ctx context.Context, action string) error {
	if err := d.live(protocol.MOniGuide); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.guides = append(d.guides, action)
	return nil
}

// Skip records the offer let go.
func (d *fakeDaemon) Skip(ctx context.Context, id string) error {
	if err := d.live(protocol.MOniSkip); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.skips = append(d.skips, id)
	return nil
}

// Pick records the click or the cancel.
func (d *fakeDaemon) Pick(ctx context.Context, p protocol.OniPickParams) error {
	if err := d.live(protocol.MOniPick); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.picks = append(d.picks, p)
	return nil
}

// Hands records the pill's or the card's action; the scripted daemon runs
// no Hands task.
func (d *fakeDaemon) Hands(ctx context.Context, action string) error {
	if err := d.live(protocol.MOniHands); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hands = append(d.hands, action)
	return nil
}

// Answer records an approval card's answer; the scripted daemon decides
// nothing.
func (d *fakeDaemon) Answer(ctx context.Context, id, option string) error {
	if err := d.live(protocol.MOniConsentAnswer); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.answers = append(d.answers, protocol.OniConsentAnswerParams{ID: id, Answer: option})
	return nil
}

// Talks lists the oni.talk actions the fake served, in order (tests).
func (d *fakeDaemon) Talks() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.talks...)
}

// Arrivals lists the oni.strandArrived calls the fake served (tests).
func (d *fakeDaemon) Arrivals() []Arrival {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Arrival(nil), d.landed...)
}

// Calls lists the methods the fake served, in order (tests).
func (d *fakeDaemon) Calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}
