package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func gameModeSandbox(t *testing.T, settings string) (stateFile string) {
	t.Helper()
	cfg, state := t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("RYOKU_STATE_PATH", "")
	t.Setenv("RYOKU_GAMEMODE_STATE_FILE", "")
	if err := os.MkdirAll(filepath.Join(cfg, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "ryoku", "ryogami.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	pauseState.mu.Lock()
	pauseState.active, pauseState.applied, pauseState.manual, pauseState.resumeTimer = false, false, nil, nil
	pauseState.mu.Unlock()
	t.Cleanup(func() {
		pauseState.mu.Lock()
		if pauseState.resumeTimer != nil {
			pauseState.resumeTimer.Stop()
		}
		pauseState.active, pauseState.applied, pauseState.manual, pauseState.resumeTimer = false, false, nil, nil
		pauseState.mu.Unlock()
	})
	return filepath.Join(state, "ryoku", "game-mode.enabled")
}

func setGameMode(t *testing.T, stateFile string, on bool) {
	t.Helper()
	if on {
		if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stateFile, []byte("enabled\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	_ = os.Remove(stateFile)
}

func applied() bool {
	pauseState.mu.Lock()
	defer pauseState.mu.Unlock()
	return pauseState.applied
}

func waitApplied(t *testing.T, want bool, what string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if applied() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s: playback paused = %v, want %v", what, applied(), want)
}

func TestGameModePausesAndResumesWallpaper(t *testing.T) {
	stateFile := gameModeSandbox(t, `{"playback":{"resumeDelay":0}}`)
	d := &daemon{events: newEventHub()}

	d.reevaluatePause()
	if applied() {
		t.Fatal("wallpaper paused with Game Mode off")
	}

	setGameMode(t, stateFile, true)
	d.reevaluatePause()
	waitApplied(t, true, "Game Mode on")

	setGameMode(t, stateFile, false)
	d.reevaluatePause()
	waitApplied(t, false, "Game Mode off")
}

func TestGameModePauseIsAChoice(t *testing.T) {
	stateFile := gameModeSandbox(t, `{"playback":{"gameMode":false,"resumeDelay":0}}`)
	d := &daemon{events: newEventHub()}

	setGameMode(t, stateFile, true)
	d.reevaluatePause()
	if applied() {
		t.Fatal("Game Mode paused the wallpaper although the setting is off")
	}
}

func TestGameModeResumeKeepsManualPause(t *testing.T) {
	stateFile := gameModeSandbox(t, `{"playback":{"resumeDelay":0}}`)
	d := &daemon{events: newEventHub()}

	d.setManualPause(nil, true)
	setGameMode(t, stateFile, true)
	d.reevaluatePause()
	setGameMode(t, stateFile, false)
	d.reevaluatePause()
	time.Sleep(100 * time.Millisecond)
	if !applied() {
		t.Fatal("ending Game Mode resumed a wallpaper the user paused by hand")
	}

	d.setManualPause(nil, false)
	waitApplied(t, false, "manual resume")
}

func TestGameModeResumeKeepsOtherRules(t *testing.T) {
	r := pauseRules{gameMode: false, fullscreen: true}
	if p, rs := evaluatePause(r, baseSnapshot(), nil); !p || !hasReason(rs, "fullscreen") || hasReason(rs, "gameMode") {
		t.Fatalf("a fullscreen window must keep the wallpaper paused once Game Mode ends: %v %v", p, rs)
	}
	r.gameMode = true
	if p, rs := evaluatePause(r, wmSnapshot{}, nil); !p || !hasReason(rs, "gameMode") {
		t.Fatalf("Game Mode alone must pause: %v %v", p, rs)
	}
}

func TestE2EPlaybackSyncFollowsGameMode(t *testing.T) {
	d := startDaemon(t)
	stateFile := filepath.Join(d.root, ".local", "state", "ryoku", "game-mode.enabled")

	sub := d.connect()
	defer sub.Close()
	fmt.Fprintln(sub, `{"method":"subscribe","params":{"prefixes":["ryogami.playback."]},"id":1}`)
	r := bufio.NewReader(sub)
	readJSONLine(t, r)

	status := func(want bool) []interface{} {
		t.Helper()
		_ = sub.SetDeadline(time.Now().Add(8 * time.Second))
		for {
			ev := readJSONLine(t, r)
			if ev["event"] != "ryogami.playback.status" {
				continue
			}
			data := ev["data"].(map[string]interface{})
			if data["paused"] == want {
				return data["reasons"].([]interface{})
			}
		}
	}

	setGameMode(t, stateFile, true)
	if got := d.send("wallpaper playback-sync"); got != "ok" {
		t.Fatalf("playback-sync reply: %s", got)
	}
	if reasons := status(true); len(reasons) != 1 || reasons[0] != "gameMode" {
		t.Fatalf("pause reasons = %v, want [gameMode]", reasons)
	}

	setGameMode(t, stateFile, false)
	if got := d.send("wallpaper playback-sync"); got != "ok" {
		t.Fatalf("playback-sync reply: %s", got)
	}
	status(false)
}
