package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// install puts a program at path the way an installer does: written beside
// it, then renamed over it.
func install(t *testing.T, path, body string) {
	t.Helper()
	fresh := path + ".new"
	if err := os.WriteFile(fresh, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(fresh, path); err != nil {
		t.Fatal(err)
	}
}

func watchedDaemon(t *testing.T, deps Deps) (*Daemon, string) {
	t.Helper()
	isolateDaemon(t)
	settle = 20 * time.Millisecond
	t.Cleanup(func() { settle = 2 * time.Second })
	path := filepath.Join(t.TempDir(), "rashind")
	install(t, path, "#!/bin/sh\n# first\n")
	d := New(deps)
	watch, err := watchProgram(path, d.retire)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(watch.close)
	d.mu.Lock()
	d.program = watch
	d.mu.Unlock()
	return d, path
}

func stopsWithin(d *Daemon, limit time.Duration) bool {
	select {
	case <-d.stop:
		return true
	case <-time.After(limit):
		return false
	}
}

// After an update the old daemon ran on for as long as the app stayed open,
// so what the update fixed stayed broken. A daemon whose program was replaced
// now exits by itself, but never under work: while an agent process is alive
// it waits, and goes the moment that ends.
func TestADaemonMakesWayForANewerProgramOnceNothingRuns(t *testing.T) {
	runner := &fakeRunner{}
	d, path := watchedDaemon(t, Deps{Harnesses: &fakeHarnesses{runner: runner}, Version: "test"})
	info, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.manager.start(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	install(t, path, "#!/bin/sh\n# second, and longer\n")
	if stopsWithin(d, 300*time.Millisecond) {
		t.Fatal("the daemon exited under a running agent")
	}
	if err := d.manager.stop(info.ID); err != nil {
		t.Fatal(err)
	}
	d.stateChanged()
	if !stopsWithin(d, 2*time.Second) {
		t.Fatal("the daemon ran on after its program was replaced and its work ended")
	}
}

// An idle daemon goes as soon as the new program has settled.
func TestAnIdleDaemonMakesWayAtOnce(t *testing.T) {
	d, path := watchedDaemon(t, Deps{Version: "test"})
	install(t, path, "#!/bin/sh\n# second, and longer\n")
	if !stopsWithin(d, 2*time.Second) {
		t.Fatal("an idle daemon ran on after its program was replaced")
	}
}

// Removing the program, or writing some other file beside it, is not an
// update, and a file that is not a program must not be what starts next.
func TestOnlyANewProgramUnderTheSameNameRetiresTheDaemon(t *testing.T) {
	d, path := watchedDaemon(t, Deps{Version: "test"})
	install(t, filepath.Join(filepath.Dir(path), "rashin"), "#!/bin/sh\n")
	if err := os.WriteFile(path+".part", []byte("half"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".part", path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if stopsWithin(d, 300*time.Millisecond) {
		t.Fatal("the daemon exited though no new program was installed")
	}
}
