package presence

import (
	"reflect"
	"slices"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
)

// guideStates are the spec's guide phases and the character state each
// derives; a phase that derives none is absent. Without the spec no phase
// derives a state.
var guideStates = func() map[string]string {
	s, err := spec.Load()
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(spec.GuidePhaseIDs))
	for _, id := range spec.GuidePhaseIDs {
		if st, ok := s.GuideState(id); ok && st != spec.NoState {
			out[id] = st
		}
	}
	return out
}()

// Guide records the guided walkthrough's frame, the card beside Oni. The
// frame goes out ahead of anything it sets off, and only when it differs
// from the one before: a guide held in one phase sends nothing more. A phase
// the spec maps to a state (reading, ready while pointing, waiting,
// explaining, done or nudging, thinking while keeping) wakes a sleeping Oni,
// shows that state and holds off sleep; winding and idle derive none, so a
// settled guide lets Oni rest. Idle ends the guide: the snapshot carries a
// guide frame only while one runs. An invalid frame is refused and changes
// nothing; so is a running phase while Oni is off.
func (s *Service) Guide(f feature.GuideFrame) error {
	if _, err := feature.EncodeFrame(&f); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.unlock()
	if s.closed {
		return ErrClosed
	}
	if f.Phase == feature.GuideIdle {
		if s.guide == nil {
			return nil
		}
		s.guide = nil
	} else {
		if !s.settings.Master {
			return ErrOff
		}
		if s.guide != nil && reflect.DeepEqual(*s.guide, f) {
			return nil
		}
		s.guide = cloneGuide(&f)
	}
	if s.last.on {
		s.send(cloneGuide(&f))
	}
	if s.guideState() != "" {
		s.wake()
	}
	s.publish()
	s.rearm()
	return nil
}

// guideState is the character state the running guide's phase derives, ""
// for none.
func (s *Service) guideState() string {
	if s.guide == nil {
		return ""
	}
	return guideStates[s.guide.Phase]
}

// guideFrame is a copy of the running guide's frame, nil while none runs.
func (s *Service) guideFrame() *feature.GuideFrame {
	if s.guide == nil {
		return nil
	}
	return cloneGuide(s.guide)
}

func cloneGuide(f *feature.GuideFrame) *feature.GuideFrame {
	g := *f
	g.Actions = slices.Clone(f.Actions)
	if f.Plan != nil {
		p := *f.Plan
		p.Done = slices.Clone(f.Plan.Done)
		p.Next = slices.Clone(f.Plan.Next)
		g.Plan = &p
	}
	return &g
}
