package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// servingOni is a daemon in temporary XDG dirs with Hyakki and Oni on,
// serving on its own Unix socket as it does for the user.
func servingOni(t *testing.T) *Daemon {
	t.Helper()
	isolateDaemon(t)
	shortRuntime(t)
	t.Setenv("HYAKKI_SOCKET", "")
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.Enabled = true
	settings.Oni.Master = true
	if err := config.Save(settings); err != nil {
		t.Fatal(err)
	}
	d := New(refuseRealDesktop(t, Deps{Version: "oni-strand-test"}))
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	t.Cleanup(func() {
		stop()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for !client.Reachable(context.Background(), paths.Socket()) {
		if time.Now().After(deadline) {
			t.Fatal("the daemon never listened")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return d
}

// socketPeer is one connection on the daemon's socket.
type socketPeer struct {
	t    *testing.T
	conn *client.Conn
}

func dialSocket(t *testing.T, kind string) *socketPeer {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: kind})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return &socketPeer{t: t, conn: conn}
}

func (p *socketPeer) call(method string, params, result any) error {
	p.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.conn.Call(ctx, method, params, result)
}

func (p *socketPeer) open(renderer string) protocol.OniOpenResult {
	p.t.Helper()
	var r protocol.OniOpenResult
	if err := p.call(protocol.MOniOpen, protocol.OniOpenParams{Renderer: renderer}, &r); err != nil {
		p.t.Fatal(err)
	}
	return r
}

func (p *socketPeer) arrived(seq uint64, skipped string) error {
	p.t.Helper()
	return p.call(protocol.MOniStrandArrived, protocol.OniStrandArrivedParams{Seq: seq, Skipped: skipped}, nil)
}

// frames waits for n oni frames, each decoded as strictly as the contract
// says.
func (p *socketPeer) frames(n int) []feature.Payload {
	p.t.Helper()
	var out []feature.Payload
	timeout := time.After(5 * time.Second)
	for len(out) < n {
		select {
		case e, ok := <-p.conn.Events():
			if !ok {
				p.t.Fatalf("the connection closed after %d of %d oni frames", len(out), n)
			}
			if e.Event != protocol.EvtOni {
				continue
			}
			f, err := feature.DecodeFrame(e.Data)
			if err != nil {
				p.t.Fatalf("the daemon sent a frame the contract refuses: %v: %s", err, e.Data)
			}
			out = append(out, f)
		case <-timeout:
			p.t.Fatalf("got %d of %d oni frames", len(out), n)
		}
	}
	return out
}

func (p *socketPeer) quiet() {
	p.t.Helper()
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case e := <-p.conn.Events():
			if e.Event == protocol.EvtOni {
				p.t.Fatalf("unexpected oni frame %s", e.Data)
			}
		case <-timeout:
			return
		}
	}
}

// strandLog records the seqs presence advanced or cancelled.
type strandLog struct {
	mu        sync.Mutex
	arrived   []uint64
	cancelled map[uint64]string
	changed   chan struct{}
}

func watchStrand(t *testing.T, d *Daemon) *strandLog {
	t.Helper()
	svc := d.oni.service.Load()
	if svc == nil {
		t.Fatal("presence is not running")
	}
	l := &strandLog{cancelled: map[uint64]string{}, changed: make(chan struct{}, 64)}
	svc.OnArrive(func(seq uint64, _ string) {
		l.mu.Lock()
		l.arrived = append(l.arrived, seq)
		l.mu.Unlock()
		l.changed <- struct{}{}
	})
	svc.OnCancel(func(seq uint64, why string) {
		l.mu.Lock()
		l.cancelled[seq] = why
		l.mu.Unlock()
		l.changed <- struct{}{}
	})
	return l
}

func (l *strandLog) advanced() []uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.arrived)
}

func (l *strandLog) waitCancel(t *testing.T, seq uint64) string {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		l.mu.Lock()
		why, ok := l.cancelled[seq]
		l.mu.Unlock()
		if ok {
			return why
		}
		select {
		case <-l.changed:
		case <-timeout:
			t.Fatalf("seq %d was never cancelled", seq)
		}
	}
}

var saveButton = feature.StrandTargetFrame{X: 0.25, Y: 0.5, W: 0.125, H: 0.0625, Label: "Save button"}

func strandTo(t *testing.T, d *Daemon) uint64 {
	t.Helper()
	seq, err := d.oniStrand(saveButton)
	if err != nil {
		t.Fatal(err)
	}
	return seq
}

