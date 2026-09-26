package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

const (
	login1Bus              = "org.freedesktop.login1"
	login1Path             = dbus.ObjectPath("/org/freedesktop/login1")
	login1Interface        = "org.freedesktop.login1.Manager"
	login1SessionInterface = "org.freedesktop.login1.Session"
	propertiesInterface    = "org.freedesktop.DBus.Properties"
)

var (
	// Some hybrid-GPU panels ignore an early DPMS-on while the display link is
	// retraining. Reassert the idempotent provider action across that window.
	wakeHold       = 15 * time.Second
	wakeStep       = time.Second
	wakeActionWait = 750 * time.Millisecond

	// login1 can reject Inhibit briefly while the previous sleep operation is
	// still finishing. Retry in-order so an acquire can never overtake a later
	// PrepareForSleep(true) edge.
	sleepGuardRetry       = 2 * time.Second
	login1CallWait        = 2 * time.Second
	sleepReconnectStep    = 100 * time.Millisecond
	errLogin1OwnerRestart = errors.New("login1 D-Bus owner changed")
	errLogin1SignalLost   = errors.New("login1 signal stream ended")
)

type sleepInhibitor interface {
	acquire(context.Context) error
	release()
	held() bool
}

type sleepDelay interface {
	sleepInhibitor
	limit() time.Duration
}

type login1Inhibitor struct {
	mu   sync.Mutex
	conn *dbus.Conn
	fd   *os.File
	mode string
	why  string
	name string
}

func (i *login1Inhibitor) acquire(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.fd != nil {
		return nil
	}
	var fd dbus.UnixFD
	if err := i.conn.Object(login1Bus, login1Path).CallWithContext(
		ctx,
		login1Interface+".Inhibit",
		0,
		"sleep",
		i.name,
		i.why,
		i.mode,
	).Store(&fd); err != nil {
		return err
	}
	i.fd = os.NewFile(uintptr(fd), "ryoku-shell-sleep-"+i.mode)
	if i.fd == nil {
		return os.ErrInvalid
	}
	return nil
}

func (i *login1Inhibitor) release() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.fd == nil {
		return
	}
	_ = i.fd.Close()
	i.fd = nil
}

func (i *login1Inhibitor) held() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.fd != nil
}

type login1SleepDelay struct {
	*login1Inhibitor
	maxMu    sync.RWMutex
	maxDelay time.Duration
}

func (d *login1SleepDelay) acquire(ctx context.Context) error {
	if err := d.login1Inhibitor.acquire(ctx); err != nil {
		return err
	}
	var value dbus.Variant
	err := d.conn.Object(login1Bus, login1Path).CallWithContext(
		ctx,
		"org.freedesktop.DBus.Properties.Get",
		0,
		login1Interface,
		"InhibitDelayMaxUSec",
	).Store(&value)
	if err != nil {
		return nil
	}
	if usec, ok := value.Value().(uint64); ok {
		d.maxMu.Lock()
		d.maxDelay = time.Duration(usec) * time.Microsecond
		d.maxMu.Unlock()
	}
	return nil
}

func (d *login1SleepDelay) limit() time.Duration {
	d.maxMu.RLock()
	defer d.maxMu.RUnlock()
	return d.maxDelay
}

// lockBudget leaves 20 percent of logind's live delay allowance for D-Bus,
// scheduling, and inhibitor release, while never making a user wait more than
// twelve seconds for a broken locker.
func lockBudget(limit time.Duration) time.Duration {
	if limit <= 0 {
		return lockWait
	}
	headroom := limit / 5
	if headroom < time.Second {
		headroom = time.Second
	}
	if headroom > limit/2 {
		headroom = limit / 2
	}
	budget := limit - headroom
	if budget > 12*time.Second {
		budget = 12 * time.Second
	}
	return budget
}

