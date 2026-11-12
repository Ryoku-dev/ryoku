package gateway

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// a request carries its images inline, so the body is large but not endless
const apiBodyLimit = 32 << 20

// API serves the router to other programs on this machine in the shape of
// OpenAI's chat completions: GET /v1/models and POST /v1/chat/completions,
// both behind the API key.
func (g *Gateway) API() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", only(http.MethodGet, g.serveModels))
	mux.HandleFunc("/v1/chat/completions", only(http.MethodPost, g.serveChat))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, http.StatusNotFound, "not_found", i18n.T("Rashin serves /v1/models and /v1/chat/completions."))
	})
	return g.guardAPI(mux)
}

// guardAPI lets nothing through without the key. Loopback is no boundary by
// itself: every account and every web page on this machine can reach it.
func (g *Gateway) guardAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// a page that got its own name to resolve here still sends that name as the host
		if !loopbackHost(r.Host) {
			writeAPIError(w, http.StatusForbidden, "forbidden", i18n.T("This address answers only as localhost."))
			return
		}
		key, err := g.APIKey(r.Context(), false)
		presented := presentedKey(r)
		if err != nil || presented == "" || subtle.ConstantTimeCompare([]byte(presented), []byte(key)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="Rashin"`)
			writeAPIError(w, http.StatusUnauthorized, "invalid_api_key", i18n.T("The Rashin API key is missing or wrong."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loopbackHost(host string) bool {
	name := host
	if split, _, err := net.SplitHostPort(host); err == nil {
		name = split
	}
	name = strings.Trim(name, "[]")
	if strings.EqualFold(name, "localhost") {
		return true
	}
	address := net.ParseIP(name)
	return address != nil && address.IsLoopback()
}

func presentedKey(r *http.Request) string {
	if header := r.Header.Get("Authorization"); len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-Api-Key"))
}

func only(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", i18n.Tf("Use %s here.", method))
			return
		}
		next(w, r)
	}
}

type apiModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// serveModels lists the routes and the models that can answer now. The whole
// catalogue stays out: a client would offer every one of them as a choice.
func (g *Gateway) serveModels(w http.ResponseWriter, r *http.Request) {
	models, err := g.Models(r.Context(), protocol.ModelsParams{})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	list := make([]apiModel, 0, len(models.Routes)+len(models.Models))
	for _, route := range models.Routes {
		list = append(list, apiModel{ID: route.ID, Object: "model", OwnedBy: "rashin"})
	}
	for _, model := range models.Models {
		if model.Available {
			list = append(list, apiModel{ID: model.ID, Object: "model", OwnedBy: model.Provider})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": list})
}

func (g *Gateway) serveChat(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, apiBodyLimit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "request_too_large", i18n.T("The request is larger than 32 MiB."))
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	call, err := decodeChat(body)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	stream, err := g.Stream(r.Context(), call.request)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "no_model_available", err.Error())
		return
	}
	defer stream.Close()
	id, err := randomID()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	answer := apiAnswer{id: "chatcmpl-" + id, created: g.now().Unix(), model: call.request.Model}
	if call.stream {
		answer.relay(w, stream, call.usage)
		return
	}
	answer.whole(w, stream)
}

// apiAnswer writes one completion back in OpenAI's shape.
type apiAnswer struct {
	id      string
	created int64
	// model is the one the caller named until the one that answers is known
	model string
	tools int
}

type apiDelta struct {
	Role      string         `json:"role,omitempty"`
	Content   *string        `json:"content,omitempty"`
	Reasoning string         `json:"reasoning_content,omitempty"`
	ToolCalls []apiToolDelta `json:"tool_calls,omitempty"`
}

type apiToolDelta struct {
	Index    int          `json:"index"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type apiChoice struct {
	Index        int       `json:"index"`
	Delta        *apiDelta `json:"delta,omitempty"`
	Message      *apiDelta `json:"message,omitempty"`
	FinishReason *string   `json:"finish_reason"`
}

type apiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type apiCompletion struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []apiChoice `json:"choices"`
	Usage   *apiUsage   `json:"usage,omitempty"`
}

