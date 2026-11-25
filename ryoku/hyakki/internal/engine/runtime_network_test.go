package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

func TestRuntimeCLIReadOnlyCannotWriteOverNetwork(t *testing.T) {
	if err := sandbox.Available(); err != nil {
		t.Skipf("real bubblewrap namespace support unavailable: %v", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected process request method: %s", r.Method)
		}
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	for _, readonly := range []bool{false, true} {
		name := "writable-positive-control"
		if readonly {
			name = "read-only-is-offline"
		}
		t.Run(name, func(t *testing.T) {
			opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
			if readonly {
				opts.Approvals = func() string { return protocol.ApprovalsReadOnly }
			}
			opts.Tools.Team.Runtime.Command = []string{program, "-test.run=^TestRuntimeACPSubprocessPeer$"}
			opts.Tools.Team.Runtime.Env = map[string]string{"RUNTIME_ACP_TEST_PEER": "1", "RUNTIME_ACP_TEST_ENDPOINT": server.URL}
			opts.Sandbox.ReadOnly = append(opts.Sandbox.ReadOnly, filepath.Dir(program))
			host := &runtimeTestHost{}
			factory := runtimeTestFactory(host, exec.Host(exec.HostOptions{Env: []string{}}))
			defer factory.Close()
			runner := runtimeTestNew(t, factory, opts)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := runner.Prompt(ctx, core.PromptInput{Turn: 1, Text: "Exercise the configured process network boundary"}, &runtimeTestSink{}); err != nil {
				t.Fatal(err)
			}
			report := host.last(t)
			if readonly {
				if !strings.Contains(report.Body, "network-rejected") || strings.Contains(report.Body, "network-succeeded") {
					t.Fatalf("read-only process did not observe an enforced refusal: %+v", report)
				}
			} else if !strings.Contains(report.Body, "network-succeeded") {
				t.Fatalf("positive control could not reach the real receiver: %+v", report)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("receiver observed %d mutations; exactly the writable control may reach it", got)
			}
		})
	}
}
