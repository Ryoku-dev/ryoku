package wm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// short so a slow action never queues the keybinds behind it
	warmSlotWait   = 50 * time.Millisecond
	warmMinBackoff = 250 * time.Millisecond
	warmMaxBackoff = 5 * time.Second
	// caps is a few KB, this only stops a runaway line
	warmMaxReply = 16 << 20
)

// worded like the one-shot failure so logs and anything matching the text read the same
var errProviderExit = errors.New("exit status 1")

// an idle desktop keeps only the watch process, a var so tests can shorten it
var warmIdleTimeout = 30 * time.Second

// opt-in, a CLI that asks once and exits would only orphan the child
func (c *Client) EnableServe() {
	if c.bin == "" {
		return
	}
	c.warm.CompareAndSwap(nil, &warmProvider{bin: c.bin, slot: make(chan struct{}, 1), idleAfter: warmIdleTimeout})
}

func (c *Client) Close() {
	if w := c.warm.Swap(nil); w != nil {
		w.close()
	}
}

func (c *Client) call(ctx context.Context, args ...string) ([]byte, error) {
	if w := c.warm.Load(); w != nil && Served(args[0]) {
		if out, served, err := w.do(ctx, args); served {
			return out, err
		}
	}
	return c.runContext(ctx, args...)
}

type warmProvider struct {
	bin string
	// the child answers in order, so one request at a time
	slot chan struct{}

	mu          sync.Mutex
	closed      bool
	child       *warmChild
	nextID      uint64
	backoff     time.Duration
	retryAt     time.Time
	unsupported binIdent
	// the idle timer reads busy and lastUsed to decide when to retire the child
	busy      bool
	lastUsed  time.Time
	idle      *time.Timer
	idleAfter time.Duration
}

// a package update swaps the file, so the child never outlives the code it started from
type binIdent struct {
	path       string
	dev, ino   uint64
	size, nsec int64
}

func identify(path string) (binIdent, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return binIdent{}, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return binIdent{}, false
	}
	return binIdent{path: path, dev: uint64(st.Dev), ino: st.Ino, size: fi.Size(), nsec: fi.ModTime().UnixNano()}, true
}

type warmChild struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	replies chan ServeReply // closed when the reply stream ends
	exited  chan struct{}   // closed once the process is reaped
	stderr  *headBuffer
	ident   binIdent
	served  bool
}

// served false means the caller runs it one-shot
func (w *warmProvider) do(ctx context.Context, args []string) (out []byte, served bool, err error) {
	if !w.acquire(ctx) {
		return nil, false, nil
	}
	defer func() { <-w.slot }()

	w.mu.Lock()
	ch := w.ready()
	if ch == nil {
		w.mu.Unlock()
		return nil, false, nil
	}
	w.nextID++
	id := w.nextID
	w.busy = true
	w.mu.Unlock()
	defer w.finished()

	line, err := json.Marshal(ServeRequest{ID: id, Argv: args})
	if err != nil {
		return nil, false, nil
	}
	if _, err := ch.stdin.Write(append(line, '\n')); err != nil {
		w.lose(ch)
		return nil, false, nil
	}
	select {
	case r, ok := <-ch.replies:
		if !ok || r.ID != id {
			w.lose(ch)
			return nil, false, nil
		}
		w.mu.Lock()
		ch.served = true
		w.backoff = 0
		w.mu.Unlock()
		if !r.OK {
			return nil, true, fmt.Errorf("%s %s: %w: %s: %s", w.bin, strings.Join(args, " "), errProviderExit, w.bin, r.Err)
		}
		return []byte(r.Out), true, nil
	case <-ctx.Done():
		w.lose(ch)
		return nil, false, nil
	}
}

func (w *warmProvider) acquire(ctx context.Context) bool {
	select {
	case w.slot <- struct{}{}:
		return true
	default:
	}
	t := time.NewTimer(warmSlotWait)
	defer t.Stop()
	select {
	case w.slot <- struct{}{}:
		return true
	case <-t.C:
	case <-ctx.Done():
	}
	return false
}

