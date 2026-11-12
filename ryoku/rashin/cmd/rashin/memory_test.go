package main

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// memoryDaemon answers hello and memory.status on a socket of its own, and
// records the methods it was asked.
func memoryDaemon(t *testing.T, status protocol.MemoryStatus) <-chan string {
	t.Helper()
	dir, err := os.MkdirTemp("", "rashin-cli")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "rashin.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	t.Setenv("RASHIN_SOCKET", socket)
	asked := make(chan string, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				frames := protocol.NewDecoder(conn)
				for {
					frame, err := frames.Next()
					if err != nil || frame.ID == nil {
						return
					}
					asked <- frame.Method
					var result any = protocol.HelloResult{Daemon: "rashind", Protocol: protocol.Version, Enabled: true}
					if frame.Method == protocol.MMemoryStatus {
						result = status
					}
					raw, _ := json.Marshal(result)
					line, _ := json.Marshal(protocol.Response{ID: *frame.ID, Result: raw})
					if _, err := conn.Write(append(line, '\n')); err != nil {
						return
					}
				}
			}()
		}
	}()
	return asked
}

// captured runs run with stdout taken and returns what it printed.
func captured(t *testing.T, run func() int) (string, int) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = write
	code := run()
	os.Stdout = stdout
	_ = write.Close()
	out, _ := io.ReadAll(read)
	return string(out), code
}

func TestMemoryStatusAsksTheDaemon(t *testing.T) {
	status := protocol.MemoryStatus{
		Engine: protocol.MemoryBuiltin, EngineState: protocol.EngineReady,
		Counts:  protocol.MemoryCounts{All: 4, You: 3, Projects: 1, Suggested: 2},
		Learner: protocol.MemoryLearner{State: protocol.LearnerReady, Model: "local/qwen"},
		Card:    protocol.MemoryCard{Text: "The user writes Go."},
		Path:    "/vault/memory/rashin",
	}
	asked := memoryDaemon(t, status)
	out, code := captured(t, func() int { return runMemory([]string{"status", "--format", "json"}) })
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	var got protocol.MemoryStatus
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Counts.All != 4 || got.Learner.Model != "local/qwen" {
		t.Fatalf("printed %q: %v", out, err)
	}
	if first, second := <-asked, <-asked; first != protocol.MHello || second != protocol.MMemoryStatus {
		t.Fatalf("the daemon was asked %s then %s", first, second)
	}
	human, code := captured(t, func() int { return runMemory([]string{"status", "--format", "human"}) })
	if code != 0 || !strings.Contains(human, "4 in use (3 about you, 1 in projects, 0 in teams)") || !strings.Contains(human, "/vault/memory/rashin") {
		t.Fatalf("human status = %q (exit %d)", human, code)
	}
}

func TestMemoryRefusesWhatItDoesNotKnow(t *testing.T) {
	for _, args := range [][]string{nil, {"status", "extra"}, {"search"}, {"forget"}, {"learn"}, {"list", "--limit", "zero"}, {"add", "fact", "--colour", "red"}, {"wander"}} {
		if _, code := captured(t, func() int { return runMemory(args) }); code == 0 {
			t.Fatalf("rashin memory %v succeeded", args)
		}
	}
}
