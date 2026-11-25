package daemon

import (
	"encoding/json"
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// one reflex call with the params it carries
func reflexGateCalls(t *testing.T) map[string][]json.RawMessage {
	return map[string][]json.RawMessage{
		protocol.MOniReminders: {
			mustMarshal(t, protocol.OniRemindersParams{Action: protocol.OniRemindersList}),
			mustMarshal(t, protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: "0123456789abcdef"}),
			mustMarshal(t, protocol.OniRemindersParams{Action: protocol.OniRemindersCancelAll}),
		},
		protocol.MOniRoutines: {
			mustMarshal(t, protocol.OniRoutinesParams{Action: protocol.OniRoutinesList}),
			mustMarshal(t, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun}),
			mustMarshal(t, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun, Name: "Morning"}),
			mustMarshal(t, protocol.OniRoutinesParams{Action: protocol.OniRoutinesForget, Name: "Morning"}),
			mustMarshal(t, protocol.OniRoutinesParams{Action: protocol.OniRoutinesForgetAll}),
		},
		protocol.MOniReflexLog: {nil, json.RawMessage(`{}`)},
	}
}

// The reminders, routines and reflex ledger calls are the app's and the
// CLI's alone: the surface drawing Oni has no action among them, and an
// agent, a team socket, an observer, another surface, the bar, the Hub and
// a program that is not the app are refused, whatever params they carry.
func TestOniReflexCallsPassTheGateOnlyForTheAppAndTheCLI(t *testing.T) {
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

	calls := reflexGateCalls(t)
	for method := range calls {
		if !oniHands[method] || !oniReflex[method] {
			t.Errorf("%s is not among the calls no agent makes", method)
		}
		if surfaceMethods[method] {
			t.Errorf("a surface may call %s", method)
		}
		if _, ok := oniPresenterActions[method]; ok {
			t.Errorf("the presenter has actions on %s", method)
		}
		if _, ok := oniVoice[method]; ok {
			t.Errorf("%s is gated as a voice call", method)
		}
	}
	if len(oniReflex) != len(calls) {
		t.Fatalf("the reflex gate holds %d calls, the test %d", len(oniReflex), len(calls))
	}
	for method, params := range calls {
		for _, raw := range params {
			for name, conn := range map[string]*connection{"the app": app, "the CLI": cli} {
				if err := d.mainGate(conn, method, raw); err != nil {
					t.Errorf("%s was refused %s %s: %v", name, method, raw, err)
				}
			}
			for name, conn := range map[string]*connection{
				"the presenter":                       presenter,
				"a surface that is not the presenter": second,
				"an observer":                         observer,
				"the app watching as an observer":     appObserver,
				"the bar":                             bar,
				"the hub":                             hub,
				"a program that is not the app":       notApp,
				"a team session's connection":         team,
			} {
				if err := d.mainGate(conn, method, raw); !forbidden(err) {
					t.Errorf("%s called %s %s: %v", name, method, raw, err)
				}
			}
			for _, info := range []protocol.SessionInfo{{ID: "ask-1", Kind: protocol.KindAsk}, {ID: "agent-1", Kind: protocol.KindAgent, Team: "t-1"}} {
				if err := d.hostGate(info, method, raw); !forbidden(err) {
					t.Errorf("an agent's tools (team %q) called %s %s: %v", info.Team, method, raw, err)
				}
			}
			if err := agentGate(method); !forbidden(err) {
				t.Errorf("a team session's socket may call %s", method)
			}
		}
	}
	// a presenter that stops presenting gains nothing
	d.oni.clientsMu.Lock()
	d.oni.presenter = nil
	d.oni.clientsMu.Unlock()
	for method, params := range calls {
		if err := d.mainGate(presenter, method, params[0]); !forbidden(err) {
			t.Errorf("a former presenter called %s: %v", method, err)
		}
	}
}

// With the reflex switches off, Oni's status lists the four reflex rows
// denied with feature-off after feeds.guard and before the guide, and the
// reflex row itself is as it was; nothing is reserved.
func TestOniReflexOffDeniesTheReflexRows(t *testing.T) {
	d := oniDaemon(t)
	status := d.oniStatus()
	var ids []string
	for _, c := range status.Capabilities.Features {
		ids = append(ids, c.ID)
		switch c.ID {
		case feature.Routines, feature.Reminders, feature.Notes, feature.Decisions, feature.Reflex:
			if c.Allow || c.Reason != feature.ReasonFeatureOff {
				t.Errorf("%s = allow %v reason %q with reflex off", c.ID, c.Allow, c.Reason)
			}
		}
	}
	at := slices.Index(ids, feature.FeedGuard)
	want := []string{feature.FeedGuard, feature.Routines, feature.Reminders, feature.Notes, feature.Decisions, feature.Guide}
	if at < 0 || at+len(want) > len(ids) || !slices.Equal(ids[at:at+len(want)], want) {
		t.Fatalf("capability order %q", ids)
	}
	if len(feature.ReservedCalls()) != 0 {
		t.Fatalf("reserved calls %q", feature.ReservedCalls())
	}
	// a reflex switch with Reflex off still allows nothing
	settings := d.Settings()
	settings.Oni.ReflexSettings = protocol.OniReflexSettings{Routines: true, Reminders: true, Notes: true, Decisions: true}
	d.mu.Lock()
	d.settings = settings
	d.mu.Unlock()
	for _, c := range d.oniStatus().Capabilities.Features {
		if slices.Contains([]string{feature.Routines, feature.Reminders, feature.Notes, feature.Decisions}, c.ID) && c.Allow {
			t.Errorf("%s allowed with Reflex off", c.ID)
		}
	}
}
