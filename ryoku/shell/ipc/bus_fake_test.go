package main

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"
)

// busCmd is one cmd a fake bus client received.
type busCmd struct {
	Target string
	Fn     string
	Prop   string
	Args   []string
}

// fakeBus is a real bus connection over net.Pipe that serves IpcHandler
// targets the way a Quickshell process would, so routing tests exercise the
// daemon's registry, cmd framing and done handling.
type fakeBus struct {
	conn net.Conn
	// handle answers each cmd; nil acks with an empty result. It runs on its
	// own goroutine, so it may block to simulate a hung client.
	handle func(busCmd) (string, error)

	mu  sync.Mutex
	got []busCmd
}

// fakeBusClient connects a bus client named client with pid (0 for none)
// serving targets, and waits until the daemon routes to it.
func fakeBusClient(t *testing.T, client string, pid int, targets ...string) *fakeBus {
	t.Helper()
	c, s := net.Pipe()
	go (&daemon{}).handle(s)
	fb := &fakeBus{conn: c}
	hs, _ := json.Marshal(map[string]any{"v": 1, "client": client, "pid": pid})
	serve, _ := json.Marshal(map[string]any{"op": "serve", "targets": targets})
	go func() {
		_, _ = c.Write([]byte("bus " + string(hs) + "\n" + string(serve) + "\n"))
	}()
	go fb.read()
	t.Cleanup(func() {
		_ = c.Close()
		busWaitFor(t, "fake bus client to unregister", func() bool { return !fb.registered(client, pid, targets) })
	})
	busWaitFor(t, "fake bus client to serve", func() bool { return fb.registered(client, pid, targets) })
	return fb
}

func (fb *fakeBus) registered(client string, pid int, targets []string) bool {
	busReg.mu.Lock()
	defer busReg.mu.Unlock()
	for _, c := range busReg.conns {
		if c.client != client || c.pid != pid {
			continue
		}
		all := true
		for _, t := range targets {
			all = all && c.serving[t]
		}
		if all {
			return true
		}
	}
	return false
}

func (fb *fakeBus) read() {
	r := bufio.NewReader(fb.conn)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return
		}
		var m struct {
			Op     string    `json:"op"`
			ID     uint64    `json:"id"`
			Target string    `json:"target"`
			Fn     string    `json:"fn"`
			Prop   string    `json:"prop"`
			Args   *[]string `json:"args"`
		}
		if json.Unmarshal(line, &m) != nil || m.Op != "cmd" {
			continue
		}
		cmd := busCmd{Target: m.Target, Fn: m.Fn, Prop: m.Prop}
		if m.Args != nil {
			cmd.Args = *m.Args
		}
		fb.mu.Lock()
		fb.got = append(fb.got, cmd)
		fb.mu.Unlock()
		go func(id uint64) {
			done := map[string]any{"op": "done", "id": id, "ok": true, "result": ""}
			if fb.handle != nil {
				value, err := fb.handle(cmd)
				if err != nil {
					done = map[string]any{"op": "done", "id": id, "ok": false, "error": err.Error()}
				} else {
					done["result"] = value
				}
			}
			b, _ := json.Marshal(done)
			_, _ = fb.conn.Write(append(b, '\n'))
		}(m.ID)
	}
}

// cmds returns every cmd received so far.
func (fb *fakeBus) cmds() []busCmd {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return append([]busCmd(nil), fb.got...)
}

func busWaitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
