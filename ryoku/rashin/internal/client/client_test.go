package client

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func isolateClient(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
	return root
}

func TestVerifySocketDirRequiresPrivateOwnedDirectory(t *testing.T) {
	root := isolateClient(t)
	dir := filepath.Join(root, "socket")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := verifySocketDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := verifySocketDir(dir); err == nil {
		t.Fatal("public socket directory was accepted")
	}
}

func TestSpawnDaemonStartsInHome(t *testing.T) {
	root := isolateClient(t)
	result := filepath.Join(root, "cwd")
	script := filepath.Join(root, "rashind-test")
	body := "#!/bin/sh\npwd > \"$RASHIN_TEST_CWD\"\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RASHIN_DAEMON", script)
	t.Setenv("RASHIN_TEST_CWD", result)
	if err := spawnDaemon(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(result)
		if err == nil {
			if got := string(content); got != root+"\n" {
				t.Fatalf("daemon cwd = %q", got)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("daemon test process did not write its cwd")
}
