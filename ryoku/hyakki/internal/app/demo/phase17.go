package demo

import (
	"encoding/json"
	"slices"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// FeatureOniPhase17 is no feature: it starts the demo as FeatureOni with the
// MCP bridge on, reminders listing and the pill line turned on, one program
// approved and another waiting, for the Settings pages of Phase 17.
const FeatureOniPhase17 = "oni-phase17"

// the demo's programs on the MCP bridge: one approved, one waiting
const (
	demoMCPApproved = "Claude Code"
	demoMCPWaiting  = "Zed"
)

// applyPhase17 turns the bridge on for FeatureOniPhase17
func applyPhase17(settings *protocol.OniSettings) {
	settings.Voice, settings.Reflex, settings.ReflexSettings.Reminders = true, true, true
	settings.MCP = protocol.OniMCPSettings{
		Enabled: true,
		Tools:   map[string]bool{protocol.OniMCPRemindersList: true, protocol.OniMCPNotify: true},
		Clients: []string{demoMCPApproved},
	}
}

// phase17Fixtures holds the demo's optional-part inventory, imported-chat
// count and bridge clients. Activity ledgers belong to their feature fixtures.
// Part removal is simulated; a walkthrough store, when supplied by the demo,
// remains the guide owner's real store and is cleared through that owner.
type phase17Fixtures struct {
	components []protocol.Component
	counts     map[string]int
	waiting    []string
	notified   []string
}

func newPhase17Fixtures() phase17Fixtures {
	const data = "/home/demo/.local/share/hyakki"
	return phase17Fixtures{
		components: []protocol.Component{
			{ID: "stt:base.en", Kind: protocol.ComponentModel, Title: "Speech model base.en", State: protocol.ComponentPresent, Bytes: 147_964_211, Path: data + "/oni/voice/models/ggml-base.en.bin", Removable: true, Feature: "oni.voice", Uses: "Talking to Oni and dictation"},
			{ID: "stt:small.en", Kind: protocol.ComponentModel, Title: "Speech model small.en", State: protocol.ComponentPresent, Bytes: 487_601_967, Path: "/usr/share/ryoku/dictation/models/ggml-small.en.bin", Feature: "oni.voice", Uses: "Talking to Oni and dictation", Why: "Ryoku installed it; Ryoku removes it."},
			{ID: "tts:kokoro", Kind: protocol.ComponentPack, Title: "Kokoro voice", State: protocol.ComponentAbsent, Feature: "oni.voiceSettings.tts", Uses: "Oni's spoken replies"},
			{ID: "hands:profile", Kind: protocol.ComponentProfile, Title: "Hands browser profile", State: protocol.ComponentPresent, Bytes: 41_203_712, Path: "/home/demo/.local/state/hyakki/hands-browser", Removable: true, Feature: "oni.hands", Uses: "Hands in one browser tab"},
			{ID: "recall:chats", Kind: protocol.ComponentData, Title: "Imported chats", State: protocol.ComponentPresent, Bytes: 2_310_144, Path: data + "/imports/chats", Removable: true, Feature: "oni.recall", Uses: "Recall"},
			{ID: "voice:cache", Kind: protocol.ComponentCache, Title: "Spoken reply cache", State: protocol.ComponentPresent, Bytes: 812_032, Path: "/home/demo/.cache/hyakki/oni/voice", Removable: true, Feature: "oni.voiceSettings.speakReplies", Uses: "Oni's spoken replies"},
			{ID: "program:tesseract", Kind: protocol.ComponentProgram, Title: "tesseract", State: protocol.ComponentPresent, Feature: "oni.guide", Uses: "Guides", Needs: []string{"tesseract", "tesseract-data-eng"}, Why: "Reads the words in the perched window, on this computer."},
			{ID: "program:weston", Kind: protocol.ComponentProgram, Title: "weston", State: protocol.ComponentAbsent, Feature: "oni.hands", Uses: "Hands on Hyakki's own desktop", Needs: []string{"weston"}, Why: "Runs the separate desktop Hands works on."},
		},
		counts:  map[string]int{protocol.OniActivityRecallImports: 14},
		waiting: []string{demoMCPWaiting},
	}
}

// the demo's components calls answer as the daemon's do: listing reads,
// removing refuses what Hyakki did not download or what is in use
func (s *server) callComponents(method string, raw json.RawMessage) (any, error) {
	p := &s.data.oni.phase17
	switch method {
	case protocol.MComponents:
		return protocol.ComponentsResult{Components: slices.Clone(p.components)}, nil
	case protocol.MComponentRemove:
		var params protocol.ComponentRemoveParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if !protocol.ValidComponentID(params.ID) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The part's id is invalid."))
		}
		i := slices.IndexFunc(p.components, func(c protocol.Component) bool { return c.ID == params.ID })
		switch {
		case i < 0:
			return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("There is no such part."))
		case !p.components[i].Removable:
			return nil, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Hyakki did not put this part here, so it does not remove it."))
		case p.components[i].InUse != "":
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", p.components[i].InUse)
		}
		c := &p.components[i]
		c.State, c.Bytes, c.Path = protocol.ComponentAbsent, 0, ""
		return protocol.ComponentsResult{Components: slices.Clone(p.components)}, nil
	}
	return nil, notFound(method)
}

