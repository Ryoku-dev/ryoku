package toolhost_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/rpcpipe"
	"ryoku-rashin/internal/toolhost"
)

func TestToolHostChild(t *testing.T) {
	if os.Getenv("RASHIN_TEST_TOOLHOST") != "1" {
		return
	}
	if err := toolhost.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxedClientServer(t *testing.T) {
	t.Setenv("RASHIN_TEST_TOOLHOST", "1")
	started := 0
	e := exec.Sandboxed(func(ctx context.Context) (*os.Process, io.ReadWriteCloser, error) {
		started++
		return exec.StartCommand(ctx, []string{os.Args[0], "-test.run=^TestToolHostChild$"})
	})
	defer e.Close()
	ctx := context.Background()
	root := t.TempDir()
	file := filepath.Join(root, "example.txt")
	if _, err := e.Stat(ctx, file); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing path: %v", err)
	}
	if started != 1 {
		t.Fatalf("lazy start: %d", started)
	}
	created, err := e.Write(ctx, exec.WriteRequest{Path: file, Content: []byte("one\ntwo\n")})
	if err != nil || created.Existed {
		t.Fatalf("write: %+v %v", created, err)
	}
	read, err := e.Read(ctx, exec.ReadRequest{Path: file})
	if err != nil || string(read.Data) != "one\ntwo\n" {
		t.Fatalf("read: %+v %v", read, err)
	}
	changed, err := e.Edit(ctx, exec.EditRequest{Path: file, IfVersion: read.Info.Version, Replacements: []exec.Replacement{{Old: "two", New: "three"}}})
	if err != nil || string(changed.New) != "one\nthree\n" {
		t.Fatalf("edit: %+v %v", changed, err)
	}
	found, err := e.Grep(ctx, exec.GrepRequest{Root: root, Pattern: "three", Literal: true})
	if err != nil || found.Files != 1 {
		t.Fatalf("grep: %+v %v", found, err)
	}
	var output strings.Builder
	id, err := e.Start(ctx, exec.StartRequest{Argv: []string{"/bin/sh", "-c", "printf 'hello\\n'"}, Dir: root}, func(o exec.Output) { output.WriteString(o.Delta) })
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	state, err := e.Wait(waitCtx, id)
	if err != nil || state.Code != 0 || state.Output != "hello\n" || output.String() != state.Output {
		t.Fatalf("wait: %+v %v output=%q", state, err, output.String())
	}
	if err := e.Release(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := e.Remove(ctx, exec.RemoveRequest{Path: file, IfVersion: changed.Info.Version}); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if e.(interface{ Alive() bool }).Alive() {
		t.Fatal("tool host survived Close")
	}
	if _, err := e.Stat(ctx, root); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if started != 2 {
		t.Fatalf("restart count %d", started)
	}
}

func TestServerOverInProcessPipe(t *testing.T) {
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- toolhost.Serve(context.Background(), right, right) }()
	client := rpcpipe.New(left, left, rpcpipe.Options{})
	defer client.Close()
	defer left.Close()
	defer right.Close()
	p := filepath.Join(t.TempDir(), "note.txt")
	var created exec.WriteResult
	if err := client.Call(context.Background(), exec.MethodWrite, exec.WriteRequest{Path: p, Content: []byte("in-process")}, &created); err != nil || created.Existed {
		t.Fatalf("write over pipe: %+v %v", created, err)
	}
	var read exec.ReadResult
	if err := client.Call(context.Background(), exec.MethodRead, exec.ReadRequest{Path: p}, &read); err != nil || string(read.Data) != "in-process" {
		t.Fatalf("read over pipe: %+v %v", read, err)
	}
	var missing json.RawMessage
	if err := client.Call(context.Background(), exec.MethodRead, exec.ReadRequest{Path: p + "-missing"}, &missing); err == nil {
		t.Fatal("missing error over pipe")
	} else {
		var rpc *rpcpipe.Error
		var typed exec.Error
		if !errors.As(err, &rpc) || rpc.Code != exec.WireErrorCode || json.Unmarshal(rpc.Data, &typed) != nil || typed.Code != exec.CodeNotFound {
			t.Fatalf("lost executor error over pipe: %v", err)
		}
	}
	_ = client.Close()
	_ = left.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop when the input closed")
	}
	_ = right.Close()
}