func TestOniStaleArrivalsAdvanceNothingOverTheSocket(t *testing.T) {
	d := servingOni(t)
	if _, err := d.oniStrand(saveButton); !errors.Is(err, presence.ErrNoPresenter) {
		t.Fatalf("a strand with nothing open: %v", err)
	}
	surface := dialSocket(t, protocol.ClientSurface)
	if opened := surface.open(protocol.OniRendererNative); !opened.Presenter {
		t.Fatalf("the surface did not become presenter: %+v", opened)
	}
	log := watchStrand(t, d)
	var seqs []uint64
	for range 6 {
		seqs = append(seqs, strandTo(t, d))
	}
	if !slices.Equal(seqs, []uint64{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("seqs = %v", seqs)
	}
	for i, f := range surface.frames(6) {
		got, ok := f.(*feature.StrandTargetFrame)
		want := saveButton
		want.Seq = seqs[i]
		if !ok || *got != want {
			t.Fatalf("frame %d = %+v, want %+v", i, f, want)
		}
	}
	if err := surface.arrived(5, ""); err != nil {
		t.Fatalf("a stale arrival was refused: %v", err)
	}
	if got := log.advanced(); len(got) != 0 {
		t.Fatalf("the stale arrival 5 advanced %v", got)
	}
	if err := surface.arrived(6, ""); err != nil {
		t.Fatal(err)
	}
	if err := surface.arrived(6, ""); err != nil {
		t.Fatalf("a repeated arrival was refused: %v", err)
	}
	if err := surface.arrived(99, feature.SkipHidden); err != nil {
		t.Fatalf("an arrival for nothing pending was refused: %v", err)
	}
	if got := log.advanced(); !slices.Equal(got, []uint64{6}) {
		t.Fatalf("advanced %v, want only 6 once", got)
	}
	var bad *protocol.Error
	if err := surface.call(protocol.MOniStrandArrived, json.RawMessage(`{"seq":7,"skipped":"tired"}`), nil); !errors.As(err, &bad) || bad.Code != protocol.CodeBadRequest {
		t.Fatalf("a malformed arrival answered %v", err)
	}
	surface.quiet()
}

func TestOniArrivalIsThePresentersAlone(t *testing.T) {
	d := servingOni(t)
	surface := dialSocket(t, protocol.ClientSurface)
	surface.open(protocol.OniRendererNative)
	second := dialSocket(t, protocol.ClientSurface)
	if busy := second.open(protocol.OniRendererNative); busy.Presenter {
		t.Fatal("a second surface became presenter")
	}
	watcher := dialSocket(t, protocol.ClientCLI)
	watcher.open(protocol.OniRendererObserver)
	cli := dialSocket(t, protocol.ClientCLI)
	log := watchStrand(t, d)
	seq := strandTo(t, d)

	for name, p := range map[string]*socketPeer{"a non-presenter surface": second, "an observer": watcher, "a CLI": cli} {
		if err := p.arrived(seq, ""); !forbidden(err) {
			t.Errorf("%s reported the landing: %v, want forbidden", name, err)
		}
	}
	for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
		err := d.ToolHost(info).Call(context.Background(), protocol.MOniStrandArrived, protocol.OniStrandArrivedParams{Seq: seq}, nil)
		if !forbidden(err) {
			t.Errorf("an agent's tools (team %q) reported the landing: %v, want forbidden", info.Team, err)
		}
	}
	if err := agentGate(protocol.MOniStrandArrived); !forbidden(err) {
		t.Errorf("a team session's socket may report the landing: %v", err)
	}
	if got := log.advanced(); len(got) != 0 {
		t.Fatalf("a forged arrival advanced %v", got)
	}
	if err := surface.arrived(seq, ""); err != nil {
		t.Fatal(err)
	}
	if got := log.advanced(); !slices.Equal(got, []uint64{seq}) {
		t.Fatalf("the presenter's arrival advanced %v", got)
	}
}

