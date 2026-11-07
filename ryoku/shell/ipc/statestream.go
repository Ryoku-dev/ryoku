package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// statestream carries typed subsystem state to QML over the same control socket
// the CLI verbs use, so a QML singleton is a view of daemon state rather than an
// independent poller. Two legacy message families ride the socket beside the
// one-shot command lines:
//
//	subscribe <topic>            long-lived: the daemon writes a full-state JSON
//	                             frame at once, then a fresh frame on every
//	                             change. Byte-identical frames are suppressed, so
//	                             an unchanged value never wakes a binding. One
//	                             line per frame.
//	call <topic>.<method> <json> request/response: the daemon runs the method
//	                             and replies with one JSON line; the connection
//	                             stays open for further calls. An "id" in the
//	                             args is echoed back so a multiplexed client can
//	                             correlate replies.
//
// A `bus` connection (bus.go) multiplexes both, plus verbs and IpcHandler
// calls, over one socket per process.
//
// State flows one way (daemon to QML) through subscribe; intent flows the other
// (QML to daemon) through call.

// eventQueueDepth bounds the unread events one subscriber may hold. A slow
// reader loses its oldest events instead of stalling the producer.
const eventQueueDepth = 256

// topicSubscriber is one subscription. A legacy line subscriber reads frames;
// a bus subscriber (bus != nil) has frames pushed into its connection outbox.
type topicSubscriber struct {
	frames   chan []byte
	done     chan struct{}
	stopOnce sync.Once
	dropped  int // events lost to a full queue; guarded by the topic mutex

	bus  *busConn
	name string
}

func (s *topicSubscriber) stop() {
	s.stopOnce.Do(func() { close(s.done) })
}

// topicMsg is one published revision. For delta topics patch turns the frame
// at base into data; it is nil when a full frame is the better send.
type topicMsg struct {
	rev   uint64
	base  uint64
	data  []byte
	patch []byte
}

// stateTopic fans a subsystem's state out to subscribers. Ordinary state
// topics coalesce unread snapshots; ordered topics keep every event in a
// bounded queue per subscriber, because a later keypress cannot replace an
// earlier one. Delivery never blocks the publisher.
type stateTopic struct {
	mu      sync.Mutex
	subs    map[*topicSubscriber]struct{}
	last    []byte
	ordered bool
	delta   bool
	rev     uint64
	lastMsg topicMsg
	busSubs int

	demandFn func(active bool)
	demandQ  []bool
	draining bool
}

func newStateTopic() *stateTopic {
	return &stateTopic{subs: map[*topicSubscriber]struct{}{}}
}

func newEventTopic() *stateTopic {
	return &stateTopic{subs: map[*topicSubscriber]struct{}{}, ordered: true}
}

func newDeltaTopic() *stateTopic {
	return &stateTopic{subs: map[*topicSubscriber]struct{}{}, delta: true}
}

// onDemand registers fn to run with true when the first subscriber (legacy or
// bus) arrives and false when the last leaves. Calls happen outside the topic
// lock and in transition order. A topic that already has subscribers reports
// true at once.
func (t *stateTopic) onDemand(fn func(active bool)) {
	t.mu.Lock()
	t.demandFn = fn
	if len(t.subs) > 0 {
		t.demandQ = append(t.demandQ, true)
	}
	t.mu.Unlock()
	t.drainDemand()
}

// count reports how many subscribers the topic has.
func (t *stateTopic) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.subs)
}

// drainDemand runs queued demand transitions. Whoever finds the queue idle
// drains it, so callbacks stay ordered and a callback that subscribes again
// queues instead of deadlocking.
func (t *stateTopic) drainDemand() {
	t.mu.Lock()
	if t.draining {
		t.mu.Unlock()
		return
	}
	t.draining = true
	for len(t.demandQ) > 0 {
		active := t.demandQ[0]
		t.demandQ = t.demandQ[1:]
		fn := t.demandFn
		t.mu.Unlock()
		if fn != nil {
			fn(active)
		}
		t.mu.Lock()
	}
	t.draining = false
	t.mu.Unlock()
}

// addLocked registers sub and queues the demand transition it causes.
func (t *stateTopic) addLocked(sub *topicSubscriber) {
	t.subs[sub] = struct{}{}
	if sub.bus != nil {
		t.busSubs++
	}
	if len(t.subs) == 1 && t.demandFn != nil {
		t.demandQ = append(t.demandQ, true)
	}
}

func (t *stateTopic) removeLocked(sub *topicSubscriber) bool {
	if _, ok := t.subs[sub]; !ok {
		return false
	}
	delete(t.subs, sub)
	if sub.bus != nil {
		t.busSubs--
	}
	if len(t.subs) == 0 && t.demandFn != nil {
		t.demandQ = append(t.demandQ, false)
	}
	return true
}

