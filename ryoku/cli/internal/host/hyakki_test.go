package host

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

type hyakkiServiceState struct{ enabled, running bool }

type hyakkiMigrationFixture struct {
	app           *App
	runner        *fakeRunner
	states        map[string]hyakkiServiceState
	migrations    int
	migrationCode int
	failRestore   bool
}

func newHyakkiMigrationFixture(t *testing.T) *hyakkiMigrationFixture {
	t.Helper()
	root, err := os.MkdirTemp("", "hyakki-host-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	f := &hyakkiMigrationFixture{states: map[string]hyakkiServiceState{}}
	f.runner = &fakeRunner{paths: map[string]bool{"hyakki": true, "systemctl": true}}
	f.app, _, _ = testApp(f.runner, map[string]string{"HOME": root, "XDG_RUNTIME_DIR": root, "XDG_CONFIG_HOME": filepath.Join(root, "config"), "RYOKU_HOST_INIT": "systemd"})
	f.app.cfg.UID = os.Getuid()
	f.runner.answer = func(c Command) Result {
		if c.Name == "hyakki" {
			f.migrations++
			for name, state := range f.states {
				if state.enabled || state.running {
					t.Fatalf("migration ran before %s was quiescent: %+v", name, state)
				}
			}
			return Result{Code: f.migrationCode}
		}
		if c.Name != "systemctl" || len(c.Args) < 3 || c.Args[0] != "--user" {
			t.Fatalf("unexpected command: %+v", c)
		}
		verb, name := c.Args[1], c.Args[len(c.Args)-1]
		state, exists := f.states[name]
		if !exists {
			return Result{Code: 4}
		}
		switch verb {
		case "show":
			if len(c.Args) > 3 && c.Args[3] == "UnitFileState" {
				if state.enabled {
					return Result{Output: "enabled\n"}
				}
				return Result{Output: "disabled\n"}
			}
			return Result{Output: "no\n"}
		case "is-enabled":
			if state.enabled {
				return Result{}
			}
			return Result{Code: 1}
		case "is-active":
			if state.running {
				return Result{}
			}
			return Result{Code: 3}
		case "enable":
			if f.failRestore {
				return Result{Code: 1}
			}
			state.enabled = true
		case "disable":
			state.enabled = false
		case "start":
			state.running = true
		case "stop":
			if !strings.HasSuffix(name, ".socket") && !strings.HasSuffix(name, ".timer") {
				t.Fatalf("migration signalled worker %s", name)
			}
			state.running = false
		default:
			t.Fatalf("unexpected service mutation %s", verb)
		}
		f.states[name] = state
		return Result{}
	}
	return f
}

func (f *hyakkiMigrationFixture) snapshot() []legacyHyakkiService {
	services := legacyHyakkiServices(Systemd)
	for i := range services {
		state := f.states[services[i].name]
		services[i].enabled, services[i].running = state.enabled, state.running
	}
	return services
}

func TestHyakkiMigrationStopsIdleActivatorsBeforeNativeMigration(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	f.states["ryoku-ai-usage.timer"] = hyakkiServiceState{true, true}
	f.states["rashind.service"] = hyakkiServiceState{false, false}
	if err := f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("idle activators should migrate immediately") }, ""); err != nil {
		t.Fatal(err)
	}
	if f.migrations != 1 {
		t.Fatalf("native migrations = %d", f.migrations)
	}
	for _, state := range f.states {
		if state.enabled || state.running {
			t.Fatalf("legacy state survived successful migration: %+v", f.states)
		}
	}
}

