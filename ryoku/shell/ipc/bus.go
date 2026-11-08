package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

const busVersion = 1

var busCmdTimeout = 3 * time.Second

type busReq struct {
	Op      string          `json:"op"`
	ID      json.RawMessage `json:"id"`
	Topics  []string        `json:"topics"`
	Method  string          `json:"method"`
	Args    json.RawMessage `json:"args"`
	Line    string          `json:"line"`
	Stdin   string          `json:"stdin"`
	Target  string          `json:"target"`
	Fn      string          `json:"fn"`
	Prop    string          `json:"prop"`
	Client  string          `json:"client"`
	Targets []string        `json:"targets"`
	OK      bool            `json:"ok"`
	Result  json.RawMessage `json:"result"`
	Error   string          `json:"error"`
}

type busReply struct {
	Op     string          `json:"op"`
	ID     json.RawMessage `json:"id,omitempty"`
	OK     bool            `json:"ok"`
	Result any             `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type busErrorMsg struct {
	Op    string `json:"op"`
	Topic string `json:"topic,omitempty"`
	Error string `json:"error"`
}

// Args is a pointer so a function call always carries an array, even empty
type busCmdMsg struct {
	Op     string    `json:"op"`
	ID     uint64    `json:"id"`
	Target string    `json:"target"`
	Fn     string    `json:"fn,omitempty"`
	Prop   string    `json:"prop,omitempty"`
	Args   *[]string `json:"args,omitempty"`
}

type busDone struct {
	value string
	err   error
}

type busOutbox struct {
	mu     sync.Mutex
	wake   chan struct{}
	closed bool
	items  []busItem
	states map[string]*busStateSlot
	events map[string]*busEventQueue
}

type busItemKind uint8

const (
	busItemRaw busItemKind = iota
	busItemState
	busItemEvent
)

type busItem struct {
	kind  busItemKind
	raw   []byte
	topic string
}

type busStateSlot struct {
	pending bool
	msg     topicMsg
	sent    uint64 // rev the client holds; 0 when unknown
}

type busEventQueue struct {
	queue   []topicMsg
	dropped int
}

func newBusOutbox() *busOutbox {
	return &busOutbox{
		wake:   make(chan struct{}, 1),
		states: map[string]*busStateSlot{},
		events: map[string]*busEventQueue{},
	}
}

func (o *busOutbox) signal() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

func (o *busOutbox) pushRaw(b []byte) bool {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return false
	}
	o.items = append(o.items, busItem{kind: busItemRaw, raw: b})
	o.mu.Unlock()
	o.signal()
	return true
}

func (o *busOutbox) slotLocked(topic string) *busStateSlot {
	s := o.states[topic]
	if s == nil {
		s = &busStateSlot{}
		o.states[topic] = s
	}
	return s
}

func (o *busOutbox) resetState(topic string) {
	o.mu.Lock()
	o.slotLocked(topic).sent = 0
	o.mu.Unlock()
}

func (o *busOutbox) pushState(topic string, msg topicMsg) {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return
	}
	s := o.slotLocked(topic)
	s.msg = msg
	if !s.pending {
		s.pending = true
		o.items = append(o.items, busItem{kind: busItemState, topic: topic})
	}
	o.mu.Unlock()
	o.signal()
}

func (o *busOutbox) pushEvent(topic string, msg topicMsg) {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return
	}
	q := o.events[topic]
	if q == nil {
		q = &busEventQueue{}
		o.events[topic] = q
	}
	// the dropped event's marker now stands for the next one so markers and events stay one to one
	if len(q.queue) >= eventQueueDepth {
		q.queue = q.queue[1:]
		q.dropped++
	} else {
		o.items = append(o.items, busItem{kind: busItemEvent, topic: topic})
	}
	q.queue = append(q.queue, msg)
	o.mu.Unlock()
	o.signal()
}

func (o *busOutbox) dropTopic(topic string) {
	o.mu.Lock()
	delete(o.states, topic)
	delete(o.events, topic)
	o.mu.Unlock()
}

func (o *busOutbox) close() {
	o.mu.Lock()
	o.closed = true
	o.items = nil
	o.mu.Unlock()
	o.signal()
}

func (o *busOutbox) take(buf *bytes.Buffer) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	items := o.items
	o.items = nil
	for _, it := range items {
		switch it.kind {
		case busItemRaw:
			buf.Write(it.raw)
			buf.WriteByte('\n')
		case busItemState:
			s := o.states[it.topic]
			if s == nil || !s.pending {
				continue
			}
			s.pending = false
			delta := s.msg.patch != nil && s.sent != 0 && s.msg.base == s.sent
			s.sent = s.msg.rev
			writeStateMsg(buf, it.topic, s.msg, delta)
		case busItemEvent:
			q := o.events[it.topic]
			if q == nil || len(q.queue) == 0 {
				continue
			}
			msg := q.queue[0]
			q.queue = q.queue[1:]
			writeEventMsg(buf, it.topic, msg, q.dropped)
			q.dropped = 0
		}
	}
	return !o.closed
}

func writeJSONString(buf *bytes.Buffer, s string) {
	b, _ := json.Marshal(s)
	buf.Write(b)
}

func writeFrameData(buf *bytes.Buffer, data []byte) {
	if len(data) == 0 {
		buf.WriteString("null")
		return
	}
	buf.Write(data)
}

func writeStateMsg(buf *bytes.Buffer, topic string, msg topicMsg, delta bool) {
	buf.WriteString(`{"op":"state","topic":`)
	writeJSONString(buf, topic)
	fmt.Fprintf(buf, `,"rev":%d`, msg.rev)
	if delta {
		fmt.Fprintf(buf, `,"base":%d,"patch":`, msg.base)
		buf.Write(msg.patch)
	} else {
		buf.WriteString(`,"data":`)
		writeFrameData(buf, msg.data)
	}
	buf.WriteString("}\n")
}

func writeEventMsg(buf *bytes.Buffer, topic string, msg topicMsg, dropped int) {
	buf.WriteString(`{"op":"event","topic":`)
	writeJSONString(buf, topic)
	fmt.Fprintf(buf, `,"seq":%d`, msg.rev)
	if dropped > 0 {
		fmt.Fprintf(buf, `,"dropped":%d`, dropped)
	}
	buf.WriteString(`,"data":`)
	writeFrameData(buf, msg.data)
	buf.WriteString("}\n")
}

type busConn struct {
	d         *daemon
	conn      net.Conn
	client    string
	pid       int
	out       *busOutbox
	done      chan struct{}
	closeOnce sync.Once

	subMu  sync.Mutex
	subs   map[string]busTopicSub // nil once closed
	wanted map[string]bool        // topics asked for before they were registered

	nsMu    sync.Mutex
	workers map[string]*busWorker

	pendMu  sync.Mutex
	pending map[uint64]chan busDone
	gone    bool

	serving map[string]bool // guarded by busReg.mu
}

type busTopicSub struct {
	topic *stateTopic
	sub   *topicSubscriber
}

type busWorker struct {
	jobs []func()
}

func newBusConn(d *daemon, conn net.Conn, client string, pid int) *busConn {
	return &busConn{
		d:       d,
		conn:    conn,
		client:  client,
		pid:     pid,
		out:     newBusOutbox(),
		done:    make(chan struct{}),
		subs:    map[string]busTopicSub{},
		wanted:  map[string]bool{},
		workers: map[string]*busWorker{},
		pending: map[uint64]chan busDone{},
		serving: map[string]bool{},
	}
}

// lines the client pipelined behind the handshake are already in r
func (d *daemon) serveBus(conn net.Conn, r *bufio.Reader, handshake string) {
	_ = conn.SetDeadline(time.Time{})
	var h struct {
		V      int    `json:"v"`
		Client string `json:"client"`
		Pid    int    `json:"pid"`
	}
	if err := json.Unmarshal([]byte(handshake), &h); err != nil {
		writeBusLine(conn, busErrorMsg{Op: "error", Error: "malformed handshake"})
		return
	}
	if h.V != busVersion {
		writeBusLine(conn, busErrorMsg{Op: "error", Error: "unsupported bus version"})
		return
	}
	c := newBusConn(d, conn, h.Client, h.Pid)
	c.out.pushRaw(d.busHello())
	busReg.add(c)
	defer c.close()
	go c.writeLoop()
	go func() {
		select {
		case <-d.quit:
			c.close()
		case <-c.done:
		}
	}()
	for {
		line, err := r.ReadBytes('\n')
		if l := bytes.TrimSpace(line); len(l) > 0 {
			c.handle(l)
		}
		if err != nil {
			return
		}
	}
}

func writeBusLine(conn net.Conn, v any) {
	b, _ := json.Marshal(v)
	_, _ = conn.Write(append(b, '\n'))
}

func (d *daemon) busHello() []byte {
	d.topicsMu.Lock()
	topics := make([]string, 0, len(d.topics))
	for name := range d.topics {
		topics = append(topics, name)
	}
	d.topicsMu.Unlock()
	d.callsMu.Lock()
	methods := make([]string, 0, len(d.calls))
	for name := range d.calls {
		methods = append(methods, name)
	}
	d.callsMu.Unlock()
	sort.Strings(topics)
	sort.Strings(methods)
	b, _ := json.Marshal(struct {
		Op      string   `json:"op"`
		V       int      `json:"v"`
		Topics  []string `json:"topics"`
		Methods []string `json:"methods"`
	}{"hello", busVersion, topics, methods})
	return b
}

func (c *busConn) writeLoop() {
	var buf bytes.Buffer
	for {
		select {
		case <-c.out.wake:
		case <-c.done:
			return
		}
		buf.Reset()
		open := c.out.take(&buf)
		if buf.Len() > 0 {
			if _, err := c.conn.Write(buf.Bytes()); err != nil {
				c.close()
				return
			}
		}
		if !open {
			return
		}
	}
}

func (c *busConn) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		busReg.remove(c)
		c.subMu.Lock()
		subs := c.subs
		c.subs, c.wanted = nil, nil
		c.subMu.Unlock()
		for _, s := range subs {
			s.topic.unsubscribe(s.sub)
		}
		c.out.close()
		c.pendMu.Lock()
		pending := c.pending
		c.pending, c.gone = nil, true
		c.pendMu.Unlock()
		for _, ch := range pending {
			ch <- busDone{err: &ipcError{ipcFailed, "disconnected"}}
		}
		_ = c.conn.Close()
	})
}

func (c *busConn) send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		b, _ = json.Marshal(busErrorMsg{Op: "error", Error: "reply marshal failed"})
	}
	c.out.pushRaw(b)
}

func (c *busConn) sendError(topic, msg string) {
	c.send(busErrorMsg{Op: "error", Topic: topic, Error: msg})
}

func (c *busConn) reply(id json.RawMessage, result any, err error) {
	r := busReply{Op: "reply", ID: id, OK: err == nil}
	if err != nil {
		r.Error = err.Error()
	} else {
		r.Result = result
	}
	b, merr := json.Marshal(r)
	if merr != nil {
		b, _ = json.Marshal(busReply{Op: "reply", ID: id, Error: "reply marshal failed"})
	}
	c.out.pushRaw(b)
}

func (c *busConn) handle(line []byte) {
	var m busReq
	if err := json.Unmarshal(line, &m); err != nil {
		c.sendError("", "malformed message")
		return
	}
	switch m.Op {
	case "sub":
		for _, name := range m.Topics {
			c.subscribe(name)
		}
	case "unsub":
		for _, name := range m.Topics {
			c.unsubscribe(name)
		}
	case "call":
		c.call(m)
	case "run":
		go c.run(m)
	case "ipc":
		go c.ipc(m)
	case "serve":
		busReg.setServing(c, m.Targets, true)
	case "unserve":
		busReg.setServing(c, m.Targets, false)
	case "done":
		c.finish(m)
	default:
		c.sendError("", "unknown op: "+m.Op)
	}
}

func (c *busConn) subscribe(name string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	if c.subs == nil {
		return
	}
	// look up under subMu, installTopic publishes the topic before it attaches waiting clients
	t := c.d.topic(name)
	if t == nil {
		c.wanted[name] = true
		c.sendError(name, "unknown topic")
		return
	}
	if s, ok := c.subs[name]; ok {
		if s.topic == t {
			t.resync(s.sub)
			return
		}
		s.topic.unsubscribe(s.sub)
		c.out.dropTopic(name)
	}
	c.subs[name] = busTopicSub{topic: t, sub: t.subscribeBus(c, name)}
}

func (c *busConn) attachWanted(name string, t *stateTopic) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	if c.subs == nil || !c.wanted[name] {
		return
	}
	delete(c.wanted, name)
	if _, ok := c.subs[name]; ok {
		return
	}
	c.subs[name] = busTopicSub{topic: t, sub: t.subscribeBus(c, name)}
}

func (c *busConn) unsubscribe(name string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	if c.subs == nil {
		return
	}
	delete(c.wanted, name)
	s, ok := c.subs[name]
	if !ok {
		return
	}
	delete(c.subs, name)
	s.topic.unsubscribe(s.sub)
	c.out.dropTopic(name)
}

func (c *busConn) call(m busReq) {
	ns := m.Method
	if i := strings.IndexByte(ns, '.'); i >= 0 {
		ns = ns[:i]
	}
	method, id, args := m.Method, m.ID, m.Args
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage("{}")
	}
	c.enqueue(ns, func() {
		h := c.d.callHandler(method)
		if h == nil {
			c.reply(id, nil, fmt.Errorf("unknown method: %s", method))
			return
		}
		result, err := h(args)
		c.reply(id, result, err)
	})
}

func (c *busConn) enqueue(ns string, job func()) {
	c.nsMu.Lock()
	defer c.nsMu.Unlock()
	w := c.workers[ns]
	if w != nil {
		w.jobs = append(w.jobs, job)
		return
	}
	w = &busWorker{jobs: []func(){job}}
	c.workers[ns] = w
	go c.drain(ns, w)
}

func (c *busConn) drain(ns string, w *busWorker) {
	for {
		c.nsMu.Lock()
		if len(w.jobs) == 0 {
			delete(c.workers, ns)
			c.nsMu.Unlock()
			return
		}
		job := w.jobs[0]
		w.jobs = w.jobs[1:]
		c.nsMu.Unlock()
		job()
	}
}

func (c *busConn) run(m busReq) {
	line := strings.TrimSpace(m.Line)
	var out string
	switch {
	case strings.HasPrefix(line, "keyring-respond"):
		out = c.d.keyringRespond(line, strings.TrimRight(m.Stdin, "\r\n"))
	case strings.HasPrefix(line, "clip-copy "):
		out = c.d.clipCopy(line)
	case strings.HasPrefix(line, "ipc "):
		out = ipcRequest(strings.TrimPrefix(line, "ipc "))
		if value, ok := strings.CutPrefix(out, "ok\n"); ok {
			c.reply(m.ID, value, nil)
			return
		}
	default:
		out = c.d.dispatch(line)
	}
	result, err := runResult(out)
	c.reply(m.ID, result, err)
}

func runResult(out string) (string, error) {
	switch {
	case out == "ok":
		return "", nil
	case out == "err":
		return "", errors.New("")
	case strings.HasPrefix(out, "err "):
		return "", errors.New(strings.TrimPrefix(out, "err "))
	}
	return out, nil
}

func (c *busConn) ipc(m busReq) {
	args, err := busArgStrings(m.Args)
	if err != nil {
		c.reply(m.ID, nil, err)
		return
	}
	if m.Target == "" || (m.Fn == "" && m.Prop == "") {
		c.reply(m.ID, nil, fmt.Errorf("ipc: expected target and fn or prop"))
		return
	}
	client := m.Client
	if client == "" {
		client = "shell"
	}
	value, err := ipcInvoke(client, m.Target, m.Fn, m.Prop, args)
	if err != nil {
		c.reply(m.ID, nil, err)
		return
	}
	c.reply(m.ID, value, nil)
}

// same positional strings qs ipc call takes, other values as their JSON text
func busArgStrings(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("ipc: args must be an array")
	}
	out := make([]string, len(items))
	for i, item := range items {
		var s string
		if json.Unmarshal(item, &s) == nil {
			out[i] = s
			continue
		}
		out[i] = string(item)
	}
	return out, nil
}

func (c *busConn) invoke(target, fn, prop string, args []string) (value string, sent bool, err error) {
	id := busReg.nextID()
	ch := make(chan busDone, 1)
	c.pendMu.Lock()
	if c.gone {
		c.pendMu.Unlock()
		return "", false, nil
	}
	c.pending[id] = ch
	c.pendMu.Unlock()

	msg := busCmdMsg{Op: "cmd", ID: id, Target: target}
	if prop != "" {
		msg.Prop = prop
	} else {
		if args == nil {
			args = []string{}
		}
		msg.Fn = fn
		msg.Args = &args
	}
	b, _ := json.Marshal(msg)
	if !c.out.pushRaw(b) {
		c.forget(id)
		return "", false, nil
	}
	timer := time.NewTimer(busCmdTimeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.value, true, r.err
	case <-timer.C:
		c.forget(id)
		return "", true, &ipcError{ipcTimedOut, "timeout"}
	}
}

func (c *busConn) forget(id uint64) {
	c.pendMu.Lock()
	delete(c.pending, id)
	c.pendMu.Unlock()
}

func (c *busConn) finish(m busReq) {
	var id uint64
	if json.Unmarshal(m.ID, &id) != nil {
		return
	}
	c.pendMu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.pendMu.Unlock()
	if ch == nil {
		return
	}
	if !m.OK {
		ch <- busDone{err: busCmdError(m.Error)}
		return
	}
	ch <- busDone{value: busResultString(m.Result)}
}

func busResultString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

// classified like classifyQs so a missing target or function stays final for every caller
func busCmdError(msg string) error {
	lower := strings.ToLower(msg)
	for _, p := range qsMissingPhrases {
		if strings.Contains(lower, p) {
			return &ipcError{ipcMissing, msg}
		}
	}
	return &ipcError{ipcFailed, msg}
}

type busRegistry struct {
	mu      sync.Mutex
	conns   []*busConn
	changed chan struct{} // closed and replaced on every serving change
	lastID  uint64
}

var busReg = &busRegistry{changed: make(chan struct{})}

func (r *busRegistry) nextID() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastID++
	return r.lastID
}

func (r *busRegistry) notifyLocked() {
	close(r.changed)
	r.changed = make(chan struct{})
}

func (r *busRegistry) add(c *busConn) {
	r.mu.Lock()
	r.conns = append(r.conns, c)
	r.mu.Unlock()
}

func (r *busRegistry) remove(c *busConn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, x := range r.conns {
		if x == c {
			r.conns = append(r.conns[:i:i], r.conns[i+1:]...)
			r.notifyLocked()
			return
		}
	}
}

func (r *busRegistry) setServing(c *busConn, targets []string, on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range targets {
		if on {
			c.serving[t] = true
		} else {
			delete(c.serving, t)
		}
	}
	r.notifyLocked()
}

func (r *busRegistry) findLocked(client string, pid int, target string) *busConn {
	for i := len(r.conns) - 1; i >= 0; i-- {
		c := r.conns[i]
		if !c.serving[target] {
			continue
		}
		if (pid > 0 && c.pid == pid) || (pid <= 0 && c.client == client) {
			return c
		}
	}
	return nil
}

func (r *busRegistry) find(client string, pid int, target string) *busConn {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.findLocked(client, pid, target)
}

func (r *busRegistry) topicInstalled(d *daemon, name string, t *stateTopic) {
	r.mu.Lock()
	var conns []*busConn
	for _, c := range r.conns {
		if c.d == d {
			conns = append(conns, c)
		}
	}
	r.mu.Unlock()
	for _, c := range conns {
		c.attachWanted(name, t)
	}
}

func busServing(c, target string) bool {
	return busReg.find(c, 0, target) != nil
}

func busAwaitServing(c, target string, timeout time.Duration) bool {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		busReg.mu.Lock()
		found := busReg.findLocked(c, 0, target) != nil
		changed := busReg.changed
		busReg.mu.Unlock()
		if found {
			return true
		}
		select {
		case <-changed:
		case <-deadline.C:
			return false
		}
	}
}

func busInvoke(client string, pid int, target, fn, prop string, args []string) (string, bool, error) {
	c := busReg.find(client, pid, target)
	if c == nil {
		return "", false, nil
	}
	return c.invoke(target, fn, prop, args)
}
