package window

import (
	"bufio"
	"context"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// readLines hands every line of rc to fn until rc ends, fn fails or ctx is
// done. It returns nil when ctx ended the read, so a cancelled Watch is not an
// error, and io.ErrUnexpectedEOF when the compositor closed the stream.
func readLines(ctx context.Context, rc io.ReadCloser, fn func([]byte) error) error {
	stop := context.AfterFunc(ctx, func() { _ = rc.Close() })
	defer func() {
		if stop() {
			_ = rc.Close()
		}
	}()
	scan := bufio.NewScanner(rc)
	scan.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for scan.Scan() {
		line := scan.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		if err := fn(line); err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := scan.Err(); err != nil {
		return err
	}
	return io.ErrUnexpectedEOF
}

// emitter suppresses a snapshot equal to the last one sent, so a burst of
// compositor events that changes nothing Oni reads costs no frame.
type emitter struct {
	fn   func(Snapshot)
	last *Snapshot
}

func (e *emitter) emit(s Snapshot) {
	if e.last != nil && reflect.DeepEqual(*e.last, s) {
		return
	}
	e.last = &s
	e.fn(s)
}

// outputByName finds an output, so global geometry can become output-local.
func outputByName(outputs []Output, name string) (Output, bool) {
	for _, o := range outputs {
		if o.Name == name {
			return o, true
		}
	}
	return Output{}, false
}

// outputAt is the output whose rectangle holds the global point.
func outputAt(outputs []Output, x, y float64) (Output, bool) {
	for _, o := range outputs {
		if x >= float64(o.X) && x < float64(o.X+o.W) && y >= float64(o.Y) && y < float64(o.Y+o.H) {
			return o, true
		}
	}
	return Output{}, false
}

// sortWindows gives every provider the same deterministic order: focus
// history first, then id (numerically when both ids are numbers).
func sortWindows(ws []Window) {
	slices.SortStableFunc(ws, func(a, b Window) int {
		if a.FocusOrder != b.FocusOrder {
			return a.FocusOrder - b.FocusOrder
		}
		return compareID(a.ID, b.ID)
	})
}

func compareID(a, b ID) int {
	na, ea := strconv.ParseInt(string(a), 10, 64)
	nb, eb := strconv.ParseInt(string(b), 10, 64)
	if ea == nil && eb == nil {
		switch {
		case na < nb:
			return -1
		case na > nb:
			return 1
		}
		return 0
	}
	return strings.Compare(string(a), string(b))
}