// called with mu held
func (w *warmProvider) ready() *warmChild {
	if w.closed {
		return nil
	}
	if ch := w.child; ch != nil {
		select {
		case <-ch.exited:
			w.forget(ch)
		default:
			if now, ok := identify(ch.ident.path); ok && now == ch.ident {
				return ch
			}
			// replaced or removed, stdin EOF ends the idle child
			_ = ch.stdin.Close()
			w.child = nil
		}
	}
	if time.Now().Before(w.retryAt) {
		return nil
	}
	path, err := exec.LookPath(w.bin)
	if err != nil {
		return nil
	}
	ident, ok := identify(path)
	if !ok || ident == w.unsupported {
		return nil
	}
	ch, err := startWarm(path, ident)
	if err != nil {
		w.scheduleRetry()
		return nil
	}
	w.child = ch
	return ch
}

func (w *warmProvider) finished() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.busy = false
	w.lastUsed = time.Now()
	if w.child == nil || w.closed {
		return
	}
	if w.idle == nil {
		w.idle = time.AfterFunc(w.idleAfter, w.retireIdle)
	} else {
		w.idle.Reset(w.idleAfter)
	}
}

// not a failure, the backoff stays and the next call starts a child at once
func (w *warmProvider) retireIdle() {
	w.mu.Lock()
	ch := w.child
	if ch == nil || w.busy || w.closed {
		w.mu.Unlock()
		return
	}
	if left := w.idleAfter - time.Since(w.lastUsed); left > 0 {
		w.idle.Reset(left)
		w.mu.Unlock()
		return
	}
	w.child = nil
	w.mu.Unlock()
	_ = ch.stdin.Close()
}

func (w *warmProvider) lose(ch *warmChild) {
	ch.kill()
	<-ch.exited
	w.mu.Lock()
	w.forget(ch)
	w.mu.Unlock()
}

// called with mu held
func (w *warmProvider) forget(ch *warmChild) {
	if w.child == ch {
		w.child = nil
	}
	// a provider older than serve refuses it, asking that binary again would fork twice per call
	if !ch.served && strings.Contains(ch.stderr.String(), `unknown verb "serve"`) {
		w.unsupported = ch.ident
	}
	w.scheduleRetry()
}

func (w *warmProvider) scheduleRetry() {
	w.backoff = min(max(w.backoff*2, warmMinBackoff), warmMaxBackoff)
	w.retryAt = time.Now().Add(w.backoff)
}

func (w *warmProvider) close() {
	w.mu.Lock()
	w.closed = true
	ch := w.child
	w.child = nil
	if w.idle != nil {
		w.idle.Stop()
	}
	w.mu.Unlock()
	if ch != nil {
		_ = ch.stdin.Close()
		ch.kill()
		<-ch.exited
	}
}

func startWarm(path string, ident binIdent) (*warmChild, error) {
	cmd := providerCommandContext(context.Background(), path, "serve")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	ch := &warmChild{
		cmd:     cmd,
		stdin:   stdin,
		replies: make(chan ServeReply, 1),
		exited:  make(chan struct{}),
		stderr:  &headBuffer{},
		ident:   ident,
	}
	cmd.Stderr = ch.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go ch.read(stdout)
	return ch, nil
}

func (ch *warmChild) read(stdout io.Reader) {
	scan := bufio.NewScanner(stdout)
	scan.Buffer(make([]byte, 0, 4096), warmMaxReply)
	broken := false
	for scan.Scan() {
		var r ServeReply
		if json.Unmarshal(scan.Bytes(), &r) != nil {
			broken = true
			break
		}
		ch.replies <- r
	}
	close(ch.replies)
	// a clean EOF is the child leaving on its own, so its group is left alone
	if broken || scan.Err() != nil {
		ch.kill()
	}
	_ = ch.cmd.Wait()
	close(ch.exited)
}

// the whole group, so a compositor client it waits on goes too, like a cancelled one-shot
func (ch *warmChild) kill() {
	if p := ch.cmd.Process; p != nil {
		_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
	}
}

// a provider older than serve names the verb it refused at the start of stderr
type headBuffer struct {
	mu  sync.Mutex
	buf []byte
}

const headBufferMax = 4096

func (b *headBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := headBufferMax - len(b.buf); room > 0 {
		b.buf = append(b.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

func (b *headBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}
