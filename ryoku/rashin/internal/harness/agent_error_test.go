package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// answering runs a connection whose agent answers its next request with
// reply, and returns the error that request failed with once the runner has
// added what its log says.
func answering(t *testing.T, log string, reply rpcError) error {
	t.Helper()
	requestIn, requestOut := io.Pipe()
	responseIn, responseOut := io.Pipe()
	t.Cleanup(func() { _ = responseOut.Close(); _ = requestIn.Close() })
	conn := newConnection(requestOut)
	conn.start(responseIn)
	go func() {
		line, err := bufio.NewReader(requestIn).ReadBytes('\n')
		if err != nil {
			return
		}
		var request rpcMessage
		_ = json.Unmarshal(line, &request)
		encoded, _ := json.Marshal(rpcMessage{JSONRPC: "2.0", ID: request.ID, Error: &reply})
		_, _ = responseOut.Write(append(encoded, '\n'))
	}()
	r := &runner{stderr: newTailBuffer(stderrTailLimit)}
	_, _ = r.stderr.Write([]byte(log))
	_, err := conn.request(context.Background(), "session/new", map[string]any{})
	if err == nil {
		t.Fatal("an error response produced no error")
	}
	return r.withAgentError(err)
}

// The adapter answers a failed session/new with a bare "Internal error" and
// the cause in data.details, while its log ends on the brace of a dumped
// object; the cause is what the banner must show.
func TestAgentErrorLeadsWithTheAdaptersDetails(t *testing.T) {
	err := answering(t, "Error: spawn failed {\n  code: 'ENOENT'\n}\n", rpcError{
		Code: -32603, Message: "Internal error", Data: json.RawMessage(`{"details":"Claude native binary not found"}`),
	})
	message := err.Error()
	if !strings.Contains(message, "Internal error: Claude native binary not found") {
		t.Fatalf("details missing: %q", message)
	}
	if strings.Contains(message, "ENOENT") {
		t.Fatalf("the log was added beside the agent's own explanation: %q", message)
	}
}

func TestAgentErrorAcceptsDataAsAPlainString(t *testing.T) {
	err := answering(t, "", rpcError{Code: -32603, Message: "Internal error", Data: json.RawMessage(`"auth expired"`)})
	if !strings.Contains(err.Error(), "Internal error: auth expired") {
		t.Fatalf("string data dropped: %q", err)
	}
}

func TestBareInternalErrorNamesTheLastRealLogLine(t *testing.T) {
	err := answering(t, "boot noise\n2026-10-03 [ERROR] provider auth failed: 401\n}\n", rpcError{Code: -32603, Message: "Internal error"})
	message := err.Error()
	if !strings.HasSuffix(message, ": 2026-10-03 [ERROR] provider auth failed: 401") {
		t.Fatalf("log cause missing or a brace: %q", message)
	}
	if strings.Contains(message, "boot noise") {
		t.Fatalf("more than one log line was added: %q", message)
	}
}

func TestOtherAgentErrorsKeepOnlyTheirMessage(t *testing.T) {
	err := answering(t, "[session/create] phase=register durationMs=2\n", rpcError{Code: -32000, Message: "Authentication required"})
	if strings.Contains(err.Error(), "phase=register") || !strings.Contains(err.Error(), "Authentication required") {
		t.Fatalf("log leaked into a non-internal error: %q", err)
	}
}

func TestLogReasonSkipsLinesWithoutWords(t *testing.T) {
	tail := newTailBuffer(stderrTailLimit)
	_, _ = tail.Write([]byte("Error: boom\n    at x {\n  code: 'E'\n}\n"))
	if got := tail.reason(); got != "code: 'E'" {
		t.Fatalf("reason = %q", got)
	}
	_, _ = tail.Write([]byte("}\n  \n"))
	if got := tail.reason(); got != "code: 'E'" {
		t.Fatalf("trailing braces and blanks changed the reason: %q", got)
	}
}
