// Package protocol is the wire contract between rashind and its clients: the
// framing, every method with its params and result types, every event, and the
// session event payloads. README.md in this directory is the reference.
package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Version is the protocol version. A client sends the version it speaks in
// hello; the daemon answers with its own.
const Version = 1

// MaxLine bounds one frame on the wire.
const MaxLine = 16 << 20

// Request asks the daemon to run a method. IDs are chosen by the client and
// unique per connection.
type Request struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response answers exactly one Request: Result on success, Error otherwise.
type Response struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// Event is pushed by the daemon without a request.
type Event struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Frame is any line on the wire. A frame with Method is a request, with Event
// an event, otherwise a response.
type Frame struct {
	ID     *int64          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
	Event  string          `json:"event,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// IsRequest reports whether the frame is a request.
func (f Frame) IsRequest() bool { return f.Method != "" }

// IsEvent reports whether the frame is an event.
func (f Frame) IsEvent() bool { return f.Event != "" }

// IsResponse reports whether the frame is a response.
func (f Frame) IsResponse() bool { return f.Method == "" && f.Event == "" && f.ID != nil }

// Error codes. CodeForbidden refuses a method the caller may not use, such as
// one only the Rashin app may call.
const (
	CodeBadRequest  = "bad_request"
	CodeNotFound    = "not_found"
	CodeUnavailable = "unavailable"
	CodeConflict    = "conflict"
	CodeDisabled    = "disabled"
	CodeUnsupported = "unsupported"
	CodeForbidden   = "forbidden"
	CodeInternal    = "internal"
)

// Error is a method failure. Message is a full sentence a user can read.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

// Errorf builds an Error with a formatted message.
func Errorf(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// AsError returns err as an *Error, wrapping anything else as internal.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var pe *Error
	if errors.As(err, &pe) {
		return pe
	}
	return &Error{Code: CodeInternal, Message: err.Error()}
}

// Encoder writes one compact JSON object per line. Safe for concurrent use.
type Encoder struct {
	mu sync.Mutex
	w  *bufio.Writer
}

// NewEncoder returns an Encoder writing to w.
func NewEncoder(w io.Writer) *Encoder { return &Encoder{w: bufio.NewWriter(w)} }

// Encode marshals v, writes it with a trailing newline and flushes.
func (e *Encoder) Encode(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) > MaxLine {
		return fmt.Errorf("frame of %d bytes exceeds the %d byte limit", len(b), MaxLine)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, err := e.w.Write(b); err != nil {
		return err
	}
	if err := e.w.WriteByte('\n'); err != nil {
		return err
	}
	return e.w.Flush()
}

// Decoder reads frames written by an Encoder.
type Decoder struct {
	s *bufio.Scanner
}

// NewDecoder returns a Decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64<<10), MaxLine+1)
	return &Decoder{s: s}
}

// Next returns the next frame. Blank lines are skipped. io.EOF ends the stream.
func (d *Decoder) Next() (Frame, error) {
	for d.s.Scan() {
		line := bytes.TrimSpace(d.s.Bytes())
		if len(line) == 0 {
			continue
		}
		var f Frame
		if err := json.Unmarshal(line, &f); err != nil {
			return Frame{}, fmt.Errorf("decode frame: %w", err)
		}
		return f, nil
	}
	if err := d.s.Err(); err != nil {
		return Frame{}, err
	}
	return Frame{}, io.EOF
}

// Marshal encodes v for a Params, Result or Data field. A nil v encodes as {}.
func Marshal(v any) (json.RawMessage, error) {
	if v == nil {
		return json.RawMessage(`{}`), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Unmarshal decodes a Params, Result or Data field into v. Empty input leaves v
// at its zero value.
func Unmarshal(raw json.RawMessage, v any) error {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return &Error{Code: CodeBadRequest, Message: err.Error()}
	}
	return nil
}
