package mcpservers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func TestSSEClientUsesAdvertisedSameOriginEndpoint(t *testing.T) {
	var mu sync.Mutex
	var events chan string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			mu.Lock()
			events = make(chan string, 4)
			out := events
			mu.Unlock()
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: endpoint\ndata: /messages\n\n")
			w.(http.Flusher).Flush()
			for {
				select {
				case line := <-out:
					fmt.Fprint(w, "data: "+line+"\n\n")
					w.(http.Flusher).Flush()
				case <-req.Context().Done():
					return
				}
			}
		}
		if req.URL.Path != "/messages" {
			t.Errorf("sent message to %s", req.URL.Path)
		}
		var call struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(req.Body).Decode(&call); err != nil {
			t.Error(err)
		}
		mu.Lock()
		out := events
		mu.Unlock()
		if call.ID > 0 {
			response := `{"jsonrpc":"2.0","id":%d,"result":%s}`
			result := `{}`
			if call.Method == "initialize" {
				result = `{"protocolVersion":"2025-03-26"}`
			}
			if call.Method == "tools/list" {
				result = `{"tools":[{"name":"inspect","inputSchema":{"type":"object"}}]}`
			}
			if call.Method == "tools/call" {
				result = `{"content":[{"type":"text","text":"SSE works"}]}`
			}
			out <- fmt.Sprintf(response, call.ID, result)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := openClient(ctx, protocol.MCPServerInfo{Type: "sse", URL: server.URL + "/events"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	found, err := client.list()
	if err != nil || len(found) != 1 {
		t.Fatalf("list: %v %v", found, err)
	}
	result, err := client.call("tools/call", map[string]any{"name": "inspect", "arguments": map[string]any{}})
	if err != nil || !strings.Contains(string(result), "SSE works") {
		t.Fatalf("call: %s %v", result, err)
	}
}
