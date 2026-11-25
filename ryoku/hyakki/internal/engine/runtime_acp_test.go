package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rpcpipe"
	"ryoku-rashin/internal/sandbox"
)

// This is a local protocol peer, not an installed CLI or a model request. It
// runs only as this test binary's explicitly configured sandboxed child.
func TestRuntimeACPSubprocessPeer(t *testing.T) {
	if os.Getenv("RUNTIME_ACP_TEST_PEER") != "1" {
		return
	}
	var connection *rpcpipe.Conn
	opened := ""
	selected := ""
	folder := ""
	var serial sync.Mutex
	ready := make(chan struct{})
	connection = rpcpipe.New(os.Stdin, os.Stdout, rpcpipe.Options{LineLimit: runtimeWireLimit,
		Handle: func(_ context.Context, method string, raw json.RawMessage) (any, error) {
			<-ready
			serial.Lock()
			defer serial.Unlock()
			switch method {
			case "initialize":
				return map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]bool{"loadSession": true}}, nil
			case "session/new", "session/load":
				var params struct {
					Session string `json:"sessionId"`
					Cwd     string `json:"cwd"`
				}
				if err := json.Unmarshal(raw, &params); err != nil {
					return nil, err
				}
				if method == "session/load" && params.Session != "sandbox-session" {
					return nil, errors.New("unknown saved session")
				}
				opened = method
				folder = params.Cwd
				response := map[string]any{"models": map[string]any{"availableModels": []any{map[string]string{"modelId": "model-one"}}}}
				if method == "session/new" {
					response["sessionId"] = "sandbox-session"
				} else if os.Getenv("RUNTIME_ACP_TEST_WRONG_LOAD") == "1" {
					response["sessionId"] = "foreign-session"
				}
				return response, nil
			case "session/set_model":
				var params struct {
					Model string `json:"modelId"`
				}
				if err := json.Unmarshal(raw, &params); err != nil {
					return nil, err
				}
				if params.Model != "model-one" {
					return nil, errors.New("model is unavailable")
				}
				selected = params.Model
				return map[string]any{}, nil
			case "session/prompt":
				text := "Observed " + opened + " with " + selected
				if os.Getenv("RUNTIME_ACP_TEST_WRITE") == "1" {
					if err := os.WriteFile(filepath.Join(folder, "forbidden-write"), []byte("external process tried writing"), 0600); err != nil {
						text += " write-rejected"
					} else {
						text += " write-succeeded"
					}
				}
				if endpoint := os.Getenv("RUNTIME_ACP_TEST_ENDPOINT"); endpoint != "" {
					client := &http.Client{Timeout: 300 * time.Millisecond}
					response, err := client.Post(endpoint, "text/plain", strings.NewReader("external mutation"))
					if err != nil {
						text += " network-rejected"
					} else {
						response.Body.Close()
						text += " network-succeeded"
					}
				}
				if os.Getenv("RUNTIME_ACP_TEST_ECHO_SECRET") == "1" {
					value := os.Getenv("API_KEY")
					quoted, err := json.Marshal(value)
					if err != nil {
						return nil, err
					}
					text += " visible-result " + strings.Join([]string{
						value, base64.StdEncoding.EncodeToString([]byte(value)),
						base64.RawURLEncoding.EncodeToString([]byte(value)),
						url.QueryEscape(value), string(quoted),
					}, " | ") + " result-tail"
				}
				if err := connection.Notify("session/update", map[string]any{"sessionId": "sandbox-session", "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": text}}}); err != nil {
					return nil, err
				}
				return map[string]string{"stopReason": protocol.StopEnd}, nil
			default:
				return nil, &rpcpipe.Error{Code: rpcpipe.CodeMethodNotFound, Message: "unsupported test peer method"}
			}
		},
	})
	close(ready)
	<-connection.Done()
	os.Exit(0)
}