func TestHyakkiMigrationFailureRestoresEnabledRunningAndConsentBytes(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	f.states["rashind.socket"] = hyakkiServiceState{false, true}
	f.states["ryoku-ai-usage.timer"] = hyakkiServiceState{true, false}
	before := map[string]hyakkiServiceState{}
	for name, state := range f.states {
		before[name] = state
	}
	path := filepath.Join(f.app.home(), "config", "ryoku", "rashin.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte("{\"enabled\":true,\"unknown\":\"keep me\"}\n")
	if err := os.WriteFile(path, body, 0o640); err != nil {
		t.Fatal(err)
	}
	f.migrationCode = 1
	err := f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("unexpected wait") }, "")
	if err == nil || !strings.Contains(err.Error(), "state restored") {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(f.states, before) {
		t.Fatalf("state = %+v, want %+v", f.states, before)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("legacy consent changed: %q, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("legacy consent mode changed: %v, %v", info, err)
	}
}

func TestHyakkiMigrationRefusesBusyWorkerBeforeMutations(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	f.states["rashind-routines.service"] = hyakkiServiceState{true, true}
	err := f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("busy worker should be refused") }, "")
	if err == nil || !strings.Contains(err.Error(), "finish its work") {
		t.Fatalf("error = %v", err)
	}
	if len(f.runner.commands) != 0 || f.migrations != 0 {
		t.Fatalf("busy worker caused mutations: %+v", f.runner.commands)
	}
}

func hyakkiHeldLegacyLock(t *testing.T, a *App, name string) *os.File {
	t.Helper()
	if err := os.MkdirAll(a.legacyHyakkiRuntime(), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(a.legacyHyakkiRuntime(), name), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestHyakkiMigrationPreservesGUIBuffers(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	hyakkiHeldLegacyLock(t, f.app, "app.lock")
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	err := f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("GUI owner should be refused") }, "")
	if err == nil || !strings.Contains(err.Error(), "saving its input") {
		t.Fatalf("error = %v", err)
	}
	if len(f.runner.commands) != 0 || f.migrations != 0 {
		t.Fatalf("GUI owner caused mutations: %+v", f.runner.commands)
	}
}

func TestHyakkiMigrationWaitsForNaturalDaemonExitWithoutSignalling(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	lock := hyakkiHeldLegacyLock(t, f.app, "rashind.lock")
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	f.states["rashind.service"] = hyakkiServiceState{false, true}
	path := filepath.Join(f.app.legacyHyakkiRuntime(), "rashin.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	waits := 0
	err = f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) {
		waits++
		if c, err := net.Dial("unix", path); err == nil {
			c.Close()
			t.Fatal("legacy ingress stayed available during the wait")
		}
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
			t.Fatal(err)
		}
		f.states["rashind.service"] = hyakkiServiceState{false, false}
	}, "")
	if err != nil || waits != 1 || f.migrations != 1 {
		t.Fatalf("result: %v, waits %d, migrations %d", err, waits, f.migrations)
	}
}

func TestHyakkiMigrationTimeoutRestoresLiveDaemonListener(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	hyakkiHeldLegacyLock(t, f.app, "rashind.lock")
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	f.states["rashind.service"] = hyakkiServiceState{false, true}
	path := filepath.Join(f.app.legacyHyakkiRuntime(), "rashin.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var waited time.Duration
	err = f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(d time.Duration) { waited += d }, "")
	if err == nil || !strings.Contains(err.Error(), "configured idle deadline") || !strings.Contains(err.Error(), "state restored") {
		t.Fatalf("error = %v", err)
	}
	if waited != 5*time.Minute+5*time.Second || f.migrations != 0 {
		t.Fatalf("waited %v, migrations %d", waited, f.migrations)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("previous live daemon listener was not restored: %v", err)
	}
	conn.Close()
	if state := f.states["rashind.socket"]; !state.enabled || !state.running {
		t.Fatalf("socket state = %+v", state)
	}
}

func TestHyakkiMigrationReportsIncompleteRestoration(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	f.migrationCode, f.failRestore = 1, true
	err := f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) {}, "")
	if err == nil || !strings.Contains(err.Error(), "restoration incomplete") {
		t.Fatalf("error = %v", err)
	}
}

func TestHyakkiRunitMarkerRestorationKeepsBytesAndMode(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"down", runitUserDisabledMarker} {
		path := filepath.Join(root, name)
		body := []byte("authored marker\n")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		before, err := snapshotLegacyHyakkiFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := before.restore(); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(body, got) {
			t.Fatalf("marker bytes changed: %q, %v", got, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("marker mode changed: %v, %v", info, err)
		}
	}
}

