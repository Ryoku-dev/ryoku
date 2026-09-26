package doctor

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"ryoku-cli/internal/sys"
	"strings"
	"testing"

	wm "ryoku-wm"
)

func TestParseProcSwaps(t *testing.T) {
	in := "Filename\t\t\t\tType\t\tSize\t\tUsed\t\tPriority\n" +
		"/swap/swapfile                          file\t\t16777212\t1744224\t\t-1\n" +
		"/dev/nvme0n1p3                          partition\t8388604\t0\t\t-2\n" +
		"/var/lib/with\\040space/swapfile         file\t\t1024\t\t0\t\t-3\n"
	got := parseProcSwaps(in)
	if len(got) != 2 {
		t.Fatalf("got %d file swaps, want 2 (partition excluded): %+v", len(got), got)
	}
	if got[0].path != "/swap/swapfile" || got[0].sizeKB != 16777212 {
		t.Errorf("first swap = %+v, want path /swap/swapfile size 16777212", got[0])
	}
	if got[1].path != "/var/lib/with space/swapfile" {
		t.Errorf("escaped path = %q, want the \\040 unescaped to a space", got[1].path)
	}
}

func TestParseProcSwapsHeaderOnly(t *testing.T) {
	if got := parseProcSwaps("Filename Type Size Used Priority\n"); len(got) != 0 {
		t.Errorf("header-only input should yield no swaps, got %+v", got)
	}
}

