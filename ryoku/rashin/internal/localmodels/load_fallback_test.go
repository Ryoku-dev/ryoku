package localmodels

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestRouterRetriesFailedLoadWithSmallerContextAndRecordsTruth(t *testing.T) {
	e := testModel(t)
	if err := os.MkdirAll(filepath.Dir(paths.LlamaLog()), 0700); err != nil {
		t.Fatal(err)
	}
	var loads atomic.Int32
	var failed atomic.Bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			http.Error(w, "unauthorized", 401)
			return
		}
		switch r.URL.Path {
		case "/models":
			state := "unloaded"
			if failed.Load() {
				state = "failed"
			} else if loads.Load() > 1 {
				state = "loaded"
			}
			fmt.Fprintf(w, `{"data":[{"id":%q,"status":{"value":%q,"failed":%v}}]}`, e.ID, state, failed.Load())
		case "/models/load":
			if loads.Add(1) == 1 {
				failed.Store(true)
				if err := os.WriteFile(paths.LlamaLog(), []byte("ggml_vk: out of memory\n"), 0600); err != nil {
					t.Error(err)
				}
			}
			w.WriteHeader(200)
		case "/models/unload":
			failed.Store(false)
			w.WriteHeader(200)
		case "/props":
			fmt.Fprint(w, `{"default_generation_settings":{"n_ctx":4096}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	s := &server{base: api.URL, key: "key", build: 12521, exited: make(chan struct{}),
		loaded: map[string]bool{}, loading: map[string]chan struct{}{}, presets: map[string]bool{e.ID: true},
		plans: map[string]protocol.LocalPlan{e.ID: {Device: "Vulkan0", Kind: protocol.LocalDeviceDiscrete, DeviceName: "Graphics card", Context: 8192, Layers: 24, GPULayers: 24, KVType: protocol.LocalKVF16}}}
	if err := m.ensureLoaded(context.Background(), s, e); err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 2 {
		t.Fatalf("model loads=%d, want 2", loads.Load())
	}
	p := s.plans[e.ID]
	if p.Context != 4096 || p.ActualContext != 4096 || p.Retry == "" || !s.loaded[e.ID] {
		t.Fatalf("fallback plan=%+v, loaded=%v", p, s.loaded[e.ID])
	}
	ini, err := os.ReadFile(paths.LlamaPresets())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ini), "fit-ctx = 4096") || !strings.Contains(string(ini), "parallel = 1") {
		t.Fatalf("retry preset=%s", ini)
	}
}
