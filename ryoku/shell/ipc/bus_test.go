package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type busMsg struct {
	Op      string          `json:"op"`
	V       int             `json:"v"`
	Topics  []string        `json:"topics"`
	Methods []string        `json:"methods"`
	Topic   string          `json:"topic"`
	Rev     uint64          `json:"rev"`
	Base    uint64          `json:"base"`
	Seq     uint64          `json:"seq"`
	Dropped int             `json:"dropped"`
	Data    json.RawMessage `json:"data"`
	Patch   json.RawMessage `json:"patch"`
	ID      json.RawMessage `json:"id"`
	OK      bool            `json:"ok"`
	Result  json.RawMessage `json:"result"`
	Error   string          `json:"error"`
	Target  string          `json:"target"`
	Fn      string          `json:"fn"`
	Args    []string        `json:"args"`
}

type busTestClient struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
}

func dialBus(t *testing.T, d *daemon, handshake string, pipelined ...any) *busTestClient {
	t.Helper()
	client, server := net.Pipe()
	go d.handle(server)
	t.Cleanup(func() { _ = client.Close() })
	payload := "bus " + handshake + "\n"
	for _, p := range pipelined {
		b, _ := json.Marshal(p)
		payload += string(b) + "\n"
	}
	_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	return &busTestClient{t: t, conn: client, r: bufio.NewReader(client)}
}

func (c *busTestClient) send(v any) {
	c.t.Helper()
	b, _ := json.Marshal(v)
	_ = c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.conn.Write(append(b, '\n')); err != nil {
		c.t.Fatal(err)
	}
}

func (c *busTestClient) next() busMsg {
	c.t.Helper()
	m, err := c.read(2 * time.Second)
	if err != nil {
		c.t.Fatalf("reading bus message: %v", err)
	}
	return m
}

func (c *busTestClient) read(wait time.Duration) (busMsg, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(wait))
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return busMsg{}, err
	}
	var m busMsg
	if err := json.Unmarshal(line, &m); err != nil {
		c.t.Fatalf("bad bus line %q: %v", line, err)
	}
	return m, nil
}

func (c *busTestClient) quiet() {
	c.t.Helper()
	if m, err := c.read(80 * time.Millisecond); err == nil {
		c.t.Fatalf("unexpected message %+v", m)
	}
}

func shellHello(t *testing.T, d *daemon, pipelined ...any) *busTestClient {
	t.Helper()
	c := dialBus(t, d, `{"v":1,"client":"shell","pid":4242}`, pipelined...)
	if m := c.next(); m.Op != "hello" {
		t.Fatalf("first message = %+v, want hello", m)
	}
	return c
}

func TestBusHandshake(t *testing.T) {
	d := &daemon{}
	d.registerTopic("wm")
	d.registerEventTopic("keypress")
	d.registerCall("settings.patch", func(json.RawMessage) (any, error) { return nil, nil })

	c := dialBus(t, d, `{"v":1,"client":"shell","pid":1}`)
	hello := c.next()
	if hello.Op != "hello" || hello.V != 1 {
		t.Fatalf("hello = %+v", hello)
	}
	if !reflect.DeepEqual(hello.Topics, []string{"keypress", "wm"}) || !reflect.DeepEqual(hello.Methods, []string{"settings.patch"}) {
		t.Fatalf("hello lists topics %v methods %v", hello.Topics, hello.Methods)
	}

	bad := dialBus(t, d, `{"v":2,"client":"shell"}`)
	if m := bad.next(); m.Op != "error" || m.Error != "unsupported bus version" {
		t.Fatalf("v2 reply = %+v", m)
	}
	if _, err := bad.read(time.Second); err == nil {
		t.Fatal("connection stayed open after an unsupported version")
	}
}

