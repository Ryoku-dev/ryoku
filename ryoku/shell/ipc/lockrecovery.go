package main

import (
	"log"
	"sync"
	"time"
)

var lockRecoverySchedule = [...]time.Duration{
	0,
	2 * time.Second,
	6 * time.Second,
	14 * time.Second,
	26 * time.Second,
	42 * time.Second,
}

const lockRecoveryPoll = time.Second

type lockRecoveryPolicy struct {
	mu         sync.Mutex
	now        func() time.Time
	launch     func() error
	wake       chan struct{} // rouses watch when a lock is confirmed
	held       bool
	recovering bool
	started    time.Time
	attempts   int
}

func newLockRecoveryPolicy(now func() time.Time, launch func() error) *lockRecoveryPolicy {
	return &lockRecoveryPolicy{now: now, launch: launch, wake: make(chan struct{}, 1)}
}

func (p *lockRecoveryPolicy) confirmed() {
	p.mu.Lock()
	p.held = true
	p.recovering = false
	p.attempts = 0
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *lockRecoveryPolicy) unlocked() {
	p.mu.Lock()
	p.held = false
	p.recovering = false
	p.attempts = 0
	p.mu.Unlock()
}

func (p *lockRecoveryPolicy) active() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.held
}

func (p *lockRecoveryPolicy) step(secure bool) (bool, error) {
	p.mu.Lock()
	if !p.held {
		p.mu.Unlock()
		return false, nil
	}
	if secure {
		p.recovering = false
		p.attempts = 0
		p.mu.Unlock()
		return false, nil
	}
	now := p.now()
	if !p.recovering {
		p.recovering = true
		p.started = now
		p.attempts = 0
	}
	if p.attempts >= len(lockRecoverySchedule) ||
		now.Before(p.started.Add(lockRecoverySchedule[p.attempts])) {
		p.mu.Unlock()
		return false, nil
	}
	p.attempts++
	launch := p.launch
	p.mu.Unlock()
	return true, launch()
}

// watch polls the lock proof only while a session lock is held; otherwise it
// sleeps until confirmed() wakes it. skip pauses steps (generation cutover).
func (p *lockRecoveryPolicy) watch(quit <-chan struct{}, poll time.Duration, skip, secure func() bool) {
	for {
		if !p.active() {
			select {
			case <-quit:
				return
			case <-p.wake:
			}
			continue
		}
		ticker := time.NewTicker(poll)
		for p.active() {
			select {
			case <-quit:
				ticker.Stop()
				return
			case <-ticker.C:
			}
			if skip() {
				continue
			}
			launched, err := p.step(secure())
			if err != nil {
				log.Printf("ryoku-shell: relaunch qylock while session lock is held: %v", err)
			} else if launched {
				log.Printf("ryoku-shell: qylock proof disappeared while session lock is held; relaunching")
			}
		}
		ticker.Stop()
	}
}

func (d *daemon) confirmLockRecovery() {
	if d.lockRecovery != nil {
		d.lockRecovery.confirmed()
	}
}

func (d *daemon) stopLockRecovery() {
	if d.lockRecovery != nil {
		d.lockRecovery.unlocked()
	}
}

func (d *daemon) startLockRecovery() {
	d.lockRecovery = newLockRecoveryPolicy(time.Now, func() error { return lockStarter(0) })
	if lockProofValid() {
		d.lockRecovery.confirmed()
	}
	go d.lockRecovery.watch(d.quit, lockRecoveryPoll, d.lockQuiesced.Load, lockProofValid)
}