// A team session's own socket refuses the arrival like any call that is not
// an index read.
func TestOniArrivalIsRefusedOnATeamSocket(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	crew := &boundTeams{handled: make(chan string, 8)}
	d := New(refuseRealDesktop(t, Deps{Teams: crew}))
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: filepath.Join(paths.Home(), "Teams", "one"), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.agentSandbox(team); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.closeAgentSockets)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, kind := range []string{protocol.ClientCLI, protocol.ClientSurface} {
		conn, err := client.Dial(ctx, client.Options{Client: kind, Socket: paths.AgentSocket(team.ID)})
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.Call(ctx, protocol.MOniStrandArrived, protocol.OniStrandArrivedParams{Seq: 1}, nil); !forbidden(err) {
			t.Errorf("a %s on a team socket reported a landing: %v, want forbidden", kind, err)
		}
		conn.Close()
	}
	select {
	case method := <-crew.handled:
		t.Fatalf("a refused call reached Teams: %s", method)
	default:
	}
}

func TestOniPresenterLeavingMidReachCancelsItsTarget(t *testing.T) {
	d := servingOni(t)
	watcher := dialSocket(t, protocol.ClientCLI)
	watcher.open(protocol.OniRendererObserver)
	if _, err := d.oniStrand(saveButton); !errors.Is(err, presence.ErrNoPresenter) {
		t.Fatalf("a strand with only an observer: %v", err)
	}
	log := watchStrand(t, d)

	surface := dialSocket(t, protocol.ClientSurface)
	surface.open(protocol.OniRendererNative)
	closed := strandTo(t, d)
	if err := surface.call(protocol.MOniClose, protocol.Empty{}, nil); err != nil {
		t.Fatal(err)
	}
	if why := log.waitCancel(t, closed); why != presence.CancelPresenterLost {
		t.Fatalf("oni.close mid-reach cancelled seq %d as %q", closed, why)
	}

	again := dialSocket(t, protocol.ClientSurface)
	again.open(protocol.OniRendererNative)
	dropped := strandTo(t, d)
	if dropped != closed+1 {
		t.Fatalf("seq after a new presenter = %d, want %d", dropped, closed+1)
	}
	again.conn.Close()
	if why := log.waitCancel(t, dropped); why != presence.CancelPresenterLost {
		t.Fatalf("a presenter that hung up mid-reach cancelled seq %d as %q", dropped, why)
	}
	if _, err := d.oniStrand(saveButton); !errors.Is(err, presence.ErrNoPresenter) {
		t.Fatalf("a strand after the presenter left: %v", err)
	}

	late := dialSocket(t, protocol.ClientSurface)
	late.open(protocol.OniRendererNative)
	if err := late.arrived(dropped, ""); err != nil {
		t.Fatal(err)
	}
	if got := log.advanced(); len(got) != 0 {
		t.Fatalf("a new presenter's report for a cancelled seq advanced %v", got)
	}
}

func TestOniStrandLeavesTheIdleClockAndCapabilitiesAlone(t *testing.T) {
	d := servingOni(t)
	surface := dialSocket(t, protocol.ClientSurface)
	surface.open(protocol.OniRendererNative)
	cli := dialSocket(t, protocol.ClientCLI)
	capabilities := func() json.RawMessage {
		var status struct {
			Capabilities json.RawMessage `json:"capabilities"`
		}
		if err := cli.call(protocol.MOniStatus, protocol.Empty{}, &status); err != nil {
			t.Fatal(err)
		}
		return status.Capabilities
	}
	before := capabilities()
	held := strandTo(t, d)
	if out := capabilities(); !bytes.Equal(out, before) {
		t.Fatalf("capabilities changed with a strand out:\n%s\n%s", out, before)
	}
	if err := surface.arrived(held, ""); err != nil {
		t.Fatal(err)
	}
	if landed := capabilities(); !bytes.Equal(landed, before) {
		t.Fatal("capabilities changed once the strand landed")
	}
	surface.frames(1)
	cli.conn.Close()
	deadline := time.Now().Add(5 * time.Second)
	for !d.Idle() {
		if time.Now().After(deadline) {
			t.Fatal("the daemon stays up with only a presenter connected")
		}
		time.Sleep(10 * time.Millisecond)
	}
	d.mu.RLock()
	timer := d.idle
	d.mu.RUnlock()

	seq := strandTo(t, d)
	if err := surface.arrived(seq, ""); err != nil {
		t.Fatal(err)
	}
	d.oni.service.Load().Retract()
	if got := frameKinds(surface.frames(2)); !slices.Equal(got, []string{feature.KindStrandTarget, feature.KindStrandRetract}) {
		t.Fatalf("frames = %q", got)
	}
	d.mu.RLock()
	reset := d.idle != timer
	d.mu.RUnlock()
	if reset || !d.Idle() {
		t.Fatal("a strand, its landing or its retract kept the daemon up or reset its clock")
	}
}
