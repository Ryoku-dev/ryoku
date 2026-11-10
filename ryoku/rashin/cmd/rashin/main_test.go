package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestBareWordAskDetection(t *testing.T) {
	for _, args := range [][]string{{"why", "is", "wifi", "slow"}, {"hello"}} {
		if !isBareAsk(args) {
			t.Fatalf("%q was not a bare ask", args)
		}
	}
	for _, args := range [][]string{{}, {"--bad"}, {"status"}, {"ask", "hello"}} {
		if isBareAsk(args) {
			t.Fatalf("%q was treated as a bare ask", args)
		}
	}
}

func TestIsCommandHelperUsesRegistryWithoutDispatching(t *testing.T) {
	for _, command := range sortedCommands() {
		if code := run([]string{"__is-command", command.Name}); code != 0 {
			t.Fatalf("registered command %q returned %d", command.Name, code)
		}
	}
	for _, flag := range []string{"-h", "--help", "--version", "--future-flag"} {
		if code := run([]string{"__is-command", flag}); code != 0 {
			t.Fatalf("flag %q returned %d", flag, code)
		}
	}
	for _, args := range [][]string{{"__is-command", "hello"}, {"__is-command"}, {"__is-command", "status", "extra"}} {
		if code := run(args); code != 1 {
			t.Fatalf("%q returned %d, want 1", args, code)
		}
	}
}

func TestParseAskArgsAcceptsOptionTerminator(t *testing.T) {
	options, err := parseAskArgs([]string{"--buffer", "--", "git log --oneline | wc -l"})
	if err != nil {
		t.Fatal(err)
	}
	if !options.buffer || !strings.EqualFold(strings.Join(options.words, " "), "git log --oneline | wc -l") {
		t.Fatalf("options = %#v", options)
	}
}

func TestPrintModelsDoesNotDuplicateProvider(t *testing.T) {
	var output bytes.Buffer
	printModels(&output, protocol.ModelsResult{Models: []protocol.ModelInfo{{ID: "ovh/model", Provider: "ovh", Available: true}}})
	if got := output.String(); got != "ovh/model\tavailable\n" {
		t.Fatalf("models output = %q", got)
	}
}

func TestAbsoluteUserPathResolvesAgainstCallerDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	got, err := absoluteUserPath(filepath.Join(".", "project"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "project"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestSessionTableTitleIsOneLineAndBounded(t *testing.T) {
	title := "Imported\n  \u250c\u2500\u2500 history\twith " + strings.Repeat("long ", 20)
	got := sessionTableTitle(title)
	if strings.ContainsAny(got, "\n\r\t") {
		t.Fatalf("title is not one line: %q", got)
	}
	if len([]rune(got)) > 60 || !strings.HasSuffix(got, "...") {
		t.Fatalf("title is not bounded: %q", got)
	}
}

func TestParseOpenSupportsProjectViewsAndWorktree(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	params, err := parseOpen([]string{
		"--session", "ask-1",
		"--tab", "agent",
		"--project", "./project",
		"--view", "changes",
		"--worktree", "./project/worktree",
	})
	if err != nil {
		t.Fatal(err)
	}
	if params.Session != "ask-1" || params.Tab != "agent" || params.Project != "./project" || params.View != "changes" {
		t.Fatalf("params = %#v", params)
	}
	if want := filepath.Join(root, "project", "worktree"); params.Worktree != want {
		t.Fatalf("worktree = %q, want %q", params.Worktree, want)
	}
	if _, err := parseOpen([]string{"--view", "unknown"}); err == nil {
		t.Fatal("invalid view was accepted")
	}
}