type sleepCycle struct {
	delay           sleepDelay
	block           sleepInhibitor
	lock            func(time.Time) error
	wake            func(context.Context)
	lighting        func(context.Context)
	suspend         func(context.Context) error
	sleepingCheck   func(context.Context) (bool, error)
	suspendOffset   func() time.Duration
	now             func() time.Time
	logf            func(string, ...any)
	retryProtect    chan struct{}
	disconnected    <-chan struct{}
	sessionPath     dbus.ObjectPath
	activeCheck     func(context.Context) (bool, error)
	activeMu        sync.RWMutex
	active          bool
	sessionBound    bool
	inactiveSecured bool
	sleeping        bool
	lastWakeOffset  time.Duration
	suspendMu       sync.Mutex
	wakeMu          sync.Mutex
	wakeCancel      context.CancelFunc
}

func (s *sleepCycle) sessionActive() bool {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return !s.sessionBound || s.active
}

func (s *sleepCycle) setSessionActive(active bool) {
	s.activeMu.Lock()
	changed := !s.sessionBound || s.active != active
	s.sessionBound = true
	s.active = active
	if active || changed {
		s.inactiveSecured = false
	}
	s.activeMu.Unlock()
	if !active {
		s.stopWake()
	}
}

func (s *sleepCycle) inactiveSessionSecured() bool {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return s.sessionBound && !s.active && s.inactiveSecured
}

func (s *sleepCycle) setSleeping(sleeping bool) {
	s.activeMu.Lock()
	s.sleeping = sleeping
	s.activeMu.Unlock()
}

func (s *sleepCycle) sleepInProgress() bool {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return s.sleeping
}

func systemSuspendOffset() time.Duration {
	var boot, monotonic unix.Timespec
	if unix.ClockGettime(unix.CLOCK_BOOTTIME, &boot) != nil ||
		unix.ClockGettime(unix.CLOCK_MONOTONIC, &monotonic) != nil {
		return 0
	}
	return time.Duration(boot.Nano() - monotonic.Nano())
}

func (s *sleepCycle) markWakeOffset() {
	if s.suspendOffset == nil {
		return
	}
	s.activeMu.Lock()
	s.lastWakeOffset = s.suspendOffset()
	s.activeMu.Unlock()
}

func (s *sleepCycle) suspendSinceLastWake() bool {
	if s.suspendOffset == nil {
		return false
	}
	current := s.suspendOffset()
	s.activeMu.RLock()
	previous := s.lastWakeOffset
	s.activeMu.RUnlock()
	return current-previous > 20*time.Millisecond
}

func (s *sleepCycle) stopWake() {
	s.wakeMu.Lock()
	if s.wakeCancel != nil {
		s.wakeCancel()
		s.wakeCancel = nil
	}
	s.wakeMu.Unlock()
}

