package hindsight

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	i18n "ryoku-i18n"
)

// Gateway is the part of Rashin's gateway a local Hindsight's model calls go
// through: its OpenAI-shaped API and the key it wants.
type Gateway interface {
	API() http.Handler
	APIKey(ctx context.Context, rotate bool) (string, error)
}

// chatPath is the one call whose model Rashin chooses.
const chatPath = "/v1/chat/completions"

// modelLimit bounds a chat request Hindsight sends; its prompts are text.
const modelLimit = 32 << 20

// modelListener serves the gateway's API on a loopback port while Hindsight
// runs. Every chat request is sent to the model memory learns with, chosen
// again for each request, so a local model that comes up is used at once.
type modelListener struct {
	server   *http.Server
	listener net.Listener
}

// listenModels opens the listener on a free loopback port. model returns the
// model to use, or "" while none may learn.
func listenModels(api http.Handler, model func(context.Context) string) (*modelListener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server := &http.Server{Handler: chooseModel(api, model), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return &modelListener{server: server, listener: listener}, nil
}

// base is the address Hindsight is given as its OpenAI base URL.
func (m *modelListener) base() string { return "http://" + m.listener.Addr().String() + "/v1" }

func (m *modelListener) close() { _ = m.server.Close() }

// chooseModel puts the learning model into each chat request before the
// gateway sees it.
func chooseModel(api http.Handler, model func(context.Context) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != chatPath {
			api.ServeHTTP(w, r)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, modelLimit))
		if err != nil {
			modelError(w, http.StatusBadRequest, err.Error())
			return
		}
		var request map[string]json.RawMessage
		if err := json.Unmarshal(raw, &request); err != nil {
			modelError(w, http.StatusBadRequest, err.Error())
			return
		}
		chosen := model(r.Context())
		if chosen == "" {
			modelError(w, http.StatusServiceUnavailable, i18n.T("Memory learns only with a local model, and none is running."))
			return
		}
		request["model"], _ = json.Marshal(chosen)
		raw, err = json.Marshal(request)
		if err != nil {
			modelError(w, http.StatusInternalServerError, err.Error())
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		r.ContentLength = int64(len(raw))
		api.ServeHTTP(w, r)
	})
}

func modelError(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": text, "type": "rashin_memory"}})
}
