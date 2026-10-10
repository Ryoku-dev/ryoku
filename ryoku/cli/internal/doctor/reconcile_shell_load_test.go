package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What the loader prints when a singleton cannot be built: the module is named,
// the file is not. The repair has to work from this alone.
const moduleFailure = `  INFO: Launching config: "/home/u/.config/quickshell/shell/shell.qml"
 ERROR: Failed to load configuration
 ERROR:   caused by @shell.qml[13:1]: module "shell.services" is not installed`

// And what it prints when it does name the file.
const fileFailure = `  INFO: Launching config: "/home/u/.config/quickshell/shell/shell.qml"
 ERROR: Failed to load configuration
 ERROR:   caused by file:///home/u/.config/quickshell/shell/services/Keyring.qml[-1:-1]: Type Media unavailable
 ERROR:   caused by file:///home/u/.config/quickshell/shell/services/Media.qml[76:1]: Syntax error`

// And what Qt prints when a compiled module was built against a newer Qt than
// the one running: no QML file is at fault, the box is behind its distribution.
const qtMismatchFailure = `  INFO: Launching config: "/home/u/.config/quickshell/shell/shell.qml"
 ERROR: Failed to load configuration
 ERROR:   caused by @shell.qml[13:1]: module "shell.services" is not installed
 ERROR:   caused by file:///home/u/.config/quickshell/shell/services/Keyring.qml[-1:-1]: The plugin '/usr/lib/qt6/qml/Ryoku/Blobs/libryoku-blobsplugin.so' uses incompatible Qt library. (6.12.0) [release]`

const defaultEnvFailure = ` ERROR: Unrecognized pragma "DefaultEnv QSG_RENDER_LOOP=threaded"`

// how many times a repair asked for the shell back
var testRestarts int

func TestParseQmlErrorTakesTheRealCause(t *testing.T) {
	file, line, reason := parseQmlError(fileFailure)
	if file != "/home/u/.config/quickshell/shell/services/Media.qml" {
		t.Fatalf("the last cause is the real one, got %q", file)
	}
	if line != "76" || reason != "Syntax error" {
		t.Fatalf("line = %q reason = %q", line, reason)
	}
	if f, l, _ := parseQmlError(`ERROR: caused by file:///x/y/Z.qml[-1:-1]: Type Media unavailable`); f != "/x/y/Z.qml" || l != "" {
		t.Fatalf("a cause with no real line should report none: %q %q", f, l)
	}
	if f, _, _ := parseQmlError("INFO: Configuration Loaded"); f != "" {
		t.Fatalf("a healthy run names no file, got %q", f)
	}
}

func TestFailingScopeNarrowsToTheModule(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/home/u/.config")
	if got := failingScope(moduleFailure); got != "quickshell/shell/services" {
		t.Fatalf("a named module scopes the repair, got %q", got)
	}
	if got := failingScope(fileFailure); got != "quickshell/shell/services" {
		t.Fatalf("a named file scopes to its directory, got %q", got)
	}
	if got := failingScope("ERROR: Failed to load configuration"); got != "quickshell" {
		t.Fatalf("with nothing named, the whole tree is in scope, got %q", got)
	}
}

