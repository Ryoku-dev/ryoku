package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// memoryCall is one memory method that reached memory, with its caller.
type memoryCall struct {
	from   core.MemoryCaller
	method string
}

// recordingMemory stands in for memory: it records each call with who made
// it, settles what is pending into Busy until told it is done, and reports a
// fixed status.
type recordingMemory struct {
	core.Memory
	mu       sync.Mutex
	calls    []memoryCall
	pending  bool
	busy     bool
	settled  int
	released int
	status   protocol.MemoryStatus
}

func (m *recordingMemory) Bind(core.MemorySessions)                     {}
func (m *recordingMemory) Brief(protocol.SessionInfo) string            { return "" }
func (m *recordingMemory) Status(context.Context) protocol.MemoryStatus { return m.status }
func (m *recordingMemory) Close() error                                 { return nil }

func (m *recordingMemory) Handle(_ context.Context, from core.MemoryCaller, method string, _ json.RawMessage) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, memoryCall{from: from, method: method})
	return protocol.Empty{}, nil
}

func (m *recordingMemory) Settle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settled++
	m.busy, m.pending = m.busy || m.pending, false
}

func (m *recordingMemory) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.busy
}

func (m *recordingMemory) Release() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.released++
}

func (m *recordingMemory) heard() []memoryCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]memoryCall(nil), m.calls...)
}

func TestATeamSocketReachesOnlyItsTeamsMemory(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	memory := &recordingMemory{}
	d := New(Deps{Teams: &boundTeams{handled: make(chan string, 8)}, Memory: memory})
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
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Socket: paths.AgentSocket(team.ID)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	allowed := []struct {
		method string
		params any
	}{
		{protocol.MMemorySearch, protocol.MemorySearchParams{Query: "deploys"}},
		{protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: "team", Text: "The site deploys from main."}},
		{protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: protocol.MemoryYou, Text: "The user reads reports in the morning.", Session: "someone-else"}},
		{protocol.MMemoryForget, protocol.MemoryForgetParams{IDs: []string{"m0000001"}}},
	}
	for _, call := range allowed {
		if err := conn.Call(ctx, call.method, call.params, nil); err != nil {
			t.Fatalf("%s on the team's socket: %v", call.method, err)
		}
	}
	for _, refused := range []struct {
		method string
		params any
	}{
		{protocol.MMemoryList, protocol.MemoryListParams{}},
		{protocol.MMemoryGet, protocol.MemoryRefParams{ID: "m0000001"}},
		{protocol.MMemoryEdit, protocol.MemoryEditParams{ID: "m0000001"}},
		{protocol.MMemoryKeep, protocol.MemoryIDsParams{IDs: []string{"m0000001"}}},
		{protocol.MMemoryForgetAll, protocol.MemoryForgetAllParams{}},
		{protocol.MMemoryCard, protocol.MemoryCardParams{}},
		{protocol.MMemorySessions, protocol.MemorySessionsParams{Query: "x"}},
		{protocol.MMemoryStatus, protocol.Empty{}},
		{protocol.MMemoryLearn, protocol.MemoryLearnParams{Session: team.ID}},
	} {
		if err := conn.Call(ctx, refused.method, refused.params, nil); !forbidden(err) {
			t.Fatalf("%s on the team's socket: %v, want forbidden", refused.method, err)
		}
	}
	heard := memory.heard()
	if len(heard) != len(allowed) {
		t.Fatalf("memory heard %d calls, want %d: %+v", len(heard), len(allowed), heard)
	}
	for index, call := range heard {
		// the caller comes from the socket, never from what the call names
		if call.method != allowed[index].method || call.from != (core.MemoryCaller{Team: "team-1", Task: "task-1"}) {
			t.Fatalf("call %d = %+v", index, call)
		}
	}
}

func TestOnlyTheAppMakesTheCallsMemoryKeepsToTheUser(t *testing.T) {
	isolateDaemon(t)
	memory := &recordingMemory{}
	d := New(Deps{Memory: memory})
	ctx := context.Background()
	everything, _ := json.Marshal(protocol.MemoryForgetAllParams{})
	if _, err := d.dispatch(ctx, &connection{}, protocol.MMemoryForgetAll, everything); !forbidden(err) {
		t.Fatalf("the CLI forgetting everything: %v, want forbidden", err)
	}
	if _, err := d.dispatch(ctx, &connection{app: true}, protocol.MMemoryForgetAll, everything); err != nil {
		t.Fatalf("the app forgetting everything: %v", err)
	}
	patch, _ := json.Marshal(protocol.SettingsSetParams{Patch: json.RawMessage(`{"memory":{"use":true,"asked":true}}`)})
	if _, err := d.dispatch(ctx, &connection{}, protocol.MSettingsSet, patch); !forbidden(err) {
		t.Fatalf("the CLI turning memory on: %v, want forbidden", err)
	}
	if heard := memory.heard(); len(heard) != 1 || heard[0].from != (core.MemoryCaller{App: true}) {
		t.Fatalf("memory heard %+v", heard)
	}
}

