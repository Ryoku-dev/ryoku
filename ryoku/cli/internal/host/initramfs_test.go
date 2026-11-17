package host

import (
	"reflect"
	"testing"
)

func TestInitramfsRebuildBackends(t *testing.T) {
	tests := []struct {
		name     string
		manager  string
		paths    map[string]bool
		output   string
		commands []string
	}{
		{
			name: "pacman", manager: "pacman",
			paths:    map[string]bool{"mkinitcpio": true},
			commands: []string{"/usr/bin/mkinitcpio -P"},
		},
		{
			// a Limine UKI box has no presets, so mkinitcpio -P builds nothing
			name: "pacman with limine", manager: "pacman",
			paths:    map[string]bool{"mkinitcpio": true, "limine-mkinitcpio": true},
			commands: []string{"limine-mkinitcpio "},
		},
		{
			name: "dnf", manager: "dnf",
			paths:    map[string]bool{"dracut": true},
			commands: []string{"dracut --regenerate-all --force"},
		},
		{
			name: "xbps", manager: "xbps",
			paths:  map[string]bool{"xbps-query": true, "xbps-reconfigure": true},
			output: "ii linux6.12-6.12.54_1 kernel\nii linux6.6-6.6.90_1 kernel\nii linux6.12-headers-6.12.54_1 headers\nii base-system-0.114_1 base\n",
			commands: []string{
				"xbps-query -l",
				"xbps-reconfigure -f linux6.12",
				"xbps-reconfigure -f linux6.6",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeRunner{paths: test.paths, answer: func(command Command) Result {
				if argv(command) == "xbps-query -l" {
					return Result{Output: test.output}
				}
				return Result{}
			}}
			app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": test.manager})
			app.cfg.UID, app.cfg.UIDSet = 0, true
			if code := app.Execute([]string{"initramfs", "rebuild"}); code != ExitOK {
				t.Fatalf("initramfs rebuild exit = %d", code)
			}
			got := make([]string, len(runner.commands))
			for index := range runner.commands {
				got[index] = argv(runner.commands[index])
			}
			if !reflect.DeepEqual(got, test.commands) {
				t.Fatalf("commands = %v, want %v", got, test.commands)
			}
		})
	}
}

func TestInitramfsRebuildEscalatesOnce(t *testing.T) {
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	app.cfg.UID, app.cfg.UIDSet = 1000, true
	if code := app.Execute([]string{"initramfs", "rebuild"}); code != ExitOK {
		t.Fatalf("initramfs rebuild exit = %d", code)
	}
	if got := argv(runner.commands[0]); got != "pkexec ryoku-host initramfs rebuild" {
		t.Fatalf("escalation = %q", got)
	}
}

func TestInitramfsRebuildRejectsMissingBackend(t *testing.T) {
	app, _, stderr := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "dnf"})
	app.cfg.UID, app.cfg.UIDSet = 0, true
	if code := app.Execute([]string{"initramfs", "rebuild"}); code != ExitAbsent {
		t.Fatalf("missing dracut exit = %d, want %d", code, ExitAbsent)
	}
	if stderr.String() != "dracut is not installed\n" {
		t.Fatalf("missing dracut stderr = %q", stderr.String())
	}
}

func TestXBPSKernelPackages(t *testing.T) {
	got := xbpsKernelPackages("ii linux6.12-6.12.54_1\nii linux6.12-6.12.53_1\nii linux6.6-6.6.90_1\nii linux6.12-headers-6.12.54_1\n")
	want := []string{"linux6.12", "linux6.6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kernel packages = %v, want %v", got, want)
	}
}
