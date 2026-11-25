package daemon

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/hands/page"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/hands/consent"
	"ryoku-rashin/internal/oni/hands/discipline"
	"ryoku-rashin/internal/oni/hands/loop"
	"ryoku-rashin/internal/oni/hands/proto"
	"ryoku-rashin/internal/protocol"
)

// one oni.hands call per action, with the params it carries
func handsGateCalls(t *testing.T) map[string]json.RawMessage {
	calls := map[string]json.RawMessage{}
	for _, action := range protocol.OniHandsActions {
		p := protocol.OniHandsParams{Action: action}
		switch action {
		case protocol.OniHandsAuthorize:
			p.Tab = "9F3A01C2"
		case protocol.OniHandsStart:
			p.Goal = "fill the contact form"
		case protocol.OniHandsSelect:
			p.Window = "17"
		}
		calls[action] = mustMarshal(t, p)
	}
	return calls
}

// The Hands calls pass the gate by action. The app sends every one, and is
// alone in forgetting the Hands browser, listing and picking live windows
// and clearing the task journal. The CLI opens the Hands browser, lists its
// tabs, stops a task, asks where Hands stands and reads the journal, never
// authorizing a tab, picking a window, starting, resuming or forgetting.
// The presenter authorizes, starts, stops and resumes, never more, and
// never starts a task on the separate desktop. The ledger is the app's and
// the CLI's. Another surface, an observer, the bar, the Hub, a program that
// is not the app, a team session and an agent's tools make no Hands call,
// a stop included.
func TestOniHandsCallsPassTheGateByAction(t *testing.T) {
	d := oniDaemon(t)
	app := gateConn(t, d, protocol.ClientApp, true, "")
	cli := gateConn(t, d, protocol.ClientCLI, false, "")
	presenter := gateConn(t, d, protocol.ClientSurface, false, "")
	second := gateConn(t, d, protocol.ClientSurface, false, "")
	observer := gateConn(t, d, protocol.ClientCLI, false, "")
	appObserver := gateConn(t, d, protocol.ClientApp, true, "")
	bar := gateConn(t, d, protocol.ClientBar, false, "")
	hub := gateConn(t, d, protocol.ClientHub, false, "")
	notApp := gateConn(t, d, protocol.ClientApp, false, "")
	team := gateConn(t, d, protocol.ClientCLI, false, "agent-1")
	d.oni.clientsMu.Lock()
	d.oni.clients = map[*connection]*oniClient{
		presenter:   {renderer: protocol.OniRendererNative},
		second:      {renderer: protocol.OniRendererNative},
		observer:    {renderer: protocol.OniRendererObserver},
		appObserver: {renderer: protocol.OniRendererObserver},
	}
	d.oni.presenter = presenter
	d.oni.clientsMu.Unlock()

	for _, method := range []string{protocol.MOniHands, protocol.MOniHandsLog} {
		if !oniHands[method] {
			t.Errorf("%s is not among the calls no agent makes", method)
		}
		if _, ok := oniPresenterActions[method]; ok {
			t.Errorf("%s is gated as a guide call", method)
		}
		if _, ok := oniVoice[method]; ok || oniReflex[method] {
			t.Errorf("%s is gated as a voice or reflex call", method)
		}
	}
	if !surfaceMethods[protocol.MOniHands] || surfaceMethods[protocol.MOniHandsLog] {
		t.Error("a surface must press Hands' buttons and never read the Hands ledger")
	}
	calls := handsGateCalls(t)
	if len(calls) != 12 {
		t.Fatalf("the test covers %d actions", len(calls))
	}
	cliMay := []string{protocol.OniHandsOpen, protocol.OniHandsTabs, protocol.OniHandsStop, protocol.OniHandsStatus, protocol.OniHandsTasks}
	presenterMay := []string{protocol.OniHandsAuthorize, protocol.OniHandsStart, protocol.OniHandsStop, protocol.OniHandsResume}
	// a start on the separate desktop is a per-task approval in the app:
	// the presenter never sends one, whatever the stage
	for _, scope := range []string{protocol.OniHandsScopeDesktop} {
		raw := mustMarshal(t, protocol.OniHandsParams{Action: protocol.OniHandsStart, Goal: "file the receipt", Scope: scope})
		if err := d.mainGate(presenter, protocol.MOniHands, raw); !forbidden(err) {
			t.Errorf("the presenter started Hands in scope %s: %v", scope, err)
		}
		if err := d.mainGate(app, protocol.MOniHands, raw); err != nil {
			t.Errorf("the app was refused a start in scope %s: %v", scope, err)
		}
	}
	for action, raw := range calls {
		if err := d.mainGate(app, protocol.MOniHands, raw); err != nil {
			t.Errorf("the app was refused %s: %v", action, err)
		}
		for name, c := range map[string]struct {
			conn *connection
			may  bool
		}{
			"the CLI":       {cli, slices.Contains(cliMay, action)},
			"the presenter": {presenter, slices.Contains(presenterMay, action)},
		} {
			err := d.mainGate(c.conn, protocol.MOniHands, raw)
			if c.may && err != nil {
				t.Errorf("%s was refused %s: %v", name, action, err)
			}
			if !c.may && !forbidden(err) {
				t.Errorf("%s sent %s: %v", name, action, err)
			}
		}
		for name, conn := range map[string]*connection{
			"a surface that is not the presenter": second,
			"an observer":                         observer,
			"the app watching as an observer":     appObserver,
			"the bar":                             bar,
			"the hub":                             hub,
			"a program that is not the app":       notApp,
			"a team session's connection":         team,
		} {
			if err := d.mainGate(conn, protocol.MOniHands, raw); !forbidden(err) {
				t.Errorf("%s sent %s: %v", name, action, err)
			}
		}
		for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
			if err := d.hostGate(info, protocol.MOniHands, raw); !forbidden(err) {
				t.Errorf("an agent's tools (team %q) sent %s: %v", info.Team, action, err)
			}
		}
	}
	for _, method := range []string{protocol.MOniHands, protocol.MOniHandsLog} {
		if err := agentGate(method); !forbidden(err) {
			t.Errorf("a team session's socket may call %s", method)
		}
	}
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`{}`)} {
		for name, conn := range map[string]*connection{"the app": app, "the CLI": cli} {
			if err := d.mainGate(conn, protocol.MOniHandsLog, raw); err != nil {
				t.Errorf("%s was refused the Hands ledger: %v", name, err)
			}
		}
		for name, conn := range map[string]*connection{"the presenter": presenter, "an observer": observer, "the bar": bar, "a team session's connection": team, "a program that is not the app": notApp} {
			if err := d.mainGate(conn, protocol.MOniHandsLog, raw); !forbidden(err) {
				t.Errorf("%s read the Hands ledger: %v", name, err)
			}
		}
		for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
			if err := d.hostGate(info, protocol.MOniHandsLog, raw); !forbidden(err) {
				t.Errorf("an agent's tools (team %q) read the Hands ledger: %v", info.Team, err)
			}
		}
	}
	// what decodes for nobody is a bad request, never a pass
	for _, raw := range []string{`{"action":"yes"}`, `{"action":"start"}`, `{"action":"stop","by":"voice"}`} {
		var perr *protocol.Error
		if err := d.mainGate(app, protocol.MOniHands, json.RawMessage(raw)); !errors.As(err, &perr) || perr.Code != protocol.CodeBadRequest {
			t.Errorf("the app sent %s: %v", raw, err)
		}
	}
	// a presenter that stops presenting gains nothing
	d.oni.clientsMu.Lock()
	d.oni.presenter = nil
	d.oni.clientsMu.Unlock()
	for _, action := range presenterMay {
		if err := d.mainGate(presenter, protocol.MOniHands, calls[action]); !forbidden(err) {
			t.Errorf("a former presenter sent %s: %v", action, err)
		}
	}
}

