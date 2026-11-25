package teams

import (
	"math"
	"reflect"

	"ryoku-rashin/internal/protocol"
)

func memberConfigEqual(left, right protocol.Member) bool {
	left.Revision, right.Revision = 0, 0
	return reflect.DeepEqual(left, right)
}

// AdvanceMemberRevisions is the shared stale-edit boundary for forms and demo.
func AdvanceMemberRevisions(previous, next []protocol.Member) error {
	before := make(map[string]protocol.Member, len(previous))
	for _, member := range previous {
		before[member.ID] = member
	}
	for i := range next {
		member := &next[i]
		was, exists := before[member.ID]
		if !exists {
			member.Revision = 1
			continue
		}
		if member.Revision != was.Revision {
			return protocol.Errorf(protocol.CodeConflict, "%s changed since it was opened. Reload its configuration before saving.", was.Name)
		}
		if memberConfigEqual(was, *member) {
			continue
		}
		if was.Revision == math.MaxUint64 {
			return protocol.Errorf(protocol.CodeConflict, "%s has exhausted its configuration revisions.", was.Name)
		}
		member.Revision++
	}
	return nil
}
