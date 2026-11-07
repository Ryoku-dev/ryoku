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

// A warm provider answers act and caps for a long-lived caller over one pipe,
// so a keybind costs a line write instead of a process start. It is opt-in: a
// CLI that asks one question and exits gains nothing from a child it would
// immediately orphan.

const (
	// warmSlotWait is how long a caller waits for the warm child to finish the
	// request ahead of it before taking today's one-shot path instead. Short,
	// so a slow action never queues the keybinds behind it.
	warmSlotWait   = 50 * time.Millisecond
	warmMinBackoff = 250 * time.Millisecond
	warmMaxBackoff = 5 * time.Second
	// warmMaxReply bounds one reply line; caps is a few kilobytes.
	warmMaxReply = 16 << 20
)

// errProviderExit words a served failure like the one-shot failure it replaces
// ("<bin> act x: exit status 1: <bin>: <err>"), so logs and any caller matching
// the text read the same either way.
var errProviderExit = errors.New("exit status 1")

// warmIdleTimeout ends a warm child that has had no request for this long, so
// an idle desktop holds only the watch process; a burst of actions still
// shares one child. A var so tests can shorten it.
var warmIdleTimeout = 30 * time.Second

// EnableServe makes act and caps go through one warm provider process started
// on first use and stopped after warmIdleTimeout without a request; the next
// call starts it again. A dead child is respawned on a later call, with
// backoff; until then, and whenever the child is busy or the provider predates
// serve, calls take the one-shot path. Safe to call more than once and
// concurrently with other calls.
func (c *Client) EnableServe() {
	if c.bin == "" {
		return
	}
	c.warm.CompareAndSwap(nil, &warmProvider{bin: c.bin, slot: make(chan struct{}, 1), idleAfter: warmIdleTimeout})
}

// Close stops the warm provider, if any. Later calls take the one-shot path.
func (c *Client) Close() {
	if w := c.warm.Swap(nil); w != nil {
		w.close()
	}
}

// call runs one provider verb, through the warm child when it can answer.
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
	// slot is held for the length of one request: the child answers in order,
	// so a second request may not start until the first is answered.
	slot chan struct{}

	mu          sync.Mutex
	closed      bool
	child       *warmChild
	nextID      uint64
	backoff     time.Duration
	retryAt     time.Time
	unsupported binIdent
	// busy is set while a request is with the child; lastUsed is when the last
	// one finished. The idle timer reads both to decide whether to retire it.
	busy      bool
	lastUsed  time.Time
	idle      *time.Timer
	idleAfter time.Duration
}

// binIdent tells a binary a package update replaced from the one running, so
// the warm child never outlives the code it was started from.
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

// do answers served == false when the warm path could not run the request at
// all, and the caller then runs it one-shot. A request whose ctx ends first
// kills the child, so the fallback reports the same cancellation a one-shot
// call would.
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

// ready returns a live child running the installed binary, starting one when
// none runs and backoff allows. Called with mu held.
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
			// Replaced or removed: stdin EOF ends the idle child on its own.
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

// finished marks the request done and restarts the idle countdown.
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

// retireIdle closes an idle child's stdin so it leaves at EOF. It is not a
// failure: the backoff is untouched, and the next call starts a child at once.
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

// lose kills a child that stopped answering in order and waits for it.
func (w *warmProvider) lose(ch *warmChild) {
	ch.kill()
	<-ch.exited
	w.mu.Lock()
	w.forget(ch)
	w.mu.Unlock()
}

// forget drops a dead child and schedules the next start. Called with mu held.
func (w *warmProvider) forget(ch *warmChild) {
	if w.child == ch {
		w.child = nil
	}
	// A provider older than serve refuses the verb before answering anything;
	// asking that same binary again would only fork twice per call.
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
	// A broken reply stream leaves nothing to talk to. A clean EOF is the
	// child leaving on its own, so its process group is left alone.
	if broken || scan.Err() != nil {
		ch.kill()
	}
	_ = ch.cmd.Wait()
	close(ch.exited)
}

// kill takes the child's process group, so a compositor client it is waiting
// on goes with it, as a cancelled one-shot call does.
func (ch *warmChild) kill() {
	if p := ch.cmd.Process; p != nil {
		_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
	}
}

// headBuffer keeps the start of the child's stderr, where a provider that
// predates serve names the verb it refused.
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