func (t *stateTopic) subscribe() *topicSubscriber {
	depth := 1
	if t.ordered {
		depth = eventQueueDepth
	}
	sub := &topicSubscriber{
		frames: make(chan []byte, depth),
		done:   make(chan struct{}),
	}
	t.mu.Lock()
	t.addLocked(sub)
	if t.last != nil {
		sub.frames <- t.last
	}
	t.mu.Unlock()
	t.drainDemand()
	return sub
}

func (t *stateTopic) unsubscribe(sub *topicSubscriber) {
	t.mu.Lock()
	t.removeLocked(sub)
	t.mu.Unlock()
	sub.stop()
	t.drainDemand()
}

// subscribeBus attaches a bus connection under the topic name it asked for
// and queues the current frame, so no publish can slip in between.
func (t *stateTopic) subscribeBus(c *busConn, name string) *topicSubscriber {
	sub := &topicSubscriber{done: make(chan struct{}), bus: c, name: name}
	t.mu.Lock()
	t.addLocked(sub)
	t.replayLocked(sub)
	t.mu.Unlock()
	t.drainDemand()
	return sub
}

// resync sends a bus subscriber the current frame in full again.
func (t *stateTopic) resync(sub *topicSubscriber) {
	t.mu.Lock()
	if _, ok := t.subs[sub]; ok {
		t.replayLocked(sub)
	}
	t.mu.Unlock()
}

func (t *stateTopic) replayLocked(sub *topicSubscriber) {
	if t.last == nil {
		sub.bus.out.resetState(sub.name)
		return
	}
	if t.ordered {
		sub.bus.out.pushEvent(sub.name, t.lastMsg)
		return
	}
	full := t.lastMsg
	full.patch = nil
	sub.bus.out.resetState(sub.name)
	sub.bus.out.pushState(sub.name, full)
}

func (t *stateTopic) publish(frame []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ordered && bytes.Equal(frame, t.last) {
		return
	}
	prev := t.last
	t.rev++
	msg := topicMsg{rev: t.rev, data: frame}
	// Patches only matter to bus subscribers; skip the diff when none listen.
	if t.delta && prev != nil && t.busSubs > 0 {
		msg.base = t.rev - 1
		if patch := framePatch(prev, frame); patch != nil && 2*len(patch) <= len(frame) {
			msg.patch = patch
		}
	}
	t.last = frame
	t.lastMsg = msg
	for sub := range t.subs {
		t.deliverLocked(sub, msg)
	}
}

func (t *stateTopic) deliverLocked(sub *topicSubscriber, msg topicMsg) {
	if sub.bus != nil {
		if t.ordered {
			sub.bus.out.pushEvent(sub.name, msg)
		} else {
			sub.bus.out.pushState(sub.name, msg)
		}
		return
	}
	// Every send happens under t.mu, so after one receive there is room.
	select {
	case sub.frames <- msg.data:
		return
	default:
	}
	select {
	case <-sub.frames:
		if t.ordered {
			sub.dropped++
		}
	default:
	}
	select {
	case sub.frames <- msg.data:
	default:
	}
}

func (d *daemon) installTopic(name string, topic *stateTopic) *stateTopic {
	d.topicsMu.Lock()
	if d.topics == nil {
		d.topics = map[string]*stateTopic{}
	}
	d.topics[name] = topic
	d.topicsMu.Unlock()
	busReg.topicInstalled(d, name, topic)
	return topic
}

// registerTopic creates a coalescing state topic. Event producers whose frames
// are individually meaningful must use registerEventTopic instead.
func (d *daemon) registerTopic(name string) *stateTopic {
	return d.installTopic(name, newStateTopic())
}

func (d *daemon) registerEventTopic(name string) *stateTopic {
	return d.installTopic(name, newEventTopic())
}

// registerDeltaTopic creates a state topic whose bus subscribers receive
// patches against the previous frame when that is clearly smaller.
func (d *daemon) registerDeltaTopic(name string) *stateTopic {
	return d.installTopic(name, newDeltaTopic())
}

func (d *daemon) topic(name string) *stateTopic {
	d.topicsMu.Lock()
	defer d.topicsMu.Unlock()
	return d.topics[name]
}

// callFunc handles one control method. args is the raw JSON object from the call
// line (it may carry an "id" the framing echoes back and the handler ignores).
type callFunc func(args json.RawMessage) (any, error)

func (d *daemon) registerCall(method string, fn callFunc) {
	d.callsMu.Lock()
	defer d.callsMu.Unlock()
	if d.calls == nil {
		d.calls = map[string]callFunc{}
	}
	d.calls[method] = fn
}

