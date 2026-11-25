package exec

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestReleaseKillsRedirectedBackgroundProcess(t *testing.T) {
	for _, action := range []string{"release", "kill", "close"} {
		t.Run(action, func(t *testing.T) {
			h := Host(HostOptions{})
			defer h.Close()
			ctx := context.Background()
			id, err := h.Start(ctx, StartRequest{Argv: []string{"sh", "-c", "sleep 60 >log 2>&1 & echo $!"}, Dir: t.TempDir()}, nil)
			if err != nil {
				t.Fatal(err)
			}
			exit, err := h.Wait(ctx, id)
			if err != nil || exit.Running {
				t.Fatalf("shell exit = %+v, %v", exit, err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(exit.Output))
			if err != nil {
				t.Fatalf("background PID in %q: %v", exit.Output, err)
			}
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err != nil {
				t.Fatalf("background child already gone before cleanup: %v", err)
			}
			switch action {
			case "release":
				err = h.Release(ctx, id)
			case "kill":
				err = h.Kill(ctx, id)
			case "close":
				err = h.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				state, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
				if os.IsNotExist(err) || err == nil && strings.HasPrefix(strings.TrimSpace(strings.SplitN(string(state), ") ", 2)[1]), "Z") {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatalf("background child %d survived %s", pid, action)
		})
	}
}