// shellSandbox is a fake home whose live desktop tree has drifted from the
// shipped one, with the module failure recorded in the daemon's surface log.
func shellSandbox(t *testing.T) (cfg, base string) {
	t.Helper()
	dir := t.TempDir()
	cfg, base = filepath.Join(dir, "config"), filepath.Join(dir, "base")
	rel := filepath.Join("quickshell", "shell", "services", "Media.qml")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(base, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(cfg, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, rel), []byte("// shipped, good\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, rel), []byte("// hand edited, broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("RYOKU_CONFIG_BASE", base)

	log := filepath.Join(dir, "state", "ryoku", "surfaces")
	if err := os.MkdirAll(log, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(log, "shell.log"), []byte(moduleFailure), 0o644); err != nil {
		t.Fatal(err)
	}

	// no desktop is up in a sandbox, and /proc cannot be faked
	prev := liveShells
	liveShells = func() []shellInstance { return nil }
	prevStableWindow := shellLoadStableWindow
	shellLoadStableWindow = 0
	// and a test never restarts the machine's own desktop
	prevRestart := restartShell
	testRestarts = 0
	restartShell = func() { testRestarts++ }
	t.Cleanup(func() {
		liveShells, restartShell = prev, prevRestart
		shellLoadStableWindow = prevStableWindow
	})
	return cfg, base
}

func TestShellLoadPutsAStaleFileBack(t *testing.T) {
	cfg, _ := shellSandbox(t)
	rel := filepath.Join("quickshell", "shell", "services", "Media.qml")

	if res := reconcileShellLoad(true); res.status != recWouldFix {
		t.Fatalf("check-only status = %v (%s)", res.status, res.detail)
	}
	if b, _ := os.ReadFile(filepath.Join(cfg, rel)); string(b) != "// hand edited, broken\n" {
		t.Fatal("check-only must not write")
	}

	res := reconcileShellLoad(false)
	if res.status != recFixed {
		t.Fatalf("fix status = %v (%s)", res.status, res.detail)
	}
	if b, _ := os.ReadFile(filepath.Join(cfg, rel)); string(b) != "// shipped, good\n" {
		t.Fatalf("the shipped file should be back, got %q", b)
	}
	if testRestarts != 1 {
		t.Fatalf("a repair brings the desktop back itself, restarts = %d", testRestarts)
	}
}

func TestShellLoadMovesABreakingOverrideAside(t *testing.T) {
	cfg, _ := shellSandbox(t)
	rel := filepath.Join("quickshell", "shell", "services", "Media.qml")
	overlay := filepath.Join(cfg, "ryoku", "user_edits", rel)
	if err := os.MkdirAll(filepath.Dir(overlay), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, []byte("// my override, broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := reconcileShellLoad(false)
	if res.status != recFixed {
		t.Fatalf("status = %v (%s)", res.status, res.detail)
	}
	if _, err := os.Stat(overlay); !os.IsNotExist(err) {
		t.Fatal("the override should have been moved aside")
	}
	if _, err := os.Stat(overlay + ".broken"); err != nil {
		t.Fatalf("the override is kept, never deleted: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(cfg, rel)); string(b) != "// shipped, good\n" {
		t.Fatalf("the shipped file should be back, got %q", b)
	}
}

// Our own shipped file at fault: nothing local to repair, so say so and name the
// commands that do help.
func TestShellLoadReportsAShippedBug(t *testing.T) {
	cfg, base := shellSandbox(t)
	rel := filepath.Join("quickshell", "shell", "services", "Media.qml")
	shipped, err := os.ReadFile(filepath.Join(base, rel))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, rel), shipped, 0o644); err != nil {
		t.Fatal(err)
	}

	res := reconcileShellLoad(false)
	if res.status != recWarn {
		t.Fatalf("status = %v (%s)", res.status, res.detail)
	}
	if !strings.Contains(res.detail, "shell.services") {
		t.Fatalf("the report should name what did not load: %s", res.detail)
	}
	if !strings.Contains(res.remedy, "ryoku update") || !strings.Contains(res.remedy, "rollback") {
		t.Fatalf("remedy should offer the update and the rollback: %s", res.remedy)
	}
}

func TestShellLoadReportsDefaultEnvPragmaFailureDuringRespawn(t *testing.T) {
	cfg, base := shellSandbox(t)
	rel := filepath.Join("quickshell", "shell", "services", "Media.qml")
	shipped, err := os.ReadFile(filepath.Join(base, rel))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, rel), shipped, 0o644); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(os.Getenv("XDG_STATE_HOME"), "ryoku", "surfaces", "shell.log")
	if err := os.WriteFile(log, []byte(defaultEnvFailure), 0o644); err != nil {
		t.Fatal(err)
	}
	liveShells = func() []shellInstance { return []shellInstance{{pid: 42, config: "shell"}} }

	res := reconcileShellLoad(true)
	if res.status != recWarn {
		t.Fatalf("status = %v (%s)", res.status, res.detail)
	}
	if !strings.Contains(res.detail, "desktop cannot load") {
		t.Fatalf("fatal pragma error was not reported: %s", res.detail)
	}
}

// A refused plugin means the system's Qt is older than the build's. The drifted
// tree is not the cause and must stay as it is; the remedy is the system update.
func TestShellLoadNamesAQtBehindTheBuild(t *testing.T) {
	cfg, _ := shellSandbox(t)
	log := filepath.Join(os.Getenv("XDG_STATE_HOME"), "ryoku", "surfaces", "shell.log")
	if err := os.WriteFile(log, []byte(qtMismatchFailure), 0o644); err != nil {
		t.Fatal(err)
	}

	res := reconcileShellLoad(false)
	if res.status != recWarn {
		t.Fatalf("status = %v (%s)", res.status, res.detail)
	}
	if !strings.Contains(res.detail, "libryoku-blobsplugin.so") || !strings.Contains(res.detail, "Qt 6.12") {
		t.Fatalf("the report should name the plugin and the Qt it needs: %s", res.detail)
	}
	if !strings.Contains(res.remedy, "pacman -Syu") {
		t.Fatalf("remedy should be the system update: %s", res.remedy)
	}
	if testRestarts != 0 {
		t.Fatal("nothing to repair, so the shell must not be restarted")
	}
	rel := filepath.Join("quickshell", "shell", "services", "Media.qml")
	if b, _ := os.ReadFile(filepath.Join(cfg, rel)); !strings.Contains(string(b), "hand edited") {
		t.Fatal("the drifted file is not the cause and must be left alone")
	}
}

func TestShellLoadStaysQuietWithALoadedDesktop(t *testing.T) {
	shellSandbox(t)
	log := filepath.Join(os.Getenv("XDG_STATE_HOME"), "ryoku", "surfaces", "shell.log")
	if err := os.WriteFile(log, []byte("INFO: Configuration Loaded\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := liveShells
	liveShells = func() []shellInstance { return []shellInstance{{pid: 1}} }
	t.Cleanup(func() { liveShells = prev })

	if res := reconcileShellLoad(false); res.status != recOK {
		t.Fatalf("a loaded desktop is never repaired: %v (%s)", res.status, res.detail)
	}
}

func TestShellConfigRelIgnoresPathsOutsideConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/home/u/.config")
	if got := shellConfigRel("/home/u/.config/quickshell/shell/x.qml"); got != "quickshell/shell/x.qml" {
		t.Fatalf("rel = %q", got)
	}
	if got := shellConfigRel("/usr/lib/qt6/qml/QtQuick/x.qml"); got != "" {
		t.Fatalf("a system path is not ours: %q", got)
	}
}

// A build box has no renderer on PATH. The log still says what happened, so the
// repair must not need `qs` to exist.
func TestShellLoadRepairsWithNoRendererOnPath(t *testing.T) {
	cfg, _ := shellSandbox(t)
	t.Setenv("PATH", t.TempDir())

	res := reconcileShellLoad(false)
	if res.status != recFixed {
		t.Fatalf("status = %v (%s)", res.status, res.detail)
	}
	rel := filepath.Join("quickshell", "shell", "services", "Media.qml")
	if b, _ := os.ReadFile(filepath.Join(cfg, rel)); string(b) != "// shipped, good\n" {
		t.Fatalf("the shipped file should be back, got %q", b)
	}
}

// The updater restarts the shell and checks immediately after. A surface still
// coming up is not a broken one, and nothing may be touched on its behalf.
func TestShellLoadWaitsForASlowStart(t *testing.T) {
	cfg, _ := shellSandbox(t)
	if err := os.Remove(filepath.Join(os.Getenv("XDG_STATE_HOME"), "ryoku", "surfaces", "shell.log")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())

	polls := 0
	prev := liveShells
	liveShells = func() []shellInstance {
		polls++
		if polls > 3 {
			return []shellInstance{{pid: 1}}
		}
		return nil
	}
	t.Cleanup(func() { liveShells = prev })

	if res := reconcileShellLoad(false); res.status != recOK {
		t.Fatalf("a slow start is not a failure: %v (%s)", res.status, res.detail)
	}
	rel := filepath.Join("quickshell", "shell", "services", "Media.qml")
	if b, _ := os.ReadFile(filepath.Join(cfg, rel)); string(b) != "// hand edited, broken\n" {
		t.Fatalf("nothing may be restored for a desktop that came up, got %q", b)
	}
}