// oni.hands is the app's setting alone: no other program may store it, in
// any spelling of the key or by replacing Oni's settings whole, off
// included. The app stores each stage, and each stage allows its own Hands
// row alone. Other Oni settings stay open to the CLI.
func TestOniHandsSettingIsTheAppsAlone(t *testing.T) {
	d := oniDaemon(t)
	set := func(c *connection, patch string) error {
		_, err := d.dispatch(t.Context(), c, protocol.MSettingsSet, mustMarshal(t, protocol.SettingsSetParams{Patch: json.RawMessage(patch)}))
		return err
	}
	row := func(id string) protocol.OniCapability {
		for _, c := range d.oniStatus().Capabilities.Features {
			if c.ID == id {
				return c
			}
		}
		t.Fatalf("no %s row", id)
		return protocol.OniCapability{}
	}
	browser := func() protocol.OniCapability { return row(feature.HandsBrowser) }
	if c := browser(); c.Allow || c.Reason != feature.ReasonFeatureOff || c.Stage != feature.StageGuarded {
		t.Fatalf("hands.browser before any patch is %+v", c)
	}
	for _, patch := range []string{
		`{"oni":{"hands":"browser"}}`, `{"oni":{"hands":"off"}}`, `{"oni":{"Hands":"browser"}}`,
		`{"ONI":{"hands":"browser"}}`, `{"Oni":{"HANDS":"browser"}}`, `{"oni":{"master":true,"hands":"browser"}}`,
		`{"oni":{"hands":"app"}}`, `{"oni":{"hands":"desktop"}}`,
		`{"oni":null}`, `{"ONI":null,"oni":null}`, `{"oni":"browser"}`, `null`,
	} {
		if err := set(&connection{}, patch); !forbidden(err) {
			t.Errorf("the CLI setting %s: %v, want forbidden", patch, err)
		}
	}
	if d.Settings().Oni.Hands != protocol.OniHandsOff || browser().Allow {
		t.Fatalf("a refused patch stored hands %q", d.Settings().Oni.Hands)
	}
	ids := map[string]string{protocol.OniHandsBrowser: feature.HandsBrowser, protocol.OniHandsApp: feature.HandsApp, protocol.OniHandsDesktop: feature.HandsDesktop}
	for _, stage := range []string{protocol.OniHandsApp, protocol.OniHandsDesktop, protocol.OniHandsBrowser} {
		if err := set(&connection{app: true}, `{"oni":{"hands":"`+stage+`"}}`); err != nil {
			t.Fatalf("the app setting hands %s: %v", stage, err)
		}
		for s, id := range ids {
			c := row(id)
			if c.Stage != feature.StageGuarded || c.Allow != (s == stage) {
				t.Errorf("%s with the %s stage on is %+v", id, stage, c)
			}
		}
	}
	if c := browser(); !c.Allow || c.Reason != feature.ReasonAllowed {
		t.Fatalf("hands.browser after the app's patch is %+v", c)
	}
	// looking at windows is what Hands builds on
	if err := set(&connection{}, `{"oni":{"inspect":false}}`); err != nil {
		t.Fatalf("the CLI turning looking at windows off: %v", err)
	}
	if c := browser(); c.Allow || c.Reason != feature.ReasonFeatureOff {
		t.Fatalf("hands.browser with looking at windows off is %+v", c)
	}
	if err := set(&connection{app: true}, `{"oni":{"hands":"off"}}`); err != nil {
		t.Fatalf("the app turning hands off: %v", err)
	}
}

