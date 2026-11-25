package localmodels

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestRouterRetriesOnProcessorAfterSecondGraphicsFailure(t *testing.T) {
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
			} else if loads.Load() > 2 {
				state = "loaded"
			}
			fmt.Fprintf(w, `{"data":[{"id":%q,"status":{"value":%q,"failed":%v}}]}`, e.ID, state, failed.Load())
		case "/models/load":
			if loads.Add(1) <= 2 {
				failed.Store(true)
				if err := os.WriteFile(paths.LlamaLog(), []byte("ggml_vk: device lost\n"), 0600); err != nil {
					t.Error(err)
				}
			}
			w.WriteHeader(200)
		case "/models/unload":
			failed.Store(false)
			w.WriteHeader(200)
		case "/props":
			fmt.Fprint(w, `{"default_generation_settings":{"n_ctx":4096},"model_info":{"n_gpu_layers_offloaded":0}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	m := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	s := &server{base: api.URL, key: "key", build: 12521, exited: make(chan struct{}),
		loaded: map[string]bool{}, loading: map[string]chan struct{}{}, presets: map[string]bool{e.ID: true},
		plans: map[string]protocol.LocalPlan{e.ID: {Device: "Vulkan0", Kind: protocol.LocalDeviceDiscrete, Context: 8192, Layers: 24, GPULayers: 24, KVType: protocol.LocalKVF16}}}
	if err := m.ensureLoaded(context.Background(), s, e); err != nil {
		t.Fatal(err)
	}
	if got := s.plans[e.ID]; loads.Load() != 3 || got.Kind != protocol.LocalDeviceCPU || got.Device != deviceNone || got.GPULayers != 0 || got.ActualContext != 4096 || got.Retry == "" {
		t.Fatalf("loads=%d, final plan=%+v", loads.Load(), got)
	}
}