func (s *sleepCycle) startWake() {
	s.wakeMu.Lock()
	if s.wakeCancel != nil {
		s.wakeCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.wakeCancel = cancel
	s.wakeMu.Unlock()
	if s.wake != nil {
		go s.wake(ctx)
	}
	if s.lighting != nil {
		go s.lighting(ctx)
	}
}

func (s *sleepCycle) confirmSessionActive(ctx context.Context) error {
	if !s.sessionActive() {
		return fmt.Errorf("graphical session is not active")
	}
	if s.activeCheck == nil {
		return nil
	}
	active, err := s.activeCheck(ctx)
	if err != nil {
		return fmt.Errorf("read graphical session state: %w", err)
	}
	if !active {
		s.setSessionActive(false)
		return fmt.Errorf("graphical session is not active")
	}
	return nil
}

func acquireSleepInhibitor(inhibitor sleepInhibitor) error {
	ctx, cancel := context.WithTimeout(context.Background(), login1CallWait)
	defer cancel()
	return inhibitor.acquire(ctx)
}

func (s *sleepCycle) protected() bool {
	return s.block.held() && s.delay.held()
}

func (s *sleepCycle) ready() bool {
	if s.sessionActive() {
		return s.protected()
	}
	return s.delay.held() && !s.block.held() && s.inactiveSessionSecured()
}

// protect takes the hard block first. An inactive session releases it only
// after qylock is compositor-secure, so a startup or fast-user-switch race
// cannot leave logind with only a finite delay inhibitor protecting the lock.
func (s *sleepCycle) protect() bool {
	s.suspendMu.Lock()
	defer s.suspendMu.Unlock()
	return s.protectLocked()
}

func (s *sleepCycle) protectLocked() bool {
	active := s.sessionActive()
	if err := acquireSleepInhibitor(s.block); err != nil {
		s.logf("ryoku-shell: sleep block inhibitor unavailable: %v", err)
		return false
	}
	if err := acquireSleepInhibitor(s.delay); err != nil {
		s.logf("ryoku-shell: sleep delay inhibitor unavailable: %v", err)
		return false
	}
	if s.sessionActive() != active {
		return s.protectLocked()
	}
	if active {
		return s.protected()
	}
	if err := s.lock(s.now().Add(lockBudget(s.delay.limit()))); err != nil {
		s.logf("ryoku-shell: inactive session lock is not compositor-secure; retaining sleep block: %v", err)
		return false
	}

	s.activeMu.Lock()
	if !s.sessionBound || s.active {
		s.activeMu.Unlock()
		return s.protectLocked()
	}
	s.inactiveSecured = true
	s.block.release()
	s.activeMu.Unlock()
	return s.ready()
}

func (s *sleepCycle) requestProtectionRetry() {
	if s.retryProtect == nil {
		return
	}
	select {
	case s.retryProtect <- struct{}{}:
	default:
	}
}

func (s *sleepCycle) handle(sleeping bool) bool {
	s.suspendMu.Lock()
	defer s.suspendMu.Unlock()
	return s.handleLocked(sleeping)
}

func (s *sleepCycle) handleLocked(sleeping bool) bool {
	if sleeping {
		s.stopWake()
		s.setSleeping(true)
		if err := s.lock(s.now().Add(lockBudget(s.delay.limit()))); err != nil {
			// A delay inhibitor has a finite ceiling. Recover the hard block as
			// well, so failed re-verification cannot authorize insecure sleep.
			if blockErr := acquireSleepInhibitor(s.block); blockErr != nil {
				s.logf("ryoku-shell: suspend lock was not compositor-secure and sleep block recovery failed: %v (block: %v)", err, blockErr)
			} else {
				s.logf("ryoku-shell: suspend lock was not compositor-secure; retaining delay and block inhibitors: %v", err)
			}
			return false
		}
		s.delay.release()
		s.logf("ryoku-shell: suspend lock is compositor-secure")
		return true
	}
	s.setSleeping(false)
	s.markWakeOffset()

	// Display recovery belongs to the foreground compositor. This daemon
	// reacquires its delay and hard blocks here; session handoff closes them
	// only after the outgoing session is secure and its replacement is ready.
	if s.sessionActive() {
		s.startWake()
	} else {
		s.stopWake()
	}
	return s.protectLocked()
}

// prepareUnlock restores the hard block before qylock invalidates its secure
// marker. A compositor keeps the session locked if a lock client crashes, but
// an authenticated unlock intentionally lowers that protection and must never
// race an uncoordinated login1 sleep.
func (s *sleepCycle) prepareUnlock() error {
	s.suspendMu.Lock()
	defer s.suspendMu.Unlock()
	activeCtx, activeCancel := context.WithTimeout(context.Background(), login1CallWait)
	err := s.confirmSessionActive(activeCtx)
	activeCancel()
	if err != nil {
		return fmt.Errorf("refuse unlock outside the active graphical session: %w", err)
	}
	if err := acquireSleepInhibitor(s.block); err != nil {
		return fmt.Errorf("restore sleep block before unlock: %w", err)
	}
	if s.sleepInProgress() {
		return fmt.Errorf("system sleep is already in progress")
	}
	if s.sleepingCheck != nil {
		sleepCtx, sleepCancel := context.WithTimeout(context.Background(), login1CallWait)
		sleeping, sleepErr := s.sleepingCheck(sleepCtx)
		sleepCancel()
		if sleepErr != nil {
			return fmt.Errorf("check login1 sleep phase before unlock: %w", sleepErr)
		}
		if sleeping {
			return fmt.Errorf("system sleep is already in progress")
		}
	}
	activeCtx, activeCancel = context.WithTimeout(context.Background(), login1CallWait)
	err = s.confirmSessionActive(activeCtx)
	activeCancel()
	if err != nil {
		return fmt.Errorf("session became inactive before unlock: %w", err)
	}
	if err := clearCurrentLockProof(); err != nil {
		return err
	}
	s.activeMu.Lock()
	s.inactiveSecured = false
	s.activeMu.Unlock()
	return nil
}

// requestSuspend is the non-cancellable entry used by explicit user and idle
// requests. Lid-close supplies its own cancellation context.
func (s *sleepCycle) requestSuspend() error {
	return s.requestSuspendContext(context.Background())
}

// requestSuspendContext releases the always-held block only after qylock is
// compositor-secure and a delay lock is already in hand. Cancellation may
// arrive while qylock is securing; the session remains locked, but login1 is
// never called.
func (s *sleepCycle) requestSuspendContext(requestCtx context.Context) error {
	s.suspendMu.Lock()
	defer s.suspendMu.Unlock()
	if err := requestCtx.Err(); err != nil {
		return err
	}
	if !s.sessionActive() {
		return fmt.Errorf("graphical session is not active")
	}
	if !s.protected() {
		return fmt.Errorf("sleep guard is not ready")
	}
	if err := s.lock(s.now().Add(lockBudget(s.delay.limit()))); err != nil {
		return fmt.Errorf("session lock is not compositor-secure: %w", err)
	}
	if err := requestCtx.Err(); err != nil {
		return err
	}
	if !s.protected() {
		return fmt.Errorf("sleep protection was lost while locking")
	}
	activeCtx, activeCancel := context.WithTimeout(requestCtx, login1CallWait)
	if err := s.confirmSessionActive(activeCtx); err != nil {
		activeCancel()
		return err
	}
	activeCancel()
	if err := requestCtx.Err(); err != nil {
		return err
	}

	s.block.release()
	ctx, cancel := context.WithTimeout(requestCtx, login1CallWait)
	defer cancel()
	if err := s.suspend(ctx); err != nil {
		if !s.protectLocked() {
			s.logf("ryoku-shell: suspend failed and sleep protection could not be restored")
			s.requestProtectionRetry()
		}
		return fmt.Errorf("request suspend: %w", err)
	}
	return nil
}

type login1SessionInfo struct {
	ID   string
	UID  uint32
	User string
	Seat string
	Path dbus.ObjectPath
}

func login1SessionProperty(
	ctx context.Context,
	conn *dbus.Conn,
	path dbus.ObjectPath,
	name string,
) (dbus.Variant, error) {
	var value dbus.Variant
	err := conn.Object(login1Bus, path).CallWithContext(
		ctx,
		propertiesInterface+".Get",
		0,
		login1SessionInterface,
		name,
	).Store(&value)
	return value, err
}

func graphicalLogin1UserSession(sessionType, sessionClass string) bool {
	return sessionType == "wayland" &&
		(sessionClass == "user" || sessionClass == "user-early")
}

func inspectLogin1Session(
	ctx context.Context,
	conn *dbus.Conn,
	path dbus.ObjectPath,
) (graphical, active bool, err error) {
	typeValue, err := login1SessionProperty(ctx, conn, path, "Type")
	if err != nil {
		return false, false, err
	}
	sessionType, ok := typeValue.Value().(string)
	if !ok {
		return false, false, nil
	}
	classValue, err := login1SessionProperty(ctx, conn, path, "Class")
	if err != nil {
		return false, false, err
	}
	sessionClass, ok := classValue.Value().(string)
	if !ok || !graphicalLogin1UserSession(sessionType, sessionClass) {
		return false, false, nil
	}
	activeValue, err := login1SessionProperty(ctx, conn, path, "Active")
	if err != nil {
		return false, false, err
	}
	active, ok = activeValue.Value().(bool)
	if !ok {
		return false, false, fmt.Errorf("login1 Active property has type %T", activeValue.Value())
	}
	return true, active, nil
}

func ownGraphicalLogin1Session(
	ctx context.Context,
	conn *dbus.Conn,
) (string, dbus.ObjectPath, bool, error) {
	manager := conn.Object(login1Bus, login1Path)
	var sessions []login1SessionInfo
	if err := manager.CallWithContext(ctx, login1Interface+".ListSessions", 0).Store(&sessions); err != nil {
		return "", "", false, err
	}
	if id := os.Getenv("XDG_SESSION_ID"); id != "" {
		if !validSessionID(id) {
			return "", "", false, fmt.Errorf("invalid XDG_SESSION_ID %q", id)
		}
		for _, session := range sessions {
			if session.ID != id {
				continue
			}
			if session.UID != uint32(os.Getuid()) {
				return "", "", false, fmt.Errorf("XDG_SESSION_ID %s belongs to uid %d", id, session.UID)
			}
			graphical, active, inspectErr := inspectLogin1Session(ctx, conn, session.Path)
			if inspectErr != nil {
				return "", "", false, fmt.Errorf("inspect XDG_SESSION_ID %s: %w", id, inspectErr)
			}
			if !graphical {
				return "", "", false, fmt.Errorf("XDG_SESSION_ID %s is not a graphical user session", id)
			}
			return id, session.Path, active, nil
		}
		return "", "", false, fmt.Errorf("XDG_SESSION_ID %s is not a live login1 session", id)
	}
	var inactive *login1SessionInfo
	for i := range sessions {
		session := &sessions[i]
		if session.UID != uint32(os.Getuid()) || !validSessionID(session.ID) {
			continue
		}
		graphical, active, inspectErr := inspectLogin1Session(ctx, conn, session.Path)
		if inspectErr != nil || !graphical {
			continue
		}
		if active {
			return session.ID, session.Path, true, nil
		}
		if inactive == nil {
			inactive = session
		}
	}
	if inactive != nil {
		return inactive.ID, inactive.Path, false, nil
	}
	return "", "", false, fmt.Errorf("no login1 graphical session for uid %d", os.Getuid())
}

func sessionActiveChange(sig *dbus.Signal, path dbus.ObjectPath) (bool, bool) {
	if sig == nil ||
		sig.Name != propertiesInterface+".PropertiesChanged" ||
		sig.Path != path ||
		len(sig.Body) < 2 {
		return false, false
	}
	iface, ok := sig.Body[0].(string)
	if !ok || iface != login1SessionInterface {
		return false, false
	}
	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return false, false
	}
	value, ok := changed["Active"]
	if !ok {
		return false, false
	}
	active, ok := value.Value().(bool)
	return active, ok
}

