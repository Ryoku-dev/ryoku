package graph

import (
	"encoding/json"
	"fmt"
	"image"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"ryoku-rashin/internal/app/conn"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/gitgraph"
	"ryoku-rashin/internal/protocol"
)

type fakeGraphServer struct {
	listener net.Listener
	methods  chan string
	mu       sync.Mutex
	pages    map[string]protocol.GitLogResult
}

func newFakeGraphServer(t *testing.T, pages map[string]protocol.GitLogResult) (*fakeGraphServer, string) {
	t.Helper()
	socketDir := t.TempDir()
	if err := os.Chmod(socketDir, 0o700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(socketDir, "rashind.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &fakeGraphServer{listener: listener, methods: make(chan string, 64), pages: pages}
	go server.serve()
	return server, socket
}

func (server *fakeGraphServer) close() { _ = server.listener.Close() }

func (server *fakeGraphServer) serve() {
	for {
		connection, err := server.listener.Accept()
		if err != nil {
			return
		}
		go server.serveConnection(connection)
	}
}

func (server *fakeGraphServer) serveConnection(connection net.Conn) {
	defer connection.Close()
	decoder := protocol.NewDecoder(connection)
	encoder := protocol.NewEncoder(connection)
	for {
		frame, err := decoder.Next()
		if err != nil {
			return
		}
		if frame.ID == nil {
			continue
		}
		select {
		case server.methods <- frame.Method:
		default:
		}
		var result any = protocol.Empty{}
		switch frame.Method {
		case protocol.MHello:
			result = protocol.HelloResult{Daemon: "test", Version: "test", Protocol: protocol.Version, Enabled: true}
		case protocol.MSubscribe:
			result = protocol.Snapshot{}
		case protocol.MGitLog:
			var params protocol.GitLogParams
			_ = json.Unmarshal(frame.Params, &params)
			server.mu.Lock()
			result = server.pages[params.Cursor]
			server.mu.Unlock()
		case protocol.MWorktreesList:
			result = protocol.WorktreesResult{Project: "project"}
		case protocol.MGitShow:
			var params protocol.GitCommitParams
			_ = json.Unmarshal(frame.Params, &params)
			result = protocol.GitCommitDetail{Commit: protocol.GitCommit{Hash: params.Hash, Subject: params.Hash}}
		case protocol.MGitStatus:
			var params protocol.WorktreeRef
			_ = json.Unmarshal(frame.Params, &params)
			result = protocol.GitStatusResult{Path: params.Path}
		case protocol.MGitDiff:
			result = protocol.DiffResult{}
		}
		payload, err := json.Marshal(result)
		if err != nil || encoder.Encode(protocol.Response{ID: *frame.ID, Result: payload}) != nil {
			return
		}
	}
}

type graphTestFrame struct {
	ops    op.Ops
	router input.Router
	now    time.Time
	size   image.Point
}

func (frame *graphTestFrame) context() layout.Context {
	frame.ops.Reset()
	return layout.Context{Ops: &frame.ops, Source: frame.router.Source(), Now: frame.now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(frame.size)}
}

func (frame *graphTestFrame) finish() {
	frame.router.Frame(&frame.ops)
	frame.now = frame.now.Add(time.Millisecond)
}

func fakeGraphEnv(t *testing.T, socket string) (*env.Env, *conn.Client, chan func()) {
	t.Helper()
	store := state.New(nil)
	connection := conn.NewOptions(store, client.Options{Socket: socket, Client: protocol.ClientApp})
	posts := make(chan func(), 64)
	e := &env.Env{Theme: look.New(0), Store: store, Client: connection, Overlays: new(look.Overlays), Post: func(fn func()) { posts <- fn }}
	deadline := time.Now().Add(3 * time.Second)
	for !connection.Connected() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !connection.Connected() {
		connection.Close()
		t.Fatal("fake graph client did not connect")
	}
	return e, connection, posts
}

func drainPosts(posts chan func()) {
	for {
		select {
		case fn := <-posts:
			fn()
		default:
			return
		}
	}
}

func waitGraph(t *testing.T, posts chan func(), ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		drainPosts(posts)
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("graph state did not become ready")
}

func TestPagingAppendsWithoutChangingEarlierRowsWithFakeClient(t *testing.T) {
	pages := map[string]protocol.GitLogResult{
		"":       {Commits: []protocol.GitCommit{{Hash: "a", Parents: []string{"b"}}, {Hash: "b", Parents: []string{"c"}}}, Next: "page-2"},
		"page-2": {Commits: []protocol.GitCommit{{Hash: "c", Parents: []string{"d"}}, {Hash: "d"}}},
	}
	server, socket := newFakeGraphServer(t, pages)
	defer server.close()
	e, connection, posts := fakeGraphEnv(t, socket)
	defer connection.Close()
	view := New()
	frame := graphTestFrame{now: time.Unix(100, 0), size: image.Pt(1200, 800)}
	gtx := frame.context()
	view.Layout(gtx, e, "project")
	e.Overlays.Layout(gtx, e.Theme)
	frame.finish()
	waitGraph(t, posts, func() bool { return view.state.loaded && len(view.state.commitLayouts) == 2 })
	first := append([]gitgraph.Row(nil), view.state.commitLayouts...)
	view.state.loadPage(e, false)
	waitGraph(t, posts, func() bool { return len(view.state.commitLayouts) == 4 })
	if !reflect.DeepEqual(first, view.state.commitLayouts[:2]) {
		t.Fatalf("first page rows changed after append\nbefore: %#v\nafter:  %#v", first, view.state.commitLayouts[:2])
	}
}

func TestSelectingWIPDoesNotRequestGitShow(t *testing.T) {
	server, socket := newFakeGraphServer(t, nil)
	defer server.close()
	e, connection, posts := fakeGraphEnv(t, socket)
	defer connection.Close()
	for len(server.methods) > 0 {
		<-server.methods
	}
	view := New()
	view.state.reset("project")
	view.state.applyPage(protocol.GitLogResult{
		Commits: []protocol.GitCommit{{Hash: "head"}},
		Wip:     []protocol.WipRow{{Worktree: "/tmp/worktree", Head: "head", Modified: 1}},
	}, true)
	view.state.selectEntry(e, view.state.entries[0])
	waitGraph(t, posts, func() bool { return !view.state.detailLoading })
	for {
		select {
		case method := <-server.methods:
			if method == protocol.MGitShow {
				t.Fatal("selecting a WIP row requested git.show")
			}
		default:
			return
		}
	}
}

func TestSecondaryClickOpensMenuOnRelease(t *testing.T) {
	server, socket := newFakeGraphServer(t, nil)
	defer server.close()
	e, connection, _ := fakeGraphEnv(t, socket)
	defer connection.Close()
	view := New()
	view.state.reset("project")
	view.state.applyPage(protocol.GitLogResult{Commits: []protocol.GitCommit{{Hash: "commit"}}}, true)
	frame := graphTestFrame{now: time.Unix(200, 0), size: image.Pt(700, 64)}
	row := view.state.row(view.state.entries[0].id())
	layoutFrame := func() {
		gtx := frame.context()
		view.state.layoutEntry(gtx, e, 0)
		e.Overlays.Layout(gtx, e.Theme)
		frame.finish()
	}
	layoutFrame()
	point := f32.Pt(400, 15)
	frame.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, PointerID: 7, Buttons: pointer.ButtonSecondary, Position: point})
	layoutFrame()
	if e.Overlays.IsOpen(&row.menuTag) {
		t.Fatal("secondary press opened the graph menu")
	}
	frame.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, PointerID: 7, Position: point})
	layoutFrame()
	layoutFrame()
	if !e.Overlays.IsOpen(&row.menuTag) {
		t.Fatal("secondary release did not open the graph menu")
	}
}

