package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

func TestRuntimeCLIRejectsContradictoryLoadedSessionIdentity(t *testing.T) {
	if err := sandbox.Available(); err != nil {
		t.Skipf("real bubblewrap namespace support unavailable: %v", err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Tools.Team.Runtime.Command = []string{program, "-test.run=^TestRuntimeACPSubprocessPeer$"}
	opts.Tools.Team.Runtime.Env = map[string]string{"RUNTIME_ACP_TEST_PEER": "1", "RUNTIME_ACP_TEST_WRONG_LOAD": "1"}
	opts.Sandbox.ReadOnly = append(opts.Sandbox.ReadOnly, filepath.Dir(program))
	host := &runtimeTestHost{}
	factory := runtimeTestFactory(host, exec.Host(exec.HostOptions{Env: []string{}}))
	defer factory.Close()
	runner := runtimeTestNew(t, factory, opts)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := runner.Prompt(ctx, core.PromptInput{Turn: 1, Text: "Complete the first local turn"}, &runtimeTestSink{}); err != nil {
		t.Fatal(err)
	}
	opts.Resume = runner.State()
	resumed := runtimeTestNew(t, factory, opts)
	result, err := resumed.Prompt(ctx, core.PromptInput{Turn: 2, Text: "Resume only the saved session"}, &runtimeTestSink{})
	if err == nil || result.Stop != protocol.StopError || resumed.Alive() {
		t.Fatalf("a contradictory loaded identity was treated as the saved task: %+v %v", result, err)
	}
	if host.count() != 1 {
		t.Fatal("the rejected resume created another completion report")
	}
}