func TestARememberOnTheMainSocketShowsInItsOwnConversation(t *testing.T) {
	isolateDaemon(t)
	memory := &recordingMemory{}
	d := New(Deps{Memory: memory})
	ordinary, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: paths.Home()})
	if err != nil {
		t.Fatal(err)
	}
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: paths.Home(), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range []string{ordinary.ID, team.ID, "no-such-session"} {
		raw, _ := json.Marshal(protocol.MemoryAddParams{Scope: protocol.MemoryYou, Text: "The user prefers tabs.", Session: session})
		if _, err := d.dispatch(context.Background(), &connection{}, protocol.MMemoryAdd, raw); err != nil {
			t.Fatal(err)
		}
	}
	// an agent's searches stay within what its conversation may read, as
	// its MCP server sends them
	for _, tool := range []string{brief.ToolMemorySearch, brief.ToolSessionSearch} {
		method, params, err := brief.MemoryParams(tool, json.RawMessage(`{"query":"tabs"}`), ordinary.ID)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(params)
		if _, err := d.dispatch(context.Background(), &connection{}, method, raw); err != nil {
			t.Fatal(err)
		}
	}
	// the app's own remember is the user's, whatever conversation it names
	raw, _ := json.Marshal(protocol.MemoryAddParams{Scope: protocol.MemoryYou, Text: "The user prefers tabs.", Session: ordinary.ID})
	if _, err := d.dispatch(context.Background(), &connection{app: true}, protocol.MMemoryAdd, raw); err != nil {
		t.Fatal(err)
	}
	heard := memory.heard()
	want := []core.MemoryCaller{{Session: ordinary.ID}, {}, {}, {Session: ordinary.ID}, {Session: ordinary.ID}, {App: true}}
	if len(heard) != len(want) {
		t.Fatalf("memory heard %+v", heard)
	}
	for index, call := range heard {
		if call.from != want[index] {
			t.Fatalf("call %d (%s) came from %+v, want %+v", index, call.method, call.from, want[index])
		}
	}
}

func TestTheIdleExitWaitsForMemoryToSettle(t *testing.T) {
	isolateDaemon(t)
	memory := &recordingMemory{pending: true}
	d := New(Deps{Memory: memory})
	t.Cleanup(d.Shutdown)
	d.onIdleDeadline()
	if memory.settled != 1 {
		t.Fatalf("settled %d times before the exit, want 1", memory.settled)
	}
	if stopped(d) || memory.released != 0 {
		t.Fatal("the daemon exited while memory was learning")
	}
	memory.mu.Lock()
	memory.busy = false
	memory.mu.Unlock()
	d.onIdleDeadline()
	if !stopped(d) {
		t.Fatal("the daemon stayed up once memory had settled")
	}
	if memory.settled != 2 || memory.released != 1 {
		t.Fatalf("settled %d and released %d times, want 2 and 1", memory.settled, memory.released)
	}
}

func TestMemoryIsNotSettledWhileAWindowIsOpen(t *testing.T) {
	isolateDaemon(t)
	memory := &recordingMemory{pending: true}
	d := New(Deps{Memory: memory})
	t.Cleanup(d.Shutdown)
	d.mu.Lock()
	d.clients[&connection{}] = struct{}{}
	d.mu.Unlock()
	d.onIdleDeadline()
	if memory.settled != 0 {
		t.Fatalf("settled %d times with a client connected, want none: the quiet time would be cut short", memory.settled)
	}
	if stopped(d) {
		t.Fatal("the daemon exited with a client connected")
	}
}

func TestAConversationWaitingForAModelDoesNotHoldTheDaemonUp(t *testing.T) {
	isolateDaemon(t)
	// a conversation waiting for a learning model is not due: settling
	// leaves memory idle
	memory := &recordingMemory{}
	d := New(Deps{Memory: memory})
	t.Cleanup(d.Shutdown)
	d.onIdleDeadline()
	if !stopped(d) || memory.settled != 1 || memory.released != 1 {
		t.Fatalf("stopped %v, settled %d, released %d; want an exit after one settle", stopped(d), memory.settled, memory.released)
	}
}

// snapshotGateway answers what the snapshot reads of the gateway.
type snapshotGateway struct{ core.Gateway }

func (snapshotGateway) Status(context.Context) protocol.GatewayStatus {
	return protocol.GatewayStatus{}
}
func (snapshotGateway) Providers(context.Context) ([]protocol.ProviderInfo, error) {
	return nil, nil
}

func TestTheSnapshotCarriesMemorysStatus(t *testing.T) {
	isolateDaemon(t)
	memory := &recordingMemory{status: protocol.MemoryStatus{Engine: protocol.MemoryBuiltin, EngineState: protocol.EngineReady, Counts: protocol.MemoryCounts{All: 3, You: 2}}}
	d := New(Deps{Gateway: snapshotGateway{}, Harnesses: &fakeHarnesses{}, Indexer: &fakeIndexer{}, Memory: memory})
	snapshot, err := d.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Memory == nil || snapshot.Memory.Counts.All != 3 || snapshot.Memory.EngineState != protocol.EngineReady {
		t.Fatalf("snapshot memory = %+v", snapshot.Memory)
	}
	without := New(Deps{Gateway: snapshotGateway{}, Harnesses: &fakeHarnesses{}, Indexer: &fakeIndexer{}})
	if snapshot, err := without.snapshot(context.Background()); err != nil || snapshot.Memory != nil {
		t.Fatalf("a daemon without memory: %+v, %v", snapshot.Memory, err)
	}
}
