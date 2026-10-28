package main

import (
	"context"
	"errors"
	"github.com/godbus/dbus/v5"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	wm "ryoku-wm"
)

// fakePowerProvider drops a `ryoku-wm-testwm` on PATH that reports the
// outputPower capability and appends every act invocation to a log file, so
// the wake guard's calls are observable without a compositor.
func fakePowerProvider(t *testing.T, logPath string) {
	t.Helper()
	bin := t.TempDir()
	provider := filepath.Join(bin, "ryoku-wm-testwm")
	script := "#!/usr/bin/env bash\n" +
		"if [[ \"$1\" == caps ]]; then printf '%s' '{\"name\":\"testwm\",\"supports\":[\"outputPower\"]}'; exit 0; fi\n" +
		"printf '%s\\n' \"$*\" >>\"" + logPath + "\"\n" +
		"exit 0\n"
	if err := os.WriteFile(provider, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func fakeFlakyPowerProvider(t *testing.T, logPath string) {
	t.Helper()
	bin := t.TempDir()
	countPath := filepath.Join(bin, "count")
	provider := filepath.Join(bin, "ryoku-wm-testwm")
	script := "#!/usr/bin/env bash\n" +
		"if [[ \"$1\" == caps ]]; then printf '%s' '{\"name\":\"testwm\",\"supports\":[\"outputPower\"]}'; exit 0; fi\n" +
		"n=0; [[ -r \"" + countPath + "\" ]] && n=$(<\"" + countPath + "\"); n=$((n+1)); printf '%s' \"$n\" >\"" + countPath + "\"\n" +
		"printf '%s\\n' \"$*\" >>\"" + logPath + "\"\n" +
		"(( n > 1 ))\n"
	if err := os.WriteFile(provider, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func fakeInitiallyHungPowerProvider(t *testing.T, countPath string) {
	t.Helper()
	bin := t.TempDir()
	provider := filepath.Join(bin, "ryoku-wm-testwm")
	script := "#!/usr/bin/env bash\n" +
		"if [[ \"$1\" == caps ]]; then printf '%s' '{\"name\":\"testwm\",\"supports\":[\"outputPower\"]}'; exit 0; fi\n" +
		"n=0; [[ -r \"" + countPath + "\" ]] && n=$(<\"" + countPath + "\"); n=$((n+1)); tmp=\"" + countPath + ".tmp.$$\"; printf '%s' \"$n\" >\"$tmp\"; mv -f \"$tmp\" \"" + countPath + "\"\n" +
		"if (( n == 1 )); then exec sleep 10; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(provider, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func isolateOutputState(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
}

func TestGraphicalLogin1UserSessionExcludesNonUserSurfaces(t *testing.T) {
	for _, tc := range []struct {
		sessionType  string
		sessionClass string
		want         bool
	}{
		{"wayland", "user", true},
		{"x11", "user-early", false},
		{"tty", "user", false},
		{"wayland", "greeter", false},
		{"wayland", "lock-screen", false},
		{"wayland", "background", false},
	} {
		if got := graphicalLogin1UserSession(tc.sessionType, tc.sessionClass); got != tc.want {
			t.Errorf("graphicalLogin1UserSession(%q, %q) = %v, want %v",
				tc.sessionType, tc.sessionClass, got, tc.want)
		}
	}
}

func TestHoldAwakeReAssertsOutputPower(t *testing.T) {
	isolateOutputState(t)
	logPath := filepath.Join(t.TempDir(), "acts.log")
	fakePowerProvider(t, logPath)

	oldHold, oldStep := wakeHold, wakeStep
	wakeHold, wakeStep = 250*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { wakeHold, wakeStep = oldHold, oldStep })

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	d.holdAwake(context.Background())

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("no act log (guard never fired): %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 {
		t.Fatalf("guard asserted %d time(s), want a re-assert loop over the window", len(lines))
	}
	for _, l := range lines {
		if l != "act output.power on" {
			t.Fatalf("act line = %q, want %q", l, "act output.power on")
		}
	}
}

func TestHoldAwakeActsBeforeFirstStep(t *testing.T) {
	isolateOutputState(t)
	logPath := filepath.Join(t.TempDir(), "acts.log")
	fakePowerProvider(t, logPath)

	oldHold, oldStep := wakeHold, wakeStep
	wakeHold, wakeStep = 5*time.Second, 5*time.Second
	t.Cleanup(func() { wakeHold, wakeStep = oldHold, oldStep })

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	done := make(chan struct{})
	go func() { d.holdAwake(context.Background()); close(done) }()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(logPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, err := os.Stat(logPath)
	close(d.quit)
	<-done
	if err != nil {
		t.Fatal("wake guard waited for the retry interval instead of acting immediately")
	}
}

func TestHoldAwakeContinuesAfterTransientProviderError(t *testing.T) {
	isolateOutputState(t)
	logPath := filepath.Join(t.TempDir(), "acts.log")
	fakeFlakyPowerProvider(t, logPath)

	oldHold, oldStep := wakeHold, wakeStep
	wakeHold, wakeStep = 120*time.Millisecond, 25*time.Millisecond
	t.Cleanup(func() { wakeHold, wakeStep = oldHold, oldStep })

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	d.holdAwake(context.Background())

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(strings.TrimSpace(string(b)), "\n") + 1; got < 2 {
		t.Fatalf("wake guard stopped after the first provider error: %d call(s)", got)
	}
}

func TestHoldAwakeTimesOutHungProviderAndRetries(t *testing.T) {
	isolateOutputState(t)
	countPath := filepath.Join(t.TempDir(), "count")
	fakeInitiallyHungPowerProvider(t, countPath)

	oldHold, oldStep, oldActionWait := wakeHold, wakeStep, wakeActionWait
	wakeHold, wakeStep, wakeActionWait = 180*time.Millisecond, 20*time.Millisecond, 30*time.Millisecond
	t.Cleanup(func() {
		wakeHold, wakeStep, wakeActionWait = oldHold, oldStep, oldActionWait
	})

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	start := time.Now()
	d.holdAwake(context.Background())
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("hung provider blocked bounded wake retries for %v", elapsed)
	}
	body, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("provider attempts = %d, want a retry after timeout", n)
	}
}

func TestHoldAwakeStopsWithoutCapability(t *testing.T) {
	isolateOutputState(t)
	logPath := filepath.Join(t.TempDir(), "acts.log")
	bin := t.TempDir()
	provider := filepath.Join(bin, "ryoku-wm-testwm")
	script := "#!/usr/bin/env bash\n" +
		"if [[ \"$1\" == caps ]]; then printf '%s' '{\"name\":\"testwm\",\"supports\":[]}'; exit 0; fi\n" +
		"printf '%s\\n' \"$*\" >>\"" + logPath + "\"\n" +
		"exit 0\n"
	if err := os.WriteFile(provider, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	oldHold, oldStep := wakeHold, wakeStep
	wakeHold, wakeStep = 150*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { wakeHold, wakeStep = oldHold, oldStep })

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	d.holdAwake(context.Background())

	if _, err := os.Stat(logPath); err == nil {
		t.Fatal("guard fired act calls on a provider without outputPower")
	}
}

func TestHoldAwakeHonoursQuit(t *testing.T) {
	isolateOutputState(t)
	logPath := filepath.Join(t.TempDir(), "acts.log")
	fakePowerProvider(t, logPath)

	oldHold, oldStep := wakeHold, wakeStep
	wakeHold, wakeStep = time.Hour, 20*time.Millisecond
	t.Cleanup(func() { wakeHold, wakeStep = oldHold, oldStep })

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	go func() { time.Sleep(120 * time.Millisecond); close(d.quit) }()
	done := make(chan struct{})
	go func() { d.holdAwake(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("holdAwake ignored the quit signal")
	}
}

func TestHoldAwakeStopsBeforeOverridingNewerOutputOff(t *testing.T) {
	isolateOutputState(t)
	logPath := filepath.Join(t.TempDir(), "acts.log")
	fakePowerProvider(t, logPath)

	oldHold, oldStep := wakeHold, wakeStep
	wakeHold, wakeStep = time.Second, 40*time.Millisecond
	t.Cleanup(func() { wakeHold, wakeStep = oldHold, oldStep })

	d := &daemon{wmc: wm.OpenNamed("testwm"), quit: make(chan struct{})}
	done := make(chan struct{})
	go func() { d.holdAwake(context.Background()); close(done) }()
	deadline := time.Now().Add(time.Second)
	for {
		if body, err := os.ReadFile(logPath); err == nil && len(body) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("wake retrain never issued its initial output-on")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := withOutputStateLock(func(path string) error {
		return os.WriteFile(path, []byte("newer-off\n"), 0o600)
	}); err != nil {
		t.Fatal(err)
	}
	bodyAtOff, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wake retrain ignored the newer output-off generation")
	}
	bodyAfter, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(bodyAfter) != string(bodyAtOff) {
		t.Fatal("wake retrain issued output-on after a newer output-off edge")
	}
}

func TestSuspendOffsetDetectsMissedWakeAndResets(t *testing.T) {
	offset := 3 * time.Second
	cycle := &sleepCycle{suspendOffset: func() time.Duration { return offset }}
	cycle.markWakeOffset()
	if cycle.suspendSinceLastWake() {
		t.Fatal("unchanged suspend offset looked like a missed wake")
	}
	offset += time.Second
	if !cycle.suspendSinceLastWake() {
		t.Fatal("suspend elapsed while signals were absent was not detected")
	}
	cycle.markWakeOffset()
	if cycle.suspendSinceLastWake() {
		t.Fatal("replayed wake did not reset the suspend baseline")
	}
}

type fakeSleepInhibitor struct {
	mu        sync.Mutex
	events    chan string
	name      string
	max       time.Duration
	heldState bool
	failFor   int
	attempts  int
	wait      <-chan struct{}
}

func (f *fakeSleepInhibitor) acquire(ctx context.Context) error {
	f.mu.Lock()
	if f.heldState {
		f.mu.Unlock()
		return nil
	}
	f.attempts++
	attempt := f.attempts
	wait := f.wait
	f.mu.Unlock()

	if f.events != nil {
		f.events <- f.name + ".acquire"
	}
	if wait != nil {
		select {
		case <-wait:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if attempt <= f.failFor {
		return errors.New("sleep operation still finishing")
	}
	f.mu.Lock()
	f.heldState = true
	f.mu.Unlock()
	return nil
}

func (f *fakeSleepInhibitor) release() {
	f.mu.Lock()
	if !f.heldState {
		f.mu.Unlock()
		return
	}
	f.heldState = false
	f.mu.Unlock()
	if f.events != nil {
		f.events <- f.name + ".release"
	}
}

func (f *fakeSleepInhibitor) held() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.heldState
}

func (f *fakeSleepInhibitor) limit() time.Duration { return f.max }

func (f *fakeSleepInhibitor) attemptCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts
}

func TestLockBudgetReservesHeadroomAndCapsWork(t *testing.T) {
	old := lockWait
	lockWait = 3 * time.Second
	defer func() { lockWait = old }()
	for _, tc := range []struct {
		limit time.Duration
		want  time.Duration
	}{
		{0, 3 * time.Second},
		{2 * time.Second, time.Second},
		{5 * time.Second, 4 * time.Second},
		{15 * time.Second, 12 * time.Second},
		{60 * time.Second, 12 * time.Second},
	} {
		if got := lockBudget(tc.limit); got != tc.want {
			t.Errorf("lockBudget(%v) = %v, want %v", tc.limit, got, tc.want)
		}
	}
}

func TestSleepEdgeReleasesDelayOnlyAfterSecureLock(t *testing.T) {
	events := make(chan string, 3)
	delay := &fakeSleepInhibitor{
		events: events, name: "delay", max: 15 * time.Second, heldState: true,
	}
	now := time.Unix(100, 0)
	var deadline time.Time
	cycle := &sleepCycle{
		delay: delay,
		block: &fakeSleepInhibitor{heldState: true},
		lock: func(got time.Time) error {
			deadline = got
			events <- "lock"
			return nil
		},
		wake:     func(context.Context) {},
		lighting: func(context.Context) {},
		now:      func() time.Time { return now },
		logf:     func(string, ...any) {},
	}

	if !cycle.handle(true) {
		t.Fatal("secure sleep edge was rejected")
	}
	if first, second := <-events, <-events; first != "lock" || second != "delay.release" {
		t.Fatalf("sleep ordering = %q then %q, want lock then delay.release", first, second)
	}
	if want := now.Add(12 * time.Second); !deadline.Equal(want) {
		t.Fatalf("secure-lock deadline = %v, want %v", deadline, want)
	}
	if delay.held() {
		t.Fatal("delay inhibitor stayed held after the compositor confirmed security")
	}
}

func TestSleepEdgeRetainsDelayWhenLockIsNotSecure(t *testing.T) {
	delay := &fakeSleepInhibitor{max: 15 * time.Second, heldState: true}
	var logged bool
	cycle := &sleepCycle{
		delay:    delay,
		block:    &fakeSleepInhibitor{heldState: true},
		lock:     func(time.Time) error { return errors.New("not secure") },
		wake:     func(context.Context) {},
		lighting: func(context.Context) {},
		now:      time.Now,
		logf:     func(string, ...any) { logged = true },
	}

	if cycle.handle(true) {
		t.Fatal("insecure sleep edge was accepted")
	}
	if !delay.held() {
		t.Fatal("delay inhibitor was released without a compositor-secure lock")
	}
	if !logged {
		t.Fatal("secure-lock failure was not reported")
	}
}

func TestSuspendTransactionRejectsLockFailure(t *testing.T) {
	events := make(chan string, 4)
	block := &fakeSleepInhibitor{events: events, name: "block", heldState: true}
	delay := &fakeSleepInhibitor{heldState: true, max: 15 * time.Second}
	cycle := &sleepCycle{
		delay: delay,
		block: block,
		lock: func(time.Time) error {
			events <- "lock"
			return errors.New("qylock crashed")
		},
		suspend: func(context.Context) error {
			events <- "suspend"
			return nil
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}

	if err := cycle.requestSuspend(); err == nil {
		t.Fatal("suspend proceeded without a compositor-secure lock")
	}
	if first := <-events; first != "lock" {
		t.Fatalf("first event = %q, want lock", first)
	}
	select {
	case event := <-events:
		t.Fatalf("unexpected event after lock failure: %q", event)
	default:
	}
	if !block.held() || !delay.held() {
		t.Fatal("suspend protection changed after lock failure")
	}
}

func TestCancelledLidTransactionNeverCallsLogin1(t *testing.T) {
	lockStarted := make(chan struct{})
	releaseLock := make(chan struct{})
	suspendCalls := 0
	cycle := &sleepCycle{
		delay: &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		block: &fakeSleepInhibitor{heldState: true},
		lock: func(time.Time) error {
			close(lockStarted)
			<-releaseLock
			return nil
		},
		suspend: func(context.Context) error {
			suspendCalls++
			return nil
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- cycle.requestSuspendContext(ctx) }()
	<-lockStarted
	cancel()
	close(releaseLock)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled transaction returned %v", err)
	}
	if suspendCalls != 0 {
		t.Fatalf("cancelled lid transaction called login1 %d time(s)", suspendCalls)
	}
	if !cycle.block.held() {
		t.Fatal("cancelled transaction released the hard sleep block")
	}
}

func TestSuspendCancellationTombstoneClosesRegistrationRace(t *testing.T) {
	d := &daemon{}
	const token = "lid-session-123"
	if err := d.cancelSuspendRequest(token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.startSuspendRequest(token); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled transaction started: %v", err)
	}

	ctx, request, err := d.startSuspendRequest(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.cancelSuspendRequest(token); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("active suspend transaction did not receive cancellation")
	}
	d.finishSuspendRequest(token, request)
}

func TestSuspendTransactionDropsBlockAfterSecureLock(t *testing.T) {
	events := make(chan string, 4)
	block := &fakeSleepInhibitor{events: events, name: "block", heldState: true}
	cycle := &sleepCycle{
		delay: &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		block: block,
		lock: func(time.Time) error {
			events <- "lock"
			return nil
		},
		suspend: func(context.Context) error {
			events <- "suspend"
			return nil
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}

	if err := cycle.requestSuspend(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"lock", "block.release", "suspend"} {
		if got := <-events; got != want {
			t.Fatalf("suspend event = %q, want %q", got, want)
		}
	}
	if block.held() {
		t.Fatal("block inhibitor remained held during the coordinated suspend")
	}
}

// TestSuspendWaitsForLogin1ToSeeTheRelease pins the #324 race: the daemon's
// Suspend call must not overtake logind observing the closed block fd, so the
// transaction polls the inhibitor list and only calls Suspend once the list
// stops naming this process.
func TestSuspendWaitsForLogin1ToSeeTheRelease(t *testing.T) {
	oldPoll := blockReleasePoll
	blockReleasePoll = 2 * time.Millisecond
	defer func() { blockReleasePoll = oldPoll }()
	events := make(chan string, 8)
	block := &fakeSleepInhibitor{events: events, name: "block", heldState: true}
	seen := 0
	cycle := &sleepCycle{
		delay: &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		block: block,
		lock:  func(time.Time) error { return nil },
		blockCleared: func(context.Context) (bool, error) {
			seen++
			if seen < 3 {
				events <- "check"
				return false, nil
			}
			events <- "check"
			return true, nil
		},
		suspend: func(context.Context) error {
			events <- "suspend"
			return nil
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}

	if err := cycle.requestSuspend(); err != nil {
		t.Fatal(err)
	}
	want := []string{"block.release", "check", "check", "check", "suspend"}
	for _, w := range want {
		if got := <-events; got != w {
			t.Fatalf("event = %q, want %q", got, w)
		}
	}
}

// TestSuspendAbortsWhenReleaseIsNeverSeen pins the denial bound: if login1
// keeps reporting the released block, the transaction gives up inside its
// window, never calls Suspend (so it cannot be denied again), and restores
// the guard for the next attempt.
func TestSuspendAbortsWhenReleaseIsNeverSeen(t *testing.T) {
	block := &fakeSleepInhibitor{name: "block", heldState: true}
	oldWait, oldPoll := blockReleaseWait, blockReleasePoll
	blockReleaseWait, blockReleasePoll = 60*time.Millisecond, 5*time.Millisecond
	defer func() { blockReleaseWait, blockReleasePoll = oldWait, oldPoll }()
	suspendCalls := 0
	cycle := &sleepCycle{
		delay: &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		block: block,
		lock:  func(time.Time) error { return nil },
		blockCleared: func(context.Context) (bool, error) {
			return false, nil
		},
		suspend: func(context.Context) error {
			suspendCalls++
			return nil
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}

	if err := cycle.requestSuspend(); err == nil {
		t.Fatal("a never-confirmed release was reported as success")
	}
	if suspendCalls != 0 {
		t.Fatalf("login1 was called %d times after an unconfirmed release", suspendCalls)
	}
	if !block.held() {
		t.Fatal("the aborted transaction left the sleep block down")
	}
}

func TestSuspendTransactionRestoresBlockWhenLogin1Rejects(t *testing.T) {
	events := make(chan string, 5)
	block := &fakeSleepInhibitor{events: events, name: "block", heldState: true}
	cycle := &sleepCycle{
		delay: &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		block: block,
		lock: func(time.Time) error {
			events <- "lock"
			return nil
		},
		suspend: func(context.Context) error {
			events <- "suspend"
			return errors.New("operation inhibited")
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}

	if err := cycle.requestSuspend(); err == nil {
		t.Fatal("login1 suspend failure was reported as success")
	}
	for _, want := range []string{"lock", "block.release", "suspend", "block.acquire"} {
		if got := <-events; got != want {
			t.Fatalf("failed-suspend event = %q, want %q", got, want)
		}
	}
	if !block.held() {
		t.Fatal("block inhibitor was not restored after login1 rejected suspend")
	}
}

func TestRejectedSuspendRetriesFailedBlockRestore(t *testing.T) {
	block := &fakeSleepInhibitor{name: "block", heldState: true, failFor: 1}
	cycle := &sleepCycle{
		delay:        &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		block:        block,
		lock:         func(time.Time) error { return nil },
		suspend:      func(context.Context) error { return errors.New("operation inhibited") },
		now:          time.Now,
		logf:         func(string, ...any) {},
		retryProtect: make(chan struct{}, 1),
	}
	sigs := make(chan *dbus.Signal)
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runSleepSignals(sigs, quit, cycle, true, 5*time.Millisecond)
		close(done)
	}()

	if err := cycle.requestSuspend(); err == nil {
		t.Fatal("login1 suspend failure was reported as success")
	}
	deadline := time.After(500 * time.Millisecond)
	for !cycle.protected() {
		select {
		case <-deadline:
			t.Fatalf("sleep block remained down after rejected suspend; attempts=%d", block.attemptCount())
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if block.attemptCount() < 2 {
		t.Fatalf("block acquire attempts = %d, want immediate attempt plus retry", block.attemptCount())
	}
	close(quit)
	<-done
}

func TestSuspendRequestRejectedBetweenFailedProtectionAndRetry(t *testing.T) {
	block := &fakeSleepInhibitor{name: "block"}
	delay := &fakeSleepInhibitor{name: "delay", failFor: 1, max: 15 * time.Second}
	lockCalls, suspendCalls := 0, 0
	cycle := &sleepCycle{
		delay: delay,
		block: block,
		lock: func(time.Time) error {
			lockCalls++
			return nil
		},
		suspend: func(context.Context) error {
			suspendCalls++
			return nil
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}

	if cycle.protect() {
		t.Fatal("protection unexpectedly succeeded during the injected delay race")
	}
	if !block.held() || delay.held() {
		t.Fatal("block inhibitor did not cover the failed delay-acquire window")
	}
	if err := cycle.requestSuspend(); err == nil {
		t.Fatal("suspend was accepted before full protection was restored")
	}
	if lockCalls != 0 || suspendCalls != 0 {
		t.Fatalf("unprotected request ran lock=%d suspend=%d times", lockCalls, suspendCalls)
	}
}

func TestWakeRecoveryStartsBeforeBoundedInhibitorAcquireReturns(t *testing.T) {
	wait := make(chan struct{})
	events := make(chan string, 5)
	block := &fakeSleepInhibitor{events: events, name: "block", wait: wait}
	delay := &fakeSleepInhibitor{events: events, name: "delay", max: 15 * time.Second}
	cycle := &sleepCycle{
		delay:    delay,
		block:    block,
		lock:     func(time.Time) error { return nil },
		wake:     func(context.Context) { events <- "wake" },
		lighting: func(context.Context) { events <- "lighting" },
		now:      time.Now,
		logf:     func(string, ...any) {},
	}
	oldWait := login1CallWait
	login1CallWait = 25 * time.Millisecond
	t.Cleanup(func() { login1CallWait = oldWait })

	start := time.Now()
	if cycle.handle(false) {
		t.Fatal("protection unexpectedly succeeded while login1 was wedged")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("bounded login1 acquire took %v", elapsed)
	}
	got := map[string]bool{}
	deadline := time.After(time.Second)
	for !got["wake"] || !got["lighting"] {
		select {
		case event := <-events:
			got[event] = true
		case <-deadline:
			t.Fatalf("wake jobs did not start before bounded acquire returned: %#v", got)
		}
	}
}

func TestNewSleepEdgeCancelsWakeGeneration(t *testing.T) {
	wakeStarted := make(chan struct{})
	wakeCancelled := make(chan struct{})
	lightingStarted := make(chan struct{})
	lightingCancelled := make(chan struct{})
	cycle := &sleepCycle{
		delay: &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		block: &fakeSleepInhibitor{heldState: true},
		lock:  func(time.Time) error { return nil },
		wake: func(ctx context.Context) {
			close(wakeStarted)
			<-ctx.Done()
			close(wakeCancelled)
		},
		lighting: func(ctx context.Context) {
			close(lightingStarted)
			<-ctx.Done()
			close(lightingCancelled)
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}
	if !cycle.handle(false) {
		t.Fatal("resume failed to restore protection")
	}
	<-wakeStarted
	<-lightingStarted
	if !cycle.handle(true) {
		t.Fatal("new sleep edge was rejected")
	}
	for name, cancelled := range map[string]<-chan struct{}{
		"output recovery": wakeCancelled,
		"lighting":        lightingCancelled,
	} {
		select {
		case <-cancelled:
		case <-time.After(time.Second):
			t.Fatalf("%s continued into the next sleep generation", name)
		}
	}
}

func TestSleepSignalsRetryProtectionAfterResumeRace(t *testing.T) {
	events := make(chan string, 12)
	block := &fakeSleepInhibitor{events: events, name: "block"}
	delay := &fakeSleepInhibitor{
		events: events, name: "delay", max: 15 * time.Second, failFor: 1,
	}
	cycle := &sleepCycle{
		delay: delay,
		block: block,
		lock:  func(time.Time) error { return nil },
		wake:  func(context.Context) { events <- "wake" },
		lighting: func(context.Context) {
			events <- "lighting"
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}
	sigs := make(chan *dbus.Signal, 1)
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runSleepSignals(sigs, quit, cycle, true, 5*time.Millisecond)
		close(done)
	}()

	sigs <- &dbus.Signal{Name: login1Interface + ".PrepareForSleep", Body: []any{false}}
	got := map[string]int{}
	deadline := time.After(500 * time.Millisecond)
	for delay.attemptCount() < 2 || !cycle.protected() || got["wake"] < 1 || got["lighting"] < 1 {
		select {
		case event := <-events:
			got[event]++
		case <-deadline:
			t.Fatalf("resume recovery events = %#v, delay attempts=%d protected=%v", got, delay.attemptCount(), cycle.protected())
		}
	}
	close(quit)
	<-done
}

func TestInactiveSessionSecuresLockBeforeReleasingGlobalBlock(t *testing.T) {
	block := &fakeSleepInhibitor{heldState: true}
	delay := &fakeSleepInhibitor{heldState: true, max: 15 * time.Second}
	lockCalls, suspendCalls := 0, 0
	cycle := &sleepCycle{
		delay: delay,
		block: block,
		lock: func(time.Time) error {
			lockCalls++
			return nil
		},
		suspend: func(context.Context) error {
			suspendCalls++
			return nil
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}

	cycle.setSessionActive(false)
	if cycle.ready() {
		t.Fatal("inactive session was ready before qylock became compositor-secure")
	}
	if !cycle.protect() {
		t.Fatal("inactive session protection failed after qylock became secure")
	}
	if block.held() {
		t.Fatal("inactive session retained the global sleep block after securing qylock")
	}
	if !delay.held() {
		t.Fatal("inactive session dropped the delay needed for global sleep")
	}
	if err := cycle.requestSuspend(); err == nil {
		t.Fatal("inactive session was allowed to suspend the machine")
	}
	if lockCalls != 1 || suspendCalls != 0 {
		t.Fatalf("inactive flow ran lock=%d suspend=%d times, want 1 and 0", lockCalls, suspendCalls)
	}
}

func TestInactiveSessionRetainsGlobalBlockWhenLockFails(t *testing.T) {
	block := &fakeSleepInhibitor{heldState: true}
	cycle := &sleepCycle{
		delay: &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		block: block,
		lock: func(time.Time) error {
			return errors.New("qylock crashed")
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}

	cycle.setSessionActive(false)
	if cycle.protect() {
		t.Fatal("inactive session accepted a failed secure lock")
	}
	if !block.held() {
		t.Fatal("inactive session released the hard block after lock failure")
	}
	if cycle.ready() {
		t.Fatal("inactive session reported ready after lock failure")
	}
}

func TestSleepReadyRequiresTheCurrentSessionProtection(t *testing.T) {
	block := &fakeSleepInhibitor{heldState: true}
	delay := &fakeSleepInhibitor{heldState: true}
	cycle := &sleepCycle{
		block: block,
		delay: delay,
		lock: func(time.Time) error {
			return nil
		},
		now:  time.Now,
		logf: func(string, ...any) {},
	}
	d := &daemon{sleep: cycle}

	if got := d.dispatch("sleep-ready"); got != "ok" {
		t.Fatalf("active ready guard = %q, want ok", got)
	}
	block.release()
	if got := d.dispatch("sleep-ready"); !strings.HasPrefix(got, "err sleep-ready") {
		t.Fatalf("active guard without block = %q, want readiness error", got)
	}
	cycle.setSessionActive(false)
	if got := d.dispatch("sleep-ready"); !strings.HasPrefix(got, "err sleep-ready") {
		t.Fatalf("unsecured inactive guard = %q, want readiness error", got)
	}
	if !cycle.protect() {
		t.Fatal("could not secure inactive session")
	}
	if got := d.dispatch("sleep-ready"); got != "ok" {
		t.Fatalf("secured inactive delay guard = %q, want ok", got)
	}
	delay.release()
	if got := d.dispatch("sleep-ready"); !strings.HasPrefix(got, "err sleep-ready") {
		t.Fatalf("inactive guard without delay = %q, want readiness error", got)
	}
}

func TestUnlockIsRejectedDuringSleepOrAnInactiveSession(t *testing.T) {
	t.Run("sleeping", func(t *testing.T) {
		cycle := &sleepCycle{
			block:         &fakeSleepInhibitor{heldState: true},
			delay:         &fakeSleepInhibitor{heldState: true},
			sleepingCheck: func(context.Context) (bool, error) { return true, nil },
		}
		if err := cycle.prepareUnlock(); err == nil {
			t.Fatal("unlock was allowed after login1 began preparing for sleep")
		}
	})

	t.Run("inactive", func(t *testing.T) {
		block := &fakeSleepInhibitor{}
		cycle := &sleepCycle{
			block: block,
			delay: &fakeSleepInhibitor{heldState: true},
		}
		cycle.setSessionActive(false)
		if err := cycle.prepareUnlock(); err == nil {
			t.Fatal("inactive graphical session was allowed to unlock")
		}
		if block.attemptCount() != 0 {
			t.Fatal("inactive unlock acquired a global block it could not release")
		}
	})
}

func TestUnlockPrepareKicksFallbackGuardWatcher(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_SESSION_ID", "unlock-kick")
	cycle := &sleepCycle{
		block: &fakeSleepInhibitor{heldState: true},
		delay: &fakeSleepInhibitor{heldState: true},
	}
	cycle.setSessionActive(true)
	d := &daemon{
		sleep:     cycle,
		guardKick: make(chan struct{}, 1),
	}

	if got := d.dispatch("unlock-prepare session unlock-kick"); got != "ok" {
		t.Fatalf("unlock prepare = %q, want ok", got)
	}
	select {
	case <-d.guardKick:
	default:
		t.Fatal("unlock prepare did not kick the fallback guard watcher")
	}
}

func TestLockDispatchKicksFallbackGuardWatcher(t *testing.T) {
	t.Setenv("XDG_SESSION_ID", "lock-kick")
	oldRunning, oldProof := lockProcessRunning, lockProofValid
	lockProcessRunning = func() bool { return true }
	lockProofValid = func() bool { return true }
	t.Cleanup(func() {
		lockProcessRunning, lockProofValid = oldRunning, oldProof
	})
	d := &daemon{guardKick: make(chan struct{}, 1)}
	if got := d.dispatch("lock"); got != "ok" {
		t.Fatalf("lock = %q, want ok", got)
	}
	select {
	case <-d.guardKick:
	default:
		t.Fatal("lock did not kick the fallback guard watcher")
	}
}

func TestSleepAndResumeKickWatchers(t *testing.T) {
	guardKicks, resumeKicks := 0, 0
	cycle := &sleepCycle{
		block:      &fakeSleepInhibitor{heldState: true},
		delay:      &fakeSleepInhibitor{heldState: true},
		lock:       func(time.Time) error { return nil },
		wake:       func(context.Context) {},
		now:        time.Now,
		logf:       func(string, ...any) {},
		guardKick:  func() { guardKicks++ },
		resumeKick: func() { resumeKicks++ },
	}
	cycle.setSessionActive(true)
	if !cycle.handle(true) {
		t.Fatal("sleep edge did not complete")
	}
	afterSleep := guardKicks
	if afterSleep == 0 {
		t.Fatal("sleep edge did not kick the fallback guard watcher")
	}
	if resumeKicks != 0 {
		t.Fatal("sleep edge kicked the resume watcher")
	}
	if !cycle.handle(false) {
		t.Fatal("resume edge did not restore protection")
	}
	if guardKicks == afterSleep {
		t.Fatal("resume edge did not kick the fallback guard watcher")
	}
	if resumeKicks != 1 {
		t.Fatalf("resume watcher kicked %d times, want once", resumeKicks)
	}
}

func TestResumeClearsSleepPhaseBeforeUnlock(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_SESSION_ID", "resume-test")
	cycle := &sleepCycle{
		block:         &fakeSleepInhibitor{heldState: true},
		delay:         &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		lock:          func(time.Time) error { return nil },
		wake:          func(context.Context) {},
		lighting:      func(context.Context) {},
		sleepingCheck: func(context.Context) (bool, error) { return false, nil },
		now:           time.Now,
		logf:          func(string, ...any) {},
	}
	if !cycle.handle(true) {
		t.Fatal("sleep edge was rejected")
	}
	if !cycle.sleepInProgress() {
		t.Fatal("sleep edge did not mark the cycle in progress")
	}
	if !cycle.handle(false) {
		t.Fatal("resume did not restore sleep protection")
	}
	if cycle.sleepInProgress() {
		t.Fatal("resume left the sleep cycle permanently in progress")
	}
	if err := cycle.prepareUnlock(); err != nil {
		t.Fatalf("secure lock could not unlock after resume: %v", err)
	}
}

func TestInactiveProtectionCannotReleaseBlockAcrossUnlockInvalidation(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("XDG_SESSION_ID", "race-test")
	marker := filepath.Join(runtimeDir, "qylock.race-test.locked")
	if err := os.WriteFile(marker, []byte("generation\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	finalCheck := make(chan struct{})
	continueCheck := make(chan struct{})
	checks := 0
	block := &fakeSleepInhibitor{heldState: true}
	cycle := &sleepCycle{
		block: block,
		delay: &fakeSleepInhibitor{heldState: true, max: 15 * time.Second},
		lock: func(time.Time) error {
			if _, err := os.Stat(marker); err != nil {
				return errors.New("unlock generation was invalidated")
			}
			return nil
		},
		activeCheck: func(context.Context) (bool, error) {
			checks++
			if checks == 2 {
				close(finalCheck)
				<-continueCheck
			}
			return true, nil
		},
		sleepingCheck: func(context.Context) (bool, error) { return false, nil },
		now:           time.Now,
		logf:          func(string, ...any) {},
	}

	unlockResult := make(chan error, 1)
	go func() { unlockResult <- cycle.prepareUnlock() }()
	<-finalCheck
	cycle.setSessionActive(false)
	protectResult := make(chan bool, 1)
	go func() { protectResult <- cycle.protect() }()
	close(continueCheck)

	if err := <-unlockResult; err != nil {
		t.Fatalf("prepare unlock failed: %v", err)
	}
	if <-protectResult {
		t.Fatal("inactive protection accepted the invalidated unlock generation")
	}
	if !block.held() {
		t.Fatal("inactive protection released the hard block across proof invalidation")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("unlock proof still exists after serialized invalidation: %v", err)
	}
}

func TestLogin1OwnerLossInvalidatesReadiness(t *testing.T) {
	block := &fakeSleepInhibitor{heldState: true}
	delay := &fakeSleepInhibitor{heldState: true}
	cycle := &sleepCycle{
		block:        block,
		delay:        delay,
		retryProtect: make(chan struct{}, 1),
		logf:         func(string, ...any) {},
	}
	sigs := make(chan *dbus.Signal, 1)
	sigs <- &dbus.Signal{
		Name: "org.freedesktop.DBus.NameOwnerChanged",
		Body: []any{login1Bus, ":1.2", ""},
	}
	err := runSleepSignals(sigs, make(chan struct{}), cycle, true, time.Second)
	if err == nil {
		t.Fatal("login1 owner loss ended the guard as a healthy shutdown")
	}
	if block.held() || delay.held() || cycle.ready() {
		t.Fatal("login1 owner loss left stale inhibitor file descriptors ready")
	}
}

func TestRemovedLogin1SessionStopsSleepLockRetries(t *testing.T) {
	block := &fakeSleepInhibitor{heldState: true}
	delay := &fakeSleepInhibitor{heldState: true}
	cycle := &sleepCycle{
		sessionID:    "old-session",
		block:        block,
		delay:        delay,
		retryProtect: make(chan struct{}, 1),
		logf:         func(string, ...any) {},
	}
	sigs := make(chan *dbus.Signal, 2)
	sigs <- &dbus.Signal{
		Name: login1Interface + ".SessionRemoved",
		Path: login1Path,
		Body: []any{"other-session", dbus.ObjectPath("/org/freedesktop/login1/session/other")},
	}
	sigs <- &dbus.Signal{
		Name: login1Interface + ".SessionRemoved",
		Path: login1Path,
		Body: []any{"old-session", dbus.ObjectPath("/org/freedesktop/login1/session/old")},
	}
	if err := runSleepSignals(sigs, make(chan struct{}), cycle, true, time.Second); !errors.Is(err, errLogin1SessionRemoved) {
		t.Fatalf("removed session returned %v", err)
	}
}

func TestClosedLogin1SignalStreamRetainsHardBlockForReconnect(t *testing.T) {
	block := &fakeSleepInhibitor{heldState: true}
	delay := &fakeSleepInhibitor{heldState: true}
	cycle := &sleepCycle{
		block:        block,
		delay:        delay,
		retryProtect: make(chan struct{}, 1),
		logf:         func(string, ...any) {},
	}
	d := &daemon{}
	d.setSleepCycle(cycle)
	sigs := make(chan *dbus.Signal)
	close(sigs)
	err := runSleepSignals(sigs, make(chan struct{}), cycle, true, time.Second)
	d.setSleepCycle(nil)
	if err == nil {
		t.Fatal("closed login1 signal stream ended the guard as healthy")
	}
	if d.currentSleepCycle() != nil {
		t.Fatal("disconnected signal stream remained published as IPC-ready")
	}
	if !block.held() || !delay.held() {
		t.Fatal("signal loss dropped protection before a replacement could overlap it")
	}
}

func TestLogin1ConnectionLossRetainsHardBlockForReconnect(t *testing.T) {
	block := &fakeSleepInhibitor{heldState: true}
	delay := &fakeSleepInhibitor{heldState: true}
	disconnected := make(chan struct{})
	cycle := &sleepCycle{
		block:        block,
		delay:        delay,
		retryProtect: make(chan struct{}, 1),
		disconnected: disconnected,
		logf:         func(string, ...any) {},
	}
	close(disconnected)
	err := runSleepSignals(
		make(chan *dbus.Signal),
		make(chan struct{}),
		cycle,
		true,
		time.Second,
	)
	if !errors.Is(err, errLogin1SignalLost) {
		t.Fatalf("connection loss returned %v, want signal-loss reconnect", err)
	}
	if !block.held() || !delay.held() {
		t.Fatal("D-Bus connection loss dropped protection before replacement")
	}
}

func TestSessionActivationReacquiresGlobalBlock(t *testing.T) {
	path := dbus.ObjectPath("/org/freedesktop/login1/session/test")
	block := &fakeSleepInhibitor{heldState: true}
	delay := &fakeSleepInhibitor{heldState: true, max: 15 * time.Second}
	cycle := &sleepCycle{
		delay:        delay,
		block:        block,
		lock:         func(time.Time) error { return nil },
		wake:         func(context.Context) {},
		lighting:     func(context.Context) {},
		now:          time.Now,
		logf:         func(string, ...any) {},
		retryProtect: make(chan struct{}, 1),
		sessionPath:  path,
	}
	cycle.setSessionActive(false)
	if !cycle.protect() {
		t.Fatal("could not establish the initial inactive-session protection")
	}
	sigs := make(chan *dbus.Signal, 2)
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runSleepSignals(sigs, quit, cycle, true, 5*time.Millisecond)
		close(done)
	}()

	sigs <- &dbus.Signal{
		Name: propertiesInterface + ".PropertiesChanged",
		Path: path,
		Body: []any{
			login1SessionInterface,
			map[string]dbus.Variant{"Active": dbus.MakeVariant(true)},
			[]string{},
		},
	}
	deadline := time.After(500 * time.Millisecond)
	for !cycle.protected() {
		select {
		case <-deadline:
			t.Fatal("newly active session did not reacquire the global sleep block")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	sigs <- &dbus.Signal{
		Name: propertiesInterface + ".PropertiesChanged",
		Path: path,
		Body: []any{
			login1SessionInterface,
			map[string]dbus.Variant{"Active": dbus.MakeVariant(false)},
			[]string{},
		},
	}
	deadline = time.After(500 * time.Millisecond)
	for block.held() {
		select {
		case <-deadline:
			t.Fatal("inactive transition retained the global sleep block")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if !delay.held() {
		t.Fatal("inactive transition dropped the per-session delay inhibitor")
	}
	close(quit)
	<-done
}

func TestClassifySystemdUserError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want systemdUserErrorKind
	}{
		{name: "success", want: systemdUserErrorNone},
		{name: "unit absent", err: dbus.Error{Name: "org.freedesktop.systemd1.NoSuchUnit"}, want: systemdUserErrorUnitAbsent},
		{name: "service unknown", err: dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown"}, want: systemdUserErrorUnavailable},
		{name: "name has no owner", err: dbus.Error{Name: "org.freedesktop.DBus.Error.NameHasNoOwner"}, want: systemdUserErrorUnavailable},
		{name: "connection closed", err: dbus.ErrClosed, want: systemdUserErrorUnavailable},
		{name: "operation rejected", err: dbus.Error{Name: "org.freedesktop.DBus.Error.AccessDenied"}, want: systemdUserErrorOther},
		{name: "ordinary error", err: errors.New("failed"), want: systemdUserErrorOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifySystemdUserError(tt.err); got != tt.want {
				t.Fatalf("classifySystemdUserError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestQylockGuardTransientNameMatchesPrepareScript(t *testing.T) {
	const unit = "ryoku-qylock-unlock-guard-session-c"
	if got := qylockGuardTransientName(unit + ".service"); got != unit {
		t.Fatalf("transient name = %q, want %q", got, unit)
	}
}

func TestUnlockFallbackGuardIdleHasNoProcessScan(t *testing.T) {
	t.Setenv("XDG_SESSION_ID", "session-c")
	cycle := &sleepCycle{
		block: &fakeSleepInhibitor{heldState: true},
		delay: &fakeSleepInhibitor{heldState: true},
	}
	cycle.setSessionActive(true)
	d := &daemon{quit: make(chan struct{}), guardKick: make(chan struct{}, 1)}
	d.setSleepCycle(cycle)

	oldActive, oldStop, oldPID := qylockUnlockGuardActive, qylockUnlockGuardStop, lockProcessPID
	defer func() {
		qylockUnlockGuardActive, qylockUnlockGuardStop, lockProcessPID = oldActive, oldStop, oldPID
	}()
	asked, scanned := make(chan struct{}, 2), make(chan struct{}, 1)
	qylockUnlockGuardActive = func(string) bool {
		asked <- struct{}{}
		return false
	}
	lockProcessPID = func() int {
		scanned <- struct{}{}
		return 0
	}
	qylockUnlockGuardStop = func(string) error {
		t.Error("released a guard that was never there")
		return nil
	}

	done := make(chan struct{})
	go func() {
		d.releaseQylockUnlockGuardWhenReady()
		close(done)
	}()
	select {
	case <-asked:
	case <-time.After(time.Second):
		t.Fatal("watcher did not inspect the guard at startup")
	}
	select {
	case <-scanned:
		t.Fatal("idle session scanned processes without an active guard")
	case <-time.After(100 * time.Millisecond):
	}
	close(d.quit)
	<-done
}

func TestUnlockFallbackGuardRetriesAfterCycleBecomesReady(t *testing.T) {
	t.Setenv("XDG_SESSION_ID", "session-ready")
	oldFallback := qylockGuardFallback
	qylockGuardFallback = 20 * time.Millisecond
	t.Cleanup(func() { qylockGuardFallback = oldFallback })

	block := &fakeSleepInhibitor{}
	delay := &fakeSleepInhibitor{}
	cycle := &sleepCycle{block: block, delay: delay}
	cycle.setSessionActive(true)
	d := &daemon{quit: make(chan struct{}), guardKick: make(chan struct{}, 1)}
	d.setSleepCycle(cycle)

	oldActive, oldStop, oldPID := qylockUnlockGuardActive, qylockUnlockGuardStop, lockProcessPID
	t.Cleanup(func() {
		qylockUnlockGuardActive, qylockUnlockGuardStop, lockProcessPID = oldActive, oldStop, oldPID
	})
	qylockUnlockGuardActive = func(string) bool { return true }
	lockProcessPID = func() int { return 0 }
	stopped := make(chan struct{}, 1)
	qylockUnlockGuardStop = func(string) error {
		stopped <- struct{}{}
		return nil
	}

	done := make(chan struct{})
	go func() {
		d.releaseQylockUnlockGuardWhenReady()
		close(done)
	}()
	t.Cleanup(func() {
		close(d.quit)
		<-done
	})

	time.Sleep(30 * time.Millisecond)
	select {
	case <-stopped:
		t.Fatal("released the guard before sleep protection was ready")
	default:
	}
	block.mu.Lock()
	block.heldState = true
	block.mu.Unlock()
	delay.mu.Lock()
	delay.heldState = true
	delay.mu.Unlock()
	select {
	case <-stopped:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("did not retry guard release after the cycle became ready")
	}
}

func TestUnlockFallbackGuardKickRousesIdleWatcher(t *testing.T) {
	t.Setenv("XDG_SESSION_ID", "session-kick")
	cycle := &sleepCycle{
		block: &fakeSleepInhibitor{heldState: true},
		delay: &fakeSleepInhibitor{heldState: true},
	}
	cycle.setSessionActive(true)
	d := &daemon{quit: make(chan struct{}), guardKick: make(chan struct{}, 1)}
	d.setSleepCycle(cycle)

	oldActive, oldStop := qylockUnlockGuardActive, qylockUnlockGuardStop
	t.Cleanup(func() {
		qylockUnlockGuardActive, qylockUnlockGuardStop = oldActive, oldStop
	})
	checks := make(chan struct{}, 4)
	qylockUnlockGuardActive = func(string) bool {
		checks <- struct{}{}
		return false
	}
	qylockUnlockGuardStop = func(string) error {
		t.Error("released a guard that was never there")
		return nil
	}

	done := make(chan struct{})
	go func() {
		d.releaseQylockUnlockGuardWhenReady()
		close(done)
	}()
	t.Cleanup(func() {
		close(d.quit)
		<-done
	})
	select {
	case <-checks:
	case <-time.After(time.Second):
		t.Fatal("watcher did not inspect the guard at startup")
	}
	d.kickQylockGuardWatcher()
	select {
	case <-checks:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("guard kick did not rouse the idle watcher")
	}
}

func TestUnlockFallbackGuardReleasedOnlyByReadyOwningSession(t *testing.T) {
	t.Setenv("XDG_SESSION_ID", "session-b")
	cycle := &sleepCycle{
		block: &fakeSleepInhibitor{heldState: true},
		delay: &fakeSleepInhibitor{heldState: true},
	}
	cycle.setSessionActive(false)
	d := &daemon{quit: make(chan struct{}), guardKick: make(chan struct{}, 1)}
	d.setSleepCycle(cycle)

	oldActive, oldStop, oldPID := qylockUnlockGuardActive, qylockUnlockGuardStop, lockProcessPID
	t.Cleanup(func() {
		qylockUnlockGuardActive, qylockUnlockGuardStop, lockProcessPID = oldActive, oldStop, oldPID
	})
	var mu sync.Mutex
	present := true
	clientPID := os.Getpid()
	stopped := make(chan string, 2)
	qylockUnlockGuardActive = func(string) bool {
		mu.Lock()
		defer mu.Unlock()
		return present
	}
	qylockUnlockGuardStop = func(unit string) error {
		mu.Lock()
		present = false
		mu.Unlock()
		stopped <- unit
		return nil
	}
	lockProcessPID = func() int {
		mu.Lock()
		defer mu.Unlock()
		return clientPID
	}
	done := make(chan struct{})
	go func() {
		d.releaseQylockUnlockGuardWhenReady()
		close(done)
	}()
	t.Cleanup(func() {
		close(d.quit)
		<-done
	})

	select {
	case unit := <-stopped:
		t.Fatalf("inactive daemon released another session's fallback guard: %s", unit)
	case <-time.After(100 * time.Millisecond):
	}
	cycle.setSessionActive(true)
	d.kickQylockGuardWatcher()
	select {
	case unit := <-stopped:
		t.Fatalf("active daemon released the guard before qylock exited: %s", unit)
	case <-time.After(100 * time.Millisecond):
	}
	mu.Lock()
	clientPID = 0
	mu.Unlock()
	d.kickQylockGuardWatcher()
	select {
	case unit := <-stopped:
		if unit != "ryoku-qylock-unlock-guard-session-b.service" {
			t.Fatalf("released %q, want this daemon's session guard", unit)
		}
	case <-time.After(time.Second):
		t.Fatal("ready active daemon did not release the fallback guard after qylock exited")
	}

	mu.Lock()
	present = true
	mu.Unlock()
	d.kickQylockGuardWatcher()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("unlock fallback watcher stopped after its first guard")
	}
}
