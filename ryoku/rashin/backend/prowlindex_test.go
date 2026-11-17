package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func stubIndexRefresh(t *testing.T, fn func(bin, repo string) error) {
	t.Helper()
	oldRefresh, oldReadyFor := prowlIndexRefresh, prowlIndexReadyFor
	prowlIndexRefresh = fn
	t.Cleanup(func() {
		prowlIndexRefresh, prowlIndexReadyFor = oldRefresh, oldReadyFor
		prowlIndex.mu.Lock()
		prowlIndex.repo, prowlIndex.readyAt = "", time.Time{}
		prowlIndex.mu.Unlock()
	})
	prowlIndex.mu.Lock()
	prowlIndex.repo, prowlIndex.readyAt = "", time.Time{}
	prowlIndex.mu.Unlock()
}

func TestSessionWaitsForAnIndexRefreshButNeverAbandonsIt(t *testing.T) {
	release := make(chan struct{})
	var runs atomic.Int32
	stubIndexRefresh(t, func(bin, repo string) error {
		runs.Add(1)
		<-release
		return nil
	})

	if prowlIndexCurrent("prowl", "/repo", 20*time.Millisecond) {
		t.Fatal("a session got the MCP server while the index was still rebuilding")
	}
	if prowlIndexCurrent("prowl", "/repo", 20*time.Millisecond) {
		t.Fatal("second session got the MCP server while the index was still rebuilding")
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("refresh ran %d times, want the one rebuild shared", got)
	}

	close(release)
	if !prowlIndexCurrent("prowl", "/repo", time.Second) {
		t.Fatal("the finished rebuild did not make the next session eligible")
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("a current index was refreshed again (%d runs)", got)
	}
}

func TestSessionDoesNotTrustAFailedRefresh(t *testing.T) {
	stubIndexRefresh(t, func(bin, repo string) error { return errors.New("database is locked") })
	if prowlIndexCurrent("prowl", "/repo", time.Second) {
		t.Fatal("a failed refresh was treated as a current index")
	}
}

func TestSessionRefreshesAgainOnceTheIndexIsOld(t *testing.T) {
	var runs atomic.Int32
	stubIndexRefresh(t, func(bin, repo string) error { runs.Add(1); return nil })
	prowlIndexReadyFor = 10 * time.Millisecond

	if !prowlIndexCurrent("prowl", "/repo", time.Second) {
		t.Fatal("fresh index was not offered")
	}
	time.Sleep(30 * time.Millisecond)
	if !prowlIndexCurrent("prowl", "/repo", time.Second) || runs.Load() != 2 {
		t.Fatalf("an old index was not refreshed again (runs=%d)", runs.Load())
	}
}

func TestSessionMCPServerOnlyOnceTheIndexIsCurrent(t *testing.T) {
	homeDir := isolateProwlGatewayTest(t)
	bin := filepath.Join(homeDir, "prowl")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(homeDir, "checkout")
	if err := os.MkdirAll(filepath.Join(checkout, ".prowl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_PROWL_BIN", bin)
	t.Setenv("RYOKU_RASHIN_REPO", checkout)

	release := make(chan struct{})
	stubIndexRefresh(t, func(bin, repo string) error { <-release; return nil })
	oldWait := prowlIndexWait
	prowlIndexWait = 20 * time.Millisecond
	t.Cleanup(func() { prowlIndexWait = oldWait })

	if got := prowlSessionMCPServers(); len(got) != 0 {
		t.Fatalf("session attached %d MCP servers during a rebuild", len(got))
	}
	close(release)
	prowlIndexWait = time.Second
	if got := prowlSessionMCPServers(); len(got) != 1 {
		t.Fatalf("session got %d MCP servers after the rebuild, want prowl", len(got))
	}
}
