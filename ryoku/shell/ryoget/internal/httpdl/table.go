package httpdl

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	splitAlign = 4096
	// end of a stream whose size is unknown until EOF
	unknownEnd = int64(math.MaxInt64)
	// the meter smooths over about this long
	meterTau = 2.0
)

// ported from prism-download-manager crates/prism-engine/src/speed.rs (MIT)
type meter struct {
	rate  float64
	last  time.Time
	total int64
	since time.Time
}

func (m *meter) update(total int64, now time.Time) {
	if m.last.IsZero() {
		m.last, m.total, m.since = now, total, now
		return
	}
	dt := now.Sub(m.last).Seconds()
	if dt < 0.05 {
		return
	}
	inst := float64(total-m.total) / dt
	if m.rate == 0 {
		m.rate = inst
	} else {
		m.rate += (1 - math.Exp(-dt/meterTau)) * (inst - m.rate)
	}
	m.last, m.total = now, total
}

type seg struct {
	start, pos, end int64
	// end of the owner's request in flight, the split hint, 0 for none
	reqEnd   int64
	owner    int
	racer    int
	received int64
	meter    meter
	peak     float64
}

func (s *seg) remaining() int64 { return max(0, s.end-s.pos) }
func (s *seg) done() bool       { return s.pos >= s.end }

type assignment struct {
	seg    *seg
	cursor int64
	race   bool
}

// one mutex, taken once per network read, is cheap next to the copy itself
// ported from prism-download-manager crates/prism-engine/src/segmented/table.rs (MIT)
type table struct {
	mu         sync.Mutex
	segs       []*seg
	minSplit   int64
	splittable bool
	// a slow range is raced only past this many seconds left, after warmup of samples
	raceSecs   float64
	warmup     time.Duration
	downloaded atomic.Int64
}

func alignDown(v int64) int64 { return v / splitAlign * splitAlign }
func alignUp(v int64) int64   { return (v + splitAlign - 1) / splitAlign * splitAlign }

func eta(bytes int64, rate float64) float64 {
	if rate <= 0 {
		return math.Inf(1)
	}
	return float64(bytes) / rate
}

func newTable(size, minSplit int64, splittable bool) *table {
	end := size
	if size < 0 {
		end = unknownEnd
	}
	return &table{
		segs:       []*seg{{end: end}},
		minSplit:   max(minSplit, splitAlign),
		splittable: splittable && size > 0,
		raceSecs:   3,
		warmup:     time.Second,
	}
}

func tableFrom(spans []span, size, minSplit int64) *table {
	t := &table{minSplit: max(minSplit, splitAlign), splittable: true, raceSecs: 3, warmup: time.Second}
	for _, s := range spans {
		t.segs = append(t.segs, &seg{start: s.Start, pos: s.Start + s.Done, end: s.End})
		t.downloaded.Add(s.Done)
	}
	return t
}

func (t *table) allDone() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range t.segs {
		if !s.done() {
			return false
		}
	}
	return true
}

func (t *table) remaining() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	var n int64
	for _, s := range t.segs {
		n += s.remaining()
	}
	return n
}

func (t *table) firstPending() *seg {
	t.mu.Lock()
	defer t.mu.Unlock()
	var first *seg
	for _, s := range t.segs {
		if !s.done() && (first == nil || s.start < first.start) {
			first = s
		}
	}
	return first
}

func (t *table) longest() *seg {
	t.mu.Lock()
	defer t.mu.Unlock()
	var best *seg
	for _, s := range t.segs {
		if s.pos > s.start && (best == nil || s.pos-s.start > best.pos-best.start) {
			best = s
		}
	}
	return best
}

func (t *table) claim(s *seg, worker int, reqEnd int64) assignment {
	t.mu.Lock()
	defer t.mu.Unlock()
	s.owner, s.reqEnd = worker, reqEnd
	return assignment{seg: s, cursor: s.pos}
}

func (t *table) hasWork(allowRace bool, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range t.segs {
		if s.owner == 0 && !s.done() {
			return true
		}
	}
	if !t.splittable {
		return false
	}
	for _, s := range t.segs {
		if s.owner != 0 && s.racer == 0 && s.remaining() >= 2*t.minSplit {
			return true
		}
	}
	return allowRace && t.raceCandidate(now) != nil
}

