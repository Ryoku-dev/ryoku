package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// providersGateway counts how often the providers are read for a broadcast.
type providersGateway struct {
	core.Gateway
	read int
}

func (g *providersGateway) Providers(context.Context) ([]protocol.ProviderInfo, error) {
	g.read++
	return nil, nil
}

// The gateway sends requests to a local provider's address, so only the app
// may name one, and only on this computer or the local network. The new
// address is announced with the providers.
func TestOnlyTheAppChangesALocalProviderAddress(t *testing.T) {
	isolateDaemon(t)
	initial := config.Defaults()
	initial.Enabled = true
	if err := config.Save(initial); err != nil {
		t.Fatal(err)
	}
	gateway := &providersGateway{}
	d := New(Deps{Gateway: gateway, Version: "test"})
	d.mu.Lock()
	d.settings = initial
	d.mu.Unlock()
	set := func(c *connection, patch string) error {
		raw, err := json.Marshal(protocol.SettingsSetParams{Patch: json.RawMessage(patch)})
		if err != nil {
			t.Fatal(err)
		}
		_, err = d.dispatch(context.Background(), c, protocol.MSettingsSet, raw)
		return err
	}
	lan := `{"providers":{"local":{"ollama-local":"http://192.168.1.20:11434/v1"}}}`
	for _, patch := range []string{lan, `{"providers":null}`} {
		if err := set(&connection{}, patch); !forbidden(err) {
			t.Fatalf("another client setting %s: %v, want forbidden", patch, err)
		}
	}
	err := set(&connection{app: true}, `{"providers":{"local":{"ollama-local":"http://203.0.113.9:11434/v1"}}}`)
	var refused *protocol.Error
	if !errors.As(err, &refused) || refused.Code != protocol.CodeBadRequest {
		t.Fatalf("the app naming a public address: %v, want a bad request", err)
	}
	if gateway.read != 0 || d.Settings().Providers.Local[protocol.ProviderOllamaLocal] != "http://127.0.0.1:11434/v1" {
		t.Fatalf("a refused patch changed the address or announced it: %#v", d.Settings().Providers)
	}
	if err := set(&connection{app: true}, lan); err != nil {
		t.Fatalf("the app naming a local network address: %v", err)
	}
	if d.Settings().Providers.Local[protocol.ProviderOllamaLocal] != "http://192.168.1.20:11434/v1" {
		t.Fatalf("the address was not saved: %#v", d.Settings().Providers)
	}
	if gateway.read != 1 {
		t.Fatalf("providers read %d times after the address changed, want once", gateway.read)
	}
	if err := set(&connection{}, `{"ask":{"habits":false}}`); err != nil {
		t.Fatalf("another client changing Ask: %v", err)
	}
	if gateway.read != 1 {
		t.Fatal("a patch that kept the addresses announced the providers")
	}
}
