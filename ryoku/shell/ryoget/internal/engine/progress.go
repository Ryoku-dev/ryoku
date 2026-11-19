package engine

import (
	"encoding/base64"
	"sync"

	"ryoget/internal/proto"
)

// maxUnits bounds what a fetcher may report through Mark; HLS playlists stay
// far below it.
const maxUnits = 1 << 20

// progress is what one run's fetchers report into; the engine samples it on
// its tick. Each part of a plan owns an equal slice of the 64 buckets.
type progress struct {
	mu    sync.Mutex
	parts []*partProgress
	stage string
}

type partProgress struct {
	p     *progress
	total int64
	done  int64
	// bytes that arrived during this run, the only ones that count as speed
	moved int64
	conns int
	units []float32
}

func newProgress() *progress { return &progress{} }

// setParts sizes the part list; hints are the sizes the plan announced (-1 unknown).
func (p *progress) setParts(hints []int64) []*partProgress {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.parts = make([]*partProgress, len(hints))
	for i, h := range hints {
		p.parts[i] = &partProgress{p: p, total: h}
	}
	return p.parts
}

func (p *progress) single() *partProgress { return p.setParts([]int64{-1})[0] }

func (p *progress) setStage(s string) {
	p.mu.Lock()
	p.stage = s
	p.mu.Unlock()
}

func (pp *partProgress) SetTotal(n int64) {
	pp.p.mu.Lock()
	pp.total = n
	pp.p.mu.Unlock()
}

func (pp *partProgress) Add(n int64) {
	pp.p.mu.Lock()
	pp.done += n
	pp.moved += n
	pp.p.mu.Unlock()
}

func (pp *partProgress) Resumed(n int64) {
	pp.p.mu.Lock()
	pp.done += n
	pp.p.mu.Unlock()
}

func (pp *partProgress) Stage(name string) { pp.p.setStage(name) }

func (pp *partProgress) Conns(n int) {
	pp.p.mu.Lock()
	pp.conns = n
	pp.p.mu.Unlock()
}

func (pp *partProgress) Mark(index, count int, fill float64) {
	if count <= 0 || count > maxUnits || index < 0 || index >= count {
		return
	}
	pp.p.mu.Lock()
	defer pp.p.mu.Unlock()
	if len(pp.units) != count {
		pp.units = make([]float32, count)
	}
	pp.units[index] = float32(min(1, max(0, fill)))
}

// complete marks a part finished in an earlier run, without counting speed.
func (pp *partProgress) complete(size int64) {
	pp.p.mu.Lock()
	defer pp.p.mu.Unlock()
	pp.total, pp.done = size, size
	pp.units = []float32{1}
}

type sample struct {
	total, done, moved int64
	conns              int
	stage              string
	pieces             [proto.PieceBuckets]byte
}

func (p *progress) sample() sample {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := sample{stage: p.stage}
	n := len(p.parts)
	for i, pp := range p.parts {
		if s.total >= 0 {
			if pp.total < 0 {
				s.total = -1
			} else {
				s.total += pp.total
			}
		}
		s.done += pp.done
		s.moved += pp.moved
		s.conns += pp.conns
		lo, hi := i*proto.PieceBuckets/n, (i+1)*proto.PieceBuckets/n
		pp.fill(s.pieces[lo:hi])
	}
	return s
}

func (pp *partProgress) fill(out []byte) {
	b := len(out)
	if b == 0 {
		return
	}
	if len(pp.units) == 0 {
		// nothing marked: a left-to-right fill from the byte count
		if pp.total <= 0 {
			return
		}
		f := float64(pp.done) / float64(pp.total) * float64(b)
		for k := range out {
			out[k] = toByte(f - float64(k))
		}
		return
	}
	c := len(pp.units)
	for k := range out {
		lo := k * c / b
		hi := max((k+1)*c/b, lo+1)
		var sum float32
		for _, u := range pp.units[lo:hi] {
			sum += u
		}
		out[k] = toByte(float64(sum) / float64(hi-lo))
	}
}

func toByte(f float64) byte {
	return byte(min(1, max(0, f))*255 + 0.5)
}

func encodePieces(b [proto.PieceBuckets]byte) string { return base64.StdEncoding.EncodeToString(b[:]) }

var (
	emptyPieces = encodePieces([proto.PieceBuckets]byte{})
	fullPieces  = func() string {
		var b [proto.PieceBuckets]byte
		for i := range b {
			b[i] = 255
		}
		return encodePieces(b)
	}()
)
