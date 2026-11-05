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
	held       bool
	recovering bool
	started    time.Time
	attempts   int
}

func newLockRecoveryPolicy(now func() time.Time, launch func() error) *lockRecoveryPolicy {
	return &lockRecoveryPolicy{now: now, launch: launch}
}

func (p *lockRecoveryPolicy) confirmed() {
	p.mu.Lock()
	p.held = true
	p.recovering = false
	p.attempts = 0
	p.mu.Unlock()
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
	go func() {
		ticker := time.NewTicker(lockRecoveryPoll)
		defer ticker.Stop()
		for {
			select {
			case <-d.quit:
				return
			case <-ticker.C:
				if d.lockQuiesced.Load() || !d.lockRecovery.active() {
					continue
				}
				launched, err := d.lockRecovery.step(lockProofValid())
				if err != nil {
					log.Printf("ryoku-shell: relaunch qylock while session lock is held: %v", err)
				} else if launched {
					log.Printf("ryoku-shell: qylock proof disappeared while session lock is held; relaunching")
				}
			}
		}
	}()
}
