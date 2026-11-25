package sound

import (
	"bytes"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// FileRecord is one line FileSink appends per sound: the PCM exactly as a
// player would have been fed it, paced in real time, including the fade
// when the sound was cancelled.
type FileRecord struct {
	Start   time.Time `json:"start"`
	RateHz  int       `json:"rateHz"`
	Samples int       `json:"samples"`         // of the buffer handed to Play
	CutAt   int       `json:"cutAt"`           // play position of the cancel, -1 when none
	Written int       `json:"written"`         // samples fed
	PCM     []byte    `json:"pcm"`             // native-endian float32, base64 in JSON
	Error   string    `json:"error,omitempty"` // write failure, if any
}

type fileSink struct {
	path string
	ramp time.Duration
	mu   sync.Mutex
}

// FileSink appends a JSON line (FileRecord) per sound to path instead of
// playing it. It is for the isolated lab only.
func FileSink(path string) Sink { return &fileSink{path: path, ramp: muteRamp()} }

func (f *fileSink) Play(pcm []float32, rateHz int) (func(), error) {
	if len(pcm) == 0 || rateHz <= 0 {
		return func() {}, nil
	}
	out, err := os.OpenFile(f.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	stop, cancel := stopper()
	start := time.Now()
	go func() {
		defer out.Close()
		var buf bytes.Buffer
		cut, err := newStream(pcm, rateHz, f.ramp).run(&buf, rateHz, stop)
		rec := FileRecord{
			Start: start, RateHz: rateHz, Samples: len(pcm), CutAt: cut,
			Written: buf.Len() / 4, PCM: buf.Bytes(),
		}
		if err != nil {
			rec.Error = err.Error()
		}
		line, _ := json.Marshal(rec)
		f.mu.Lock()
		_, _ = out.Write(append(line, '\n'))
		f.mu.Unlock()
	}()
	return cancel, nil
}
