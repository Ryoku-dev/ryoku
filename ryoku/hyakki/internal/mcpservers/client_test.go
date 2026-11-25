package mcpservers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

type mcpPermission struct {
	called  int
	allowed bool
}

func (p *mcpPermission) Allow(_ context.Context, req tool.Request) (tool.Decision, error) {
	p.called++
	return tool.Decision{Allowed: p.allowed, Reason: "Denied"}, nil
}
func (*mcpPermission) Choose(context.Context, tool.Question) (string, error) { return "", nil }

func mcpEnv() tool.Env {
	return tool.Env{Settings: func() protocol.Settings {
		return protocol.Settings{Features: map[string]bool{protocol.FeatureMCP: true}}
	}, Limits: tool.Limits{Told: 1000, Head: 400, Tail: 400}}
}

func TestClientListsAndCallsWithVaultHeaders(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Private"); got != "vault-secret" {
			t.Errorf("missing vault header: %q", got)
		}
		var request struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Method != "initialize" && r.Header.Get("Mcp-Session-Id") != "test-session" {
			t.Errorf("lost session after initialization: %q", r.Header.Get("Mcp-Session-Id"))
		}
		mu.Lock()
		seen = append(seen, request.Method)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "test-session")
		switch request.Method {
		case "initialize":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-03-26"}}`, request.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			var page struct {
				Cursor string `json:"cursor"`
			}
			_ = json.Unmarshal(request.Params, &page)
			if page.Cursor == "" {
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"nextCursor":"second","tools":[{"name":"read","description":"Read a thing","inputSchema":{"type":"object","properties":{}},"annotations":{"readOnlyHint":true}}]}}`, request.ID)
			} else {
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":"write","description":"Write a thing","inputSchema":{"type":"object","properties":{"text":{"type":"string"}}}}]}}`, request.ID)
			}
		case "tools/call":
			var call struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.Unmarshal(request.Params, &call)
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":%q}]}}`, request.ID, call.Name)
		default:
			t.Errorf("unexpected method %s", request.Method)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	v := vault.OpenWith(filepath.Join(dir, "vault.db"), testKeeper{})
	defer v.Close()
	store, err := Open(filepath.Join(dir, "servers.json"), v)
	if err != nil {
		t.Fatal(err)
	}
	info, err := store.Save(protocol.MCPServer{Name: "Service", Type: "http", URL: server.URL, Headers: map[string]string{"X-Private": "vault-secret"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if checked, err := store.Check(context.Background(), info.ID); err != nil || checked.Tools != 2 {
		t.Fatalf("check: %+v %v", checked, err)
	}
	tools := NewProvider(func() (*Store, error) { return store, nil }).Tools(context.Background(), mcpEnv())
	if len(tools) != 2 || tools[0].Name() != "mcp_service_read" || tools[1].Name() != "mcp_service_write" {
		t.Fatalf("tools: %v", tools)
	}
	if !tools[0].ReadOnly() || tools[1].ReadOnly() {
		t.Fatal("readOnlyHint not applied")
	}
	read := tools[0].Run(context.Background(), tool.Call{}, mcpEnv())
	if read.IsError || read.Text != "read" {
		t.Fatalf("read: %+v", read)
	}
	blocked := mcpEnv()
	denied := &mcpPermission{allowed: false}
	blocked.Permissions = denied
	if result := tools[1].Run(context.Background(), tool.Call{Args: json.RawMessage(`{"text":"hi"}`)}, blocked); !result.IsError || denied.called != 1 {
		t.Fatalf("permission: %+v %+v", result, denied)
	}
	approved := &mcpPermission{allowed: true}
	blocked.Permissions = approved
	if result := tools[1].Run(context.Background(), tool.Call{Args: json.RawMessage(`{"text":"hi"}`)}, blocked); result.IsError || result.Text != "write" || approved.called != 1 {
		t.Fatalf("approved: %+v %+v", result, approved)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Count(strings.Join(seen, ","), "tools/call") != 2 {
		t.Fatalf("unexpected call sequence: %v", seen)
	}
	if strings.Count(strings.Join(seen, ","), "tools/list") != 2 {
		t.Fatalf("tool definitions were listed again after a successful check: %v", seen)
	}
	raw, _ := json.Marshal(store.List())
	if strings.Contains(string(raw), "vault-secret") {
		t.Fatal("vault secret leaked through public server metadata")
	}
}

func TestFeatureOffDoesNotOpenServer(t *testing.T) {
	opens := 0
	provider := NewProvider(func() (*Store, error) { opens++; return nil, nil })
	if tools := provider.Tools(context.Background(), tool.Env{}); len(tools) != 0 || opens != 0 {
		t.Fatalf("opened when disabled: %d %v", opens, tools)
	}
}

func TestMCPStdioChild(t *testing.T) {
	if os.Getenv("HYAKKI_MCP_TEST_CHILD") != "1" {
		return
	}
	r := bufio.NewScanner(os.Stdin)
	for r.Scan() {
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(r.Bytes(), &req) != nil {
			os.Exit(2)
		}
		switch req.Method {
		case "initialize":
			fmt.Printf(`{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-03-26"}}`+"\n", req.ID)
		case "tools/list":
			fmt.Printf(`{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":"probe","inputSchema":{"type":"object"}}]}}`+"\n", req.ID)
		case "tools/call":
			fmt.Printf(`{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"stdio works"}]}}`+"\n", req.ID)
		}
	}
	os.Exit(0)
}

func TestStdioClientStartsAndStopsServerForCall(t *testing.T) {
	old := os.Getenv("HYAKKI_MCP_TEST_CHILD")
	if err := os.Setenv("HYAKKI_MCP_TEST_CHILD", "1"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HYAKKI_MCP_TEST_CHILD", old)
	ctx := context.Background()
	client, err := openClient(ctx, protocol.MCPServerInfo{Type: "stdio", Command: os.Args[0], Args: []string{"-test.run=^TestMCPStdioChild$"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	tools, err := client.list()
	if err != nil || len(tools) != 1 || tools[0].Name != "probe" {
		t.Fatalf("list: %v %v", tools, err)
	}
	raw, err := client.call("tools/call", map[string]any{"name": "probe", "arguments": map[string]any{}})
	if err != nil || !strings.Contains(string(raw), "stdio works") {
		t.Fatalf("call: %s %v", raw, err)
	}
}
