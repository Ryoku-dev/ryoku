package daemon

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// mcpConn is a connection from the MCP bridge for the program name
func mcpConn(t *testing.T, d *Daemon, name string) *connection {
	t.Helper()
	c := gateConn(t, d, protocol.ClientMCP, false, "")
	c.name = name
	return c
}

// Listing the optional parts and reading Oni's activity is open to the
// app, the CLI and the Hub; removing a part and deleting activity are the
// app's alone. An observer, the bar, a surface, a team session, an agent's
// tools and the MCP bridge make none of them.
func TestPhase17CallsPassTheGateByCaller(t *testing.T) {
	d := oniDaemon(t)
	app := gateConn(t, d, protocol.ClientApp, true, "")
	cli := gateConn(t, d, protocol.ClientCLI, false, "")
	hub := gateConn(t, d, protocol.ClientHub, false, "")
	bar := gateConn(t, d, protocol.ClientBar, false, "")
	surface := gateConn(t, d, protocol.ClientSurface, false, "")
	observer := gateConn(t, d, protocol.ClientCLI, false, "")
	appObserver := gateConn(t, d, protocol.ClientApp, true, "")
	team := gateConn(t, d, protocol.ClientCLI, false, "agent-1")
	bridge := mcpConn(t, d, "Claude Code")
	d.oni.clientsMu.Lock()
	d.oni.clients = map[*connection]*oniClient{
		observer:    {renderer: protocol.OniRendererObserver},
		appObserver: {renderer: protocol.OniRendererObserver},
	}
	d.oni.clientsMu.Unlock()
	calls := []struct {
		method  string
		raw     json.RawMessage
		appOnly bool
	}{
		{protocol.MComponents, mustMarshal(t, protocol.Empty{}), false},
		{protocol.MComponentRemove, mustMarshal(t, protocol.ComponentRemoveParams{ID: "stt:base.en"}), true},
		{protocol.MOniActivity, mustMarshal(t, protocol.Empty{}), false},
		{protocol.MOniActivityClear, mustMarshal(t, protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityAll}}), true},
	}
	for _, c := range calls {
		if _, ok := protocol.Lookup(c.method); !ok {
			t.Fatalf("%s is not registered", c.method)
		}
		if appOnly, ok := partsAndActivity[c.method]; !ok || appOnly != c.appOnly {
			t.Errorf("%s has the caller rule %v %v", c.method, appOnly, ok)
		}
		if err := d.mainGate(app, c.method, c.raw); err != nil {
			t.Errorf("the app was refused %s: %v", c.method, err)
		}
		for name, conn := range map[string]*connection{"the CLI": cli, "the Hub": hub} {
			if err := d.mainGate(conn, c.method, c.raw); c.appOnly && !forbidden(err) || !c.appOnly && err != nil {
				t.Errorf("%s calling %s: %v", name, c.method, err)
			}
		}
		for name, conn := range map[string]*connection{
			"the bar":                         bar,
			"a surface":                       surface,
			"an observer":                     observer,
			"the app watching as an observer": appObserver,
			"a team session's connection":     team,
			"the MCP bridge":                  bridge,
		} {
			if err := d.mainGate(conn, c.method, c.raw); !forbidden(err) {
				t.Errorf("%s called %s: %v", name, c.method, err)
			}
		}
		if err := agentGate(c.method); !forbidden(err) {
			t.Errorf("a team session's socket may call %s", c.method)
		}
		for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
			if err := d.hostGate(info, c.method, c.raw); !forbidden(err) {
				t.Errorf("an agent's tools (team %q) called %s: %v", info.Team, c.method, err)
			}
		}
	}
	// a line on the pill is the app's or an approved program's; the
	// bridge's state is the app's and the CLI's to read
	note := mustMarshal(t, protocol.OniNotifyParams{Text: "Build finished"})
	if err := d.mainGate(app, protocol.MOniNotify, note); err != nil {
		t.Errorf("the app was refused oni.notify: %v", err)
	}
	for name, conn := range map[string]*connection{"the CLI": cli, "the Hub": hub, "the bar": bar, "a surface": surface, "a team session's connection": team} {
		if err := d.mainGate(conn, protocol.MOniNotify, note); !forbidden(err) {
			t.Errorf("%s called oni.notify: %v", name, err)
		}
	}
	for name, conn := range map[string]*connection{"the app": app, "the CLI": cli} {
		if err := d.mainGate(conn, protocol.MOniMCP, nil); err != nil {
			t.Errorf("%s was refused oni.mcp: %v", name, err)
		}
	}
	for name, conn := range map[string]*connection{"the Hub": hub, "the bar": bar, "a surface": surface, "an observer": observer, "a team session's connection": team} {
		if err := d.mainGate(conn, protocol.MOniMCP, nil); !forbidden(err) {
			t.Errorf("%s called oni.mcp: %v", name, err)
		}
	}
	for _, method := range []string{protocol.MOniNotify, protocol.MOniMCP} {
		if err := d.hostGate(protocol.SessionInfo{ID: "ask-1", Kind: protocol.KindAsk}, method, nil); !forbidden(err) {
			t.Errorf("an agent's tools called %s: %v", method, err)
		}
	}
}

