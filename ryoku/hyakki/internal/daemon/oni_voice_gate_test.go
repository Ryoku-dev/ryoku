package daemon

import (
	"encoding/json"
	"errors"
	"net"
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// the voice calls with params each would carry
func voiceCalls(t *testing.T) map[string]json.RawMessage {
	return map[string]json.RawMessage{
		protocol.MOniTalk:      mustMarshal(t, protocol.OniTalkParams{Action: protocol.OniVoiceStart}),
		protocol.MOniSpeak:     mustMarshal(t, protocol.OniSpeakParams{Action: protocol.OniVoiceStop}),
		protocol.MOniDictate:   mustMarshal(t, protocol.OniDictateParams{Action: protocol.OniVoiceStart}),
		protocol.MOniVoiceTest: mustMarshal(t, protocol.OniVoiceTestParams{}),
	}
}

// gateConn is a main-socket connection of kind, not served: the gate reads
// only who it is.
func gateConn(t *testing.T, d *Daemon, kind string, app bool, session string) *connection {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })
	c := newConnection(d, server)
	c.kind, c.app, c.session = kind, app, session
	return c
}

// The voice calls open the user's microphone or stop Oni speaking: the app,
// the CLI and the surface drawing Oni make them (the mic meter only the app
// and the CLI), and nobody else, whatever params they carry.
func TestOniVoiceCallsPassTheGateOnlyForTheAppTheCLIAndThePresenter(t *testing.T) {
	d := oniDaemon(t)
	app := gateConn(t, d, protocol.ClientApp, true, "")
	cli := gateConn(t, d, protocol.ClientCLI, false, "")
	presenter := gateConn(t, d, protocol.ClientSurface, false, "")
	second := gateConn(t, d, protocol.ClientSurface, false, "")
	observer := gateConn(t, d, protocol.ClientCLI, false, "")
	bar := gateConn(t, d, protocol.ClientBar, false, "")
	hub := gateConn(t, d, protocol.ClientHub, false, "")
	notApp := gateConn(t, d, protocol.ClientApp, false, "")
	team := gateConn(t, d, protocol.ClientCLI, false, "agent-1")
	d.oni.clientsMu.Lock()
	d.oni.clients = map[*connection]*oniClient{
		presenter: {renderer: protocol.OniRendererNative},
		second:    {renderer: protocol.OniRendererNative},
		observer:  {renderer: protocol.OniRendererObserver},
	}
	d.oni.presenter = presenter
	d.oni.clientsMu.Unlock()

	for method, raw := range voiceCalls(t) {
		if !oniHands[method] {
			t.Errorf("%s is not among the calls no agent makes", method)
		}
		for name, c := range map[string]*connection{"the app": app, "the CLI": cli} {
			if err := d.mainGate(c, method, raw); err != nil {
				t.Errorf("%s was refused %s: %v", name, method, err)
			}
		}
		err := d.mainGate(presenter, method, raw)
		if wantPresenter := method != protocol.MOniVoiceTest; (err == nil) != wantPresenter || (err != nil && !forbidden(err)) {
			t.Errorf("the presenter calling %s: %v", method, err)
		}
		for name, c := range map[string]*connection{
			"a surface that is not the presenter": second,
			"an observer":                         observer,
			"the bar":                             bar,
			"the hub":                             hub,
			"a program that is not the app":       notApp,
			"a team session's connection":         team,
		} {
			if err := d.mainGate(c, method, raw); !forbidden(err) {
				t.Errorf("%s called %s: %v", name, method, err)
			}
		}
		for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
			if err := d.hostGate(info, method, raw); !forbidden(err) {
				t.Errorf("an agent's tools (team %q) called %s: %v", info.Team, method, err)
			}
		}
		if err := agentGate(method); !forbidden(err) {
			t.Errorf("a team session's socket may call %s", method)
		}
	}
	// a surface that stops presenting loses the calls with it
	d.oni.clientsMu.Lock()
	d.oni.presenter = nil
	d.oni.clientsMu.Unlock()
	if err := d.mainGate(presenter, protocol.MOniTalk, voiceCalls(t)[protocol.MOniTalk]); !forbidden(err) {
		t.Errorf("a former presenter called oni.talk: %v", err)
	}
}

// With voice off the voice calls are served and refused with the
// decision's reason: nothing opens the microphone, oni.spec no longer
// reserves them, and the presenter gets the frames it always did.
func TestOniVoiceOffKeepsTheWireAsItWas(t *testing.T) {
	r := serveWire(t, true, localEyes)
	cli := dialSocket(t, protocol.ClientCLI)
	for method := range voiceCalls(t) {
		if _, ok := protocol.Lookup(method); !ok {
			t.Fatalf("%s is not registered", method)
		}
	}
	// oni.voice.test is left out: it runs with voice off by design, and
	// this daemon's microphone is the real one
	for _, method := range []string{protocol.MOniTalk, protocol.MOniDictate} {
		var refused *protocol.Error
		if err := cli.call(method, voiceCalls(t)[method], nil); !errors.As(err, &refused) || refused.Code != protocol.CodeDisabled {
			t.Errorf("%s answered %v", method, err)
		}
	}
	var stopped protocol.OniVoiceResult
	if err := cli.call(protocol.MOniSpeak, voiceCalls(t)[protocol.MOniSpeak], &stopped); err != nil || stopped.Phase != feature.VoiceIdle {
		t.Errorf("oni.speak stop answered %+v, %v", stopped, err)
	}
	var spec protocol.OniSpecResult
	if err := cli.call(protocol.MOniSpec, protocol.Empty{}, &spec); err != nil {
		t.Fatal(err)
	}
	for method := range voiceCalls(t) {
		if slices.Contains(spec.Calls, method) {
			t.Errorf("oni.spec still reserves %s", method)
		}
	}
	if !slices.Contains(spec.Frames, feature.KindLevel) {
		t.Error("oni.spec does not list the level frame")
	}
	surface := dialSocket(t, protocol.ClientSurface)
	open := surface.open(protocol.OniRendererNative)
	if !open.Presenter {
		t.Fatal("the surface is no presenter")
	}
	var got []string
	for _, raw := range open.Frames {
		p, err := feature.DecodeFrame(raw)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p.Kind())
	}
	if want := []string{feature.KindState, feature.KindCharacter, feature.KindMotion, feature.KindPlacement, feature.KindVoice}; !slices.Equal(got, want) {
		t.Fatalf("the snapshot is %q, want %q", got, want)
	}
	if voice := string(open.Frames[len(open.Frames)-1]); voice != `{"kind":"voice","data":{"phase":"idle"}}` {
		t.Fatalf("the voice frame is %s", voice)
	}
	var status struct {
		Capabilities json.RawMessage `json:"capabilities"`
	}
	if err := cli.call(protocol.MOniStatus, protocol.Empty{}, &status); err != nil {
		t.Fatal(err)
	}
	settings := protocol.EffectiveOni(r.d.Settings())
	want, err := json.Marshal(feature.Capabilities(settings))
	if err != nil {
		t.Fatal(err)
	}
	if string(status.Capabilities) != string(want) {
		t.Fatalf("capabilities\n%s\nwant\n%s", status.Capabilities, want)
	}
	if r.clock.Armed() != 0 {
		t.Fatal("a timer is armed with voice off")
	}
	surface.quiet()
}
