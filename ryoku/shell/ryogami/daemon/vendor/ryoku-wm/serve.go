package wm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type ServeRequest struct {
	ID   uint64   `json:"id"`
	Argv []string `json:"argv"`
}

// Err is what the one-shot verb would print on stderr, without the program prefix
type ServeReply struct {
	ID  uint64 `json:"id"`
	OK  bool   `json:"ok"`
	Out string `json:"out"`
	Err string `json:"err,omitempty"`
}

// watch never returns and apply writes config, so both keep their own process
func Served(verb string) bool { return verb == "act" || verb == "caps" }

const serveMaxLine = 1 << 20

// verbOut writes into the running request's buffer, so a verb's output never lands in another reply
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
		// flush before switching back so a verb that failed halfway still reports its output, like one-shot
		if ferr := verbOut.Flush(); err == nil {
			err = ferr
		}
		verbOut.Reset(out)
	}()
	return run(argv)
}