// The MCP bridge reaches nothing while it is off, only its own state
// until Settings approves its program, and then only the calls behind the
// tools turned on, each with the one action its tool stands for.
func TestPhase17GateLetsTheMCPBridgeReachOnlyItsTools(t *testing.T) {
	d := oniDaemon(t)
	bridge := mcpConn(t, d, "Claude Code")
	stranger := mcpConn(t, d, "Zed")
	due := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	tools := map[string]struct {
		method string
		raw    json.RawMessage
	}{
		protocol.OniMCPRemindersList: {protocol.MOniReminders, mustMarshal(t, protocol.OniRemindersParams{Action: protocol.OniRemindersList})},
		protocol.OniMCPRemindersAdd:  {protocol.MOniReminders, mustMarshal(t, protocol.OniRemindersParams{Action: protocol.OniRemindersAdd, Kind: protocol.OniReminderKindReminder, Due: due, Text: "Stretch"})},
		protocol.OniMCPNotesSearch:   {protocol.MMemorySearch, mustMarshal(t, protocol.MemorySearchParams{Query: "dentist"})},
		protocol.OniMCPNotesAdd:      {protocol.MMemoryImport, mustMarshal(t, protocol.MemoryImportParams{Text: "Prefers short answers."})},
		protocol.OniMCPFocusStatus:   {protocol.MOniFocus, mustMarshal(t, protocol.OniFocusParams{Action: protocol.OniFocusStatus})},
		protocol.OniMCPNotify:        {protocol.MOniNotify, mustMarshal(t, protocol.OniNotifyParams{Text: "Build finished"})},
		protocol.OniMCPScreenAsk:     {protocol.MOniAsk, mustMarshal(t, protocol.OniAskParams{Text: "Where is Save?"})},
	}
	if len(tools) != len(protocol.OniMCPTools) {
		t.Fatalf("the test covers %d of %d tools", len(tools), len(protocol.OniMCPTools))
	}
	// never another call or action, whatever is on
	never := []struct {
		method string
		raw    json.RawMessage
	}{
		{protocol.MSettingsSet, mustMarshal(t, protocol.SettingsSetParams{Patch: json.RawMessage(`{"oni":{"mcp":{"clients":["Zed"]}}}`)})},
		{protocol.MSettingsGet, nil},
		{protocol.MSubscribe, nil},
		{protocol.MMemoryAdd, mustMarshal(t, protocol.MemoryAddParams{Text: "I am the user."})},
		{protocol.MMemoryForgetAll, mustMarshal(t, protocol.MemoryForgetAllParams{})},
		{protocol.MOniReminders, mustMarshal(t, protocol.OniRemindersParams{Action: protocol.OniRemindersCancelAll})},
		{protocol.MOniReminders, mustMarshal(t, protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: "0123456789abcdef"})},
		{protocol.MOniFocus, mustMarshal(t, protocol.OniFocusParams{Action: protocol.OniFocusArm, Minutes: 20})},
		{protocol.MOniFocus, mustMarshal(t, protocol.OniFocusParams{Action: protocol.OniFocusBack})},
		{protocol.MOniConsentAnswer, mustMarshal(t, protocol.OniConsentAnswerParams{ID: "c-1", Answer: "yes"})},
		{protocol.MOniInspect, nil},
		{protocol.MOniRewrite, mustMarshal(t, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})},
		{protocol.MOniHands, mustMarshal(t, protocol.OniHandsParams{Action: protocol.OniHandsStatus})},
		{protocol.MOniActivity, nil},
		{protocol.MOniActivityClear, mustMarshal(t, protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityAll}})},
		{protocol.MComponents, nil},
		{protocol.MComponentRemove, mustMarshal(t, protocol.ComponentRemoveParams{ID: "stt:base.en"})},
		{protocol.MAccountStatus, nil},
		{protocol.MTeamsList, nil},
		{protocol.MAsk, mustMarshal(t, protocol.AskParams{Question: "hi"})},
	}
	set := func(s protocol.OniMCPSettings) {
		settings := d.Settings()
		settings.Oni.MCP = s
		d.mu.Lock()
		d.settings = settings
		d.mu.Unlock()
	}
	all := map[string]bool{}
	for _, id := range protocol.OniMCPTools {
		all[id] = true
	}
	// off: nothing, not even its own state
	set(protocol.OniMCPSettings{Tools: all, Clients: []string{"Claude Code"}})
	if err := d.mainGate(bridge, protocol.MOniMCP, nil); !forbidden(err) {
		t.Errorf("the bridge read its state while off: %v", err)
	}
	for id, c := range tools {
		if err := d.mainGate(bridge, c.method, c.raw); !forbidden(err) {
			t.Errorf("%s passed while the bridge is off: %v", id, err)
		}
	}
	// on, every tool alone and all at once, for the approved program
	combos := []map[string]bool{{}, all}
	for _, id := range protocol.OniMCPTools {
		combos = append(combos, map[string]bool{id: true})
	}
	for _, on := range combos {
		set(protocol.OniMCPSettings{Enabled: true, Tools: on, Clients: []string{"Claude Code"}})
		for name, conn := range map[string]*connection{"the approved program": bridge, "a waiting program": stranger} {
			if err := d.mainGate(conn, protocol.MOniMCP, nil); err != nil {
				t.Errorf("%s was refused its state: %v", name, err)
			}
		}
		for id, c := range tools {
			err := d.mainGate(bridge, c.method, c.raw)
			if on[id] && err != nil || !on[id] && !forbidden(err) {
				t.Errorf("with %v on, %s: %v", on, id, err)
			}
			if err := d.mainGate(stranger, c.method, c.raw); !forbidden(err) {
				t.Errorf("a program nobody approved used %s: %v", id, err)
			}
		}
		for _, c := range never {
			if err := d.mainGate(bridge, c.method, c.raw); !forbidden(err) {
				t.Errorf("with %v on, the bridge called %s %s: %v", on, c.method, c.raw, err)
			}
		}
	}
	// what decodes for nobody is a bad request, never a pass
	set(protocol.OniMCPSettings{Enabled: true, Tools: all, Clients: []string{"Claude Code"}})
	for _, raw := range []string{`{"action":"add","kind":"reminder"}`, `{"action":"snooze"}`} {
		if err := d.mainGate(bridge, protocol.MOniReminders, json.RawMessage(raw)); codeOf(err) != protocol.CodeBadRequest {
			t.Errorf("the bridge sent oni.reminders %s: %v", raw, err)
		}
	}
	// the bridge never holds the daemon up
	d.mu.Lock()
	d.clients[bridge] = struct{}{}
	d.mu.Unlock()
	if !d.restingBesideMemory() {
		t.Error("the bridge's connection kept the daemon up")
	}
}