func (d *daemon) callHandler(method string) callFunc {
	d.callsMu.Lock()
	defer d.callsMu.Unlock()
	return d.calls[method]
}

// serveSubscription streams one topic to a client until it disconnects. The
// control socket's per-command deadline is cleared first: a subscription is
// long-lived, unlike the one-shot verbs handle() otherwise bounds.
func (d *daemon) serveSubscription(conn net.Conn, cmd string) {
	name := strings.TrimSpace(strings.TrimPrefix(cmd, "subscribe"))
	t := d.topic(name)
	if t == nil {
		fmt.Fprintf(conn, "err unknown topic: %s\n", name)
		return
	}
	_ = conn.SetDeadline(time.Time{})
	sub := t.subscribe()
	defer t.unsubscribe(sub)
	done := make(chan struct{})
	go func() {
		// Any further input, or a half-close, ends the stream.
		_, _ = io.Copy(io.Discard, conn)
		close(done)
	}()
	for {
		select {
		case frame := <-sub.frames:
			if _, err := conn.Write(append(frame, '\n')); err != nil {
				return
			}
		case <-done:
			return
		case <-d.quit:
			return
		}
	}
}

// serveCalls runs control methods on one connection until it closes. first is
// the "call ..." line handle() already read.
func (d *daemon) serveCalls(conn net.Conn, r *bufio.Reader, first string) {
	_ = conn.SetDeadline(time.Time{})
	line := first
	for {
		if strings.HasPrefix(line, "call ") {
			if _, err := fmt.Fprintln(conn, d.handleCall(line)); err != nil {
				return
			}
		}
		next, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(next)
	}
}

func (d *daemon) handleCall(line string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "call "))
	method := rest
	var raw json.RawMessage
	if sp := strings.IndexByte(rest, ' '); sp >= 0 {
		method = rest[:sp]
		raw = json.RawMessage(strings.TrimSpace(rest[sp+1:]))
	}
	id := callID(raw)
	h := d.callHandler(method)
	if h == nil {
		return callReply(id, nil, fmt.Errorf("unknown method: %s", method))
	}
	result, err := h(raw)
	return callReply(id, result, err)
}

func callID(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m["id"]
}

func callReply(id json.RawMessage, result any, err error) string {
	reply := map[string]any{"ok": err == nil}
	if len(id) > 0 {
		reply["id"] = id
	}
	if err != nil {
		reply["error"] = err.Error()
	} else if result != nil {
		reply["result"] = result
	}
	b, e := json.Marshal(reply)
	if e != nil {
		return `{"ok":false,"error":"reply marshal failed"}`
	}
	return string(b)
}

// framePatch returns the patch ops that turn prev into next, as a JSON array,
// or nil when either frame is not a JSON object. Unchanged subtrees are
// skipped by comparing their bytes; arrays and scalars are replaced whole.
func framePatch(prev, next []byte) []byte {
	if !isJSONObject(prev) || !isJSONObject(next) {
		return nil
	}
	var ops bytes.Buffer
	ops.WriteByte('[')
	if !diffJSON(&ops, nil, prev, next) {
		return nil
	}
	ops.WriteByte(']')
	return ops.Bytes()
}

func isJSONObject(b []byte) bool {
	b = bytes.TrimLeft(b, " \t\r\n")
	return len(b) > 0 && b[0] == '{'
}

// diffJSON appends the ops for one subtree. It reports false when a frame
// does not parse, so the caller falls back to a full frame.
func diffJSON(ops *bytes.Buffer, path []string, prev, next json.RawMessage) bool {
	if bytes.Equal(prev, next) {
		return true
	}
	if len(path) > 0 && (!isJSONObject(prev) || !isJSONObject(next)) {
		writePatchOp(ops, path, next)
		return true
	}
	var a, b map[string]json.RawMessage
	if json.Unmarshal(prev, &a) != nil || json.Unmarshal(next, &b) != nil {
		return false
	}
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		sub := append(path[:len(path):len(path)], k)
		va, inA := a[k]
		vb, inB := b[k]
		switch {
		case !inB:
			writePatchOp(ops, sub, nil)
		case !inA:
			writePatchOp(ops, sub, vb)
		default:
			if !diffJSON(ops, sub, va, vb) {
				return false
			}
		}
	}
	return true
}

// writePatchOp appends a set op, or a delete op when value is nil.
func writePatchOp(ops *bytes.Buffer, path []string, value json.RawMessage) {
	if ops.Len() > 1 {
		ops.WriteByte(',')
	}
	p, _ := json.Marshal(path)
	ops.WriteString(`{"p":`)
	ops.Write(p)
	if value == nil {
		ops.WriteString(`,"x":true}`)
		return
	}
	ops.WriteString(`,"v":`)
	ops.Write(value)
	ops.WriteByte('}')
}
