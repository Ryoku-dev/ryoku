package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// oniPeer is a connection to the daemon that keeps the oni frames it is
// sent, as one the daemon accepted on its socket.
type oniPeer struct {
	t         *testing.T
	enc       *protocol.Encoder
	seq       int64
	responses chan protocol.Frame
	frames    chan []byte
	leave     func()
}

func connectOni(t *testing.T, d *Daemon, kind string, app bool) *oniPeer {
	t.Helper()
	server, client := net.Pipe()
	c := newConnection(d, server)
	c.app = app
	d.mu.Lock()
	d.clients[c] = struct{}{}
	d.mu.Unlock()
	stopped := make(chan struct{})
	go func() { c.serve(); close(stopped) }()
	p := &oniPeer{t: t, enc: protocol.NewEncoder(client), responses: make(chan protocol.Frame, 64), frames: make(chan []byte, 256)}
	go func() {
		dec := protocol.NewDecoder(client)
		for {
			f, err := dec.Next()
			if err != nil {
				return
			}
			switch {
			case f.IsEvent() && f.Event == protocol.EvtOni:
				p.frames <- f.Data
			case f.ID != nil:
				p.responses <- f
			}
		}
	}()
	var once sync.Once
	p.leave = func() {
		once.Do(func() {
			client.Close()
			c.close()
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Error("connection did not close")
			}
		})
	}
	t.Cleanup(p.leave)
	if hello := p.call(protocol.MHello, protocol.HelloParams{Client: kind, Version: protocol.Version}); hello.Error != nil {
		t.Fatalf("hello failed: %v", hello.Error)
	}
	return p
}

func (p *oniPeer) call(method string, params any) protocol.Frame {
	p.t.Helper()
	p.seq++
	id := p.seq
	if err := p.enc.Encode(protocol.Frame{ID: &id, Method: method, Params: mustMarshal(p.t, params)}); err != nil {
		p.t.Fatal(err)
	}
	select {
	case f := <-p.responses:
		if *f.ID != id {
			p.t.Fatalf("wrong response identity for %s", method)
		}
		return f
	case <-time.After(5 * time.Second):
		p.t.Fatalf("%s did not answer", method)
	}
	return protocol.Frame{}
}

// next waits for n oni frames.
func (p *oniPeer) next(n int) []feature.Payload {
	p.t.Helper()
	var out []feature.Payload
	for range n {
		select {
		case raw := <-p.frames:
			f, err := feature.DecodeFrame(raw)
			if err != nil {
				p.t.Fatalf("the daemon sent a frame the contract refuses: %v: %s", err, raw)
			}
			out = append(out, f)
		case <-time.After(5 * time.Second):
			p.t.Fatalf("got %d of %d oni frames", len(out), n)
		}
	}
	return out
}

// quiet asserts no oni frame arrives for a moment.
func (p *oniPeer) quiet() {
	p.t.Helper()
	select {
	case raw := <-p.frames:
		p.t.Fatalf("unexpected oni frame %s", raw)
	case <-time.After(100 * time.Millisecond):
	}
}

func (p *oniPeer) open(renderer string) protocol.OniOpenResult {
	p.t.Helper()
	return oniResult[protocol.OniOpenResult](p.t, p.call(protocol.MOniOpen, protocol.OniOpenParams{Renderer: renderer}))
}

func decodeFrames(t *testing.T, raws []json.RawMessage) []feature.Payload {
	t.Helper()
	var out []feature.Payload
	for _, raw := range raws {
		f, err := feature.DecodeFrame(raw)
		if err != nil {
			t.Fatalf("%v: %s", err, raw)
		}
		out = append(out, f)
	}
	return out
}

func frameKinds(ps []feature.Payload) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Kind())
	}
	return out
}

var snapshotKinds = []string{feature.KindState, feature.KindCharacter, feature.KindMotion, feature.KindPlacement, feature.KindVoice}

