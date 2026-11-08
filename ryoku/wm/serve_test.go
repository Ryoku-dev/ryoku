package wm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// the daemon waits on every reply before it sends the next request
func TestServeAnswersEachRequestBeforeEOF(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	verbOut := bufio.NewWriter(io.Discard)
	done := make(chan error, 1)
	go func() {
		done <- Serve(inR, outW, verbOut, func(argv []string) error {
			fmt.Fprint(verbOut, strings.Join(argv[1:], " "))
			return nil
		})
		outW.Close()
	}()
	replies := bufio.NewScanner(outR)
	for i := 1; i <= 3; i++ {
		fmt.Fprintf(inW, `{"id":%d,"argv":["act","n%d"]}`+"\n", i, i)
		if !replies.Scan() {
			t.Fatalf("no reply to request %d", i)
		}
		var r ServeReply
		if err := json.Unmarshal(replies.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		if r.ID != uint64(i) || !r.OK || r.Out != fmt.Sprintf("n%d", i) {
			t.Fatalf("reply %d = %+v", i, r)
		}
	}
	inW.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve at EOF = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return at stdin EOF")
	}
}

func TestServeMapsFailuresPerRequest(t *testing.T) {
	var verbBuf bytes.Buffer
	verbOut := bufio.NewWriter(&verbBuf)
	in := strings.Join([]string{
		`not json`,
		`{"id":2,"argv":["watch"]}`,
		`{"id":3,"argv":["act","fail"]}`,
		`{"id":4,"argv":["act","panic"]}`,
		`{"id":5,"argv":[]}`,
		`{"id":6,"argv":["act","ok"]}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	err := Serve(strings.NewReader(in), &out, verbOut, func(argv []string) error {
		switch argv[1] {
		case "fail":
			fmt.Fprint(verbOut, "partial")
			return errors.New("act fail: no live session")
		case "panic":
			panic("boom")
		}
		fmt.Fprint(verbOut, "fine")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []ServeReply
	dec := json.NewDecoder(&out)
	for dec.More() {
		var r ServeReply
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if len(got) != 6 {
		t.Fatalf("got %d replies, want 6: %+v", len(got), got)
	}
	checks := []struct {
		ok      bool
		out     string
		errPart string
	}{
		{false, "", "bad request"},
		{false, "", `verb "watch" is not served`},
		{false, "partial", "act fail: no live session"},
		{false, "", "panic: boom"},
		{false, "", "missing verb"},
		{true, "fine", ""},
	}
	for i, c := range checks {
		r := got[i]
		if r.OK != c.ok || r.Out != c.out || !strings.Contains(r.Err, c.errPart) {
			t.Errorf("reply %d = %+v, want ok=%v out=%q err~%q", i, r, c.ok, c.out, c.errPart)
		}
	}
	if verbBuf.Len() != 0 {
		t.Errorf("verb output leaked to the verb writer's own target: %q", verbBuf.String())
	}
}