func TestBusSubSendsCurrentFramesAndResyncs(t *testing.T) {
	d := &daemon{}
	d.registerTopic("network").publish([]byte(`{"up":true}`))
	d.registerTopic("music").publish([]byte(`{"playing":false}`))

	c := shellHello(t, d, map[string]any{"op": "sub", "topics": []string{"network", "music", "nope"}})
	got := map[string]string{}
	for range 3 {
		m := c.next()
		switch m.Op {
		case "state":
			if m.Rev != 1 || m.Patch != nil {
				t.Fatalf("initial frame %+v, want full rev 1", m)
			}
			got[m.Topic] = string(m.Data)
		case "error":
			if m.Topic != "nope" || m.Error != "unknown topic" {
				t.Fatalf("error = %+v", m)
			}
		}
	}
	if got["network"] != `{"up":true}` || got["music"] != `{"playing":false}` {
		t.Fatalf("initial frames = %v", got)
	}

	d.topic("network").publish([]byte(`{"up":false}`))
	if m := c.next(); m.Topic != "network" || m.Rev != 2 || string(m.Data) != `{"up":false}` {
		t.Fatalf("update = %+v", m)
	}
	c.send(map[string]any{"op": "sub", "topics": []string{"network"}})
	if m := c.next(); m.Topic != "network" || m.Rev != 2 || string(m.Data) != `{"up":false}` {
		t.Fatalf("re-sub frame = %+v, want the full current frame", m)
	}

	c.send(map[string]any{"op": "unsub", "topics": []string{"network"}})
	c.send(map[string]any{"op": "sub", "topics": []string{"music"}})
	if m := c.next(); m.Topic != "music" {
		t.Fatalf("after unsub got %+v", m)
	}
	d.topic("network").publish([]byte(`{"up":true}`))
	c.quiet()

	d.registerTopic("nope").publish([]byte(`{"late":1}`))
	if m := c.next(); m.Topic != "nope" || string(m.Data) != `{"late":1}` {
		t.Fatalf("late topic frame = %+v", m)
	}
}

func TestBusCoalescesStateForSlowReader(t *testing.T) {
	d := &daemon{}
	topic := d.registerTopic("perf")
	topic.publish([]byte(`0`))
	c := shellHello(t, d, map[string]any{"op": "sub", "topics": []string{"perf"}})
	c.next()

	start := time.Now()
	for i := 1; i <= 500; i++ {
		topic.publish([]byte(strconv.Itoa(i)))
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("publishing to a reader that is not reading took %v", elapsed)
	}
	var frames []string
	for {
		m, err := c.read(150 * time.Millisecond)
		if err != nil {
			break
		}
		frames = append(frames, string(m.Data))
	}
	if len(frames) == 0 || frames[len(frames)-1] != "500" {
		t.Fatalf("frames = %v, want the newest last", frames)
	}
	if len(frames) > 3 {
		t.Fatalf("got %d frames for one slow read, want them coalesced", len(frames))
	}
}

func TestBusOrderedEventsDropOldestWithoutBlocking(t *testing.T) {
	d := &daemon{}
	topic := d.registerEventTopic("keypress")
	c := shellHello(t, d, map[string]any{"op": "sub", "topics": []string{"keypress"}})
	// the sub lands once the hello went out and the reader moved on
	busWaitFor(t, "bus subscriber", func() bool { return topic.count() == 1 })

	// one batch can sit in the writer's blocked Write besides the full queue
	const total = 3 * eventQueueDepth
	start := time.Now()
	for i := 1; i <= total; i++ {
		topic.publish([]byte(strconv.Itoa(i)))
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("publishing events to a stalled reader took %v", elapsed)
	}
	received, dropped := 0, 0
	var last uint64
	for {
		m, err := c.read(150 * time.Millisecond)
		if err != nil {
			break
		}
		if m.Op != "event" || m.Seq <= last || string(m.Data) != strconv.FormatUint(m.Seq, 10) {
			t.Fatalf("event %+v after seq %d", m, last)
		}
		if m.Seq != last+1 && uint64(m.Dropped) != m.Seq-last-1 {
			t.Fatalf("gap %d..%d reported dropped=%d", last, m.Seq, m.Dropped)
		}
		last = m.Seq
		received++
		dropped += m.Dropped
	}
	if last != total || dropped == 0 || received+dropped != total {
		t.Fatalf("received %d dropped %d last %d, want %d in total with drops", received, dropped, last, total)
	}
}