func TestOniOpenAnswersTheSnapshotAndAdmitsOnePresenter(t *testing.T) {
	d := oniDaemon(t)
	surface := connectOni(t, d, protocol.ClientSurface, false)
	opened := surface.open(protocol.OniRendererNative)
	if !opened.Presenter || opened.Reason != "" {
		t.Fatalf("the first surface was not made presenter: %+v", opened)
	}
	frames := decodeFrames(t, opened.Frames)
	if got := frameKinds(frames); !slices.Equal(got, snapshotKinds) {
		t.Fatalf("snapshot kinds = %q, want %q", got, snapshotKinds)
	}
	if st := frames[0].(*feature.StateFrame); st.State != "idle" || st.Reason != "" {
		t.Fatalf("state = %+v", st)
	}
	if c := frames[1].(*feature.CharacterFrame); c.ID != protocol.OniDefaultCharacter || c.Name != "Oni" || len(c.Fallbacks) != 0 {
		t.Fatalf("character = %+v", c)
	}
	if m := frames[2].(*feature.MotionFrame); m.Reduce != protocol.OniReduceFollow || !m.Courtesy {
		t.Fatalf("motion = %+v", m)
	}

	second := connectOni(t, d, protocol.ClientSurface, false)
	if busy := second.open(protocol.OniRendererNative); busy.Presenter || busy.Reason != protocol.OniOpenBusy || len(busy.Frames) != len(snapshotKinds) {
		t.Fatalf("a second surface opened as %+v", busy)
	}
	cli := connectOni(t, d, protocol.ClientCLI, false)
	if f := cli.call(protocol.MOniOpen, protocol.OniOpenParams{Renderer: protocol.OniRendererNative}); errorCode(f) != protocol.CodeForbidden {
		t.Fatalf("a CLI connection asked to draw Oni: %+v", f.Error)
	}
	if watched := cli.open(protocol.OniRendererObserver); watched.Presenter || watched.Reason != protocol.OniOpenObserver {
		t.Fatalf("an observer opened as %+v", watched)
	}
	if ryoku := cli.open(protocol.OniRendererRyoku); ryoku.Presenter || ryoku.Reason != protocol.OniOpenUnavailable {
		t.Fatalf("a Ryoku renderer opened as %+v", ryoku)
	}
	if f := cli.call(protocol.MOniOpen, json.RawMessage(`{"renderer":"native","force":true}`)); errorCode(f) != protocol.CodeBadRequest {
		t.Fatalf("a malformed open answered %+v", f.Error)
	}

	oniResult[protocol.Empty](t, surface.call(protocol.MOniClose, protocol.Empty{}))
	if again := second.open(protocol.OniRendererNative); !again.Presenter {
		t.Fatalf("the presenter's close did not free the place: %+v", again)
	}
}

func TestOniMasterOffTellsThePresenterToExit(t *testing.T) {
	d := oniDaemon(t)
	surface := connectOni(t, d, protocol.ClientSurface, false)
	surface.open(protocol.OniRendererNative)
	app := connectOni(t, d, protocol.ClientApp, true)
	oniResult[protocol.Settings](t, app.call(protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(`{"oni":{"master":false}}`)}))
	got := surface.next(1)
	if st := got[0].(*feature.StateFrame); *st != (feature.StateFrame{State: "sleeping", Reason: presence.ReasonOff}) {
		t.Fatalf("turning Oni off sent %+v", st)
	}
	surface.quiet()

	late := connectOni(t, d, protocol.ClientSurface, false)
	opened := late.open(protocol.OniRendererNative)
	if opened.Presenter || opened.Reason != protocol.OniOpenOff {
		t.Fatalf("a surface opened while Oni is off as %+v", opened)
	}
	if frames := decodeFrames(t, opened.Frames); len(frames) != 1 || *frames[0].(*feature.StateFrame) != (feature.StateFrame{State: "sleeping", Reason: presence.ReasonOff}) {
		t.Fatalf("frames while off = %+v", frames)
	}
}

