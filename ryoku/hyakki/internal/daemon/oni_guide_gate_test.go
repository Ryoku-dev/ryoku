package daemon

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/protocol"
)

// one guide, keep or pick call with the params it carries, and whether the
// surface drawing Oni may send it
type guideGateCall struct {
	method    string
	raw       json.RawMessage
	presenter bool
}

func guideGateCalls(t *testing.T) []guideGateCall {
	return []guideGateCall{
		{protocol.MOniGuide, mustMarshal(t, protocol.OniGuideParams{Action: protocol.OniGuideStart, Goal: "save this file"}), false},
		{protocol.MOniGuide, mustMarshal(t, protocol.OniGuideParams{Action: protocol.OniGuideNext}), true},
		{protocol.MOniGuide, mustMarshal(t, protocol.OniGuideParams{Action: protocol.OniGuideStop}), true},
		{protocol.MOniGuide, mustMarshal(t, protocol.OniGuideParams{Action: protocol.OniGuideCircle}), true},
		{protocol.MOniKeep, mustMarshal(t, protocol.OniKeepParams{ID: "keep-1"}), false},
		{protocol.MOniSkip, mustMarshal(t, protocol.OniKeepParams{ID: "keep-1"}), true},
		{protocol.MOniPick, mustMarshal(t, protocol.OniPickParams{Action: protocol.OniPickArm}), false},
		{protocol.MOniPick, mustMarshal(t, protocol.OniPickParams{Action: protocol.OniPickArm, Text: "What does this do?"}), false},
		{protocol.MOniPick, mustMarshal(t, protocol.OniPickParams{Action: protocol.OniPickPoint, Output: "DP-1", X: 900, Y: 400}), true},
		{protocol.MOniPick, mustMarshal(t, protocol.OniPickParams{Action: protocol.OniPickCancel}), true},
		{protocol.MOniPick, mustMarshal(t, protocol.OniPickParams{Action: protocol.OniPickWait}), false},
	}
}

// The guide, keep and pick calls are the app's and the CLI's; the presenter
// sends only its card's Next, Stop and circle, Skip and a pick's click; an
// agent, a team socket, an observer, another surface, the bar and the hub
// send none of them, whatever params they carry.
func TestOniGuideCallsPassTheGateOnlyForTheAppTheCLIAndThePresentersActions(t *testing.T) {
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

	for _, method := range []string{protocol.MOniGuide, protocol.MOniKeep, protocol.MOniSkip, protocol.MOniPick} {
		if !oniHands[method] {
			t.Errorf("%s is not among the calls no agent makes", method)
		}
		if surfaceMethods[method] == (method == protocol.MOniKeep) {
			t.Errorf("a surface may call %s: %v", method, surfaceMethods[method])
		}
	}
	for _, c := range guideGateCalls(t) {
		for name, conn := range map[string]*connection{"the app": app, "the CLI": cli} {
			if err := d.mainGate(conn, c.method, c.raw); err != nil {
				t.Errorf("%s was refused %s %s: %v", name, c.method, c.raw, err)
			}
		}
		if err := d.mainGate(presenter, c.method, c.raw); (err == nil) != c.presenter || (err != nil && !forbidden(err)) {
			t.Errorf("the presenter calling %s %s: %v", c.method, c.raw, err)
		}
		for name, conn := range map[string]*connection{
			"a surface that is not the presenter": second,
			"an observer":                         observer,
			"the bar":                             bar,
			"the hub":                             hub,
			"a program that is not the app":       notApp,
			"a team session's connection":         team,
		} {
			if err := d.mainGate(conn, c.method, c.raw); !forbidden(err) {
				t.Errorf("%s called %s %s: %v", name, c.method, c.raw, err)
			}
		}
		for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
			if err := d.hostGate(info, c.method, c.raw); !forbidden(err) {
				t.Errorf("an agent's tools (team %q) called %s %s: %v", info.Team, c.method, c.raw, err)
			}
		}
		if err := agentGate(c.method); !forbidden(err) {
			t.Errorf("a team session's socket may call %s", c.method)
		}
	}
	// the presenter's action is read strictly: a malformed call is no
	// action it may send
	for _, c := range [][2]string{
		{protocol.MOniGuide, `{"action":"next","goal":"save"}`},
		{protocol.MOniGuide, `{"action":"Next"}`},
		{protocol.MOniPick, `{"action":"point"}`},
		{protocol.MOniSkip, `{}`},
	} {
		var refused *protocol.Error
		if err := d.mainGate(presenter, c[0], json.RawMessage(c[1])); !errors.As(err, &refused) || refused.Code != protocol.CodeBadRequest {
			t.Errorf("the presenter's %s %s passed the gate: %v", c[0], c[1], err)
		}
	}
	// a surface that stops presenting loses the calls with it
	d.oni.clientsMu.Lock()
	d.oni.presenter = nil
	d.oni.clientsMu.Unlock()
	for _, c := range guideGateCalls(t) {
		if err := d.mainGate(presenter, c.method, c.raw); !forbidden(err) {
			t.Errorf("a former presenter called %s %s: %v", c.method, c.raw, err)
		}
	}
}

