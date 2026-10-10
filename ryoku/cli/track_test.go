package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTrackArgs(t *testing.T) {
	cases := []struct {
		args    []string
		channel string
		source  bool
		wantErr bool
	}{
		{[]string{"stable"}, "stable", false, false},
		{[]string{"unstable"}, "unstable", false, false},
		{[]string{"stable", "--source"}, "stable", true, false},
		{[]string{"--source", "unstable"}, "unstable", true, false},
		{[]string{"v0.55.7-beta.19"}, "v0.55.7-beta.19", false, false},
		{[]string{"v0.94.1-beta.20.dev.12+g1234567"}, "v0.94.1-beta.20.dev.12+g1234567", false, false},
		{[]string{}, "", false, true},
		{[]string{"stable", "extra"}, "", false, true},
		{[]string{"--bogus", "stable"}, "", false, true},
	}
	for _, c := range cases {
		ch, src, err := parseTrackArgs(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("%v: err = %v, wantErr %v", c.args, err, c.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if ch != c.channel || src != c.source {
			t.Errorf("%v: got (%q, %v), want (%q, %v)", c.args, ch, src, c.channel, c.source)
		}
	}
}

// unstable selects the testing package channel; stable and frozen versions pass
// through; testing is a quiet synonym; the retired main/unstable-dev names name
// no package channel (cmdTrack reports their replacements first).
func TestPackageChannelForAliases(t *testing.T) {
	cases := map[string]string{
		"unstable":                        "testing",
		"stable":                          "stable",
		"testing":                         "testing",
		"v0.55.7-beta.19":                 "v0.55.7-beta.19",
		"v0.94.1-beta.20.dev.12+g1234567": "v0.94.1-beta.20.dev.12+g1234567",
		"v0.94.1-beta.20.dev.12_g1234567": "",
		"main":                            "",
		"unstable-dev":                    "",
		"bogus":                           "",
		"":                                "",
	}
	for in, want := range cases {
		if got := packageChannelFor(in); got != want {
			t.Errorf("packageChannelFor(%q) = %q, want %q", in, got, want)
		}
	}
}

// The retired track arguments fail with the replacement named, with or without
// --source, and never touch the system.
func TestCmdTrackRetiresOldNames(t *testing.T) {
	repl := map[string]string{"main": "stable", "unstable-dev": "unstable"}
	for name, want := range repl {
		for _, args := range [][]string{{name}, {name, "--source"}} {
			err := cmdTrack(args)
			if err == nil {
				t.Fatalf("cmdTrack(%v) succeeded, want a retirement error", args)
			}
			if !strings.Contains(err.Error(), "ryoku track "+want) {
				t.Errorf("cmdTrack(%v) error %q does not name `ryoku track %s`", args, err, want)
			}
		}
	}
}

// `ryoku track <channel> --source` (either order) translates the channel to its
// git branch (stable -> main, unstable -> unstable-dev) and hands the branch to
// the local bin/ryoku-track.
func TestTrackSourceMapsChannelToBranch(t *testing.T) {
	cases := map[string]string{"stable": "main", "unstable": "unstable-dev"}
	for channel, branch := range cases {
		t.Run(channel, func(t *testing.T) {
			repo := t.TempDir()
			initGit := exec.Command("git", "-C", repo, "init")
			initGit.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
			if out, err := initGit.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			marker := filepath.Join(repo, "ran")
			script := filepath.Join(repo, "bin", "ryoku-track")
			if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
				t.Fatal(err)
			}
			body := "#!/bin/sh\nprintf '%s' \"$1\" > " + shellQuote(marker) + "\n"
			if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("RYOKU_REPO", repo)

			if err := cmdTrack([]string{"--source", channel}); err != nil {
				t.Fatalf("cmdTrack --source %s: %v", channel, err)
			}
			got, err := os.ReadFile(marker)
			if err != nil {
				t.Fatalf("local track script did not run: %v", err)
			}
			if string(got) != branch {
				t.Errorf("track %s --source ran the script with %q, want the branch %q", channel, got, branch)
			}
		})
	}
}
