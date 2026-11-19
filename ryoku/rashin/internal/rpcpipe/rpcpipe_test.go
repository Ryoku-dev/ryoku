package rpcpipe

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

func TestPipeRoundTripErrorsAndCancel(t *testing.T) {
	started := make(chan struct{})
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	cancelled := make(chan struct{})
	server := New(right, right, Options{Handle: func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		switch method {
		case "echo":
			var v struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(params, &v); err != nil {
				return nil, err
			}
			return v, nil
		case "fail":
			return nil, &Error{Code: CodeInvalidParams, Message: "bad input"}
		case "block":
			close(started)
			<-ctx.Done()
			close(cancelled)
			return nil, ctx.Err()
		default:
			return nil, &Error{Code: CodeMethodNotFound, Message: method}
		}
	}})
	defer server.Close()
	client := New(left, left, Options{})
	defer client.Close()
	var got struct {
		Text string `json:"text"`
	}
	if err := client.Call(context.Background(), "echo", map[string]string{"text": "value"}, &got); err != nil || got.Text != "value" {
		t.Fatalf("echo: %+v %v", got, err)
	}
	if err := client.Call(context.Background(), "fail", nil, nil); err == nil {
		t.Fatal("missing error")
	} else {
		var rpc *Error
		if !errors.As(err, &rpc) || rpc.Code != CodeInvalidParams {
			t.Fatalf("error: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	waiting := make(chan error, 1)
	go func() { waiting <- client.Call(ctx, "block", nil, nil) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler never started")
	}
	cancel()
	select {
	case err := <-waiting:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("call: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("call never cancelled")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("handler never cancelled")
	}
}
