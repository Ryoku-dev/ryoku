package updater

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mustGit runs git in dir with an isolated identity and config, failing the test
// on error. dir "" runs without -C (for clone/init that take an explicit path).
func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	// -c gc.auto=0 / maintenance.auto=false: git commit can fork a background gc
	// that keeps .git busy, so t.TempDir's RemoveAll flakes with "directory not
	// empty" on a loaded CI runner. Disable it for every test git invocation.
	gopts := []string{"-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	if dir != "" {
		gopts = append(gopts, "-C", dir)
	}
	full := append(gopts, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitPush writes a file in work, commits it, and pushes the channel branch.
func commitPush(t *testing.T, work, name, body, msg, branch string) {
	t.Helper()
	writeFile(t, filepath.Join(work, name), body)
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", msg)
	mustGit(t, work, "push", "origin", branch)
}

func TestResolveChannelTarget(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	builder := filepath.Join(root, "builder")
	work := filepath.Join(root, "work")

	mustGit(t, "", "init", "--bare", "-b", "unstable-dev", origin)
	mustGit(t, "", "clone", origin, builder)
	commitPush(t, builder, "a", "checked\n", "checked change", "unstable-dev")
	a := strings.TrimSpace(mustGit(t, builder, "rev-parse", "HEAD"))

	// The box stays on A while the remote advances through the release bot's
	// version bump and then a human change.
	mustGit(t, "", "clone", origin, work)
	writeFile(t, filepath.Join(builder, "VERSION"), "1.2.3\n")
	mustGit(t, builder, "add", "-A")
	mustGit(t, builder, "commit", "--author=github-actions[bot] <actions@github.com>",
		"-m", "release: bump unstable to 1.2.3")
	mustGit(t, builder, "push", "origin", "unstable-dev")
	b := strings.TrimSpace(mustGit(t, builder, "rev-parse", "HEAD"))
	commitPush(t, builder, "c", "unchecked\n", "human change", "unstable-dev")
	c := strings.TrimSpace(mustGit(t, builder, "rev-parse", "HEAD"))
	mustGit(t, work, "fetch", "origin", "unstable-dev")

	passedA := checkedRef{Commit: a, Run: "https://example.test/passed", At: time.Now().Add(-time.Minute).Format(time.RFC3339)}
	cases := []struct {
		name         string
		tip          string
		pointer      checkedPointer
		pointerOK    bool
		force        bool
		want         string
		noteContains string
	}{
		{
			name:      "release bump inherits passed gate",
			tip:       b,
			pointer:   checkedPointer{Commit: a, State: "passed", Passed: passedA},
			pointerOK: true,
			want:      b,
		},
		{
			name: "checking stays on passed commit",
			tip:  c,
			pointer: checkedPointer{
				Commit: c, State: "checking", Run: "https://example.test/checking",
				At: time.Now().Add(-2 * time.Minute).Format(time.RFC3339), Passed: passedA,
			},
			pointerOK:    true,
			want:         a,
			noteContains: "still running",
		},
		{
			name: "failed stays on passed commit",
			tip:  c,
			pointer: checkedPointer{
				Commit: c, State: "failed", Run: "https://example.test/failed", Passed: passedA,
			},
			pointerOK:    true,
			want:         a,
			noteContains: "failed its checks",
		},
		{
			name:      "force takes unchecked tip",
			tip:       c,
			pointer:   checkedPointer{Commit: c, State: "failed", Passed: passedA},
			pointerOK: true,
			force:     true,
			want:      c,
		},
		{
			name:         "unreachable pointer stays on head",
			tip:          c,
			want:         a,
			noteContains: "cannot read the testing gate",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustGit(t, work, "update-ref", "refs/remotes/origin/unstable-dev", tc.tip)
			target, note := resolveChannelTarget(work, "unstable-dev", tc.pointer, tc.pointerOK, tc.force)
			got := strings.TrimSpace(mustGit(t, work, "rev-parse", target))
			if got != tc.want {
				t.Errorf("target %q resolves to %s, want %s", target, got, tc.want)
			}
			if tc.noteContains != "" && !strings.Contains(note, tc.noteContains) {
				t.Errorf("note = %q, want it to contain %q", note, tc.noteContains)
			}
			if tc.noteContains == "" && note != "" {
				t.Errorf("note = %q, want empty", note)
			}
		})
	}
}

