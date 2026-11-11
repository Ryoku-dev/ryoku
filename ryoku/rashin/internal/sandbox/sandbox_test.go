package sandbox

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// probeEnv is set on the host side of every probe, so a test can show Unset
// removes it.
const probeEnv = "RASHIN_SANDBOX_PROBE_UNSET"

// tree makes a test root and a Spec rooted in it. The root is Writable
// because it sits under the private /tmp the sandbox would otherwise hide.
func tree(t *testing.T) (string, Spec) {
	t.Helper()
	if err := Available(); err != nil {
		t.Skipf("bubblewrap cannot run here: %s", describe(err))
	}
	root := t.TempDir()
	spec := Spec{
		Folder:     mkdir(t, root, "folder"),
		Writable:   []string{root},
		RuntimeDir: mkdir(t, root, "run"),
	}
	return root, spec
}

// describe renders an Available error with its cause.
func describe(err error) string {
	if cause := errors.Unwrap(err); cause != nil {
		return err.Error() + " (" + cause.Error() + ")"
	}
	return err.Error()
}

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(parts...)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// probe runs script under /bin/sh inside the sandbox with args as $1...
// and returns its trimmed output.
func probe(t *testing.T, spec Spec, script string, args ...string) string {
	t.Helper()
	argv, err := Command(spec, append([]string{"/bin/sh", "-c", script, "probe"}, args...))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), probeEnv+"=host")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sandbox probe failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestHiddenPathsReadEmpty(t *testing.T) {
	root, spec := tree(t)
	secrets := mkdir(t, root, "secrets")
	writeFile(t, filepath.Join(secrets, "key"), "secret")
	token := filepath.Join(root, "token")
	writeFile(t, token, "secret")
	spec.Hidden = []string{secrets, token, filepath.Join(root, "missing")}

	got := probe(t, spec, `printf '%s|%s' "$(ls -A "$1")" "$(cat "$2")"`, secrets, token)
	if got != "|" {
		t.Fatalf("hidden paths read %q, want both empty", got)
	}
	if content, err := os.ReadFile(token); err != nil || string(content) != "secret" {
		t.Fatalf("hidden file on the host changed: %q, %v", content, err)
	}
}

func TestFolderIsWritableAndTheRestReadOnly(t *testing.T) {
	_, spec := tree(t)
	spec.Writable = nil
	outside := readOnlyCandidate(t)
	stray := filepath.Join(outside, ".rashin-sandbox-probe")
	t.Cleanup(func() { os.Remove(stray) })

	got := probe(t, spec, `
		touch "$1/made" && echo folder
		test -d "$2" && ! test -w "$2" && echo read-only
		mkdir "$3" 2>/dev/null || echo refused
	`, spec.Folder, outside, stray)
	if got != "folder\nread-only\nrefused" {
		t.Fatalf("probe said %q", got)
	}
	if _, err := os.Stat(filepath.Join(spec.Folder, "made")); err != nil {
		t.Fatalf("file written in the folder is not on the host: %v", err)
	}
}

// readOnlyCandidate is a directory the test may write on the host and the
// sandbox sees through its read-only root: not under the private /tmp or
// /dev/shm.
func readOnlyCandidate(t *testing.T) string {
	t.Helper()
	var candidates []string
	if dir, err := os.Getwd(); err == nil {
		candidates = append(candidates, dir)
	}
	if dir, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, dir)
	}
	private := []string{"/tmp", "/dev/shm", filepath.Clean(os.TempDir())}
	for _, dir := range candidates {
		masked := slices.ContainsFunc(private, func(p string) bool { return dir == p || inside(dir, p) })
		if !masked && syscall.Access(dir, writeOK) == nil {
			return dir
		}
	}
	t.Skip("no host-writable directory outside /tmp and /dev/shm to probe")
	return ""
}

// writeOK is access(2)'s W_OK.
const writeOK = 0x2

// TestReadOnlyShowsAPrivatePath runs from the test root, which sits under the
// private /tmp unless TMPDIR moves it, as a daemon run from /tmp does.
func TestReadOnlyShowsAPrivatePath(t *testing.T) {
	root, spec := tree(t)
	spec.Writable = nil
	bin := mkdir(t, root, "bin")
	writeFile(t, filepath.Join(bin, "rashin"), "binary")
	spec.ReadOnly = []string{bin, filepath.Join(root, "missing")}

	got := probe(t, spec, `
		cat "$1/rashin"; echo
		touch "$1/new" 2>/dev/null || echo refused
		test -w "$1/rashin" || echo read-only
	`, bin)
	if got != "binary\nrefused\nread-only" {
		t.Fatalf("probe said %q", got)
	}
	if _, err := os.Stat(filepath.Join(bin, "new")); err == nil {
		t.Fatal("the sandbox wrote into a ReadOnly directory")
	}
}