// settingsForApp matches keys as the settings decoder does, without regard
// to case, so no spelling reaches what only the app may change.
func TestSettingsForAppFoldsKeys(t *testing.T) {
	for _, patch := range []string{
		`{"TEAMS":{}}`, `{"Providers":{}}`, `{"MEMORY":{"use":true}}`, `{"ſkills":{}}`,
		`{"ASK":{"model":"ember/rashin-fast"}}`, `{"oni":{"hands":"off"}}`, `{"oni":{"HANDS":"off"}}`, `{"ONI":{"hands":"browser"}}`,
		`{"oni":null}`, `{"oni":[]}`,
	} {
		if !settingsForApp(json.RawMessage(patch)) {
			t.Errorf("%s is open to every program", patch)
		}
	}
	for _, patch := range []string{`{"oni":{"master":true}}`, `{"oni":{"inspect":false,"handsy":1}}`, `{"ask":{"model":"openrouter/x"}}`, `{"enabled":false}`} {
		if settingsForApp(json.RawMessage(patch)) {
			t.Errorf("%s is the app's alone", patch)
		}
	}
}

// With Hands off nothing of Hands is armed, watched or shown: no timer, no
// window read, no hands frame in the snapshot, and nothing reserved.
// Storing the browser stage, and turning it off again, arms nothing either.
func TestOniHandsOffArmsNothing(t *testing.T) {
	r := serveWire(t, true, localEyes)
	cli := dialSocket(t, protocol.ClientCLI)
	app := r.dialApp(t)
	surface := dialSocket(t, protocol.ClientSurface)
	open := surface.open(protocol.OniRendererNative)
	if !open.Presenter {
		t.Fatal("the surface is no presenter")
	}
	for _, raw := range open.Frames {
		p, err := feature.DecodeFrame(raw)
		if err != nil {
			t.Fatal(err)
		}
		if p.Kind() == feature.KindHands {
			t.Fatalf("the snapshot carries a hands frame with Hands off: %s", raw)
		}
	}
	var spec protocol.OniSpecResult
	if err := cli.call(protocol.MOniSpec, protocol.Empty{}, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Calls == nil || len(spec.Calls) != 0 {
		t.Errorf("oni.spec reserves %q", spec.Calls)
	}
	if err := cli.call(protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(`{"oni":{"hands":"browser"}}`)}, nil); !forbidden(err) {
		t.Fatalf("the CLI turned Hands on: %v", err)
	}
	if r.clock.Armed() != 0 || r.detects.Load() != 0 {
		t.Fatalf("Hands off armed %d timers and read the desktop %d times", r.clock.Armed(), r.detects.Load())
	}
	for _, stage := range []string{protocol.OniHandsBrowser, protocol.OniHandsOff} {
		var settings protocol.Settings
		if err := app.call(protocol.MSettingsSet, protocol.SettingsSetParams{Patch: json.RawMessage(`{"oni":{"hands":"` + stage + `"}}`)}, &settings); err != nil {
			t.Fatalf("the app setting hands %s: %v", stage, err)
		}
		if settings.Oni.Hands != stage {
			t.Fatalf("hands stored as %q", settings.Oni.Hands)
		}
		time.Sleep(50 * time.Millisecond)
		if r.clock.Armed() != 0 || r.detects.Load() != 0 || len(r.model.sent()) != 0 {
			t.Fatalf("hands %s armed %d timers, read the desktop %d times and sent %d requests", stage, r.clock.Armed(), r.detects.Load(), len(r.model.sent()))
		}
	}
	surface.quiet()
}

