package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-cli/internal/host"
)

func memoryFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", "memory", name)
}

func TestParseMeminfoFixture(t *testing.T) {
	raw, err := os.ReadFile(memoryFixture(t, "meminfo"))
	if err != nil {
		t.Fatal(err)
	}
	values := parseMeminfo(string(raw))
	want := map[string]uint64{
		"MemTotal":     16384000,
		"MemAvailable": 6291456,
		"SwapTotal":    12582912,
		"SwapFree":     10485760,
	}
	for key, value := range want {
		if values[key] != value {
			t.Errorf("%s = %d, want %d", key, values[key], value)
		}
	}
}

func TestParseSwapDevicesFixture(t *testing.T) {
	raw, err := os.ReadFile(memoryFixture(t, "swaps"))
	if err != nil {
		t.Fatal(err)
	}
	devices := parseSwapDevices(string(raw))
	if len(devices) != 2 {
		t.Fatalf("got %d swap devices, want 2", len(devices))
	}
	if devices[0].name != "/dev/zram0" || devices[0].sizeKiB != 8388608 || devices[0].usedKiB != 1048576 {
		t.Errorf("zram device = %+v", devices[0])
	}
}

func TestParseMemoryEventsFixture(t *testing.T) {
	raw, err := os.ReadFile(memoryFixture(t, "memory.events"))
	if err != nil {
		t.Fatal(err)
	}
	values := parseMemoryEvents(string(raw))
	if values["oom"] != 2 || values["oom_kill"] != 1 || values["high"] != 12 {
		t.Errorf("memory events = %#v", values)
	}
}

func TestCgroupProcessesFixture(t *testing.T) {
	root := memoryFixture(t, "")
	processes := cgroupProcesses(filepath.Join(root, "cgroup"), filepath.Join(root, "proc"))
	want := []memoryProcess{
		{pid: 101, name: "qs", rssKiB: 921600},
		{pid: 202, name: "electron", rssKiB: 524288},
		{pid: 303, name: "python3", rssKiB: 65536},
		{pid: 404, name: "ryoku-shell", rssKiB: 16384},
	}
	if !reflect.DeepEqual(processes, want) {
		t.Fatalf("processes = %#v, want %#v", processes, want)
	}
	apps := userAppsInShell(processes, 0)
	if len(apps) != 1 || apps[0].name != "electron" {
		t.Fatalf("user apps = %#v, want electron only", apps)
	}
	if got := memoryProcessList(processes, 2); got != "qs 900 MiB, electron 512 MiB" {
		t.Errorf("top RSS = %q", got)
	}
}

func TestLooksLikeUserApp(t *testing.T) {
	tests := []struct {
		name    string
		userApp bool
	}{
		{name: "qs"},
		{name: "ryoku-shell"},
		{name: "ryogami"},
		{name: "ryogami-live"},
		{name: "wl-clip-persist"},
		{name: "python3"},
		{name: "python3.13"},
		{name: "adb"},
		{name: "cliphist"},
		{name: "flock"},
		{name: "bash"},
		{name: "sh"},
		{name: "dbus-monitor"},
		{name: "ryoku"},
		{name: "lspci"},
		{name: "grep"},
		{name: "cava", userApp: true},
		{name: "inotifywait", userApp: true},
		{name: "electron", userApp: true},
		{name: "firefox", userApp: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := looksLikeUserApp(test.name); got != test.userApp {
				t.Errorf("looksLikeUserApp(%q) = %t, want %t", test.name, got, test.userApp)
			}
		})
	}
}

func TestShellMemoryHelperMatchesShellWatchers(t *testing.T) {
	joined := func(argv ...string) string {
		return strings.Join(append(append([]string{}, argv...), ""), "\x00") + "\x00"
	}
	helpers := [][]string{
		{"python3", "-u", "-c", "# ryoku-python kb_locks watcher\nimport sys\n"},
		{"python3", "-u", "/home/u/.local/state/ryoku/store/unpacked/barstyles/iris/0f3a/qml/inir/scripts/daemon/keyboard_lock_state_daemon.py"},
		{"sh", "/home/u/.local/state/ryoku/store/unpacked/subshells/caelestia/9b1c/qml/scripts/poll.sh"},
		{"python3", "-u", "-c", "import glob\nlast_caps = read_sysfs(\"/sys/class/leds/*capslock*/brightness\")\n"},
		{"bash", "/home/u/.config/quickshell/shell/watchers/kb_locks.sh", "watch"},
		{"inotifywait", "-m", "-e", "modify", "/home/u/.cache/ryoku/python/recording"},
		{"cava", "-p", "/run/user/1000/ryoku-cava-101.conf"},
	}
	for _, argv := range helpers {
		if !isShellMemoryHelper(joined(argv...)) {
			t.Errorf("doctor misses the shell helper %q", argv[0]+" "+argv[len(argv)-1])
		}
	}
	notHelpers := [][]string{
		{"python3", "-m", "http.server"},
		{"bash", "-c", "sleep 1"},
		{"inotifywait", "-m", "/home/u/projects"},
		{"cava"},
	}
	for _, argv := range notHelpers {
		if isShellMemoryHelper(joined(argv...)) {
			t.Errorf("doctor ignores an unrelated %q", argv[0])
		}
	}
}

