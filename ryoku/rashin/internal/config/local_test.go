package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// A local provider's address is one the gateway will call, so only this
// computer and the local network are accepted.
func TestLocalAddressesStayOnThisComputerOrTheNetwork(t *testing.T) {
	cur := Defaults()
	for patch, want := range map[string]string{
		`{"providers":{"local":{"ollama-local":"http://192.168.1.20:11434"}}}`:  "http://192.168.1.20:11434/v1",
		`{"providers":{"local":{"ollama-local":"http://localhost:11434/v1/"}}}`: "http://localhost:11434/v1",
		`{"providers":{"local":{"ollama-local":"https://[fd00::7]:8443/api"}}}`: "https://[fd00::7]:8443/api",
		`{"providers":{"local":{"ollama-local":null}}}`:                         "http://127.0.0.1:11434/v1",
	} {
		next, err := Merge(cur, json.RawMessage(patch))
		if err != nil {
			t.Fatalf("%s: %v", patch, err)
		}
		if got := next.Providers.Local[protocol.ProviderOllamaLocal]; got != want {
			t.Fatalf("%s: address = %q, want %q", patch, got, want)
		}
		if got := next.Providers.Local[protocol.ProviderLlamaCpp]; got != "http://127.0.0.1:8080/v1" {
			t.Fatalf("%s: llama.cpp address = %q", patch, got)
		}
	}
	for _, address := range []string{
		"http://203.0.113.9:11434/v1", "http://ollama.example.com/v1", "http://169.254.169.254/v1",
		"ftp://127.0.0.1/v1", "http://user:secret@127.0.0.1:11434/v1", "http://127.0.0.1:11434/v1?x=1",
		"http://[2001:db8::1]:11434/v1", "",
	} {
		patch, _ := json.Marshal(map[string]any{"providers": map[string]any{"local": map[string]string{protocol.ProviderLlamaCpp: address}}})
		if _, err := Merge(cur, patch); err == nil {
			t.Fatalf("address %q was accepted", address)
		}
	}
	if _, err := Merge(cur, json.RawMessage(`{"providers":{"local":{"vllm":"http://127.0.0.1:8000/v1"}}}`)); err == nil {
		t.Fatal("an unknown local provider was accepted")
	}
}

// A hand-edited file cannot point the gateway off the network either.
func TestLoadFallsBackToTheDefaultLocalAddress(t *testing.T) {
	s := fromRaw(rawSettings(t, `{"version":2,"providers":{"local":{"llamacpp":"http://203.0.113.9:8080/v1","ollama-local":"http://10.0.0.5:11434/v1","other":"http://127.0.0.1/v1"}}}`))
	want := map[string]string{protocol.ProviderOllamaLocal: "http://10.0.0.5:11434/v1", protocol.ProviderLlamaCpp: "http://127.0.0.1:8080/v1"}
	if len(s.Providers.Local) != len(want) {
		t.Fatalf("local = %#v", s.Providers.Local)
	}
	for id, address := range want {
		if s.Providers.Local[id] != address {
			t.Fatalf("local = %#v", s.Providers.Local)
		}
	}
}

// Rashin's own llama.cpp is never configured to stay up forever, never runs a
// program that is not there, and light work only goes to a routable model.
func TestRashinModelsSettings(t *testing.T) {
	if got := Defaults().Providers.Models.IdleMinutes; got != protocol.DefaultLocalIdleMinutes {
		t.Fatalf("default idle minutes = %d", got)
	}
	s := fromRaw(rawSettings(t, `{"version":2,"providers":{"models":{"idleMinutes":9999,"binary":"bin/llama-server","light":"tiny"}}}`))
	if m := s.Providers.Models; m.IdleMinutes != protocol.LocalIdleMinutesMax || m.Binary != "" || m.Light != "" {
		t.Fatalf("models = %#v", m)
	}
	binary := filepath.Join(t.TempDir(), "llama-server")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	patch, _ := json.Marshal(map[string]any{"providers": map[string]any{"models": map[string]any{"binary": binary, "light": "rashin-models/lfm3-omni-2b", "remoteFirst": true}}})
	next, err := Merge(Defaults(), patch)
	if err != nil {
		t.Fatal(err)
	}
	if m := next.Providers.Models; m.Binary != binary || m.Light != "rashin-models/lfm3-omni-2b" || !m.RemoteFirst {
		t.Fatalf("models = %#v", m)
	}
	for _, bad := range []map[string]any{
		{"binary": filepath.Join(t.TempDir(), "missing")},
		{"binary": "llama-server"},
		{"light": "tiny"},
	} {
		patch, _ := json.Marshal(map[string]any{"providers": map[string]any{"models": bad}})
		if _, err := Merge(Defaults(), patch); err == nil {
			t.Fatalf("%v was accepted", bad)
		}
	}
	if _, err := Merge(Defaults(), json.RawMessage(`{"providers":{"local":{"rashin-models":"http://127.0.0.1:8080/v1"}}}`)); err == nil {
		t.Fatal("rashin-models was given an address")
	}
}

func TestModelOverrideValidationAndAutomaticReset(t *testing.T) {
	base := Defaults()
	patch := json.RawMessage(`{"providers":{"models":{"overrides":{"lfm3-omni-2b":{"device":"Vulkan0","context":16384,"kvType":"q8_0"}}}}}`)
	next, err := Merge(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Providers.Models.Overrides["lfm3-omni-2b"]; got.Context != 16384 || got.KVType != protocol.LocalKVQ8 {
		t.Fatalf("override = %+v", got)
	}
	reset, err := Merge(next, json.RawMessage(`{"providers":{"models":{"overrides":{"lfm3-omni-2b":null}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reset.Providers.Models.Overrides["lfm3-omni-2b"]; ok {
		t.Fatal("automatic reset retained override")
	}
	for _, bad := range []string{
		`{"providers":{"models":{"overrides":{"lfm3-omni-2b":{"context":131072}}}}}`,
		`{"providers":{"models":{"overrides":{"lfm3-omni-2b":{"kvType":"q4_0"}}}}}`,
		`{"providers":{"models":{"overrides":{"../outside":{"device":"cpu"}}}}}`,
		`{"providers":{"models":{"overrides":{"lfm3-omni-2b":{"device":"Vulkan0\nkey = x"}}}}}`,
	} {
		if _, err := Merge(base, json.RawMessage(bad)); err == nil {
			t.Fatalf("accepted unsafe override %s", bad)
		}
	}
}