func login1OwnerChanged(sig *dbus.Signal) bool {
	if sig == nil ||
		sig.Name != "org.freedesktop.DBus.NameOwnerChanged" ||
		len(sig.Body) < 3 {
		return false
	}
	name, nameOK := sig.Body[0].(string)
	oldOwner, oldOK := sig.Body[1].(string)
	newOwner, newOK := sig.Body[2].(string)
	return nameOK && oldOK && newOK && name == login1Bus && oldOwner != newOwner
}

// runSleepSignals serializes retries with PrepareForSleep edges. An acquire can
// therefore never overtake a later true edge and reintroduce a delay FD that the
// same edge just released.
func runSleepSignals(
	sigs <-chan *dbus.Signal,
	quit <-chan struct{},
	cycle *sleepCycle,
	initialProtected bool,
	retryAfter time.Duration,
) error {
	var retry *time.Timer
	var retryC <-chan time.Time
	scheduleRetry := func() {
		if retry != nil {
			return
		}
		retry = time.NewTimer(retryAfter)
		retryC = retry.C
	}
	stopRetry := func() {
		if retry == nil {
			return
		}
		if !retry.Stop() {
			select {
			case <-retry.C:
			default:
			}
		}
		retry = nil
		retryC = nil
	}
	defer stopRetry()
	if !initialProtected {
		scheduleRetry()
	}

	for {
		select {
		case <-cycle.disconnected:
			return errLogin1SignalLost
		case <-quit:
			return nil
		case <-cycle.retryProtect:
			scheduleRetry()
		case <-retryC:
			if !cycle.protect() {
				retry.Reset(retryAfter)
				retryC = retry.C
				continue
			}
			cycle.logf("ryoku-shell: sleep protection restored")
			retry = nil
			retryC = nil
		case sig, ok := <-sigs:
			if !ok {
				return errLogin1SignalLost
			}
			if login1OwnerChanged(sig) {
				cycle.delay.release()
				cycle.block.release()
				return errLogin1OwnerRestart
			}
			if active, changed := sessionActiveChange(sig, cycle.sessionPath); changed {
				cycle.setSessionActive(active)
				if cycle.protect() {
					stopRetry()
				} else {
					scheduleRetry()
				}
				continue
			}
			if sig == nil ||
				sig.Name != login1Interface+".PrepareForSleep" ||
				len(sig.Body) < 1 {
				continue
			}
			sleeping, ok := sig.Body[0].(bool)
			if !ok {
				continue
			}
			if sleeping {
				stopRetry()
				cycle.handle(true)
				continue
			}
			if cycle.handle(false) {
				stopRetry()
			} else {
				scheduleRetry()
			}
		}
	}
}

