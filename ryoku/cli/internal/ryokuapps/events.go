package ryokuapps

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
)

// Step is one package in a plan event.
type Step struct {
	Manager string `json:"manager"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// events writes the NDJSON progress stream gpk defines (plan, step, log,
// progress, done), one object per line, for the verbs ryoku-host runs itself.
type events struct {
	mu       sync.Mutex
	w        io.Writer
	finished bool
}

func (e *events) emit(v any) {
	line, err := json.Marshal(v)
	if err != nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_, _ = e.w.Write(append(line, '\n'))
}

// raw forwards one line another producer already encoded.
func (e *events) raw(line []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, _ = e.w.Write(append(append([]byte(nil), line...), '\n'))
}

func (e *events) plan(action string, steps []Step) {
	if steps == nil {
		steps = []Step{}
	}
	e.emit(struct {
		Event  string `json:"event"`
		Action string `json:"action"`
		Steps  []Step `json:"steps"`
	}{"plan", action, steps})
}

func (e *events) stepStart(index int) {
	e.emit(struct {
		Event string `json:"event"`
		Index int    `json:"index"`
		Phase string `json:"phase"`
	}{"step", index, "start"})
}

func (e *events) stepDone(index, exit int) {
	e.emit(struct {
		Event string `json:"event"`
		Index int    `json:"index"`
		Phase string `json:"phase"`
		Exit  int    `json:"exit"`
	}{"step", index, "done", exit})
}

func (e *events) log(index int, line string) {
	e.emit(struct {
		Event string `json:"event"`
		Index int    `json:"index"`
		Line  string `json:"line"`
	}{"log", index, line})
}

// done ends the stream; only the first call writes.
func (e *events) done(ok bool, exit int, message string) {
	e.mu.Lock()
	if e.finished {
		e.mu.Unlock()
		return
	}
	e.finished = true
	e.mu.Unlock()
	e.emit(struct {
		Event string `json:"event"`
		OK    bool   `json:"ok"`
		Exit  int    `json:"exit"`
		Error string `json:"error"`
	}{"done", ok, exit, message})
}

// logWriter turns a child's output into log events, one per line. Carriage
// returns end a line too, so a progress bar redrawn in place becomes lines.
type logWriter struct {
	events  *events
	index   int
	pending []byte
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for {
		cut := bytes.IndexAny(w.pending, "\r\n")
		if cut < 0 {
			return len(p), nil
		}
		w.line(w.pending[:cut])
		w.pending = w.pending[cut+1:]
	}
}

// Flush emits a final line that had no newline.
func (w *logWriter) Flush() {
	w.line(w.pending)
	w.pending = nil
}

func (w *logWriter) line(b []byte) {
	if text := string(bytes.TrimRight(b, " \t")); text != "" {
		w.events.log(w.index, text)
	}
}
