package teams

import (
	"sort"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"ryoku-rashin/internal/cron"
	"ryoku-rashin/internal/protocol"
)

// routineLate is how long after its time a run still counts as on time. A
// run found later than this was missed: the computer was off or asleep.
const routineLate = 2 * time.Minute

// Optional user services wake Oni for scheduled work after login or idle exit.
const (
	wakeTimer = "oni-routines.timer"
	wakeRunit = "oni-routines"
)

// logind's sleep signal; its argument is false once the computer resumed.
const (
	login1Path      = "/org/freedesktop/login1"
	login1Manager   = "org.freedesktop.login1.Manager"
	prepareForSleep = "PrepareForSleep"
)

// scheduler shares one timer between routines and per-seat intervals.
type scheduler struct {
	s *Service

	mu         sync.Mutex
	running    bool
	timer      *time.Timer
	next       map[string]clockEntry
	bus        *dbus.Conn
	planMu     sync.Mutex
	recovering bool

	// edit serialises read-modify-write of stored routines
	edit sync.Mutex

	// setWake holds the shared wake unit for the teams or lets it go;
	// wakeKnown and wakeOn track the hold as last set, so it changes only
	// when whether any routine is on changes
	setWake   func(on bool)
	wake      sync.Mutex
	wakeKnown bool
	wakeOn    bool
}

// clockEntry is when a routine next falls due, and the schedule that time
// came from, so an edited schedule is planned again.
type clockEntry struct {
	at           time.Time
	schedule     string
	team         string
	member       string
	everyMinutes int
}

func newScheduler(s *Service, setWake func(on bool)) *scheduler {
	return &scheduler{s: s, setWake: setWake, next: make(map[string]clockEntry)}
}

// start plans every routine that is on, counting runs that fell due while
// the daemon was down as missed, arms the timer, listens for the computer
// resuming and sets the wake unit.
func (c *scheduler) start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()
	c.refresh(true)
	c.s.startWorker(c.watchResume)
	c.s.startWorker(c.syncWake)
	c.s.recoverHeartbeatMessages()
}

// stop disarms the clock. The wake unit stays as it is, so routines wake
// Rashin again later.
func (c *scheduler) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	clear(c.next)
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if c.bus != nil {
		_ = c.bus.Close()
		c.bus = nil
	}
}

// armed reports any routine that is on in a team that is not paused.
func (c *scheduler) armed() bool {
	if len(c.live()) > 0 {
		return true
	}
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	for _, team := range c.s.state.teams() {
		for _, member := range team.Members {
			if member.Heartbeat != nil && checkMemberHeartbeat(member.Heartbeat) == nil && c.s.automaticSeatReadyLocked(team, member, protocol.HeartbeatInterval) {
				return true
			}
		}
	}
	return false
}

// rearm plans again after routines or teams changed and sets the wake unit
// when whether any routine is on changed.
func (c *scheduler) rearm() {
	c.refresh(false)
	c.s.recoverHeartbeatMessages()
	c.s.startWorker(c.syncWake)
}

