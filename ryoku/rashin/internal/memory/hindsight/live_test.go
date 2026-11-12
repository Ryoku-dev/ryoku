package hindsight

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/protocol"
)

// proxyGateway stands in for Rashin's gateway: it checks the gateway key and
// passes model calls to an OpenAI-compatible server.
type proxyGateway struct{ target *url.URL }

func (g proxyGateway) API() http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(g.target)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gateway-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

func (proxyGateway) APIKey(context.Context, bool) (string, error) { return "gateway-key", nil }

// leftovers lists the processes whose command line names dir: a
// hindsight-api or a Postgres under it.
func leftovers(dir string) []string {
	var found []string
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err == nil && strings.Contains(string(raw), dir) {
			found = append(found, entry.Name()+": "+strings.ReplaceAll(string(raw), "\x00", " "))
		}
	}
	return found
}

// TestLive sets up a real Hindsight from the shipped lock under
// RASHIN_HINDSIGHT_LIVE and runs it against the OpenAI-compatible fake at
// RASHIN_HINDSIGHT_LLM. It needs uv and the network, and takes minutes the
// first time.
func TestLive(t *testing.T) {
	dir, llm := os.Getenv("RASHIN_HINDSIGHT_LIVE"), os.Getenv("RASHIN_HINDSIGHT_LLM")
	if dir == "" || llm == "" {
		t.Skip("set RASHIN_HINDSIGHT_LIVE to a scratch folder and RASHIN_HINDSIGHT_LLM to a fake model server")
	}
	target, err := url.Parse(llm)
	if err != nil {
		t.Fatal(err)
	}
	layout := Layout{Data: filepath.Join(dir, "data"), State: filepath.Join(dir, "state"), Cache: filepath.Join(dir, "cache")}
	proc := NewProcess(Config{
		Layout: layout, Secrets: &fakeSecrets{values: map[string]string{}}, Gateway: proxyGateway{target: target},
		Model: func(context.Context, string) string { return "fake-model" },
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	start := time.Now()
	err = proc.Setup(ctx, func(step string, part, parts int) {
		t.Logf("%s setup %d/%d: %s", time.Since(start).Round(time.Second), part, parts, step)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !layout.Installed() {
		t.Fatal("setup finished without marking the install")
	}
	if left := leftovers(dir); len(left) != 0 {
		t.Fatalf("setup left processes: %q", left)
	}
	h := Local(proc)
	defer h.Close()

	start = time.Now()
	kept, err := h.Remember(ctx, []protocol.Memory{
		{Scope: protocol.MemoryYou, Text: "The user prefers the Helix editor.", Source: protocol.MemorySource{Kind: protocol.MemoryFromYou}},
		{Scope: protocol.MemoryProjectPrefix + "3f2a9c1d0b7e", Text: "The project builds with meson."},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("start and first remember took %s", time.Since(start).Round(time.Millisecond))
	if kept[0].Text != "The user prefers the Helix editor." {
		t.Fatalf("an exact remember came back as %q", kept[0].Text)
	}
	found, err := h.Recall(ctx, backend.Query{Text: "Which editor does the user prefer?", Scopes: []string{protocol.MemoryYou}, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 || found[0].ID != kept[0].ID {
		t.Fatalf("recall found %+v", found)
	}

	err = h.Observe(ctx, backend.Transcript{
		Session: "live-1", Kind: protocol.MemoryFromAsk, Title: "Languages", Scope: protocol.MemoryYou,
		Turns: []backend.Turn{{At: time.Now(), User: "I write Go every day. I live in Lisbon.", Reply: "Noted."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var learned []protocol.Memory
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); time.Sleep(time.Second) {
		all, _, err := h.List(ctx, protocol.MemoryYou, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		learned = slices.DeleteFunc(all, func(m protocol.Memory) bool { return m.Source.Session != "live-1" })
		if len(learned) > 0 {
			break
		}
	}
	if len(learned) == 0 {
		t.Fatal("nothing was learned from the observed turn")
	}
	t.Logf("learned %d facts from the turn: %q", len(learned), learned[0].Text)

	// a learned fact is invalidated, an exact one deleted with its document
	if err := h.Forget(ctx, []string{learned[0].ID, kept[0].ID}); err != nil {
		t.Fatal(err)
	}
	all, _, err := h.List(ctx, protocol.MemoryYou, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(all, func(m protocol.Memory) bool { return m.ID == learned[0].ID || m.ID == kept[0].ID }) {
		t.Fatalf("forgotten facts are still listed: %+v", all)
	}
	var rest []string
	for _, m := range all {
		rest = append(rest, m.ID)
	}
	if err := h.Forget(ctx, rest); err != nil {
		t.Fatal(err)
	}
	if all, _, err = h.List(ctx, protocol.MemoryYou, 0, 100); err != nil || len(all) != 0 {
		t.Fatalf("after forgetting the rest: %+v, %v", all, err)
	}
	if _, err := h.Card(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.ForgetScope(ctx, protocol.MemoryProjectPrefix+"3f2a9c1d0b7e"); err != nil {
		t.Fatal(err)
	}
	if projects, _, err := h.List(ctx, protocol.MemoryProjects, 0, 10); err != nil || len(projects) != 0 {
		t.Fatalf("after forgetting the project: %+v, %v", projects, err)
	}

	base, _, err := proc.endpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(base + "/v1/default/banks")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a call without the key got %d", resp.StatusCode)
	}
	api, err := h.conn.api(ctx)
	if err != nil {
		t.Fatal(err)
	}
	version, _, err := api.MonitoringAPI.GetVersion(ctx).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if version.Features.LlmTrace || version.Features.StoreDocumentText || version.Features.Mcp || version.Features.FileUploadApi {
		t.Fatalf("hardening is off: %+v", version.Features)
	}
	t.Logf("Hindsight %s", version.ApiVersion)
	if status := h.Status(ctx); status.State != protocol.EngineReady {
		t.Fatalf("status %+v", status)
	}

	stop := time.Now()
	h.Release()
	t.Logf("stop took %s", time.Since(stop).Round(time.Millisecond))
	if left := leftovers(dir); len(left) != 0 {
		t.Fatalf("processes outlived Close: %q", left)
	}
}
