package daemon

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// dialBridge is a connection from the MCP bridge for the program name.
func dialBridge(t *testing.T, name string) (*socketPeer, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientMCP, Name: name})
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { conn.Close() })
	return &socketPeer{t: t, conn: conn}, nil
}

func mcpOn(t *testing.T, d *Daemon, tools ...string) {
	t.Helper()
	on := map[string]bool{}
	for _, id := range tools {
		on[id] = true
	}
	patch, _ := json.Marshal(map[string]any{"oni": map[string]any{"mcp": map[string]any{"enabled": true, "tools": on, "clients": []string{"agent"}}}})
	if _, err := d.applySettings(context.Background(), patch); err != nil {
		t.Fatal(err)
	}
}

// A screen ask from the bridge always waits for the person's answer on the
// surface, even for a route said yes to this session; the bridge cannot
// answer it itself, and a no or an expiry sends nothing.
func TestMcpScreenAskWaitsForThePersonEveryTime(t *testing.T) {
	r := serveWire(t, true, localEyes)
	mcpOn(t, r.d, protocol.OniMCPScreenAsk)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	perchOn(t, cli, "0x1")
	surface.frames(2)
	bridge, err := dialBridge(t, "agent")
	if err != nil {
		t.Fatal(err)
	}

	ask := func() (chan callResult, *feature.ConsentFrame) {
		ch := bridge.async(protocol.MOniAsk, protocol.OniAskParams{Text: "Where do I save?"})
		select {
		case got := <-ch:
			t.Fatalf("the bridge's ask returned at once: %+v, %v", got.res, got.err)
		case <-time.After(200 * time.Millisecond):
		}
		f := surface.frames(1)[0].(*feature.ConsentFrame)
		return ch, f
	}

	// the bridge cannot answer its own question; the surface's no sends nothing
	ch, f := ask()
	if err := bridge.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: f.ID, Answer: "yes"}, nil); !forbidden(err) {
		t.Fatalf("the bridge answered its own question: %v", err)
	}
	if err := surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: f.ID, Answer: "no"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := <-ch; got.err != nil || got.res.Status != protocol.OniAskDeclined || got.res.Consent != nil {
		t.Fatalf("after a no = %+v, %v", got.res, got.err)
	}
	if len(r.model.sent()) != 0 || !r.shots.cleared() {
		t.Fatal("a no sent or kept the picture")
	}

	// a yes on the surface sends it; the bridge gets the words only
	ch, f = ask()
	r.model.reply("Save sits at the top left.\n[POINT:0.5,0.5,0.125,0.0625:Save][CONF:0.9]")
	if err := surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: f.ID, Answer: "yes"}, nil); err != nil {
		t.Fatal(err)
	}
	got := <-ch
	if got.err != nil || got.res.Status != protocol.OniAskAnswered || got.res.Answer == nil || got.res.Answer.Text != "Save sits at the top left." || got.res.Consent != nil {
		t.Fatalf("after a yes = %+v, %v", got.res, got.err)
	}
	if len(r.model.sent()) != 1 {
		t.Fatalf("%d requests after one yes", len(r.model.sent()))
	}
	surface.frames(2) // the strand to the point and the answer's bubble

	// the route said yes to is asked again for the bridge, and expires to a no
	ch, _ = ask()
	r.clock.Advance(feature.ConsentTTL)
	if got := <-ch; got.err != nil || got.res.Status != protocol.OniAskDeclined {
		t.Fatalf("after an expiry = %+v, %v", got.res, got.err)
	}
	if len(r.model.sent()) != 1 {
		t.Fatal("an expired question was sent")
	}
}

// The bridge's hello is refused while it is off; an unapproved program and
// a tool turned off are refused every call.
func TestMcpBridgeIsRefusedWhenOffUnapprovedOrToolOff(t *testing.T) {
	r := serveWire(t, true, localEyes)
	if _, err := dialBridge(t, "agent"); err == nil {
		t.Fatal("hello from the bridge passed while it is off")
	}
	mcpOn(t, r.d, protocol.OniMCPNotify)
	stranger, err := dialBridge(t, "stranger")
	if err != nil {
		t.Fatal(err)
	}
	var status protocol.OniMCPStatus
	if err := stranger.call(protocol.MOniMCP, protocol.Empty{}, &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Clients) != 1 || status.Clients[0].Name != "stranger" || status.Clients[0].State != protocol.OniMCPClientWaiting {
		t.Fatalf("a waiting program sees %+v", status.Clients)
	}
	if err := stranger.call(protocol.MOniNotify, protocol.OniNotifyParams{Text: "hi"}, nil); !forbidden(err) {
		t.Fatalf("an unapproved program notified: %v", err)
	}
	agent, err := dialBridge(t, "agent")
	if err != nil {
		t.Fatal(err)
	}
	if err := agent.call(protocol.MOniAsk, protocol.OniAskParams{Text: "Where do I save?"}, nil); !forbidden(err) {
		t.Fatalf("a tool turned off was called: %v", err)
	}
	if err := agent.call(protocol.MSettingsSet, json.RawMessage(`{"oni":{"mcp":{"clients":["stranger","agent"]}}}`), nil); !forbidden(err) {
		t.Fatalf("the bridge changed settings: %v", err)
	}
	// the app sees both, one approved and connected, one waiting
	app := r.dialApp(t)
	if err := app.call(protocol.MOniMCP, protocol.Empty{}, &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Clients) != 2 || status.Clients[0] != (protocol.OniMCPClient{Name: "agent", State: protocol.OniMCPClientApproved, Connected: true}) || status.Clients[1].State != protocol.OniMCPClientWaiting {
		t.Fatalf("the app sees %+v", status.Clients)
	}
}

