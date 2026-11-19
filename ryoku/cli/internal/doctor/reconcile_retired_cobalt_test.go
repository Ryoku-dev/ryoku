package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCobaltDocker is a docker CLI stand-in. Its daemon state lives as marker
// files in state/ (reachable, container, image, image-busy) and every call is
// appended to state/log, so a test can assert exactly what doctor asked docker.
const fakeCobaltDocker = `#!/bin/sh
state="$FAKE_DOCKER_STATE"
printf '%s\n' "$*" >>"$state/log"
case "$1 $2" in
"info "*)
  [ -e "$state/reachable" ] && exit 0
  echo "permission denied while trying to connect to the Docker daemon socket" >&2
  exit 1 ;;
"container inspect")
  [ "$5" = ryoku-cobalt ] && [ -e "$state/container" ] && { echo /ryoku-cobalt; exit 0; }
  echo "Error: No such container: $5" >&2
  exit 1 ;;
"container rm")
  [ "$4" = ryoku-cobalt ] && rm -f "$state/container" && exit 0
  exit 1 ;;
"image inspect")
  [ -e "$state/image" ] && { echo sha256:1; exit 0; }
  exit 1 ;;
"image rm")
  [ -e "$state/image-busy" ] && { echo "Error: image is being used by stopped container 0abc" >&2; exit 1; }
  rm -f "$state/image"
  exit 0 ;;
esac
echo "unexpected docker call: $*" >&2
exit 2
`

type cobaltFixture struct {
	state string
	files []string
}

func newCobaltFixture(t *testing.T, docker bool, markers ...string) cobaltFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	bin := filepath.Join(home, "bin")
	state := filepath.Join(home, "docker-state")
	for _, d := range []string{bin, state} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("FAKE_DOCKER_STATE", state)
	if docker {
		if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(fakeCobaltDocker), 0o755); err != nil {
			t.Fatal(err)
		}
		// the stub needs rm; nothing else on PATH may answer to "docker".
		t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	} else {
		t.Setenv("PATH", bin)
	}
	for _, m := range markers {
		seedFile(t, filepath.Join(state, m))
	}

	files := retiredCobaltFiles()
	seedFile(t, files[0])
	seedFile(t, filepath.Join(files[1], "0123abcd.json"))
	return cobaltFixture{state: state, files: files}
}

