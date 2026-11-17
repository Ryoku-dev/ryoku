package main

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

// A Prowl MCP server rebuilds a stale index before it answers initialize, and
// a session/new that waits on it dies at acpRequestTimeout. The abandoned
// server takes the half-done rebuild with it, so the next try starts slower.
// The rebuild runs here instead, in a process the session never owns, and
// the server is attached only once the index is current.
var (
	prowlIndexReadyFor   = 5 * time.Minute
	prowlIndexWait       = 8 * time.Second
	prowlIndexRefreshMax = 15 * time.Minute

	// prowlIndexRefresh brings the repo's index up to date; tests replace it.
	prowlIndexRefresh = func(bin, repo string) error {
		ctx, cancel := context.WithTimeout(context.Background(), prowlIndexRefreshMax)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, "status", "--json")
		cmd.Dir = repo
		return cmd.Run()
	}
)

var prowlIndex struct {
	mu      sync.Mutex
	repo    string
	readyAt time.Time
	done    chan struct{}
}

// prowlSessionMCPServers is prowlMCPServers for a hermes session: the server
// is offered only when the index is current, waiting briefly for a refresh
// that is almost done. A session that cannot wait opens without it, as it
// does when no checkout is indexed, and the refresh carries on for the next.
func prowlSessionMCPServers() []any {
	bin, ok := findProwl()
	repo := prowlRepo()
	if !ok || repo == "" {
		return []any{}
	}
	if !prowlIndexCurrent(bin, repo, prowlIndexWait) {
		return []any{}
	}
	return prowlMCPServers()
}

func prowlIndexCurrent(bin, repo string, wait time.Duration) bool {
	prowlIndex.mu.Lock()
	if prowlIndex.repo == repo && time.Since(prowlIndex.readyAt) < prowlIndexReadyFor {
		prowlIndex.mu.Unlock()
		return true
	}
	done := prowlIndex.done
	if done == nil {
		done = make(chan struct{})
		prowlIndex.done = done
		go func() {
			err := prowlIndexRefresh(bin, repo)
			prowlIndex.mu.Lock()
			if err == nil {
				prowlIndex.repo, prowlIndex.readyAt = repo, time.Now()
			}
			prowlIndex.done = nil
			prowlIndex.mu.Unlock()
			close(done)
		}()
	}
	prowlIndex.mu.Unlock()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		return false
	}
	prowlIndex.mu.Lock()
	defer prowlIndex.mu.Unlock()
	return prowlIndex.repo == repo && time.Since(prowlIndex.readyAt) < prowlIndexReadyFor
}