func TestLaneCollapseMarkerAppearsPastTwentyFourLanes(t *testing.T) {
	e := &env.Env{Theme: look.New(0), Overlays: new(look.Overlays)}
	view := New()
	view.state.reset("project")
	view.state.entries = []graphEntry{{kind: rowCommit, commit: protocol.GitCommit{Hash: "wide"}, layout: gitgraph.Row{Commit: "wide", Lane: 0, Width: 25}}}
	frame := graphTestFrame{now: time.Unix(300, 0), size: image.Pt(900, 64)}
	gtx := frame.context()
	view.state.layoutEntry(gtx, e, 0)
	frame.finish()
	if !view.state.laneMarkerVisible {
		t.Fatal("lane collapse marker was not laid out for 25 lanes")
	}
}

func TestGraphRowsAreThirtyDPAndDefaultViewShowsTwentyTwo(t *testing.T) {
	e := &env.Env{Theme: look.New(0), Overlays: new(look.Overlays)}
	view := New()
	view.state.reset("project")
	commits := make([]protocol.GitCommit, 60)
	for index := range commits {
		commits[index] = protocol.GitCommit{
			Hash:    fmt.Sprintf("commit-%02d", index),
			Author:  "Mika",
			Subject: "Keep the history dense and readable",
		}
	}
	view.state.applyPage(protocol.GitLogResult{Commits: commits}, true)
	view.state.loaded = true

	rowFrame := graphTestFrame{now: time.Unix(400, 0), size: image.Pt(900, 80)}
	rowDimensions := view.state.layoutEntry(rowFrame.context(), e, 0)
	rowFrame.finish()
	if rowDimensions.Size.Y != graphRowHeight {
		t.Fatalf("graph row height = %d, want %d", rowDimensions.Size.Y, graphRowHeight)
	}

	frame := graphTestFrame{now: time.Unix(500, 0), size: image.Pt(1280, 820)}
	gtx := frame.context()
	view.Layout(gtx, e, "project")
	e.Overlays.Layout(gtx, e.Theme)
	frame.finish()
	if visible := view.state.scroll.Position.Count; visible < 22 {
		t.Fatalf("visible graph rows at 1280x820 = %d, want at least 22", visible)
	}
}

func TestDetailPanePlacementByWidth(t *testing.T) {
	tests := []struct {
		width int
		side  bool
		pane  int
	}{
		{width: 999},
		{width: 1000, side: true, pane: 400},
		{width: 1280, side: true, pane: 512},
		{width: 1600, side: true, pane: 560},
	}
	for _, test := range tests {
		side, pane := detailPanePlacement(test.width)
		if side != test.side || pane != test.pane {
			t.Errorf("detailPanePlacement(%d) = (%t, %d), want (%t, %d)", test.width, side, pane, test.side, test.pane)
		}
	}
}