func TestChannelStatus(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")

	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")

	t.Setenv("RYOKU_REPO", work)
	t.Setenv("RYOKU_CHANNEL", "main")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate TrackedChannel from the host's environment.d
	t.Setenv("XDG_STATE_HOME", t.TempDir())  // baseline falls back to HEAD, no recorded deploy

	// A fresh checkout in sync with its channel: nothing behind.
	r, ok := channelStatus()
	if !ok {
		t.Fatal("channelStatus not ok with a checkout present")
	}
	if r.Channel != "main" {
		t.Errorf("channel = %q, want main", r.Channel)
	}
	if r.Available || r.Behind != 0 || len(r.Updates) != 0 {
		t.Errorf("fresh checkout should be up to date: behind=%d available=%v updates=%d",
			r.Behind, r.Available, len(r.Updates))
	}

	// A second clone advances origin/main by two commits (a push to the channel).
	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "feature", "x\n", "add feature", "main")
	commitPush(t, other, "feature2", "y\n", "tweak feature", "main")

	// The checkout is now two commits behind; channelStatus fetches and reports it.
	r, ok = channelStatus()
	if !ok {
		t.Fatal("channelStatus not ok after the remote advanced")
	}
	if !r.Available || r.Behind != 2 {
		t.Errorf("behind = %d available = %v, want 2 / true", r.Behind, r.Available)
	}
	if len(r.Updates) != 2 {
		t.Fatalf("updates = %d, want 2", len(r.Updates))
	}
	// Newest first; subject in Name, short hash in New, no from/to pair.
	if got := r.Updates[0].Name; got != "tweak feature" {
		t.Errorf("updates[0].Name = %q, want %q", got, "tweak feature")
	}
	if r.Updates[0].New == "" || r.Updates[0].Old != "" {
		t.Errorf("commit row wants a short hash in New and empty Old, got %+v", r.Updates[0])
	}
	if r.Latest == r.Installed {
		t.Errorf("latest (%q) should differ from installed (%q) when behind", r.Latest, r.Installed)
	}
}

// The testing gate checks unstable-dev pushes only. A checkout on any other
// branch follows its tip, so status, and the Hub reading it, must not say that
// branch waits on checks it never gets.
func TestStatusReportsTheGateOnlyOnUnstableDev(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"commit":"abc1234","state":"checking","run":"https://example.test/run/1","at":"2026-11-09T00:00:00Z"}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("RYOKU_RELEASE_BASE", server.URL)

	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")
	for _, branch := range []string{"unstable-dev", "hyperion"} {
		mustGit(t, work, "push", "origin", "main:"+branch)
	}
	t.Setenv("RYOKU_REPO", work)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	for ch, wantGate := range map[string]bool{"unstable-dev": true, "main": false, "hyperion": false} {
		t.Setenv("RYOKU_CHANNEL", ch)
		if got := baseStatus().ChannelCheck != nil; got != wantGate {
			t.Errorf("%s: gate reported = %v, want %v", ch, got, wantGate)
		}
	}
}

func TestChannelStatusNoCheckout(t *testing.T) {
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // no recorded repo here
	// HOME too: ResolveRepo falls back to ~/ryoku-arch, so on a machine that has
	// one (any box that ran `ryoku track`) this test found a real checkout and
	// failed for the environment instead of the code.
	t.Setenv("HOME", t.TempDir())
	if _, ok := channelStatus(); ok {
		t.Error("channelStatus should report not-ok without a checkout")
	}
}

func TestChannelStatusUsesDeployedBaseline(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	state := t.TempDir()

	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")
	deployed := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))

	// Advance both HEAD and origin/main two commits past the deployed commit.
	commitPush(t, work, "f1", "x\n", "second commit", "main")
	commitPush(t, work, "f2", "y\n", "third commit", "main")

	t.Setenv("RYOKU_REPO", work)
	t.Setenv("RYOKU_CHANNEL", "main")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate TrackedChannel from the host's environment.d
	t.Setenv("XDG_STATE_HOME", state)
	writeFile(t, filepath.Join(state, "ryoku", "deployed"), deployed+"\n")

	// HEAD == origin/main now, so a HEAD baseline would report 0; the deployed
	// baseline (two commits back, what is actually running) must report 2.
	r, ok := channelStatus()
	if !ok {
		t.Fatal("channelStatus not ok")
	}
	if !r.Available || r.Behind != 2 {
		t.Errorf("deployed baseline: behind=%d available=%v, want 2/true", r.Behind, r.Available)
	}
}

