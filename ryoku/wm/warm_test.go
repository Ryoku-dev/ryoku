package wm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The warm-path tests run this test binary as the provider: with fakeProviderEnv
// set it behaves as ryoku-wm-fakewm, logging every process start to
// FAKE_STARTS and every verb it runs to FAKE_RUNS.
const fakeProviderEnv = "RYOKU_WM_FAKE_PROVIDER"

func TestMain(m *testing.M) {
	if os.Getenv(fakeProviderEnv) != "" {
		os.Exit(fakeProvider(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func appendLine(path, line string) {
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	fmt.Fprintln(f, line)
	f.Close()
}

func fakeProvider(args []string) int {
	appendLine(os.Getenv("FAKE_STARTS"), strings.Join(args, " "))
	if len(args) == 0 {
		return 2
	}
	out := bufio.NewWriter(os.Stdout)
	if args[0] == "serve" {
		if os.Getenv("FAKE_NO_SERVE") != "" {
			fmt.Fprintf(os.Stderr, "ryoku-wm-fakewm: unknown verb %q\n", "serve")
			return 1
		}
		if err := Serve(os.Stdin, os.Stdout, out, func(argv []string) error { return fakeVerb(out, argv) }); err != nil {
			return 1
		}
		return 0
	}
	err := fakeVerb(out, args)
	out.Flush()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ryoku-wm-fakewm: %v\n", err)
		return 1
	}
	return 0
}

func fakeVerb(out io.Writer, argv []string) error {
	appendLine(os.Getenv("FAKE_RUNS"), strings.Join(argv, " "))
	switch argv[0] {
	case "caps":
		fmt.Fprint(out, `{"name":"fakewm","supports":["nightLight"]}`)
		return nil
	case "act":
	default:
		return fmt.Errorf("unknown verb %q", argv[0])
	}
	rest := argv[2:]
	switch argv[1] {
	case "echo":
		fmt.Fprint(out, strings.Join(rest, " "))
	case "pid":
		fmt.Fprint(out, os.Getpid())
	case "fail":
		return fmt.Errorf("act fail: %s", strings.Join(rest, " "))
	case "sleep":
		d, _ := time.ParseDuration(rest[0])
		time.Sleep(d)
		fmt.Fprint(out, "slept")
	case "die":
		os.Exit(3)
	}
	return nil
}

type fakeLogs struct{ starts, runs string }

func (l fakeLogs) lines(path string) []string {
	b, _ := os.ReadFile(path)
	return strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(b)), " ", "_"))
}

func (l fakeLogs) count(path, line string) int {
	n := 0
	for _, s := range l.lines(path) {
		if s == strings.ReplaceAll(line, " ", "_") {
			n++
		}
	}
	return n
}

// fakeProviderDir puts ryoku-wm-fakewm first on PATH, pointing at this test
// binary, and returns where its logs land.
func fakeProviderDir(t *testing.T) (string, fakeLogs) {
	t.Helper()
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, filepath.Join(dir, "ryoku-wm-fakewm")); err != nil {
		t.Fatal(err)
	}
	logs := fakeLogs{starts: filepath.Join(dir, "starts"), runs: filepath.Join(dir, "runs")}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(fakeProviderEnv, "1")
	t.Setenv("FAKE_STARTS", logs.starts)
	t.Setenv("FAKE_RUNS", logs.runs)
	return dir, logs
}

func warmClient(t *testing.T) *Client {
	t.Helper()
	c := OpenNamed("fakewm")
	c.EnableServe()
	t.Cleanup(c.Close)
	return c
}

func actPid(t *testing.T, c *Client) int {
	t.Helper()
	out, err := c.ActOutput("pid")
	if err != nil {
		t.Fatalf("pid: %v", err)
	}
	pid, err := strconv.Atoi(out)
	if err != nil {
		t.Fatalf("pid answered %q", out)
	}
	return pid
}

// The point of the warm path: a hundred actions, plus the caps probe, cost one
// provider start, each answer is its own, and caps is still asked only once.
func TestWarmProviderServesManyActsFromOneProcess(t *testing.T) {
	_, logs := fakeProviderDir(t)
	c := warmClient(t)

	for range 3 {
		caps, err := c.Caps()
		if err != nil || caps.Name != "fakewm" || !caps.Has(CapNightLight) {
			t.Fatalf("caps = %+v, %v", caps, err)
		}
	}
	for i := range 100 {
		out, err := c.ActOutput("echo", "n", strconv.Itoa(i))
		if err != nil {
			t.Fatalf("act %d: %v", i, err)
		}
		if want := "n " + strconv.Itoa(i); out != want {
			t.Fatalf("act %d answered %q, want %q", i, out, want)
		}
	}
	if got := logs.lines(logs.starts); len(got) != 1 || got[0] != "serve" {
		t.Fatalf("provider starts = %q, want exactly one serve", got)
	}
	if n := logs.count(logs.runs, "caps"); n != 1 {
		t.Errorf("caps ran %d times, want 1: a successful probe stays cached", n)
	}
	if a, b := actPid(t, c), actPid(t, c); a != b {
		t.Errorf("consecutive actions ran in pids %d and %d, want one warm process", a, b)
	}
}

// A served failure reads exactly like the one-shot failure it replaces.
func TestWarmProviderFailureMatchesOneShot(t *testing.T) {
	fakeProviderDir(t)
	_, warmErr := warmClient(t).ActOutput("fail", "no", "session")
	_, coldErr := OpenNamed("fakewm").ActOutput("fail", "no", "session")
	if warmErr == nil || coldErr == nil {
		t.Fatalf("errors = %v / %v, want both to fail", warmErr, coldErr)
	}
	if warmErr.Error() != coldErr.Error() {
		t.Errorf("warm error %q differs from one-shot %q", warmErr, coldErr)
	}
}