func (f cobaltFixture) calls(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.state, "log"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func (f cobaltFixture) has(marker string) bool {
	_, err := os.Stat(filepath.Join(f.state, marker))
	return err == nil
}

func (f cobaltFixture) filesGone(t *testing.T) bool {
	t.Helper()
	for _, p := range f.files {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (f cobaltFixture) filesPresent(t *testing.T) bool {
	t.Helper()
	for _, p := range f.files {
		if _, err := os.Lstat(p); err != nil {
			return false
		}
	}
	return true
}

// Doctor drives docker through the user's CLI only: no sudo, no service
// management, and no container or image but Ryoku's own.
func assertCobaltCallsScoped(t *testing.T, calls []string) {
	t.Helper()
	for _, c := range calls {
		switch {
		case c == "info",
			c == "container inspect --format {{.Name}} ryoku-cobalt",
			c == "container rm --force ryoku-cobalt",
			c == "image inspect --format {{.Id}} ghcr.io/imputnet/cobalt:11",
			c == "image rm ghcr.io/imputnet/cobalt:11":
		default:
			t.Fatalf("doctor made an out-of-scope docker call: %q", c)
		}
	}
}

func TestReconcileRetiredCobaltCleanBoxNeverRunsDocker(t *testing.T) {
	f := newCobaltFixture(t, true, "reachable", "container", "image")
	for _, p := range f.files {
		if err := os.RemoveAll(p); err != nil {
			t.Fatal(err)
		}
	}
	for _, checkOnly := range []bool{true, false} {
		if r := reconcileRetiredCobalt(checkOnly); r.status != recOK {
			t.Fatalf("checkOnly=%v: status=%s want ok (detail=%s)", checkOnly, r.status.label(), r.detail)
		}
	}
	if calls := f.calls(t); len(calls) != 0 {
		t.Fatalf("a box without the engine's files must not run docker, got %q", calls)
	}
	if !f.has("container") || !f.has("image") {
		t.Fatal("a box without the engine's files lost its container or image")
	}
}

func TestReconcileRetiredCobaltReachable(t *testing.T) {
	f := newCobaltFixture(t, true, "reachable", "container", "image")

	r := reconcileRetiredCobalt(true)
	if r.status != recWouldFix || !strings.Contains(r.detail, "ryoku-cobalt") {
		t.Fatalf("check: status=%s detail=%q, want would-fix naming the container", r.status.label(), r.detail)
	}
	if !f.filesPresent(t) || !f.has("container") || !f.has("image") {
		t.Fatal("check-only changed something")
	}
	for _, c := range f.calls(t) {
		if strings.Contains(c, " rm ") {
			t.Fatalf("check-only removed through docker: %q", c)
		}
	}

	r = reconcileRetiredCobalt(false)
	if r.status != recFixed {
		t.Fatalf("sweep: status=%s want fixed (detail=%s)", r.status.label(), r.detail)
	}
	if !f.filesGone(t) {
		t.Fatal("stash.json or the probe cache survived the sweep")
	}
	if f.has("container") || f.has("image") {
		t.Fatal("the cobalt container or image survived the sweep")
	}
	assertCobaltCallsScoped(t, f.calls(t))

	if r := reconcileRetiredCobalt(false); r.status != recOK {
		t.Fatalf("second run: status=%s want ok (detail=%s)", r.status.label(), r.detail)
	}
}

func TestReconcileRetiredCobaltReachableWithoutContainer(t *testing.T) {
	f := newCobaltFixture(t, true, "reachable", "image")
	r := reconcileRetiredCobalt(false)
	if r.status != recFixed {
		t.Fatalf("status=%s want fixed (detail=%s)", r.status.label(), r.detail)
	}
	if !f.filesGone(t) {
		t.Fatal("files survived the sweep")
	}
	if !f.has("image") {
		t.Fatal("an image without Ryoku's container is not Ryoku's to remove")
	}
	assertCobaltCallsScoped(t, f.calls(t))
}

func TestReconcileRetiredCobaltImageInUse(t *testing.T) {
	f := newCobaltFixture(t, true, "reachable", "container", "image", "image-busy")
	r := reconcileRetiredCobalt(false)
	if r.status != recWarn || !strings.Contains(r.remedy, "docker image rm ghcr.io/imputnet/cobalt:11") {
		t.Fatalf("status=%s remedy=%q, want a warning naming the image removal", r.status.label(), r.remedy)
	}
	if f.has("container") || !f.has("image") || !f.filesGone(t) {
		t.Fatal("container and files should go, the busy image should stay")
	}
}

func TestReconcileRetiredCobaltUnreachable(t *testing.T) {
	f := newCobaltFixture(t, true, "container", "image")

	r := reconcileRetiredCobalt(true)
	if r.status != recWouldFix || !f.filesPresent(t) {
		t.Fatalf("check: status=%s, want would-fix with files untouched", r.status.label())
	}

	r = reconcileRetiredCobalt(false)
	if r.status != recWarn {
		t.Fatalf("status=%s want warn (detail=%s)", r.status.label(), r.detail)
	}
	for _, want := range []string{"docker rm -f ryoku-cobalt", "docker image rm ghcr.io/imputnet/cobalt:11"} {
		if !strings.Contains(r.remedy, want) {
			t.Fatalf("remedy %q must name %q", r.remedy, want)
		}
	}
	if !f.filesGone(t) {
		t.Fatal("files survived the sweep")
	}
	if !f.has("container") || !f.has("image") {
		t.Fatal("an unreachable daemon was changed anyway")
	}
	for _, c := range f.calls(t) {
		if c != "info" {
			t.Fatalf("an unreachable daemon got more than info: %q", c)
		}
	}

	if r := reconcileRetiredCobalt(false); r.status != recOK {
		t.Fatalf("the warning must not repeat: status=%s detail=%s", r.status.label(), r.detail)
	}
}

func TestReconcileRetiredCobaltNoDocker(t *testing.T) {
	f := newCobaltFixture(t, false)
	if r := reconcileRetiredCobalt(true); r.status != recWouldFix {
		t.Fatalf("check: status=%s want would-fix", r.status.label())
	}
	r := reconcileRetiredCobalt(false)
	if r.status != recFixed {
		t.Fatalf("status=%s want fixed (detail=%s)", r.status.label(), r.detail)
	}
	if !f.filesGone(t) {
		t.Fatal("files survived the sweep")
	}
}
