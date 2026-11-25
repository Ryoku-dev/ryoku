package daemon

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// one call per Phase 16 method and action, with the params it carries
func phase16GateCalls(t *testing.T) []struct {
	method, action string
	raw            json.RawMessage
} {
	var calls []struct {
		method, action string
		raw            json.RawMessage
	}
	add := func(method, action string, p any) {
		calls = append(calls, struct {
			method, action string
			raw            json.RawMessage
		}{method, action, mustMarshal(t, p)})
	}
	for _, action := range protocol.OniRewriteActions {
		p := protocol.OniRewriteParams{Action: action}
		if action != protocol.OniRewriteStart {
			p.ID = "rewrite-1"
		}
		add(protocol.MOniRewrite, action, p)
	}
	add(protocol.MOniRecall, protocol.OniRecallAsk, protocol.OniRecallParams{Action: protocol.OniRecallAsk, Text: "what did I ask about eigenvalues"})
	add(protocol.MOniRecall, protocol.OniRecallKnow, protocol.OniRecallParams{Action: protocol.OniRecallKnow})
	add(protocol.MOniRecallImport, "", protocol.OniRecallImportParams{Paths: []string{"/home/u/export.zip"}})
	for _, action := range protocol.OniFocusActions {
		p := protocol.OniFocusParams{Action: action}
		if action == protocol.OniFocusArm {
			p.Minutes = 20
		}
		add(protocol.MOniFocus, action, p)
	}
	for _, action := range protocol.OniAssignmentActions {
		add(protocol.MOniAssignment, action, protocol.OniAssignmentParams{Action: action})
	}
	add(protocol.MOniAssignmentLog, "", protocol.Empty{})
	return calls
}

// The Phase 16 calls pass the gate by caller and action. The app and the
// CLI send every one. The presenter stops the focus guard and presses an
// assignment card's Next, Back, Done and Stop, never more: it never reads
// a selection, recalls, imports, arms the guard, brings a feed back, starts
// an assignment or reads its ledger. Another surface, an observer, the bar,
// the Hub, a team session and an agent's tools make none of them.
func TestOniPhase16CallsPassTheGateByCaller(t *testing.T) {
	d := oniDaemon(t)
	app := gateConn(t, d, protocol.ClientApp, true, "")
	cli := gateConn(t, d, protocol.ClientCLI, false, "")
	presenter := gateConn(t, d, protocol.ClientSurface, false, "")
	second := gateConn(t, d, protocol.ClientSurface, false, "")
	observer := gateConn(t, d, protocol.ClientCLI, false, "")
	appObserver := gateConn(t, d, protocol.ClientApp, true, "")
	bar := gateConn(t, d, protocol.ClientBar, false, "")
	hub := gateConn(t, d, protocol.ClientHub, false, "")
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

	methods := []string{protocol.MOniRewrite, protocol.MOniRecall, protocol.MOniRecallImport, protocol.MOniFocus, protocol.MOniAssignment, protocol.MOniAssignmentLog}
	for _, method := range methods {
		if !oniHands[method] {
			t.Errorf("%s is not among the calls no agent makes", method)
		}
		if _, ok := oniUser[method]; !ok {
			t.Errorf("%s has no caller rule", method)
		}
		if surfaceMethods[method] != (method == protocol.MOniFocus || method == protocol.MOniAssignment) {
			t.Errorf("a surface may call %s: %v", method, surfaceMethods[method])
		}
		if err := agentGate(method); !forbidden(err) {
			t.Errorf("a team session's socket may call %s", method)
		}
	}
	presenterMay := map[string][]string{
		protocol.MOniFocus:      {protocol.OniFocusStop},
		protocol.MOniAssignment: {protocol.OniAssignmentNext, protocol.OniAssignmentBack, protocol.OniAssignmentDone, protocol.OniAssignmentStop},
	}
	calls := phase16GateCalls(t)
	if len(calls) != 18 {
		t.Fatalf("the test covers %d calls", len(calls))
	}
	for _, c := range calls {
		for name, conn := range map[string]*connection{"the app": app, "the CLI": cli} {
			if err := d.mainGate(conn, c.method, c.raw); err != nil {
				t.Errorf("%s was refused %s %s: %v", name, c.method, c.action, err)
			}
		}
		may := slices.Contains(presenterMay[c.method], c.action)
		if err := d.mainGate(presenter, c.method, c.raw); may && err != nil || !may && !forbidden(err) {
			t.Errorf("the presenter sending %s %s: %v, may %v", c.method, c.action, err, may)
		}
		for name, conn := range map[string]*connection{
			"a surface that is not the presenter": second,
			"an observer":                         observer,
			"the app watching as an observer":     appObserver,
			"the bar":                             bar,
			"the hub":                             hub,
			"a team session's connection":         team,
		} {
			if err := d.mainGate(conn, c.method, c.raw); !forbidden(err) {
				t.Errorf("%s sent %s %s: %v", name, c.method, c.action, err)
			}
		}
		for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
			if err := d.hostGate(info, c.method, c.raw); !forbidden(err) {
				t.Errorf("an agent's tools (team %q) sent %s %s: %v", info.Team, c.method, c.action, err)
			}
		}
	}
	// what decodes for nobody is a bad request, never a pass
	for _, c := range [][2]string{
		{protocol.MOniRewrite, `{"action":"start","text":"the selection"}`},
		{protocol.MOniRecall, `{"action":"ask"}`},
		{protocol.MOniRecallImport, `{"paths":["../x.zip"]}`},
		{protocol.MOniFocus, `{"action":"arm","minutes":20,"untilApp":"Word"}`},
		{protocol.MOniAssignment, `{"action":"start","model":"x"}`},
		{protocol.MOniAssignmentLog, `{"all":true}`},
	} {
		for name, conn := range map[string]*connection{"the app": app, "the CLI": cli} {
			var perr *protocol.Error
			if err := d.mainGate(conn, c[0], json.RawMessage(c[1])); !errors.As(err, &perr) || perr.Code != protocol.CodeBadRequest {
				t.Errorf("%s sent %s %s: %v", name, c[0], c[1], err)
			}
		}
	}
	// a presenter that stops presenting gains nothing
	d.oni.clientsMu.Lock()
	d.oni.presenter = nil
	d.oni.clientsMu.Unlock()
	if err := d.mainGate(presenter, protocol.MOniFocus, mustMarshal(t, protocol.OniFocusParams{Action: protocol.OniFocusStop})); !forbidden(err) {
		t.Errorf("a former presenter stopped the guard: %v", err)
	}
}