func TestFolderInsideHiddenIsVisible(t *testing.T) {
	root, spec := tree(t)
	teams := mkdir(t, root, "teams")
	writeFile(t, filepath.Join(mkdir(t, teams, "other"), "transcript"), "secret")
	spec.Folder = mkdir(t, teams, "mine")
	writeFile(t, filepath.Join(spec.Folder, "own"), "mine")
	spec.Hidden = []string{teams}

	got := probe(t, spec, `printf '%s|%s|' "$(ls -A "$1")" "$(cat own)"; touch new && echo wrote`, teams)
	if got != "mine|mine|wrote" {
		t.Fatalf("probe said %q", got)
	}
	if _, err := os.Stat(filepath.Join(spec.Folder, "new")); err != nil {
		t.Fatalf("file written in the folder is not on the host: %v", err)
	}
}

func TestHiddenInsideWritableStaysHidden(t *testing.T) {
	root, spec := tree(t)
	cache := mkdir(t, root, "cache")
	rashin := mkdir(t, cache, "rashin")
	writeFile(t, filepath.Join(rashin, "index"), "secret")
	spec.Writable = []string{root, cache}
	spec.Hidden = []string{rashin}

	got := probe(t, spec, `printf '%s|' "$(ls -A "$2")"; touch "$1/tool" && echo wrote`, cache, rashin)
	if got != "|wrote" {
		t.Fatalf("probe said %q", got)
	}
}

func TestRuntimeDirHoldsOnlyBinds(t *testing.T) {
	root, spec := tree(t)
	writeFile(t, filepath.Join(spec.RuntimeDir, "bus"), "host")
	socket := filepath.Join(mkdir(t, root, "sock"), "agent.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	spec.Binds = []Bind{{Source: socket, Target: filepath.Join(spec.RuntimeDir, "rashin", "agent.sock")}}

	got := probe(t, spec, `printf '%s|%s|' "$(ls -A "$1")" "$(stat -c %a "$1")"; test -S "$1/rashin/agent.sock" && echo socket`, spec.RuntimeDir)
	if got != "rashin|700|socket" {
		t.Fatalf("probe said %q", got)
	}
}

func TestEnvironmentAndStartDir(t *testing.T) {
	_, spec := tree(t)
	spec.Env = map[string]string{"RASHIN_SANDBOX_PROBE_SET": "inside"}
	spec.Unset = []string{probeEnv}

	got := probe(t, spec, `printf '%s|%s|%s' "${`+probeEnv+`-absent}" "$RASHIN_SANDBOX_PROBE_SET" "$(pwd -P)"`)
	folder, err := filepath.EvalSymlinks(spec.Folder)
	if err != nil {
		t.Fatal(err)
	}
	if want := "absent|inside|" + folder; got != want {
		t.Fatalf("probe said %q, want %q", got, want)
	}
}

func TestOwnPIDNamespace(t *testing.T) {
	_, spec := tree(t)
	got := probe(t, spec, `cat /proc/1/comm; test -e "/proc/$1" && echo host-visible || echo own`, strconv.Itoa(os.Getpid()))
	if got != "bwrap\nown" {
		t.Fatalf("probe said %q", got)
	}
}

func TestDevicesOnlyWhenAsked(t *testing.T) {
	_, spec := tree(t)
	if got := probe(t, spec, `test -e /dev/dri && echo present || echo absent`); got != "absent" {
		t.Fatalf("without Devices /dev/dri is %s", got)
	}
	if _, err := os.Stat("/dev/dri"); err != nil {
		return
	}
	spec.Devices = true
	if got := probe(t, spec, `test -e /dev/dri && echo present || echo absent`); got != "present" {
		t.Fatalf("with Devices /dev/dri is %s", got)
	}
}

func TestCommandRefusesBadSpecs(t *testing.T) {
	root := t.TempDir()
	good := Spec{Folder: root, RuntimeDir: filepath.Join(root, "run")}
	missing := filepath.Join(root, "missing")
	for name, spec := range map[string]Spec{
		"missing folder":      {Folder: missing, RuntimeDir: good.RuntimeDir},
		"missing writable":    {Folder: root, RuntimeDir: good.RuntimeDir, Writable: []string{missing}},
		"missing bind source": {Folder: root, RuntimeDir: good.RuntimeDir, Binds: []Bind{{Source: missing, Target: "/x"}}},
		"relative bind":       {Folder: root, RuntimeDir: good.RuntimeDir, Binds: []Bind{{Source: root, Target: "x"}}},
		"no runtime dir":      {Folder: root},
		"relative hidden":     {Folder: root, RuntimeDir: good.RuntimeDir, Hidden: []string{"secrets"}},
		"relative read-only":  {Folder: root, RuntimeDir: good.RuntimeDir, ReadOnly: []string{"bin"}},
	} {
		if _, err := Command(spec, []string{"true"}); err == nil {
			t.Errorf("%s: Command accepted it", name)
		}
	}
	argv, err := Command(good, []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	if argv[0] != "bwrap" || argv[len(argv)-2] != "--" || argv[len(argv)-1] != "true" {
		t.Fatalf("argv = %q", argv)
	}
}
