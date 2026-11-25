package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeManifestFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "services.tsv")
	header := "id\tscope\tactivation\tsystemd_unit\trunit_service\tuser_disable_rule\n"
	if err := os.WriteFile(path, []byte(header+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func touchFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

type systemdManifestFixture struct {
	app         *App
	runner      *fakeRunner
	root        string
	home        string
	state       string
	systemUnits string
	userUnits   string
	runtime     string
}

func newSystemdManifestFixture(t *testing.T, rows string, extraEnv map[string]string) systemdManifestFixture {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	state := filepath.Join(root, "state")
	env := map[string]string{
		"RYOKU_HOST_INIT": "systemd",
		"HOME":            home,
		"XDG_STATE_HOME":  state,
	}
	for name, value := range extraEnv {
		env[name] = value
	}
	runner := &fakeRunner{}
	app, _, _ := testApp(runner, env)
	app.cfg.UIDSet, app.cfg.UID = true, 0
	app.cfg.ServiceManifest = writeManifestFixture(t, rows)
	app.cfg.SystemdSystemDir = filepath.Join(root, "system")
	app.cfg.SystemdUserDir = filepath.Join(root, "user")
	app.cfg.SystemdRuntimeDir = filepath.Join(root, "run", "systemd", "system")
	return systemdManifestFixture{
		app: app, runner: runner, root: root, home: home, state: state,
		systemUnits: app.cfg.SystemdSystemDir,
		userUnits:   app.cfg.SystemdUserDir,
		runtime:     app.cfg.SystemdRuntimeDir,
	}
}

func manifestCommandLines(runner *fakeRunner) string {
	var commands []string
	for _, command := range runner.commands {
		commands = append(commands, argv(command))
	}
	return strings.Join(commands, "\n")
}

func TestSystemdManifestSystemReconcileRestoresGlobalEnableWithoutStartingBootOrConditional(t *testing.T) {
	globalMarker := filepath.Join(t.TempDir(), "global-enabled")
	armed := filepath.Join(t.TempDir(), "armed")
	disarmed := filepath.Join(t.TempDir(), "disarmed")
	fixture := newSystemdManifestFixture(t, strings.Join([]string{
		"boot\tsystem\tboot\tboot.service\t-\tnone",
		"conditional-on\tsystem\tconditional\tconditional-on.service\t-\tcondition:" + armed,
		"conditional-off\tsystem\tconditional\tconditional-off.service\t-\tcondition:" + disarmed,
		"required-global\tuser\tsession-global\trequired-global.service\t-\tnone",
		"global\tuser\tsession\tglobal.service\t-\tpreserve:" + globalMarker,
	}, "\n")+"\n", nil)
	for _, path := range []string{
		filepath.Join(fixture.systemUnits, "boot.service"),
		filepath.Join(fixture.systemUnits, "conditional-on.service"),
		filepath.Join(fixture.systemUnits, "conditional-off.service"),
		filepath.Join(fixture.userUnits, "required-global.service"),
		filepath.Join(fixture.userUnits, "global.service"),
	} {
		touchFixture(t, path)
	}
	touchFixture(t, armed)
	if err := os.MkdirAll(fixture.runtime, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture.runner.answer = func(command Command) Result {
		got := argv(command)
		if got == "systemctl is-enabled --quiet conditional-off.service" {
			return Result{}
		}
		if strings.Contains(got, " is-enabled ") {
			return Result{Code: 1}
		}
		return Result{}
	}

	if code := fixture.app.Service([]string{"reconcile", "--system"}); code != ExitOK {
		t.Fatalf("system reconcile exit = %d", code)
	}
	got := manifestCommandLines(fixture.runner)
	for _, want := range []string{
		"systemctl enable boot.service",
		"systemctl enable conditional-on.service",
		"systemctl disable conditional-off.service",
		"systemctl --global is-enabled --quiet required-global.service",
		"systemctl --global enable required-global.service",
		"systemctl --global enable global.service",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("system commands = %q; missing %q", got, want)
		}
	}
	if strings.Contains(got, "--now") {
		t.Fatalf("boot or conditional unit was started during a package transaction: %q", got)
	}
	if !fileExists(globalMarker) {
		t.Fatal("global user activation marker was not written")
	}
}

func TestSystemdManifestSystemCheckDoesNotRequireRoot(t *testing.T) {
	fixture := newSystemdManifestFixture(t,
		"shell\tuser\tsession-global\tshell.service\t-\tnone\n", nil)
	fixture.app.cfg.UID = 1000
	touchFixture(t, filepath.Join(fixture.userUnits, "shell.service"))
	fixture.runner.answer = func(command Command) Result {
		if strings.Contains(argv(command), "is-enabled") {
			return Result{Code: 1}
		}
		return Result{}
	}

	if code := fixture.app.Service([]string{"reconcile", "--system", "--check"}); code != ExitFalse {
		t.Fatalf("unprivileged system check exit = %d", code)
	}
	if got := manifestCommandLines(fixture.runner); strings.Contains(got, " enable ") {
		t.Fatalf("check-only changed service state: %q", got)
	}
}

func TestSystemdManifestBootEnabledButInactiveNeedsNoAction(t *testing.T) {
	fixture := newSystemdManifestFixture(t, "boot\tsystem\tboot\tboot.service\t-\tnone\n", nil)
	touchFixture(t, filepath.Join(fixture.systemUnits, "boot.service"))
	if err := os.MkdirAll(fixture.runtime, 0o755); err != nil {
		t.Fatal(err)
	}

	if code := fixture.app.Service([]string{"reconcile", "--system"}); code != ExitOK {
		t.Fatalf("system reconcile exit = %d", code)
	}
	if got := manifestCommandLines(fixture.runner); got != "systemctl is-enabled --quiet boot.service" {
		t.Fatalf("enabled boot service should not be started: %q", got)
	}
}

func TestSystemdManifestExistingUserBaselinePreservesChoices(t *testing.T) {
	fixture := newSystemdManifestFixture(t, strings.Join([]string{
		"idle\tuser\tsession\tidle.service\t-\tpreserve",
		"optional\tuser\tsession\toptional.service\t-\tpreserve",
	}, "\n")+"\n", map[string]string{"WAYLAND_DISPLAY": "wayland-1"})
	touchFixture(t, filepath.Join(fixture.state, "ryoku", "welcome-seen"))
	touchFixture(t, filepath.Join(fixture.userUnits, "idle.service"))
	touchFixture(t, filepath.Join(fixture.userUnits, "optional.service"))
	if code := fixture.app.Service([]string{"reconcile", "--user", "--check"}); code != ExitFalse {
		t.Fatalf("baseline check exit = %d", code)
	}
	if fileExists(filepath.Join(fixture.state, "ryoku", "service-manifest")) {
		t.Fatal("check-only baseline mutated user state")
	}

	if code := fixture.app.Service([]string{"reconcile", "--user"}); code != ExitOK {
		t.Fatalf("user reconcile exit = %d", code)
	}
	if len(fixture.runner.commands) != 0 {
		t.Fatalf("existing user's services were re-enabled: %q", manifestCommandLines(fixture.runner))
	}
	for _, id := range []string{"idle", "optional"} {
		if !fileExists(filepath.Join(fixture.state, "ryoku", "service-manifest", id+"-enabled")) {
			t.Fatalf("baseline marker for %s was not written", id)
		}
	}
}

func TestSystemdManifestFreshUserEnablesWithoutWayland(t *testing.T) {
	fixture := newSystemdManifestFixture(t, "shell\tuser\tsession\tshell.service\t-\tpreserve\n", nil)
	touchFixture(t, filepath.Join(fixture.userUnits, "shell.service"))

	if code := fixture.app.Service([]string{"reconcile", "--user"}); code != ExitOK {
		t.Fatalf("user reconcile exit = %d", code)
	}
	if got := manifestCommandLines(fixture.runner); got != "systemctl --user enable shell.service" {
		t.Fatalf("fresh TTY user commands = %q", got)
	}
}

func TestSystemdManifestBaselineWithoutSessionRowsSettles(t *testing.T) {
	fixture := newSystemdManifestFixture(t,
		"shell\tuser\tsession-global\tshell.service\t-\tnone\n", nil)
	touchFixture(t, filepath.Join(fixture.state, "ryoku", "welcome-seen"))
	touchFixture(t, filepath.Join(fixture.userUnits, "shell.service"))

	if code := fixture.app.Service([]string{"reconcile", "--user"}); code != ExitOK {
		t.Fatalf("user reconcile exit = %d", code)
	}
	if code := fixture.app.Service([]string{"reconcile", "--user", "--check"}); code != ExitOK {
		t.Fatalf("check after a clean reconcile exit = %d", code)
	}
}

func TestSystemdManifestExistingBaselineEnablesLaterRows(t *testing.T) {
	fixture := newSystemdManifestFixture(t, "idle\tuser\tsession\tidle.service\t-\tpreserve\n", nil)
	touchFixture(t, filepath.Join(fixture.state, "ryoku", "welcome-seen"))
	touchFixture(t, filepath.Join(fixture.userUnits, "idle.service"))
	if code := fixture.app.Service([]string{"reconcile", "--user"}); code != ExitOK {
		t.Fatalf("baseline reconcile exit = %d", code)
	}

	fixture.app.cfg.ServiceManifest = writeManifestFixture(t, strings.Join([]string{
		"idle\tuser\tsession\tidle.service\t-\tpreserve",
		"added\tuser\tsession\tadded.service\t-\tpreserve",
	}, "\n")+"\n")
	touchFixture(t, filepath.Join(fixture.userUnits, "added.service"))
	fixture.runner.commands = nil
	if code := fixture.app.Service([]string{"reconcile", "--user"}); code != ExitOK {
		t.Fatalf("updated reconcile exit = %d", code)
	}
	if got := manifestCommandLines(fixture.runner); got != "systemctl --user enable added.service" {
		t.Fatalf("new manifest row commands = %q", got)
	}
}

func TestSystemdManifestUserStartsOnlyWithWayland(t *testing.T) {
	fixture := newSystemdManifestFixture(t, "shell\tuser\tsession\tshell.service\t-\tpreserve\n",
		map[string]string{"WAYLAND_DISPLAY": "wayland-1"})
	touchFixture(t, filepath.Join(fixture.userUnits, "shell.service"))

	if code := fixture.app.Service([]string{"reconcile", "--user"}); code != ExitOK {
		t.Fatalf("user reconcile exit = %d", code)
	}
	if got := manifestCommandLines(fixture.runner); got != "systemctl --user enable --now shell.service" {
		t.Fatalf("graphical user commands = %q", got)
	}
}

func TestRunitManifestReconcileWritesRosterAndStartsOnlyNewSessionServices(t *testing.T) {
	root := t.TempDir()
	manifest := writeManifestFixture(t, strings.Join([]string{
		"existing\tuser\tsession\t-\texisting\tpreserve",
		"added\tuser\tsession-global\t-\tadded\tnone",
		"required\tuser\tsession-global\t-\trequired\tnone",
		"disabled\tuser\tsession\t-\tdisabled\tpreserve",
		"manual\tuser\ton-demand\t-\tmanual\tpreserve",
	}, "\n")+"\n")
	serviceRoot := filepath.Join(root, "service")
	for _, name := range []string{"existing", "added", "required", "disabled", "manual"} {
		touchFixture(t, filepath.Join(serviceRoot, name, "down"))
		touchFixture(t, filepath.Join(serviceRoot, name, "supervise", "stat"))
	}
	touchFixture(t, filepath.Join(serviceRoot, "disabled", runitUserDisabledMarker))
	touchFixture(t, filepath.Join(serviceRoot, "required", runitUserDisabledMarker))
	touchFixture(t, filepath.Join(serviceRoot, ".ryoku-session-services"))
	if err := os.WriteFile(filepath.Join(serviceRoot, ".ryoku-session-services"), []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_INIT": "runit", "HOME": filepath.Join(root, "home")})
	app.cfg.ServiceManifest = manifest
	app.cfg.UserServiceDir = serviceRoot
	if code := app.Service([]string{"reconcile", "--user"}); code != ExitOK {
		t.Fatalf("reconcile exit = %d", code)
	}
	body, err := os.ReadFile(filepath.Join(serviceRoot, ".ryoku-session-services"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "existing\nadded\nrequired\n"; got != want {
		t.Fatalf("roster = %q, want %q", got, want)
	}
	if got, want := manifestCommandLines(runner),
		"sv up "+filepath.Join(serviceRoot, "added")+"\nsv up "+filepath.Join(serviceRoot, "required"); got != want {
		t.Fatalf("runit commands = %q, want %q", got, want)
	}
	for _, name := range []string{"added", "required"} {
		if _, err := os.Stat(filepath.Join(serviceRoot, name, "down")); !os.IsNotExist(err) {
			t.Fatalf("required session service %s stayed parked: %v", name, err)
		}
	}
	for _, name := range []string{"disabled", "manual"} {
		if _, err := os.Stat(filepath.Join(serviceRoot, name, "down")); err != nil {
			t.Fatalf("%s was enabled: %v", name, err)
		}
	}
}

func TestRunitSystemManifestReconcileLinksBootServicesOnce(t *testing.T) {
	root := t.TempDir()
	manifest := writeManifestFixture(t, strings.Join([]string{
		"always\tsystem\tboot\t-\talways\tnone",
		"once\tsystem\tenable-once\t-\tonce\tpreserve",
		"manual\tsystem\ton-demand\t-\tmanual\tpreserve",
	}, "\n")+"\n")
	definitions := filepath.Join(root, "sv")
	for _, name := range []string{"always", "once", "manual"} {
		if err := os.MkdirAll(filepath.Join(definitions, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	app, _, _ := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_INIT": "runit"})
	app.cfg.UIDSet, app.cfg.UID = true, 0
	app.cfg.ServiceManifest = manifest
	app.cfg.SystemServiceDir = definitions
	app.cfg.SystemBootDir = filepath.Join(root, "boot")
	app.cfg.SystemLiveDir = filepath.Join(root, "live")
	app.cfg.ServiceStateDir = filepath.Join(root, "state")
	if code := app.Service([]string{"reconcile", "--system"}); code != ExitOK {
		t.Fatalf("reconcile exit = %d", code)
	}
	for _, name := range []string{"always", "once"} {
		if target, err := os.Readlink(filepath.Join(app.cfg.SystemBootDir, name)); err != nil || target != filepath.Join(definitions, name) {
			t.Fatalf("%s link = %q, %v", name, target, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(app.cfg.SystemBootDir, "manual")); !os.IsNotExist(err) {
		t.Fatalf("on-demand service was enabled: %v", err)
	}
}
