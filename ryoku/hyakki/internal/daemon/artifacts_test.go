package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/protocol"
)

func TestDisablingArtifactsClosesOpenPreviews(t *testing.T) {
	isolateDaemon(t)
	initial := config.Defaults()
	initial.Enabled = true
	initial.Features = map[string]bool{"artifacts": true}
	if err := config.Save(initial); err != nil {
		t.Fatal(err)
	}
	d := New(Deps{Version: "test"})
	d.mu.Lock()
	d.settings = initial
	d.mu.Unlock()
	t.Cleanup(d.Shutdown)
	opened, err := artifactPreviews.Open(context.Background(), protocol.ArtifactOpenParams{Session: "test", Code: "<p>Page</p>", Kind: "html"}, d.artifactAllowed)
	if err != nil {
		t.Fatal(err)
	}
	if d.Idle() {
		t.Fatal("daemon can exit while a preview is open")
	}
	if _, err := d.applySettings(context.Background(), json.RawMessage(`{"features":{"artifacts":false}}`)); err != nil {
		t.Fatal(err)
	}
	if artifactPreviews.Busy() || !d.Idle() {
		t.Fatal("disabled previews keep the daemon awake")
	}
	if response, err := http.Get(opened.URL); err == nil {
		_ = response.Body.Close()
		t.Fatal("disabled preview listener is still open")
	}
	if _, err := artifactPreviews.Open(context.Background(), protocol.ArtifactOpenParams{Session: "test", Code: "<p>Page</p>", Kind: "html"}, d.artifactAllowed); err == nil {
		t.Fatal("an in-flight preview can reopen after the feature is disabled")
	}
}

func TestShutdownClosesArtifactPreviews(t *testing.T) {
	isolateDaemon(t)
	d := New(Deps{Version: "test"})
	d.mu.Lock()
	d.settings.Features = map[string]bool{"artifacts": true}
	d.mu.Unlock()
	t.Cleanup(d.Shutdown)
	opened, err := artifactPreviews.Open(context.Background(), protocol.ArtifactOpenParams{Session: "test", Code: "<p>Page</p>", Kind: "html"}, d.artifactAllowed)
	if err != nil {
		t.Fatal(err)
	}
	d.Shutdown()
	if artifactPreviews.Busy() || d.artifactAllowed() {
		t.Fatal("shutdown left artifact previews active")
	}
	if response, err := http.Get(opened.URL); err == nil {
		_ = response.Body.Close()
		t.Fatal("shutdown left the preview listener open")
	}
}
