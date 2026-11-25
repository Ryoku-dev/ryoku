package presence

import (
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// Hands records Hands' frame. An active frame is a running task: it goes
// out ahead of anything it sets off and only when it differs from the one
// before, wakes a sleeping Oni and holds off sleep, and rides in the
// snapshot after the guide. The inactive frame ends the task: it goes out
// once, only when an active one was held, and is never kept, so every
// indication goes with it and a renderer starting later sees no hands frame
// at all. An invalid frame is refused and changes nothing; so is an active
// one while Oni is off.
func (s *Service) Hands(f feature.HandsFrame) error {
	if _, err := feature.EncodeFrame(&f); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.unlock()
	if s.closed {
		return ErrClosed
	}
	if !f.Active {
		if s.hands == nil {
			return nil
		}
		s.hands, s.handsCard = nil, nil
		if s.last.on {
			s.send(&f)
		}
		s.rearm()
		return nil
	}
	if !s.settings.Master {
		return ErrOff
	}
	if s.hands != nil && *s.hands == f {
		return nil
	}
	held := f
	s.hands = &held
	if f.Phase != protocol.OniHandsStateWaiting {
		s.handsCard = nil
	}
	if s.last.on {
		sent := f
		s.send(&sent)
	}
	s.wake()
	s.publish()
	s.rearm()
	return nil
}

// handsBusy is a Hands task running: Oni stays awake while it does.
func (s *Service) handsBusy() bool { return s.hands != nil }

// keepHandsCard holds a Hands handoff card while the task waits on it, so a
// renderer that starts later still shows it, whichever scope the task works
// in; any other card is the app's.
func (s *Service) keepHandsCard(c feature.ConsentFrame) {
	if c.Feature != feature.HandsBrowser && c.Feature != feature.HandsApp && c.Feature != feature.HandsDesktop {
		return
	}
	if s.hands == nil || s.hands.Phase != protocol.OniHandsStateWaiting {
		s.handsCard = nil
		return
	}
	held := c
	s.handsCard = &held
}

// handsFrames are the running task's frame and the handoff card it waits
// on, for the snapshot; none while no task runs.
func (s *Service) handsFrames() []feature.Payload {
	if s.hands == nil {
		return nil
	}
	f := *s.hands
	out := []feature.Payload{&f}
	if c := s.handsCard; c != nil {
		card := *c
		out = append(out, &card)
	}
	return out
}