// The answer modes and actions the wire names are the parser's and the
// frames' own.
func TestOniGuideWireNamesAreThePackagesOwn(t *testing.T) {
	if protocol.OniModeGuide != screenask.KindGuide || protocol.OniModeSelect != screenask.KindSelect {
		t.Errorf("answer modes %q %q, screen-ask kinds %q %q", protocol.OniModeGuide, protocol.OniModeSelect, screenask.KindGuide, screenask.KindSelect)
	}
	for _, a := range []string{protocol.OniGuideNext, protocol.OniGuideStop, protocol.OniGuideCircle} {
		if !slices.Contains([]string{feature.GuideActionNext, feature.GuideActionStop, feature.GuideActionCircle}, a) {
			t.Errorf("guide action %q is not a frame action", a)
		}
	}
	for _, a := range oniPresenterActions[protocol.MOniGuide] {
		if !slices.Contains(protocol.OniGuideActions, a) || a == protocol.OniGuideStart {
			t.Errorf("the presenter may send guide action %q", a)
		}
	}
	if got := oniPresenterActions[protocol.MOniPick]; !slices.Equal(got, []string{protocol.OniPickPoint, protocol.OniPickCancel}) {
		t.Errorf("the presenter may send pick actions %q", got)
	}
}

// With the guide off the served calls refuse and nothing runs: a start or a
// keep is disabled with its feature's reason, next and circle have no guide,
// a skip has no offer, a pick with nothing drawing Oni arms nothing;
// oni.spec reserves nothing, the snapshot carries no guide frame, the
// guide's capability rows say feature-off, and nothing is armed, watched or
// captured.
func TestOniGuideOffKeepsTheWireAsItWas(t *testing.T) {
	r := serveWire(t, true, localEyes)
	cli := dialSocket(t, protocol.ClientCLI)
	if g := r.d.Settings().Oni.Guide; g != (protocol.OniGuideSettings{}) {
		t.Fatalf("the guide settings are %#v", g)
	}
	for _, c := range guideGateCalls(t) {
		if _, ok := protocol.Lookup(c.method); !ok {
			t.Fatalf("%s is not registered", c.method)
		}
		var p struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal(c.raw, &p)
		want, reason := protocol.CodeConflict, ""
		switch {
		case c.method == protocol.MOniGuide && p.Action == protocol.OniGuideStart, c.method == protocol.MOniKeep:
			want, reason = protocol.CodeDisabled, feature.ReasonFeatureOff
		case c.method == protocol.MOniSkip:
			want = protocol.CodeNotFound
		case c.method == protocol.MOniGuide && p.Action == protocol.OniGuideStop,
			c.method == protocol.MOniPick && p.Action == protocol.OniPickCancel:
			want = ""
		case c.method == protocol.MOniPick && p.Action == protocol.OniPickArm:
			want, reason = protocol.CodeUnavailable, feature.PerchPresenterLost
		}
		err := cli.call(c.method, c.raw, nil)
		if code, got := reasonOf(err); code != want || got != reason || (want == "" && err != nil) {
			t.Errorf("%s %s answered %v, want %q %q", c.method, c.raw, err, want, reason)
		}
	}
	var spec protocol.OniSpecResult
	if err := cli.call(protocol.MOniSpec, protocol.Empty{}, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Calls == nil || len(spec.Calls) != 0 {
		t.Errorf("oni.spec reserves %q", spec.Calls)
	}
	if !slices.Contains(spec.Frames, feature.KindGuide) {
		t.Error("oni.spec does not list the guide frame")
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
	var status protocol.OniStatus
	if err := cli.call(protocol.MOniStatus, protocol.Empty{}, &status); err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, c := range status.Capabilities.Features {
		switch c.ID {
		case feature.Guide, feature.GuideKeep, feature.GuideHints:
			rows++
			if c.Allow || c.Reason != feature.ReasonFeatureOff {
				t.Errorf("with the guide off %s = %+v", c.ID, c)
			}
		case feature.Inspect:
			if !c.Allow {
				t.Errorf("looking at windows is %+v", c)
			}
		}
	}
	if rows != 3 {
		t.Fatalf("capabilities carry %d guide rows", rows)
	}
	if r.clock.Armed() != 0 {
		t.Fatal("a timer is armed with the guide off")
	}
	if r.detects.Load() != 0 || len(r.model.sent()) != 0 {
		t.Fatalf("the guide off read the desktop %d times and sent %d requests", r.detects.Load(), len(r.model.sent()))
	}
	surface.quiet()
}
