package state

import (
	"encoding/json"

	"ryoku-rashin/internal/protocol"
)

type Learning struct {
	Skills     uint64
	Decisions  uint64
	Rules      uint64
	Grievances uint64
}

type learningState struct {
	counts Learning
}

// a future learning kind invalidates every part rather than leaving stale data
func (l *learningState) applyLocked(event protocol.Event) (handled bool, err error) {
	if event.Event != protocol.EvtLearningChanged {
		return false, nil
	}
	var changed protocol.LearningChanged
	if err = json.Unmarshal(event.Data, &changed); err != nil {
		return true, err
	}
	switch changed.Kind {
	case protocol.LearningSkills:
		l.counts.Skills++
	case protocol.LearningDecisions:
		l.counts.Decisions++
	case protocol.LearningRules:
		l.counts.Rules++
	case protocol.LearningGrievances:
		l.counts.Grievances++
	default:
		l.counts.Skills++
		l.counts.Decisions++
		l.counts.Rules++
		l.counts.Grievances++
	}
	return true, nil
}

func (s *Store) Learning() Learning {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.learningState.counts
}
