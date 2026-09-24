package main

// daemonclient.go: the Hub's one window onto the shell daemon's control socket.
// Anything the daemon owns (clipboard history, the live power profile, and
// whatever else moves behind the socket) must be read and written THROUGH the
// daemon, never by a second writer beside it: the daemon banks the user's
// profile pick, restores it across reboots, and re-applies the Ryoku CPU
// definition after power-profiles-daemon settles, and a raw
// `powerprofilesctl set` from the Hub would skip all of that.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// daemonCall runs one control method on the shell daemon and decodes its result
// into out (nil to discard it). The reply is one JSON line: {ok,result,error}.
func daemonCall(method string, args any, out any) error {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	conn, err := net.DialTimeout("unix", filepath.Join(base, "ryoku-shell.sock"), 3*time.Second)
	if err != nil {
		return fmt.Errorf("the shell daemon is not answering; is the Ryoku session running?")
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	line := "call " + method
	if args != nil {
		b, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("%s: %w", method, err)
		}
		line += " " + string(b)
	}
	if _, err := io.WriteString(conn, line+"\n"); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	return decodeCallReply(method, reply, out)
}

// decodeCallReply turns one reply line into a result or an error. Split out so
// the wire contract is testable without a live daemon.
func decodeCallReply(method, line string, out any) error {
	var reply struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &reply); err != nil {
		return fmt.Errorf("%s: unreadable daemon reply", method)
	}
	if !reply.OK {
		if reply.Error == "" {
			reply.Error = "the daemon refused the request"
		}
		return fmt.Errorf("%s: %s", method, reply.Error)
	}
	if out != nil && len(reply.Result) > 0 {
		if err := json.Unmarshal(reply.Result, out); err != nil {
			return fmt.Errorf("%s: unreadable daemon result", method)
		}
	}
	return nil
}
