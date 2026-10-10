package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	maxMouseSequenceSteps  = 64
	maxMouseSequenceRepeat = 100
	maxMouseSequenceDelay  = 60 * time.Second
	maxMouseSequenceRun    = 10 * time.Minute
)

type mouseMacroStep struct {
	Kind    string   `json:"kind"`
	Keys    []string `json:"keys,omitempty"`
	DelayMS int      `json:"delayMs,omitempty"`
}

func validateMouseSequence(target mouseTarget) error {
	if len(target.Sequence) == 0 {
		return fmt.Errorf("sequence target requires at least one step")
	}
	if len(target.Sequence) > maxMouseSequenceSteps {
		return fmt.Errorf("sequence target has more than %d steps", maxMouseSequenceSteps)
	}
	repeat := target.Repeat
	if repeat == 0 {
		repeat = 1
	}
	if repeat < 1 || repeat > maxMouseSequenceRepeat {
		return fmt.Errorf("sequence repeat must be between 1 and %d", maxMouseSequenceRepeat)
	}
	var delay time.Duration
	for i, step := range target.Sequence {
		switch step.Kind {
		case "down", "up", "tap":
			if len(step.Keys) == 0 {
				return fmt.Errorf("sequence step %d requires at least one key", i+1)
			}
			if step.DelayMS != 0 {
				return fmt.Errorf("sequence step %d cannot combine keys and delay", i+1)
			}
			if _, err := chordCodes(step.Keys); err != nil {
				return fmt.Errorf("sequence step %d: %w", i+1, err)
			}
		case "delay":
			if len(step.Keys) != 0 {
				return fmt.Errorf("sequence step %d delay cannot contain keys", i+1)
			}
			if step.DelayMS <= 0 || step.DelayMS > int(maxMouseSequenceDelay/time.Millisecond) {
				return fmt.Errorf("sequence step %d delay must be between 1 and %d ms", i+1, maxMouseSequenceDelay/time.Millisecond)
			}
			delay += time.Duration(step.DelayMS) * time.Millisecond
		default:
			return fmt.Errorf("sequence step %d has unknown kind %q", i+1, step.Kind)
		}
	}
	if delay*time.Duration(repeat) > maxMouseSequenceRun {
		return fmt.Errorf("sequence delays exceed %s", maxMouseSequenceRun)
	}
	return nil
}

type mouseMacroWait func(context.Context, time.Duration) bool

type mouseMacroExecutor struct {
	ctx   context.Context
	sink  func(synthEvent) error
	wait  mouseMacroWait
	mu    sync.Mutex
	runs  map[uint16]context.CancelFunc
	wg    sync.WaitGroup
	write sync.Mutex
}

func newMouseMacroExecutor(ctx context.Context, sink func(synthEvent) error, wait mouseMacroWait) *mouseMacroExecutor {
	if wait == nil {
		wait = func(ctx context.Context, delay time.Duration) bool {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
				return true
			case <-ctx.Done():
				return false
			}
		}
	}
	return &mouseMacroExecutor{ctx: ctx, sink: sink, wait: wait, runs: map[uint16]context.CancelFunc{}}
}

func (e *mouseMacroExecutor) start(source uint16, target mouseTarget) <-chan struct{} {
	done := make(chan struct{})
	if validateMouseSequence(target) != nil {
		close(done)
		return done
	}
	e.mu.Lock()
	if _, running := e.runs[source]; running {
		e.mu.Unlock()
		close(done)
		return done
	}
	ctx, cancel := context.WithCancel(e.ctx)
	e.runs[source] = cancel
	e.wg.Add(1)
	e.mu.Unlock()
	go func() {
		defer cancel()
		defer close(done)
		defer e.wg.Done()
		e.run(ctx, target)
		e.mu.Lock()
		delete(e.runs, source)
		e.mu.Unlock()
	}()
	return done
}

func (e *mouseMacroExecutor) release(source uint16, cancelOnRelease bool) {
	if !cancelOnRelease {
		return
	}
	e.mu.Lock()
	cancel := e.runs[source]
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *mouseMacroExecutor) stop() {
	e.mu.Lock()
	for _, cancel := range e.runs {
		cancel()
	}
	e.mu.Unlock()
	e.wg.Wait()
}
func (e *mouseMacroExecutor) emit(events []synthEvent) error {
	e.write.Lock()
	defer e.write.Unlock()
	for _, event := range events {
		if err := e.sink(event); err != nil {
			return err
		}
	}
	return e.sink(synthEvent{etype: evSyn})
}

func (e *mouseMacroExecutor) run(ctx context.Context, target mouseTarget) {
	repeat := target.Repeat
	if repeat == 0 {
		repeat = 1
	}
	held := []uint16{}
	defer func() {
		if len(held) == 0 {
			return
		}
		releases := make([]synthEvent, 0, len(held))
		for i := len(held) - 1; i >= 0; i-- {
			releases = append(releases, synthEvent{etype: evKey, code: held[i], value: 0})
		}
		_ = e.emit(releases)
	}()

	for range repeat {
		for _, step := range target.Sequence {
			if ctx.Err() != nil {
				return
			}
			if step.Kind == "delay" {
				if !e.wait(ctx, time.Duration(step.DelayMS)*time.Millisecond) {
					return
				}
				continue
			}
			codes, _ := chordCodes(step.Keys)
			switch step.Kind {
			case "down":
				events := make([]synthEvent, 0, len(codes))
				for _, code := range codes {
					events = append(events, synthEvent{etype: evKey, code: code, value: 1})
					held = append(held, code)
				}
				if e.emit(events) != nil {
					return
				}
			case "up":
				events := make([]synthEvent, 0, len(codes))
				for i := len(codes) - 1; i >= 0; i-- {
					events = append(events, synthEvent{etype: evKey, code: codes[i], value: 0})
					held = removeHeldMouseKey(held, codes[i])
				}
				if e.emit(events) != nil {
					return
				}
			case "tap":
				down := make([]synthEvent, 0, len(codes))
				up := make([]synthEvent, 0, len(codes))
				for _, code := range codes {
					down = append(down, synthEvent{etype: evKey, code: code, value: 1})
				}
				for i := len(codes) - 1; i >= 0; i-- {
					up = append(up, synthEvent{etype: evKey, code: codes[i], value: 0})
				}
				if e.emit(down) != nil || e.emit(up) != nil {
					return
				}
			}
		}
	}
}

func removeHeldMouseKey(held []uint16, code uint16) []uint16 {
	for i := len(held) - 1; i >= 0; i-- {
		if held[i] == code {
			return append(held[:i], held[i+1:]...)
		}
	}
	return held
}