func (t *table) acquire(worker int, allowRace bool, now time.Time) (assignment, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	var free *seg
	for _, s := range t.segs {
		if s.owner == 0 && !s.done() && (free == nil || s.remaining() > free.remaining()) {
			free = s
		}
	}
	if free != nil {
		free.owner, free.reqEnd = worker, 0
		return assignment{seg: free, cursor: free.pos}, true
	}
	if !t.splittable {
		return assignment{}, false
	}
	var victim *seg
	for _, s := range t.segs {
		if s.owner != 0 && s.racer == 0 && s.remaining() >= 2*t.minSplit && (victim == nil || s.remaining() > victim.remaining()) {
			victim = s
		}
	}
	if victim != nil {
		mid := alignDown(victim.pos + victim.remaining()/2)
		at := mid
		// splitting behind the victim's request in flight keeps its socket alive
		if hint := alignUp(victim.reqEnd); hint > mid && hint < victim.end && victim.end-hint >= t.minSplit {
			at = hint
		}
		if at > victim.pos && at < victim.end {
			piece := &seg{start: at, pos: at, end: victim.end, owner: worker}
			victim.end = at
			t.segs = append(t.segs, piece)
			return assignment{seg: piece, cursor: at}, true
		}
	}
	if allowRace {
		if s := t.raceCandidate(now); s != nil {
			s.racer = worker
			return assignment{seg: s, cursor: s.pos, race: true}, true
		}
	}
	return assignment{}, false
}

func (t *table) raceCandidate(now time.Time) *seg {
	var peaks []float64
	for _, s := range t.segs {
		if s.peak > 0 {
			peaks = append(peaks, s.peak)
		}
	}
	if len(peaks) == 0 {
		return nil
	}
	sort.Float64s(peaks)
	typical := peaks[len(peaks)/2]
	var best *seg
	var bestEta float64
	for _, s := range t.segs {
		// a range needs some samples before its speed means anything
		if s.owner == 0 || s.racer != 0 || s.done() || s.end == unknownEnd || s.meter.since.IsZero() || now.Sub(s.meter.since) < t.warmup {
			continue
		}
		own := eta(s.remaining(), s.meter.rate)
		if own > t.raceSecs && own > 4*eta(s.remaining(), typical) && (best == nil || own > bestEta) {
			best, bestEta = s, own
		}
	}
	return best
}

func (t *table) setReqEnd(s *seg, worker int, reqEnd int64) {
	t.mu.Lock()
	if s.owner == worker {
		s.reqEnd = reqEnd
	}
	t.mu.Unlock()
}

func (t *table) endOf(s *seg) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return s.end
}

// race partners write the same bytes, only the one ahead progresses
func (t *table) advance(s *seg, worker int, cursor, n int64) (accept, progressed int64, stop bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if (s.owner != worker && s.racer != worker) || s.done() || cursor >= s.end {
		return 0, 0, true
	}
	accept = min(n, s.end-cursor)
	next := cursor + accept
	if next > s.pos {
		progressed = next - s.pos
		s.pos = next
		s.received += progressed
		t.downloaded.Add(progressed)
	}
	if s.done() {
		s.owner, s.racer = 0, 0
	}
	return accept, progressed, s.done() || next >= s.end
}

func (t *table) completeUnknown(s *seg) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s.end, s.owner = s.pos, 0
}

func (t *table) resetSingle() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	lost := t.downloaded.Swap(0)
	for _, s := range t.segs {
		s.pos = s.start
	}
	return lost
}

func (t *table) release(s *seg, worker int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch worker {
	case s.owner:
		s.owner, s.racer = s.racer, 0
		s.reqEnd = 0
	case s.racer:
		s.racer = 0
	}
}

func (t *table) tick(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range t.segs {
		if s.done() {
			continue
		}
		s.meter.update(s.received, now)
		s.peak = max(s.peak, s.meter.rate)
	}
}

type span struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
	Done  int64 `json:"done"`
}

func (t *table) spans() []span {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]span, 0, len(t.segs))
	for _, s := range t.segs {
		out = append(out, span{Start: s.start, End: s.end, Done: s.pos - s.start})
	}
	slices.SortFunc(out, func(a, b span) int { return cmp.Compare(a.Start, b.Start) })
	return out
}

// ported from prism-download-manager crates/prism-engine/src/resume.rs (MIT)
func partitions(spans []span, size int64) bool {
	if size <= 0 || len(spans) == 0 {
		return false
	}
	sorted := slices.Clone(spans)
	slices.SortFunc(sorted, func(a, b span) int { return cmp.Compare(a.Start, b.Start) })
	var expect int64
	for _, s := range sorted {
		if s.Start != expect || s.End <= s.Start || s.Done < 0 || s.Done > s.End-s.Start {
			return false
		}
		expect = s.End
	}
	return expect == size
}