func TestOniSecondCharacterReachesThePresenterWithTheSameCapabilities(t *testing.T) {
	d := oniDaemon(t)
	installOniCharacter(t, "testmoth", "testmoth", nil)
	installOniCharacter(t, "testmoth", "partial", func(p map[string]any) {
		delete(p["animations"].(map[string]any)["states"].(map[string]any), "listening")
	})
	installOniCharacter(t, "soft-bare", "bare", nil)
	surface := connectOni(t, d, protocol.ClientSurface, false)
	surface.open(protocol.OniRendererNative)
	app := connectOni(t, d, protocol.ClientApp, true)
	before := capabilitiesBytes(t, app.call(protocol.MOniStatus, protocol.Empty{}))

	selectAndExpect := func(id string, want feature.CharacterFrame) {
		t.Helper()
		selected := app.call(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: id})
		if !bytes.Equal(capabilitiesBytes(t, selected), before) {
			t.Fatalf("capabilities changed with %s", id)
		}
		got := surface.next(1)
		c, ok := got[0].(*feature.CharacterFrame)
		if !ok || !reflect.DeepEqual(*c, want) {
			t.Fatalf("selecting %s sent %+v, want %+v", id, got[0], want)
		}
		if status := oniResult[protocol.OniStatus](t, surface.call(protocol.MOniStatus, protocol.Empty{})); status.Character != id {
			t.Fatalf("status names %q after selecting %s", status.Character, id)
		}
	}
	selectAndExpect("testmoth", feature.CharacterFrame{ID: "testmoth", Name: "Moth (test)"})
	selectAndExpect("partial", feature.CharacterFrame{ID: "partial", Name: "Moth (test)", Fallbacks: []protocol.OniFallback{
		{Slot: "animations.states.listening", Reason: profile.FallbackMissing, Used: profile.UsedStaticPose},
	}})
	bare, _ := profile.Load(os.DirFS(paths.CharactersDir()), "bare")
	if !bare.Valid || len(bare.Report.Fallbacks) == 0 {
		t.Fatalf("bare loads as %+v", bare)
	}
	var bareFallbacks []protocol.OniFallback
	for _, f := range bare.Report.Fallbacks {
		bareFallbacks = append(bareFallbacks, protocol.OniFallback{Slot: f.Slot, Reason: f.Reason, Used: f.Used})
	}
	if !slices.ContainsFunc(bareFallbacks, func(f protocol.OniFallback) bool {
		return f.Slot == "artwork" && f.Used == profile.UsedSilhouette
	}) {
		t.Fatalf("bare reports no silhouette: %+v", bareFallbacks)
	}
	selectAndExpect("bare", feature.CharacterFrame{ID: "bare", Name: "Moth (test)", Fallbacks: bareFallbacks})

	oniResult[protocol.OniStatus](t, app.call(protocol.MOniCharacterReset, protocol.Empty{}))
	if c := surface.next(1)[0].(*feature.CharacterFrame); c.ID != protocol.OniDefaultCharacter {
		t.Fatalf("reset sent %+v", c)
	}
	if f := surface.call(protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "testmoth"}); errorCode(f) != protocol.CodeForbidden {
		t.Fatalf("a surface selected a character: %+v", f.Error)
	}
	if f := surface.call(protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(`{"oni":{"inspect":true}}`)}); errorCode(f) != protocol.CodeForbidden {
		t.Fatalf("a surface changed the settings: %+v", f.Error)
	}
}

