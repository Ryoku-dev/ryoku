package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func phonePackageTable(t *testing.T, edition string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), edition+".tsv")
	header := "arch\t" + edition + "\tlanes\tnotes\n"
	v4l2 := "v4l2-native"
	if edition == "fedora" {
		v4l2 = "v4l2-native v4l2-dkms-native"
	}
	rows := []string{
		"kdeconnect\tkde-native\toptional\t",
		"scrcpy\tscrcpy-native\toptional\t",
		"android-tools\tadb-native\toptional\t",
		"python-gobject\tgobject-native\toptional\t",
		"python-dbus\tdbus-native\toptional\t",
		"avahi\tavahi-native\toptional\t",
		"python-pillow\tpillow-native\toptional\t",
		"v4l2loopback-dkms\t" + v4l2 + "\toptional\t",
		"linux-headers\theaders-a headers-b\toptional\t",
	}
	if err := os.WriteFile(path, []byte(header+strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func decodePhonePlan(t *testing.T, body string) phonePlan {
	t.Helper()
	var plan phonePlan
	if err := json.Unmarshal([]byte(body), &plan); err != nil {
		t.Fatalf("decode plan %q: %v", body, err)
	}
	return plan
}

func phoneKernelModules(t *testing.T, kernels map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for version, pkgbase := range kernels {
		dir := filepath.Join(root, version)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if pkgbase != "" {
			if err := os.WriteFile(filepath.Join(dir, "pkgbase"), []byte(pkgbase+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func TestPhonePacmanKernelHeaders(t *testing.T) {
	cases := []struct {
		name    string
		kernels map[string]string
		headers []string
	}{
		{
			name: "cachyos with nvidia modules",
			kernels: map[string]string{
				"7.3.0-4-cachyos":     "linux-cachyos",
				"nvidia-open-7.3.0-4": "",
				"zfs-7.3.0-4":         "",
			},
			headers: []string{"linux-cachyos-headers"},
		},
		{
			name:    "stock kernel",
			kernels: map[string]string{"7.2.11-arch1-1": "linux"},
			headers: []string{"linux-headers"},
		},
		{
			name: "two kernels",
			kernels: map[string]string{
				"6.12.1-1-lts": "linux-lts",
				"7.3.0-4-zen":  "linux-zen",
			},
			headers: []string{"linux-lts-headers", "linux-zen-headers"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, stdout, stderr := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
			app.cfg.KernelModulesDir = phoneKernelModules(t, tc.kernels)
			if code := app.Phone([]string{"plan-enable"}); code != ExitOK {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			want := append(append([]string{}, phoneBasePackages...), tc.headers...)
			plan := decodePhonePlan(t, stdout.String())
			if !reflect.DeepEqual(plan.Packages, want) {
				t.Fatalf("packages = %#v, want %#v", plan.Packages, want)
			}
		})
	}
}

func TestPhoneEnablePlansExactTransactionPerEdition(t *testing.T) {
	cases := []struct {
		name    string
		manager string
		paths   map[string]bool
		want    [][]string
	}{
		{
			name: "arch", manager: "pacman", paths: map[string]bool{},
			want: [][]string{
				{"pacman", "-S", "--needed", "--noconfirm", "kdeconnect", "scrcpy", "android-tools", "python-gobject", "python-dbus", "avahi", "python-pillow", "v4l2loopback-dkms", "linux-cachyos-headers"},
				{"modprobe", "v4l2loopback"},
			},
		},
		{
			name: "fedora", manager: "dnf", paths: map[string]bool{"dnf5": true},
			want: [][]string{
				{"install", "-Dm644", "<generated>", "COPR/ryoku-copr-boobaa-scrcpy.repo"},
				{"install", "-Dm644", "<generated>", "COPR/ryoku-copr-kuya-carlo-v4l2loopback.repo"},
				{"dnf5", "install", "-y", "--setopt=install_weak_deps=False", "kde-native", "scrcpy-native", "adb-native", "gobject-native", "dbus-native", "avahi-native", "pillow-native", "v4l2-native", "v4l2-dkms-native", "headers-a", "headers-b"},
				{"modprobe", "v4l2loopback"},
			},
		},
		{
			name: "void", manager: "xbps", paths: map[string]bool{},
			want: [][]string{
				{"xbps-install", "-Sy", "kde-native", "scrcpy-native", "adb-native", "gobject-native", "dbus-native", "avahi-native", "pillow-native", "v4l2-native", "headers-a", "headers-b"},
				{"modprobe", "v4l2loopback"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{paths: tc.paths}
			env := map[string]string{"RYOKU_HOST_PKGMGR": tc.manager}
			if tc.manager != "pacman" {
				env["RYOKU_HOST_PKG_TABLE"] = phonePackageTable(t, tc.name)
			}
			app, stdout, stderr := testApp(runner, env)
			app.cfg.KernelModulesDir = phoneKernelModules(t, map[string]string{
				"7.3.0-4-cachyos": "linux-cachyos",
				"nvidia-open":     "",
			})
			app.cfg.PhoneModulesLoadPath = "ETC/modules-load.d/phone-v4l2loopback.conf"
			app.cfg.PhoneModprobePath = "ETC/modprobe.d/phone-v4l2loopback.conf"
			app.cfg.PhoneStatePath = filepath.Join(t.TempDir(), "missing-state")
			app.cfg.PhoneCoprDir = "COPR"
			if code := app.Phone([]string{"plan-enable"}); code != ExitOK {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			plan := decodePhonePlan(t, stdout.String())
			if !reflect.DeepEqual(plan.Commands, tc.want) {
				t.Fatalf("commands = %#v, want %#v", plan.Commands, tc.want)
			}
			if plan.Writes["ETC/modules-load.d/phone-v4l2loopback.conf"] != phoneModulesLoad || plan.Writes["ETC/modprobe.d/phone-v4l2loopback.conf"] != phoneModprobe {
				t.Fatalf("privileged files = %#v", plan.Writes)
			}
			wantRemoves := []string{"ETC/modules-load.d/ryoku-phone.conf", "ETC/modprobe.d/ryoku-phone.conf"}
			if !reflect.DeepEqual(plan.Removes, wantRemoves) {
				t.Fatalf("legacy removals = %#v, want %#v", plan.Removes, wantRemoves)
			}
		})
	}
}

func TestPhoneDisablePlanWithoutInstallRecordRemovesNoPackages(t *testing.T) {
	runner := &fakeRunner{paths: map[string]bool{"yay": true}}
	app, stdout, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	app.cfg.PhoneStatePath = filepath.Join(t.TempDir(), "absent")
	app.cfg.PhoneModulesLoadPath = "ETC/modules-load.d/phone-v4l2loopback.conf"
	app.cfg.PhoneModprobePath = "ETC/modprobe.d/phone-v4l2loopback.conf"
	if code := app.Phone([]string{"plan-disable"}); code != ExitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	plan := decodePhonePlan(t, stdout.String())
	want := [][]string{{"modprobe", "-r", "v4l2loopback"}}
	if !reflect.DeepEqual(plan.Commands, want) {
		t.Fatalf("commands = %#v, want %#v", plan.Commands, want)
	}
	wantRemoves := []string{
		"ETC/modules-load.d/phone-v4l2loopback.conf",
		"ETC/modprobe.d/phone-v4l2loopback.conf",
		app.cfg.PhoneStatePath,
		"ETC/modules-load.d/ryoku-phone.conf",
		"ETC/modprobe.d/ryoku-phone.conf",
	}
	if !reflect.DeepEqual(plan.Removes, wantRemoves) {
		t.Fatalf("removes = %#v, want %#v", plan.Removes, wantRemoves)
	}
}

func TestPhonePackageStatePreservesPreexistingPackages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phone-packages")
	app := New(Config{PhoneStatePath: path})
	if err := app.writePhoneState([]string{"scrcpy", "kdeconnect", "scrcpy"}); err != nil {
		t.Fatal(err)
	}
	if err := app.writePhoneState([]string{"android-tools"}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "android-tools\nkdeconnect\nscrcpy\n"; got != want {
		t.Fatalf("state = %q, want %q", got, want)
	}
}

func TestPhoneEnableMigratesLegacyConfigAndPreservesPackageRecord(t *testing.T) {
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules-load.d")
	modprobeDir := filepath.Join(root, "modprobe.d")
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(modprobeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyModules := filepath.Join(modulesDir, "ryoku-phone.conf")
	legacyModprobe := filepath.Join(modprobeDir, "ryoku-phone.conf")
	if err := os.WriteFile(legacyModules, []byte(phoneModulesLoad), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyModprobe, []byte(phoneModprobe), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "phone-packages")
	if err := os.WriteFile(statePath, []byte("scrcpy\nkdeconnect\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{answer: func(command Command) Result {
		switch argv(command) {
		case "scrcpy --version":
			return Result{Output: "scrcpy 3.3\n"}
		case "pacman -Q --quiet android-tools":
			return Result{Code: 1}
		default:
			return Result{}
		}
	}}
	app, _, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	app.cfg.PhoneModulesLoadPath = filepath.Join(modulesDir, "phone-v4l2loopback.conf")
	app.cfg.PhoneModprobePath = filepath.Join(modprobeDir, "phone-v4l2loopback.conf")
	app.cfg.PhoneStatePath = statePath

	if code := app.Phone([]string{"enable"}); code != ExitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	for _, path := range []string{legacyModules, legacyModprobe} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("legacy config %s still exists", path)
		}
	}
	for path, want := range map[string]string{
		app.cfg.PhoneModulesLoadPath: phoneModulesLoad,
		app.cfg.PhoneModprobePath:    phoneModprobe,
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != want {
			t.Fatalf("%s = %q, want %q", path, body, want)
		}
	}
	body, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "android-tools\nkdeconnect\nscrcpy\n"; got != want {
		t.Fatalf("state = %q, want %q", got, want)
	}
}

func TestPhoneDisableRemovesCurrentAndLegacyConfig(t *testing.T) {
	root := t.TempDir()
	modulesDir := filepath.Join(root, "modules-load.d")
	modprobeDir := filepath.Join(root, "modprobe.d")
	paths := []string{
		filepath.Join(modulesDir, "phone-v4l2loopback.conf"),
		filepath.Join(modprobeDir, "phone-v4l2loopback.conf"),
		filepath.Join(modulesDir, "ryoku-phone.conf"),
		filepath.Join(modprobeDir, "ryoku-phone.conf"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("phone\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	runner := &fakeRunner{}
	app, _, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	app.cfg.PhoneModulesLoadPath = paths[0]
	app.cfg.PhoneModprobePath = paths[1]
	app.cfg.PhoneStatePath = filepath.Join(root, "missing-state")
	if code := app.Phone([]string{"disable"}); code != ExitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("phone config %s still exists", path)
		}
	}
}
