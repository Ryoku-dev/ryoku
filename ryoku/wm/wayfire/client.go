package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

// request is the single choke point for every compositor call, so the
// isolation gate has one line to allow. A var so tests can pin the emitted
// request without a live compositor.
var request = sendRequest

// ipcRequest is wayfire's request envelope: the method name and its data,
// framed as four bytes little-endian of payload length before the JSON.
type ipcRequest struct {
	Method string `json:"method"`
	Data   any    `json:"data"`
}

// sendRequest dials the ipc socket, sends one method call and reads one
// reply. A reply carrying a non-empty "error" string is wayfire refusing the
// call; anything else is the method's own result, passed through verbatim so
// a protocol change reads as itself.
func sendRequest(method string, data any) (json.RawMessage, error) {
	body, err := json.Marshal(ipcRequest{Method: method, Data: data})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	path := socketPath()
	if path == "" {
		return nil, fmt.Errorf("wayfire %s: no socket", method)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("wayfire %s: %w", method, err)
	}
	defer conn.Close()

	if err := writeFrame(conn, body); err != nil {
		return nil, fmt.Errorf("wayfire %s: %w", method, err)
	}
	reply, err := readFrame(conn)
	if err != nil {
		return nil, fmt.Errorf("wayfire %s: %w", method, err)
	}
	// Only an object reply can carry an error envelope: the list methods
	// answer with a bare array, which must pass through untouched.
	trimmed := bytes.TrimSpace(reply)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(reply, &envelope); err != nil {
			return nil, fmt.Errorf("wayfire %s: bad reply %q", method, string(reply))
		}
		if raw, ok := envelope["error"]; ok {
			var msg string
			if json.Unmarshal(raw, &msg) == nil && msg != "" {
				return nil, fmt.Errorf("wayfire %s: %s", method, msg)
			}
		}
	}
	return reply, nil
}

func writeFrame(w io.Writer, payload []byte) error {
	var head [4]byte
	binary.LittleEndian.PutUint32(head[:], uint32(len(payload)))
	if _, err := w.Write(head[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readFrame(r io.Reader) ([]byte, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, err
	}
	body := make([]byte, binary.LittleEndian.Uint32(head[:]))
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}
