package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

func TestRuntimeACPExportedSecretFormsNeverReachReportOrTranscript(t *testing.T) {
	if err := sandbox.Available(); err != nil {
		t.Skipf("real bubblewrap namespace support unavailable: %v", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const secret = "local-fixture/a+b=<encoded>&\"value"
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Tools.Team.Runtime.Command = []string{program, "-test.run=^TestRuntimeACPSubprocessPeer$"}
	opts.Tools.Team.Runtime.Model = "model-one"
	opts.Tools.Team.Runtime.Env = map[string]string{"RUNTIME_ACP_TEST_PEER": "1", "RUNTIME_ACP_TEST_ECHO_SECRET": "1"}
	opts.Tools.Team.Runtime.SecretEnv = map[string]string{"API_KEY": "explicit-fixture-grant"}
	opts.RuntimeEnvironment = func(context.Context) (map[string]string, error) {
		return map[string]string{"API_KEY": secret}, nil
	}
	opts.Sandbox.ReadOnly = append(opts.Sandbox.ReadOnly, filepath.Dir(program))
	host := &runtimeTestHost{}
	factory := runtimeTestFactory(host, exec.Host(exec.HostOptions{Env: []string{}}))
	defer factory.Close()
	runner := runtimeTestNew(t, factory, opts)
	sink := &runtimeTestSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := runner.Prompt(ctx, core.PromptInput{Turn: 1, Text: "Return the local protocol result"}, sink); err != nil {
		t.Fatal(err)
	}
	report := host.last(t)
	sink.mu.Lock()
	transcript := sink.text
	sink.mu.Unlock()
	quoted, err := json.Marshal(secret)
	if err != nil {
		t.Fatal(err)
	}
	forms := []string{
		secret, base64.StdEncoding.EncodeToString([]byte(secret)),
		base64.RawURLEncoding.EncodeToString([]byte(secret)),
		url.QueryEscape(secret), string(quoted[1 : len(quoted)-1]),
	}
	for _, output := range []string{report.Body, transcript} {
		if !strings.Contains(output, "visible-result") || !strings.Contains(output, "result-tail") {
			t.Fatal("redaction discarded unrelated result text")
		}
		for _, form := range forms {
			if strings.Contains(output, form) {
				t.Fatal("an exported credential form reached an authenticated report or visible transcript")
			}
		}
	}
}