type connectedSleepGuard struct {
	conn  *dbus.Conn
	sigs  chan *dbus.Signal
	cycle *sleepCycle
}

func (g *connectedSleepGuard) release() {
	g.cycle.stopWake()
	g.cycle.delay.release()
	g.cycle.block.release()
	_ = g.conn.Close()
}

func (d *daemon) connectSleepGuard() (_ *connectedSleepGuard, err error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect to the system bus: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = conn.Close()
		}
	}()

	sessionCtx, sessionCancel := context.WithTimeout(context.Background(), login1CallWait)
	sessionID, sessionPath, sessionActive, err := ownGraphicalLogin1Session(sessionCtx, conn)
	sessionCancel()
	if err != nil {
		return nil, fmt.Errorf("identify this graphical session: %w", err)
	}
	if current := os.Getenv("XDG_SESSION_ID"); current == "" {
		if err := os.Setenv("XDG_SESSION_ID", sessionID); err != nil {
			return nil, fmt.Errorf("bind discovered graphical session %s: %w", sessionID, err)
		}
	} else if current != sessionID {
		return nil, fmt.Errorf("selected graphical session %s does not match XDG_SESSION_ID %s", sessionID, current)
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender(login1Bus),
		dbus.WithMatchObjectPath(login1Path),
		dbus.WithMatchInterface(login1Interface),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		return nil, fmt.Errorf("install sleep signal match: %w", err)
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender(login1Bus),
		dbus.WithMatchObjectPath(sessionPath),
		dbus.WithMatchInterface(propertiesInterface),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		return nil, fmt.Errorf("install session activity match: %w", err)
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchObjectPath(dbus.ObjectPath("/org/freedesktop/DBus")),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, login1Bus),
	); err != nil {
		return nil, fmt.Errorf("install login1 owner match: %w", err)
	}

	sigs := make(chan *dbus.Signal, 16)
	conn.Signal(sigs)
	block := &login1Inhibitor{
		conn: conn,
		mode: "block",
		name: "ryoku-shell",
		why:  "route suspend through a compositor-secure session lock",
	}
	delay := &login1SleepDelay{login1Inhibitor: &login1Inhibitor{
		conn: conn,
		mode: "delay",
		name: "ryoku-shell",
		why:  "finish securing the session lock before suspend",
	}}
	cycle := &sleepCycle{
		delay:        delay,
		block:        block,
		lock:         ensureSessionLocked,
		wake:         d.holdAwake,
		lighting:     applyWakeLighting,
		retryProtect: make(chan struct{}, 1),
		disconnected: conn.Context().Done(),
		sessionPath:  sessionPath,
		activeCheck: func(ctx context.Context) (bool, error) {
			value, err := login1SessionProperty(ctx, conn, sessionPath, "Active")
			if err != nil {
				return false, err
			}
			active, ok := value.Value().(bool)
			if !ok {
				return false, fmt.Errorf("login1 Active property has type %T", value.Value())
			}
			return active, nil
		},
		suspend: func(ctx context.Context) error {
			return conn.Object(login1Bus, login1Path).
				CallWithContext(ctx, login1Interface+".Suspend", 0, false).Err
		},
		sleepingCheck: func(ctx context.Context) (bool, error) {
			var value dbus.Variant
			err := conn.Object(login1Bus, login1Path).CallWithContext(
				ctx,
				propertiesInterface+".Get",
				0,
				login1Interface,
				"PreparingForSleep",
			).Store(&value)
			if err != nil {
				return false, err
			}
			sleeping, ok := value.Value().(bool)
			if !ok {
				return false, fmt.Errorf("login1 PreparingForSleep property has type %T", value.Value())
			}
			return sleeping, nil
		},
		suspendOffset: systemSuspendOffset,
		now:           time.Now,
		logf:          log.Printf,
	}
	cycle.setSessionActive(sessionActive)
	cycle.markWakeOffset()
	keep = true
	return &connectedSleepGuard{conn: conn, sigs: sigs, cycle: cycle}, nil
}

