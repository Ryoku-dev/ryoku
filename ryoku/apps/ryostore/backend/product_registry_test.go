package main

import (
	"encoding/json"
	"testing"
)

// The registry now carries each product's provenance links. They are pass-through
// only: the JSON tags must decode a registry entry, and productEntryItem must copy
// them onto the item the app renders. A missing discord is the common case (only
// upstream is required), so it must land as an empty string, not leak a stale one.
func TestProductEntryItemCarriesProvenanceLinks(t *testing.T) {
	raw := []byte(`[
		{"id":"linked","upstream":"https://github.com/ryoku-dev/orbit","discord":"https://discord.gg/ryoku"},
		{"id":"lone","upstream":"https://github.com/ryoku-dev/lone"}
	]`)
	var entries []ProductEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("decode registry entries: %v", err)
	}

	linked, err := productEntryItem("", "barstyles", entries[0])
	if err != nil {
		t.Fatalf("productEntryItem: %v", err)
	}
	if linked.Upstream != "https://github.com/ryoku-dev/orbit" {
		t.Errorf("upstream = %q, want the entry's own url", linked.Upstream)
	}
	if linked.Discord != "https://discord.gg/ryoku" {
		t.Errorf("discord = %q, want the entry's own invite", linked.Discord)
	}

	lone, err := productEntryItem("", "barstyles", entries[1])
	if err != nil {
		t.Fatalf("productEntryItem: %v", err)
	}
	if lone.Upstream != "https://github.com/ryoku-dev/lone" {
		t.Errorf("upstream = %q, want the entry's own url", lone.Upstream)
	}
	if lone.Discord != "" {
		t.Errorf("discord = %q, want empty when the entry names none", lone.Discord)
	}
}
