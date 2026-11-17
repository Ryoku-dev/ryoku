package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPickerImportDirsCoverSystemLibdirs(t *testing.T) {
	dirs := strings.Split(pickerImportDirs(), string(os.PathListSeparator))
	have := make(map[string]bool, len(dirs))
	for _, dir := range dirs {
		have[dir] = true
	}
	for _, want := range []string{"/usr/lib/qt6/qml", "/usr/lib64/qt6/qml"} {
		if !have[want] {
			t.Errorf("picker import dirs %q do not include %q", dirs, want)
		}
	}
}

func TestPickerImportDirsLetHomeBuildWinOnlyWhenPresent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	local := filepath.Join(home(), ".local", "lib", "qt6", "qml")
	first := func() string {
		return strings.Split(pickerImportDirs(), string(os.PathListSeparator))[0]
	}
	if got := first(); got != "/usr/lib/qt6/qml" {
		t.Fatalf("packaged picker: first import dir = %q, want the system dir", got)
	}
	if err := os.MkdirAll(filepath.Join(local, "Ryoku", "Ryogami"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "Ryoku", "Ryogami", "qmldir"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := first(); got != local {
		t.Fatalf("home-built picker: first import dir = %q, want %q", got, local)
	}
}

func TestConcurrentEagerLaunchAndEnsurePreserveOpenRequest(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("RYOGAMI_HEADLESS", "")
	m := &managedProcess{
		qml: "/tmp/ryogami-launch-race-test.qml",
		command: func(string, ...string) *exec.Cmd {
			return exec.Command("sleep", "30")
		},
	}
	t.Cleanup(m.kill)

	start := make(chan struct{})
	launchDone := make(chan struct{})
	ensureDone := make(chan bool)
	go func() {
		<-start
		m.launch()
		close(launchDone)
	}()
	go func() {
		<-start
		ensureDone <- m.ensure()
	}()
	close(start)

	select {
	case <-launchDone:
	case <-time.After(3 * time.Second):
		t.Fatal("eager launch did not finish")
	}
	select {
	case ready := <-ensureDone:
		if ready {
			t.Fatal("picker reported ready before subscribing")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ensure did not finish")
	}

	m.mu.Lock()
	cmd := m.cmd
	pending := m.pending.name
	m.mu.Unlock()
	if cmd == nil {
		t.Fatal("concurrent launch did not start a picker process")
	}
	visible := false
	for _, entry := range cmd.Env {
		if entry == "RYOGAMI_START_VISIBLE=1" {
			visible = true
			break
		}
	}
	if !visible && pending != "ryogami.wall.toggle" {
		t.Fatal("hidden eager launch won without retaining the open request")
	}
}

func TestSecondOpenDuringVisibleColdStartDoesNotQueueToggle(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("RYOGAMI_HEADLESS", "")
	m := &managedProcess{
		qml: "/tmp/ryogami-visible-launch-test.qml",
		command: func(string, ...string) *exec.Cmd {
			return exec.Command("sleep", "30")
		},
	}
	t.Cleanup(m.kill)

	if m.ensure() {
		t.Fatal("cold-started picker reported ready before subscribing")
	}
	if m.ensure() {
		t.Fatal("visible picker reported ready before subscribing")
	}
	delivered := 0
	m.subscribed(1, func(string, map[string]interface{}) {
		delivered++
	})
	if delivered != 0 {
		t.Fatalf("visible cold start delivered %d redundant toggle events", delivered)
	}
}

func TestRicePickerOpenSurvivesColdStart(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("RYOGAMI_HEADLESS", "")
	d := newSettingsDaemon(t)
	d.ui = &managedProcess{
		qml: "/tmp/ryogami-rice-launch-test.qml",
		command: func(string, ...string) *exec.Cmd {
			return exec.Command("sleep", "30")
		},
	}
	t.Cleanup(d.ui.kill)
	if got := d.wallpaperVerb("wallpaper ui rices"); got != "ok" {
		t.Fatal(got)
	}
	d.ui.mu.Lock()
	cmd := d.ui.cmd
	d.ui.mu.Unlock()
	if cmd == nil {
		t.Fatal("rice handoff did not start the picker")
	}
	if !strings.Contains(strings.Join(cmd.Env, "\n"), "RYOGAMI_START_RICES=1") {
		t.Fatal("cold picker did not receive the requested collection")
	}
	if got := d.wallpaperVerb("wallpaper ui rices"); got != "ok" {
		t.Fatal(got)
	}
	delivered := ""
	d.ui.subscribed(1, func(name string, _ map[string]interface{}) {
		delivered = name
	})
	if delivered != "ryogami.wall.rices" {
		t.Fatalf("handoff during startup delivered %q", delivered)
	}
}

func TestPickerParkDue(t *testing.T) {
	now := time.Unix(1000, 0)
	const grace = 60 * time.Second
	tests := []struct {
		name           string
		hidden, busy   bool
		grace, elapsed time.Duration
		want           bool
	}{
		{name: "sixty seconds hidden and idle", hidden: true, grace: grace, elapsed: grace, want: true},
		{name: "always resident", hidden: true, grace: 0, elapsed: 24 * time.Hour},
		{name: "visible", grace: grace, elapsed: grace},
		{name: "busy", hidden: true, busy: true, grace: grace, elapsed: grace},
		{name: "within grace", hidden: true, grace: grace, elapsed: grace - time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pickerParkDue(now.Add(-tc.elapsed), tc.hidden, tc.busy, tc.grace, now)
			if got != tc.want {
				t.Fatalf("pickerParkDue() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPickerIdleGraceReadsRyogamiSetting(t *testing.T) {
	d := newSettingsDaemon(t)
	if got := d.pickerIdleGrace(); got != 60*time.Second {
		t.Fatalf("default grace = %s, want 1m", got)
	}
	d.ui = &managedProcess{parkWake: make(chan struct{}, 1)}
	if err := d.setSetting(pickerReleaseSetting, float64(0)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-d.ui.parkWake:
	default:
		t.Fatal("grace change did not wake the park timer")
	}
	if got := d.pickerIdleGrace(); got != 0 {
		t.Fatalf("stored zero grace = %s, want disabled parking", got)
	}
	if err := d.setSetting(pickerReleaseSetting, float64(600)); err != nil {
		t.Fatal(err)
	}
	if got := d.pickerIdleGrace(); got != 10*time.Minute {
		t.Fatalf("stored grace = %s, want 10m", got)
	}
}

func TestEnsureWaitsForStoppingPickerBeforeColdStart(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("RYOGAMI_HEADLESS", "")

	oldDone := make(chan struct{})
	started := make(chan struct{}, 1)
	m := &managedProcess{
		cmd:      &exec.Cmd{},
		done:     oldDone,
		stopping: true,
		qml:      "/tmp/ryogami-stop-start-order-test.qml",
		command: func(string, ...string) *exec.Cmd {
			started <- struct{}{}
			return exec.Command("sleep", "30")
		},
	}
	t.Cleanup(m.kill)

	ensured := make(chan bool, 1)
	go func() { ensured <- m.ensure() }()

	select {
	case <-started:
		t.Fatal("replacement picker started before the stopping picker exited")
	case <-time.After(100 * time.Millisecond):
	}

	m.mu.Lock()
	m.cmd = nil
	m.done = nil
	m.stopping = false
	close(oldDone)
	m.mu.Unlock()

	select {
	case ready := <-ensured:
		if ready {
			t.Fatal("cold-started picker reported ready before subscribing")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ensure did not resume after the old picker exited")
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("replacement picker did not start after the old picker exited")
	}
}
