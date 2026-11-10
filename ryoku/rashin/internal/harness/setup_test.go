package harness

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// omp keeps a model per role in its own settings file. Rashin reads them so
// the setup shows before omp ever runs.
func TestOmpSetupReadsTheModelBehindEachRole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	config := strings.Join([]string{
		"setupVersion: 2",
		"modelRoles:",
		"  smol: openai-codex/gpt-5.6-luna:medium",
		"  task: \"openai-codex/gpt-5.6-sol:high\"",
		"  default: anthropic/claude-opus-5-5  # the main one",
		"defaultThinkingLevel: auto",
		"providers:",
		"  anthropic:",
		"    slowMode: auto",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []protocol.HarnessFact{
		{Label: "Model", Value: "anthropic/claude-opus-5-5"},
		{Label: "smol model", Value: "openai-codex/gpt-5.6-luna:medium"},
		{Label: "task model", Value: "openai-codex/gpt-5.6-sol:high"},
		{Label: "Thinking", Value: "auto"},
	}
	if got := ompSetup(path); !reflect.DeepEqual(got, want) {
		t.Fatalf("the setup reads %#v", got)
	}
	if got := ompSetup(filepath.Join(t.TempDir(), "absent.yml")); got != nil {
		t.Fatalf("a missing settings file reads %#v", got)
	}
}
