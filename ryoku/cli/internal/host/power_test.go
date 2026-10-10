package host

import "testing"

func TestPowerUsesHostInitCommand(t *testing.T) {
	tests := []struct {
		init    string
		manager string
		command string
	}{
		{"systemd", "pacman", "systemctl"},
		{"runit", "xbps", "loginctl"},
	}
	for _, tc := range tests {
		t.Run(tc.init, func(t *testing.T) {
			for _, action := range []string{"poweroff", "reboot", "suspend", "hibernate"} {
				runner := &fakeRunner{}
				app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": tc.init, "RYOKU_HOST_PKGMGR": tc.manager})
				if code := app.Execute([]string{"power", action}); code != ExitOK {
					t.Fatalf("%s exit = %d", action, code)
				}
				if got := argv(runner.commands[0]); got != tc.command+" "+action {
					t.Fatalf("%s argv = %q", action, got)
				}
			}
		})
	}
}

func TestPowerRejectsUnknownActionAndPropagatesFailure(t *testing.T) {
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit", "RYOKU_HOST_PKGMGR": "xbps"})
	if code := app.Power([]string{"halt"}); code != ExitUsage {
		t.Fatalf("unknown action exit = %d", code)
	}
	if len(runner.commands) != 0 {
		t.Fatal("unknown action ran a command")
	}

	runner.results = []Result{{Code: 7}}
	if code := app.Power([]string{"suspend"}); code != ExitFailure {
		t.Fatalf("failed suspend exit = %d", code)
	}
}
