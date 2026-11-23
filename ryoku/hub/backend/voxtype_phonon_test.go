package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledPhononIsLocalAndSelectable(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte("#!/bin/sh\nif [ \"$1\" = init ]; then echo systemd; fi\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if _, ok := presetByKey(phononPresetKey); !ok || phononInstalled() {
		t.Fatal("Phonon must remain available to install without claiming it is installed")
	}
	for _, path := range []string{filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), phononUnitPath(), phononReceiptPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	p, ok := presetByKey(phononPresetKey)
	if !ok || p.Cloud || p.KeyKind != "" || p.Language != "English" {
		t.Fatalf("invalid installed Phonon preset: %#v", p)
	}
	presets := voxtypePresets()
	for i, preset := range presets {
		if preset.Key == phononPresetKey && (i == 0 || presets[i-1].Key != "whisper-fast") {
			t.Fatal("Phonon must appear directly after Base English")
		}
	}
	config := buildVoxtypeConfig(p, "cloud-secret")
	for _, want := range []string{"mode = \"remote\"", "\nmodel = \"phonon-2\"\n", "remote_endpoint = \"http://127.0.0.1:8010\"", "remote_model = \"phonon-2\"", "language = \"en\"", "max_duration_secs = 120"} {
		if !strings.Contains(config, want) {
			t.Fatalf("missing %s in config", want)
		}
	}
	if selectedPreset(config) != phononPresetKey || strings.Contains(config, "cloud-secret") {
		t.Fatal("selection did not round trip or cloud credentials leaked to local engine")
	}
	if err := configurePhononService(true); err != nil {
		t.Fatal(err)
	}
	dropin := filepath.Join(configHome(), "systemd/user/voxtype.service.d/ryoku-phonon.conf")
	if !strings.Contains(readFileString(dropin), "Requires=phonon-2.service") {
		t.Fatal("Phonon selection did not establish its service dependency")
	}
	if err := configurePhononService(false); err != nil {
		t.Fatal(err)
	}
	if fileExists(dropin) {
		t.Fatal("switching away retained the Phonon dependency")
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":false,"openaiKey":"cloud-secret"}`); err != nil {
		t.Fatal(err)
	}
	if storedVoxtypeOpenAIKey() != "cloud-secret" || strings.Contains(readFileString(voxtypeConfigPath()), "cloud-secret") {
		t.Fatal("cloud key was lost or included in localhost configuration")
	}
	if err := voxtypeSet(`{"preset":"openai","enabled":false}`); err != nil {
		t.Fatal(err)
	}
	if extractConfigString(readFileString(voxtypeConfigPath()), "remote_api_key") != "cloud-secret" {
		t.Fatal("switching back to OpenAI lost its saved key")
	}
	openaiPreset, _ := presetByKey("openai")
	if err := os.WriteFile(voxtypeConfigPath(), []byte(buildVoxtypeConfig(openaiPreset, "")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := voxtypeSet(`{"preset":"whisper-fast","enabled":false}`); err != nil {
		t.Fatal(err)
	}
	if fileExists(voxtypeOpenAIKeyPath()) || storedVoxtypeOpenAIKey() != "" {
		t.Fatal("a key removed from the active OpenAI config was restored from the stale private copy")
	}
	previous := readFileString(voxtypeConfigPath())
	failingHost := "#!/bin/sh\nif [ \"$1\" = init ]; then echo systemd; fi\nif [ \"$*\" = 'svc --user enable --now phonon-2' ]; then exit 1; fi\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(failingHost), 0700); err != nil {
		t.Fatal(err)
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":true}`); err == nil {
		t.Fatal("accepted a failed Phonon server start")
	}
	if readFileString(voxtypeConfigPath()) != previous || fileExists(dropin) {
		t.Fatal("a failed Phonon start changed the current preset or its dependency")
	}
}

func TestRunitPhononIsSelectableAndUsesPackagedUserService(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte("#!/bin/sh\nif [ \"$1\" = init ]; then echo runit; exit 0; fi\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	serviceRun := filepath.Join(phononRunitServicePath(), "run")
	for _, path := range []string{filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), serviceRun, phononReceiptPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if !phononInstalled() {
		t.Fatal("Phonon should be installed when its runtime, model and runit service are present")
	}
	presets := voxtypePresets()
	for i, preset := range presets {
		if preset.Key == phononPresetKey && (i == 0 || presets[i-1].Key != "whisper-fast") {
			t.Fatal("Phonon must appear directly after Base English on runit")
		}
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":false}`); err != nil {
		t.Fatal(err)
	}
	if selectedPreset(readFileString(voxtypeConfigPath())) != phononPresetKey || fileExists(phononUnitPath()) {
		t.Fatal("runit selection did not write the local Phonon config without a systemd unit")
	}
}

func TestFailedPhononSelectionRestoresConfigDependenciesAndServices(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	state := filepath.Join(root, "service-state")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	writeFakeVoxtypeSystemdHost(t, bin, state, true)
	for _, path := range []string{
		filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), phononUnitPath(), phononReceiptPath(),
		reduxModelPath(), reduxAdapterPath(), reduxReceiptPath(), filepath.Join(configHome(), "systemd/user/voxtype-redux.service"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{voxtypeUnitPath(), reduxDependencyDropinPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("previous "+filepath.Base(path)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	base, _ := presetByKey("whisper-fast")
	previousConfig := buildVoxtypeConfig(base, "")
	if err := os.MkdirAll(filepath.Dir(voxtypeConfigPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeConfigPath(), []byte(previousConfig), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"voxtype", "voxtype-redux"} {
		for _, stateName := range []string{"enabled", "active"} {
			path := filepath.Join(state, stateName, name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "model": phononPresetKey})
	}))
	defer server.Close()
	previousEndpoint := phononEndpoint
	phononEndpoint = server.URL
	t.Cleanup(func() { phononEndpoint = previousEndpoint })

	if err := voxtypeSet(`{"preset":"phonon-2","enabled":true}`); err == nil {
		t.Fatal("accepted a selection after Redux disable reported failure")
	}
	if got := readFileString(voxtypeConfigPath()); got != previousConfig {
		t.Fatalf("failed selection changed Voxtype config:\n%s", got)
	}
	if got := readFileString(reduxDependencyDropinPath()); got != "previous ryoku-redux.conf" {
		t.Fatalf("failed selection did not restore Redux dependency: %q", got)
	}
	if fileExists(phononDependencyDropinPath()) || fileExists(voxtypeResilienceDropinPath()) {
		t.Fatal("failed selection left a new dependency or resilience drop-in")
	}
	for _, name := range []string{"phonon-2", "voxtype-redux", "voxtype"} {
		wantEnabled := name != "phonon-2"
		wantActive := name != "phonon-2"
		for _, stateName := range []struct {
			name string
			want bool
		}{{"enabled", wantEnabled}, {"active", wantActive}} {
			got := fileExists(filepath.Join(state, stateName.name, name))
			if got != stateName.want {
				t.Errorf("%s state for %s = %t, want %t", stateName.name, name, got, stateName.want)
			}
		}
	}
	assertVoxtypeRestartCount(t, state, 2)
}

func TestDisablingCustomVoxtypeStopsPhononWithoutChangingConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	state := filepath.Join(root, "service-state")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	writeFakeVoxtypeSystemdHost(t, bin, state, false)
	if err := os.MkdirAll(filepath.Dir(phononUnitPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(phononUnitPath(), []byte("[Service]\nExecStart=/bin/true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config := "engine = \"parakeet\"\n[parakeet]\nmodel = \"user-edited-model\"\n"
	if err := os.MkdirAll(filepath.Dir(voxtypeConfigPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeConfigPath(), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	for _, stateName := range []string{"enabled", "active"} {
		path := filepath.Join(state, stateName, "phonon-2")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	if err := voxtypeSet(`{"preset":"custom","enabled":false}`); err != nil {
		t.Fatal(err)
	}
	if got := readFileString(voxtypeConfigPath()); got != config {
		t.Fatalf("disabling Custom changed the user's config:\n%s", got)
	}
	for _, stateName := range []string{"enabled", "active"} {
		if fileExists(filepath.Join(state, stateName, "phonon-2")) {
			t.Errorf("disabling Custom left Phonon %s", stateName)
		}
	}
}

func TestVoxtypeConfigUsesEditedPhononEndpoint(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"custom model and path on local server", "[whisper]\nmode = \"remote\"\nremote_model = \"user-model\"\nremote_endpoint = \"http://localhost:8010/v1\"\n", true},
		{"non-loopback server", "[whisper]\nmode = \"remote\"\nremote_endpoint = \"http://example.com:8010\"\n", false},
		{"local engine with stale endpoint", "[whisper]\nmode = \"local\"\nremote_endpoint = \"http://127.0.0.1:8010\"\n", false},
		{"invalid config", "[whisper\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := voxtypeConfigUsesPhononEndpoint(tt.text); got != tt.want {
				t.Fatalf("config uses Phonon = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRemovePhononRedirectsCustomConfigAndKeepsVoxtypeEnabled(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	state := filepath.Join(root, "service-state")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	writeFakeVoxtypeSystemdHost(t, bin, state, false)
	actions := filepath.Join(state, "setup-actions")
	t.Setenv("PHONON_TEST_ACTIONS", actions)
	python := "#!/bin/sh\nprintf '%s\\n' \"$2\" >> \"$PHONON_TEST_ACTIONS\"\n"
	if err := os.WriteFile(filepath.Join(bin, "python3"), []byte(python), 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{phononUnitPath(), phononReceiptPath(), voxtypeUnitPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	customConfig := "engine = \"whisper\"\n[whisper]\nmode = \"remote\"\nmodel = \"custom-endpoint\"\nremote_model = \"edited-model\"\nremote_endpoint = \"http://127.0.0.1:8010/v1\"\n"
	if err := os.MkdirAll(filepath.Dir(voxtypeConfigPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeConfigPath(), []byte(customConfig), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"phonon-2", "voxtype"} {
		for _, stateName := range []string{"enabled", "active"} {
			path := filepath.Join(state, stateName, name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := removePhonon(); err != nil {
		t.Fatal(err)
	}
	if got := selectedPreset(readFileString(voxtypeConfigPath())); got != "whisper-fast" {
		t.Fatalf("uninstall left Voxtype selected on the removed server: %q", got)
	}
	for _, stateName := range []string{"enabled", "active"} {
		if !fileExists(filepath.Join(state, stateName, "voxtype")) {
			t.Errorf("uninstall disabled Voxtype %s", stateName)
		}
		if fileExists(filepath.Join(state, stateName, "phonon-2")) {
			t.Errorf("uninstall left Phonon %s", stateName)
		}
	}
	if got := readFileString(actions); got != "check-remove\nremove\n" {
		t.Fatalf("unexpected installer lifecycle: %q", got)
	}
}

func TestFailedSelectionRemovesNewVoxtypeUnitAndState(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	state := filepath.Join(root, "service-state")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	writeFakeVoxtypeSystemdHost(t, bin, state, false)
	if err := os.WriteFile(filepath.Join(state, "fail-voxtype-restart"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	voxtype := `#!/bin/sh
if [ "$1" = setup ] && [ "$2" = systemd ]; then
  mkdir -p "$XDG_CONFIG_HOME/systemd/user"
  printf '[Service]\nExecStart=/bin/true\n' > "$XDG_CONFIG_HOME/systemd/user/voxtype.service"
fi
`
	if err := os.WriteFile(filepath.Join(bin, "voxtype"), []byte(voxtype), 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), phononUnitPath(), phononReceiptPath(),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	base, _ := presetByKey("whisper-fast")
	previousConfig := buildVoxtypeConfig(base, "")
	if err := os.MkdirAll(filepath.Dir(voxtypeConfigPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeConfigPath(), []byte(previousConfig), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "model": phononPresetKey})
	}))
	defer server.Close()
	previousEndpoint := phononEndpoint
	phononEndpoint = server.URL
	t.Cleanup(func() { phononEndpoint = previousEndpoint })

	if err := voxtypeSet(`{"preset":"phonon-2","enabled":true}`); err == nil {
		t.Fatal("accepted a selection after the newly generated Voxtype service failed to restart")
	}
	if got := readFileString(voxtypeConfigPath()); got != previousConfig {
		t.Fatalf("failed selection changed Voxtype config:\n%s", got)
	}
	for _, path := range []string{voxtypeUnitPath(), voxtypeResilienceDropinPath(), phononDependencyDropinPath()} {
		if fileExists(path) {
			t.Errorf("failed selection left generated file %s", path)
		}
	}
	for _, name := range []string{"phonon-2", "voxtype"} {
		for _, stateName := range []string{"enabled", "active"} {
			if fileExists(filepath.Join(state, stateName, name)) {
				t.Errorf("failed selection left %s %s", name, stateName)
			}
		}
	}
}

func TestFailedRunitPhononSelectionRestoresServiceStates(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	serviceDir := filepath.Join(root, "services")
	t.Setenv("RYOKU_USER_SERVICE_DIR", serviceDir)
	bin := filepath.Join(root, "bin")
	state := filepath.Join(root, "service-state")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	writeFakeVoxtypeRunitHost(t, bin, state)
	for _, name := range []string{"phonon-2", "voxtype"} {
		run := filepath.Join(serviceDir, name, "run")
		if err := os.MkdirAll(filepath.Dir(run), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(run, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), phononReceiptPath(),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	base, _ := presetByKey("whisper-fast")
	previousConfig := buildVoxtypeConfig(base, "")
	if err := os.MkdirAll(filepath.Dir(voxtypeConfigPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeConfigPath(), []byte(previousConfig), 0600); err != nil {
		t.Fatal(err)
	}
	for _, stateName := range []string{"enabled", "active"} {
		path := filepath.Join(state, stateName, "voxtype")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(state, "fail-voxtype-restart"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "model": phononPresetKey})
	}))
	defer server.Close()
	previousEndpoint := phononEndpoint
	phononEndpoint = server.URL
	t.Cleanup(func() { phononEndpoint = previousEndpoint })

	if err := voxtypeSet(`{"preset":"phonon-2","enabled":true}`); err == nil {
		t.Fatal("accepted a selection after the Voxtype runit restart failed")
	}
	if got := readFileString(voxtypeConfigPath()); got != previousConfig {
		t.Fatalf("failed runit selection changed Voxtype config:\n%s", got)
	}
	for _, name := range []string{"phonon-2", "voxtype"} {
		wantActive := name == "voxtype"
		wantEnabled := name == "voxtype"
		for _, stateName := range []struct {
			name string
			want bool
		}{{"enabled", wantEnabled}, {"active", wantActive}} {
			got := fileExists(filepath.Join(state, stateName.name, name))
			if got != stateName.want {
				t.Errorf("runit %s state for %s = %t, want %t", stateName.name, name, got, stateName.want)
			}
		}
	}
	assertVoxtypeRestartCount(t, state, 2)
}

func writeFakeVoxtypeSystemdHost(t *testing.T, bin, state string, failReduxDisable bool) {
	t.Helper()
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	host := `#!/bin/sh
state_dir='` + state + `'
if [ "$1" = init ]; then echo systemd; exit 0; fi
if [ "$1" != svc ] || [ "$2" != --user ]; then exit 0; fi
verb=$3
case "$verb" in
  is-enabled|is-active)
    name=$4
    state_name=enabled
    [ "$verb" = is-active ] && state_name=active
    [ -e "$state_dir/$state_name/$name" ] && exit 0
    exit 3
    ;;
  enable|disable)
    now=false
    if [ "$4" = --now ]; then name=$5; now=true; else name=$4; fi
    if [ "$verb" = enable ]; then
      mkdir -p "$state_dir/enabled" "$state_dir/active"
      touch "$state_dir/enabled/$name"
      [ "$now" = true ] && touch "$state_dir/active/$name"
      exit 0
    fi
    rm -f "$state_dir/enabled/$name"
    if [ "$now" = true ]; then rm -f "$state_dir/active/$name"; fi
    if [ "$name" = voxtype-redux ] && [ -e "$state_dir/fail-redux-disable" ] && [ ! -e "$state_dir/redux-disable-failed" ]; then
      touch "$state_dir/redux-disable-failed"
      exit 1
    fi
    exit 0
    ;;
  start|restart)
    name=$4
    mkdir -p "$state_dir/active"
    touch "$state_dir/active/$name"
    if [ "$verb" = restart ] && [ "$name" = voxtype ]; then printf 'restart\n' >> "$state_dir/voxtype-restarts"; fi
    if [ "$verb" = restart ] && [ "$name" = voxtype ] && [ -e "$state_dir/fail-voxtype-restart" ] && [ ! -e "$state_dir/voxtype-restart-failed" ]; then
      touch "$state_dir/voxtype-restart-failed"
      exit 1
    fi
    exit 0
    ;;
  stop)
    rm -f "$state_dir/active/$4"
    exit 0
    ;;
  *) exit 0 ;;
esac
`
	if failReduxDisable {
		if err := os.WriteFile(filepath.Join(state, "fail-redux-disable"), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(host), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
}

func writeFakeVoxtypeRunitHost(t *testing.T, bin, state string) {
	t.Helper()
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	host := `#!/bin/sh
state_dir='` + state + `'
if [ "$1" = init ]; then echo runit; exit 0; fi
if [ "$1" != svc ] || [ "$2" != --user ]; then exit 0; fi
verb=$3
case "$verb" in
  is-enabled|is-active)
    name=$4
    state_name=enabled
    [ "$verb" = is-active ] && state_name=active
    [ -e "$state_dir/$state_name/$name" ] && exit 0
    exit 3
    ;;
  enable|disable)
    now=false
    if [ "$4" = --now ]; then name=$5; now=true; else name=$4; fi
    if [ "$verb" = enable ]; then
      mkdir -p "$state_dir/enabled" "$state_dir/active"
      touch "$state_dir/enabled/$name"
      [ "$now" = true ] && touch "$state_dir/active/$name"
      exit 0
    fi
    rm -f "$state_dir/enabled/$name"
    if [ "$now" = true ]; then rm -f "$state_dir/active/$name"; fi
    exit 0
    ;;
  start|restart)
    name=$4
    mkdir -p "$state_dir/active"
    touch "$state_dir/active/$name"
    if [ "$verb" = restart ] && [ "$name" = voxtype ]; then printf 'restart\n' >> "$state_dir/voxtype-restarts"; fi
    if [ "$verb" = restart ] && [ "$name" = voxtype ] && [ -e "$state_dir/fail-voxtype-restart" ] && [ ! -e "$state_dir/voxtype-restart-failed" ]; then
      touch "$state_dir/voxtype-restart-failed"
      exit 1
    fi
    exit 0
    ;;
  stop)
    rm -f "$state_dir/active/$4"
    exit 0
    ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(host), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
}

func assertVoxtypeRestartCount(t *testing.T, state string, want int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(state, "voxtype-restarts"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "restart\n"); got != want {
		t.Fatalf("Voxtype restart count = %d, want %d", got, want)
	}
}

func TestFailedPhononSelectionRestoresPreviouslyStoppedService(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	state := filepath.Join(root, "phonon-state")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	writeFakeVoxtypeSystemdHost(t, bin, state, false)
	for _, path := range []string{filepath.Join(dataHome(), "phonon-2/venv/bin/phonon"), phononModelPath(), phononUnitPath(), phononReceiptPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "model": phononPresetKey})
	}))
	defer server.Close()
	previousEndpoint := phononEndpoint
	phononEndpoint = server.URL
	t.Cleanup(func() { phononEndpoint = previousEndpoint })
	configDir := filepath.Join(configHome(), "voxtype")
	if err := os.MkdirAll(filepath.Dir(configDir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configDir, []byte("blocks the config directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":true}`); err == nil {
		t.Fatal("accepted a config write through a file that blocks its parent directory")
	}
	if fileExists(filepath.Join(state, "enabled", "phonon-2")) || fileExists(filepath.Join(state, "active", "phonon-2")) {
		t.Fatal("failed Phonon selection left its newly started service enabled or active")
	}
	if err := os.WriteFile(filepath.Join(state, "active", "phonon-2"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := voxtypeSet(`{"preset":"phonon-2","enabled":true}`); err == nil {
		t.Fatal("accepted a config write while restoring a previously active Phonon server")
	}
	if !fileExists(filepath.Join(state, "active", "phonon-2")) || fileExists(filepath.Join(state, "enabled", "phonon-2")) {
		t.Fatal("failed selection changed the prior active-but-disabled Phonon service state")
	}
}

func TestStoredOpenAIKeyUsesActiveVoxtypeConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	configPath := voxtypeConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	preset, ok := presetByKey("openai")
	if !ok {
		t.Fatal("OpenAI preset unavailable")
	}
	if err := os.WriteFile(configPath, []byte(buildVoxtypeConfig(preset, "edited-key")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeOpenAIKeyPath(), []byte("stale-key"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := storedVoxtypeOpenAIKey(); got != "edited-key" {
		t.Fatalf("saved key = %q, want active config value", got)
	}
}

func TestStoredOpenAIKeyHonorsClearedActiveConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	configPath := voxtypeConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	preset, ok := presetByKey("openai")
	if !ok {
		t.Fatal("OpenAI preset unavailable")
	}
	if err := os.WriteFile(configPath, []byte(buildVoxtypeConfig(preset, "")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeOpenAIKeyPath(), []byte("stale-key"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := storedVoxtypeOpenAIKey(); got != "" {
		t.Fatalf("saved key = %q after it was cleared from active config", got)
	}
}