func TestChannelStatusVersionIsChannelTip(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")

	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")
	tip := strings.TrimSpace(mustGit(t, work, "rev-parse", "--short", "main"))

	// A local commit ahead of main that is never pushed (a maintainer's WIP), the
	// shape that made the Hub show a commit the repo did not have.
	writeFile(t, filepath.Join(work, "wip"), "z\n")
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "local work ahead of main")

	t.Setenv("RYOKU_REPO", work)
	t.Setenv("RYOKU_CHANNEL", "main")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate TrackedChannel from the host's environment.d
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	r, ok := channelStatus()
	if !ok {
		t.Fatal("channelStatus not ok")
	}
	// Ahead of the channel: up to date, and the version is the channel tip (what
	// the repo shows on main), not the local commit ahead of it.
	if r.Available || r.Behind != 0 {
		t.Errorf("ahead of channel should be up to date: behind=%d available=%v", r.Behind, r.Available)
	}
	if r.Installed != tip || r.Latest != tip {
		t.Errorf("version should be the channel tip %s, got installed=%s latest=%s", tip, r.Installed, r.Latest)
	}
}

func TestChannelUpdateFastForwards(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")

	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	// A stub deploy script stands in for the real one channelUpdate runs.
	writeFile(t, filepath.Join(work, "ryoku", "shell", "deploy.sh"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(work, "ryoku", "shell", "deploy.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "seed with deploy stub")
	mustGit(t, work, "push", "origin", "main")

	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "feature", "x\n", "add feature", "main")

	t.Setenv("RYOKU_REPO", work)
	t.Setenv("RYOKU_CHANNEL", "main")
	t.Setenv("XDG_RUNTIME_DIR", root) // contain publishRun's state file
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	if err := channelUpdate(); err != nil {
		t.Fatalf("channelUpdate: %v", err)
	}
	if got := mustGit(t, work, "rev-parse", "HEAD"); got != mustGit(t, work, "rev-parse", "refs/remotes/origin/main") {
		t.Error("checkout did not fast-forward to origin/main")
	}
	if r, _ := channelStatus(); r.Behind != 0 {
		t.Errorf("after update, behind = %d, want 0", r.Behind)
	}
}

func TestChannelUpdateDeploysWithoutFastForwardOffChannel(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")

	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	writeFile(t, filepath.Join(work, "ryoku", "shell", "deploy.sh"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(work, "ryoku", "shell", "deploy.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "seed with deploy stub")
	mustGit(t, work, "push", "origin", "main")

	// A maintainer mid-dev: a feature branch with its own commit origin/main lacks.
	mustGit(t, work, "checkout", "-b", "feature-branch")
	writeFile(t, filepath.Join(work, "wip"), "z\n")
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "local wip")
	feature := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))

	// origin/main advances independently, so the branch and origin/main diverge.
	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "feature", "x\n", "add feature", "main")

	t.Setenv("RYOKU_REPO", work)
	t.Setenv("RYOKU_CHANNEL", "main")
	t.Setenv("XDG_RUNTIME_DIR", root)

	// The git path still redeploys, but a diverged feature branch keeps its
	// commits: branch management stays with the maintainer.
	if err := channelUpdate(); err != nil {
		t.Fatalf("channelUpdate off-channel: err=%v, want nil", err)
	}
	if got := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD")); got != feature {
		t.Errorf("off-channel update moved the maintainer's branch (HEAD %s, want %s)", got, feature)
	}
}

