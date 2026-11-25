package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	osexec "os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rpcpipe"
	"ryoku-rashin/internal/sandbox"
)

// This explicit local protocol peer is not oni or a third-party ACP CLI. The
// test executable cannot serve oni's __runtime-mcp command, so it connects to
// the actual per-turn sandbox Unix MCP socket directly. Its prompt never ends
// after a governance request; only the supervised runner can stop execution.
func TestRuntimeACPFinishPeer(t *testing.T) {
	if os.Getenv("RUNTIME_ACP_FINISH_PEER") != "1" {
		return
	}
	folder := os.Getenv("RUNTIME_ACP_FINISH_FOLDER")
	alive, err := net.Dial("unix", filepath.Join(folder, "p.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer alive.Close()
	if _, err := io.WriteString(alive, "ready\n"); err != nil {
		t.Fatal(err)
	}
	ready, err := net.Listen("unix", filepath.Join(folder, "c.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer ready.Close()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := osexec.Command(program, "-test.run=^TestRuntimeACPFinishDescendant$")
	child.Env = append(os.Environ(), "RUNTIME_ACP_FINISH_DESCENDANT=1")
	// Escape the ACP process group: cleanup must also reap the private PID
	// namespace, not merely kill children sharing the bwrap process group.
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = child.Wait() }()
	ack, err := ready.Accept()
	if err != nil {
		t.Fatal(err)
	}
	_ = ack.Close()
	connection := rpcpipe.New(os.Stdin, os.Stdout, rpcpipe.Options{LineLimit: runtimeWireLimit,
		Handle: func(ctx context.Context, method string, _ json.RawMessage) (any, error) {
			switch method {
			case "initialize":
				return map[string]any{"protocolVersion": 1}, nil
			case "session/new":
				return map[string]string{"sessionId": "finish-session"}, nil
			case "session/prompt":
				remote, err := net.Dial("unix", runtimeBridgeSocket)
				if err != nil {
					return nil, err
				}
				defer remote.Close()
				mcp := rpcpipe.New(remote, remote, rpcpipe.Options{LineLimit: runtimeWireLimit})
				defer mcp.Close()
				if err := mcp.Call(ctx, "initialize", map[string]string{"protocolVersion": "2024-11-05"}, nil); err != nil {
					return nil, err
				}
				var response struct {
					IsError bool `json:"isError"`
				}
				request := json.RawMessage(os.Getenv("RUNTIME_ACP_FINISH_REQUEST"))
				err = mcp.Call(ctx, "tools/call", request, &response)
				mode := os.Getenv("RUNTIME_ACP_FINISH_MODE")
				if mode == "accepted" {
					// Even a transport close cannot make this peer finish its
					// ACP prompt voluntarily. The host records real acceptance.
					for {
						time.Sleep(time.Hour)
					}
				}
				if err != nil || response.IsError != (mode != "status") {
					return nil, errors.New("fixture did not observe its expected MCP outcome")
				}
				if _, err := io.WriteString(alive, "continued\n"); err != nil {
					return nil, err
				}
				// The parent controls when a genuine ACP failure occurs, so
				// refusal/nonterminal success must leave the prompt running.
				var command [1]byte
				if _, err := alive.Read(command[:]); err != nil {
					return nil, err
				}
				return nil, &rpcpipe.Error{Code: rpcpipe.CodeInvalidRequest, Message: "fixture prompt failure"}
			default:
				return nil, &rpcpipe.Error{Code: rpcpipe.CodeMethodNotFound, Message: "unsupported fixture ACP method"}
			}
		},
	})
	defer connection.Close()
	for {
		time.Sleep(time.Hour)
	}
}

func TestRuntimeACPFinishDescendant(t *testing.T) {
	if os.Getenv("RUNTIME_ACP_FINISH_DESCENDANT") != "1" {
		return
	}
	folder := os.Getenv("RUNTIME_ACP_FINISH_FOLDER")
	alive, err := net.Dial("unix", filepath.Join(folder, "d.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer alive.Close()
	if _, err := io.WriteString(alive, "ready\n"); err != nil {
		t.Fatal(err)
	}
	ready, err := net.Dial("unix", filepath.Join(folder, "c.sock"))
	if err != nil {
		t.Fatal(err)
	}
	_ = ready.Close()
	for {
		time.Sleep(time.Hour)
	}
}

func TestRuntimeACPFinish(t *testing.T) {
	cases := []struct {
		name, method, tool, args, mode  string
		dream, delegate, reject, cancel bool
	}{
		{name: "report", method: protocol.MTeamsAgentReport, tool: tool.Report, args: `{"title":"Actual result","body":"Only the accepted report."}`, mode: "accepted"},
		{name: "delegate", method: protocol.MTeamsAgentDelegate, tool: tool.Delegate, args: `{"member":"report","title":"A piece","brief":"Perform this piece."}`, mode: "accepted", delegate: true},
		{name: "ask", method: protocol.MTeamsAgentAsk, tool: tool.AskUser, args: `{"title":"Decision","question":"Which path should we take?"}`, mode: "accepted"},
		{name: "plan", method: protocol.MTeamsAgentPlan, tool: tool.ProposePlan, args: `{"summary":"A strategy","steps":["Perform work"]}`, mode: "accepted"},
		{name: "help", method: protocol.MTeamsAgentHelp, tool: tool.RequestHelp, args: `{"member":"peer","title":"Review","brief":"Please review this work."}`, mode: "accepted"},
		{name: "escalate", method: protocol.MTeamsAgentEscalate, tool: tool.Escalate, args: `{"title":"Blocked","detail":"A decision is needed."}`, mode: "accepted"},
		{name: "dream-update", method: protocol.MTeamsAgentDream, tool: tool.DreamStatus, args: `{"phase":"building","summary":"Work is underway."}`, mode: "accepted", dream: true},
		{name: "present", method: protocol.MTeamsAgentPresent, tool: tool.Present, args: `{"title":"Result","body":"Work and evidence."}`, mode: "accepted", dream: true},
		{name: "rejected", tool: tool.Report, args: `{"title":"Rejected","body":"Not accepted."}`, mode: "rejected", reject: true},
		{name: "invalid", tool: tool.RequestHelp, args: `{"member":"","title":"Invalid","brief":"Missing peer."}`, mode: "invalid"},
		{name: "status", method: protocol.MTeamsAgentStatus, tool: tool.TeamStatus, args: `{}`, mode: "status"},
		{name: "cancel", tool: tool.Report, args: `{"title":"Rejected","body":"Not accepted."}`, mode: "rejected", reject: true, cancel: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := sandbox.Available(); err != nil {
				t.Skipf("real bubblewrap namespace support unavailable: %v", err)
			}
			program, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
			opts.Tools.Team.Lead = tc.dream || tc.delegate
			opts.Tools.Team.Dream = tc.dream
			opts.Tools.Team.Delegate = tc.delegate
			opts.Tools.Team.Runtime.Command = []string{program, "-test.run=^TestRuntimeACPFinishPeer$"}
			request, err := json.Marshal(map[string]any{"name": tc.tool, "arguments": json.RawMessage(tc.args)})
			if err != nil {
				t.Fatal(err)
			}
			opts.Tools.Team.Runtime.Env = map[string]string{
				"RUNTIME_ACP_FINISH_PEER": "1", "RUNTIME_ACP_FINISH_FOLDER": opts.Cwd,
				"RUNTIME_ACP_FINISH_REQUEST": string(request), "RUNTIME_ACP_FINISH_MODE": tc.mode,
			}
			opts.Sandbox.ReadOnly = append(opts.Sandbox.ReadOnly, filepath.Dir(program))
			peerListener := runtimeACPFinishListener(t, filepath.Join(opts.Cwd, "p.sock"))
			descendantListener := runtimeACPFinishListener(t, filepath.Join(opts.Cwd, "d.sock"))
			host := &runtimeTestHost{fail: tc.reject}
			factory := runtimeTestFactory(host, exec.Host(exec.HostOptions{Env: []string{}}))
			defer factory.Close()
			runner := runtimeTestNew(t, factory, opts)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			type outcome struct {
				result core.TurnResult
				err    error
			}
			done := make(chan outcome, 1)
			go func() {
				result, err := runner.Prompt(ctx, core.PromptInput{Turn: 1, Text: "Exercise the local ACP governance fixture."}, &runtimeTestSink{})
				done <- outcome{result: result, err: err}
			}()
			peer, peerReader := runtimeACPFinishAccept(t, peerListener)
			descendant, descendantReader := runtimeACPFinishAccept(t, descendantListener)
			if tc.mode != "accepted" {
				line, err := peerReader.ReadString('\n')
				if err != nil || line != "continued\n" {
					t.Fatalf("refused/nonterminal MCP call terminated the peer: %q, %v", line, err)
				}
				select {
				case got := <-done:
					t.Fatalf("prompt ended before the controlled failure: %+v", got)
				default:
				}
				if tc.cancel {
					cancel()
				} else if _, err := peer.Write([]byte{'!'}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case got := <-done:
				if tc.mode == "accepted" {
					if got.err != nil || got.result.Stop != protocol.StopEnd {
						t.Fatalf("accepted governance was not an honest successful turn: %+v", got)
					}
				} else if got.err == nil || tc.cancel && got.result.Stop != protocol.StopCancelled || !tc.cancel && got.result.Stop != protocol.StopError {
					t.Fatalf("genuine failure/cancellation became completion: %+v", got)
				}
				if runner.Alive() || got.result.Usage == nil || !got.result.Usage.Unmetered {
					t.Fatalf("runtime lifecycle/usage is incorrect: %+v", got)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("governance/failure did not return and reap the hanging ACP prompt")
			}
			for name, reader := range map[string]*bufio.Reader{"peer": peerReader, "detached descendant": descendantReader} {
				if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
					t.Fatalf("%s retained its live socket after Prompt returned: %v", name, err)
				}
			}
			_ = descendant.Close()
			host.mu.Lock()
			defer host.mu.Unlock()
			want := 1
			if tc.reject || tc.mode == "invalid" {
				want = 0
			}
			if len(host.methods) != want || want == 1 && host.methods[0] != tc.method {
				t.Fatalf("unexpected or fabricated governance completion: %v", host.methods)
			}
		})
	}
}

func runtimeACPFinishListener(t *testing.T, path string) *net.UnixListener {
	t.Helper()
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return listener
}

func runtimeACPFinishAccept(t *testing.T, listener *net.UnixListener) (net.Conn, *bufio.Reader) {
	t.Helper()
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(connection)
	if line, err := reader.ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("sandboxed fixture did not become ready: %q, %v", line, err)
	}
	return connection, reader
}