func TestOniCharacterAssetsAreTheDeclaredBytes(t *testing.T) {
	d := oniDaemon(t)
	installOniCharacter(t, "testmoth", "testmoth", nil)
	installOniCharacter(t, "broken-capabilities", "hostile", nil)
	surface := connectOni(t, d, protocol.ClientSurface, false)

	assets := oniResult[protocol.OniCharacterAssets](t, surface.call(protocol.MOniCharacterAssets, protocol.OniCharacterAssetsParams{ID: "testmoth"}))
	if got, want := slices.Sorted(maps.Keys(assets.Files)), []string{"moth.svg", "poses/alert.svg", "poses/rest.svg", profile.ProfileFile}; assets.ID != "testmoth" || !slices.Equal(got, want) {
		t.Fatalf("testmoth assets = %s %q, want %q", assets.ID, got, want)
	}
	for name, data := range assets.Files {
		disk, err := os.ReadFile(filepath.Join(paths.CharactersDir(), "testmoth", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, disk) {
			t.Errorf("%s differs from the declared file", name)
		}
	}
	shipped := oniResult[protocol.OniCharacterAssets](t, surface.call(protocol.MOniCharacterAssets, protocol.OniCharacterAssetsParams{ID: protocol.OniDefaultCharacter}))
	if want := profile.Default().Files; !reflect.DeepEqual(shipped.Files, want) {
		t.Fatalf("shipped assets = %q", slices.Sorted(maps.Keys(shipped.Files)))
	}
	if f := surface.call(protocol.MOniCharacterAssets, protocol.OniCharacterAssetsParams{ID: "hostile"}); errorCode(f) != protocol.CodeBadRequest {
		t.Fatalf("a rejected character's assets answered %+v", f.Error)
	}
	if f := surface.call(protocol.MOniCharacterAssets, protocol.OniCharacterAssetsParams{ID: "nobody"}); errorCode(f) != protocol.CodeNotFound {
		t.Fatalf("a missing character's assets answered %+v", f.Error)
	}
	if f := surface.call(protocol.MOniCharacterAssets, protocol.OniCharacterAssetsParams{ID: "../oni"}); errorCode(f) != protocol.CodeBadRequest {
		t.Fatalf("a malformed id answered %+v", f.Error)
	}
}

func TestOniSessionsDriveTheStateAndAWokenOniIsIdle(t *testing.T) {
	d := oniDaemon(t)
	surface := connectOni(t, d, protocol.ClientSurface, false)
	surface.open(protocol.OniRendererNative)
	d.Broadcast(protocol.EvtSessionInfo, protocol.SessionInfo{ID: "ask-1", State: protocol.StateWorking})
	if st := surface.next(1)[0].(*feature.StateFrame); st.State != "thinking" {
		t.Fatalf("a running turn sent %+v", st)
	}
	d.Broadcast(protocol.EvtSessionInfo, protocol.SessionInfo{ID: "ask-1", State: protocol.StateIdle})
	got := surface.next(2)
	if g, ok := got[0].(*feature.GestureFrame); !ok || g.ID != "done" {
		t.Fatalf("an ended turn sent %+v first", got[0])
	}
	if st, ok := got[1].(*feature.StateFrame); !ok || st.State != "idle" {
		t.Fatalf("an ended turn sent %+v second", got[1])
	}
	d.Broadcast(protocol.EvtSessionInfo, protocol.SessionInfo{ID: "ask-2", State: protocol.StateWorking})
	d.Broadcast(protocol.EvtSessionInfo, protocol.SessionInfo{ID: "ask-2", State: protocol.StateFailed})
	got = surface.next(3)
	if g, ok := got[1].(*feature.GestureFrame); !ok || g.ID != "error" {
		t.Fatalf("a failed turn sent %+v", got[1])
	}
	if st, ok := got[2].(*feature.StateFrame); !ok || st.State != "error" {
		t.Fatalf("a failed turn left state %+v", got[2])
	}
	oniResult[protocol.Empty](t, surface.call(protocol.MOniInteract, protocol.OniInteractParams{Kind: protocol.OniInteractPoke}))
	if st := surface.next(1)[0].(*feature.StateFrame); st.State != "idle" {
		t.Fatalf("a poke after an error sent %+v", st)
	}
	if f := surface.call(protocol.MOniInteract, protocol.OniInteractParams{Kind: "tickle"}); errorCode(f) != protocol.CodeBadRequest {
		t.Fatalf("an unknown interaction answered %+v", f.Error)
	}
}

func TestOniPlacementSurvivesADaemonRestart(t *testing.T) {
	d := oniDaemon(t)
	surface := connectOni(t, d, protocol.ClientSurface, false)
	surface.open(protocol.OniRendererNative)
	oniResult[protocol.Empty](t, surface.call(protocol.MOniPlace, protocol.OniPlaceParams{Output: "DP-1", FX: 0.3, FY: 0.6}))
	placed := surface.next(1)[0].(*feature.PlacementFrame)
	if placed.Corner != "" || placed.Active != "DP-1" || !reflect.DeepEqual(placed.Outputs, []feature.PlacementOutput{{ID: "DP-1", FX: 0.3, FY: 0.6}}) {
		t.Fatalf("placing sent %+v", placed)
	}
	if f := surface.call(protocol.MOniPlace, protocol.OniPlaceParams{Output: "DP-1", FX: 1.5}); errorCode(f) != protocol.CodeBadRequest {
		t.Fatalf("a spot off the output answered %+v", f.Error)
	}
	d.Shutdown()

	restarted := New(refuseRealDesktop(t, Deps{Version: "oni-wire-test"}))
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	restarted.mu.Lock()
	restarted.settings = settings
	restarted.mu.Unlock()
	again := connectOni(t, restarted, protocol.ClientSurface, false)
	frames := decodeFrames(t, again.open(protocol.OniRendererNative).Frames)
	got := frames[3].(*feature.PlacementFrame)
	if got.Corner != "" || got.Active != "DP-1" || !reflect.DeepEqual(got.Outputs, placed.Outputs) {
		t.Fatalf("after a restart placement = %+v", got)
	}

	// pinning with nothing open still saves
	cli := connectOni(t, restarted, protocol.ClientCLI, false)
	oniResult[protocol.Empty](t, again.call(protocol.MOniClose, protocol.Empty{}))
	oniResult[protocol.Empty](t, cli.call(protocol.MOniPlace, protocol.OniPlaceParams{Corner: protocol.OniCornerTopLeft}))
	if p, err := presence.ReadPlace(presence.PlacementFile()); err != nil || p.Corner != protocol.OniCornerTopLeft || len(p.Outputs) != 1 {
		t.Fatalf("pinned place = %+v, %v", p, err)
	}
}

func TestOniPresenterNeitherKeepsTheDaemonUpNorResetsItsClock(t *testing.T) {
	d := oniDaemon(t)
	surface := connectOni(t, d, protocol.ClientSurface, false)
	surface.open(protocol.OniRendererNative)
	if !d.Idle() {
		t.Fatal("a surface drawing Oni keeps the daemon up")
	}
	d.stateChanged()
	d.mu.RLock()
	timer := d.idle
	d.mu.RUnlock()
	oniResult[protocol.Empty](t, surface.call(protocol.MOniPlace, protocol.OniPlaceParams{Corner: protocol.OniCornerTopRight}))
	oniResult[protocol.Empty](t, surface.call(protocol.MOniInteract, protocol.OniInteractParams{Kind: protocol.OniInteractPoke}))
	if p, ok := surface.next(1)[0].(*feature.PlacementFrame); !ok || p.Corner != protocol.OniCornerTopRight {
		t.Fatalf("placing sent %+v", p)
	}
	d.mu.RLock()
	reset := d.idle != timer
	d.mu.RUnlock()
	if reset {
		t.Fatal("Oni's calls or frames reset the idle clock")
	}

	cli := connectOni(t, d, protocol.ClientCLI, false)
	if d.Idle() {
		t.Fatal("a CLI connection does not keep the daemon up")
	}
	cli.leave()
	if !d.Idle() {
		t.Fatal("the daemon stays up after the CLI left")
	}
	d.onIdleDeadline()
	select {
	case <-d.stop:
	default:
		t.Fatal("the daemon did not exit with only a presenter connected")
	}
}

func TestOniPresenterCallsAreRefusedToAgentTools(t *testing.T) {
	d := oniDaemon(t)
	place := mustMarshal(t, protocol.OniPlaceParams{Corner: protocol.OniCornerTopLeft})
	cases := map[string]json.RawMessage{
		protocol.MOniPlace:    place,
		protocol.MOniInteract: mustMarshal(t, protocol.OniInteractParams{Kind: protocol.OniInteractWake}),
		protocol.MOniOpen:     mustMarshal(t, protocol.OniOpenParams{Renderer: protocol.OniRendererObserver}),
		protocol.MOniClose:    mustMarshal(t, protocol.Empty{}),
	}
	for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
		for method, raw := range cases {
			if err := d.hostGate(info, method, raw); err == nil || protocol.AsError(err).Code != protocol.CodeForbidden {
				t.Errorf("an agent's tools (team %q) may call %s: %v", info.Team, method, err)
			}
		}
		for _, method := range []string{protocol.MOniCharacterSelect, protocol.MOniCharacterReset} {
			if err := d.hostGate(info, method, mustMarshal(t, protocol.Empty{})); err == nil && info.Team != "" {
				t.Errorf("a team agent may call %s", method)
			}
		}
	}
	if err := agentGate(protocol.MOniPlace); err == nil {
		t.Error("an agent's own socket may place Oni")
	}
	if _, err := os.Stat(presence.PlacementFile()); !os.IsNotExist(err) {
		t.Fatalf("a refused place wrote the file: %v", err)
	}
}

