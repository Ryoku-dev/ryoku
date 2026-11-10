package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func pictureRequest(model string, strict bool) llm.Request {
	return llm.Request{Model: model, Strict: strict, Messages: []llm.Message{{
		Role: llm.RoleUser, Content: "What does this say?", Images: []llm.Image{{Mime: "image/png", Data: []byte("picture")}},
	}}}
}

// The best model on the list is often one that cannot see. Sent a picture, it
// answers about a picture it never saw, so the router has to pass it over.
func TestAPictureGoesOnlyToAModelThatReadsPictures(t *testing.T) {
	var blindHits atomic.Int32
	blind := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blindHits.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"a guess"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer blind.Close()
	var seen atomic.Value
	sighted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.Store(string(body))
		sse(w, `{"choices":[{"delta":{"content":"it says hello"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer sighted.Close()

	gateway := newTestGateway(t)
	reads := testProvider("sighted", sighted.URL, 60, true)
	reads.Models[0].Vision = true
	setTestProviders(gateway, testProvider("blind", blind.URL, 100, true), reads)

	stream, err := gateway.Stream(context.Background(), pictureRequest(protocol.RouteAuto, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collect(t, stream); err != nil {
		t.Fatal(err)
	}
	if blindHits.Load() != 0 {
		t.Fatalf("a model without vision was sent the picture %d times", blindHits.Load())
	}
	var sent struct {
		Messages []struct {
			Content []struct {
				Type     string `json:"type"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"content"`
		} `json:"messages"`
	}
	body, _ := seen.Load().(string)
	if err := json.Unmarshal([]byte(body), &sent); err != nil || len(sent.Messages) != 1 || len(sent.Messages[0].Content) != 2 ||
		!strings.HasPrefix(sent.Messages[0].Content[1].ImageURL.URL, "data:image/png;base64,") {
		t.Fatalf("the model that reads pictures was sent %s (%v)", body, err)
	}

	// without a picture the better model is still the one that answers
	stream, err = gateway.Stream(context.Background(), llm.Request{Model: protocol.RouteAuto, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collect(t, stream); err != nil || blindHits.Load() != 1 {
		t.Fatalf("a question without a picture reached the better model %d times (%v)", blindHits.Load(), err)
	}
}

func TestAPictureNoProviderCanReadSaysWhoCould(t *testing.T) {
	gateway := newTestGateway(t)
	could := testProvider("could", "http://127.0.0.1:1", 50, true)
	could.Name, could.Keyless, could.Auth = "Sighted Cloud", false, "bearer"
	could.Models[0].Vision = true
	setTestProviders(gateway, testProvider("blind", "http://127.0.0.1:1", 100, true), could)

	_, err := gateway.Stream(context.Background(), pictureRequest(protocol.RouteAuto, false))
	if err == nil || !strings.Contains(err.Error(), "Sighted Cloud") {
		t.Fatalf("with no provider in use that reads pictures the router said %v", err)
	}
	_, err = gateway.Stream(context.Background(), pictureRequest("blind/chat", true))
	if err == nil || !strings.Contains(err.Error(), "cannot read pictures") {
		t.Fatalf("a picture pinned to a model without vision got %v", err)
	}
}
