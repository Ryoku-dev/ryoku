package main

import (
	"sync"
	"time"
)

// randomRotation runs the timed random-wallpaper loop the picker's settings
// drive through wall.random_start/stop/status.
type randomRotation struct {
	mu       sync.Mutex
	stopCh   chan struct{}
	interval int64
	types    []string
	favOnly  bool
}

func newRandomRotation() *randomRotation { return &randomRotation{} }

func (r *randomRotation) start(intervalSecs int64, types []string, favOnly bool, tick func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopCh != nil {
		close(r.stopCh)
	}
	if intervalSecs < 5 {
		intervalSecs = 5
	}
	stop := make(chan struct{})
	r.stopCh = stop
	r.interval = intervalSecs
	r.types = types
	r.favOnly = favOnly
	go func() {
		t := time.NewTicker(time.Duration(intervalSecs) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				tick()
			case <-stop:
				return
			}
		}
	}()
}

// stop ends the loop and reports whether one was running.
func (r *randomRotation) stop() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopCh == nil {
		return false
	}
	close(r.stopCh)
	r.stopCh = nil
	return true
}

func (r *randomRotation) status() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopCh == nil {
		return map[string]interface{}{"running": false}
	}
	return map[string]interface{}{
		"running":         true,
		"interval":        r.interval,
		"types":           r.types,
		"favourites_only": r.favOnly,
	}
}
