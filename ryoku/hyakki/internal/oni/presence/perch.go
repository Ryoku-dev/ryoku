package presence

import (
	"errors"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// Why Perch sends nothing, besides ErrClosed, ErrOff and ErrNoPresenter.
var (
	ErrHidden     = errors.New("oni is hidden")
	ErrNotPerched = errors.New("oni is not perched on a window")
	ErrNoStrand   = errors.New("no strand is out for the bubble to hang on")
)

// perch is the window Oni sits on. Seq grows from 1 for the service's life
// and is never the strand's; frame is nil while detached. Nothing here arms
// a timer.
type perch struct {
	seq   uint64
	frame *feature.PerchFrame

	onDetach func(reason string)
	onRest   func(resting bool)
	// notes are the detach reasons, and rests the sleeps and wakes of a
	// perched Oni, to report once the lock is released
	notes []string
	rests []bool
}

// OnDetach sets what runs when the perch ends, with one of
// feature.PerchReasons. It runs after the service's lock is released.
func (s *Service) OnDetach(f func(reason string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.perch.onDetach = f
}

// OnRest sets what runs when a perched Oni falls asleep (true) or wakes
// (false), so what follows the window can rest with it. It runs after the
// service's lock is released.
func (s *Service) OnRest(f func(resting bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.perch.onRest = f
}

// rest notes a sleep or a wake while perched.
func (s *Service) rest(resting bool) {
	if s.perch.frame != nil {
		s.perch.rests = append(s.perch.rests, resting)
	}
}

// Perch attaches Oni to the window p names, with its Box, Output and Corner,
// and returns the perch frame's seq. The frame goes out, then a perch strand
// target curls on the window's frame. A perch on another window replaces
// the one before. With Oni off or hidden, or nothing drawing it, it sends
// nothing and fails.
func (s *Service) Perch(p feature.PerchFrame) (uint64, error) {
	s.mu.Lock()
	defer s.unlock()
	switch {
	case s.closed:
		return 0, ErrClosed
	case !s.settings.Master:
		return 0, ErrOff
	case s.settings.Hidden:
		return 0, ErrHidden
	case !s.strand.presenter:
		return 0, ErrNoPresenter
	}
	p.Attached = true
	p.Seq = s.perch.seq + 1
	if p.Box != nil {
		box := *p.Box
		p.Box = &box
	}
	if _, err := feature.EncodeFrame(&p); err != nil {
		return 0, err
	}
	s.perch.seq = p.Seq
	s.perch.frame = &p
	s.sendPerch()
	s.curl()
	return p.Seq, nil
}

// PerchUpdate follows the perched window to box on output and returns the
// perch frame's seq: a new one, with a new frame sent, when either changed,
// and the current one when neither did. A move keeps the strand on screen,
// whose target is fractions of the window; a new output curls it again.
// Detached, it returns 0 and sends nothing.
func (s *Service) PerchUpdate(box feature.Box, output string) uint64 {
	s.mu.Lock()
	defer s.unlock()
	cur := s.perch.frame
	if s.closed || cur == nil {
		return 0
	}
	if *cur.Box == box && cur.Output == output {
		return cur.Seq
	}
	next := *cur
	next.Seq = s.perch.seq + 1
	next.Box = &box
	next.Output = output
	if _, err := feature.EncodeFrame(&next); err != nil {
		return cur.Seq
	}
	moved := next.Output != cur.Output
	s.perch.seq = next.Seq
	s.perch.frame = &next
	s.sendPerch()
	if moved {
		s.curl()
	}
	return next.Seq
}

// Detach lets go of the perched window for reason, one of
// feature.PerchReasons: the strand rewinds and a detached perch frame
// follows. Detached, nothing happens.
func (s *Service) Detach(reason string) {
	s.mu.Lock()
	defer s.unlock()
	if s.closed {
		return
	}
	s.detach(reason)
	s.rearm()
}

// Perched is the current perch frame while Oni is attached.
func (s *Service) Perched() (feature.PerchFrame, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.perch.frame == nil {
		return feature.PerchFrame{}, false
	}
	p := *s.perch.frame
	box := *p.Box
	p.Box = &box
	return p, true
}

// Spot is where on output Oni stands, as fractions of it: a corner when
// pinned, the saved spot on that output otherwise. With neither it is
// false.
func (s *Service) Spot(output string) (fx, fy float64, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.place.Corner {
	case protocol.OniCornerTopLeft:
		return 0, 0, true
	case protocol.OniCornerTopRight:
		return 1, 0, true
	case protocol.OniCornerBottomLeft:
		return 0, 1, true
	case protocol.OniCornerBottomRight:
		return 1, 1, true
	}
	for _, o := range s.place.Outputs {
		if o.ID == output {
			return o.FX, o.FY, true
		}
	}
	return 0, 0, false
}

// Bubble shows one line at the tip of the strand seq went out under, or,
// with seq 0, of the strand on screen now. With no strand out it sends
// nothing.
func (s *Service) Bubble(seq uint64, tone, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.closed:
		return ErrClosed
	case !s.settings.Master:
		return ErrOff
	}
	if seq == 0 {
		seq = s.strand.out
	}
	if seq == 0 {
		return ErrNoStrand
	}
	b := feature.BubbleFrame{Seq: seq, Tone: tone, Text: text}
	if _, err := feature.EncodeFrame(&b); err != nil {
		return err
	}
	s.send(&b)
	return nil
}

// Notice shows a plain program notice beside Oni, independently of a strand.
func (s *Service) Notice(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.closed:
		return ErrClosed
	case !s.settings.Master:
		return ErrOff
	case !s.shown():
		return ErrHidden
	case !s.strand.presenter:
		return ErrNoPresenter
	}
	f := feature.NoticeFrame{Text: text}
	if _, err := feature.EncodeFrame(&f); err != nil {
		return err
	}
	s.send(&f)
	return nil
}

// Consent puts a consent question on the frames, so a renderer can show
// that one is waiting. It carries no picture. A Hands handoff card is also
// held while the task waits on it (keepHandsCard).
func (s *Service) Consent(c feature.ConsentFrame) error {
	if _, err := feature.EncodeFrame(&c); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.closed:
		return ErrClosed
	case !s.settings.Master:
		return ErrOff
	}
	s.keepHandsCard(c)
	s.keepAppApproval(c)
	s.send(&c)
	return nil
}

// detach ends the perch with the lock held: the strand on screen rewinds,
// its pending target is cancelled, and a detached frame goes out while Oni
// is on. The reason is reported to OnDetach once the lock is released.
func (s *Service) detach(reason string) {
	if s.perch.frame == nil {
		return
	}
	s.perch.frame = nil
	s.perch.notes = append(s.perch.notes, reason)
	if out := s.strand.out; out != 0 {
		s.strand.out = 0
		s.cancelPending(CancelRetracted)
		if s.settings.Master {
			s.send(&feature.StrandRetractFrame{Seq: out})
		}
	}
	if s.settings.Master {
		s.perch.seq++
		s.send(&feature.PerchFrame{Seq: s.perch.seq})
	}
}

func (s *Service) sendPerch() {
	if !s.last.on {
		return
	}
	p := *s.perch.frame
	box := *p.Box
	p.Box = &box
	s.send(&p)
}

// curl sends the strand to the perched window's own frame.
func (s *Service) curl() {
	_, _ = s.strandTo(feature.StrandTargetFrame{Mode: feature.StrandModePerch, W: 1, H: 1})
}