func TestHyakkiRunitFailureRestoresDormantEnabledServiceWithoutStartingIt(t *testing.T) {
	root := t.TempDir()
	service := filepath.Join(root, "service", "ryoku-ai-usage")
	if err := os.MkdirAll(filepath.Join(service, "supervise"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(service, "supervise", "stat"), []byte("down\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := []byte("user keeps this service down\n")
	if err := os.WriteFile(filepath.Join(service, "down"), body, 0o640); err != nil {
		t.Fatal(err)
	}
	var files []legacyHyakkiFile
	for _, name := range []string{"down", runitUserDisabledMarker} {
		snapshot, err := snapshotLegacyHyakkiFile(filepath.Join(service, name))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, snapshot)
	}
	migrated := false
	runner := &fakeRunner{answer: func(c Command) Result {
		if c.Name == "hyakki" {
			migrated = true
			if !fileExists(filepath.Join(service, runitUserDisabledMarker)) {
				t.Fatal("native migration ran while runit service was enabled")
			}
			return Result{Code: 1}
		}
		if c.Name != "sv" || len(c.Args) != 2 || c.Args[0] != "status" {
			t.Fatalf("dormant worker was signalled: %+v", c)
		}
		return Result{Output: "down: " + service + ": 10s\n"}
	}}
	app, _, _ := testApp(runner, map[string]string{"HOME": root, "XDG_RUNTIME_DIR": root, "RYOKU_HOST_INIT": "runit"})
	app.cfg.UserServiceDir = filepath.Dir(service)
	states := []legacyHyakkiService{{name: "ryoku-ai-usage", enabled: true}}
	err := app.migrateHyakkiQuiesced(states, files, app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("unexpected wait") }, "")
	if !migrated || err == nil || !strings.Contains(err.Error(), "state restored") {
		t.Fatalf("migration result = %v, ran %v", err, migrated)
	}
	got, err := os.ReadFile(filepath.Join(service, "down"))
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("down marker changed: %q, %v", got, err)
	}
	if fileExists(filepath.Join(service, runitUserDisabledMarker)) {
		t.Fatal("previously enabled service stayed disabled")
	}
	info, err := os.Stat(filepath.Join(service, "down"))
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("down mode changed: %v, %v", info, err)
	}
}

func TestHyakkiStaticUnitIsNotAnEnabledActivationSource(t *testing.T) {
	app, _, _ := testApp(&fakeRunner{results: []Result{{}, {Output: "static\n"}}}, map[string]string{"RYOKU_HOST_INIT": "systemd"})
	enabled, state, err := app.legacyHyakkiEnabled("rashind.service")
	if err != nil || enabled || state != "static" {
		t.Fatalf("static service state = %v, %q, %v", enabled, state, err)
	}
}

func TestHyakkiLateActivationCannotStopSocketWhileOwnerHoldsLocks(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	f.states["rashind.service"] = hyakkiServiceState{}
	services := f.snapshot()
	base := f.runner.answer
	var owner *os.File
	ownerAlive := false
	protectedStop := false
	f.runner.answer = func(c Command) Result {
		if c.Name == "systemctl" && c.Args[1] == "disable" && c.Args[len(c.Args)-1] == "rashind.socket" {
			result := base(c)
			owner = hyakkiHeldLegacyLock(t, f.app, "rashind.lock")
			ownerAlive = true
			f.states["rashind.service"] = hyakkiServiceState{false, true}
			return result
		}
		if c.Name == "systemctl" && c.Args[1] == "stop" && c.Args[len(c.Args)-1] == "rashind.socket" {
			if ownerAlive {
				t.Fatal("Requires-linked socket stop would cancel the late daemon's work")
			}
			for _, name := range []string{"app.lock", "rashind.lock", "config.lock"} {
				busy, err := legacyHyakkiLockBusy(filepath.Join(f.app.legacyHyakkiRuntime(), name))
				if err != nil || !busy {
					t.Fatalf("socket stop did not hold predecessor %s: busy=%v err=%v", name, busy, err)
				}
			}
			protectedStop = true
		}
		if c.Name == "hyakki" {
			if ready, err := f.app.legacyHyakkiReleased(); err != nil || !ready {
				t.Fatalf("host retained ownership over native handoff: ready=%v err=%v", ready, err)
			}
		}
		return base(c)
	}
	// Deliberately stale readiness: acquiring the actual locks must still
	// protect the stop, rather than trusting a previous owner-free probe.
	err := f.app.migrateHyakkiQuiesced(services, nil, func() (bool, error) { return true, nil }, func(time.Duration) {
		if !ownerAlive || !f.states["rashind.socket"].running {
			t.Fatal("late owner or its activation socket did not survive quiescing")
		}
		if err := syscall.Flock(int(owner.Fd()), syscall.LOCK_UN); err != nil {
			t.Fatal(err)
		}
		ownerAlive = false
		f.states["rashind.service"] = hyakkiServiceState{}
	}, "")
	if err != nil || !protectedStop || ownerAlive {
		t.Fatalf("late activation handoff: err=%v protectedStop=%v ownerAlive=%v", err, protectedStop, ownerAlive)
	}
}