// A notice shows on the pill headed with the program's name, three a
// minute at most, and never a link or a private number.
func TestMcpNotifyShowsThreeAMinuteHeadedWithTheProgram(t *testing.T) {
	r := serveWire(t, true, localEyes)
	mcpOn(t, r.d, protocol.OniMCPNotify)
	surface, _ := r.presenting(t)
	agent, err := dialBridge(t, "agent")
	if err != nil {
		t.Fatal(err)
	}
	for _, refused := range []string{"see https://example.com", "card 4111 1111 1111 1111", "two\nlines"} {
		if code, _ := reasonOf(agent.call(protocol.MOniNotify, protocol.OniNotifyParams{Text: refused}, nil)); code != protocol.CodeBadRequest {
			t.Fatalf("%q: %s", refused, code)
		}
	}
	shown := 0
	for range 100 {
		if agent.call(protocol.MOniNotify, protocol.OniNotifyParams{Text: "Build finished."}, nil) == nil {
			shown++
		}
	}
	if shown != oniNotifyPerMinute {
		t.Fatalf("%d of 100 notices shown", shown)
	}
	for _, f := range surface.frames(shown) {
		if notice := f.(*feature.NoticeFrame); notice.Text != "agent: Build finished." {
			t.Fatalf("notice = %+v", notice)
		}
	}
}

// A note from the bridge waits as a suggestion from the program, never a
// fact; a card number is refused, and the bridge cannot add a fact.
func TestMcpNotesBecomeSuggestionsNeverFacts(t *testing.T) {
	r := serveMemory(t, func(s *protocol.Settings) {
		s.Oni.MCP = protocol.OniMCPSettings{Enabled: true, Tools: map[string]bool{protocol.OniMCPNotesAdd: true, protocol.OniMCPNotesSearch: true}, Clients: []string{"agent"}}
	})
	agent, err := dialBridge(t, "agent")
	if err != nil {
		t.Fatal(err)
	}
	var change protocol.MemoryChange
	if err := agent.call(protocol.MMemoryImport, protocol.MemoryImportParams{Text: "Prefers short answers.", From: "someone else"}, &change); err != nil {
		t.Fatal(err)
	}
	if len(change.Items) != 1 || change.Items[0].Op != protocol.MemoryOpSuggested {
		t.Fatalf("change = %+v", change)
	}
	if facts := r.facts(t); len(facts) != 0 {
		t.Fatalf("a bridge note became a fact: %v", facts)
	}
	var suggested protocol.MemoryListResult
	if err := r.app.call(protocol.MMemoryList, protocol.MemoryListParams{Scope: protocol.MemoryYou, Status: protocol.MemorySuggested}, &suggested); err != nil {
		t.Fatal(err)
	}
	if len(suggested.Memories) != 1 || suggested.Memories[0].Source.From != "agent" || suggested.Memories[0].Source.Kind != protocol.MemoryFromImport {
		t.Fatalf("suggested = %+v", suggested.Memories)
	}
	if code, _ := reasonOf(agent.call(protocol.MMemoryImport, protocol.MemoryImportParams{Text: "My card is 4111 1111 1111 1111."}, nil)); code != protocol.CodeBadRequest {
		t.Fatalf("a card number: %s", code)
	}
	if err := agent.call(protocol.MMemoryAdd, protocol.MemoryAddParams{Text: "Likes tea."}, nil); !forbidden(err) {
		t.Fatalf("the bridge added a fact: %v", err)
	}
	if err := agent.call(protocol.MMemoryKeep, protocol.MemoryIDsParams{IDs: []string{suggested.Memories[0].ID}}, nil); !forbidden(err) {
		t.Fatalf("the bridge kept its own suggestion: %v", err)
	}
}
