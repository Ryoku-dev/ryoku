package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type apiHarness struct {
	t      *testing.T
	server *httptest.Server
	key    string
}

func newAPIHarness(t *testing.T, providers ...provider) (*Gateway, *apiHarness) {
	t.Helper()
	gateway := newTestGateway(t)
	setTestProviders(gateway, providers...)
	key, err := gateway.APIKey(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(gateway.API())
	t.Cleanup(server.Close)
	return gateway, &apiHarness{t: t, server: server, key: key}
}

func (h *apiHarness) do(method, path, body string, headers map[string]string) (*http.Response, string) {
	h.t.Helper()
	request, err := http.NewRequest(method, h.server.URL+path, strings.NewReader(body))
	if err != nil {
		h.t.Fatal(err)
	}
	for name, value := range headers {
		if name == "Host" {
			request.Host = value
			continue
		}
		request.Header.Set(name, value)
	}
	response, err := h.server.Client().Do(request)
	if err != nil {
		h.t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	return response, string(data)
}

func (h *apiHarness) chat(body string) (*http.Response, string) {
	return h.do(http.MethodPost, "/v1/chat/completions", body, map[string]string{"Authorization": "Bearer " + h.key})
}

// frames splits a server-sent stream into the JSON of each data line.
func frames(t *testing.T, stream string) []map[string]any {
	t.Helper()
	var decoded []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(stream))
	for scanner.Scan() {
		line := strings.TrimPrefix(scanner.Text(), "data: ")
		if line == scanner.Text() || line == "[DONE]" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("frame %q: %v", line, err)
		}
		decoded = append(decoded, frame)
	}
	return decoded
}

func answering(t *testing.T, id string, frames ...string) (provider, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, r.Header.Get("Authorization")+"\n"+string(data))
		mu.Unlock()
		sse(w, frames...)
	}))
	t.Cleanup(upstream.Close)
	return testProvider(id, upstream.URL, 100, true), &bodies
}