func TestLegacyEventSubscriberNeverBlocksProducer(t *testing.T) {
	topic := newEventTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)
	const total = eventQueueDepth + 44
	done := make(chan struct{})
	go func() {
		for i := range total {
			topic.publish([]byte(strconv.Itoa(i)))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("producer blocked on a legacy subscriber that is not reading")
	}
	if got := string(<-sub.frames); got != "44" {
		t.Fatalf("oldest kept event = %s, want 44", got)
	}
	topic.mu.Lock()
	n := sub.dropped
	topic.mu.Unlock()
	if n != 44 {
		t.Fatalf("dropped = %d, want 44", n)
	}

	d := &daemon{}
	keys := d.registerEventTopic("mouse")
	client, server := net.Pipe()
	go d.handle(server)
	defer client.Close()
	if _, err := client.Write([]byte("subscribe mouse\n")); err != nil {
		t.Fatal(err)
	}
	busWaitFor(t, "legacy subscriber", func() bool { return keys.count() == 1 })
	start := time.Now()
	for i := range 10 * eventQueueDepth {
		keys.publish([]byte(`{"n":` + strconv.Itoa(i) + `}`))
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("legacy socket subscriber stalled the producer for %v", elapsed)
	}
}

func TestBusCallsOrderedPerNamespaceParallelAcross(t *testing.T) {
	d := &daemon{}
	release := make(chan struct{})
	var mu sync.Mutex
	var order []string
	note := func(s string) {
		mu.Lock()
		order = append(order, s)
		mu.Unlock()
	}
	d.registerCall("slow.a", func(json.RawMessage) (any, error) {
		<-release
		note("a")
		return "A", nil
	})
	d.registerCall("slow.b", func(raw json.RawMessage) (any, error) {
		note("b")
		return json.RawMessage(raw), nil
	})
	d.registerCall("fast.x", func(json.RawMessage) (any, error) { return nil, errors.New("boom") })

	c := shellHello(t, d,
		map[string]any{"op": "call", "id": 1, "method": "slow.a"},
		map[string]any{"op": "call", "id": 2, "method": "slow.b", "args": map[string]any{"k": "v"}},
		map[string]any{"op": "call", "id": "x", "method": "fast.x"},
		map[string]any{"op": "call", "id": 3, "method": "nope.y"},
	)
	seen := map[string]busMsg{}
	for range 2 {
		m := c.next()
		seen[string(m.ID)] = m
	}
	if m := seen[`"x"`]; m.Op != "reply" || m.OK || m.Error != "boom" {
		t.Fatalf("fast.x reply = %+v, want it while slow.a blocks", m)
	}
	if m := seen["3"]; m.OK || !strings.Contains(m.Error, "unknown method") {
		t.Fatalf("unknown method reply = %+v", m)
	}
	mu.Lock()
	ranB := len(order) > 0
	mu.Unlock()
	if ranB {
		t.Fatal("slow.b ran before slow.a finished")
	}
	close(release)
	a, b := c.next(), c.next()
	if string(a.ID) != "1" || string(a.Result) != `"A"` || string(b.ID) != "2" || string(b.Result) != `{"k":"v"}` {
		t.Fatalf("slow replies = %+v then %+v", a, b)
	}
	if !reflect.DeepEqual(order, []string{"a", "b"}) {
		t.Fatalf("order = %v", order)
	}
}

func TestBusCallRunsAfterClientLeaves(t *testing.T) {
	d := &daemon{}
	ran := make(chan struct{})
	release := make(chan struct{})
	d.registerCall("shells.commit", func(json.RawMessage) (any, error) {
		<-release
		close(ran)
		return nil, nil
	})
	c := shellHello(t, d, map[string]any{"op": "call", "id": 1, "method": "shells.commit"})
	_ = c.conn.Close()
	close(release)
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("a call read before the disconnect did not run")
	}
}