// The recall, rewrite and assignment settings are the app's alone, in any
// spelling of the key or by replacing Oni's settings whole; the CLI still
// changes the rest of Oni's settings, the feed guard among them.
func TestOniPhase16SettingsAreTheAppsAlone(t *testing.T) {
	d := oniDaemon(t)
	set := func(c *connection, patch string) error {
		_, err := d.dispatch(t.Context(), c, protocol.MSettingsSet, mustMarshal(t, protocol.SettingsSetParams{Patch: json.RawMessage(patch)}))
		return err
	}
	for _, patch := range []string{
		`{"oni":{"recall":{"enabled":true}}}`, `{"oni":{"RECALL":{"vault":true}}}`, `{"ONI":{"Recall":{"enabled":false}}}`,
		`{"oni":{"rewrite":{"enabled":true}}}`, `{"oni":{"Rewrite":{"style":"email"}}}`,
		`{"oni":{"assignment":{"enabled":true}}}`, `{"oni":{"ASSIGNMENT":{"model":"openrouter/x"}}}`,
		`{"oni":{"master":true,"rewrite":null}}`,
	} {
		if err := set(&connection{}, patch); !forbidden(err) {
			t.Errorf("the CLI setting %s: %v, want forbidden", patch, err)
		}
		if !settingsForApp(json.RawMessage(patch)) {
			t.Errorf("%s is open to every program", patch)
		}
	}
	o := d.Settings().Oni
	if o.Recall.Enabled || o.Recall.Vault || o.Rewrite.Enabled || o.Rewrite.Style != protocol.OniRewriteGrammar || o.Assignment.Enabled || o.Assignment.Model != "" {
		t.Fatalf("a refused patch stored %+v %+v %+v", o.Recall, o.Rewrite, o.Assignment)
	}
	if err := set(&connection{app: true}, `{"oni":{"recall":{"enabled":true,"vault":true},"rewrite":{"enabled":true},"assignment":{"enabled":true}}}`); err != nil {
		t.Fatalf("the app turning Phase 16 on: %v", err)
	}
	if o := d.Settings().Oni; !o.Recall.Vault || !o.Rewrite.Enabled || !o.Assignment.Enabled {
		t.Fatalf("the app's patch stored %+v %+v %+v", o.Recall, o.Rewrite, o.Assignment)
	}
	for _, patch := range []string{`{"oni":{"feeds":{"guard":true}}}`, `{"oni":{"reflexSettings":{"notes":true}}}`, `{"oni":{"recallish":1,"master":true}}`} {
		if settingsForApp(json.RawMessage(patch)) {
			t.Errorf("%s is the app's alone", patch)
		}
	}
}