// orderedPeer keeps every line the daemon sends it, in the order sent.
type orderedPeer struct {
	mu      sync.Mutex
	log     []protocol.Frame
	marker  string
	opened  chan struct{}
	changed chan struct{}
	done    chan struct{}
}

func (p *orderedPeer) lines() []protocol.Frame {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.log)
}

func (p *orderedPeer) waitOpen() error {
	select {
	case <-p.opened:
		return nil
	default:
	}
	timer := time.NewTimer(writeTimeout)
	defer timer.Stop()
	select {
	case <-p.opened:
		return nil
	case <-p.done:
		return net.ErrClosed
	case <-timer.C:
		return context.DeadlineExceeded
	}
}

func (p *orderedPeer) fence(id string) error {
	timer := time.NewTimer(writeTimeout)
	defer timer.Stop()
	for {
		p.mu.Lock()
		reached := p.marker == id
		p.mu.Unlock()
		if reached {
			return nil
		}
		select {
		case <-p.changed:
		case <-p.done:
			return net.ErrClosed
		case <-timer.C:
			return context.DeadlineExceeded
		}
	}
}

// openOrdered says hello and opens the oni event as renderer without
// waiting in between, recording what comes back.
func openOrdered(t *testing.T, d *Daemon, renderer string) *orderedPeer {
	t.Helper()
	server, client := net.Pipe()
	c := newConnection(d, server)
	d.mu.Lock()
	d.clients[c] = struct{}{}
	d.mu.Unlock()
	stopped := make(chan struct{})
	go func() { c.serve(); close(stopped) }()
	p := &orderedPeer{opened: make(chan struct{}), changed: make(chan struct{}, 1), done: make(chan struct{})}
	go func() {
		defer close(p.done)
		dec := protocol.NewDecoder(client)
		for {
			f, err := dec.Next()
			if err != nil {
				return
			}
			marker := ""
			if f.IsEvent() && f.Event == protocol.EvtOni {
				payload, _ := feature.DecodeFrame(f.Data)
				if card, ok := payload.(*feature.ConsentFrame); ok && card.Feature == "reflex.routines" {
					marker = card.ID
				}
			}
			p.mu.Lock()
			p.log = append(p.log, f)
			if marker != "" {
				p.marker = marker
			}
			p.mu.Unlock()
			if f.ID != nil && *f.ID == 2 {
				close(p.opened)
			}
			if marker != "" {
				select {
				case p.changed <- struct{}{}:
				default:
				}
			}
		}
	}()
	t.Cleanup(func() {
		client.Close()
		c.close()
		<-stopped
	})
	enc := protocol.NewEncoder(client)
	hello, open := int64(1), int64(2)
	if err := enc.Encode(protocol.Frame{ID: &hello, Method: protocol.MHello, Params: mustMarshal(t, protocol.HelloParams{Client: protocol.ClientSurface, Version: protocol.Version})}); err != nil {
		t.Error(err)
		return p
	}
	if err := enc.Encode(protocol.Frame{ID: &open, Method: protocol.MOniOpen, Params: mustMarshal(t, protocol.OniOpenParams{Renderer: renderer})}); err != nil {
		t.Error(err)
	}
	return p
}

