package engine

import (
	"context"
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestRuntimeMCPAcceptedHandoffRevokesFurtherTaskTools(t *testing.T) {
	opts := runtimeTestOptions(t, protocol.MemberRuntimeCLI)
	opts.Tools.Team.Runtime.Command = []string{"configured-agent"}
	host := &runtimeTestHost{}
	factory := runtimeTestFactory(host, nil)
	defer factory.Close()
	runner := runtimeTestNew(t, factory, opts)
	bridge, err := runner.newBridge(context.Background(), runner.processSpec(), &runtimeTestSink{})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.close()
	bridge.active = true
	request := json.RawMessage(`{"name":"request_help","arguments":{"member":"peer","title":"Review required","brief":"Please route this request through the manager."}}`)
	accepted, err := bridge.handle(context.Background(), "tools/call", request)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(accepted)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &response); err != nil || response.IsError {
		t.Fatalf("governed handoff was rejected: %s", raw)
	}
	if host.count() != 1 || !bridge.yielded {
		t.Fatal("accepted handoff did not move this prompt out of execution")
	}
	for _, request := range []json.RawMessage{
		request,
		json.RawMessage(`{"name":"report","arguments":{"title":"False completion","body":"The handoff is still pending."}}`),
	} {
		if _, err := bridge.handle(context.Background(), "tools/call", request); err == nil {
			t.Fatal("a yielded prompt retained authority to mutate or finish its task")
		}
	}
	if host.count() != 1 || bridge.reported {
		t.Fatal("a post-handoff action reached the authenticated controller")
	}
}