func TestUserAppsInShellExcludesShellOwnedCavaAndInotify(t *testing.T) {
	processes := []memoryProcess{
		{pid: 100, name: "cava", cmdline: "cava\x00-p\x00/run/user/1000/ryoku-cava-100.conf\x00"},
		{pid: 200, name: "inotifywait", cmdline: "inotifywait\x00-m\x00/home/u/.cache/ryoku/python/recording\x00"},
		{pid: 300, name: "cava", cmdline: "cava\x00"},
		{pid: 400, name: "inotifywait", cmdline: "inotifywait\x00-m\x00/home/u/projects\x00"},
	}
	want := processes[2:]
	if got := userAppsInShell(processes, 0); !reflect.DeepEqual(got, want) {
		t.Fatalf("user apps = %#v, want %#v", got, want)
	}
}

func TestUserAppsInShellExcludesDoctorProcessTree(t *testing.T) {
	processes := []memoryProcess{
		{pid: 100, ppid: 10, name: "ryoku"},
		{pid: 200, ppid: 100, name: "electron"},
		{pid: 300, ppid: 200, name: "kitty"},
		{pid: 400, ppid: 10, name: "firefox"},
	}
	tests := []struct {
		name      string
		doctorPID int
		want      []memoryProcess
	}{
		{
			name:      "doctor outside cgroup",
			doctorPID: 999,
			want:      processes[1:],
		},
		{
			name:      "doctor inside cgroup",
			doctorPID: 100,
			want:      processes[3:],
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := userAppsInShell(processes, test.doctorPID); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("user apps = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestMemoryReportLines(t *testing.T) {
	diagnostic := memoryDiagnostics{
		systemd:         true,
		ramTotalKiB:     16384000,
		ramAvailableKiB: 6291456,
		swapTotalKiB:    12582912,
		swapFreeKiB:     10485760,
		zramTotalKiB:    8388608,
		zramUsedKiB:     1048576,
		zramDevices:     1,
		swappiness:      "150",
		oomd:            "active",
		cgroupFound:     true,
		cgroupCurrent:   1600000000,
		cgroupPeak:      12000000000,
		cgroupOOMKills:  1,
		processes:       []memoryProcess{{name: "qs", rssKiB: 921600}},
		kernelOOMKills:  2,
		kernelOOMKnown:  true,
	}
	got := strings.Join(memoryReportLines(diagnostic), "\n")
	for _, want := range []string{
		"RAM: 16000 MiB total, 6144 MiB available",
		"swap: 12288 MiB total, 2048 MiB used",
		"zram: 8192 MiB total, 1024 MiB used (1 device(s))",
		"vm.swappiness: 150",
		"systemd-oomd: active",
		"ryoku-shell.service: 1526 MiB current, 11444 MiB peak, 1 OOM kill(s)",
		"top RSS in ryoku-shell.service: qs 900 MiB",
		"kernel OOM kills this boot: 2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("memory report missing %q:\n%s", want, got)
		}
	}
}

func TestVoidMemoryDiagnosticsSkipsSystemd(t *testing.T) {
	paths := memoryPaths{
		meminfo:    memoryFixture(t, "meminfo"),
		swaps:      memoryFixture(t, "swaps"),
		swappiness: filepath.Join(t.TempDir(), "missing"),
		cgroupRoot: filepath.Join(t.TempDir(), "cgroup"),
		procRoot:   filepath.Join(t.TempDir(), "proc"),
	}
	diagnostic := gatherMemoryDiagnostics(host.Runit, paths)
	lines := strings.Join(memoryReportLines(diagnostic), "\n")
	if strings.Contains(lines, "systemd-oomd") || strings.Contains(lines, "ryoku-shell.service") {
		t.Fatalf("Void memory report contains systemd details:\n%s", lines)
	}
}