// the demo's Phase 17 Oni calls: the activity across the reflex fixtures
// and its own, deleting stores, a line on the pill, and the bridge's state
func (s *server) callOniPhase17(method string, raw json.RawMessage) (any, error) {
	o := &s.data.oni
	p := &o.phase17
	settings := protocol.EffectiveOni(s.data.snapshot.Settings)
	switch method {
	case protocol.MOniActivity:
		var params protocol.Empty
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return o.activity()
	case protocol.MOniActivityClear:
		var params protocol.OniActivityClearParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if !protocol.ValidOniActivityStores(params.Stores) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Name the activity to delete, each once, or all of it."))
		}
		if settings.Master || o.phase16.watching || o.hands.state == protocol.OniHandsStateWorking || o.guides.running != nil {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Turn Oni off and wait for running work before clearing its records."))
		}
		stores := params.Stores
		if stores[0] == protocol.OniActivityAll {
			stores = protocol.OniActivityStores
		}
		for _, store := range stores {
			if store == protocol.OniActivityWalkthroughs && o.guides.store != nil {
				if _, err := o.guides.store.List(); err != nil {
					return nil, err
				}
			}
		}
		for _, store := range stores {
			switch store {
			case protocol.OniActivityReflexLog:
				o.reflex.rows = nil
			case protocol.OniActivityHandsLog:
				o.hands.rows, o.hands.tasks = nil, nil
			case protocol.OniActivityAssignment:
				o.phase16.rows = nil
			case protocol.OniActivityWalkthroughs:
				if o.guides.store != nil {
					if err := o.guides.store.ForgetAll(); err != nil {
						return nil, err
					}
				}
				clear(o.guides.kept)
			case protocol.OniActivityRoutines:
				o.reflex.routines = nil
			case protocol.OniActivityReminders:
				o.reflex.reminders = nil
			default:
				delete(p.counts, store)
			}
		}
		return o.activity()
	case protocol.MOniNotify:
		var params protocol.OniNotifyParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		if !protocol.ValidOniNotifyText(params.Text) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A line on Oni's pill is one line of at most %d characters.", protocol.OniNotifyMax))
		}
		if !settings.Master || settings.Hidden {
			return nil, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("Oni is off or hidden, so nothing is shown."))
		}
		p.notified = append(p.notified, params.Text)
		return protocol.Empty{}, nil
	case protocol.MOniMCP:
		status := protocol.OniMCPStatus{Enabled: settings.MCP.Enabled, Tools: []string{}, Clients: []protocol.OniMCPClient{}}
		for _, id := range protocol.OniMCPTools {
			if settings.MCP.Enabled && settings.MCP.Tools[id] {
				status.Tools = append(status.Tools, id)
			}
		}
		for _, name := range settings.MCP.Clients {
			status.Clients = append(status.Clients, protocol.OniMCPClient{Name: name, State: protocol.OniMCPClientApproved})
		}
		for _, name := range p.waiting {
			if settings.MCP.Enabled && !slices.Contains(settings.MCP.Clients, name) {
				status.Clients = append(status.Clients, protocol.OniMCPClient{Name: name, State: protocol.OniMCPClientWaiting, Connected: true})
			}
		}
		return status, nil
	}
	return nil, notFound(method)
}

// activity is the demo's oni.activity: the reflex fixtures' reminders,
// routines and ledger with the Phase 17 fixtures' counts and rows, newest
// first, at most OniActivityRowsMax of them
func (o *oniFixtures) activity() (protocol.OniActivityResult, error) {
	p := &o.phase17
	out := protocol.OniActivityResult{Counts: make(map[string]int, len(protocol.OniActivityStores)), Rows: []protocol.OniActivityRow{}}
	for _, store := range protocol.OniActivityStores {
		out.Counts[store] = p.counts[store]
	}
	out.Counts[protocol.OniActivityReflexLog] = len(o.reflex.rows)
	out.Counts[protocol.OniActivityHandsLog] = len(o.hands.rows)
	out.Counts[protocol.OniActivityAssignment] = len(o.phase16.rows)
	out.Counts[protocol.OniActivityWalkthroughs] = len(o.guides.kept)
	if o.guides.store != nil {
		entries, err := o.guides.store.List()
		if err != nil {
			delete(out.Counts, protocol.OniActivityWalkthroughs)
			out.Errors = map[string]string{protocol.OniActivityWalkthroughs: i18n.T("These local records could not be read. Their count is unknown.")}
		} else {
			out.Counts[protocol.OniActivityWalkthroughs] = len(entries)
		}
	}
	out.Counts[protocol.OniActivityRoutines] = len(o.reflex.routines)
	out.Counts[protocol.OniActivityReminders] = len(o.reflex.reminders)
	for _, r := range o.reflex.rows {
		out.Rows = append(out.Rows, protocol.OniActivityRow{At: r.At, Store: protocol.OniActivityReflexLog, Action: r.Intent, Route: r.Route, Ms: r.Ms, Unmetered: true, Outcome: r.Outcome})
	}
	for _, r := range o.hands.rows {
		out.Rows = append(out.Rows, protocol.OniActivityRow{At: r.At, Store: protocol.OniActivityHandsLog, Action: r.Tool, Ms: r.Ms, Unmetered: true, Outcome: r.Outcome})
	}
	for _, r := range o.phase16.rows {
		out.Rows = append(out.Rows, protocol.OniActivityRow{At: r.At, Store: protocol.OniActivityAssignment, Action: r.Lane, Route: r.Route, Input: r.Input, Output: r.Output, Estimated: r.Estimated, Unmetered: r.Unmetered, Outcome: r.Outcome})
	}
	slices.SortStableFunc(out.Rows, func(a, b protocol.OniActivityRow) int { return b.At.Compare(a.At) })
	if n := len(out.Rows); n > 0 {
		out.Since = out.Rows[n-1].At
	}
	if len(out.Rows) > protocol.OniActivityRowsMax {
		out.Rows = out.Rows[:protocol.OniActivityRowsMax]
	}
	return out, nil
}