func TestChannelUpdateDeploysWithoutFastForwardWhenDirty(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")

	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	writeFile(t, filepath.Join(work, "ryoku", "shell", "deploy.sh"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(work, "ryoku", "shell", "deploy.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(work, "README"), "clean\n")
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "seed with deploy stub")
	mustGit(t, work, "push", "origin", "main")
	on := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))

	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "feature", "x\n", "add feature", "main")

	writeFile(t, filepath.Join(work, "README"), "dirty\n") // uncommitted edit to a tracked file

	t.Setenv("RYOKU_REPO", work)
	t.Setenv("RYOKU_CHANNEL", "main")
	t.Setenv("XDG_RUNTIME_DIR", root)

	// A dirty tree must not be fast-forwarded (it would clobber work in progress),
	// but the redeploy still runs.
	if err := channelUpdate(); err != nil {
		t.Fatalf("channelUpdate dirty: err=%v, want nil", err)
	}
	if got := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD")); got != on {
		t.Errorf("dirty update fast-forwarded the tree (HEAD %s, want %s)", got, on)
	}
}

func TestChannelUpdateReconcilesDivergence(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")

	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	writeFile(t, filepath.Join(work, "ryoku", "shell", "deploy.sh"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(work, "ryoku", "shell", "deploy.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "seed with deploy stub")
	mustGit(t, work, "push", "origin", "main")

	// origin/main advances...
	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "upstream", "u\n", "upstream fix", "main")

	// ...while local main diverges onto a commit origin/main lacks: no ff possible.
	writeFile(t, filepath.Join(work, "stray"), "s\n")
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "stray local commit")

	t.Setenv("RYOKU_REPO", work)
	t.Setenv("RYOKU_CHANNEL", "main")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate TrackedChannel from the host's environment.d
	t.Setenv("XDG_RUNTIME_DIR", root)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	if err := channelUpdate(); err != nil {
		t.Fatalf("channelUpdate on a diverged checkout: %v", err)
	}
	head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))
	want := strings.TrimSpace(mustGit(t, work, "rev-parse", "refs/remotes/origin/main"))
	if head != want {
		t.Errorf("diverged checkout was not reconciled to origin/main (HEAD %s, want %s)", head, want)
	}
	if r, _ := channelStatus(); r.Behind != 0 {
		t.Errorf("after reconcile, behind = %d, want 0", r.Behind)
	}
}

// gatedDeployFixture is a source box on unstable-dev whose checkout sits on the
// seed commit while origin has moved on to a commit the gate checked and, past
// it, one still being checked. Its deploy stub leaves a marker, so a test can
// tell whether the update laid the checkout down. gate builds the testing gate's
// answer once those commits exist.
func gatedDeployFixture(t *testing.T, gate func(checked, unchecked string) http.HandlerFunc) (work, state, marker, seed, checked, unchecked string) {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work = filepath.Join(root, "work")
	marker = filepath.Join(root, "deploy-ran")

	mustGit(t, "", "init", "--bare", "-b", "unstable-dev", origin)
	mustGit(t, "", "clone", origin, work)
	writeFile(t, filepath.Join(work, "ryoku", "shell", "deploy.sh"), "#!/bin/sh\ntouch '"+marker+"'\n")
	if err := os.Chmod(filepath.Join(work, "ryoku", "shell", "deploy.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "seed with deploy stub")
	mustGit(t, work, "push", "origin", "unstable-dev")
	seed = strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))

	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "checked", "c\n", "checked work", "unstable-dev")
	checked = strings.TrimSpace(mustGit(t, other, "rev-parse", "HEAD"))
	commitPush(t, other, "unchecked", "u\n", "work still being checked", "unstable-dev")
	unchecked = strings.TrimSpace(mustGit(t, other, "rev-parse", "HEAD"))

	server := httptest.NewServer(gate(checked, unchecked))
	t.Cleanup(server.Close)
	state = t.TempDir()
	t.Setenv("RYOKU_REPO", work)
	t.Setenv("RYOKU_CHANNEL", "unstable-dev")
	t.Setenv("RYOKU_RELEASE_BASE", server.URL)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate TrackedChannel from the host's environment.d
	t.Setenv("XDG_RUNTIME_DIR", root)
	t.Setenv("XDG_STATE_HOME", state)
	force, wait := updateForce, updateWait
	updateForce, updateWait = false, false
	t.Cleanup(func() { updateForce, updateWait = force, wait })
	return work, state, marker, seed, checked, unchecked
}

