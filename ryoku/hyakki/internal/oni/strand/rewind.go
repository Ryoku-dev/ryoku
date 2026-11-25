package strand

// rewindFrom is what was out when the rewind began.
type rewindFrom struct {
	u, curl, kRev, pen float64
	uncoil, recoil     float64 // perch: curl unrolls first, then the thread
	lassoS, reelS      float64
	dur                float64
}

// startRewind winds the thread home from wherever it is:
// u = from·(1 − ease(t/d)) for each part. A point thread takes
// rewind.durationS; a perch uncoils first then recoils; a ring rewinds over
// lassoS·lassoScale; a region mark reels in over reelS beside it.
func (e *Engine) startRewind() {
	if e.phase == PhaseSwing {
		// Rewind what is drawn: the blended thread becomes the path.
		shown := e.last.Strokes
		if len(shown) > 0 {
			e.path = resample(shown[0].Pts, e.s.Strand.BowedPath.Points)
			e.a0 = e.path[0]
		}
		e.k, e.u, e.from = kPoint, 1, nil
	}
	sr := e.s.Strand
	r := rewindFrom{u: e.u, curl: e.curl, kRev: e.kRev, pen: e.pen}
	switch e.k {
	case kPoint:
		r.recoil = sr.Rewind.DurationS
		r.dur = r.recoil
	case kPerch:
		if r.curl > 0 {
			r.uncoil = sr.Perch.UncoilS
		}
		r.recoil = sr.Perch.RecoilS
		r.dur = r.uncoil + r.recoil
	case kLasso, kRegion:
		if e.lasso && r.kRev > 0 {
			r.lassoS = sr.Rewind.LassoS * sr.Rewind.LassoScale
		}
		if e.k == kRegion && r.pen > 0 {
			r.reelS = sr.Region.ReelS
		}
		r.dur = max(r.lassoS, r.reelS)
	}
	e.rw = r
	e.tapV = 0
	e.burst = -1
	e.phase, e.el = PhaseRewind, 0
	if !(r.dur > 0) {
		e.home()
	}
}

// back is the rewind fraction left of a part rewinding over d after a delay.
func (e *Engine) back(from, delay, d float64) float64 {
	if !(d > 0) {
		return 0
	}
	return from * (1 - ease(e.s.Strand.Rewind.Ease, e.el-delay, d))
}

func (e *Engine) rewindProgress() {
	r := e.rw
	switch e.k {
	case kPoint:
		e.u = e.back(r.u, 0, r.recoil)
	case kPerch:
		if e.el < r.uncoil {
			e.curl = e.back(r.curl, 0, r.uncoil)
			return
		}
		e.curl = 0
		e.u = e.back(r.u, r.uncoil, r.recoil)
	case kLasso, kRegion:
		e.kRev = e.back(r.kRev, 0, r.lassoS)
		e.pen = e.back(r.pen, 0, r.reelS)
		if e.lasso && e.path != nil {
			e.u = clamp01(e.kRev / float64(e.ls.mainSegs))
		}
	}
}