// Gather policy/state snapshots outside mu: callbacks can take Service.mu.
func (c *scheduler) refresh(initial bool) {
	c.planMu.Lock()
	defer c.planMu.Unlock()
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	active := make(map[string]bool)
	for id, entry := range c.next {
		if entry.everyMinutes > 0 {
			active[id] = true
		}
	}
	c.mu.Unlock()
	now := c.s.now()
	routines := c.live()
	intervals, failures := c.s.intervalPlans(now, initial, active)
	for _, failure := range failures {
		c.s.recordWakeError(failure.team, "", failure.err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	c.plan(now, initial, routines, intervals)
	c.arm(now)
}

// due is when the clock next fires a routine, zero when it does not.
func (c *scheduler) due(id string) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.next[id].at
}

// change edits one stored routine under the edit lock. Edit returns false to
// leave the routine as it was.
func (c *scheduler) change(id string, edit func(*protocol.Routine) bool) (protocol.Routine, bool) {
	c.edit.Lock()
	defer c.edit.Unlock()
	r, ok := c.s.state.routine(id)
	if !ok || !edit(&r) {
		return r, false
	}
	if c.s.state.putRoutine(r) != nil {
		return r, false
	}
	return r, true
}

// live lists the routines that are on in teams that are not paused; none
// while the teams settings pause every team.
func (c *scheduler) live() []protocol.Routine {
	if c.s.settings().Teams.Paused {
		return nil
	}
	var out []protocol.Routine
	for _, r := range c.s.state.routines() {
		if r.Enabled && c.s.routineAutomaticAllowed(r) {
			out = append(out, r)
		}
	}
	return out
}

// plan keeps a next time for every live routine. A routine new to the clock
// is planned from now; at start, from its last run or edit, so runs that fell
// due while the daemon was down show as missed. The caller holds mu.
func (c *scheduler) plan(now time.Time, initial bool, routines []protocol.Routine, intervals map[string]clockEntry) {
	keep := make(map[string]bool)
	for _, r := range routines {
		keep[r.ID] = true
		if entry, ok := c.next[r.ID]; ok && entry.schedule == r.Schedule {
			continue
		}
		schedule, err := cron.Parse(r.Schedule)
		if err != nil {
			delete(c.next, r.ID)
			continue
		}
		from := now
		if initial {
			from = routineAnchor(r)
		}
		if at := schedule.Next(from); !at.IsZero() {
			c.next[r.ID] = clockEntry{at: at, schedule: r.Schedule}
		}
	}
	for id, entry := range intervals {
		keep[id] = true
		c.next[id] = entry
	}
	for id := range c.next {
		if !keep[id] {
			delete(c.next, id)
		}
	}
}

// routineAnchor is the time a routine's schedule counts on from: its last
// run, or when it was last edited.
func routineAnchor(r protocol.Routine) time.Time {
	anchor := r.Updated
	if r.Created.After(anchor) {
		anchor = r.Created
	}
	if r.LastRun != nil && r.LastRun.After(anchor) {
		anchor = *r.LastRun
	}
	return anchor
}

// arm sets the one timer to the earliest planned run. The caller holds mu.
func (c *scheduler) arm(now time.Time) {
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	var first time.Time
	for _, entry := range c.next {
		if first.IsZero() || entry.at.Before(first) {
			first = entry.at
		}
	}
	if !first.IsZero() {
		c.timer = time.AfterFunc(max(first.Sub(now), 0), func() { c.s.startWorker(c.tick) })
	}
}

// tick handles every routine due by the wall clock. Go timers count the
// monotonic clock, which stops while the computer sleeps, so a timer can fire
// early or late against the wall clock; tick trusts only the wall clock and
// arms again.
func (c *scheduler) tick() {
	type dueRun struct {
		id    string
		entry clockEntry
	}
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	now := c.s.now()
	var due []dueRun
	for id, entry := range c.next {
		if entry.at.After(now) {
			continue
		}
		due = append(due, dueRun{id: id, entry: entry})
		if entry.everyMinutes > 0 {
			continue
		}
		if schedule, err := cron.Parse(entry.schedule); err == nil {
			c.next[id] = clockEntry{at: schedule.Next(now), schedule: entry.schedule}
		} else {
			delete(c.next, id)
		}
	}
	c.mu.Unlock()
	sort.Slice(due, func(i, j int) bool {
		if due[i].entry.at.Equal(due[j].entry.at) {
			return due[i].id < due[j].id
		}
		return due[i].entry.at.Before(due[j].entry.at)
	})
	for _, run := range due {
		if c.s.background.Err() != nil {
			break
		}
		if run.entry.everyMinutes > 0 {
			c.s.checkHeartbeat(run.entry, now)
		} else {
			c.s.checkRoutine(run.id, run.entry.at, now)
		}
	}
	c.rearm()
}

// watchResume checks the clock at once when the computer wakes from sleep.
// Without logind on the system bus the clock still runs, it only notices
// missed runs at the next timer.
func (c *scheduler) watchResume() {
	bus, err := dbus.ConnectSystemBus(dbus.WithContext(c.s.background))
	if err != nil {
		return
	}
	err = bus.AddMatchSignal(
		dbus.WithMatchObjectPath(login1Path),
		dbus.WithMatchInterface(login1Manager),
		dbus.WithMatchMember(prepareForSleep),
	)
	if err != nil {
		_ = bus.Close()
		return
	}
	signals := make(chan *dbus.Signal, 4)
	bus.Signal(signals)
	c.mu.Lock()
	if !c.running || c.bus != nil {
		c.mu.Unlock()
		_ = bus.Close()
		return
	}
	c.bus = bus
	c.mu.Unlock()
	for {
		select {
		case <-c.s.background.Done():
			return
		case signal, ok := <-signals:
			if !ok {
				return
			}
			if signal.Name != login1Manager+"."+prepareForSleep || len(signal.Body) != 1 {
				continue
			}
			if sleeping, ok := signal.Body[0].(bool); ok && !sleeping {
				c.tick()
			}
		}
	}
}

// syncWake holds the shared wake unit for the teams while any routine is on
// and the teams settings let routines wake Oni, and lets it go otherwise.
// The unit itself goes off only once Oni's reminders let it go too.
func (c *scheduler) syncWake() {
	c.wake.Lock()
	defer c.wake.Unlock()
	on := c.armed() && c.s.settings().Teams.RoutinesWake
	if c.wakeKnown && c.wakeOn == on {
		return
	}
	c.setWake(on)
	c.wakeKnown, c.wakeOn = true, on
}