// Regression: offline, the update could not read the gate, stayed on a checkout
// a week behind the running desktop and deployed it anyway, removing the
// default bar. Without the gate there is nothing new to lay down.
func TestChannelUpdateKeepsTheDesktopWhenTheGateIsUnreadable(t *testing.T) {
	work, state, marker, seed, _, unchecked := gatedDeployFixture(t, func(_, _ string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unreachable", http.StatusServiceUnavailable)
		}
	})
	writeFile(t, filepath.Join(state, "ryoku", "deployed"), unchecked+"\n")

	if err := channelUpdate(); err != nil {
		t.Fatalf("channelUpdate: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the checkout was deployed although the gate could not be read")
	}
	if head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD")); head != seed {
		t.Errorf("checkout moved to %s without the gate, want it left on %s", head, seed)
	}
}

// The checkout advances to the last checked commit either way, but it is laid
// down only when that is not older than the commit the desktop already runs.
func TestChannelUpdateNeverDeploysOlderThanTheRunningDesktop(t *testing.T) {
	for _, tc := range []struct {
		name       string
		running    func(seed, unchecked string) string
		wantDeploy bool
	}{
		{"running a newer commit", func(seed, unchecked string) string { return unchecked }, false},
		{"running an older commit", func(seed, unchecked string) string { return seed }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work, state, marker, seed, checked, unchecked := gatedDeployFixture(t, func(checked, unchecked string) http.HandlerFunc {
				pointer := fmt.Sprintf(`{"commit":%q,"state":"checking","run":"r","at":%q,"passed":{"commit":%q}}`,
					unchecked, time.Now().UTC().Format(time.RFC3339), checked)
				return func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, pointer) }
			})
			writeFile(t, filepath.Join(state, "ryoku", "deployed"), tc.running(seed, unchecked)+"\n")

			if err := channelUpdate(); err != nil {
				t.Fatalf("channelUpdate: %v", err)
			}
			if head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD")); head != checked {
				t.Errorf("checkout on %s, want the last checked commit %s", head, checked)
			}
			_, err := os.Stat(marker)
			if deployed := err == nil; deployed != tc.wantDeploy {
				t.Errorf("deployed = %v, want %v", deployed, tc.wantDeploy)
			}
		})
	}
}

func TestRyokuChannelDefaultAndOverride(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // no recorded checkout
	t.Setenv("XDG_CONFIG_HOME", cfg)        // no persisted environment.d channel, so default applies
	t.Setenv("RYOKU_CHANNEL", "")
	packagedConf(t, "") // no [ryoku] stanza: not packaged, so the tracked/env/default path applies
	if got := ryokuChannel(); got != "main" {
		t.Errorf("default channel = %q, want main", got)
	}
	t.Setenv("RYOKU_CHANNEL", "unstable-dev")
	if got := ryokuChannel(); got != "unstable-dev" {
		t.Errorf("override channel = %q, want unstable-dev", got)
	}
	// `ryoku track` persisted the switch, but the session still carries the
	// env it captured at login: the persisted channel must win, or status and the
	// Hub report the old channel until a reboot.
	if err := os.MkdirAll(filepath.Join(cfg, "environment.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "environment.d", "ryoku.conf"), []byte("RYOKU_CHANNEL=main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ryokuChannel(); got != "main" {
		t.Errorf("tracked channel = %q, want main over the stale env", got)
	}
}

// shortCommit pulls the gNNNN commit token out of the repo-built package version
// (the form that lets the Hub and `ryoku status` show the exact commit), and
// leaves a version without that token (a hand-pinned 0.1.0-3, a bare hash) alone.
func TestShortCommit(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0.1.0.r241.g07bf14d-1", "07bf14d"},
		{"0.1.0.r241.g07bf14d", "07bf14d"},
		{"1.2.3.r5.gabcdef0", "abcdef0"},
		{"0.1.0-3", "0.1.0-3"},
		{"0.1.0", "0.1.0"},
		{"07bf14d", "07bf14d"},
		{"", ""},
	}
	for _, c := range cases {
		if got := shortCommit(c.in); got != c.want {
			t.Errorf("shortCommit(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSyncChannelFastForwardsNonMainBranch(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")

	// The dev box develops on unstable-dev, branched from the first commit; its
	// update channel is still main.
	mustGit(t, work, "checkout", "-b", "unstable-dev")

	// origin/main advances two commits ahead of the dev checkout.
	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "f1", "x\n", "release commit one", "main")
	commitPush(t, other, "f2", "y\n", "release commit two", "main")
	mustGit(t, work, "fetch", "origin", "main")

	if err := syncChannel(work, "main", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("syncChannel: %v", err)
	}

	head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))
	want := strings.TrimSpace(mustGit(t, work, "rev-parse", "refs/remotes/origin/main"))
	if head != want {
		t.Errorf("HEAD = %s, want origin/main %s: a clean non-main branch must fast-forward onto the channel", head, want)
	}
	if br := strings.TrimSpace(mustGit(t, work, "symbolic-ref", "--short", "HEAD")); br != "unstable-dev" {
		t.Errorf("branch = %s, want unstable-dev: the fast-forward must not switch branches", br)
	}
}