func (a *apiAnswer) toolCalls(calls []llm.ToolCall) []apiToolDelta {
	deltas := make([]apiToolDelta, len(calls))
	for index, call := range calls {
		id := call.ID
		if id == "" {
			// some free providers leave the id out, and the caller needs one to answer the call
			id = "call_" + strconv.Itoa(a.tools)
		}
		deltas[index] = apiToolDelta{Index: a.tools, ID: id, Type: "function", Function: wireFunction{Name: call.Name, Arguments: call.Arguments}}
		a.tools++
	}
	return deltas
}

func (a *apiAnswer) finish(final llm.Final) string {
	switch {
	case final.Finish != "":
		return final.Finish
	case a.tools > 0:
		return llm.FinishToolCalls
	}
	return llm.FinishStop
}

func usageOf(final llm.Final) *apiUsage {
	return &apiUsage{PromptTokens: final.Usage.Input, CompletionTokens: final.Usage.Output, TotalTokens: final.Usage.Input + final.Usage.Output}
}

// relay streams the completion as server-sent events. The status line waits
// for the first piece, so a request no provider could start still fails as an
// ordinary HTTP error and not as a broken stream.
func (a *apiAnswer) relay(w http.ResponseWriter, stream llm.Stream, usage bool) {
	flusher, _ := w.(http.Flusher)
	send := func(completion apiCompletion) {
		data, err := json.Marshal(completion)
		if err != nil {
			return
		}
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(data)
		_, _ = w.Write([]byte("\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}
	chunk := func(delta apiDelta, finish *string) apiCompletion {
		return apiCompletion{
			ID: a.id, Object: "chat.completion.chunk", Created: a.created, Model: a.model,
			Choices: []apiChoice{{Delta: &delta, FinishReason: finish}},
		}
	}
	started := false
	for {
		delta, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if !started {
				writeAPIError(w, http.StatusBadGateway, "upstream_failed", err.Error())
				return
			}
			data, _ := json.Marshal(apiErrorBody("upstream_failed", "server_error", err.Error()))
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		if !started {
			started = true
			header := w.Header()
			header.Set("Content-Type", "text/event-stream")
			header.Set("Cache-Control", "no-cache")
			header.Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			if routed, ok := stream.(*routedStream); ok {
				a.model = routed.answering()
			}
			send(chunk(apiDelta{Role: "assistant"}, nil))
		}
		piece := apiDelta{Reasoning: delta.Reasoning, ToolCalls: a.toolCalls(delta.ToolCalls)}
		if delta.Text != "" {
			piece.Content = &delta.Text
		}
		send(chunk(piece, nil))
	}
	if !started {
		writeAPIError(w, http.StatusBadGateway, "upstream_failed", i18n.T("The provider answered with nothing."))
		return
	}
	final := stream.Final()
	finish := a.finish(final)
	send(chunk(apiDelta{}, &finish))
	if usage {
		send(apiCompletion{ID: a.id, Object: "chat.completion.chunk", Created: a.created, Model: a.model, Choices: []apiChoice{}, Usage: usageOf(final)})
	}
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
}

// whole collects the stream and answers once, for a caller that did not ask to stream.
func (a *apiAnswer) whole(w http.ResponseWriter, stream llm.Stream) {
	var text, reasoning strings.Builder
	message := apiDelta{Role: "assistant"}
	for {
		delta, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeAPIError(w, http.StatusBadGateway, "upstream_failed", err.Error())
			return
		}
		text.WriteString(delta.Text)
		reasoning.WriteString(delta.Reasoning)
		message.ToolCalls = append(message.ToolCalls, a.toolCalls(delta.ToolCalls)...)
	}
	final := stream.Final()
	// an answer that is only tool calls has null content, as OpenAI sends it
	if content := text.String(); content != "" || len(message.ToolCalls) == 0 {
		message.Content = &content
	}
	message.Reasoning = reasoning.String()
	finish := a.finish(final)
	writeJSON(w, http.StatusOK, apiCompletion{
		ID: a.id, Object: "chat.completion", Created: a.created, Model: final.Model,
		Choices: []apiChoice{{Message: &message, FinishReason: &finish}}, Usage: usageOf(final),
	})
}

func apiErrorBody(code, kind, message string) map[string]any {
	return map[string]any{"error": map[string]any{"message": message, "type": kind, "code": code}}
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	kind := "invalid_request_error"
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		kind = "authentication_error"
	case status >= 500:
		kind = "server_error"
	}
	writeJSON(w, status, apiErrorBody(code, kind, message))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
