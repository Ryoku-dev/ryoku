package audio

import (
	"context"
	"encoding/binary"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// Capture opens the microphone for one press. Start begins recording from
// node ("" is the default source) and returns the PCM as it arrives, 16 kHz
// signed 16-bit mono, and a stop function. The channel closes when the
// recording ends: on stop, on ctx's end, or when the recorder exits. stop
// kills and reaps the recorder, waits for the channel to close, may be
// called more than once, and reports a recorder that failed on its own.
type Capture interface {
	Start(ctx context.Context, node string) (<-chan []int16, func() error, error)
}

// Recorder is the Capture over a recorder child process: pw-cat when it is
// installed, else parec. Nothing starts until Start; there is no warm mic.
type Recorder struct {
	stream Stream
	have   func(name string) bool
}

// NewRecorder returns the real Recorder: allowlisted children run with env
// (nil inherits), and the recorder is chosen by what is on PATH at Start.
func NewRecorder(env []string) *Recorder {
	return NewRecorderWith(ExecStream(env), onPath)
}

// NewRecorderWith returns a Recorder that starts children through stream
// and asks have whether a recorder is installed. Every argv still passes
// the allowlist before stream sees it.
func NewRecorderWith(stream Stream, have func(name string) bool) *Recorder {
	return &Recorder{stream: stream, have: have}
}

func onPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// recordArgv is the recorder command for node ("" is the default source).
func (r *Recorder) recordArgv(node string) (string, []string, error) {
	node = strings.TrimSpace(node)
	if node != "" && !validNode(node) {
		return "", nil, ErrBadNode
	}
	switch {
	case r.have("pw-cat"):
		args := append([]string{}, pwCatArgs...)
		if node != "" {
			args = append(args, "--target", node)
		}
		return "pw-cat", append(args, "-"), nil
	case r.have("parec"):
		args := append([]string{}, parecArgs...)
		if node != "" {
			args = append(args, "--device="+node)
		}
		return "parec", args, nil
	}
	return "", nil, ErrNoRecorder
}

// chunkBytes is the most one read hands on: 100 ms of capture.
const chunkBytes = SampleRateHz / 10 * 2

// Start implements Capture.
func (r *Recorder) Start(ctx context.Context, node string) (<-chan []int16, func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	name, args, err := r.recordArgv(node)
	if err != nil {
		return nil, nil, err
	}
	if !Allowed(name, args...) {
		return nil, nil, refuse(name, args)
	}
	rc, err := r.stream(ctx, name, args...)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	var (
		once sync.Once
		cerr error
	)
	end := func() { once.Do(func() { cerr = rc.Close() }) }
	out := make(chan []int16, 16)
	done := make(chan struct{})
	// The kill on ctx's end unblocks a read that is waiting on the child.
	go func() {
		<-ctx.Done()
		end()
	}()
	go func() {
		defer close(done)
		defer close(out)
		defer end()
		defer cancel()
		read(ctx, rc, out)
	}()
	stop := func() error {
		cancel()
		end()
		<-done
		return cerr
	}
	return out, stop, nil
}

// read turns rc's little-endian bytes into samples, carrying an odd byte
// over to the next read, until rc ends or ctx does.
func read(ctx context.Context, rc io.Reader, out chan<- []int16) {
	buf := make([]byte, chunkBytes+1)
	have := 0
	for {
		n, err := rc.Read(buf[have:])
		have += n
		if whole := have &^ 1; whole > 0 {
			pcm := make([]int16, whole/2)
			for i := range pcm {
				pcm[i] = int16(binary.LittleEndian.Uint16(buf[2*i:]))
			}
			copy(buf, buf[whole:have])
			have -= whole
			select {
			case out <- pcm:
			case <-ctx.Done():
				return
			}
		}
		if err != nil || ctx.Err() != nil {
			return
		}
	}
}
