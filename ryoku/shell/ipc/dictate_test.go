package main

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// The voice verb and the card are driven entirely by ryoku-dictate's socket, so
// these tests run a fake daemon and assert what the desktop would see: the
// frames on the `dictation` topic, and which command a tap sends.

// fakeDictateServer mirrors the real daemon's split: a command's reply goes to
// the connection that asked, every state event goes to all of them, and a new
// connection is greeted with the current state so a late watcher is never blind.
func fakeDictateServer(t *testing.T) (commands chan string, broadcast func(string)) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	socket := filepath.Join(dir, "ryoku-dictate.sock")

	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	var mu sync.Mutex
	var peers []net.Conn
	state := "idle"
	commands = make(chan string, 16)
	broadcast = func(line string) {
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range peers {
			_, _ = conn.Write([]byte(line + "\n"))
		}
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			peers = append(peers, conn)
			greeting := "state " + state + "\n"
			mu.Unlock()

			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = conn.Write([]byte(greeting))
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					line := scanner.Text()
					commands <- line
					switch line {
					case "status":
						mu.Lock()
						current := state
						mu.Unlock()
						_, _ = conn.Write([]byte("state " + current + "\nenabled true\nend\n"))
					case "start":
						mu.Lock()
						state = "recording"
						mu.Unlock()
						broadcast("state recording")
						_, _ = conn.Write([]byte("ok\n"))
					case "stop":
						mu.Lock()
						state = "idle"
						mu.Unlock()
						broadcast("state transcribing")
						broadcast("final And so my fellow Americans\n")
						broadcast("state idle")
						_, _ = conn.Write([]byte("ok\n"))
					default:
						_, _ = conn.Write([]byte("ok\n"))
					}
				}
			}(conn)
		}
	}()
	return commands, broadcast
}

func readFrame(t *testing.T, d *daemon) dictationFrame {
	t.Helper()
	d.topicsMu.Lock()
	topic := d.topics["dictation"]
	d.topicsMu.Unlock()
	topic.mu.Lock()
	defer topic.mu.Unlock()
	var frame dictationFrame
	if len(topic.last) > 0 {
		if err := json.Unmarshal(topic.last, &frame); err != nil {
			t.Fatalf("frame %q: %v", topic.last, err)
		}
	}
	return frame
}

