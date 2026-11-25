package presence

import (
	"errors"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
)

// ArrivalTimeout is how long a strand target waits for the presenter's
// report before it is cancelled, the spec's session.arrivalTimeoutMs. The
// spec is embedded and its loader tested, so it loads; the number written
// here is that same value.
var ArrivalTimeout = func() time.Duration {
	if s, err := spec.Load(); err == nil && s.Session.ArrivalTimeoutMs > 0 {
		return time.Duration(s.Session.ArrivalTimeoutMs * float64(time.Millisecond))
	}
	return 10 * time.Second
}()

// Why Strand sends nothing.
var (
	ErrClosed      = errors.New("oni presence has stopped")
	ErrOff         = errors.New("oni is off")
	ErrNoPresenter = errors.New("no surface is drawing oni")
)

// Why a pending strand target ended without arriving. A cancelled target
// advances nothing.
const (
	CancelSuperseded    = "superseded"
	CancelRetracted     = "retracted"
	CancelPresenterLost = "presenter-lost"
	CancelTimeout       = "timeout"
	CancelOff           = "oni-off"
	CancelClosed        = "closed"
)

// strand is the one strand target the daemon waits on. Seq grows from 1 for
// the service's life, so a report for an older target never matches.
type strand struct {
	presenter bool
	seq       uint64
	// out is the target whose strand is on screen until it is retracted
	out uint64
	// pending is the target whose arrival is awaited; 0 when none is
	pending uint64
	timer   Timer
	armed   uint64

	onArrive func(seq uint64, skipped string)
	onCancel func(seq uint64, why string)
	notes    []strandNote
}

// strandNote is an arrival or a cancellation to report once the lock is
// released.
type strandNote struct {
	seq     uint64
	skipped string
	why     string
	arrived bool
}

// OnArrive sets what runs when the pending target lands, or is reported
// skipped: the work that waited on it may advance. It runs after the
// service's lock is released.
func (s *Service) OnArrive(f func(seq uint64, skipped string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strand.onArrive = f
}

// OnCancel sets what runs when the pending target ends without a report:
// superseded, retracted, its presenter gone, Oni off, the presence stopped,
// or ArrivalTimeout passed. It runs after the service's lock is released.
func (s *Service) OnCancel(f func(seq uint64, why string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strand.onCancel = f
}

// Presenter records whether a surface is drawing Oni. A presenter that
// leaves, or one that starts over, takes its strand and its perch with it
// and cancels the pending target.
func (s *Service) Presenter(on bool) {
	s.mu.Lock()
	defer s.unlock()
	if s.closed {
		return
	}
	s.detach(feature.PerchPresenterLost)
	s.strand.presenter = on
	s.strand.out = 0
	s.cancelPending(CancelPresenterLost)
	s.rearm()
}

// Strand sends the strand to t and returns the seq the presenter reports it
// under. A sleeping Oni wakes first. A newer target supersedes the pending
// one. With no presenter, or Oni off, it sends nothing and fails at once; a
// perch target needs an attached perch.
func (s *Service) Strand(t feature.StrandTargetFrame) (uint64, error) {
	s.mu.Lock()
	defer s.unlock()
	if t.Mode == feature.StrandModePerch && s.perch.frame == nil {
		return 0, ErrNotPerched
	}
	return s.strandTo(t)
}

// strandTo is Strand with the lock held.
func (s *Service) strandTo(t feature.StrandTargetFrame) (uint64, error) {
	switch {
	case s.closed:
		return 0, ErrClosed
	case !s.settings.Master:
		return 0, ErrOff
	case !s.strand.presenter:
		return 0, ErrNoPresenter
	}
	t.Seq = s.strand.seq + 1
	if _, err := feature.EncodeFrame(&t); err != nil {
		return 0, err
	}
	s.strand.seq = t.Seq
	s.cancelPending(CancelSuperseded)
	if s.asleep && s.shown() {
		s.wake()
		s.publish()
	}
	s.strand.pending, s.strand.out = t.Seq, t.Seq
	s.send(&t)
	s.armArrival()
	s.rearm()
	return t.Seq, nil
}

// Arrived takes the presenter's report for seq. Only the pending target
// advances, once; a stale, repeated or unknown seq changes nothing. The
// caller checks the report came from the presenter.
func (s *Service) Arrived(seq uint64, skipped string) bool {
	s.mu.Lock()
	defer s.unlock()
	if s.closed || seq == 0 || seq != s.strand.pending {
		return false
	}
	s.strand.pending = 0
	s.disarmArrival()
	if skipped != "" && s.strand.out == seq {
		s.strand.out = 0
	}
	s.strand.notes = append(s.strand.notes, strandNote{seq: seq, skipped: skipped, arrived: true})
	s.rearm()
	return true
}

// Retract rewinds the strand on screen, cancelling its target if it has not
// landed. Nothing out, nothing sent. It says whether the presenter was told
// to draw the strand back.
func (s *Service) Retract() bool {
	s.mu.Lock()
	defer s.unlock()
	if s.closed || s.strand.out == 0 {
		return false
	}
	seq := s.strand.out
	s.strand.out = 0
	s.cancelPending(CancelRetracted)
	sent := false
	if s.settings.Master {
		s.send(&feature.StrandRetractFrame{Seq: seq})
		sent = true
	}
	s.rearm()
	return sent
}

// cancelPending ends the pending target without advancing it.
func (s *Service) cancelPending(why string) {
	if s.strand.pending == 0 {
		return
	}
	s.strand.notes = append(s.strand.notes, strandNote{seq: s.strand.pending, why: why})
	s.strand.pending = 0
	s.disarmArrival()
}

// armArrival is the backstop for a presenter that never reports. It is
// armed only while a target is pending.
func (s *Service) armArrival() {
	s.disarmArrival()
	s.strand.armed++
	armed := s.strand.armed
	s.strand.timer = s.clock.AfterFunc(ArrivalTimeout, func() { s.arrivalTimedOut(armed) })
}

func (s *Service) disarmArrival() {
	if s.strand.timer != nil {
		s.strand.timer.Stop()
		s.strand.timer = nil
	}
}

func (s *Service) arrivalTimedOut(armed uint64) {
	s.mu.Lock()
	defer s.unlock()
	if armed != s.strand.armed || s.closed || s.strand.pending == 0 {
		return
	}
	s.strand.timer = nil
	s.cancelPending(CancelTimeout)
	s.rearm()
}

// unlock releases the lock, then reports the arrivals, cancellations,
// detaches and perched rests made while it was held, so OnArrive, OnCancel,
// OnDetach and OnRest may call back in.
func (s *Service) unlock() {
	notes, arrive, cancel := s.strand.notes, s.strand.onArrive, s.strand.onCancel
	detached, onDetach := s.perch.notes, s.perch.onDetach
	rests, onRest := s.perch.rests, s.perch.onRest
	s.strand.notes, s.perch.notes, s.perch.rests = nil, nil, nil
	s.mu.Unlock()
	for _, n := range notes {
		switch {
		case n.arrived && arrive != nil:
			arrive(n.seq, n.skipped)
		case !n.arrived && cancel != nil:
			cancel(n.seq, n.why)
		}
	}
	if onRest != nil {
		for _, resting := range rests {
			onRest(resting)
		}
	}
	if onDetach != nil {
		for _, reason := range detached {
			onDetach(reason)
		}
	}
}
