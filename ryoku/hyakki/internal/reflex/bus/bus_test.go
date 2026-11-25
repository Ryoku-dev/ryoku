package bus

import (
	"context"
	"errors"
	"testing"
)

type fake struct{ calls int }

func (f *fake) Call(context.Context, string, string, string, []any, ...any) error {
	f.calls++
	return nil
}
func (f *fake) Close() error { return nil }

func TestGuardChecksEveryCall(t *testing.T) {
	f := &fake{}
	allow := func(dest, path, method string, args []any) bool { return method == "a.B" && len(args) == 0 }
	conn, err := Guard(allow, func(context.Context) (Conn, error) { return f, nil })(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Call(context.Background(), "d", "/p", "a.B", nil); err != nil || f.calls != 1 {
		t.Fatalf("allowed call: %v", err)
	}
	if err := conn.Call(context.Background(), "d", "/p", "a.C", nil); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("other method: %v", err)
	}
	if err := conn.Call(context.Background(), "d", "/p", "a.B", []any{1}); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("other args: %v", err)
	}
	if f.calls != 1 {
		t.Fatal("a refused call reached the bus")
	}
	if _, err := Guard(allow, nil)(context.Background()); err == nil {
		t.Fatal("no dialer must be an error")
	}
}