// hello admits the MCP bridge only while it is on and only with a program
// name; no other kind names one, and an unknown kind is still refused. A
// refused hello ends the connection with nothing stored on it; the refusal
// may close the line before its answer is read, so a code is checked only
// when the answer arrives.
func TestPhase17GateHelloAdmitsTheMCPBridgeOnlyWhileItIsOn(t *testing.T) {
	d := oniDaemon(t)
	hello := func(p protocol.HelloParams, refusal string) {
		t.Helper()
		server, client := net.Pipe()
		c := newConnection(d, server)
		d.mu.Lock()
		d.clients[c] = struct{}{}
		d.mu.Unlock()
		stopped := make(chan struct{})
		go func() { c.serve(); close(stopped) }()
		t.Cleanup(func() { client.Close(); c.close(); <-stopped })
		id := int64(1)
		if err := protocol.NewEncoder(client).Encode(protocol.Frame{ID: &id, Method: protocol.MHello, Params: mustMarshal(t, p)}); err != nil {
			t.Fatal(err)
		}
		f, err := protocol.NewDecoder(client).Next()
		if refusal == "" {
			if err != nil || f.Error != nil {
				t.Fatalf("hello %+v was refused: %v %+v", p, err, f.Error)
			}
			c.mu.RLock()
			kind, name := c.kind, c.name
			c.mu.RUnlock()
			if kind != p.Client || name != p.Name {
				t.Errorf("hello stored %q %q", kind, name)
			}
			return
		}
		if err == nil && (f.Error == nil || f.Error.Code != refusal) {
			t.Errorf("hello %+v answered %+v, want %s", p, f.Error, refusal)
		}
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Fatalf("hello %+v left the connection open", p)
		}
		c.mu.RLock()
		kind, name := c.kind, c.name
		c.mu.RUnlock()
		if kind != "" || name != "" {
			t.Errorf("a refused hello %+v stored %q %q", p, kind, name)
		}
	}
	bridge := protocol.HelloParams{Client: protocol.ClientMCP, Version: protocol.Version, Name: "Claude Code"}
	hello(bridge, protocol.CodeForbidden)
	settings := d.Settings()
	settings.Oni.MCP.Enabled = true
	d.mu.Lock()
	d.settings = settings
	d.mu.Unlock()
	hello(bridge, "")
	for _, p := range []protocol.HelloParams{
		{Client: protocol.ClientMCP, Version: protocol.Version},
		{Client: protocol.ClientMCP, Version: protocol.Version, Name: " padded "},
		{Client: protocol.ClientMCP, Version: protocol.Version, Name: "two\nlines"},
		{Client: protocol.ClientCLI, Version: protocol.Version, Name: "Claude Code"},
		{Client: "agent", Version: protocol.Version},
		{Client: "MCP", Version: protocol.Version, Name: "Claude Code"},
	} {
		hello(p, protocol.CodeBadRequest)
	}
	hello(protocol.HelloParams{Client: protocol.ClientCLI, Version: protocol.Version}, "")
}