func TestOniOpenRepliesBeforeAnyFrameAndEndsOnTheCurrentState(t *testing.T) {
	d := oniDaemon(t)
	keeper := openOrdered(t, d, protocol.OniRendererObserver)
	if err := keeper.waitOpen(); err != nil {
		t.Fatal(err)
	}
	const peers = 64
	ready := make(chan *orderedPeer, peers)

	stop := make(chan struct{})
	var stopping sync.Once
	stopPublishing := func() { stopping.Do(func() { close(stop) }) }
	defer stopPublishing()
	type published struct {
		count int
		err   error
	}
	emitted := make(chan published, 1)
	go func() {
		result := published{}
		defer func() { emitted <- result }()
		watching := []*orderedPeer{keeper}
		marker := 0
		fence := func() error {
			for len(ready) > 0 {
				watching = append(watching, <-ready)
			}
			for _, p := range watching {
				if err := p.waitOpen(); err != nil {
					return err
				}
			}
			marker++
			id := "ordering-" + strconv.Itoa(marker)
			card := feature.ConsentFrame{ID: id, Feature: "reflex.routines", Class: "per-action", Destination: "local", Question: "Delivery fence", ExpiresMs: 1000}
			if err := d.oni.service.Load().Consent(card); err != nil {
				return err
			}
			for _, p := range watching {
				if err := p.fence(id); err != nil {
					return err
				}
			}
			return nil
		}
		states := []protocol.SessionState{protocol.StateWorking, protocol.StateWaiting, protocol.StateIdle, protocol.StateWorking, protocol.StateFailed}
		for i := 0; ; i++ {
			select {
			case <-stop:
				if result.count >= 600 {
					result.err = fence()
					return
				}
			default:
			}
			d.Broadcast(protocol.EvtSessionInfo, protocol.SessionInfo{ID: "ask-race", State: states[i%len(states)]})
			if i%7 == 0 {
				if svc := d.oni.service.Load(); svc != nil {
					svc.Interact(protocol.OniInteractPoke)
				}
			}
			result.count++
			// This checks ordering, not slow-reader eviction. Published markers
			// bound each batch below the event queue on single-core builders.
			if result.count%8 == 0 {
				if result.err = fence(); result.err != nil {
					return
				}
			}
			runtime.Gosched()
		}
	}()

	opened := make([]*orderedPeer, peers)
	var wg sync.WaitGroup
	for i := range peers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(i) * 250 * time.Microsecond)
			opened[i] = openOrdered(t, d, protocol.OniRendererObserver)
			ready <- opened[i]
		}()
	}
	wg.Wait()
	deadline := time.Now().Add(10 * time.Second)
	for _, p := range opened {
		for !slices.ContainsFunc(p.lines(), func(f protocol.Frame) bool { return f.ID != nil && *f.ID == 2 }) {
			if time.Now().After(deadline) {
				t.Fatal("an oni.open was not answered")
			}
			time.Sleep(time.Millisecond)
		}
	}
	stopPublishing()
	result := <-emitted
	if result.err != nil {
		t.Fatalf("presence delivery fence: %v", result.err)
	}
	if result.count < 600 {
		t.Fatalf("only %d session changes raced the opens", result.count)
	}

	current := *d.oni.service.Load().Snapshot()[0].(*feature.StateFrame)
	after := 0
	for i, p := range opened {
		log := p.lines()
		reply := slices.IndexFunc(log, func(f protocol.Frame) bool { return f.ID != nil && *f.ID == 2 })
		for j, f := range log[:reply] {
			if f.IsEvent() {
				t.Fatalf("peer %d: %s event %s arrived at %d, before the oni.open reply at %d", i, f.Event, f.Data, j, reply)
			}
		}
		result := oniResult[protocol.OniOpenResult](t, log[reply])
		frames := decodeFrames(t, result.Frames)
		state := *frames[0].(*feature.StateFrame)
		gestureSeq := uint64(0)
		for _, f := range log[reply+1:] {
			if !f.IsEvent() || f.Event != protocol.EvtOni {
				continue
			}
			after++
			p, err := feature.DecodeFrame(f.Data)
			if err != nil {
				t.Fatal(err)
			}
			switch frame := p.(type) {
			case *feature.StateFrame:
				if *frame == state {
					t.Fatalf("peer %d was sent the state it already had: %+v", i, frame)
				}
				state = *frame
			case *feature.GestureFrame:
				if frame.Seq <= gestureSeq {
					t.Fatalf("peer %d: gesture seq %d after %d", i, frame.Seq, gestureSeq)
				}
				gestureSeq = frame.Seq
			}
		}
		if state != current {
			t.Fatalf("peer %d ended on %+v, the service is at %+v", i, state, current)
		}
	}
	if after == 0 {
		t.Fatal("no frame followed any reply, so nothing raced")
	}
}
