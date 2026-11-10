package gitgraph

import (
	"sort"

	"ryoku-rashin/internal/protocol"
)

type SegmentKind uint8

const (
	Straight SegmentKind = iota
	MergeOut
	ConvergeIn
)

type Segment struct {
	From  int
	To    int
	Color int
	Kind  SegmentKind
}

type Row struct {
	Commit   string
	Lane     int
	Color    int
	Segments []Segment
	Above    []Segment
	IsMerge  bool
	IsRoot   bool
	Width    int
}

type lane struct {
	waitingFor string
	color      int
	active     bool
}

type Layout struct {
	lanes        []lane
	waitingIndex map[string][]int
	nextColor    uint32
	maxLanes     int
}

func New() *Layout {
	return &Layout{waitingIndex: make(map[string][]int)}
}

// Append lays out commits in newest-first topological order while preserving
// the boundary needed by the next page.
func (l *Layout) Append(commits []protocol.GitCommit) []Row {
	if l.waitingIndex == nil {
		l.waitingIndex = make(map[string][]int)
	}

	rows := make([]Row, 0, len(commits))
	for _, commit := range commits {
		above := l.above()
		incoming := l.incoming(commit.Hash)
		commitLane, color := l.commitLane(commit.Hash, incoming)
		delete(l.waitingIndex, commit.Hash)

		base := make([]Segment, 0, len(l.lanes)+len(incoming))
		for _, index := range incoming {
			if index == commitLane {
				continue
			}
			base = append(base, Segment{From: index, To: commitLane, Color: l.lanes[index].color, Kind: ConvergeIn})
			l.release(index)
		}

		mergeOut := make([]Segment, 0, max(0, len(commit.Parents)-1))
		created := make(map[int]struct{}, max(0, len(commit.Parents)-1))
		if len(commit.Parents) == 0 {
			l.release(commitLane)
		} else {
			first := commit.Parents[0]
			if target, ok := l.lowestWaiting(first); ok && target != commitLane {
				if target < commitLane {
					base = append(base, Segment{From: commitLane, To: target, Color: color, Kind: ConvergeIn})
					l.release(commitLane)
				} else {
					base = append(base, Segment{From: target, To: commitLane, Color: l.lanes[target].color, Kind: ConvergeIn})
					l.release(target)
					l.wait(commitLane, first, color)
				}
			} else {
				l.wait(commitLane, first, color)
			}

			for _, parent := range commit.Parents[1:] {
				if target, ok := l.lowestWaiting(parent); ok {
					mergeOut = append(mergeOut, Segment{From: commitLane, To: target, Color: l.lanes[target].color, Kind: MergeOut})
					continue
				}
				target := l.lowestFree()
				branchColor := l.allocateColor()
				l.wait(target, parent, branchColor)
				created[target] = struct{}{}
				mergeOut = append(mergeOut, Segment{From: commitLane, To: target, Color: branchColor, Kind: MergeOut})
			}
		}

		for index, current := range l.lanes {
			if !current.active {
				continue
			}
			if _, isNew := created[index]; isNew {
				continue
			}
			base = append(base, Segment{From: index, To: index, Color: current.color, Kind: Straight})
		}
		sort.SliceStable(base, func(i, j int) bool {
			if base[i].From != base[j].From {
				return base[i].From < base[j].From
			}
			if base[i].To != base[j].To {
				return base[i].To < base[j].To
			}
			if base[i].Kind != base[j].Kind {
				return base[i].Kind < base[j].Kind
			}
			return base[i].Color < base[j].Color
		})

		segments := append(base, mergeOut...)
		width := rowWidth(commitLane, above, segments)
		if width > l.maxLanes {
			l.maxLanes = width
		}
		rows = append(rows, Row{
			Commit:   commit.Hash,
			Lane:     commitLane,
			Color:    color,
			Segments: segments,
			Above:    above,
			IsMerge:  len(commit.Parents) >= 2,
			IsRoot:   len(commit.Parents) == 0,
			Width:    width,
		})
	}
	return rows
}

// Clone returns an independent copy of the current page boundary.
func (l *Layout) Clone() *Layout {
	clone := &Layout{
		lanes:        append([]lane(nil), l.lanes...),
		waitingIndex: make(map[string][]int, len(l.waitingIndex)),
		nextColor:    l.nextColor,
		maxLanes:     l.maxLanes,
	}
	for parent, lanes := range l.waitingIndex {
		clone.waitingIndex[parent] = append([]int(nil), lanes...)
	}
	return clone
}

// Lanes reports the number of active lanes at the current page boundary.
func (l *Layout) Lanes() int {
	active := 0
	for _, current := range l.lanes {
		if current.active {
			active++
		}
	}
	return active
}

func (l *Layout) above() []Segment {
	above := make([]Segment, 0, l.Lanes())
	for index, current := range l.lanes {
		if current.active {
			above = append(above, Segment{From: index, To: index, Color: current.color, Kind: Straight})
		}
	}
	return above
}

func (l *Layout) incoming(hash string) []int {
	indexed := l.waitingIndex[hash]
	incoming := make([]int, 0, len(indexed))
	for _, index := range indexed {
		if index < 0 || index >= len(l.lanes) {
			continue
		}
		current := l.lanes[index]
		if current.active && current.waitingFor == hash {
			incoming = append(incoming, index)
		}
	}
	sort.Ints(incoming)
	return incoming
}

func (l *Layout) commitLane(hash string, incoming []int) (int, int) {
	if len(incoming) > 0 {
		index := incoming[0]
		return index, l.lanes[index].color
	}
	index := l.lowestFree()
	color := l.allocateColor()
	l.lanes[index] = lane{waitingFor: hash, color: color, active: true}
	return index, color
}

func (l *Layout) lowestFree() int {
	for index, current := range l.lanes {
		if !current.active {
			return index
		}
	}
	l.lanes = append(l.lanes, lane{})
	return len(l.lanes) - 1
}

func (l *Layout) lowestWaiting(parent string) (int, bool) {
	lowest := 0
	found := false
	for _, index := range l.waitingIndex[parent] {
		if index < 0 || index >= len(l.lanes) {
			continue
		}
		current := l.lanes[index]
		if !current.active || current.waitingFor != parent {
			continue
		}
		if !found || index < lowest {
			lowest, found = index, true
		}
	}
	return lowest, found
}

func (l *Layout) wait(index int, parent string, color int) {
	l.lanes[index] = lane{waitingFor: parent, color: color, active: true}
	l.waitingIndex[parent] = append(l.waitingIndex[parent], index)
}

func (l *Layout) release(index int) {
	l.lanes[index] = lane{}
}

func (l *Layout) allocateColor() int {
	color := int(l.nextColor % 8)
	l.nextColor++
	return color
}

func rowWidth(commitLane int, above, below []Segment) int {
	width := commitLane + 1
	for _, segment := range above {
		width = max(width, segment.From+1, segment.To+1)
	}
	for _, segment := range below {
		width = max(width, segment.From+1, segment.To+1)
	}
	return width
}
