package sound

import (
	"sync"
	"time"
)

// Sink plays mono float32 PCM. Play returns at once; cancel fades the sound
// out within the spec's mute ramp and may be called any number of times,
// also after the sound ended.
type Sink interface {
	Play(pcm []float32, rateHz int) (cancel func(), err error)
}

// muteRamp is how long a cancelled sound takes to reach silence: the
// shorter of the gate's quiet bound and the setting's mute ramp.
func muteRamp() time.Duration {
	g := builtin().Sound.Gating
	return millis(min(g.QuietMs, g.MuteRampMs))
}

// stopper returns a stop channel and an idempotent cancel closing it.
func stopper() (<-chan struct{}, func()) {
	ch := make(chan struct{})
	var once sync.Once
	return ch, func() { once.Do(func() { close(ch) }) }
}
