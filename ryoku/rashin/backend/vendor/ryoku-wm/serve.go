package wm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// A provider's `serve` verb answers one-shot verbs over a pipe, so a daemon pays
// one process start for a session of keybinds instead of one per key. Only the
// verbs that read or act and then return are served: watch streams forever and
// apply authors config files, and both keep their own process.

// ServeRequest is one line on the provider's stdin.
type ServeRequest struct {
	ID   uint64   `json:"id"`
	Argv []string `json:"argv"`
}

// ServeReply answers exactly one request on the provider's stdout, in request
// order. Out is what the one-shot verb would have printed; Err is the error the
// one-shot verb would have reported on stderr, without the program prefix.
type ServeReply struct {
	ID  uint64 `json:"id"`
	OK  bool   `json:"ok"`
	Out string `json:"out"`
	Err string `json:"err,omitempty"`
}

// Served reports whether a warm provider answers verb.
func Served(verb string) bool { return verb == "act" || verb == "caps" }

// serveMaxLine bounds one request; argv is a handful of short tokens.
const serveMaxLine = 1 << 20

// Serve answers requests from in until EOF, one at a time and in order. verbOut
// is the buffered writer the provider's verbs print to: while run executes it
// points at a buffer of that request alone, so nothing a verb prints can land
// between two reply lines or in another request's answer. A panic in run
// answers that request with an error rather than ending the session.
func Serve(in io.Reader, out io.Writer, verbOut *bufio.Writer, run func(argv []string) error) error {
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 0, 4096), serveMaxLine)
	replies := bufio.NewWriter(out)
	enc := json.NewEncoder(replies)
	var buf bytes.Buffer
	for scan.Scan() {
		line := bytes.TrimSpace(scan.Bytes())
		if len(line) == 0 {
			continue
		}
		var req ServeRequest
		var reply ServeReply
		if err := json.Unmarshal(line, &req); err != nil {
			reply = ServeReply{ID: req.ID, Err: fmt.Sprintf("bad request: %v", err)}
		} else {
			buf.Reset()
			reply = ServeReply{ID: req.ID}
			err := serveOne(req.Argv, &buf, verbOut, out, run)
			reply.Out = buf.String()
			if err != nil {
				reply.Err = err.Error()
			} else {
				reply.OK = true
			}
		}
		if err := enc.Encode(reply); err != nil {
			return err
		}
		if err := replies.Flush(); err != nil {
			return err
		}
	}
	return scan.Err()
}

func serveOne(argv []string, buf *bytes.Buffer, verbOut *bufio.Writer, out io.Writer, run func([]string) error) (err error) {
	if len(argv) == 0 {
		return fmt.Errorf("missing verb")
	}
	if !Served(argv[0]) {
		return fmt.Errorf("verb %q is not served", argv[0])
	}
	verbOut.Reset(buf)
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
		// Flush before pointing back at out, so a verb that printed a partial
		// result and then failed still reports what it managed, as one-shot does.
		if ferr := verbOut.Flush(); err == nil {
			err = ferr
		}
		verbOut.Reset(out)
	}()
	return run(argv)
}
