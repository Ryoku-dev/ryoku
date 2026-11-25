package gateway

import (
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func TestCooldownSurvivesGatewayRestart(t *testing.T) {
	stateDir := t.TempDir()
	settings := func() protocol.Settings { return protocol.Settings{} }
	first, err := Open(stateDir, settings)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	until := now.Add(24 * time.Hour)
	if err := first.store.setCooldown(modelScope("provider", "model"), until, "model unavailable"); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(stateDir, settings)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	got, ok, err := second.store.cooldown(modelScope("provider", "model"), now)
	if err != nil || !ok || !got.Equal(until) {
		t.Fatalf("cooldown = %s, %v, %v", got, ok, err)
	}
}
