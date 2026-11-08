package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIdleGatewayStopsOnlyWhenNothingNeedsIt(t *testing.T) {
	// the pair sets its own config dir, so it comes before the one this test writes
	conn, _ := newTestPair(t)
	home := quickTestEnv(t)
	quickGateway(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	bin := filepath.Join(home, "prowl")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_PROWL_BIN", bin)
	prevSpawn, prevSystemd, prevNow := gatewaySpawn, gatewaySystemdPresent, gatewayNow
	t.Cleanup(func() { gatewaySpawn, gatewaySystemdPresent, gatewayNow = prevSpawn, prevSystemd, prevNow })
	gatewaySystemdPresent = func() bool { return false }
	stops := 0
	gatewaySpawn = func(name string, args ...string) error {
		if name == bin && len(args) > 1 && args[0] == "gateway" && args[1] == "down" {
			stops++
		}
		return nil
	}
	now := time.Now()
	gatewayNow = func() time.Time { return now }
	noteGatewayUse()

	retireIdleGateway(false)
	if stops != 0 {
		t.Fatal("a gateway used moments ago was stopped")
	}

	now = now.Add(gatewayIdle + time.Minute)
	if err := updateConfig(func(c *Config) error { c.KeepProwl = true; return nil }); err != nil {
		t.Fatal(err)
	}
	retireIdleGateway(false)
	if stops != 0 {
		t.Fatal("the gateway was stopped while the user keeps it on")
	}

	if err := updateConfig(func(c *Config) error {
		c.KeepProwl = false
		c.ProwlHarnesses = []string{"hermes"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h := newChatHub(ryokuLane())
	conn.mu.Lock()
	conn.agentID = "hermes"
	conn.mu.Unlock()
	h.mu.Lock()
	h.conn = conn
	h.mu.Unlock()
	retireIdleGateway(false)
	if stops != 0 {
		t.Fatal("the gateway was stopped under a live agent that reaches its model through Prowl")
	}

	h.resetConn()
	retireIdleGateway(false)
	if stops != 1 {
		t.Fatalf("an idle gateway nothing needs was stopped %d times, want once", stops)
	}
}