func (d *daemon) superviseSleepWake() {
	var stale *connectedSleepGuard
	var lastFailureLog time.Time
	defer func() {
		d.setSleepCycle(nil)
		if stale != nil {
			stale.release()
		}
	}()

	for {
		select {
		case <-d.quit:
			return
		default:
		}

		guard, err := d.connectSleepGuard()
		if err == nil {
			activeCtx, activeCancel := context.WithTimeout(context.Background(), login1CallWait)
			active, activeErr := guard.cycle.activeCheck(activeCtx)
			activeCancel()
			if activeErr != nil {
				err = fmt.Errorf("refresh graphical session activity: %w", activeErr)
			} else {
				guard.cycle.setSessionActive(active)
				if !guard.cycle.protect() {
					err = fmt.Errorf("acquire live login1 sleep protection")
				}
			}
		}
		if err == nil {
			sleepCtx, sleepCancel := context.WithTimeout(context.Background(), login1CallWait)
			sleeping, sleepErr := guard.cycle.sleepingCheck(sleepCtx)
			sleepCancel()
			if sleepErr != nil {
				err = fmt.Errorf("refresh login1 sleep phase: %w", sleepErr)
			} else if sleeping {
				if !guard.cycle.handle(true) {
					err = fmt.Errorf("restore protection for in-progress sleep")
				}
			} else if stale != nil &&
				(stale.cycle.sleepInProgress() || stale.cycle.suspendSinceLastWake()) {
				if !guard.cycle.handle(false) {
					err = fmt.Errorf("replay wake recovery after lost login1 signal")
				}
			}
		}
		if err != nil {
			if guard != nil {
				guard.release()
			}
			if time.Since(lastFailureLog) >= 2*time.Second {
				log.Printf("ryoku-shell: sleep guard reconnect pending: %v", err)
				lastFailureLog = time.Now()
			}
			timer := time.NewTimer(sleepReconnectStep)
			select {
			case <-d.quit:
				if !timer.Stop() {
					<-timer.C
				}
				return
			case <-timer.C:
				continue
			}
		}

		d.setSleepCycle(guard.cycle)
		if stale != nil {
			stale.release()
			stale = nil
		}
		err = runSleepSignals(guard.sigs, d.quit, guard.cycle, true, sleepGuardRetry)
		d.setSleepCycle(nil)
		if err == nil {
			guard.release()
			return
		}
		log.Printf("ryoku-shell: sleep guard reconnecting: %v", err)
		if errors.Is(err, errLogin1OwnerRestart) {
			// The old FDs died with logind, but its phase and suspend-clock
			// baseline still identify a missed resume on the replacement.
			guard.release()
			stale = guard
		} else {
			// A lost signal stream does not invalidate login1's inhibitor FDs.
			// Keep the old hard block until its replacement is fully protected.
			stale = guard
		}
	}
}

