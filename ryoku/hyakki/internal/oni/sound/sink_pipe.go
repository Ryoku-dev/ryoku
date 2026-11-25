package sound

import (
	"errors"
	"os/exec"
	"strconv"
	"time"
)

// reapAfter bounds how long a player may keep running after its input
// closed before it is killed; a motif plus the player's own buffer is far
// shorter.
const reapAfter = 2 * time.Second

// players are tried in order. ALSA's aplay is never used: it can open the
// hardware directly, bypassing the session's routing.
var players = []struct {
	name string
	args func(rateHz int) []string
}{
	{"pw-cat", func(rateHz int) []string {
		return []string{
			"--playback", "--raw", "--format=f32", "--channels=1",
			"--rate=" + strconv.Itoa(rateHz), "--latency=" + strconv.Itoa(leadMs) + "ms",
			"--media-role=Notification", "-",
		}
	}},
	{"paplay", func(rateHz int) []string {
		return []string{
			"--raw", "--format=float32ne", "--channels=1",
			"--rate=" + strconv.Itoa(rateHz), "--latency-msec=" + strconv.Itoa(leadMs),
			"--property=media.role=event",
		}
	}},
}

type pipeSink struct {
	path string
	args func(rateHz int) []string
	ramp time.Duration
}

// PipeSink finds pw-cat, else paplay, on PATH. It starts nothing: each Play
// runs one player child process for that sound alone, fed PCM on stdin in
// real time, and the child exits when the sound ends.
func PipeSink() (Sink, error) {
	for _, p := range players {
		if path, err := exec.LookPath(p.name); err == nil {
			return &pipeSink{path: path, args: p.args, ramp: muteRamp()}, nil
		}
	}
	return nil, errors.New("sound: neither pw-cat nor paplay is installed")
}

func (p *pipeSink) Play(pcm []float32, rateHz int) (func(), error) {
	if len(pcm) == 0 || rateHz <= 0 {
		return func() {}, nil
	}
	cmd := exec.Command(p.path, p.args(rateHz)...)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	stop, cancel := stopper()
	go func() {
		_, _ = newStream(pcm, rateHz, p.ramp).run(in, rateHz, stop)
		_ = in.Close()
		exited := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(exited)
		}()
		select {
		case <-exited:
		case <-time.After(reapAfter):
			_ = cmd.Process.Kill()
			<-exited
		}
	}()
	return cancel, nil
}
