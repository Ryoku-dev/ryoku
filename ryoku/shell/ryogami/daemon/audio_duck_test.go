package main

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

type fakePactl struct {
	mu         sync.Mutex
	lookups    int
	lists      int
	subscribes int
	started    chan context.Context
}

func installFakePactl(t *testing.T) *fakePactl {
	t.Helper()
	f := &fakePactl{started: make(chan context.Context, 8)}
	saved := duckPactl
	duckPactl = pactlRunner{
		lookPath: func() string {
			f.mu.Lock()
			f.lookups++
			f.mu.Unlock()
			return "/usr/bin/pactl"
		},
		list: func(context.Context, string) ([]byte, error) {
			f.mu.Lock()
			f.lists++
			f.mu.Unlock()
			return nil, nil
		},
		// A healthy subscribe stays open until the watcher cancels it.
		subscribe: func(ctx context.Context, _ string) (io.Reader, func() error, error) {
			f.mu.Lock()
			f.subscribes++
			f.mu.Unlock()
			r, w := io.Pipe()
			go func() {
				<-ctx.Done()
				w.Close()
			}()
			f.started <- ctx
			return r, func() error { return nil }, nil
		},
	}
	t.Cleanup(func() {
		stopDuckWatcher()
		duckPactl = saved
	})
	return f
}

func (f *fakePactl) counts() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lookups, f.lists, f.subscribes
}

func duckWatcherRunning() bool {
	duckWatcher.mu.Lock()
	defer duckWatcher.mu.Unlock()
	return duckWatcher.cancel != nil
}

func TestDuckOffNeverRunsPactl(t *testing.T) {
	d := newSettingsDaemon(t)
	f := installFakePactl(t)

	d.syncDuckWatcher()
	if err := d.setSetting(duckSettingKey, false); err != nil {
		t.Fatal(err)
	}
	d.evaluateDuck(context.Background())
	time.Sleep(50 * time.Millisecond)

	if l, li, s := f.counts(); l+li+s != 0 {
		t.Fatalf("setting off touched pactl: lookups=%d lists=%d subscribes=%d", l, li, s)
	}
	if duckWatcherRunning() {
		t.Fatal("watcher running with the setting off")
	}
}

func TestDuckWatcherFollowsSetting(t *testing.T) {
	d := newSettingsDaemon(t)
	f := installFakePactl(t)

	if err := d.setSetting(duckSettingKey, true); err != nil {
		t.Fatal(err)
	}
	var ctx context.Context
	select {
	case ctx = <-f.started:
	case <-time.After(3 * time.Second):
		t.Fatal("turning the setting on never started pactl subscribe")
	}
	// Setting it on again must not start a second subscribe.
	if err := d.setSetting(duckSettingKey, true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, _, s := f.counts(); s != 1 {
		t.Fatalf("subscribes = %d, want one long-lived subscribe", s)
	}

	if err := d.setSetting(duckSettingKey, false); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() == nil {
		t.Fatal("pactl subscribe still running after the setting turned off")
	}
	if duckWatcherRunning() {
		t.Fatal("watcher still registered after the setting turned off")
	}
	_, lists, subs := f.counts()
	time.Sleep(duckRestartMin + 200*time.Millisecond)
	if _, l, s := f.counts(); l != lists || s != subs {
		t.Fatalf("pactl ran after stop: lists %d->%d subscribes %d->%d", lists, l, subs, s)
	}
}

func TestDuckRestartBackoff(t *testing.T) {
	var b restartBackoff
	short := 100 * time.Millisecond
	want := []time.Duration{
		time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second,
		16 * time.Second, 32 * time.Second, time.Minute, time.Minute,
	}
	for i, w := range want {
		if got := b.after(short); got != w {
			t.Fatalf("restart %d waits %s, want %s", i, got, w)
		}
	}
	if got := b.after(duckHealthyRun); got != duckRestartMin {
		t.Fatalf("after a healthy minute the wait is %s, want %s", got, duckRestartMin)
	}
	if got := b.after(short); got != 2*time.Second {
		t.Fatalf("backoff after reset waits %s, want 2s", got)
	}
}
