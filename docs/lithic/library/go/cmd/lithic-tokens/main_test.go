package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
		"font.sans", "font.mono", "font.icons",
		"motion.curve", "motion.press", "motion.hoverIn", "motion.hoverOut", "motion.focusIn", "motion.focusOut", "motion.select", "motion.field", "motion.notch", "motion.unfold", "motion.navigate", "motion.menuIn", "motion.menuOut", "motion.dialogIn", "motion.dialogOut", "motion.tooltipIn", "motion.tooltipDelay", "motion.placardIn", "motion.placardLife", "motion.wheel", "motion.working", "motion.sweep",
		"alpha.disabled", "alpha.hoverFill", "alpha.hoverInk", "alpha.hoverPlane", "alpha.pressRow", "alpha.scrimDark", "alpha.scrimLight", "alpha.hatch", "alpha.workingBase", "alpha.onSel2", "alpha.selMix",
		"breakpoint.medium", "breakpoint.expanded",
		"size.plaqueHeight", "size.plaqueHeightSmall", "size.plaqueInset", "size.iconPlaque", "size.tabletHeight", "size.tabletHeightCompact", "size.railExpanded", "size.railMedium", "size.listExpanded", "size.listMedium", "size.menuWidthMin", "size.menuItem", "size.dialogWidth", "size.draftFormMax", "size.draftAsideMin", "size.draftAsideMax", "size.draftWideAt", "size.boardMax", "size.boardColumnsAt",
		"sizeNotes.plaqueHeight", "sizeNotes.boardColumnsAt",
		"minerals.dark.jade", "minerals.dark.lapis", "minerals.dark.amber", "minerals.light.jade", "minerals.light.lapis", "minerals.light.amber",
		"palette.accents.gold", "palette.accents.jasper", "palette.accents.scrim", "palette.accents.sel", "palette.accents.selMix", "palette.dye.neutral", "palette.dye.ink",
		"dye.boost.gold", "dye.boost.ground", "dye.boost.vein",
		"dye.chroma.gold", "dye.chroma.ground", "dye.chroma.ink", "dye.chroma.jasper", "dye.chroma.onGold", "dye.chroma.onSel", "dye.chroma.sel", "dye.chroma.stone",
		"dye.fallback.goldChroma", "dye.fallback.goldHue", "dye.fallback.groundChroma", "dye.fallback.inkChroma", "dye.fallback.jasperHue", "dye.fallback.neutralHue", "dye.fallback.selChroma", "dye.fallback.stoneChroma",
		"dye.mineral.jade", "dye.mineral.lapis", "dye.mineral.amber", "dye.mineral.jasper", "dye.tint.low", "dye.tint.medium",
		"edition.fedora", "edition.nixos", "edition.void", "contrast.minimum",
	}
	for _, style := range []string{"name", "sentence", "section", "title", "row", "body", "proof", "label", "detail", "caption", "group", "mono", "monoSm"} {
		for _, field := range []string{"family", "weight", "size", "line"} {
			required = append(required, "type."+style+"."+field)
		}
	}
	roles := []string{"ground", "stone1", "stone2", "stone3", "stone4", "slab", "field", "vein", "edge", "fg", "fg2", "fg3", "sel", "onSel", "gold", "onGold", "goldInk", "jasper"}
	for _, table := range []string{"dark", "darkHigh", "light", "lightHigh"} {
		for _, role := range slices.Concat(roles, []string{"onSel2", "scrim"}) {
			required = append(required, "signature."+table+"."+role)
		}
		for _, role := range slices.Concat(roles, []string{"mineralField"}) {
			required = append(required, "tone."+table+"."+role)
		}
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
	for _, group := range []string{"alpha", "dye", "edition", "minerals", "palette", "signature", "tone"} {
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
