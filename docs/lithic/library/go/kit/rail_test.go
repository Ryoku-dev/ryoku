package kit

import (
	"reflect"
	"testing"
)

func TestRailPositions(t *testing.T) {
	const start, tile, gap, foot = 96, 64, 8, 504
	cases := []struct {
		name      string
		count     int
		footIndex int
		newSlot   bool
		want      []int
	}{
		{"today", 3, 0, true, []int{96, 240, 440}},
		{"explicit last", 3, 2, true, []int{96, 240, 440}},
		{"no new", 3, 0, false, []int{96, 168, 440}},
		{"long rail", 6, 4, true, []int{96, 240, 312, 384, 368, 440}},
		{"none at foot", 2, 2, true, []int{96, 240}},
		{"one", 1, 0, true, []int{440}},
	}
	for _, test := range cases {
		got := railPositions(test.count, test.footIndex, test.newSlot, start, tile, gap, foot)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: railPositions = %v, want %v", test.name, got, test.want)
		}
	}
}
