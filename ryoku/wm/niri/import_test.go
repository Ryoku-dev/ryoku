package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseKDLCommentsRawStringsAndSlashDash(t *testing.T) {
	src := `// line comment
/* outer /* nested */ done */
include r#"cfg/a.kdl"#
binds {
    Mod+X repeat=false { spawn-sh r##"printf "// raw""##; }
    /- Mod+Z { spawn "ignored"; }
}
`
	nodes, err := parseKDL("fixture.kdl", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Name != "include" || nodes[0].Args[0].String() != "cfg/a.kdl" {
		t.Fatalf("unexpected top-level parse: %#v", nodes)
	}
	binds := nodes[1]
	if len(binds.Children) != 1 || binds.Children[0].Name != "Mod+X" {
		t.Fatalf("slash-dash node was not discarded: %#v", binds.Children)
	}
	action := firstAction(binds.Children[0])
	if action == nil || action.Name != "spawn-sh" || action.Args[0].String() != `printf "// raw"` {
		t.Fatalf("raw action parse: %#v", action)
	}
}

func TestParseKDLRejectsMalformedInput(t *testing.T) {
	for name, src := range map[string]string{
		"unterminated block comment": "/* open",
		"unterminated raw string":    `node r#"open`,
		"missing closing brace":      "layout {\n gaps 4\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseKDL("broken.kdl", src); err == nil {
				t.Fatal("malformed KDL was accepted")
			}
		})
	}
}

func TestScanNiriImportRichTree(t *testing.T) {
	scan, err := scanNiriImport(filepath.Join("testdata", "rich"))
	if err != nil {
		t.Fatal(err)
	}
	meta := niriImporterMetadata()
	if scan.Provider != meta.ID || scan.Name != meta.Name {
		t.Fatalf("provider metadata: %#v", scan)
	}
	if got := importValue(scan.Patch, "desktop", "input", "kbLayout"); got != "us,de" {
		t.Fatalf("keyboard layout = %#v", got)
	}
	if got := importValue(scan.Patch, "desktop", "input", "repeatRate"); got != 32 {
		t.Fatalf("repeat rate = %#v", got)
	}
	if got := importValue(scan.Patch, "desktop", "appearance", "gapsOut"); got != 18 {
		t.Fatalf("gaps = %#v", got)
	}
	if got := importValue(scan.Patch, "wm", meta.ID, "defaultColumnWidth"); got != 0.55 {
		t.Fatalf("column width = %#v", got)
	}
	if got := importValue(scan.Patch, "wm", meta.ID, "anim", "windowOpen", "mode"); got != "spring" {
		t.Fatalf("window-open mode = %#v", got)
	}
	if got := importValue(scan.Patch, "wm", meta.ID, "anim", "windowClose", "durationMs"); got != 180 {
		t.Fatalf("window-close duration = %#v", got)
	}
	if got := importValue(scan.Patch, "desktop", "apps", "browser"); got != "zen-browser" {
		t.Fatalf("browser = %#v", got)
	}
	if got := importValue(scan.Patch, "wm", meta.ID, "preferNoCsd"); got != true {
		t.Fatalf("unreferenced cfg tree was not read: %#v", got)
	}
	if got := importValue(scan.Patch, "wm", meta.ID, "screenshotPath"); got != "~/Pictures/Screenshots/%Y-%m-%d // capture.png" {
		t.Fatalf("raw screenshot path = %#v", got)
	}
	if len(scan.Binds) != 2 {
		t.Fatalf("mapped binds = %d, want 2", len(scan.Binds))
	}
	if len(scan.Conflicts) != 1 || scan.Conflicts[0].Combo != "SUPER + Return" {
		t.Fatalf("bind conflicts: %#v", scan.Conflicts)
	}
	if !strings.Contains(scan.Preserved, `output "eDP-1"`) ||
		!strings.Contains(scan.Preserved, "power-off-monitors") ||
		!strings.Contains(scan.Preserved, "xwayland-satellite") {
		t.Fatalf("preserved KDL is incomplete:\n%s", scan.Preserved)
	}
	if !strings.Contains(scan.Preserved, "/- binds {") {
		t.Fatalf("unsupported bind must be preserved without shadowing a shipped chord:\n%s", scan.Preserved)
	}
	if len(scan.Losses) < 4 {
		t.Fatalf("losses = %d, want output, metadata, action, and unknown node", len(scan.Losses))
	}
	if got := importSliceLen(scan.Patch, "desktop", "windowRules"); got != 3 {
		t.Fatalf("window rules = %d, want float, opacity and minsize", got)
	}
	if got := importSliceLen(scan.Patch, "wm", meta.ID, "layerRules"); got != 1 {
		t.Fatalf("layer rules = %d", got)
	}
	if got := importSliceLen(scan.Patch, "desktop", "autostart"); got != 2 {
		t.Fatalf("autostart rows = %d", got)
	}
}

func importValue(root map[string]any, path ...string) any {
	var cur any = root
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[key]
	}
	return cur
}

func importSliceLen(root map[string]any, path ...string) int {
	value := importValue(root, path...)
	rows, _ := value.([]any)
	return len(rows)
}
