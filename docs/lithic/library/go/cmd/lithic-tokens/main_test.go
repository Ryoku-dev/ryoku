package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var outputNames = []string{
	"library/qml/Lithic/LithicTokens.qml",
	"library/web/tokens.css",
	"library/web/tokens.js",
	"library/go/tokens/tokens.go",
}

func TestRunningTwiceIsByteIdentical(t *testing.T) {
	root := tempRoot(t)
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	first := readOutputs(t, root)
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	second := readOutputs(t, root)
	for _, name := range outputNames {
		if !bytes.Equal(first[name], second[name]) {
			t.Fatalf("%s changed between identical runs", name)
		}
	}
}

func TestCheckReportsTokenDrift(t *testing.T) {
	root := tempRoot(t)
	if err := run(root, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "tokens.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(data, []byte(`"s1": 4`), []byte(`"s1": 5`), 1)
	if bytes.Equal(data, changed) {
		t.Fatal("space.s1 fixture was not found")
	}
	if err := os.WriteFile(path, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	err = run(root, true)
	var stale *staleError
	if !errors.As(err, &stale) {
		t.Fatalf("-check error = %v, want staleError", err)
	}
	for _, name := range outputNames {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("-check did not name %s: %v", name, err)
		}
	}
}

func TestRequiredContractKeys(t *testing.T) {
	tokens, err := readTokens(filepath.Join(sourceRoot(), "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		"space.s1", "space.s2", "space.s3", "space.s4", "space.s5", "space.s6",
		"cut.token", "cut.small", "cut.mid", "cut.large", "cut.notch", "cut.overlay", "cut.menu", "cut.placard", "cut.tooltip", "cut.toggleTrack", "cut.toggleKnob",
		"type.name.family", "type.name.weight", "type.name.size", "type.name.line",
		"type.sentence.family", "type.sentence.weight", "type.sentence.size", "type.sentence.line",
		"type.section.family", "type.section.weight", "type.section.size", "type.section.line",
		"type.title.family", "type.title.weight", "type.title.size", "type.title.line",
		"type.row.family", "type.row.weight", "type.row.size", "type.row.line",
		"type.body.family", "type.body.weight", "type.body.size", "type.body.line",
		"type.proof.family", "type.proof.weight", "type.proof.size", "type.proof.line",
		"type.label.family", "type.label.weight", "type.label.size", "type.label.line",
		"type.caption.family", "type.caption.weight", "type.caption.size", "type.caption.line",
		"type.group.family", "type.group.weight", "type.group.size", "type.group.line",
		"type.mono.family", "type.mono.weight", "type.mono.size", "type.mono.line",
		"type.monoSm.family", "type.monoSm.weight", "type.monoSm.size", "type.monoSm.line",
		"font.sans", "font.mono", "font.icons",
		"motion.curve", "motion.press", "motion.hoverIn", "motion.hoverOut", "motion.focusIn", "motion.focusOut", "motion.select", "motion.field", "motion.notch", "motion.unfold", "motion.navigate", "motion.menuIn", "motion.menuOut", "motion.dialogIn", "motion.dialogOut", "motion.tooltipIn", "motion.tooltipDelay", "motion.placardIn", "motion.placardLife", "motion.wheel", "motion.working", "motion.sweep",
		"alpha.disabled", "alpha.hoverFill", "alpha.hoverInk", "alpha.hoverPlane", "alpha.pressRow", "alpha.scrimDark", "alpha.scrimLight", "alpha.hatch", "alpha.workingBase", "alpha.mineralJade", "alpha.mineral", "alpha.onSel2",
		"breakpoint.medium", "breakpoint.expanded",
		"size.plaqueHeight", "size.plaqueHeightSmall", "size.plaqueInset", "size.iconPlaque", "size.tabletHeight", "size.tabletHeightCompact", "size.railExpanded", "size.railMedium", "size.listExpanded", "size.listMedium", "size.menuWidthMin", "size.menuItem", "size.dialogWidth", "size.draftFormMax", "size.draftAsideMin", "size.draftAsideMax", "size.draftWideAt", "size.boardMax", "size.boardColumnsAt",
		"sizeNotes.plaqueHeight", "sizeNotes.boardColumnsAt",
		"signature.dark.ground", "signature.dark.stone1", "signature.dark.stone2", "signature.dark.stone3", "signature.dark.stone4", "signature.dark.vein", "signature.dark.fg", "signature.dark.fg2", "signature.dark.fg3", "signature.dark.sel", "signature.dark.onSel", "signature.dark.onSel2", "signature.dark.gold", "signature.dark.onGold", "signature.dark.goldInk", "signature.dark.jasper", "signature.dark.scrim",
		"signature.light.ground", "signature.light.stone1", "signature.light.stone2", "signature.light.stone3", "signature.light.stone4", "signature.light.vein", "signature.light.fg", "signature.light.fg2", "signature.light.fg3", "signature.light.sel", "signature.light.onSel", "signature.light.onSel2", "signature.light.gold", "signature.light.onGold", "signature.light.goldInk", "signature.light.jasper", "signature.light.scrim",
		"minerals.dark.jade", "minerals.dark.lapis", "minerals.dark.amber", "minerals.light.jade", "minerals.light.lapis", "minerals.light.amber",
		"palette.dark.ground", "palette.dark.stone1", "palette.dark.stone2", "palette.dark.stone3", "palette.dark.stone4", "palette.dark.vein", "palette.dark.fg", "palette.dark.fg2", "palette.dark.fg3",
		"palette.light.ground", "palette.light.stone1", "palette.light.stone2", "palette.light.stone3", "palette.light.stone4", "palette.light.vein", "palette.light.fg", "palette.light.fg2", "palette.light.fg3",
		"palette.accents.gold", "palette.accents.onGold", "palette.accents.sel", "palette.accents.onSel", "palette.accents.jasper", "palette.accents.scrim", "palette.toned.goldInk", "palette.toned.jasper",
		"contrast.minimum",
		"edition.arch", "edition.cachyos", "edition.fedora", "edition.void", "edition.nixos", "edition.ubuntu", "edition.debian", "edition.opensuse", "edition.manjaro", "edition.endeavouros", "edition.linuxmint", "edition.pop", "edition.gentoo", "edition.alpine",
	}
	for _, key := range required {
		if _, ok := tokenAt(tokens, key); !ok {
			t.Errorf("missing required token %s", key)
		}
	}
	sizes, _ := tokens["size"].(map[string]any)
	notes, _ := tokens["sizeNotes"].(map[string]any)
	for key := range sizes {
		if _, ok := notes[key]; !ok {
			t.Errorf("missing required token sizeNotes.%s", key)
		}
	}
	for key := range notes {
		if _, ok := sizes[key]; !ok {
			t.Errorf("missing required token size.%s", key)
		}
	}
}

func TestQMLRejectsSignalHandlerPropertyNames(t *testing.T) {
	_, err := qmlOutput(document{
		"space": map[string]any{"onDanger": "invalid"},
	})
	if err == nil || !strings.Contains(err.Error(), "space.onDanger") {
		t.Fatalf("qml signal-handler key error = %v, want space.onDanger", err)
	}
}

func TestQMLRoleGroupsAreJavaScriptObjects(t *testing.T) {
	tokens, err := readTokens(filepath.Join(sourceRoot(), "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	output, err := qmlOutput(tokens)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"alpha", "edition", "minerals", "palette", "signature"} {
		want := "readonly property var " + group + ": ("
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("QML output does not contain %q", want)
		}
	}
	if bytes.Contains(output, []byte("property color onGold")) || bytes.Contains(output, []byte("property string onGold")) {
		t.Fatal("QML output emitted onGold as a signal-handler property")
	}
}

func tokenAt(tokens document, path string) (any, bool) {
	var current any = map[string]any(tokens)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func tempRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	data, err := os.ReadFile(filepath.Join(sourceRoot(), "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tokens.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func sourceRoot() string {
	return filepath.Clean(filepath.Join("..", "..", "..", ".."))
}

func readOutputs(t *testing.T, root string) map[string][]byte {
	t.Helper()
	outputs := make(map[string][]byte, len(outputNames))
	for _, name := range outputNames {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		outputs[name] = data
	}
	return outputs
}