func TestSyncChannelPreservesDivergedBranch(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")

	// A maintainer mid-dev: a feature branch with its own unpushed commit.
	mustGit(t, work, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(work, "wip"), "z\n")
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "local wip")
	wip := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))

	// origin/main advances independently, so feature and origin/main diverge.
	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "f1", "x\n", "upstream commit", "main")
	mustGit(t, work, "fetch", "origin", "main")

	if err := syncChannel(work, "main", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("syncChannel: %v", err)
	}
	if head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD")); head != wip {
		t.Errorf("HEAD = %s, want the local wip %s: a diverged feature branch must not be moved", head, wip)
	}
}

func TestSyncChannelResetsDivergedChannelBranch(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")

	// A stray local commit on the channel branch that was never pushed.
	writeFile(t, filepath.Join(work, "local"), "l\n")
	mustGit(t, work, "add", "-A")
	mustGit(t, work, "commit", "-m", "stray local main commit")

	// origin/main advances differently, so local main and origin/main diverge.
	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "f1", "x\n", "upstream commit", "main")
	mustGit(t, work, "fetch", "origin", "main")

	if err := syncChannel(work, "main", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("syncChannel: %v", err)
	}
	// The channel branch mirrors upstream: it is reset onto origin/main, dropping
	// the stray commit.
	head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))
	want := strings.TrimSpace(mustGit(t, work, "rev-parse", "refs/remotes/origin/main"))
	if head != want {
		t.Errorf("HEAD = %s, want origin/main %s: a diverged channel branch resets onto upstream", head, want)
	}
}

func TestSyncChannelLeavesDirtyTree(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")
	before := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))

	// origin/main advances; the checkout is behind but has uncommitted changes.
	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "f1", "x\n", "upstream commit", "main")
	mustGit(t, work, "fetch", "origin", "main")
	writeFile(t, filepath.Join(work, "README"), "dirty edit\n")

	if err := syncChannel(work, "main", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("syncChannel: %v", err)
	}
	if head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD")); head != before {
		t.Errorf("HEAD = %s, want unchanged %s: a dirty tree must not be fast-forwarded", head, before)
	}
}

func TestSyncChannelNoOpWhenCurrent(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")
	mustGit(t, work, "fetch", "origin", "main")
	before := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))

	if err := syncChannel(work, "main", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("syncChannel: %v", err)
	}
	if head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD")); head != before {
		t.Errorf("HEAD = %s, want unchanged %s: a current checkout is a no-op", head, before)
	}
}

// TestUpdateClearsBehindOnDevBranch is the regression guard for the reported
// symptom: a dev box on unstable-dev, behind origin/main, kept reporting updates
// after every `ryoku update`. It runs the channel sync `ryoku update` does and
// records the deployed commit the way deploy.sh does, then asserts status reads
// up to date.
func TestUpdateClearsBehindOnDevBranch(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")
	mustGit(t, work, "checkout", "-b", "unstable-dev")

	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "f1", "x\n", "release commit one", "main")
	commitPush(t, other, "f2", "y\n", "release commit two", "main")

	t.Setenv("RYOKU_REPO", work)
	t.Setenv("RYOKU_CHANNEL", "main")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate TrackedChannel from the host's environment.d
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	// Before the update: the dev box is behind the channel.
	if r, ok := channelStatus(); !ok || !r.Available || r.Behind == 0 {
		t.Fatalf("precondition: expected behind, got ok=%v available=%v behind=%d", ok, r.Available, r.Behind)
	}

	// The channel sync `ryoku update` runs, then deploy.sh records HEAD as the
	// deployed commit.
	if err := syncChannel(work, "main", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("syncChannel: %v", err)
	}
	head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(state, "ryoku", "deployed"), head+"\n")

	// After the update: nothing left behind.
	r, ok := channelStatus()
	if !ok {
		t.Fatal("channelStatus not ok after the sync")
	}
	if r.Available || r.Behind != 0 || len(r.Updates) != 0 {
		t.Errorf("after update want up to date, got available=%v behind=%d updates=%d", r.Available, r.Behind, len(r.Updates))
	}
}