func TestAPIAnswersNobodyWithoutTheKey(t *testing.T) {
	up, _ := answering(t, "test", `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
	_, api := newAPIHarness(t, up)
	attempts := map[string]map[string]string{
		"no credential":        {},
		"another key":          {"Authorization": "Bearer rashin-0000"},
		"the start of the key": {"Authorization": "Bearer " + api.key[:len(api.key)-1]},
		"an empty bearer":      {"Authorization": "Bearer "},
		"a bare scheme":        {"Authorization": "Bearer"},
		"the wrong scheme":     {"Authorization": "Basic " + api.key},
		"an empty header key":  {"X-Api-Key": ""},
	}
	for name, headers := range attempts {
		for _, path := range []string{"/v1/models", "/v1/chat/completions", "/", "/v1/unknown"} {
			response, body := api.do(http.MethodGet, path, "", headers)
			if response.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s on %s: status %d, want 401", name, path, response.StatusCode)
			}
			if strings.Contains(body, api.key) {
				t.Errorf("%s on %s: the answer carries the key", name, path)
			}
		}
	}
	for name, headers := range map[string]map[string]string{
		"a bearer":         {"Authorization": "Bearer " + api.key},
		"the key header":   {"X-Api-Key": api.key},
		"a lowercase word": {"Authorization": "bearer " + api.key},
	} {
		if response, _ := api.do(http.MethodGet, "/v1/models", "", headers); response.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d, want 200", name, response.StatusCode)
		}
	}
}

func TestAPIAnswersOnlyAsLocalhost(t *testing.T) {
	_, api := newAPIHarness(t)
	headers := map[string]string{"Authorization": "Bearer " + api.key, "Host": "rebound.example:3600"}
	if response, _ := api.do(http.MethodGet, "/v1/models", "", headers); response.StatusCode != http.StatusForbidden {
		t.Fatalf("a foreign host name got status %d, want 403", response.StatusCode)
	}
	headers["Host"] = "localhost:3600"
	if response, _ := api.do(http.MethodGet, "/v1/models", "", headers); response.StatusCode != http.StatusOK {
		t.Fatalf("localhost got status %d, want 200", response.StatusCode)
	}
}

func TestAPIKeyStaysUntilRotated(t *testing.T) {
	gateway, api := newAPIHarness(t)
	again, err := gateway.APIKey(context.Background(), false)
	if err != nil || again != api.key {
		t.Fatalf("the key changed between two reads: %q then %q (%v)", api.key, again, err)
	}
	rotated, err := gateway.APIKey(context.Background(), true)
	if err != nil || rotated == api.key || !strings.HasPrefix(rotated, apiKeyPrefix) {
		t.Fatalf("rotating gave %q (%v) after %q", rotated, err, api.key)
	}
	if response, _ := api.do(http.MethodGet, "/v1/models", "", map[string]string{"Authorization": "Bearer " + api.key}); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("the old key still got status %d", response.StatusCode)
	}
	if response, _ := api.do(http.MethodGet, "/v1/models", "", map[string]string{"Authorization": "Bearer " + rotated}); response.StatusCode != http.StatusOK {
		t.Fatalf("the new key got status %d", response.StatusCode)
	}
}

func TestAPIListsRoutesAndWhatCanAnswer(t *testing.T) {
	up, _ := answering(t, "up")
	keyed := testProvider("keyed", "http://127.0.0.1:1", 90, true)
	keyed.Keyless, keyed.Auth = false, "bearer"
	_, api := newAPIHarness(t, up, keyed)
	_, body := api.do(http.MethodGet, "/v1/models", "", map[string]string{"Authorization": "Bearer " + api.key})
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, entry := range list.Data {
		ids = append(ids, entry.ID)
	}
	if got := strings.Join(ids, " "); got != "auto auto:fast auto:smart up/chat" {
		t.Fatalf("models = %q", got)
	}
}

func TestAPIStreamsACompletion(t *testing.T) {
	up, bodies := answering(t, "test",
		`{"choices":[{"delta":{"role":"assistant","reasoning_content":"think "}}]}`,
		`{"choices":[{"delta":{"content":"done"}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"search","arguments":"{\"q\":\"ryoku\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":11,"completion_tokens":7}}`,
		`[DONE]`,
	)
	_, api := newAPIHarness(t, up)
	response, stream := api.chat(`{
		"model": "auto", "stream": true, "stream_options": {"include_usage": true}, "top_p": 0.5, "tool_choice": "required", "user": "someone",
		"messages": [{"role": "system", "content": "be brief"}, {"role": "user", "content": [{"type": "text", "text": "find it"}]}],
		"tools": [{"type": "function", "function": {"name": "search", "parameters": {"type": "object"}}}]
	}`)
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status %d, content type %q: %s", response.StatusCode, response.Header.Get("Content-Type"), stream)
	}
	if !strings.HasSuffix(strings.TrimSpace(stream), "data: [DONE]") {
		t.Fatalf("the stream does not end with [DONE]: %q", stream)
	}
	sent := frames(t, stream)
	if len(sent) != 6 {
		t.Fatalf("%d frames, want role, thought, text, tool call, finish and usage: %s", len(sent), stream)
	}
	delta := func(index int) map[string]any {
		return sent[index]["choices"].([]any)[0].(map[string]any)["delta"].(map[string]any)
	}
	if delta(0)["role"] != "assistant" || delta(1)["reasoning_content"] != "think " || delta(2)["content"] != "done" {
		t.Fatalf("role, thought or text frame is wrong: %s", stream)
	}
	for index, frame := range sent {
		if frame["model"] != "test/chat" {
			t.Fatalf("frame %d names the model %v, want the one that answered", index, frame["model"])
		}
	}
	call := delta(3)["tool_calls"].([]any)[0].(map[string]any)
	if call["id"] != "call_1" || call["function"].(map[string]any)["arguments"] != `{"q":"ryoku"}` {
		t.Fatalf("tool call frame = %#v", call)
	}
	last := sent[4]["choices"].([]any)[0].(map[string]any)
	if last["finish_reason"] != "tool_calls" {
		t.Fatalf("finish frame = %#v", sent[4])
	}
	usage := sent[5]["usage"].(map[string]any)
	if usage["prompt_tokens"] != float64(11) || usage["completion_tokens"] != float64(7) || usage["total_tokens"] != float64(18) {
		t.Fatalf("usage frame = %#v", usage)
	}

	authorization, body, _ := strings.Cut((*bodies)[0], "\n")
	if authorization != "" || strings.Contains(body, api.key) {
		t.Fatalf("the provider was sent the caller's key: %q", authorization)
	}
	var upstream map[string]any
	if err := json.Unmarshal([]byte(body), &upstream); err != nil {
		t.Fatal(err)
	}
	if upstream["top_p"] != 0.5 || upstream["tool_choice"] != "required" {
		t.Fatalf("top_p and tool_choice were not passed on: %s", body)
	}
	if _, passed := upstream["user"]; passed {
		t.Fatalf("a field Rashin does not pass on reached the provider: %s", body)
	}
}

func TestAPIAnswersWholeWhenNotAskedToStream(t *testing.T) {
	up, _ := answering(t, "test",
		`{"choices":[{"delta":{"content":"all "}}]}`,
		`{"choices":[{"delta":{"content":"good"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`,
		`[DONE]`,
	)
	_, api := newAPIHarness(t, up)
	response, body := api.chat(`{"messages": [{"role": "user", "content": "status?"}]}`)
	var completion struct {
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(body), &completion); err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("status %d, %v: %s", response.StatusCode, err, body)
	}
	if completion.Object != "chat.completion" || completion.Model != "test/chat" || len(completion.Choices) != 1 {
		t.Fatalf("completion = %s", body)
	}
	choice := completion.Choices[0]
	if choice.Message.Role != "assistant" || choice.Message.Content != "all good" || choice.FinishReason != "stop" || completion.Usage.TotalTokens != 5 {
		t.Fatalf("completion = %s", body)
	}
}

func TestAPIFailsBeforeTheStreamAsAnHTTPError(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()
	_, api := newAPIHarness(t, testProvider("broken", broken.URL, 100, true))
	response, body := api.chat(`{"stream": true, "messages": [{"role": "user", "content": "hello"}]}`)
	if response.StatusCode != http.StatusBadGateway || !strings.Contains(body, `"error"`) || strings.Contains(body, "data:") {
		t.Fatalf("status %d: %s", response.StatusCode, body)
	}
}

func TestAPIGivesTheNamedModelOrNothing(t *testing.T) {
	up, bodies := answering(t, "up", `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
	_, api := newAPIHarness(t, up)
	response, body := api.chat(`{"model": "elsewhere/chat", "messages": [{"role": "user", "content": "hello"}]}`)
	if response.StatusCode != http.StatusServiceUnavailable || len(*bodies) != 0 {
		t.Fatalf("a model that is not here was answered by another: status %d, %s", response.StatusCode, body)
	}
	// Prowl's other route names still mean the router picks
	if response, body := api.chat(`{"model": "auto:cheap", "messages": [{"role": "user", "content": "hello"}]}`); response.StatusCode != http.StatusOK {
		t.Fatalf("auto:cheap got status %d: %s", response.StatusCode, body)
	}
}

func TestAPIRefusesWhatItCannotPassOn(t *testing.T) {
	up, bodies := answering(t, "up", `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
	_, api := newAPIHarness(t, up)
	for name, body := range map[string]string{
		"not JSON":              `hello`,
		"no messages":           `{"model": "auto"}`,
		"two choices":           `{"n": 2, "messages": [{"role": "user", "content": "hello"}]}`,
		"an unknown role":       `{"messages": [{"role": "narrator", "content": "hello"}]}`,
		"an image by address":   `{"messages": [{"role": "user", "content": [{"type": "image_url", "image_url": {"url": "http://127.0.0.1:1/x.png"}}]}]}`,
		"a tool without a name": `{"messages": [{"role": "user", "content": "hello"}], "tools": [{"type": "function", "function": {}}]}`,
	} {
		if response, answer := api.chat(body); response.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400: %s", name, response.StatusCode, answer)
		}
	}
	if len(*bodies) != 0 {
		t.Fatalf("a refused request still reached a provider: %v", *bodies)
	}
}

func TestAPIPassesAnInlineImageOnToAModelThatReadsPictures(t *testing.T) {
	const picture = `{"messages": [{"role": "user", "content": [
		{"type": "text", "text": "what is this"},
		{"type": "image_url", "image_url": {"url": "data:image/png;base64,aGVsbG8="}}
	]}]}`
	up, bodies := answering(t, "up", `{"choices":[{"delta":{"content":"a cat"},"finish_reason":"stop"}]}`, `[DONE]`)
	up.Models[0].Vision = true
	_, api := newAPIHarness(t, up)
	response, body := api.chat(picture)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", response.StatusCode, body)
	}
	if !strings.Contains((*bodies)[0], "data:image/png;base64,aGVsbG8=") {
		t.Fatalf("the image did not reach the provider: %s", (*bodies)[0])
	}

	blind, unseen := answering(t, "blind", `{"choices":[{"delta":{"content":"a guess"},"finish_reason":"stop"}]}`, `[DONE]`)
	_, api = newAPIHarness(t, blind)
	if response, body := api.chat(picture); response.StatusCode != http.StatusServiceUnavailable || len(*unseen) != 0 {
		t.Fatalf("with no model that reads pictures the API answered %d %s after %d upstream calls", response.StatusCode, body, len(*unseen))
	}
}