var (
	qylockUnlockGuardActive = func(unit string) bool {
		return exec.Command("systemctl", "--user", "is-active", "--quiet", unit).Run() == nil
	}
	qylockUnlockGuardStop = func(unit string) error {
		return exec.Command("systemctl", "--user", "stop", unit).Run()
	}
)

func (d *daemon) releaseQylockUnlockGuardWhenReady() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-d.quit:
			return
		case <-ticker.C:
			cycle := d.currentSleepCycle()
			if cycle == nil || !cycle.ready() || !cycle.sessionActive() ||
				lockProcessRunning() {
				continue
			}
			session := lockSessionID()
			if session == "" {
				continue
			}
			unit := "ryoku-qylock-unlock-guard-" + session + ".service"
			if !qylockUnlockGuardActive(unit) {
				continue
			}
			if err := qylockUnlockGuardStop(unit); err != nil {
				log.Printf("ryoku-shell: release qylock unlock sleep guard: %v", err)
			}
		}
	}
}

// startSleepWake keeps one protected login1 connection published to IPC. A
// broken signal stream reconnects in-process so the shell's general crash
// budget is never consumed and an old hard block overlaps its replacement.
func (d *daemon) startSleepWake() {
	go d.superviseSleepWake()
	go d.releaseQylockUnlockGuardWhenReady()
}