// Regression: a checkout with untracked files (build output, an app dir copied
// in from another branch) still fast-forwards. Treating those as "dirty" pinned
// real boxes to an old commit while `ryoku update` reported success.
func TestSyncChannelFastForwardsPastUntrackedFiles(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")

	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "f1", "x\n", "upstream commit", "main")
	mustGit(t, work, "fetch", "origin", "main")
	want := strings.TrimSpace(mustGit(t, work, "rev-parse", "refs/remotes/origin/main"))

	writeFile(t, filepath.Join(work, "scratch", "build.out"), "junk\n")

	if err := syncChannel(work, "main", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("syncChannel: %v", err)
	}
	if head := strings.TrimSpace(mustGit(t, work, "rev-parse", "HEAD")); head != want {
		t.Errorf("HEAD = %s, want %s: untracked files must not block the fast-forward", head, want)
	}
}

// An untracked file the incoming commits also add is a real conflict: it must
// fail loudly rather than skip the update in silence.
func TestSyncChannelErrorsOnUntrackedCollision(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "--bare", "-b", "main", origin)
	mustGit(t, "", "clone", origin, work)
	commitPush(t, work, "README", "one\n", "first commit", "main")

	other := filepath.Join(root, "other")
	mustGit(t, "", "clone", origin, other)
	commitPush(t, other, "app/new.qml", "upstream\n", "add app", "main")
	mustGit(t, work, "fetch", "origin", "main")

	writeFile(t, filepath.Join(work, "app", "new.qml"), "mine\n")

	if err := syncChannel(work, "main", "refs/remotes/origin/main"); err == nil {
		t.Fatal("syncChannel: err=nil, want an error when an untracked file collides")
	}
}

// A box just switched with `ryoku track` has RYOKU_CHANNEL in environment.d but
// the live env still carries the channel captured at login. ryokuChannel must
// read the persisted value over the env, else status and the Hub (which runs
// status under the session env) report the old channel until a relogin.
func TestChannelUsesPersistedChannel(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", cfg)
	packagedConf(t, "") // no packaged channel, so the persisted RYOKU_CHANNEL is authoritative
	writeFile(t, filepath.Join(cfg, "environment.d", "ryoku.conf"), "RYOKU_CHANNEL=unstable-dev\n")

	t.Setenv("RYOKU_CHANNEL", "")
	if got := ryokuChannel(); got != "unstable-dev" {
		t.Fatalf("persisted channel: got %q, want unstable-dev", got)
	}
	t.Setenv("RYOKU_CHANNEL", "main") // the stale session env from before the switch
	if got := ryokuChannel(); got != "unstable-dev" {
		t.Fatalf("stale env: got %q, want the persisted unstable-dev", got)
	}
}

// A migrated box carries a stale RYOKU_CHANNEL=unstable-dev in environment.d
// (and the login env) but a packaged testing Server. The packaged channel is
// authoritative, so status/version/doctor report testing, not unstable-dev.
func TestRyokuChannelPrefersPackagedOverStaleEnv(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // no checkout: this box is packaged
	t.Setenv("XDG_CONFIG_HOME", cfg)
	packagedConf(t, "testing")
	writeFile(t, filepath.Join(cfg, "environment.d", "ryoku.conf"), "RYOKU_CHANNEL=unstable-dev\n")
	t.Setenv("RYOKU_CHANNEL", "unstable-dev") // the stale login env
	if got := ryokuChannel(); got != "testing" {
		t.Fatalf("ryokuChannel = %q, want testing (the packaged Server wins over the stale env)", got)
	}
}