func TestHyakkiForeignRuntimeSocketIsRefusedBeforeMutation(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	foreign := t.TempDir()
	lock, err := os.OpenFile(filepath.Join(foreign, "rashind.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(foreign, "rashin.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	getenv := f.app.cfg.Getenv
	f.app.cfg.Getenv = func(key string) string {
		if key == "RASHIN_SOCKET" {
			return path
		}
		return getenv(key)
	}
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	err = f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("foreign owner must not be waited on") }, "")
	if err == nil || !strings.Contains(err.Error(), "cannot correlate RASHIN_SOCKET") {
		t.Fatalf("foreign runtime result = %v", err)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("foreign listener was changed: %v", err)
	}
	conn.Close()
	if !f.states["rashind.socket"].enabled || !f.states["rashind.socket"].running || f.migrations != 0 {
		t.Fatalf("foreign runtime changed migration state: %+v", f.states)
	}
	if busy, err := legacyHyakkiLockBusy(filepath.Join(foreign, "rashind.lock")); err != nil || !busy {
		t.Fatalf("foreign ownership did not survive: busy=%v err=%v", busy, err)
	}
}

func TestHyakkiRunitExistingUninspectableServicesAreNotAbsent(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("public migration deliberately refuses root")
	}
	for _, supervised := range []bool{false, true} {
		t.Run(fmt.Sprint(supervised), func(t *testing.T) {
			root := t.TempDir()
			service := filepath.Join(root, "services", "ryoku-ai-usage")
			if err := os.MkdirAll(service, 0o700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(service, "down")
			body := []byte("keep this dormant service\n")
			if err := os.WriteFile(marker, body, 0o640); err != nil {
				t.Fatal(err)
			}
			if supervised {
				if err := os.Mkdir(filepath.Join(service, "supervise"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(service, "supervise", "stat"), []byte("unknown\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			runner := &fakeRunner{paths: map[string]bool{"hyakki": true, "sv": true}, answer: func(c Command) Result {
				if c.Name != "sv" || c.Args[0] != "status" {
					t.Fatalf("uninspectable service caused mutation: %+v", c)
				}
				return Result{Code: 1}
			}}
			app, _, _ := testApp(runner, map[string]string{"HOME": root, "XDG_RUNTIME_DIR": root, "RYOKU_HOST_INIT": "runit"})
			app.cfg.UID = os.Getuid()
			app.cfg.UserServiceDir = filepath.Dir(service)
			err := app.MigrateHyakki("")
			if err == nil || !strings.Contains(err.Error(), "supervisor cannot be inspected") {
				t.Fatalf("uninspectable service result = %v", err)
			}
			got, err := os.ReadFile(marker)
			info, statErr := os.Stat(marker)
			if err != nil || statErr != nil || !bytes.Equal(got, body) || info.Mode().Perm() != 0o640 {
				t.Fatalf("uninspectable service marker changed: %q, %v, %v", got, err, statErr)
			}
			if fileExists(filepath.Join(service, runitUserDisabledMarker)) {
				t.Fatal("uninspectable service was disabled")
			}
		})
	}
}

func TestHyakkiRuntimeEnabledRollbackRestoresExactActivationState(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	services := f.snapshot()
	services[0].enabledState = "enabled-runtime"
	unitState := "enabled-runtime"
	base := f.runner.answer
	f.runner.answer = func(c Command) Result {
		if c.Name == "systemctl" && c.Args[1] == "show" && c.Args[3] == "UnitFileState" {
			return Result{Output: unitState + "\n"}
		}
		result := base(c)
		if c.Name == "systemctl" && c.Args[len(c.Args)-1] == "rashind.socket" {
			if c.Args[1] == "disable" {
				unitState = "disabled"
			}
			if c.Args[1] == "enable" {
				unitState = "enabled"
				for _, arg := range c.Args {
					if arg == "--runtime" {
						unitState = "enabled-runtime"
					}
				}
			}
		}
		return result
	}
	f.migrationCode = 1
	err := f.app.migrateHyakkiQuiesced(services, nil, f.app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("unexpected wait") }, "")
	if err == nil || !strings.Contains(err.Error(), "state restored") || unitState != "enabled-runtime" {
		t.Fatalf("runtime activation rollback = %q, %v", unitState, err)
	}
	if state := f.states["rashind.socket"]; !state.enabled || !state.running {
		t.Fatalf("runtime activation not restored: %+v", state)
	}
}

func TestHyakkiRetirementRefusesRestartedOwnerWithoutRollingBackMigration(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("development retirement deliberately refuses root")
	}
	f := newHyakkiMigrationFixture(t)
	binDir := filepath.Join(f.app.home(), "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	var retired []string
	for _, name := range []string{"rashind", "rashin", "rashin-app", "ryoku-rashin", "ryoku-ai-usage"} {
		retired = append(retired, filepath.Join(binDir, name))
	}
	dataDir := filepath.Join(f.app.home(), ".local", "share")
	for _, name := range []string{"rashin-app", "dev.ryoku.rashin"} {
		retired = append(retired, filepath.Join(dataDir, "applications", name+".desktop"))
		retired = append(retired, filepath.Join(dataDir, "icons", "hicolor", "scalable", "apps", name+".svg"))
	}
	unrelated := filepath.Join(binDir, "keep-me")
	for _, path := range append(retired, unrelated) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("original development file\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	base := f.runner.answer
	var restarted *os.File
	committed := filepath.Join(f.app.home(), "native-commit")
	f.runner.answer = func(c Command) Result {
		result := base(c)
		if c.Name == "hyakki" {
			if err := os.WriteFile(committed, []byte("native data migrated\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			restarted = hyakkiHeldLegacyLock(t, f.app, "app.lock")
		}
		return result
	}
	err := f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("unexpected wait") }, binDir)
	if err == nil || !strings.Contains(err.Error(), "data migration completed") || !strings.Contains(err.Error(), "predecessor restarted") {
		t.Fatalf("restarted owner retirement = %v", err)
	}
	if !fileExists(committed) || f.states["rashind.socket"].enabled || f.states["rashind.socket"].running {
		t.Fatal("post-commit retirement refusal rolled back native state or restored old activation")
	}
	for _, path := range retired {
		body, err := os.ReadFile(path)
		if err != nil || string(body) != "original development file\n" {
			t.Fatalf("restarted owner's file changed: %s: %q, %v", path, body, err)
		}
	}
	if err := syscall.Flock(int(restarted.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(binDir, "rashin")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("ryoku-rashin", filepath.Join(binDir, "rashin")); err != nil {
		t.Fatal(err)
	}
	f.runner.answer = func(c Command) Result {
		if c.Name == "systemctl" {
			for _, name := range []string{"app.lock", "rashind.lock", "config.lock"} {
				if busy, err := legacyHyakkiLockBusy(filepath.Join(f.app.legacyHyakkiRuntime(), name)); err != nil || !busy {
					t.Fatalf("retirement supervisor check not protected by %s: busy=%v err=%v", name, busy, err)
				}
			}
		}
		return base(c)
	}
	if err := f.app.retireLegacyHyakkiDevelopment(binDir, legacyHyakkiServices(Systemd)); err != nil {
		t.Fatal(err)
	}
	for _, path := range retired {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("known development file was not retired: %s: %v", path, err)
		}
	}
	if body, err := os.ReadFile(unrelated); err != nil || string(body) != "original development file\n" {
		t.Fatalf("unrelated development file changed: %q, %v", body, err)
	}
}

func TestHyakkiRetirementRejectsSymlinkFilesAndDirectories(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("development retirement deliberately refuses root")
	}
	for _, directoryLink := range []bool{false, true} {
		t.Run(fmt.Sprint(directoryLink), func(t *testing.T) {
			f := newHyakkiMigrationFixture(t)
			binDir := filepath.Join(f.app.home(), "bin")
			if err := os.Mkdir(binDir, 0o700); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(f.app.home(), "unrelated")
			if err := os.WriteFile(outside, []byte("keep outside file\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			original := filepath.Join(binDir, "rashind")
			if err := os.WriteFile(original, []byte("keep dev owner\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			if directoryLink {
				link := filepath.Join(f.app.home(), "linked-bin")
				if err := os.Symlink(binDir, link); err != nil {
					t.Fatal(err)
				}
				binDir = link
			} else if err := os.Symlink(outside, filepath.Join(binDir, "rashin")); err != nil {
				t.Fatal(err)
			}
			if err := f.app.retireLegacyHyakkiDevelopment(binDir, nil); err == nil {
				t.Fatal("unsafe retirement was accepted")
			}
			if body, err := os.ReadFile(outside); err != nil || string(body) != "keep outside file\n" {
				t.Fatalf("symlink target changed: %q, %v", body, err)
			}
			if body, err := os.ReadFile(original); err != nil || string(body) != "keep dev owner\n" {
				t.Fatalf("retirement partially removed development files: %q, %v", body, err)
			}
		})
	}
}

func TestHyakkiMigrationNativeOwnershipRefusalRestoresActivation(t *testing.T) {
	f := newHyakkiMigrationFixture(t)
	f.states["rashind.socket"] = hyakkiServiceState{true, true}
	base := f.runner.answer
	var contender *os.File
	f.runner.answer = func(c Command) Result {
		if c.Name == "hyakki" {
			contender = hyakkiHeldLegacyLock(t, f.app, "config.lock")
			result := base(c)
			result.Code = 1
			return result
		}
		return base(c)
	}
	err := f.app.migrateHyakkiQuiesced(f.snapshot(), nil, f.app.legacyHyakkiReleased, func(time.Duration) { t.Fatal("unexpected wait") }, "")
	if err == nil || !strings.Contains(err.Error(), "state restored") || contender == nil {
		t.Fatalf("native ownership refusal = %v", err)
	}
	if state := f.states["rashind.socket"]; !state.enabled || !state.running {
		t.Fatalf("native ownership refusal did not restore activation: %+v", state)
	}
	if busy, err := legacyHyakkiLockBusy(filepath.Join(f.app.legacyHyakkiRuntime(), "config.lock")); err != nil || !busy {
		t.Fatalf("native handoff contender lost its lock: busy=%v err=%v", busy, err)
	}
}

func TestHyakkiMigrationCLIRejectsAmbiguousRetirementArguments(t *testing.T) {
	for _, args := range [][]string{
		{"hyakki", "migrate", "--retire-dev-binaries"},
		{"hyakki", "migrate", "--retire-dev-binaries", "relative"},
		{"hyakki", "migrate", "--retire-dev-binaries", ""},
		{"hyakki", "migrate", "--retire-dev-binaries", "/tmp", "extra"},
		{"hyakki", "migrate", "--unknown", "/tmp"},
	} {
		f := newHyakkiMigrationFixture(t)
		if code := f.app.Execute(args); code != ExitUsage {
			t.Fatalf("%q returned %d, want usage error", args, code)
		}
		if f.migrations != 0 || len(f.runner.commands) != 0 {
			t.Fatalf("invalid CLI arguments caused migration: %q", args)
		}
	}
}

func TestHyakkiMigrationCLIExplicitDirectoryRetiresOnlyKnownEntrypoints(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("public migration deliberately refuses root")
	}
	f := newHyakkiMigrationFixture(t)
	binDir := filepath.Join(f.app.home(), "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rashind", "keep-me"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if code := f.app.Execute([]string{"hyakki", "migrate", "--retire-dev-binaries", binDir}); code != ExitOK {
		t.Fatalf("explicit development retirement exited %d", code)
	}
	if _, err := os.Lstat(filepath.Join(binDir, "rashind")); !os.IsNotExist(err) {
		t.Fatalf("known predecessor entrypoint survived: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(binDir, "keep-me")); err != nil || string(body) != "keep-me" {
		t.Fatalf("unrelated entrypoint changed: %q, %v", body, err)
	}
}