func TestBusRunMapsVerbReplies(t *testing.T) {
	for _, tc := range []struct {
		out, result, err string
	}{
		{"ok", "", ""},
		{"err no such thing", "", "no such thing"},
		{"shell: stopped", "shell: stopped", ""},
	} {
		result, err := runResult(tc.out)
		gotErr := ""
		if err != nil {
			gotErr = err.Error()
		}
		if result != tc.result || gotErr != tc.err {
			t.Fatalf("runResult(%q) = %q, %q", tc.out, result, gotErr)
		}
	}

	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) { return []byte("7\n"), nil, nil })
	d := &daemon{}
	c := shellHello(t, d,
		map[string]any{"op": "run", "id": 1, "line": "ping"},
		map[string]any{"op": "run", "id": 2, "line": "status"},
		map[string]any{"op": "run", "id": 3, "line": "clip-copy 1"},
		map[string]any{"op": "run", "id": 4, "line": "keyring-respond x", "stdin": "secret\n"},
		map[string]any{"op": "run", "id": 5, "line": encodeIPCRequest([]string{"get", "bar", "height"})},
	)
	replies := map[string]busMsg{}
	for range 5 {
		m := c.next()
		replies[string(m.ID)] = m
	}
	if m := replies["1"]; !m.OK || string(m.Result) != `""` {
		t.Fatalf("ping = %+v", m)
	}
	if m := replies["2"]; !m.OK || string(m.Result) != `"shell: stopped"` {
		t.Fatalf("status = %+v", m)
	}
	if m := replies["3"]; m.OK || m.Error != "clipboard not running" {
		t.Fatalf("clip-copy = %+v", m)
	}
	if m := replies["4"]; m.OK || m.Error != "keyring prompter not running" {
		t.Fatalf("keyring-respond = %+v", m)
	}
	if m := replies["5"]; !m.OK || string(m.Result) != `"7"` {
		t.Fatalf("ipc line = %+v", m)
	}
	if n := len(calls()); n != 1 {
		t.Fatalf("qs ran %d times", n)
	}
}

func setRuntimeReady(t *testing.T, pid int) {
	t.Helper()
	ready := filepath.Join(t.TempDir(), "ready")
	if err := os.WriteFile(ready, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		t.Fatal(err)
	}
	runtimeIPCReadyFile.Store(ready)
	t.Cleanup(func() { runtimeIPCReadyFile.Store("") })
}

func TestIpcInvokeRoutingOrder(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) { return []byte("qs\n"), nil, nil })

	older := fakeBusClient(t, "shell", 0, "bar")
	older.handle = func(busCmd) (string, error) { return "older", nil }
	newer := fakeBusClient(t, "shell", 0, "bar", "shell")
	newer.handle = func(c busCmd) (string, error) { return "newer:" + strings.Join(c.Args, ","), nil }
	hub := fakeBusClient(t, "hub", 0, "nav")
	hub.handle = func(c busCmd) (string, error) { return "hub:" + c.Prop, nil }

	if v, err := ipcInvoke("shell", "bar", "move", "", []string{"clock", ""}); err != nil || v != "newer:clock," {
		t.Fatalf("by client name = %q, %v; want the newest serving client", v, err)
	}
	if got := newer.cmds(); len(got) != 1 || got[0].Fn != "move" || !reflect.DeepEqual(got[0].Args, []string{"clock", ""}) {
		t.Fatalf("cmd = %+v", got)
	}
	if v, err := ipcInvoke("hub", "nav", "", "section", nil); err != nil || v != "hub:section" {
		t.Fatalf("prop read = %q, %v", v, err)
	}
	if !busServing("hub", "nav") || busServing("hub", "bar") || busServing("keys", "nav") {
		t.Fatal("busServing disagrees with the registry")
	}
	if v, err := ipcInvoke("shell", "theme", "reload", "", nil); err != nil || v != "qs" {
		t.Fatalf("unserved target = %q, %v; want the qs fallback", v, err)
	}
	if got := calls(); len(got) != 1 || !reflect.DeepEqual(got[0], []string{"-c", "shell", "ipc", "call", "theme", "reload"}) {
		t.Fatalf("qs argv = %v", got)
	}
	if n := len(older.cmds()); n != 0 {
		t.Fatalf("older client got %d cmds", n)
	}

	// a ready item runtime answers shell calls by pid even over a newer client named shell
	pid := os.Getpid()
	setRuntimeReady(t, pid)
	runtime := fakeBusClient(t, "item", pid, "bar")
	runtime.handle = func(busCmd) (string, error) { return "runtime", nil }
	if v, err := ipcInvoke("shell", "bar", "move", "", nil); err != nil || v != "runtime" {
		t.Fatalf("runtime route = %q, %v", v, err)
	}
	if got := runtime.cmds(); len(got) != 1 || got[0].Args == nil || len(got[0].Args) != 0 {
		t.Fatalf("runtime cmd = %+v, want an empty args array", got)
	}
	if _, err := ipcInvoke("shell", "shell", "openSurface", "", []string{"DP-1"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"ipc", "--pid", strconv.Itoa(pid), "call", "shell", "openSurface", "DP-1"}
	if got := calls(); len(got) != 2 || !reflect.DeepEqual(got[1], want) {
		t.Fatalf("runtime qs argv = %v, want %v", got, want)
	}
	if n := len(newer.cmds()); n != 1 {
		t.Fatalf("resident shell got %d cmds while the runtime is ready", n)
	}
}

