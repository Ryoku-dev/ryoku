package httpdl

import (
	"math/rand/v2"
	"time"
)

const (
	backoffBase = 500 * time.Millisecond
	backoffCap  = 30 * time.Second
)

// ported from prism-download-manager crates/prism-engine/src/backoff.rs (MIT)
func backoffDelay(attempt int, r float64) time.Duration {
	d := backoffCap
	if attempt < 16 {
		d = min(backoffCap, backoffBase<<attempt)
	}
	// equal jitter in [d/2, d): connections that failed together do not retry as one burst
	return d/2 + time.Duration(float64(d/2)*min(max(r, 0), 1))
}

func jitter() float64 { return rand.Float64() }