// The MCP bridge's settings and the tour's state are the app's alone, in any
// spelling of the key or by replacing Oni's settings whole.
func TestPhase17SettingsAreTheAppsAlone(t *testing.T) {
	d := oniDaemon(t)
	set := func(c *connection, patch string) error {
		_, err := d.dispatch(t.Context(), c, protocol.MSettingsSet, mustMarshal(t, protocol.SettingsSetParams{Patch: json.RawMessage(patch)}))
		return err
	}
	for _, patch := range []string{
		`{"oni":{"mcp":{"enabled":true}}}`, `{"oni":{"MCP":{"clients":["Zed"]}}}`, `{"ONI":{"Mcp":{"tools":{"notify":true}}}}`,
		`{"oni":{"master":true,"mcp":null}}`,
		`{"onboarding":{"seen":true}}`, `{"Onboarding":{"done":true}}`, `{"ONBOARDING":null}`,
	} {
		if err := set(&connection{}, patch); !forbidden(err) {
			t.Errorf("the CLI setting %s: %v, want forbidden", patch, err)
		}
		if !settingsForApp(json.RawMessage(patch)) {
			t.Errorf("%s is open to every program", patch)
		}
	}
	if s := d.Settings(); s.Oni.MCP.Enabled || s.Oni.MCP.Tools != nil || s.Oni.MCP.Clients != nil || s.Onboarding != (protocol.OnboardingSettings{}) {
		t.Fatalf("a refused patch stored %+v %+v", s.Oni.MCP, s.Onboarding)
	}
	if err := set(&connection{app: true}, `{"oni":{"mcp":{"enabled":true,"tools":{"notify":true},"clients":["Claude Code"]}},"onboarding":{"seen":true}}`); err != nil {
		t.Fatalf("the app's patch: %v", err)
	}
	if s := d.Settings(); !s.Oni.MCP.Enabled || !s.Oni.MCP.Tools[protocol.OniMCPNotify] || len(s.Oni.MCP.Clients) != 1 || !s.Onboarding.Seen || s.Onboarding.Done {
		t.Fatalf("the app's patch stored %+v %+v", s.Oni.MCP, s.Onboarding)
	}
	if settingsForApp(json.RawMessage(`{"oni":{"mcpish":1,"master":true}}`)) {
		t.Error("an unknown key next to mcp is the app's alone")
	}
}
