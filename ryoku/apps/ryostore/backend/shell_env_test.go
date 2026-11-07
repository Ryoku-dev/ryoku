package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidateShellEnv(t *testing.T) {
	valid := []map[string]string{
		nil,
		{},
		{"OMARCHY_PATH": "$XDG_DATA_HOME/ryoku/nomarchy"},
		{"ITEM_CACHE": "$XDG_CACHE_HOME/item", "ITEM_HOME": "$RYOKU_ITEM_DIR/data:$HOME/x", "PLAIN": "no dollars"},
	}
	for _, env := range valid {
		if err := validateShellEnv(env); err != nil {
			t.Fatalf("validateShellEnv(%v) = %v", env, err)
		}
	}
	invalid := map[string]map[string]string{
		"lower-case key":  {"omarchy_path": "x"},
		"leading digit":   {"1PATH": "x"},
		"PATH":            {"PATH": "/tmp"},
		"LC_ prefix":      {"LC_ALL": "C"},
		"XDG_ prefix":     {"XDG_DATA_HOME": "/tmp"},
		"QML prefix":      {"QML2_IMPORT_PATH": "/tmp"},
		"RYOKU_ prefix":   {"RYOKU_WM": "niri"},
		"DISPLAY":         {"DISPLAY": ":1"},
		"other variable":  {"ITEM": "$USER/x"},
		"braced variable": {"ITEM": "${HOME}/x"},
		"longer name":     {"ITEM": "$HOMEDIR"},
		"bare dollar":     {"ITEM": "costs $5"},
		"trailing dollar": {"ITEM": "x$"},
		"NUL byte":        {"ITEM": "a\x00b"},
	}
	for name, env := range invalid {
		if err := validateShellEnv(env); err == nil {
			t.Fatalf("%s: validateShellEnv(%v) accepted", name, env)
		}
	}
}

func TestShellsIndexCarriesEnvAndDropsInvalidEnv(t *testing.T) {
	f := newShellFixture(t)
	f.addShipped("orpheus", shippedTOML("orpheus")+`
[shell.env]
ORPHEUS_DATA = "$XDG_STATE_HOME/orpheus"
`)
	f.addShipped("kairos", shippedTOML("kairos"))
	good := testShellManifest()
	good.Env = map[string]string{"OMARCHY_PATH": "$XDG_DATA_HOME/ryoku/nomarchy"}
	installTestShell(t, f, "barstyles", "nomarchy", good)
	bad := testShellManifest()
	bad.Env = map[string]string{"PATH": "/tmp/evil"}
	installTestShell(t, f, "barstyles", "sneaky", bad)

	result, err := runShellsSync(false)
	if err != nil {
		t.Fatal(err)
	}
	index := f.readIndex()
	if env := indexItem(t, index, "subshells", "orpheus").Env; !reflect.DeepEqual(env, map[string]string{"ORPHEUS_DATA": "$XDG_STATE_HOME/orpheus"}) {
		t.Fatalf("shipped env = %v", env)
	}
	if env := indexItem(t, index, "subshells", "kairos").Env; env == nil || len(env) != 0 {
		t.Fatalf("absent env = %#v, want {}", env)
	}
	if env := indexItem(t, index, "barstyles", "nomarchy").Env; !reflect.DeepEqual(env, map[string]string{"OMARCHY_PATH": "$XDG_DATA_HOME/ryoku/nomarchy"}) {
		t.Fatalf("store env = %v, want it unexpanded", env)
	}
	sneaky := indexItem(t, index, "barstyles", "sneaky")
	if sneaky.Env == nil || len(sneaky.Env) != 0 {
		t.Fatalf("invalid env kept: %#v", sneaky.Env)
	}
	reported := false
	for _, problem := range result.Problems {
		if strings.Contains(problem, "barstyles/sneaky") && strings.Contains(problem, "PATH") {
			reported = true
		}
	}
	if !reported {
		t.Fatalf("problems = %v, want the invalid env reported", result.Problems)
	}
	if receipt, err := readReceipt("barstyles", "sneaky"); err != nil || receipt.Shell.Env["PATH"] != "/tmp/evil" {
		t.Fatalf("the receipt should keep what the catalogue shipped: %#v, %v", receipt.Shell, err)
	}
}