func TestDictationFramesReachTheTopic(t *testing.T) {
	// sup already holds "shell" so the surface open never spawns Quickshell.
	d := &daemon{sup: map[string]bool{"shell": true}, activeMon: "DP-1", quit: make(chan struct{})}
	t.Cleanup(func() { close(d.quit) })
	d.dictTopic = d.registerTopic("dictation")
	d.dictFrame = dictationFrame{State: "idle", Enabled: true, Device: "auto", Mode: "paste"}

	client, server := net.Pipe()
	go d.streamDictation(client)
	go func() {
		for _, line := range []string{
			"state loading",
			"backend CPU",
			"state recording",
			"level 0.250000 0.600000",
			"state transcribing",
			"final And so my fellow\\nAmericans",
			"took 184",
			"state idle",
		} {
			_, _ = server.Write([]byte(line + "\n"))
			time.Sleep(5 * time.Millisecond)
		}
	}()

	deadline := time.Now().Add(2 * time.Second)
	var frame dictationFrame
	for time.Now().Before(deadline) {
		frame = readFrame(t, d)
		if frame.State == "idle" && frame.Text != "" && frame.TookMs == 184 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if frame.State != "idle" {
		t.Fatalf("state = %q, want idle after the stream finished", frame.State)
	}
	if frame.Text != "And so my fellow\nAmericans" {
		t.Fatalf("text = %q, want the escaped newline decoded", frame.Text)
	}
	if frame.TookMs != 184 {
		t.Fatalf("tookMs = %d, want 184", frame.TookMs)
	}
	if frame.Backend != "CPU" {
		t.Fatalf("backend = %q, want CPU", frame.Backend)
	}
	// The mode and device the page shows come from the earlier status frame and
	// are never cleared by an event that does not carry them.
	if frame.Mode != "paste" || frame.Device != "auto" {
		t.Fatalf("frame lost its settings: %+v", frame)
	}
	_ = server.Close()
}

func TestVoiceTapTogglesThroughTheDaemon(t *testing.T) {
	commands, _ := fakeDictateServer(t)

	d := &daemon{sup: map[string]bool{"shell": true}, activeMon: "DP-1", quit: make(chan struct{})}
	// Stop the watcher so no goroutine outlives the test.
	t.Cleanup(func() { close(d.quit) })
	d.startDictation()

	if got := d.dispatch("voice"); got != "ok" {
		t.Fatalf("dispatch(voice) = %q", got)
	}
	select {
	case got := <-commands:
		if got != "start" {
			t.Fatalf("first tap sent %q, want start", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first tap reached no daemon command")
	}

	// The card opens on the daemon's own recording event, not on the tap.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if readFrame(t, d).State == "recording" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if state := readFrame(t, d).State; state != "recording" {
		t.Fatalf("topic state = %q, want recording", state)
	}
	if got := d.dispatch("voice"); got != "ok" {
		t.Fatalf("second dispatch(voice) = %q", got)
	}
	stopped := false
	for !stopped {
		select {
		case got := <-commands:
			if got == "stop" {
				stopped = true
			}
		case <-time.After(3 * time.Second):
			t.Fatal("a tap while recording never stopped the daemon")
		}
	}

	// Returning to idle on the topic is what unmaps the card.
	for time.Now().Before(deadline) {
		if readFrame(t, d).State == "idle" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the topic never returned to idle: %q", readFrame(t, d).State)
}

// A refused tap must say so on the topic: the card is the shell's own popup and
// reads the refusal there, so no bar surface is involved.
func TestVoiceFallsToTheOffNoteWithoutTheEngine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir) // no socket: the daemon is not running
	t.Setenv("PATH", dir)            // and ryoku-host cannot start it

	calls := make(chan string, 4)
	stubShellIpc(t, calls)

	d := &daemon{sup: map[string]bool{"shell": true}, activeMon: "DP-1", quit: make(chan struct{})}
	t.Cleanup(func() { close(d.quit) })
	d.dictTopic = d.registerTopic("dictation")
	d.dictConn = &dictateConn{}
	d.dictFrame = dictationFrame{State: "idle", Enabled: false}

	if got := d.dispatch("voice"); got != "ok" {
		t.Fatalf("dispatch(voice) = %q, want ok", got)
	}
	frame := d.currentDictation()
	if frame.Notice != "off" || frame.AttentionMs == 0 {
		t.Fatalf("a refused tap did not mark the topic: %+v", frame)
	}
	select {
	case got := <-calls:
		t.Fatalf("a refused tap emitted shell IPC %q", got)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestDictateSendRetriesOnceOnADeadConnection(t *testing.T) {
	commands := make(chan string, 8)
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	socket := filepath.Join(dir, "ryoku-dictate.sock")

	var mu sync.Mutex
	var peers []net.Conn
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			peers = append(peers, conn)
			mu.Unlock()
			go func(conn net.Conn) {
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					commands <- scanner.Text()
					_, _ = conn.Write([]byte("ok\n"))
				}
			}(conn)
		}
	}()

	c := &dictateConn{}
	if _, err := c.send("status"); err != nil {
		t.Fatalf("first send: %v", err)
	}
	// Kill the daemon's side of the connection, as a package update would.
	mu.Lock()
	for _, peer := range peers {
		_ = peer.Close()
	}
	peers = nil
	mu.Unlock()

	start := time.Now()
	if _, err := c.send("start"); err != nil {
		t.Fatalf("send after the daemon restarted: %v", err)
	}
	if spent := time.Since(start); spent > 2*time.Second {
		t.Errorf("the retry took %v; a dead connection must be replaced, not waited out", spent)
	}
	deadline := time.After(2 * time.Second)
	seen := []string{}
	for len(seen) < 2 {
		select {
		case got := <-commands:
			seen = append(seen, got)
		case <-deadline:
			t.Fatalf("commands after the restart: %v", seen)
		}
	}
	if seen[len(seen)-1] != "start" {
		t.Fatalf("last command = %q, want start (seen %v)", seen[len(seen)-1], seen)
	}
	c.close()
}

func TestUnescapeDictationText(t *testing.T) {
	cases := map[string]string{
		`plain`:                      "plain",
		`two\nlines`:                 "two\nlines",
		`tab\there`:                  `tab\there`,
		`back\\slash`:                `back\slash`,
		`And so my fellow\Americans`: `And so my fellow\Americans`,
		`\n`:                         "\n",
		"":                           "",
		`café \u2014 not escaped`:    "café \\u2014 not escaped",
	}
	for in, want := range cases {
		if got := unescapeDictation(in); got != want {
			t.Errorf("unescapeDictation(%q) = %q, want %q", in, got, want)
		}
	}
}