func TestIpcInvokeBusErrorsAreFinal(t *testing.T) {
	calls := fakeQs(t, func(context.Context, int) ([]byte, []byte, error) { return []byte("qs\n"), nil, nil })
	prev := busCmdTimeout
	busCmdTimeout = 150 * time.Millisecond
	t.Cleanup(func() { busCmdTimeout = prev })

	hung := fakeBusClient(t, "keys", 0, "hung")
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	hung.handle = func(busCmd) (string, error) { <-block; return "", nil }
	start := time.Now()
	_, err := ipcInvoke("keys", "hung", "f", "", nil)
	var ierr *ipcError
	if !errors.As(err, &ierr) || ierr.kind != ipcTimedOut || err.Error() != "timeout" {
		t.Fatalf("hung client err = %v, want timeout", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timeout took %v", elapsed)
	}

	missing := fakeBusClient(t, "keys", 0, "gone")
	missing.handle = func(busCmd) (string, error) { return "", errors.New("function not found") }
	if _, err := ipcInvoke("keys", "gone", "f", "", nil); !errors.As(err, &ierr) || ierr.kind != ipcMissing {
		t.Fatalf("missing function err = %v", err)
	}
	if got := ipcCallN("keys", "gone", "f"); got != "err qs ipc keys/f: function not found" {
		t.Fatalf("ipcCallN = %q", got)
	}

	drop := fakeBusClient(t, "keys", 0, "drop")
	drop.handle = func(busCmd) (string, error) { _ = drop.conn.Close(); return "never sent", nil }
	if _, err := ipcInvoke("keys", "drop", "f", "", nil); err == nil || err.Error() != "disconnected" {
		t.Fatalf("disconnect err = %v", err)
	}
	if n := len(calls()); n != 0 {
		t.Fatalf("qs ran %d times after a delivered cmd", n)
	}
}

func TestBusIpcOpAndServe(t *testing.T) {
	fakeQs(t, func(context.Context, int) ([]byte, []byte, error) {
		return nil, []byte("No running instances for shell\n"), errors.New("exit status 255")
	})
	d := &daemon{}
	served := shellHello(t, d, map[string]any{"op": "serve", "targets": []string{"osd"}})
	if !busAwaitServing("shell", "osd", time.Second) {
		t.Fatal("serve did not register")
	}
	caller := dialBus(t, d, `{"v":1,"client":"keys"}`,
		map[string]any{"op": "ipc", "id": 9, "target": "osd", "fn": "show", "args": []any{"volume", 40, true}})
	caller.next() // hello

	cmd := served.next()
	if cmd.Op != "cmd" || cmd.Target != "osd" || cmd.Fn != "show" || !reflect.DeepEqual(cmd.Args, []string{"volume", "40", "true"}) {
		t.Fatalf("served got %+v", cmd)
	}
	served.send(map[string]any{"op": "done", "id": json.RawMessage(cmd.ID), "ok": true, "result": "shown"})
	m := caller.next()
	if m.Op != "reply" || string(m.ID) != "9" || !m.OK || string(m.Result) != `"shown"` {
		t.Fatalf("ipc reply = %+v", m)
	}

	served.send(map[string]any{"op": "unserve", "targets": []string{"osd"}})
	busWaitFor(t, "unserve", func() bool { return !busServing("shell", "osd") })
	caller.send(map[string]any{"op": "ipc", "id": 10, "target": "osd", "prop": "visible"})
	if m := caller.next(); m.OK || !strings.Contains(m.Error, "no shell is running") {
		t.Fatalf("unserved ipc = %+v, want the qs path's not-running error", m)
	}

	if busAwaitServing("shell", "never", 50*time.Millisecond) {
		t.Fatal("busAwaitServing reported a target nobody serves")
	}
	go func() {
		time.Sleep(30 * time.Millisecond)
		served.send(map[string]any{"op": "serve", "targets": []string{"later"}})
	}()
	if !busAwaitServing("shell", "later", time.Second) {
		t.Fatal("busAwaitServing missed a serve")
	}
}

func TestOnDemandAcrossLegacyAndBus(t *testing.T) {
	d := &daemon{}
	topic := d.registerTopic("weather")
	events := make(chan bool, 16)
	topic.onDemand(func(active bool) { events <- active })
	expect := func(want ...bool) {
		t.Helper()
		for _, w := range want {
			select {
			case got := <-events:
				if got != w {
					t.Fatalf("demand = %v, want %v", got, w)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("no demand transition, want %v", w)
			}
		}
		select {
		case got := <-events:
			t.Fatalf("extra demand transition %v", got)
		case <-time.After(30 * time.Millisecond):
		}
	}

	legacy := topic.subscribe()
	expect(true)
	c := shellHello(t, d, map[string]any{"op": "sub", "topics": []string{"weather"}})
	busWaitFor(t, "bus subscriber", func() bool { return topic.count() == 2 })
	topic.unsubscribe(legacy)
	expect()
	c.send(map[string]any{"op": "unsub", "topics": []string{"weather"}})
	expect(false)
	c.send(map[string]any{"op": "sub", "topics": []string{"weather"}})
	expect(true)
	_ = c.conn.Close()
	expect(false)
	if n := topic.count(); n != 0 {
		t.Fatalf("count = %d after everyone left", n)
	}

	late := newStateTopic()
	sub := late.subscribe()
	defer late.unsubscribe(sub)
	lateEvents := make(chan bool, 1)
	late.onDemand(func(active bool) { lateEvents <- active })
	if !<-lateEvents {
		t.Fatal("onDemand on a topic with subscribers did not report true")
	}
}

func applyPatch(t *testing.T, doc map[string]any, patch json.RawMessage) {
	t.Helper()
	var ops []struct {
		P []string        `json:"p"`
		V json.RawMessage `json:"v"`
		X bool            `json:"x"`
	}
	if err := json.Unmarshal(patch, &ops); err != nil {
		t.Fatalf("bad patch %s: %v", patch, err)
	}
	for _, op := range ops {
		parent := doc
		for _, k := range op.P[:len(op.P)-1] {
			child, ok := parent[k].(map[string]any)
			if !ok {
				child = map[string]any{}
				parent[k] = child
			}
			parent = child
		}
		key := op.P[len(op.P)-1]
		if op.X {
			delete(parent, key)
			continue
		}
		var v any
		if err := json.Unmarshal(op.V, &v); err != nil {
			t.Fatalf("bad value in %s: %v", patch, err)
		}
		parent[key] = v
	}
}

func decodeFrame(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("bad frame %s: %v", b, err)
	}
	return m
}

func TestBusDeltaFramesReproduceFullFrame(t *testing.T) {
	pad := strings.Repeat("x", 2000)
	frames := []string{
		`{"bar":{"style":"orpheus","height":32,"items":["a","b"]},"theme":{"mode":"dark"},"pad":"` + pad + `","gone":1}`,
		`{"bar":{"style":"kairos","height":32,"items":["a","b"]},"theme":{"mode":"dark"},"pad":"` + pad + `","gone":1}`,
		`{"bar":{"style":"kairos","height":32,"items":["a","c","d"]},"theme":{"mode":"dark","accent":null},"pad":"` + pad + `"}`,
		`{"bar":{"style":"kairos","height":32},"theme":[1,2],"pad":"` + pad + `","new":{"deep":{"k":true}}}`,
	}
	d := &daemon{}
	topic := d.registerDeltaTopic("settings")
	topic.publish([]byte(frames[0]))
	c := shellHello(t, d, map[string]any{"op": "sub", "topics": []string{"settings"}})
	first := c.next()
	if first.Patch != nil || first.Rev != 1 {
		t.Fatalf("first frame = %+v, want full", first)
	}
	doc := decodeFrame(t, first.Data)
	rev := first.Rev
	for i, f := range frames[1:] {
		topic.publish([]byte(f))
		m := c.next()
		if m.Patch == nil {
			t.Fatalf("frame %d came full, want a patch", i+1)
		}
		if m.Base != rev || m.Rev != rev+1 {
			t.Fatalf("patch rev %d base %d, client at %d", m.Rev, m.Base, rev)
		}
		if len(m.Patch)*2 > len(f) {
			t.Fatalf("patch %d bytes for a %d byte frame", len(m.Patch), len(f))
		}
		applyPatch(t, doc, m.Patch)
		if want := decodeFrame(t, []byte(f)); !reflect.DeepEqual(doc, want) {
			t.Fatalf("patched frame %d = %v, want %v (patch %s)", i+1, doc, want, m.Patch)
		}
		rev = m.Rev
	}

	small := d.registerDeltaTopic("small")
	small.publish([]byte(`{"a":1,"b":2}`))
	c.send(map[string]any{"op": "sub", "topics": []string{"small"}})
	c.next()
	small.publish([]byte(`{"a":3,"b":4}`))
	if m := c.next(); m.Patch != nil || string(m.Data) != `{"a":3,"b":4}` {
		t.Fatalf("large patch went out as %+v", m)
	}

	c.send(map[string]any{"op": "sub", "topics": []string{"settings"}})
	if m := c.next(); m.Patch != nil || m.Rev != rev || !reflect.DeepEqual(decodeFrame(t, m.Data), decodeFrame(t, []byte(frames[3]))) {
		t.Fatalf("resync = %+v", m)
	}
}

func TestBusOutboxSkippedRevisionGoesFull(t *testing.T) {
	o := newBusOutbox()
	take := func() []busMsg {
		var b bytes.Buffer
		o.take(&b)
		var out []busMsg
		for _, line := range strings.Split(strings.TrimSpace(b.String()), "\n") {
			if line == "" {
				continue
			}
			var m busMsg
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				t.Fatalf("bad line %q", line)
			}
			out = append(out, m)
		}
		return out
	}
	patch := []byte(`[{"p":["k"],"v":1}]`)
	o.pushState("s", topicMsg{rev: 1, data: []byte(`{"k":0}`)})
	if got := take(); len(got) != 1 || got[0].Data == nil {
		t.Fatalf("first take = %+v", got)
	}
	o.pushState("s", topicMsg{rev: 2, base: 1, data: []byte(`{"k":1}`), patch: patch})
	o.pushState("s", topicMsg{rev: 3, base: 2, data: []byte(`{"k":2}`), patch: patch})
	if got := take(); len(got) != 1 || got[0].Rev != 3 || got[0].Patch != nil || string(got[0].Data) != `{"k":2}` {
		t.Fatalf("coalesced take = %+v, want rev 3 full", got)
	}
	o.pushState("s", topicMsg{rev: 4, base: 3, data: []byte(`{"k":1}`), patch: patch})
	if got := take(); len(got) != 1 || got[0].Patch == nil || got[0].Base != 3 {
		t.Fatalf("in-step take = %+v, want a patch on base 3", got)
	}

	for i := 1; i <= eventQueueDepth+5; i++ {
		o.pushEvent("e", topicMsg{rev: uint64(i), data: []byte(strconv.Itoa(i))})
	}
	got := take()
	if len(got) != eventQueueDepth || got[0].Seq != 6 || got[0].Dropped != 5 || got[1].Dropped != 0 {
		t.Fatalf("events: %d, first %+v", len(got), got[0])
	}
}