func TestRuntimeCLIStartsSelectsModelResumesAndStopsRealSandbox(t *testing.T) {
	if err := sandbox.Available(); err != nil {
		t.Skipf("real bubblewrap namespace support unavailable: %v", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Tools.Team.Runtime.Command = []string{program, "-test.run=^TestRuntimeACPSubprocessPeer$"}
	opts.Tools.Team.Runtime.Model = "model-one"
	opts.Tools.Team.Runtime.Env = map[string]string{"RUNTIME_ACP_TEST_PEER": "1"}
	// Go's test binary commonly lives under /tmp, private in the sandbox.
	// Expose only that executable's directory read-only, not host /tmp.
	opts.Sandbox.ReadOnly = append(opts.Sandbox.ReadOnly, filepath.Dir(program))
	host := &runtimeTestHost{}
	f := runtimeTestFactory(host, exec.Host(exec.HostOptions{Env: []string{}}))
	defer f.Close()
	r := runtimeTestNew(t, f, opts)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := r.Prompt(ctx, core.PromptInput{Turn: 1, Text: "Perform local protocol work"}, &runtimeTestSink{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Usage == nil || !result.Usage.Unmetered || r.Alive() {
		t.Fatalf("bad CLI lifecycle: %+v", result)
	}
	if report := host.last(t); !strings.Contains(report.Body, "Observed session/new with model-one") {
		t.Fatalf("new session/model not observed: %+v", report)
	}
	opts.Resume = r.State()
	resumed := runtimeTestNew(t, f, opts)
	if _, err := resumed.Prompt(ctx, core.PromptInput{Turn: 2, Text: "Continue local protocol work"}, &runtimeTestSink{}); err != nil {
		t.Fatal(err)
	}
	if report := host.last(t); !strings.Contains(report.Body, "Observed session/load with model-one") {
		t.Fatalf("saved session did not resume: %+v", report)
	}
	if err := resumed.Stop(); err != nil || resumed.Alive() {
		t.Fatalf("CLI left a worker after Stop: %v", err)
	}
}

func TestRuntimeCLIReadOnlyPreventsDirectForeignProcessWrites(t *testing.T) {
	if err := sandbox.Available(); err != nil {
		t.Skipf("real bubblewrap namespace support unavailable: %v", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Approvals = func() string { return protocol.ApprovalsReadOnly }
	opts.Tools.Team.Runtime.Command = []string{program, "-test.run=^TestRuntimeACPSubprocessPeer$"}
	opts.Tools.Team.Runtime.Env = map[string]string{"RUNTIME_ACP_TEST_PEER": "1", "RUNTIME_ACP_TEST_WRITE": "1"}
	opts.Sandbox.ReadOnly = append(opts.Sandbox.ReadOnly, filepath.Dir(program))
	host := &runtimeTestHost{}
	f := runtimeTestFactory(host, exec.Host(exec.HostOptions{Env: []string{}}))
	defer f.Close()
	r := runtimeTestNew(t, f, opts)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := r.Prompt(ctx, core.PromptInput{Turn: 1, Text: "Read-only task"}, &runtimeTestSink{}); err != nil {
		t.Fatal(err)
	}
	if report := host.last(t); !strings.Contains(report.Body, "write-rejected") || strings.Contains(report.Body, "write-succeeded") {
		t.Fatalf("foreign process was not actually constrained: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(opts.Cwd, "forbidden-write")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("external process wrote through a read-only team runtime")
	}
}

func TestRuntimeACPModelCapabilitiesAreNegotiatedNotAssumed(t *testing.T) {
	for _, kind := range []string{"legacy", "config", "unsupported"} {
		t.Run(kind, func(t *testing.T) {
			left, right := net.Pipe()
			defer left.Close()
			defer right.Close()
			methodSeen := make(chan string, 1)
			peer := rpcpipe.New(right, right, rpcpipe.Options{Handle: func(_ context.Context, method string, _ json.RawMessage) (any, error) {
				methodSeen <- method
				return map[string]any{}, nil
			}})
			defer peer.Close()
			connection := rpcpipe.New(left, left, rpcpipe.Options{})
			defer connection.Close()
			a := &runtimeACP{conn: connection}
			var opened runtimeACPSession
			wire := `{"sessionId":"session","models":{"availableModels":[{"modelId":"selected"}]}}`
			if kind == "config" {
				wire = `{"sessionId":"session","configOptions":[{"id":"engine-model","category":"model","type":"select","options":[{"value":"selected"}]}]}`
			}
			if kind == "unsupported" {
				wire = `{"sessionId":"session"}`
			}
			if err := json.Unmarshal([]byte(wire), &opened); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := a.selectModel(ctx, opened, "selected")
			if kind == "unsupported" {
				if err == nil {
					t.Fatal("unsupported model selection silently accepted")
				}
				select {
				case <-methodSeen:
					t.Fatal("unadvertised model method was invoked")
				default:
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			method := <-methodSeen
			if kind == "legacy" && method != "session/set_model" || kind == "config" && method != "session/set_config_option" {
				t.Fatalf("wrong negotiated model method: %s", method)
			}
		})
	}
}

func TestRuntimeACPSetupCancellationClosesBlockedTransport(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	connection := rpcpipe.New(left, left, rpcpipe.Options{})
	a := &runtimeACP{conn: connection, abort: func() { _ = left.Close() }}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.call(ctx, "initialize", map[string]int{"protocolVersion": 1}, nil) }()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled setup succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ACP setup stuck on peer that never reads")
	}
	_ = connection.Close()
}