// The wire's Hands words are the ones the rules and the executor use:
// scopes, frame phases, pause reasons, ledger tools and reasons.
func TestOniHandsWireNamesAreThePackagesOwn(t *testing.T) {
	if !slices.Equal(protocol.OniHandsScopes, proto.Scopes) {
		t.Errorf("wire scopes %q, rules' %q", protocol.OniHandsScopes, proto.Scopes)
	}
	if want := []string{loop.FrameWorking, loop.FrameWaiting, loop.FramePaused}; !slices.Equal(protocol.OniHandsPhases, want) {
		t.Errorf("wire phases %q, the loop's frames %q", protocol.OniHandsPhases, want)
	}
	ends := []string{
		loop.EndDone, loop.EndAsked, loop.EndAnswered, loop.EndStopped, loop.EndBudget,
		loop.EndRepeat, loop.EndUnconfirmed, loop.EndRefused, loop.EndExpired, loop.EndAway,
		loop.EndTimeout, loop.EndFailed, loop.EndClosed, loop.EndUnread,
	}
	if !slices.Equal(protocol.OniHandsEnds, ends) {
		t.Errorf("wire ends %q, the loop's %q", protocol.OniHandsEnds, ends)
	}
	for _, pause := range []string{loop.PauseNavigation, loop.PauseWindow, loop.PauseFocus} {
		if !slices.Contains(protocol.OniHandsBrowserPauses, pause) {
			t.Errorf("the loop pauses a tab for %q, which no frame may show", pause)
		}
	}
	for _, kind := range []page.Kind{page.Hidden, page.Dialog, page.Popup, page.Download} {
		if !slices.Contains(protocol.OniHandsBrowserPauses, string(kind)) {
			t.Errorf("the tab's %q event has no pause", kind)
		}
	}
	for _, pause := range []discipline.Pause{
		discipline.PauseFocus, discipline.PauseWindow, discipline.PauseHidden, discipline.PauseOutput, discipline.PauseTitle,
		discipline.PauseGeometry, discipline.PauseCovered, discipline.PauseSelf, discipline.PausePrivate, discipline.PauseKey, discipline.PausePresenter,
	} {
		if !slices.Contains(protocol.OniHandsPauses, string(pause)) {
			t.Errorf("the window check pauses for %q, which the wire does not name", pause)
		}
	}
	seen := map[string]bool{}
	for _, pause := range protocol.OniHandsPauses {
		if seen[pause] {
			t.Errorf("pause %q is named twice", pause)
		}
		seen[pause] = true
	}
	at := time.Unix(1_800_000_000, 0)
	for _, tool := range proto.Tools {
		for _, reason := range []consent.Reason{"", consent.ReasonRiskyLabel, consent.ReasonPrivate, consent.ReasonOtherOrigin, consent.ReasonUnknownRole} {
			if row := (protocol.OniHandsRow{At: at, Task: "t-1", Step: 1, Tool: tool, Outcome: protocol.OniHandsOutcomeHandoff, Reason: string(reason), Ms: 12}); !protocol.ValidOniHandsRow(row) {
				t.Errorf("a ledger row cannot hold %+v", row)
			}
		}
	}
	for _, row := range []protocol.OniHandsRow{
		{At: at, Task: "t-1", Tool: "fill", Outcome: protocol.OniHandsOutcomeDone, Reason: "Ada Lovelace"},
		{At: at, Task: "t-1", Tool: "Send", Outcome: protocol.OniHandsOutcomeDone},
		{At: at, Task: "t-1", Tool: "fill", Outcome: "typed"},
		{Task: "t-1", Tool: "fill", Outcome: protocol.OniHandsOutcomeDone},
		{At: at, Task: "t-1", Tool: "fill", Outcome: protocol.OniHandsOutcomeDone, Ms: -1},
	} {
		if protocol.ValidOniHandsRow(row) {
			t.Errorf("a ledger row holds %+v", row)
		}
	}
}