func TestDirOnlyContains(t *testing.T) {
	dir := t.TempDir()
	if dirOnlyContains(dir, "swapfile") {
		t.Error("empty dir should not report only-contains")
	}
	if err := os.WriteFile(filepath.Join(dir, "swapfile"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !dirOnlyContains(dir, "swapfile") {
		t.Error("dir holding only the swapfile should match")
	}
	// a second file must block the auto-fix; surgery never runs on a shared dir.
	if err := os.WriteFile(filepath.Join(dir, "other"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	if dirOnlyContains(dir, "swapfile") {
		t.Error("dir with an extra file must not match")
	}
}

func TestNonEmptyLines(t *testing.T) {
	got := nonEmptyLines("a\n\n  \nb\n")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("nonEmptyLines dropped wrong lines: %q", got)
	}
}

func TestStaleInstallMapper(t *testing.T) {
	const node = "/dev/mapper/root"
	cases := []struct {
		name    string
		nodes   []string
		root    string
		mounted map[string]bool
		want    string
	}{
		{"orphan, nothing mounted", []string{node}, "/dev/nvme0n1p2", nil, "root"},
		{"is the live root", []string{node}, node, map[string]bool{node: true}, ""},
		{"mounted as a target", []string{node}, "/dev/sda1", map[string]bool{node: true}, ""},
		{"absent", nil, "/dev/nvme0n1p2", nil, ""},
		{"only a differently named crypt", []string{"/dev/mapper/cr"}, "/dev/sda1", nil, ""},
	}
	for _, c := range cases {
		if got := staleInstallMapper(c.nodes, c.root, c.mounted); got != c.want {
			t.Errorf("%s: staleInstallMapper = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestParseCryptMapperNodes(t *testing.T) {
	if got := parseCryptMapperNodes("No devices found\n"); got != nil {
		t.Errorf("\"No devices found\" should yield no nodes, got %v", got)
	}
	if got := parseCryptMapperNodes(""); got != nil {
		t.Errorf("empty output should yield no nodes, got %v", got)
	}
	got := parseCryptMapperNodes("root\t(254:0)\nbackup (254:1)\n")
	if len(got) != 2 || got[0] != "/dev/mapper/root" || got[1] != "/dev/mapper/backup" {
		t.Errorf("parseCryptMapperNodes = %v, want the two /dev/mapper paths", got)
	}
}

func TestBaseSource(t *testing.T) {
	if got := baseSource("  /dev/mapper/root[/@home] "); got != "/dev/mapper/root" {
		t.Errorf("baseSource kept the subvolume suffix: %q", got)
	}
	if got := baseSource("/dev/nvme0n1p2"); got != "/dev/nvme0n1p2" {
		t.Errorf("baseSource mangled a plain device: %q", got)
	}
}

func TestTailLines(t *testing.T) {
	if got := tailLines("1\n2\n3\n4\n5", 2); got != "4\n5" {
		t.Errorf("tailLines = %q, want \"4\\n5\"", got)
	}
	if got := tailLines("1\n2", 10); got != "1\n2" {
		t.Errorf("tailLines fewer-than-n = %q, want \"1\\n2\"", got)
	}
}

func TestReportPathOverride(t *testing.T) {
	if got := reportPath("/tmp/x.txt"); got != "/tmp/x.txt" {
		t.Errorf("explicit path = %q, want /tmp/x.txt", got)
	}
	t.Setenv("XDG_STATE_HOME", "/state")
	if got := reportPath(""); got != "/state/ryoku/doctor-report.txt" {
		t.Errorf("default path = %q, want /state/ryoku/doctor-report.txt", got)
	}
}

func TestRecStatusLabels(t *testing.T) {
	for s, want := range map[recStatus]string{recOK: "ok", recFixed: "fixed", recWouldFix: "todo", recWarn: "warn", recFailed: "fail"} {
		if got := s.label(); got != want {
			t.Errorf("label(%d) = %q, want %q", s, got, want)
		}
	}
}

func TestGatherReportIncludesFindings(t *testing.T) {
	fs := []finding{{"swap kept out of snapshots", warnRes("swapfile in @").withFix("ryoku doctor")}}
	rep := gatherReport(fs)
	for _, want := range []string{"Ryoku diagnostic report", "swap kept out of snapshots", "swapfile in @", "fix: ryoku doctor", "## system", "## packages"} {
		if !strings.Contains(rep, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

func TestShellDaemonReachable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	if shellDaemonReachable() {
		t.Fatal("with no socket the daemon must read as unreachable")
	}
	ln, err := net.Listen("unix", filepath.Join(dir, "ryoku-shell.sock"))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			b := make([]byte, 64)
			n, _ := c.Read(b)
			if strings.HasPrefix(strings.TrimSpace(string(b[:n])), "ping") {
				fmt.Fprintln(c, "ok")
			}
			c.Close()
		}
	}()
	if !shellDaemonReachable() {
		t.Fatal("a daemon answering ping with ok must read as reachable")
	}
}

func TestReconcileShellDaemonOutsideSession(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no provider binary resolves, so no live instance
	if r := reconcileShellDaemon(true); r.status != recOK {
		t.Fatalf("outside a live session the daemon check must be ok, got %q: %s", r.status.label(), r.detail)
	}
}

func TestDaemonIsStale(t *testing.T) {
	cases := []struct {
		name      string
		live, sig string
		ok        bool
		want      bool
	}{
		{"bound to a dead instance", "live", "dead", true, true},
		{"bound to the live instance", "live", "live", true, false},
		{"signature unavailable (old daemon)", "live", "", false, false},
	}
	for _, c := range cases {
		if got := daemonIsStale(c.live, c.sig, c.ok); got != c.want {
			t.Errorf("%s: daemonIsStale(%q, %q, %v) = %v, want %v", c.name, c.live, c.sig, c.ok, got, c.want)
		}
	}
}

func TestDaemonBinaryReplaced(t *testing.T) {
	cases := []struct {
		name, cmdline, exe string
		want               bool
	}{
		{"packaged daemon on deleted binary", "/usr/bin/ryoku-shell\x00daemon\x00", "/usr/bin/ryoku-shell (deleted)", true},
		{"bare name on deleted binary", "ryoku-shell\x00daemon\x00", "/home/u/.local/bin/ryoku-shell (deleted)", true},
		{"daemon on live binary", "/usr/bin/ryoku-shell\x00daemon\x00", "/usr/bin/ryoku-shell", false},
		{"other subcommand", "/usr/bin/ryoku-shell\x00quit\x00", "/usr/bin/ryoku-shell (deleted)", false},
		{"other binary", "/usr/bin/other-shell\x00daemon\x00", "/usr/bin/other-shell (deleted)", false},
		{"doctor's own grep-alike shell", "bash\x00-c\x00ryoku-shell daemon\x00", "/usr/bin/bash", false},
	}
	for _, c := range cases {
		if got := daemonBinaryReplaced(c.cmdline, c.exe); got != c.want {
			t.Errorf("%s: daemonBinaryReplaced = %v, want %v", c.name, got, c.want)
		}
	}
}

// A reachable daemon bound to a previous compositor instance must be flagged for
// a restart, not passed as healthy -- the frozen-workspaces / dead-power bug.
func TestReconcileShellDaemonStale(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ryoku-shell"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A fake provider whose caps report a live instance handle, resolved
	// hermetically through RYOKU_WM so the test needs no session.
	caps := `#!/bin/sh
if [ "$1" = caps ]; then
  echo '{"name":"hyprland","instance":"live-instance","supports":[],"workspaceModel":"fixed"}'
fi
`
	if err := os.WriteFile(filepath.Join(bin, "ryoku-wm-hyprland"), []byte(caps), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("RYOKU_WM", "hyprland")

	ln, err := net.Listen("unix", filepath.Join(dir, "ryoku-shell.sock"))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			b := make([]byte, 64)
			n, _ := c.Read(b)
			switch strings.TrimSpace(string(b[:n])) {
			case "ping":
				fmt.Fprintln(c, "ok")
			case "signature":
				fmt.Fprintln(c, "stale-instance") // a different instance than live
			}
			c.Close()
		}
	}()

	if r := reconcileShellDaemon(true); r.status != recWouldFix { // check-only: detect, don't restart
		t.Fatalf("a reachable-but-stale daemon must be flagged (todo), got %q: %s", r.status.label(), r.detail)
	}
}

func TestHyprLuaSane(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid monitor", "hl.monitor({ output = \"\", mode = \"highrr\", scale = 1 })\n", true},
		{"comment only", "-- managed by ryoku-monitor\n", true},
		{"truncated mid-call", "hl.monitor({ output = \"DP-1\", mode = \"hi", false},
		{"empty", "   \n\n", false},
	}
	for _, c := range cases {
		if got := hyprLuaSane(c.in); got != c.want {
			t.Errorf("%s: hyprLuaSane()=%v, want %v", c.name, got, c.want)
		}
	}
}

// torn generated drop-in -> detected and repaired to a parseable safe seed;
// a valid sibling stays untouched, and the fix is idempotent. PATH is wiped
// so the test never touches luac/the provider/ryoku-monitor: just the structural
// check + the safe-seed fallback, deterministically.
func TestReconcileHyprlandConfigRepairsCorruptDropin(t *testing.T) {
	t.Setenv("RYOKU_WM", "hyprland") // this reconciler is Hyprland's; pin the provider
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", "")

	hypr := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(hypr, 0o755); err != nil {
		t.Fatal(err)
	}
	put := func(name, body string) {
		if err := os.WriteFile(filepath.Join(hypr, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	get := func(name string) string {
		b, err := os.ReadFile(filepath.Join(hypr, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	put("hyprland.lua", "pcall(require, \"monitors\")\n")
	put("monitors.lua", "hl.monitor({ output = \"DP-1\", mode = \"hi") // truncated mid-write
	put("gpu.lua", "-- placeholder\n")                                 // valid

	if r := reconcileHyprlandConfig(true); r.status != recWouldFix {
		t.Fatalf("check-only: status=%s detail=%q, want todo", r.status.label(), r.detail)
	}
	if !strings.Contains(get("monitors.lua"), "mode = \"hi") {
		t.Fatal("check-only must not modify the drop-in")
	}

	if r := reconcileHyprlandConfig(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if got := get("monitors.lua"); !hyprLuaSane(got) {
		t.Fatalf("monitors.lua not parseable after repair: %q", got)
	}
	if got := get("gpu.lua"); got != "-- placeholder\n" {
		t.Fatalf("valid gpu.lua must be left untouched, got %q", got)
	}

	if r := reconcileHyprlandConfig(false); r.status != recOK {
		t.Fatalf("second run: status=%s, want ok", r.status.label())
	}
}

func TestReconcileHyprlandConfigNoConfig(t *testing.T) {
	t.Setenv("RYOKU_WM", "hyprland") // this reconciler is Hyprland's; pin the provider
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", "")
	if r := reconcileHyprlandConfig(false); r.status != recOK {
		t.Fatalf("no hyprland.lua: status=%s, want ok", r.status.label())
	}
}

func TestReconcileThemeLua(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", "")

	hypr := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(hypr, 0o755); err != nil {
		t.Fatal(err)
	}
	themeLua := filepath.Join(hypr, "theme.lua")

	// absent: nothing to prune.
	if r := reconcileThemeLua(false); r.status != recOK {
		t.Fatalf("absent: status=%s, want ok", r.status.label())
	}

	// present: check-only reports the fix without removing the file.
	if err := os.WriteFile(themeLua, []byte("hl.curve(\"x\", {})\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileThemeLua(true); r.status != recWouldFix {
		t.Fatalf("check-only: status=%s detail=%q, want todo", r.status.label(), r.detail)
	}
	if !sys.Exists(themeLua) {
		t.Fatal("check-only must not remove theme.lua")
	}

	// fix: the orphaned file is pruned.
	if r := reconcileThemeLua(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if sys.Exists(themeLua) {
		t.Fatal("fix must remove the orphaned theme.lua")
	}

	// idempotent: a second run is a clean ok.
	if r := reconcileThemeLua(false); r.status != recOK {
		t.Fatalf("second run: status=%s, want ok", r.status.label())
	}
}

// the snapper reconciler's decision logic lives in planSnapper, a pure
// function of an observed snapperState. exercising it directly stays
// hermetic: no real /etc, no snapper or btrfs invocation, just the branch
// table reconcileSnapper switches on.
func TestPlanSnapper(t *testing.T) {
	consistent := snapperState{
		rootIsBtrfs:         true,
		configExists:        true,
		snapshotsExists:     true,
		snapshotsIsSubvol:   true,
		snapshotsMode:       0o750,
		confdExists:         true,
		confdContents:       "SNAPPER_CONFIGS=\"root\"\n",
		snapperInstalled:    true,
		snapPacInstalled:    true,
		limineInstalled:     true,
		limineSyncInstalled: true,
		limineSyncEnabled:   true,
	}
	withMode := func(m os.FileMode) snapperState { s := consistent; s.snapshotsMode = m; return s }
	withConfd := func(c string) snapperState { s := consistent; s.confdContents = c; return s }
	plainSnapshotsDir := func() snapperState { s := consistent; s.snapshotsIsSubvol = false; return s }
	noSnapPac := func() snapperState { s := consistent; s.snapPacInstalled = false; return s }
	nonLimine := func() snapperState {
		s := consistent
		s.limineInstalled, s.limineSyncInstalled, s.limineSyncEnabled = false, false, false
		return s
	}
	noLimineSync := func() snapperState {
		s := consistent
		s.limineSyncInstalled = false
		s.limineSyncEnabled = false
		return s
	}
	syncDisabled := func() snapperState { s := consistent; s.limineSyncEnabled = false; return s }

	cases := []struct {
		name        string
		in          snapperState
		want        snapperOutcome
		wantProblem string // substring expected in problems, empty when none
	}{
		{"missing config + btrfs + snapper installed converges with create", snapperState{rootIsBtrfs: true, snapperInstalled: true}, snapperCreate, ""},
		{"missing config + btrfs + snapper not installed recommends install", snapperState{rootIsBtrfs: true}, snapperWarnMissingPkgs, ""},
		{"missing config + non-btrfs root warns honestly", snapperState{rootIsBtrfs: false}, snapperWarnNotBtrfs, ""},
		{"missing config + install-time opt-out marker is respected, not converged", snapperState{rootIsBtrfs: true, snapperInstalled: true, optedOut: true}, snapperOptedOut, ""},
		{"existing config wins over a stale opt-out marker", func() snapperState { s := consistent; s.optedOut = true; return s }(), snapperOK, ""},
		{"present + consistent reads ok", consistent, snapperOK, ""},
		{"/.snapshots wrong mode warns inconsistent", withMode(0o755), snapperWarnInconsistent, "mode 0755"},
		{"conf.d missing root warns inconsistent", withConfd("SNAPPER_CONFIGS=\"home\"\n"), snapperWarnInconsistent, "does not list the root config"},
		{"/.snapshots is plain dir warns inconsistent", plainSnapshotsDir(), snapperWarnInconsistent, "plain directory"},
		{"configured but snap-pac missing recommends it", noSnapPac(), snapperWarnInconsistent, "snap-pac"},
		{"configured but limine-snapper-sync missing recommends it", noLimineSync(), snapperWarnInconsistent, "limine-snapper-sync"},
		{"sync installed but service disabled tells the exact enable", syncDisabled(), snapperWarnInconsistent, "limine-snapper-sync.service is disabled"},
		{"non-limine box is healthy without the sync package", nonLimine(), snapperOK, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, problems := planSnapper(c.in)
			if got != c.want {
				t.Fatalf("planSnapper outcome = %d, want %d (problems=%v)", got, c.want, problems)
			}
			if c.wantProblem == "" {
				if len(problems) != 0 {
					t.Errorf("unexpected problems: %v", problems)
				}
				return
			}
			joined := strings.Join(problems, " | ")
			if !strings.Contains(joined, c.wantProblem) {
				t.Errorf("problems = %q, want one containing %q", joined, c.wantProblem)
			}
		})
	}
}

// mergedConfdRoot decides how /etc/conf.d/snapper changes when doctor writes
// the snapper root config. it must add "root" without dropping anything a
// human (or another tool) already put in the file.
func TestMergedConfdRoot(t *testing.T) {
	// missing file: doctor writes the canonical snippet.
	out, changed := mergedConfdRoot(false, "")
	if !changed || !strings.Contains(out, `SNAPPER_CONFIGS="root"`) {
		t.Errorf("missing file: changed=%v out=%q, want canonical content with root", changed, out)
	}

	// already lists root: leave the file alone (idempotent doctor).
	in := "SNAPPER_CONFIGS=\"root\"\n"
	if out, changed := mergedConfdRoot(true, in); changed || out != in {
		t.Errorf("present+root: changed=%v out=%q, want unchanged", changed, out)
	}

	// lists another config: append root, keep the existing one.
	in = "SNAPPER_CONFIGS=\"home\"\n"
	out, changed = mergedConfdRoot(true, in)
	if !changed || !strings.Contains(out, `SNAPPER_CONFIGS="home root"`) {
		t.Errorf("present+home: changed=%v out=%q, want root appended after home", changed, out)
	}

	// no SNAPPER_CONFIGS line at all: add one, keep surrounding lines.
	in = "# user comment\n"
	out, changed = mergedConfdRoot(true, in)
	if !changed || !strings.Contains(out, `SNAPPER_CONFIGS="root"`) || !strings.Contains(out, "# user comment") {
		t.Errorf("no SNAPPER_CONFIGS line: changed=%v out=%q, want root line added and comment kept", changed, out)
	}
}

// reconcileDisplayModes gates on a live session and delegates to `ryoku-monitor
// settle`; drive detection and stub ryoku-monitor so the outcomes are deterministic.
func TestReconcileDisplayModes(t *testing.T) {
	bin := t.TempDir()
	mkExec := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// ryoku-monitor stub: `settle --check` exits $CHK, `settle` exits $SET.
	mkExec("ryoku-monitor", "#!/bin/sh\n"+
		"if [ \"$1\" = settle ] && [ \"$2\" = --check ]; then exit ${CHK:-0}; fi\n"+
		"if [ \"$1\" = settle ]; then exit ${SET:-0}; fi\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CURRENT_DESKTOP", "hyprland") // a live session for wm.Detect()

	t.Setenv("CHK", "0") // every display at its best available mode
	if r := reconcileDisplayModes(false); r.status != recOK {
		t.Fatalf("settled: got %s (%q), want ok", r.status.label(), r.detail)
	}
	t.Setenv("CHK", "1") // a display is below its available resolution
	if r := reconcileDisplayModes(true); r.status != recWouldFix {
		t.Fatalf("drift check-only: got %s, want todo", r.status.label())
	}
	t.Setenv("SET", "0") // settle recovers it
	if r := reconcileDisplayModes(false); r.status != recFixed {
		t.Fatalf("drift apply: got %s, want fixed", r.status.label())
	}
	t.Setenv("SET", "1") // settle cannot recover it
	if r := reconcileDisplayModes(false); r.status != recWarn {
		t.Fatalf("settle failed: got %s, want warn", r.status.label())
	}
	t.Setenv("RYOKU_WM", "none") // no live session for wm.Detect()
	if r := reconcileDisplayModes(false); r.status != recOK {
		t.Fatalf("no session: got %s, want ok", r.status.label())
	}
}

func TestConfiguredCursor(t *testing.T) {
	if th, sz := configuredCursor(nil); th != defaultCursorTheme || sz != 24 {
		t.Fatalf("no store: got %q/%d, want %s/24", th, sz, defaultCursorTheme)
	}
	if th, sz := configuredCursor([]byte(`{"desktop":{"cursor":{"theme":"phinger-cursors","size":32}}}`)); th != "phinger-cursors" || sz != 32 {
		t.Fatalf("override: got %q/%d, want phinger-cursors/32", th, sz)
	}
	if th, sz := configuredCursor([]byte(`not json`)); th != defaultCursorTheme || sz != 24 {
		t.Fatalf("garbage store must fall back to the default: got %q/%d", th, sz)
	}
	// "Follow the wallpaper" is a role, not a theme name; the check must see the
	// concrete theme it resolves to, or it resets a working pick.
	if th, _ := configuredCursor([]byte(`{"desktop":{"cursor":{"theme":"DYNAMIC"}}}`)); th != wm.CursorThemeMaterial {
		t.Fatalf("DYNAMIC resolved to %q, want %q", th, wm.CursorThemeMaterial)
	}
}

func TestStripCursorMaterial(t *testing.T) {
	raw := []byte(`{"desktop":{"cursor":{"material":true,"size":18}}}`)
	out, changed, err := stripCursorMaterial(raw)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v, want the key dropped", changed, err)
	}
	if _, sz := configuredCursor(out); sz != 18 {
		t.Fatalf("strip lost the size: got %d", sz)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	cur := cfg["desktop"].(map[string]any)["cursor"].(map[string]any)
	if _, ok := cur["material"]; ok {
		t.Fatal("material survived")
	}
	if _, changed, err := stripCursorMaterial([]byte(`{"desktop":{"cursor":{"size":18}}}`)); changed || err != nil {
		t.Fatalf("a clean store must not rewrite: changed=%v err=%v", changed, err)
	}
}

func TestCursorThemeInstalled(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Real-Theme", "cursors"), 0o755); err != nil {
		t.Fatal(err)
	}
	// a theme dir with no cursors/ subdir is an index-only stub, not usable.
	if err := os.MkdirAll(filepath.Join(root, "Stub-Theme"), 0o755); err != nil {
		t.Fatal(err)
	}
	dirs := []string{root}
	if !cursorThemeInstalled("Real-Theme", dirs) {
		t.Error("Real-Theme with a cursors/ dir must count as installed")
	}
	if cursorThemeInstalled("Stub-Theme", dirs) {
		t.Error("a theme with no cursors/ dir must not count as installed")
	}
	if cursorThemeInstalled("", dirs) || cursorThemeInstalled("Absent", dirs) {
		t.Error("empty or absent theme must not count as installed")
	}
}

func TestResetCursorTheme(t *testing.T) {
	out, err := resetCursorTheme([]byte(`{"desktop":{"cursor":{"theme":"phinger-cursors","size":32},"input":{"sensitivity":0.2}}}`), defaultCursorTheme)
	if err != nil {
		t.Fatal(err)
	}
	th, sz := configuredCursor(out)
	if th != defaultCursorTheme || sz != 32 {
		t.Fatalf("reset kept theme %q size %d, want %s/32 (size preserved)", th, sz, defaultCursorTheme)
	}
	if !strings.Contains(string(out), "sensitivity") {
		t.Error("reset dropped an unrelated key")
	}
}

// the converge path: a Hub-picked theme that is not on disk resets to the
// package-guaranteed default, then a re-run is a no-op.
func TestReconcileCursorThemeConverge(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", t.TempDir()) // no provider on PATH; the live setcursor no-ops
	if err := os.MkdirAll(filepath.Join(home, ".config", "hypr"), 0o755); err != nil {
		t.Fatal(err)
	}
	// the default is present in a per-user icon dir (independent of /usr/share).
	if err := os.MkdirAll(filepath.Join(home, ".local", "share", "icons", defaultCursorTheme, "cursors"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(home, ".config", "ryoku", "desktop.json")
	if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store, []byte(`{"desktop":{"cursor":{"theme":"No-Such-Cursor-Theme","size":28}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileCursorTheme(true); r.status != recWouldFix {
		t.Fatalf("check-only on a missing theme: got %s, want todo", r.status.label())
	}
	r := reconcileCursorTheme(false)
	if r.status != recFixed {
		t.Fatalf("apply on a missing theme: got %s (%s), want fixed", r.status.label(), r.detail)
	}
	// the detail must name both sides so the user understands the reset.
	if !strings.Contains(r.detail, "No-Such-Cursor-Theme") || !strings.Contains(r.detail, defaultCursorTheme) {
		t.Errorf("fixed detail must name the missing theme and the default: %q", r.detail)
	}
	raw, _ := os.ReadFile(store)
	if th, sz := configuredCursor(raw); th != defaultCursorTheme || sz != 28 {
		t.Fatalf("after reset the store has %q/%d, want %s/28 (size kept)", th, sz, defaultCursorTheme)
	}
	if r := reconcileCursorTheme(false); r.status != recOK {
		t.Fatalf("idempotent re-run: got %s (%s), want ok", r.status.label(), r.detail)
	}
}

// The sddm greeter (neither owner nor group member of the theme) reads it only
// through the world bits, and the dir must be root-owned. The broken case the
// reconciler heals is a catalogue skin left 0700 user-owned by an old `cp -a`.
func TestGreeterThemeHealthy(t *testing.T) {
	cases := []struct {
		name     string
		uid      uint32
		dir, qml os.FileMode
		want     bool
	}{
		{"root-owned, world-readable (fresh install)", 0, 0o755, 0o644, true},
		{"user-owned 0700 catalogue skin (the bug)", 1000, 0o700, 0o644, false},
		{"user-owned but world-readable: still wrong owner", 1000, 0o755, 0o644, false},
		{"root-owned but dir not traversable by other", 0, 0o700, 0o644, false},
		{"root-owned but Main.qml not world-readable", 0, 0o755, 0o600, false},
		{"root-owned group-only: sddm is other, not group", 0, 0o750, 0o640, false},
	}
	for _, c := range cases {
		if got := greeterThemeHealthy(c.uid, c.dir, c.qml); got != c.want {
			t.Errorf("%s: greeterThemeHealthy(%d, %o, %o) = %v, want %v", c.name, c.uid, c.dir, c.qml, got, c.want)
		}
	}
}

func TestSDDMWaylandBodyForcesQtWayland(t *testing.T) {
	body := sddmWaylandBody()
	for _, line := range []string{
		"[General]",
		"DisplayServer=wayland",
		"GreeterEnvironment=QT_QPA_PLATFORM=wayland,XCURSOR_THEME=Bibata-Modern-Ice,XCURSOR_SIZE=24,QML_XHR_ALLOW_FILE_READ=1",
		"[Wayland]",
		"CompositorCommand=",
		"SessionCommand=",
	} {
		if !strings.Contains(body, line) {
			t.Errorf("sddmWaylandBody() missing %q:\n%s", line, body)
		}
	}
}

// the greeter compositor fallback must pin weston's software cursor path on
// NVIDIA: the driver accepts the hardware cursor plane without displaying it,
// so the login pointer vanishes (#184).
func TestGreeterCompositorSoftwareCursorOnNVIDIA(t *testing.T) {
	wrapper, vendorGlob := greeterCompositorBin, nvidiaVendorGlob
	defer func() { greeterCompositorBin, nvidiaVendorGlob = wrapper, vendorGlob }()

	dir := t.TempDir()
	vendor := filepath.Join(dir, "card0", "device", "vendor")
	if err := os.MkdirAll(filepath.Dir(vendor), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vendor, []byte("0x8086\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nvidiaVendorGlob = filepath.Join(dir, "card*", "device", "vendor")

	greeterCompositorBin = "/nonexistent/greeter"
	if got := greeterCompositor(); got != "weston --shell=kiosk" {
		t.Errorf("greeterCompositor() on Intel = %q", got)
	}

	if err := os.WriteFile(vendor, []byte("0x10de\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := greeterCompositor(); got != "weston --shell=kiosk --renderer=pixman" {
		t.Errorf("greeterCompositor() on NVIDIA = %q", got)
	}
}

// the login-screen pointer break: the greeter and weston fall back to the cursor
// theme literally named "default", and Ryoku shipped none. The stub must be a
// valid index.theme that inherits the shipped Bibata set so the fallback lands
// on a real cursor.
func TestDefaultCursorIndexInheritsShipped(t *testing.T) {
	body := defaultCursorIndexBody()
	for _, line := range []string{
		"[Icon Theme]",
		"Inherits=" + defaultCursorTheme,
	} {
		if !strings.Contains(body, line) {
			t.Errorf("defaultCursorIndexBody() missing %q:\n%s", line, body)
		}
	}
	if defaultCursorTheme != "Bibata-Modern-Ice" {
		t.Errorf("default cursor theme drifted from the shipped Bibata set: %q", defaultCursorTheme)
	}
}

// the limine layout reconciler's decision logic lives in planLimineLayout, a
// pure function of an observed limineLayoutState: no real /boot, no
// efibootmgr. the config surgery (mergeLimineConf) is exercised on literal
// configs shaped like the old installer's shadow file and like
// limine-mkinitcpio-hook's generated tree.
func TestPlanLimineLayout(t *testing.T) {
	treeConf := "timeout: 3\ndefault_entry: 2\n\n/+Ryoku\n    comment: Ryoku\n//linux\n    protocol: efi\n"
	flatConf := "timeout: 3\ndefault_entry: 1\n\n/Ryoku Linux\n    protocol: linux\n"

	cases := []struct {
		name       string
		in         limineLayoutState
		want       limineLayoutOutcome
		wantAction string // substring expected in the actions, empty when none
	}{
		{"no limine package skips", limineLayoutState{}, limineLayoutSkip, ""},
		{"limine installed but no configs under /boot skips",
			limineLayoutState{limineInstalled: true}, limineLayoutSkip, ""},
		{"healthy tool-managed layout reads ok",
			limineLayoutState{limineInstalled: true, espConfExists: true, espConfReadable: true, espConf: treeConf, toolEFIExists: true},
			limineLayoutOK, ""},
		{"shadow config must merge",
			limineLayoutState{limineInstalled: true, espConfExists: true, espConfReadable: true, espConf: treeConf, shadowExists: true, shadowReadable: true, shadowConf: flatConf},
			limineLayoutMigrate, "shadows the generated boot entries"},
		{"shadow without esp conf still merges (offline box)",
			limineLayoutState{limineInstalled: true, shadowExists: true, shadowReadable: true, shadowConf: flatConf},
			limineLayoutMigrate, "merge"},
		{"tree with default_entry 1 repoints the default",
			limineLayoutState{limineInstalled: true, espConfExists: true, espConfReadable: true, espConf: strings.Replace(treeConf, "default_entry: 2", "default_entry: 1", 1)},
			limineLayoutMigrate, "default_entry"},
		{"legacy hand-copied binary is retired",
			limineLayoutState{limineInstalled: true, espConfExists: true, espConfReadable: true, espConf: treeConf, legacyEFIExists: true},
			limineLayoutMigrate, "stale hand-copied bootloader"},
		{"unreadable configs punt to sudo",
			limineLayoutState{limineInstalled: true, espConfExists: true},
			limineLayoutUnreadable, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, actions := planLimineLayout(c.in)
			if got != c.want {
				t.Fatalf("planLimineLayout = %d, want %d (actions=%v)", got, c.want, actions)
			}
			if c.wantAction == "" {
				if len(actions) != 0 {
					t.Errorf("unexpected actions: %v", actions)
				}
				return
			}
			joined := strings.Join(actions, " | ")
			if !strings.Contains(joined, c.wantAction) {
				t.Errorf("actions = %q, want one containing %q", joined, c.wantAction)
			}
		})
	}
}

// mergeLimineConf must (a) never lose generated entries, (b) put the Ryoku
// branding in charge of the globals, (c) keep foreign globals a user or the
// tool added, and (d) pick a bootable default_entry for the resulting menu
// shape.
func TestMergeLimineConf(t *testing.T) {
	shadow := `# Ryoku limine config = global look + branding only.
timeout: 3
default_entry: 1
interface_branding: Ryoku Bootloader
term_background: 171717

/Ryoku Linux
    protocol: linux
    kernel_path: boot():/vmlinuz-linux

# >>> ryoku-windows-entry (managed) >>>
/Windows
    comment: Boot into Windows
    protocol: efi_chainload
    path: uuid(abc):/EFI/Microsoft/Boot/bootmgfw.efi
# <<< ryoku-windows-entry (managed) <<<
`
	tree := `### Read more at the config document
timeout: 3
default_entry: 2
remember_last_entry: yes

/+Ryoku
    comment: Ryoku
//linux
    protocol: efi
    path: boot():/EFI/Linux/ryoku_linux.efi
//+Snapshots
///ID=42 2026-07-01
    protocol: efi
`

	t.Run("tool tree as base keeps entries and snapshots", func(t *testing.T) {
		got := mergeLimineConf(tree, shadow)
		for _, want := range []string{"/+Ryoku", "//+Snapshots", "interface_branding: Ryoku Bootloader", "remember_last_entry: yes", "default_entry: Ryoku/linux"} {
			if !strings.Contains(got, want) {
				t.Errorf("merged config missing %q:\n%s", want, got)
			}
		}
		if strings.Contains(got, "/Ryoku Linux") {
			t.Errorf("flat shadow entry leaked into the tool-managed merge:\n%s", got)
		}
		if strings.Count(got, "default_entry:") != 1 || strings.Count(got, "timeout:") != 1 {
			t.Errorf("branded globals duplicated:\n%s", got)
		}
	})

	t.Run("flat shadow as base stays bootable with default 1", func(t *testing.T) {
		got := mergeLimineConf("", shadow)
		for _, want := range []string{"/Ryoku Linux", "/Windows", "default_entry: 1", "# >>> ryoku-windows-entry (managed) >>>"} {
			if !strings.Contains(got, want) {
				t.Errorf("merged config missing %q:\n%s", want, got)
			}
		}
		if strings.Contains(got, "default_entry: 2") {
			t.Errorf("flat menu must not default past the first entry (Windows would autoboot):\n%s", got)
		}
	})
}

func TestStaleLimineBootNums(t *testing.T) {
	out := `BootCurrent: 0003
Timeout: 1 seconds
BootOrder: 0003,0001,0000
Boot0000* Windows Boot Manager	HD(1,GPT,aaa)/\EFI\Microsoft\Boot\bootmgfw.efi
Boot0001* Limine	HD(1,GPT,bbb)/\EFI\limine\limine_x64.efi
Boot0003* Ryoku	HD(1,GPT,bbb)/\EFI\limine\limine.efi
`
	got := staleLimineBootNums(out)
	if len(got) != 1 || got[0] != "0003" {
		t.Fatalf("staleLimineBootNums = %v, want [0003] (only the legacy limine.efi, never limine_x64.efi)", got)
	}
	if got := staleLimineBootNums(""); len(got) != 0 {
		t.Fatalf("empty efibootmgr output must yield nothing, got %v", got)
	}
}

func TestLimineConfProbes(t *testing.T) {
	tree := "default_entry: 2\n/+Ryoku\n//linux\n"
	flat := "default_entry: 1\n/Ryoku Linux\n    protocol: linux\n"
	if !limineHasBootTree(tree) || limineHasBootTree(flat) {
		t.Error("boot-tree probe must key on the /+ directory marker only")
	}
	if limineDefaultEntry(tree) != "2" || limineDefaultEntry(flat) != "1" || limineDefaultEntry("") != "" {
		t.Error("default_entry probe misparsed")
	}
}

// fastfetchLogoSource lifts the single logo image path out of the JSONC
// config the reconciler keys on. it must read the value verbatim, skip a
// "source" that only appears inside a // comment, and report absence rather
// than guess -- an absent source is how the reconciler recognizes a box with
// no Ryoku fastfetch logo to defend.
func TestFastfetchLogoSource(t *testing.T) {
	cases := []struct {
		name   string
		cfg    string
		want   string
		wantOK bool
	}{
		{
			name: "logo block source is read verbatim",
			cfg: "{\n" +
				"    \"logo\": {\n" +
				"        \"type\": \"kitty-direct\",\n" +
				"        \"source\": \"~/.config/fastfetch/fastfetch-emblem.png\",\n" +
				"        \"width\": 30\n" +
				"    }\n" +
				"}\n",
			want:   "~/.config/fastfetch/fastfetch-emblem.png",
			wantOK: true,
		},
		{
			name: "commented-out source is skipped",
			cfg: "{\n" +
				"    \"logo\": {\n" +
				"        // \"source\": \"~/.config/fastfetch/fastfetch-emblem.png\",\n" +
				"        \"type\": \"builtin\"\n" +
				"    }\n" +
				"}\n",
			wantOK: false,
		},
		{
			name: "no source line",
			cfg: "{\n" +
				"    \"logo\": {\n" +
				"        \"type\": \"builtin\",\n" +
				"        \"width\": 30\n" +
				"    }\n" +
				"}\n",
			wantOK: false,
		},
		{name: "empty config", cfg: "", wantOK: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := fastfetchLogoSource(c.cfg)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v (got source %q)", ok, c.wantOK, got)
			}
			if ok && got != c.want {
				t.Errorf("source = %q, want %q", got, c.want)
			}
		})
	}
}

// expandTilde must resolve a leading ~ against the home dir the way fastfetch
// does at runtime, and leave an already-absolute path untouched. asserting
// against home() (rather than a literal) keeps it robust to how home() is
// resolved while still pinning the mapping: bare ~ -> home, ~/x/y -> joined,
// absolute -> verbatim.
func TestExpandTilde(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	h := sys.Home()
	if got := expandTilde("~"); got != h {
		t.Errorf("expandTilde(\"~\") = %q, want %q", got, h)
	}
	if got, want := expandTilde("~/x/y"), filepath.Join(h, "x/y"); got != want {
		t.Errorf("expandTilde(\"~/x/y\") = %q, want %q", got, want)
	}
	if got := expandTilde("/usr/share/x.png"); got != "/usr/share/x.png" {
		t.Errorf("absolute path must pass through unchanged, got %q", got)
	}
}

// reconcileFastfetchEmblem keeps the branded readout off the stock Arch logo.
// exercised end to end through real IO in temp dirs (config + packaged base
// tree), driven entirely by env so it never touches the real HOME or system
// paths. one subtest per branch of the reconciler's decision.
func TestReconcileFastfetchEmblem(t *testing.T) {
	const emblemSrc = "~/.config/fastfetch/fastfetch-emblem.png"
	baseBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 1, 2, 3}

	type dirs struct{ home, base string }
	setup := func(t *testing.T) dirs {
		home, base := t.TempDir(), t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
		t.Setenv("RYOKU_CONFIG_BASE", base)
		return dirs{home: home, base: base}
	}
	writeConfig := func(t *testing.T, home, source string) {
		dir := filepath.Join(home, ".config", "fastfetch")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		cfg := "{\n    \"logo\": {\n        \"type\": \"kitty-direct\",\n" +
			"        \"source\": \"" + source + "\"\n    }\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "config.jsonc"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeBlob := func(t *testing.T, path string, b []byte) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	baseEmblem := func(d dirs) string { return filepath.Join(d.base, "fastfetch", fastfetchEmblem) }

	t.Run("no config file is a quiet ok", func(t *testing.T) {
		setup(t)
		if r := reconcileFastfetchEmblem(false); r.status != recOK {
			t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
		}
	})

	t.Run("user-customized logo is left alone", func(t *testing.T) {
		d := setup(t)
		writeConfig(t, d.home, "~/.config/fastfetch/my-logo.png")
		writeBlob(t, baseEmblem(d), baseBytes) // present, and must still not be touched
		if r := reconcileFastfetchEmblem(false); r.status != recOK {
			t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
		}
		// the reconciler must create nothing: neither the custom logo it does
		// not own, nor the emblem beside the config.
		if p := expandTilde("~/.config/fastfetch/my-logo.png"); sys.Exists(p) {
			t.Errorf("must not create the user's logo at %s", p)
		}
		if p := expandTilde(emblemSrc); sys.Exists(p) {
			t.Errorf("must not drop an emblem into the config dir at %s", p)
		}
	})

	t.Run("emblem already present resolves ok", func(t *testing.T) {
		d := setup(t)
		writeConfig(t, d.home, emblemSrc)
		writeBlob(t, expandTilde(emblemSrc), baseBytes)
		if r := reconcileFastfetchEmblem(false); r.status != recOK {
			t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
		}
	})

	t.Run("check-only reports the fix without applying it", func(t *testing.T) {
		d := setup(t)
		writeConfig(t, d.home, emblemSrc)
		writeBlob(t, baseEmblem(d), baseBytes)
		r := reconcileFastfetchEmblem(true)
		if r.status != recWouldFix {
			t.Fatalf("status=%s detail=%q, want todo", r.status.label(), r.detail)
		}
		if r.remedy != "ryoku materialize" {
			t.Errorf("remedy = %q, want \"ryoku materialize\"", r.remedy)
		}
		if dst := expandTilde(emblemSrc); sys.Exists(dst) {
			t.Errorf("check-only must not create the emblem at %s", dst)
		}
	})

	t.Run("missing emblem is restored from the base tree", func(t *testing.T) {
		d := setup(t)
		writeConfig(t, d.home, emblemSrc)
		writeBlob(t, baseEmblem(d), baseBytes)
		if r := reconcileFastfetchEmblem(false); r.status != recFixed {
			t.Fatalf("status=%s detail=%q, want fixed", r.status.label(), r.detail)
		}
		dst := expandTilde(emblemSrc)
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("emblem not restored at %s: %v", dst, err)
		}
		if string(got) != string(baseBytes) {
			t.Errorf("restored bytes = %v, want the base copy %v", got, baseBytes)
		}
		// idempotent: with the emblem now present, a second run is a no-op ok.
		if r := reconcileFastfetchEmblem(false); r.status != recOK {
			t.Fatalf("second run status=%s, want ok (idempotent)", r.status.label())
		}
	})

	// The recurring "doctor overwrites my custom fastfetch" report: prove the
	// reconciler only ever restores the emblem PNG and never rewrites config.jsonc.
	t.Run("restoring the emblem never rewrites config.jsonc", func(t *testing.T) {
		d := setup(t)
		writeConfig(t, d.home, emblemSrc)
		writeBlob(t, baseEmblem(d), baseBytes)
		cfgPath := filepath.Join(d.home, ".config", "fastfetch", "config.jsonc")
		before, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		if r := reconcileFastfetchEmblem(false); r.status != recFixed {
			t.Fatalf("status=%s, want fixed (emblem restored)", r.status.label())
		}
		after, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Errorf("config.jsonc was modified by the emblem reconciler:\n before=%q\n after=%q", before, after)
		}
	})

	t.Run("base tree lacking the emblem warns to update", func(t *testing.T) {
		d := setup(t)
		writeConfig(t, d.home, emblemSrc)
		// no base emblem: pre-fix package, cure is to pull it first.
		r := reconcileFastfetchEmblem(false)
		if r.status != recWarn {
			t.Fatalf("status=%s detail=%q, want warn", r.status.label(), r.detail)
		}
		if r.remedy != "ryoku update" {
			t.Errorf("remedy = %q, want \"ryoku update\"", r.remedy)
		}
		if dst := expandTilde(emblemSrc); sys.Exists(dst) {
			t.Errorf("nothing to copy: must not create %s", dst)
		}
	})
}

// hasRyokuBootEntry answers the boot-critical guard: does some ACTIVE NVRAM
// entry already boot the package-refreshed limine, so `ryoku update` may retire
// the legacy entry / the healing reconciler can stand down? "present" means an
// active entry (BootXXXX* ...) either loads \limine_x64.efi or carries the
// limine-install label "Limine" (VenHw, no file path). The legacy \limine.efi
// entry, inactive entries, foreign entries, and empty output must all read NOT
// present. Fixtures mirror real efibootmgr: a header block, then one entry per
// line with a TAB between label and device path.
func TestHasRyokuBootEntry(t *testing.T) {
	const header = "BootCurrent: 0004\nTimeout: 3\nBootOrder: 0004,0002\n"
	const legacy = "Boot0003* Ryoku\tHD(1,GPT,abcd)/File(\\EFI\\limine\\limine.efi)\n"
	cases := []struct {
		name       string
		efibootmgr string
		want       bool
	}{
		{
			"limine-install VenHw entry, label Limine, no file path",
			header + "Boot0004* Limine\tVenHw(99e275e7-75a0-4b37-a2e6-c5385e6c00cb)\n",
			true,
		},
		{
			"installer Ryoku entry loading limine_x64.efi",
			header + "Boot0002* Ryoku\tHD(1,GPT,abcd)/File(\\EFI\\limine\\limine_x64.efi)\n",
			true,
		},
		{
			"only the legacy limine.efi entry: migration owns it, not present",
			header + legacy,
			false,
		},
		{
			"inactive limine entry only (no *): not active, not present",
			header + "Boot0005  Limine\tVenHw(99e275e7-75a0-4b37-a2e6-c5385e6c00cb)\n",
			false,
		},
		{
			"only a foreign Windows Boot Manager entry",
			header + "Boot0000* Windows Boot Manager\tHD(1,GPT,aaa)/File(\\EFI\\Microsoft\\Boot\\bootmgfw.efi)\n",
			false,
		},
		{"empty efibootmgr output", "", false},
	}
	for _, c := range cases {
		if got := hasRyokuBootEntry(c.efibootmgr); got != c.want {
			t.Errorf("%s: hasRyokuBootEntry = %v, want %v", c.name, got, c.want)
		}
	}

	// the legacy-only fixture is exactly what staleLimineBootNums must claim, so
	// the layout migration (not the healing reconciler) converts it.
	if got := staleLimineBootNums(header + legacy); len(got) != 1 || got[0] != "0003" {
		t.Errorf("staleLimineBootNums(legacy) = %v, want [0003]", got)
	}
}

// limineBootLabel extracts the label field of an efibootmgr line: the text
// after BootXXXX and an optional *, up to the first TAB (space-separated
// fallback when there is no tab). A label with spaces must survive intact
// (run to the tab, not the first space), and a non-entry header line must yield
// "" because isHex4(line[4:8]) fails.
func TestLimineBootLabel(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"limine-install label", "Boot0004* Limine\tVenHw(99e275e7-75a0-4b37-a2e6-c5385e6c00cb)", "Limine"},
		{"installer label", "Boot0002* Ryoku\tHD(1,GPT,abcd)/File(\\EFI\\limine\\limine_x64.efi)", "Ryoku"},
		{"spaced label runs to the tab", "Boot0000* Windows Boot Manager\tHD(1,GPT,aaa)/File(\\EFI\\Microsoft\\Boot\\bootmgfw.efi)", "Windows Boot Manager"},
		{"space-separated fallback, no tab", "Boot0006* Limine VenHw(x)", "Limine"},
		{"BootOrder header is not an entry", "BootOrder: 0004,0002", ""},
		{"BootCurrent header is not an entry", "BootCurrent: 0004", ""},
	}
	for _, c := range cases {
		if got := limineBootLabel(c.line); got != c.want {
			t.Errorf("%s: limineBootLabel(%q) = %q, want %q", c.name, c.line, got, c.want)
		}
	}
}

// parseEspDiskPart derives the efibootmgr --disk/--part pair the boot-entry
// writer needs. It trims all three inputs and refuses (ok=false) unless the
// mount source is under /dev/ and both the parent-disk name and partition
// number are non-empty; on success the disk is "/dev/"+pkname and the part is
// the trimmed partition number.
func TestParseEspDiskPart(t *testing.T) {
	cases := []struct {
		name                 string
		source, pkname, part string
		wantDisk, wantPart   string
		wantOK               bool
	}{
		{"nvme, part has trailing newline", "/dev/nvme0n1p1", "nvme0n1", "1\n", "/dev/nvme0n1", "1", true},
		{"sata, padded part number", "/dev/sda2", "sda", " 2 ", "/dev/sda", "2", true},
		{"source not under /dev", "mapper/foo", "sda", "1", "", "", false},
		{"empty pkname", "/dev/sda1", "", "1", "", "", false},
		{"empty partition", "/dev/sda1", "sda", "", "", "", false},
	}
	for _, c := range cases {
		disk, part, ok := parseEspDiskPart(c.source, c.pkname, c.part)
		if ok != c.wantOK || disk != c.wantDisk || part != c.wantPart {
			t.Errorf("%s: parseEspDiskPart(%q,%q,%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.name, c.source, c.pkname, c.part, disk, part, ok, c.wantDisk, c.wantPart, c.wantOK)
		}
	}
}

// hyprSetFollowMouse must flip only desktop.input.followMouse and preserve every
// other field.
func TestHyprFollowMouseRewrite(t *testing.T) {
	raw := `{"desktop":{"input":{"kbLayout":"us","followMouse":1,"tapToClick":true},"appearance":{"gapsIn":5}}}`
	if fm, ok := hyprGetFollowMouse(raw); !ok || fm != 1 {
		t.Fatalf("get: got (%d,%v), want (1,true)", fm, ok)
	}
	fixed, err := hyprSetFollowMouse(raw, 2)
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if fm, ok := hyprGetFollowMouse(fixed); !ok || fm != 2 {
		t.Errorf("after set: got (%d,%v), want (2,true)", fm, ok)
	}
	// untouched fields survive (the store is written indented).
	for _, want := range []string{`"kbLayout": "us"`, `"tapToClick": true`, `"gapsIn": 5`} {
		if !strings.Contains(fixed, want) {
			t.Errorf("healed JSON dropped %s: %s", want, fixed)
		}
	}
}

// A config that never held the retired default (or has no followMouse) is a no-op.
func TestHyprFollowMouseNotDefault(t *testing.T) {
	if fm, ok := hyprGetFollowMouse(`{"desktop":{"input":{"followMouse":2}}}`); !ok || fm != 2 {
		t.Errorf("followMouse=2: got (%d,%v)", fm, ok)
	}
	if _, ok := hyprGetFollowMouse(`{"desktop":{"input":{}}}`); ok {
		t.Errorf("missing followMouse should report absent")
	}
}

func TestMigrateShellConfig(t *testing.T) {
	legacy := []byte(`{
        "barEnabled": false, "barPosition": "bottom", "barHeight": 26, "atollVariant": "ryoku",
        "sidebarLeftPanes": ["stash"], "sidebarRightPanes": ["weather", "calendar", "media"], "sidebarWidth": 360
    }`)
	out, changes, err := migrateShellConfig(legacy)
	if err != nil || len(changes) == 0 {
		t.Fatalf("legacy file should migrate: changes=%v err=%v", changes, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("migrated JSON does not parse: %v", err)
	}
	frameBars, ok := cfg["frameBars"].(map[string]any)
	if !ok {
		t.Fatal("legacy settings must seed frameBars")
	}
	rails := frameBars["rails"].(map[string]any)
	if top := rails["top"].(map[string]any); top["enabled"].(bool) {
		t.Error("legacy settings must keep the empty top reference rail disabled")
	}
	if left := rails["left"].(map[string]any); !left["enabled"].(bool) {
		t.Error("legacy settings must seed the left reference rail")
	}
	surfaces := frameBars["surfaces"].(map[string]any)
	stash := surfaces["stash"].(map[string]any)
	if stash["anchor"] != "right" {
		t.Errorf("stash anchor was not normalized: %v", stash["anchor"])
	}
	if stash["minWidth"].(float64) != 360 {
		t.Errorf("stash width was not preserved: %v", stash["minWidth"])
	}
	if got := stash["panes"]; !reflect.DeepEqual(got, []any{"stash"}) {
		t.Errorf("left sidebar panes were not preserved: %v", got)
	}
	if _, present := surfaces["system"]; present {
		t.Error("retired system sidebar was recreated")
	}
	for _, key := range []string{"sidebarLeftPanes", "sidebarRightPanes", "sidebarWidth"} {
		if _, ok := cfg[key]; ok {
			t.Errorf("sidebar-only key %s survived migration", key)
		}
	}
	for _, key := range retiredShellKeys {
		if _, ok := cfg[key]; ok {
			t.Errorf("retired key %s survived migration", key)
		}
	}
	out, changes, err = migrateShellConfig(out)
	if err != nil || out != nil || changes != nil {
		t.Fatalf("legacy migration must be idempotent: out=%s changes=%v err=%v", out, changes, err)
	}

	malformed := []byte(`{
        "frameBars": {
            "style": "bad",
            "rails": {
                "top": { "size": 2, "start": ["tray", "tray", "dock", "unknown"] },
                "left": { "size": 999, "top": ["clock", "clock", "unknown"] }
            },
            "menus": { "quick-settings": { "anchor": "wrong", "modules": ["media", "future-module", "media"] } },
            "surfaces": { "stash": { "anchor": "wrong" } }
        }
    }`)
	out, changes, err = migrateShellConfig(malformed)
	if err != nil || len(changes) == 0 {
		t.Fatalf("malformed frameBars should normalize: changes=%v err=%v", changes, err)
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("normalized JSON does not parse: %v", err)
	}
	frameBars = cfg["frameBars"].(map[string]any)
	rails = frameBars["rails"].(map[string]any)
	top := rails["top"].(map[string]any)
	left := rails["left"].(map[string]any)
	if top["size"].(float64) != 16 || left["size"].(float64) != 112 {
		t.Errorf("rail sizes were not clamped: top=%v left=%v", top["size"], left["size"])
	}
	if got := top["start"]; !reflect.DeepEqual(got, []any{"tray"}) {
		t.Errorf("horizontal ids were not normalized: %v", got)
	}
	if got := left["top"]; !reflect.DeepEqual(got, []any{"clock"}) {
		t.Errorf("vertical ids were not normalized: %v", got)
	}
	menus := frameBars["menus"].(map[string]any)
	if menus["quick-settings"].(map[string]any)["anchor"] != "left" {
		t.Errorf("menu anchor was not normalized: %v", menus)
	}
	surfaces = frameBars["surfaces"].(map[string]any)
	if surfaces["stash"].(map[string]any)["anchor"] != "right" {
		t.Errorf("surface anchor was not normalized: %v", surfaces)
	}
	if got := menus["quick-settings"].(map[string]any)["modules"]; !reflect.DeepEqual(got, []any{"media", "future-module"}) {
		t.Errorf("quick-settings module configuration was not preserved: %v", got)
	}

	out, changes, err = migrateShellConfig(out)
	if err != nil || out != nil || changes != nil {
		t.Fatalf("normalized frameBars must be idempotent: out=%s changes=%v err=%v", out, changes, err)
	}
	if _, _, err := migrateShellConfig([]byte("not json")); err == nil {
		t.Fatal("garbage must error, not silently rewrite")
	}
}

// A store carrying the pre-parity menu shape (the retired `launcher` menu id and
// a stale quick-settings widget list) converges: the launcher key is dropped and
// the quick-settings stack resolves to its fixed widget, so the shell re-seeds
// the reference menus from defaults on the next read.
func TestMigrateShellConfigConvergesMenus(t *testing.T) {
	before := []byte(`{
        "frameBars": {
            "style": "slate-frame",
            "menus": {
                "launcher": { "anchor": "left", "minWidth": 420, "expansion": "always", "widgets": ["launcher"] },
                "quick-settings": { "anchor": "left", "minWidth": 410, "expansion": "always", "widgets": ["clock", "network", "audio-output"] },
                "clock": { "anchor": "top", "minWidth": 280, "expansion": "never", "widgets": ["clock"] }
            }
        }
    }`)
	out, changes, err := migrateShellConfig(before)
	if err != nil || len(changes) == 0 {
		t.Fatalf("pre-parity menus should converge: changes=%v err=%v", changes, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("converged JSON does not parse: %v", err)
	}
	menus := cfg["frameBars"].(map[string]any)["menus"].(map[string]any)
	if _, ok := menus["launcher"]; ok {
		t.Errorf("retired launcher menu id survived migration: %v", menus)
	}
	qs, ok := menus["quick-settings"].(map[string]any)
	if !ok {
		t.Fatalf("quick-settings menu missing after convergence: %v", menus)
	}
	if got := qs["widgets"]; !reflect.DeepEqual(got, []any{"quick-settings"}) {
		t.Errorf("quick-settings widgets did not converge to the fixed stack: %v", got)
	}
	if out2, changes2, err := migrateShellConfig(out); err != nil || out2 != nil || changes2 != nil {
		t.Fatalf("menu convergence must be idempotent: out=%s changes=%v err=%v", out2, changes2, err)
	}
}

// A box upgrading from any Atoll-era release carries the whole retired set, not
// just the four geometry keys. Every one of them must go, and the settings the
// shell still reads must survive untouched.
func TestMigrateShellConfigDropsEveryRetiredKey(t *testing.T) {
	cfg := map[string]any{"fontScale": 0.96, "language": "Auto"}
	for _, key := range retiredShellKeys {
		cfg[key] = "carried"
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, changes, err := migrateShellConfig(raw)
	if err != nil || len(changes) == 0 {
		t.Fatalf("retired keys should migrate: changes=%v err=%v", changes, err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("migrated JSON does not parse: %v", err)
	}
	for _, key := range retiredShellKeys {
		if _, ok := got[key]; ok {
			t.Errorf("retired key %s survived migration", key)
		}
	}
	for key, want := range map[string]any{"fontScale": 0.96, "language": "Auto"} {
		if got[key] != want {
			t.Errorf("live setting %s = %v, want %v", key, got[key], want)
		}
	}
	if _, ok := got["frameBars"]; !ok {
		t.Error("migration must leave a frame-bars object behind")
	}
	if out, changes, err := migrateShellConfig(out); err != nil || out != nil || changes != nil {
		t.Fatalf("retired-key migration must be idempotent: out=%s changes=%v err=%v", out, changes, err)
	}
}

// The pluggable bar-style selector must survive migration and default to the
// shipped QS Bar: reconcileShellConfig no longer strips the live barStyle, and
// reconcileSumiBar defaults an absent or retired Atoll-era style to "qsbar" (not
// the old "sumi"), so an existing box lands on the current default bar on update
// while an explicit sumi/store choice is kept.
func TestBarStyleDefaultsToQsbar(t *testing.T) {
	barStyleOf := func(raw []byte) string {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("output does not parse: %v", err)
		}
		s, _ := m["barStyle"].(string)
		return s
	}
	// migrateShellConfig must not strip the live selector while shedding Atoll keys.
	out, _, err := migrateShellConfig([]byte(`{"barStyle":"qsbar","atollVariant":"x"}`))
	if err != nil {
		t.Fatalf("migrateShellConfig: %v", err)
	}
	if got := barStyleOf(out); got != "qsbar" {
		t.Errorf("migrateShellConfig dropped the live barStyle: got %q, want qsbar", got)
	}
	cases := []struct{ name, in, want string }{
		{"qsbar kept", `{"barStyle":"qsbar"}`, "qsbar"},
		{"sumi kept", `{"barStyle":"sumi"}`, "sumi"},
		{"absent defaults to qsbar", `{"fontScale":1.3}`, "qsbar"},
		{"retired atoll to qsbar", `{"barStyle":"atoll"}`, "qsbar"},
		{"retired washi to qsbar", `{"barStyle":"washi"}`, "qsbar"},
	}
	for _, c := range cases {
		got, changed, err := migrateSumiBar([]byte(c.in))
		if err != nil {
			t.Fatalf("%s: migrateSumiBar: %v", c.name, err)
		}
		result := c.in
		if changed {
			result = string(got)
		}
		if bs := barStyleOf([]byte(result)); bs != c.want {
			t.Errorf("%s: barStyle = %q, want %q", c.name, bs, c.want)
		}
	}
}

// migrateDockStore lifts the retired qsbar.dock* knobs into a top-level dock
// object once the dock became its own shell surface. It must move exactly the
// five persisted knobs under their new names, drop them from qsbar, never
// clobber a dock object the shell already wrote, leave the shell-defaulted keys
// absent, and be idempotent so a second doctor run reads clean.
func TestMigrateDockStore(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		changed bool
	}{
		{"absent both", `{"barStyle":"qsbar","qsbar":{"barGapTop":3}}`, false},
		{"qsbar keys only", `{"qsbar":{"dockEnabled":true,"dockMagnify":false,"dockPinned":["kitty.desktop"],"dockFrost":true,"dockShadow":false,"barGapTop":3}}`, true},
		{"both present", `{"dock":{"enabled":false,"edge":"top"},"qsbar":{"dockEnabled":true,"dockPinned":["kitty.desktop"],"barGapTop":3}}`, true},
		{"already migrated", `{"dock":{"enabled":true,"pinned":["kitty.desktop"]},"qsbar":{"barGapTop":3}}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, changed, err := migrateDockStore([]byte(c.in))
			if err != nil {
				t.Fatalf("migrateDockStore: %v", err)
			}
			if changed != c.changed {
				t.Fatalf("changed = %v, want %v", changed, c.changed)
			}
			if !changed {
				return
			}
			var cfg map[string]any
			if err := json.Unmarshal(out, &cfg); err != nil {
				t.Fatalf("migrated JSON does not parse: %v", err)
			}
			qsbar, _ := cfg["qsbar"].(map[string]any)
			for old := range oldDockKeys {
				if _, present := qsbar[old]; present {
					t.Errorf("retired key %q survived in qsbar", old)
				}
			}
			if qsbar["barGapTop"].(float64) != 3 {
				t.Errorf("qsbar lost an unrelated key: %v", qsbar)
			}
			// idempotent: the migrated store is now a no-op.
			if _, again, err := migrateDockStore(out); err != nil || again {
				t.Errorf("re-migrating must be a no-op: changed=%v err=%v", again, err)
			}
		})
	}

	// qsbar-keys-only: every knob lands in the new dock object under its new name.
	out, _, err := migrateDockStore([]byte(`{"qsbar":{"dockEnabled":true,"dockMagnify":false,"dockPinned":["kitty.desktop"],"dockFrost":true,"dockShadow":false}}`))
	if err != nil {
		t.Fatalf("migrateDockStore: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("migrated JSON does not parse: %v", err)
	}
	dock := cfg["dock"].(map[string]any)
	if dock["enabled"] != true || dock["magnify"] != false || dock["frost"] != true || dock["shadow"] != false {
		t.Errorf("dock knobs did not move correctly: %v", dock)
	}
	if pins, ok := dock["pinned"].([]any); !ok || len(pins) != 1 || pins[0] != "kitty.desktop" {
		t.Errorf("dockPinned did not move to dock.pinned: %v", dock["pinned"])
	}
	// the keys the shell defaults stay absent, so its Config.qml default applies.
	for _, absent := range []string{"edge", "autohide", "labels", "media"} {
		if _, present := dock[absent]; present {
			t.Errorf("migration invented a %q key; the shell default must apply", absent)
		}
	}

	// both-present: a dock object the shell already wrote is never clobbered, but
	// the keys it lacks are still filled from qsbar and the old keys are dropped.
	out, _, err = migrateDockStore([]byte(`{"dock":{"enabled":false,"edge":"top"},"qsbar":{"dockEnabled":true,"dockPinned":["kitty.desktop"]}}`))
	if err != nil {
		t.Fatalf("migrateDockStore: %v", err)
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("migrated JSON does not parse: %v", err)
	}
	dock = cfg["dock"].(map[string]any)
	if dock["enabled"] != false {
		t.Errorf("existing dock.enabled was clobbered: %v", dock["enabled"])
	}
	if dock["edge"] != "top" {
		t.Errorf("existing dock.edge was lost: %v", dock["edge"])
	}
	if pins, ok := dock["pinned"].([]any); !ok || len(pins) != 1 {
		t.Errorf("dock.pinned was not filled from qsbar: %v", dock["pinned"])
	}
	if q := cfg["qsbar"].(map[string]any); q["dockEnabled"] != nil {
		t.Errorf("qsbar dockEnabled was not deleted: %v", q)
	}

	// garbage errors rather than silently rewriting.
	if _, _, err := migrateDockStore([]byte("not json")); err == nil {
		t.Fatal("garbage must error, not silently rewrite")
	}
}

// limineDropFlat mirrors the installer's promote surgery: flat placeholder
// entries go (with their indented options), default_entry moves off the tree
// directory, globals and the /+ tree survive untouched.
func TestLimineDropFlat(t *testing.T) {
	conf := "timeout: 3\n" +
		"default_entry: 1\n" +
		"/Ryoku Linux\n" +
		"    protocol: linux\n" +
		"    kernel_path: boot():/vmlinuz-linux\n" +
		"/Ryoku Linux (CachyOS)\n" +
		"    protocol: linux\n" +
		"/+Ryoku\n" +
		"//linux (UKI)\n" +
		"    protocol: efi\n"
	out, changed := limineDropFlat(conf)
	if !changed {
		t.Fatal("flat placeholders present; must report a change")
	}
	if strings.Contains(out, "/Ryoku Linux") {
		t.Errorf("flat entries survived:\n%s", out)
	}
	if strings.Contains(out, "kernel_path: boot():/vmlinuz-linux\n") {
		t.Errorf("flat entry options survived:\n%s", out)
	}
	if !strings.Contains(out, "default_entry: 2") {
		t.Errorf("default_entry not promoted past the tree directory:\n%s", out)
	}
	if !strings.Contains(out, "/+Ryoku") || !strings.Contains(out, "//linux (UKI)") || !strings.Contains(out, "timeout: 3") {
		t.Errorf("tree or globals damaged:\n%s", out)
	}

	if _, changed := limineDropFlat(out); changed {
		t.Error("promoted config must be a fixed point")
	}
	if !limineHasUKITree(out) || limineHasUKITree("timeout: 3\n/Ryoku Linux\n") {
		t.Error("limineHasUKITree misreads the tree marker")
	}
}

// 1.37+ limine-entry-tool adopts the flat placeholder as the tree root: the
// entry must survive promotion, only default_entry moves off the directory.
func TestLimineDropFlatAdoptedLayout(t *testing.T) {
	conf := "default_entry: 1\n" +
		"/Ryoku Linux\n" +
		"    kernel_path: boot():/vmlinuz-linux\n" +
		"\n" +
		"  //linux\n" +
		"  protocol: efi\n" +
		"     //Snapshots\n"
	out, changed := limineDropFlat(conf)
	if !changed {
		t.Fatal("default_entry: 1 on a directory must count as a change")
	}
	if !strings.Contains(out, "/Ryoku Linux") || !strings.Contains(out, "kernel_path: boot():/vmlinuz-linux") {
		t.Errorf("adopted tree root must survive:\n%s", out)
	}
	if !strings.Contains(out, "default_entry: 2") {
		t.Errorf("default_entry not moved off the directory:\n%s", out)
	}
	if _, changed := limineDropFlat(out); changed {
		t.Error("promoted adopted layout must be a fixed point")
	}
	if !limineHasUKITree(conf) {
		t.Error("indented //kernel children must count as a tree")
	}
}

// The hijack the portal-routing reconciler heals: a niri-era generic
// portals.conf routing to the gnome backend, which hangs app launches under
// Hyprland. hyprland anywhere in the default list means the file is intent,
// not residue.
func TestPortalRoutesBackend(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"packaged hyprland config", "[preferred]\ndefault=hyprland;gtk\n", true},
		{"niri leftover", "[preferred]\ndefault=gnome;gtk;\n", false},
		{"spaces around the key", "[preferred]\ndefault = hyprland\n", true},
		{"hyprland last still counts", "[preferred]\ndefault=gtk;hyprland\n", true},
		{"per-interface tweak without default", "[preferred]\norg.freedesktop.impl.portal.FileChooser=gtk\n", false},
		{"wildcard is not a route", "[preferred]\ndefault=*\n", false},
		{"default outside preferred", "[other]\ndefault=hyprland\n", false},
		{"substring must not match", "[preferred]\ndefault=hyprland-fork\n", false},
		{"empty file", "", false},
	}
	for _, c := range cases {
		if got := portalRoutesBackend(c.in, "hyprland"); got != c.want {
			t.Errorf("%s: portalRoutesBackend = %v, want %v", c.name, got, c.want)
		}
	}
}

// The precedence claim the reconciler rests on (portals.conf(5)): a user-level
// generic portals.conf outranks the packaged hyprland-portals.conf, and within
// one directory the desktop-specific name is read first.
func TestPortalConfigCandidatesOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_DIRS", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_DATA_DIRS", "")
	got := portalConfigCandidates(home, "hyprland")
	idx := func(p string) int {
		for i, c := range got {
			if c == p {
				return i
			}
		}
		t.Fatalf("candidate %s missing from %v", p, got)
		return -1
	}
	userSpecific := filepath.Join(home, ".config/xdg-desktop-portal/hyprland-portals.conf")
	userGeneric := filepath.Join(home, ".config/xdg-desktop-portal/portals.conf")
	if idx(userSpecific) != 0 || idx(userGeneric) != 1 {
		t.Errorf("user config must lead the order, got %v", got[:2])
	}
	if idx(userGeneric) > idx("/usr/share/xdg-desktop-portal/hyprland-portals.conf") {
		t.Error("a user-level generic portals.conf must outrank the packaged hyprland one")
	}
	if idx("/etc/xdg-desktop-portal/portals.conf") > idx("/usr/share/xdg-desktop-portal/hyprland-portals.conf") {
		t.Error("an /etc-level portals.conf must outrank the packaged hyprland one")
	}
}

// The desktop token drives which <desktop>-portals.conf the portal loads: the
// running session's first XDG_CURRENT_DESKTOP entry wins, the provider name is
// the fallback, and the packaged candidate is named for that token (so a niri
// box looks for niri-portals.conf, not hyprland's).
func TestPortalDesktopToken(t *testing.T) {
	// table-driven so the desktop names read as data, not as a branch on which
	// compositor is running (the isolation gate's whole point).
	cases := []struct {
		env, provider, want string
	}{
		{"niri:wayland", "niri", "niri"},
		{"GNOME", "gnome", "gnome"},
		{"", "Niri", "niri"}, // empty session env falls back to the provider
		{"", "", ""},         // neither: nothing desktop-specific to look for
	}
	for _, c := range cases {
		if got := portalDesktopToken(c.env, c.provider); got != c.want {
			t.Errorf("portalDesktopToken(%q, %q) = %q, want %q", c.env, c.provider, got, c.want)
		}
	}
}

// A niri session's packaged candidate is niri-portals.conf, and it outranks the
// generic portals.conf in the same directory (portals.conf(5) order).
func TestPortalConfigCandidatesNiri(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_DIRS", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_DATA_DIRS", "")
	got := portalConfigCandidates(home, "niri")
	idx := func(p string) int {
		for i, c := range got {
			if c == p {
				return i
			}
		}
		t.Fatalf("candidate %s missing from %v", p, got)
		return -1
	}
	if idx(filepath.Join(home, ".config/xdg-desktop-portal/niri-portals.conf")) != 0 {
		t.Error("the user niri-portals.conf must lead the order")
	}
	if idx("/usr/share/xdg-desktop-portal/niri-portals.conf") > idx("/usr/share/xdg-desktop-portal/portals.conf") {
		t.Error("the packaged niri-portals.conf must outrank the generic one")
	}
	// an empty token yields only the generic candidate, never a bare -portals.conf.
	for _, c := range portalConfigCandidates(home, "") {
		if strings.HasSuffix(c, "/-portals.conf") {
			t.Fatalf("empty token produced %s", c)
		}
	}
}

// End to end on a temp home: the winning user file routes to gnome, doctor in
// fix mode moves it aside and the packaged-style file behind it wins again.
func TestReconcilePortalRoutingHealsUserHijack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(home, "empty-etc-xdg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	data := filepath.Join(home, "data")
	t.Setenv("XDG_DATA_DIRS", data)
	// A fake provider reporting a portal backend, resolved via RYOKU_WM; PATH
	// holds only it, so the fix's systemctl nudge never reaches a live session.
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	caps := "#!/bin/sh\n[ \"$1\" = caps ] && echo '{\"name\":\"hyprland\",\"portalBackend\":\"hyprland\",\"supports\":[],\"workspaceModel\":\"fixed\"}'\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-wm-hyprland"), []byte(caps), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("RYOKU_WM", "hyprland")
	// no session env: the token falls back to the provider name (hyprland).
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	userDir := filepath.Join(home, ".config/xdg-desktop-portal")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hijack := filepath.Join(userDir, "portals.conf")
	if err := os.WriteFile(hijack, []byte("[preferred]\ndefault=gnome;gtk;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	packaged := filepath.Join(data, "xdg-desktop-portal")
	if err := os.MkdirAll(packaged, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packaged, "hyprland-portals.conf"),
		[]byte("[preferred]\ndefault=hyprland;gtk\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if r := reconcilePortalRouting(true); r.status != recWouldFix {
		t.Fatalf("check mode on a hijacked box = %q (%s), want would-fix", r.status.label(), r.detail)
	}
	if r := reconcilePortalRouting(false); r.status != recFixed {
		t.Fatalf("fix mode = %q (%s), want fixed", r.status.label(), r.detail)
	}
	if sys.Exists(hijack) {
		t.Error("hijacking portals.conf still in place after the fix")
	}
	if !sys.Exists(hijack + ".ryoku-bak") {
		t.Error("hijacking portals.conf must be kept as .ryoku-bak, not deleted")
	}
	if r := reconcilePortalRouting(true); r.status != recOK {
		t.Errorf("healed box must be ok, got %q: %s", r.status.label(), r.detail)
	}
}

// A niri box routes portals through the packaged niri-portals.conf (gnome
// backend, FileChooser to gtk). Before the candidate list was desktop-aware the
// doctor only ever looked for hyprland-portals.conf, so it could not see this
// file and falsely warned that nothing routes to gnome. The session env names
// the desktop, so the token is niri and the packaged file reads as healthy.
func TestReconcilePortalRoutingNiriPackaged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(home, "empty-etc-xdg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	data := filepath.Join(home, "data")
	t.Setenv("XDG_DATA_DIRS", data)
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	caps := "#!/bin/sh\n[ \"$1\" = caps ] && echo '{\"name\":\"niri\",\"portalBackend\":\"gnome\",\"supports\":[],\"workspaceModel\":\"dynamic\"}'\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-wm-niri"), []byte(caps), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("RYOKU_WM", "niri")
	t.Setenv("XDG_CURRENT_DESKTOP", "niri")
	packaged := filepath.Join(data, "xdg-desktop-portal")
	if err := os.MkdirAll(packaged, 0o755); err != nil {
		t.Fatal(err)
	}
	// the shipped routing: gnome default, FileChooser pinned to gtk.
	if err := os.WriteFile(filepath.Join(packaged, "niri-portals.conf"),
		[]byte("[preferred]\ndefault=gnome;gtk\norg.freedesktop.impl.portal.FileChooser=gtk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcilePortalRouting(true); r.status != recOK {
		t.Fatalf("packaged niri box = %q (%s), want ok", r.status.label(), r.detail)
	}
}

// limineAdoptedDirty mirrors a live /boot/limine.conf after limine-entry-tool
// 1.37+ adopts the flat "/Ryoku Linux" placeholder as the menu directory: the
// placeholder's boot stanza (protocol/kernel_path/cmdline/module_path) is left
// wedged between the directory title and its first "//" sub-entry, where
// Limine allows only a comment. that malformed "directory that is also a boot
// entry" cannot autoboot, so the timeout countdown restarts forever.
const limineAdoptedDirty = `timeout: 3
default_entry: 2
interface_branding: Ryoku Bootloader

/Ryoku Linux
    protocol: linux
    kernel_path: boot():/vmlinuz-linux
    cmdline: root=UUID=x rw quiet splash
    module_path: boot():/initramfs-linux.img

  //linux
  comment: Kernel version: 7.0.12
  protocol: efi
  path: boot():/EFI/Linux/ryoku_linux.efi

     //Snapshots
     comment: 5 / 5 snapshots

/EFI fallback
    protocol: efi
    path: boot():/EFI/BOOT/BOOTX64.EFI
`

func TestStripLiminePlaceholderBody(t *testing.T) {
	out := stripLiminePlaceholderBody(limineAdoptedDirty)
	if strings.Contains(out, "kernel_path:") || strings.Contains(out, "module_path:") {
		t.Fatalf("placeholder boot stanza not stripped from the directory:\n%s", out)
	}
	// the directory title and every sub-entry survive verbatim.
	for _, want := range []string{
		"/Ryoku Linux",
		"  //linux",
		"comment: Kernel version: 7.0.12",
		"path: boot():/EFI/Linux/ryoku_linux.efi",
		"//Snapshots",
		"/EFI fallback",
		"path: boot():/EFI/BOOT/BOOTX64.EFI",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("strip dropped %q:\n%s", want, out)
		}
	}
	if !limineDirtyRoot(limineAdoptedDirty) {
		t.Error("limineDirtyRoot must flag the adopted directory carrying a boot stanza")
	}
	if limineDirtyRoot(out) {
		t.Error("stripped config must be clean (idempotent)")
	}
	if out2 := stripLiminePlaceholderBody(out); out2 != out {
		t.Error("second strip changed the file (not idempotent)")
	}
}

func TestStripLiminePlaceholderBodyLeavesFlatPlaceholder(t *testing.T) {
	// offline install: "/Ryoku Linux" is a bootable leaf, not a directory (no
	// "//" sub-entry). its boot body is legitimate and must not be touched.
	flat := "timeout: 3\ndefault_entry: 1\n\n/Ryoku Linux\n    protocol: linux\n    kernel_path: boot():/vmlinuz-linux\n    module_path: boot():/initramfs-linux.img\n"
	if got := stripLiminePlaceholderBody(flat); got != flat {
		t.Errorf("flat placeholder must be left intact, got:\n%s", got)
	}
	if limineDirtyRoot(flat) {
		t.Error("flat placeholder is not a dirty directory")
	}
}

func TestPlanLimineLayoutHealsDirtyRoot(t *testing.T) {
	st := limineLayoutState{
		limineInstalled: true,
		espConfExists:   true,
		espConfReadable: true,
		espConf:         limineAdoptedDirty,
	}
	outcome, actions := planLimineLayout(st)
	if outcome != limineLayoutMigrate {
		t.Fatalf("outcome = %v, want migrate", outcome)
	}
	if !strings.Contains(strings.Join(actions, "; "), "strip the leftover boot stanza") {
		t.Errorf("plan missing the strip action: %v", actions)
	}
}

func TestMergeLimineConfHealsAdoptedRoot(t *testing.T) {
	merged := mergeLimineConf(limineAdoptedDirty, "")
	if strings.Contains(merged, "kernel_path:") {
		t.Errorf("merge left the placeholder boot stanza:\n%s", merged)
	}
	for _, want := range []string{
		"default_entry: Ryoku Linux/linux", // entry path into the boot directory autoboots the kernel
		"  //linux",
		"interface_branding: Ryoku Bootloader",
		"/EFI fallback",
	} {
		if !strings.Contains(merged, want) {
			t.Errorf("merged config missing %q:\n%s", want, merged)
		}
	}
}

// TestLimineEnsureAutoboot covers the countdown-loop fix on the real adopted
// layout: default_entry: 2 lands on the /EFI fallback (which re-launches Limine),
// so it must become the kernel's entry path, with remember_last_entry enabled.
func TestLimineEnsureAutoboot(t *testing.T) {
	const conf = `timeout: 3
default_entry: 2
interface_branding: Ryoku Bootloader

/Ryoku Linux
  //linux
  protocol: efi
  path: boot():/EFI/Linux/ryoku_linux.efi

     //Snapshots
     comment: 5 / 5 snapshots

/EFI fallback
    protocol: efi
    path: boot():/EFI/BOOT/BOOTX64.EFI

/Windows
    protocol: efi_chainload
`
	if p := limineFirstKernelPath(conf); p != "Ryoku Linux/linux" {
		t.Fatalf("first kernel path = %q, want Ryoku Linux/linux", p)
	}
	got, changed := limineEnsureAutoboot(conf)
	if !changed {
		t.Fatal("expected a change: default_entry: 2 loops on the EFI fallback")
	}
	if !strings.Contains(got, "default_entry: Ryoku Linux/linux") {
		t.Errorf("default_entry not repointed at the kernel path:\n%s", got)
	}
	if strings.Contains(got, "default_entry: 2") {
		t.Errorf("the looping default_entry: 2 is still present:\n%s", got)
	}
	if !strings.Contains(got, "remember_last_entry: yes") {
		t.Errorf("remember_last_entry not enabled:\n%s", got)
	}
	if again, changed2 := limineEnsureAutoboot(got); changed2 || again != got {
		t.Error("limineEnsureAutoboot is not idempotent")
	}
	// a flat menu (no nested kernel) keeps default_entry: 1, the bootable placeholder.
	const flat = "timeout: 3\ndefault_entry: 2\n\n/Ryoku Linux\n    protocol: linux\n    path: boot():/vmlinuz-linux\n"
	if p := limineFirstKernelPath(flat); p != "" {
		t.Errorf("flat menu should have no nested kernel path, got %q", p)
	}
	if fg, _ := limineEnsureAutoboot(flat); !strings.Contains(fg, "default_entry: 1") {
		t.Errorf("flat menu should default to entry 1:\n%s", fg)
	}
}

func TestReconcileStaleUpdateRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	path := filepath.Join(dir, "ryoku-update.json")

	// hermetic: a dev box's real `ryoku update` (or a sandbox where process
	// scans stall) must not steer the result.
	prev := updateProcessLive
	updateProcessLive = func() bool { return false }
	t.Cleanup(func() { updateProcessLive = prev })

	// no run-state at all: nothing to do.
	if res := reconcileStaleUpdateRun(true); res.status != recOK {
		t.Errorf("missing file = %v (%s), want ok", res.status, res.detail)
	}

	// a settled phase is left alone.
	if err := os.WriteFile(path, []byte(`{"phase":"idle"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if res := reconcileStaleUpdateRun(true); res.status != recOK {
		t.Errorf("idle phase = %v (%s), want ok", res.status, res.detail)
	}

	// a crashed run (phase running, no live `ryoku update`) is flagged...
	if err := os.WriteFile(path, []byte(`{"phase":"running","label":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if res := reconcileStaleUpdateRun(true); res.status != recWouldFix {
		t.Errorf("stale running = %v (%s), want would-fix", res.status, res.detail)
	}
	// ...and idled in place by the fix pass.
	if res := reconcileStaleUpdateRun(false); res.status != recFixed {
		t.Errorf("fix pass = %v (%s), want fixed", res.status, res.detail)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Phase string `json:"phase"`
	}
	if json.Unmarshal(b, &st) != nil || st.Phase != "idle" {
		t.Errorf("run-state after fix = %s, want phase idle", b)
	}
}

// reconcileBrandLogo keeps the desktop brand off a broken mark image. driven
// entirely through env so it never touches the real HOME; one subtest per
// branch of the decision: absent file, empty markImage, unparseable file,
// resolvable image (plain path and file:// with ~), check-only, and a dangling
// image cleared in place while the user's other fields survive.
func TestReconcileBrandLogo(t *testing.T) {
	blob := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 1, 2, 3}

	setup := func(t *testing.T) string {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		return home
	}
	brandPath := func() string { return filepath.Join(sys.ConfigHome(), "ryoku", "brand.json") }
	writeBrand := func(t *testing.T, body string) {
		p := brandPath()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeBlob := func(t *testing.T, path string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, blob, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("absent brand.json is a quiet ok", func(t *testing.T) {
		setup(t)
		if r := reconcileBrandLogo(false); r.status != recOK {
			t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
		}
	})

	t.Run("empty markImage uses the text seal", func(t *testing.T) {
		setup(t)
		writeBrand(t, `{"markText":"力","markImage":"","markTint":true,"name":"Ryoku"}`)
		if r := reconcileBrandLogo(false); r.status != recOK {
			t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
		}
	})

	t.Run("unparseable brand.json is a quiet ok", func(t *testing.T) {
		setup(t)
		writeBrand(t, `{not json`)
		if r := reconcileBrandLogo(false); r.status != recOK {
			t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
		}
	})

	t.Run("resolvable image resolves ok", func(t *testing.T) {
		home := setup(t)
		img := filepath.Join(home, "logo.png")
		writeBlob(t, img)
		writeBrand(t, `{"markText":"力","markImage":"`+img+`","markTint":false,"name":"Acme"}`)
		if r := reconcileBrandLogo(false); r.status != recOK {
			t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
		}
	})

	t.Run("file:// url with tilde resolves ok", func(t *testing.T) {
		home := setup(t)
		writeBlob(t, filepath.Join(home, ".local", "logo.svg"))
		writeBrand(t, `{"markText":"力","markImage":"file://~/.local/logo.svg","markTint":true,"name":"Ryoku"}`)
		if r := reconcileBrandLogo(false); r.status != recOK {
			t.Fatalf("status=%s detail=%q, want ok", r.status.label(), r.detail)
		}
	})

	t.Run("check-only reports the fix without touching the file", func(t *testing.T) {
		setup(t)
		body := `{"markText":"力","markImage":"~/gone.png","markTint":true,"name":"Ryoku"}`
		writeBrand(t, body)
		r := reconcileBrandLogo(true)
		if r.status != recWouldFix {
			t.Fatalf("status=%s detail=%q, want todo", r.status.label(), r.detail)
		}
		got, err := os.ReadFile(brandPath())
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != body {
			t.Errorf("check-only rewrote brand.json:\n before=%q\n after=%q", body, got)
		}
	})

	t.Run("missing image is cleared in place, other fields preserved", func(t *testing.T) {
		setup(t)
		writeBrand(t, `{"markText":"氣","markImage":"~/gone.png","markTint":false,"name":"Acme"}`)
		if r := reconcileBrandLogo(false); r.status != recFixed {
			t.Fatalf("status=%s detail=%q, want fixed", r.status.label(), r.detail)
		}
		var b struct {
			MarkText  string `json:"markText"`
			MarkImage string `json:"markImage"`
			MarkTint  bool   `json:"markTint"`
			Name      string `json:"name"`
		}
		raw, err := os.ReadFile(brandPath())
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &b); err != nil {
			t.Fatalf("brand.json no longer parses after fix: %v", err)
		}
		if b.MarkImage != "" {
			t.Errorf("markImage = %q, want cleared", b.MarkImage)
		}
		if b.MarkText != "氣" || b.MarkTint != false || b.Name != "Acme" {
			t.Errorf("fix dropped user fields: markText=%q markTint=%v name=%q", b.MarkText, b.MarkTint, b.Name)
		}
		// idempotent: markImage now empty, a second run is a no-op ok.
		if r := reconcileBrandLogo(false); r.status != recOK {
			t.Fatalf("second run status=%s, want ok (idempotent)", r.status.label())
		}
	})
}

func TestReconcileRyodecors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	src := filepath.Join(repo, "ryoku", "assets", "ryodecors")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a.png", "b.gif"} {
		if err := os.WriteFile(filepath.Join(src, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("RYOKU_REPO", repo)
	dst := filepath.Join(home, "Pictures", "ryodecors")

	// check-only reports the gap without copying.
	if r := reconcileRyodecors(true); r.status != recWouldFix {
		t.Fatalf("check-only: status=%s detail=%q, want would-fix", r.status.label(), r.detail)
	}
	if _, err := os.Stat(filepath.Join(dst, "a.png")); !os.IsNotExist(err) {
		t.Fatalf("check-only must not copy anything")
	}

	// fix seeds every shipped file.
	if r := reconcileRyodecors(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	for _, n := range []string{"a.png", "b.gif"} {
		if _, err := os.Stat(filepath.Join(dst, n)); err != nil {
			t.Fatalf("expected %s seeded: %v", n, err)
		}
	}

	// a file the user added or swapped survives, and a settled folder is a no-op.
	userFile := filepath.Join(dst, "mine.png")
	if err := os.WriteFile(userFile, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileRyodecors(false); r.status != recOK {
		t.Fatalf("idempotent run: status=%s, want ok", r.status.label())
	}
	if b, _ := os.ReadFile(userFile); string(b) != "mine" {
		t.Fatalf("a user's own decor file must survive the reconciler")
	}
}

// reconcileFrameBarsStyle converges a store that still names a retired bar style
// onto the current default, leaving every other key untouched, and does nothing
// once the value is current or absent.
func TestMigrateFrameBarsStyle(t *testing.T) {
	// a retired style name migrates, and unrelated keys survive untouched.
	retired := []byte(`{"frameBars":{"style":"retired-frame","rails":{"top":{"size":44}}},"weatherLocation":"Oslo","fontScale":1.3}`)
	out, changed, err := migrateFrameBarsStyle(retired)
	if err != nil || !changed {
		t.Fatalf("retired style must migrate: changed=%v err=%v", changed, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("migrated JSON does not parse: %v", err)
	}
	frameBars := cfg["frameBars"].(map[string]any)
	if frameBars["style"] != "slate-frame" {
		t.Errorf("style did not converge on the default: %v", frameBars["style"])
	}
	if size := frameBars["rails"].(map[string]any)["top"].(map[string]any)["size"].(float64); size != 44 {
		t.Errorf("unrelated frameBars key was lost: rail size = %v", size)
	}
	if cfg["weatherLocation"] != "Oslo" || cfg["fontScale"].(float64) != 1.3 {
		t.Errorf("unrelated top-level keys were lost: %v", cfg)
	}

	// migrating is idempotent: the default it wrote is now a no-op.
	if _, changed, err := migrateFrameBarsStyle(out); err != nil || changed {
		t.Errorf("re-migrating the current value must be a no-op: changed=%v err=%v", changed, err)
	}

	// both current style names are left alone.
	for _, current := range []string{"slate-frame", "ryoku-frame"} {
		body := []byte(fmt.Sprintf(`{"frameBars":{"style":%q}}`, current))
		if _, changed, err := migrateFrameBarsStyle(body); err != nil || changed {
			t.Errorf("current style %q must be untouched: changed=%v err=%v", current, changed, err)
		}
	}

	// an absent style key, or no frameBars at all, is untouched.
	if _, changed, err := migrateFrameBarsStyle([]byte(`{"frameBars":{"rails":{}},"weatherLocation":"Oslo"}`)); err != nil || changed {
		t.Errorf("absent style must be untouched: changed=%v err=%v", changed, err)
	}
	if _, changed, err := migrateFrameBarsStyle([]byte(`{"weatherLocation":"Oslo"}`)); err != nil || changed {
		t.Errorf("absent frameBars must be untouched: changed=%v err=%v", changed, err)
	}
}

// stripLegacyStyleKnobs drops the retired surfaceColor / roundness / shadow keys,
// leaves every other key (including the live fontFamily) untouched, and does
// nothing once they are gone.
func TestStripLegacyStyleKnobs(t *testing.T) {
	// retired knobs present alongside live keys (fontFamily is live now): the
	// knobs go, the rest stay.
	full := []byte(`{"surfaceColor":"#0f1115","fontFamily":"Space Grotesk","roundness":0,"frameSmoothing":8,"shadowStrength":0.63,"shadowSize":12,"frameRadius":9,"frameBorder":59,"frameEnabled":true,"frameBars":{"style":"slate-frame"},"weatherLocation":"Oslo"}`)
	out, changed, err := stripLegacyStyleKnobs(full)
	if err != nil || !changed {
		t.Fatalf("legacy knobs must be stripped: changed=%v err=%v", changed, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("stripped JSON does not parse: %v", err)
	}
	for _, key := range legacyStyleKnobs {
		if _, present := cfg[key]; present {
			t.Errorf("retired knob %q survived the strip", key)
		}
	}
	if cfg["frameEnabled"] != true || cfg["weatherLocation"] != "Oslo" {
		t.Errorf("unrelated top-level keys were lost: %v", cfg)
	}
	if cfg["fontFamily"] != "Space Grotesk" {
		t.Errorf("fontFamily is a live key now and must survive the strip: %v", cfg["fontFamily"])
	}
	if frameBars, ok := cfg["frameBars"].(map[string]any); !ok || frameBars["style"] != "slate-frame" {
		t.Errorf("nested frameBars was not preserved: %v", cfg["frameBars"])
	}

	// stripping is idempotent: the cleaned store is now a no-op.
	if _, changed, err := stripLegacyStyleKnobs(out); err != nil || changed {
		t.Errorf("re-stripping a clean store must be a no-op: changed=%v err=%v", changed, err)
	}

	// a store carrying only some of the knobs strips exactly those present.
	partial := []byte(`{"roundness":8,"fontScale":1.3}`)
	out, changed, err = stripLegacyStyleKnobs(partial)
	if err != nil || !changed {
		t.Fatalf("a lone knob must still strip: changed=%v err=%v", changed, err)
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("stripped JSON does not parse: %v", err)
	}
	if _, present := cfg["roundness"]; present {
		t.Error("roundness survived a partial strip")
	}
	if cfg["fontScale"].(float64) != 1.3 {
		t.Errorf("unrelated key lost in a partial strip: %v", cfg)
	}

	// a store with none of the knobs is left untouched.
	if _, changed, err := stripLegacyStyleKnobs([]byte(`{"frameOpacity":1,"weatherUnit":"auto"}`)); err != nil || changed {
		t.Errorf("a store with no retired knobs must be untouched: changed=%v err=%v", changed, err)
	}

	// garbage errors rather than silently rewriting.
	if _, _, err := stripLegacyStyleKnobs([]byte("not json")); err == nil {
		t.Fatal("garbage must error, not silently rewrite")
	}
}

// keyboard.lua is user-owned, so materialize never repairs it, and hyprland.lua
// used to hard-require it: a torn one wedged the whole config into emergency
// mode with no way back (a snapshot restores /, not ~/.config on /home).
// doctor must reseed it like any other drop-in.
func TestReconcileHyprlandConfigRepairsCorruptKeyboard(t *testing.T) {
	t.Setenv("RYOKU_WM", "hyprland") // this reconciler is Hyprland's; pin the provider
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", "")

	hypr := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(hypr, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(hypr, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("hyprland.lua", "pcall(require, \"keyboard\")\n")
	write("keyboard.lua", "hl.config({ input = { kb_layout = \"u") // truncated mid-write

	if r := reconcileHyprlandConfig(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	b, err := os.ReadFile(filepath.Join(hypr, "keyboard.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !hyprLuaSane(string(b)) {
		t.Fatalf("keyboard.lua not parseable after repair: %q", b)
	}
	if !strings.Contains(string(b), "kb_layout") {
		t.Fatalf("reseeded keyboard.lua must still set a layout, got %q", b)
	}
}

// transientAppScope must match systemd's per-launch GUI scopes (app-*.scope) so
// doctor auto-clears their lingering failed state, and must never match a real
// service, which stays reported.
func TestTransientAppScope(t *testing.T) {
	for _, u := range []string{"app-discord-155691.scope", "app-firefox-8043.scope", "app-org.foo.Bar-1.scope"} {
		if !transientAppScope(u) {
			t.Errorf("transientAppScope(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"app-daemon.service", "ryoku-shell.service", "session-2.scope", "user@1000.service", "sshd.service"} {
		if transientAppScope(u) {
			t.Errorf("transientAppScope(%q) = true, want false", u)
		}
	}
}

// strayRyokuFiles must return only the deploy-seeded paths pacman does not own:
// removing a package-owned file would break the install, and missing an unowned
// one leaves the -Syu conflict that wedges every update.
func TestStrayRyokuFilesSelectsUnownedOnly(t *testing.T) {
	glob := func(pat string) ([]string, error) {
		switch pat {
		case "/usr/bin/ryoku-*":
			return []string{"/usr/bin/ryoku-dns", "/usr/bin/ryoku-gpu"}, nil
		case "/usr/share/polkit-1/rules.d/*ryoku*.rules":
			return []string{"/usr/share/polkit-1/rules.d/50-ryoku-dns.rules"}, nil
		case "/usr/share/plymouth/themes/ryoku/*":
			return []string{"/usr/share/plymouth/themes/ryoku/bullet.png", "/usr/share/plymouth/themes/ryoku/logo.png"}, nil
		case "/usr/lib/systemd/system/ryoku-*":
			return []string{"/usr/lib/systemd/system/ryoku-network-kill-guard.service"}, nil
		case "/usr/share/ryoku/boot/*":
			return []string{"/usr/share/ryoku/boot/default.conf"}, nil
		case "/etc/systemd/logind.conf.d/10-ryoku-lid.conf":
			return []string{"/etc/systemd/logind.conf.d/10-ryoku-lid.conf"}, nil
		}
		return nil, nil
	}
	// packaged; the rest are seeded unowned by the installer / a dev deploy.
	owned := func(p string) bool {
		return p == "/usr/bin/ryoku-gpu" || p == "/usr/share/plymouth/themes/ryoku/logo.png"
	}
	got := strayRyokuFiles(ryokuSystemGlobs, glob, owned)
	want := map[string]bool{
		"/usr/bin/ryoku-dns":                                       true,
		"/usr/share/polkit-1/rules.d/50-ryoku-dns.rules":           true,
		"/usr/share/plymouth/themes/ryoku/bullet.png":              true,
		"/usr/lib/systemd/system/ryoku-network-kill-guard.service": true,
		"/usr/share/ryoku/boot/default.conf":                       true,
		"/etc/systemd/logind.conf.d/10-ryoku-lid.conf":             true,
	}
	if len(got) != len(want) {
		t.Fatalf("strayRyokuFiles = %v, want exactly the unowned paths", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("strayRyokuFiles returned %q; package-owned files must be excluded", g)
		}
	}
}

// resolveGtkThemeName maps the theme.json GTK choice + session mode to the exact
// gsettings gtk-theme name the daemon writes (C3), so the GTK session reconciler
// converges to a name the next repaint will not immediately flip. system means
// Ryoku never owns gtk-theme, so it resolves to the empty name, and an absent
// choice reads as adw.
func TestResolveGtkThemeName(t *testing.T) {
	cases := []struct {
		pref string
		dark bool
		want string
	}{
		{"adw", true, "adw-gtk3-dark"},
		{"adw", false, "adw-gtk3"},
		{"adwaita", true, "Adwaita-dark"},
		{"adwaita", false, "Adwaita"},
		{"system", true, ""},
		{"system", false, ""},
		{"", true, "adw-gtk3-dark"},
	}
	for _, c := range cases {
		if got := resolveGtkThemeName(c.pref, c.dark); got != c.want {
			t.Errorf("resolveGtkThemeName(%q, %v) = %q, want %q", c.pref, c.dark, got, c.want)
		}
	}
}

func TestUpgradeFastfetchOSLine(t *testing.T) {
	old := `{ "type": "command", "key": "OS",     "text": "echo \"Ryoku $(ryoku version 2>/dev/null || echo dev)\"" },
{ "type": "command", "key": "BRANCH", "text": "ryoku version --branch 2>/dev/null || echo main" },`
	got, changed := upgradeFastfetchOSLine(old)
	if !changed {
		t.Fatal("the pre-release OS line must be upgraded")
	}
	if !strings.Contains(got, `ryoku version --pretty 2>/dev/null || echo dev`) {
		t.Fatalf("OS line not moved to --pretty:\n%s", got)
	}
	if !strings.Contains(got, `ryoku version --branch 2>/dev/null || echo main`) {
		t.Fatalf("BRANCH line must be untouched:\n%s", got)
	}
	if again, changed := upgradeFastfetchOSLine(got); changed || again != got {
		t.Fatal("upgrading twice must be a no-op")
	}
	if _, changed := upgradeFastfetchOSLine(`{ "key": "OS", "text": "echo mine" }`); changed {
		t.Fatal("a user-rewritten OS line must be left alone")
	}
	saved := `"text": "echo \"Ryoku $(ryoku version 2\u003e/dev/null || echo dev)\""`
	got, changed = upgradeFastfetchOSLine(saved)
	if !changed || !strings.Contains(got, `ryoku version --pretty 2\u003e/dev/null`) {
		t.Fatalf("a Hub-saved config (JSON-escaped >) must be upgraded too:\n%s", got)
	}
}

func TestStaleUserRyotunesSpotsTheWrapperOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "share"))
	bin := filepath.Join(dir, "ryotunes")
	os.WriteFile(bin, []byte("#!/usr/bin/env bash\nexec chromium --app=https://music.youtube.com\n"), 0o755)
	if got := staleUserRyotunes(bin); got != "the Chromium YouTube Music wrapper" {
		t.Fatalf("wrapper not recognised: %q", got)
	}
	os.WriteFile(bin, []byte("\x7fELF..."), 0o755)
	if got := staleUserRyotunes(bin); got != "" {
		t.Fatalf("a user's own binary must be left alone, got %q", got)
	}
	os.MkdirAll(filepath.Join(dir, "share", "ryoku"), 0o755)
	os.WriteFile(filepath.Join(dir, "share", "ryoku", "ryotunes.commit"), []byte("abc\n"), 0o644)
	if got := staleUserRyotunes(bin); got != "a locally built ryotunes" {
		t.Fatalf("dev-deploy build not recognised: %q", got)
	}
	if got := staleUserRyotunes(filepath.Join(dir, "missing")); got != "" {
		t.Fatalf("missing file must be nothing, got %q", got)
	}
}

// The portal frontend a session needs is the one its compositor declares
// (wm.Caps.PortalBackend): a niri box must not be told to install Hyprland's.
func TestPortalFrontendCheckFollowsTheDeclaredBackend(t *testing.T) {
	cases := []struct{ provider, backend, wantPkg string }{
		{"niri", "gnome", "xdg-desktop-portal-gnome"},
		{"hyprland", "hyprland", "xdg-desktop-portal-hyprland"},
	}
	for _, c := range cases {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		bin := filepath.Join(home, "bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		caps := "#!/bin/sh\n[ \"$1\" = caps ] && echo '{\"name\":\"" + c.provider +
			"\",\"portalBackend\":\"" + c.backend + "\",\"supports\":[],\"workspaceModel\":\"fixed\"}'\nexit 0\n"
		if err := os.WriteFile(filepath.Join(bin, "ryoku-wm-"+c.provider), []byte(caps), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin)
		t.Setenv("RYOKU_WM", c.provider)

		fix, pkgs := portalFrontendCheck()
		if len(pkgs) != 1 || pkgs[0] != c.wantPkg {
			t.Errorf("%s: pkgs=%v, want [%s]", c.provider, pkgs, c.wantPkg)
		}
		if !strings.Contains(fix, c.wantPkg) {
			t.Errorf("%s: fix hint %q should name %s", c.provider, fix, c.wantPkg)
		}
	}

	// With no provider answering, every frontend is accepted rather than one
	// compositor's: the old behaviour pointed a niri box at Hyprland's portal.
	t.Setenv("RYOKU_WM", "none")
	t.Setenv("PATH", t.TempDir())
	fix, pkgs := portalFrontendCheck()
	if fix != "" || len(pkgs) < 2 {
		t.Fatalf("no provider: fix=%q pkgs=%v, want no hint and every frontend", fix, pkgs)
	}
}
