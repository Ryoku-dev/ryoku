package exec

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	osexec "os/exec"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/rpcpipe"
)

func TestSandboxedStartCanceledBeforeReplyReleasesProcess(t *testing.T) {
	h := Host(HostOptions{})
	defer h.Close()
	started := make(chan string, 1)
	reply := make(chan struct{})
	client, server := net.Pipe()
	remote := rpcpipe.New(server, server, rpcpipe.Options{Handle: func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var req StartRequest
		var p ProcessParams
		switch method {
		case MethodStart:
			if err := json.Unmarshal(raw, &req); err != nil {
				return nil, err
			}
			id, err := h.Start(ctx, req, nil)
			if err != nil {
				return nil, err
			}
			started <- id
			<-reply
			return StartResult{ID: id}, nil
		case MethodKill, MethodRelease:
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			if method == MethodKill {
				return nil, h.Kill(ctx, p.ID)
			}
			return nil, h.Release(ctx, p.ID)
		}
		return nil, nil
	}})
	defer remote.Close()
	child := osexec.Command("sleep", "30")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	x := Sandboxed(func(context.Context) (*os.Process, io.ReadWriteCloser, error) {
		return child.Process, client, nil
	})
	defer x.Close()
	ctx, cancel := context.WithCancel(context.Background())
	dir := t.TempDir()
	result := make(chan error, 1)
	go func() {
		_, err := x.Start(ctx, StartRequest{Argv: []string{"sleep", "30"}, Dir: dir}, nil)
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("tool host never started a process")
	}
	cancel()
	close(reply)
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("start canceled with %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled start waited on abandoned output")
	}
	if ids := h.Running(); len(ids) != 0 {
		t.Fatalf("canceled start left tool host processes: %v", ids)
	}
}
