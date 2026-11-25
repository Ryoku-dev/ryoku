package config

import (
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestDecisionSettingsCannotRelabelLANOrPublicTrafficAsLocal(t *testing.T) {
	cur := Defaults()
	for _, address := range []string{"http://192.168.1.2:11435/v1", "http://10.0.0.2/v1", "https://remote.example/v1", "http://user:key@127.0.0.1/v1", "http://127.0.0.1/v1?forward=remote"} {
		patch, _ := json.Marshal(map[string]any{"providers": map[string]any{"decisions": map[string]any{"route": "local", "localAddress": address, "localModel": "fixture"}}})
		if _, err := Merge(cur, patch); err == nil {
			t.Fatalf("a local-labelled decision route admitted %s", address)
		}
	}
	loaded := fromRaw(rawSettings(t, `{"version":2,"providers":{"decisions":{"route":"local","localAddress":"https://remote.example/v1","localModel":"fixture"}}}`))
	if loaded.Providers.Decisions.LocalModel != "" || loaded.Providers.Decisions.Route != protocol.DecisionRules {
		t.Fatalf("invalid stored local route remained usable: %#v", loaded.Providers.Decisions)
	}
}

func TestDecisionRoutingConsentAndFeaturePinsSurviveSaveAndReopen(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"providers":{"decisions":{"route":"auto","localAddress":"http://localhost:11435/v1","localModel":"fixture:local","allowHosted":false,"features":{"oni.point":"local"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(next); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	decision := loaded.Providers.Decisions
	if decision.Route != protocol.DecisionAuto || decision.AllowHosted || decision.LocalAddress != "http://127.0.0.1:11435/v1" || decision.Features["oni.point"] != protocol.DecisionLocal {
		t.Fatalf("saved routing or consent changed: %#v", decision)
	}
	if _, err := Merge(loaded, json.RawMessage(`{"providers":{"decisions":{"features":{"oni.point":"unknown-paid-route"}}}}`)); err == nil {
		t.Fatal("an unrecognized feature route was admitted")
	}
}