// A child that dies mid-request does not lose the call, and the next start
// waits out the backoff instead of forking a fresh child per call.
func TestWarmProviderRespawnsAfterDeath(t *testing.T) {
	_, logs := fakeProviderDir(t)
	c := warmClient(t)
	first := actPid(t, c)

	died := time.Now()
	if err := c.Act("die"); err == nil {
		t.Fatal("die should fail: the one-shot fallback dies too")
	}
	if n := logs.count(logs.starts, "act die"); n != 1 {
		t.Errorf("one-shot fallback ran %d times, want 1", n)
	}
	w := c.warm.Load()
	w.mu.Lock()
	child, retryAt := w.child, w.retryAt
	w.mu.Unlock()
	if child != nil || retryAt.Before(died.Add(warmMinBackoff)) {
		t.Errorf("dead child kept (%v) or respawn not held back (retry at +%v)", child != nil, retryAt.Sub(died))
	}
	if pid := actPid(t, c); pid == first {
		t.Error("the dead child answered")
	}
	time.Sleep(warmMinBackoff + 100*time.Millisecond)
	again := actPid(t, c)
	if again != actPid(t, c) {
		t.Error("after the backoff, actions should be warm again")
	}
	if n := logs.count(logs.starts, "serve"); n != 2 {
		t.Errorf("serve starts = %d, want 2 after one respawn", n)
	}
}

// A request whose ctx ends kills the warm child, and the fallback reports the
// cancellation without running the action a second time.
func TestWarmProviderTimeoutKillsChild(t *testing.T) {
	_, logs := fakeProviderDir(t)
	c := warmClient(t)
	first := actPid(t, c)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := c.ActContext(ctx, "sleep", "10s")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("timed-out act took %v", took)
	}
	if n := logs.count(logs.runs, "act sleep 10s"); n != 1 {
		t.Errorf("sleep ran %d times, want 1: a cancelled call must not be retried", n)
	}
	time.Sleep(warmMinBackoff + 100*time.Millisecond)
	if pid := actPid(t, c); pid == first {
		t.Error("the timed-out child is still answering; it should have been killed")
	}
}

// While the warm child is busy, a concurrent call does not queue behind it.
func TestWarmProviderBusyFallsBackToOneShot(t *testing.T) {
	fakeProviderDir(t)
	c := warmClient(t)
	warm := actPid(t, c)

	slowDone := make(chan struct{})
	go func() {
		defer close(slowDone)
		if out, err := c.ActOutput("sleep", "3s"); err != nil || out != "slept" {
			t.Errorf("slow act = %q, %v", out, err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	pid := actPid(t, c)
	select {
	case <-slowDone:
		t.Error("concurrent act waited behind the busy child")
	default:
	}
	if pid == warm {
		t.Error("concurrent act was answered by the busy warm child")
	}
	<-slowDone
	if actPid(t, c) != warm {
		t.Error("the warm child should answer again once free")
	}
}

// A provider older than the serve verb refuses it once; after that the same
// binary is asked one-shot only, even past the backoff.
func TestWarmProviderOldProviderStaysOneShot(t *testing.T) {
	_, logs := fakeProviderDir(t)
	t.Setenv("FAKE_NO_SERVE", "1")
	c := warmClient(t)
	for range 3 {
		if out, err := c.ActOutput("echo", "x"); err != nil || out != "x" {
			t.Fatalf("act = %q, %v", out, err)
		}
	}
	time.Sleep(warmMinBackoff + 100*time.Millisecond)
	if _, err := c.ActOutput("echo", "x"); err != nil {
		t.Fatal(err)
	}
	if n := logs.count(logs.starts, "serve"); n != 1 {
		t.Errorf("serve asked %d times, want once", n)
	}
	if n := logs.count(logs.starts, "act echo x"); n != 4 {
		t.Errorf("one-shot runs = %d, want 4", n)
	}
}

// A package update replaces the provider binary; the next call runs the new
// one instead of the process started from the old file.
func TestWarmProviderRestartsOnReplacedBinary(t *testing.T) {
	dir, logs := fakeProviderDir(t)
	c := warmClient(t)
	first := actPid(t, c)

	exe, _ := os.Executable()
	next := filepath.Join(dir, "next")
	if err := os.Symlink(exe, next); err != nil {
		t.Fatal(err)
	}
	// A copy, so the replacement is a different file, as pacman leaves it.
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(next)
	if err := os.WriteFile(next, b, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(next, filepath.Join(dir, "ryoku-wm-fakewm")); err != nil {
		t.Fatal(err)
	}
	second := actPid(t, c)
	if second == first {
		t.Error("the child started from the replaced binary still answers")
	}
	if second != actPid(t, c) {
		t.Error("the new binary should be served warm")
	}
	if n := logs.count(logs.starts, "serve"); n != 2 {
		t.Errorf("serve starts = %d, want 2", n)
	}
}

// A client that never opts in keeps today's one process per call.
func TestOneShotClientNeverStartsServe(t *testing.T) {
	_, logs := fakeProviderDir(t)
	c := OpenNamed("fakewm")
	for range 3 {
		if _, err := c.ActOutput("echo", "x"); err != nil {
			t.Fatal(err)
		}
	}
	if n := logs.count(logs.starts, "serve"); n != 0 {
		t.Errorf("serve started %d times without EnableServe", n)
	}
	if n := logs.count(logs.starts, "act echo x"); n != 3 {
		t.Errorf("one-shot runs = %d, want 3", n)
	}
}