func applyWakeLighting(ctx context.Context) {
	if _, err := exec.LookPath("ryoku-hub"); err != nil {
		return
	}
	if err := exec.CommandContext(ctx, "ryoku-hub", "lighting", "apply").Run(); err != nil &&
		ctx.Err() == nil {
		log.Printf("ryoku-shell: restore lighting after wake: %v", err)
	}
}

func outputStatePaths() (string, string) {
	dir := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "ryoku-idle-output")
	if os.Getenv("XDG_RUNTIME_DIR") == "" {
		dir = filepath.Join(os.TempDir(), "ryoku-idle-output")
	}
	return filepath.Join(dir, "lock"), filepath.Join(dir, "generation")
}

func withOutputStateLock(fn func(string) error) error {
	lockPath, generationPath := outputStatePaths()
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN) }()
	return fn(generationPath)
}

func readOutputGeneration(path string) string {
	value, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(value))
}

// holdAwake shares ryoku-idle's lock and generation token. A newer idle-off
// edge waits out at most one provider call, replaces the token, and cancels
// every remaining resume retry before any stale output-on can run.
func (d *daemon) holdAwake(ctx context.Context) {
	var generation string
	if err := withOutputStateLock(func(path string) error {
		generation = readOutputGeneration(path)
		return nil
	}); err != nil {
		log.Printf("ryoku-shell: open output transition state: %v", err)
		return
	}

	deadline := time.Now().Add(wakeHold)
	wakeCtx, stopWake := context.WithCancel(ctx)
	defer stopWake()
	go func() {
		select {
		case <-d.quit:
			stopWake()
		case <-wakeCtx.Done():
		}
	}()
	succeeded := false
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		cancelled := false
		err := withOutputStateLock(func(path string) error {
			if readOutputGeneration(path) != generation {
				cancelled = true
				return nil
			}
			attemptWait := remaining
			if attemptWait > wakeActionWait {
				attemptWait = wakeActionWait
			}
			attemptCtx, cancel := context.WithTimeout(wakeCtx, attemptWait)
			defer cancel()
			err := d.wmc.ActContext(attemptCtx, "output.power", "on")
			if err == nil {
				succeeded = true
			}
			return err
		})
		if cancelled || wakeCtx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("ryoku-shell: wake output power: %v", err)
		}
		remaining = time.Until(deadline)
		if remaining <= 0 {
			break
		}
		pause := wakeStep
		if remaining < pause {
			pause = remaining
		}
		timer := time.NewTimer(pause)
		select {
		case <-wakeCtx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
	if !succeeded {
		return
	}
	if err := withOutputStateLock(func(path string) error {
		if readOutputGeneration(path) == generation {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Printf("ryoku-shell: finish output transition: %v", err)
	}
}
