package text

import "testing"

func TestEditDistanceReleaseByteWrapping(t *testing.T) {
	target := make([]rune, 257)
	for i := range target {
		target[i] = 'a'
	}
	equal(t, EditDistance(nil, target[:256]), uint8(0))
	equal(t, EditDistance(nil, target), uint8(1))
	equal(t, EditDistance([]rune{'a'}, target[:256]), uint8(1))
	equal(t, EditDistance([]rune{'a'}, target), uint8(0))
	mustPanic(t, func() { EditDistance(target, nil) })
}
