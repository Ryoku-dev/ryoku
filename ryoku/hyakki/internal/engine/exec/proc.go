package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	osexec "os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

const (
	// defaultKeep is StartRequest.Keep's zero value.
	defaultKeep = 1 << 20
	// killGrace is how long a process group has between SIGTERM and SIGKILL.
	killGrace = 2 * time.Second
	// drainGrace lets the reader take what a program wrote just before it
	// exited; a background child still holding the pipe does not hold the
	// exit back longer.
	drainGrace = 200 * time.Millisecond
)

// process is one started program and its process group.
type process struct {
	id       string
	cmd      *osexec.Cmd
	read     *os.File
	started  time.Time
	done     chan struct{} // closed once the exit is recorded
	readDone chan struct{} // closed once nothing holds the output pipe open

	mu       sync.Mutex
	output   retained
	exited   bool
	code     int
	signal   string
	timedOut bool
	killed   bool
	ended    time.Time
	timer    *time.Timer
}

func (h *host) Start(ctx context.Context, req StartRequest, out OutputFunc) (string, error) {
	if len(req.Argv) == 0 || req.Argv[0] == "" {
		return "", &Error{Code: CodeNotFound, Message: "no command to run"}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	dir, err := absPath(req.Dir)
	if err != nil {
		return "", err
	}
	if err := isDir(dir); err != nil {
		return "", err
	}
	env := h.environ(req.Env)
	program, err := lookPath(req.Argv[0], env, dir)
	if err != nil {
		return "", err
	}
	keep := req.Keep
	if keep <= 0 {
		keep = defaultKeep
	}

	h.procMu.Lock()
	defer h.procMu.Unlock()
	running := 0
	for _, p := range h.procs {
		if !p.isDone() {
			running++
		}
	}
	if running >= h.opts.MaxProcesses {
		return "", &Error{Code: CodeUnavailable, Message: fmt.Sprintf("%d programs are already running; wait for one or stop it first", running)}
	}
	read, write, err := os.Pipe()
	if err != nil {
		return "", err
	}
	cmd := &osexec.Cmd{
		Path: program,
		Args: req.Argv,
		Dir:  dir,
		Env:  env,
		// one pipe for both keeps stdout and stderr in arrival order
		Stdout:      write,
		Stderr:      write,
		SysProcAttr: &syscall.SysProcAttr{Setpgid: true},
		// a stray child holding stdin must not hold the exit back
		WaitDelay: time.Second,
	}
	if req.Stdin != nil {
		cmd.Stdin = bytes.NewReader(req.Stdin)
	}
	if err := ctx.Err(); err != nil {
		_ = read.Close()
		_ = write.Close()
		return "", err
	}
	if err := cmd.Start(); err != nil {
		_ = read.Close()
		_ = write.Close()
		return "", startError(req.Argv[0], err)
	}
	if err := ctx.Err(); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
		_ = read.Close()
		_ = write.Close()
		return "", err
	}
	_ = write.Close()
	h.next++
	p := &process{
		id:       "p" + strconv.Itoa(h.next),
		cmd:      cmd,
		read:     read,
		started:  time.Now(),
		done:     make(chan struct{}),
		readDone: make(chan struct{}),
		output:   retained{head: keep - keep/4, tail: keep / 4},
	}
	h.procs[p.id] = p
	if req.Timeout > 0 {
		p.timer = time.AfterFunc(req.Timeout, func() {
			p.mu.Lock()
			if !p.exited {
				p.timedOut = true
			}
			p.mu.Unlock()
			p.stop()
		})
	}
	go p.stream(out)
	go p.wait()
	return p.id, nil
}

func (h *host) Wait(ctx context.Context, id string) (Exit, error) {
	p, err := h.process(id)
	if err != nil {
		return Exit{}, err
	}
	select {
	case <-p.done:
		return p.snapshot(), nil
	case <-ctx.Done():
		return Exit{}, ctx.Err()
	}
}

func (h *host) Peek(ctx context.Context, id string) (Exit, error) {
	p, err := h.process(id)
	if err != nil {
		return Exit{}, err
	}
	return p.snapshot(), nil
}

func (h *host) Kill(ctx context.Context, id string) error {
	p, err := h.process(id)
	if err != nil {
		return err
	}
	p.mu.Lock()
	if !p.exited {
		p.killed = true
	}
	p.mu.Unlock()
	p.stop()
	return nil
}

func (h *host) Release(ctx context.Context, id string) error {
	h.procMu.Lock()
	p := h.procs[id]
	delete(h.procs, id)
	h.procMu.Unlock()
	if p == nil {
		return &Error{Code: CodeNoProcess, Message: "no process " + id}
	}
	p.release()
	return nil
}

func (h *host) Running() []string {
	h.procMu.Lock()
	ids := slices.Collect(maps.Keys(h.procs))
	h.procMu.Unlock()
	slices.SortFunc(ids, func(a, b string) int {
		an, _ := strconv.Atoi(strings.TrimPrefix(a, "p"))
		bn, _ := strconv.Atoi(strings.TrimPrefix(b, "p"))
		return an - bn
	})
	return ids
}

// Close stops every process group (each its TERM grace, all at once) and
// forgets them; the executor stays usable.
func (h *host) Close() error {
	h.procMu.Lock()
	procs := slices.Collect(maps.Values(h.procs))
	clear(h.procs)
	h.procMu.Unlock()
	var wg sync.WaitGroup
	for _, p := range procs {
		wg.Go(func() {
			p.stop()
			p.release()
		})
	}
	wg.Wait()
	return nil
}

func (h *host) process(id string) (*process, error) {
	h.procMu.Lock()
	defer h.procMu.Unlock()
	p := h.procs[id]
	if p == nil {
		return nil, &Error{Code: CodeNoProcess, Message: "no process " + id}
	}
	return p, nil
}

func (h *host) environ(extra map[string]string) []string {
	base := h.opts.Env
	if base == nil {
		base = safeHostEnv(os.Environ())
	}
	if len(extra) == 0 {
		return base
	}
	env := slices.Clone(base)
	for _, key := range slices.Sorted(maps.Keys(extra)) {
		env = setEnv(env, key, extra[key])
	}
	return env
}

func safeHostEnv(base []string) []string {
	env := base[:0]
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok && !secretEnvKey(key) {
			env = append(env, entry)
		}
	}
	return env
}

func secretEnvKey(key string) bool {
	key = strings.ToUpper(key)
	return strings.HasPrefix(key, "HYAKKI_") || strings.HasPrefix(key, "RASHIN_") ||
		key == "KEY" || key == "TOKEN" || key == "SECRET" ||
		strings.HasSuffix(key, "_KEY") || strings.HasSuffix(key, "_TOKEN") ||
		strings.HasSuffix(key, "_SECRET") || strings.Contains(key, "PASSWORD") ||
		strings.Contains(key, "CREDENTIAL") || key == "SUDO_ASKPASS" ||
		key == "SSH_ASKPASS" || key == "GIT_ASKPASS" ||
		key == "SSH_AUTH_SOCK" || key == "GPG_AGENT_INFO" ||
		key == "AWS_ACCESS_KEY_ID" || key == "DATABASE_URL" ||
		key == "REDIS_URL" || key == "POSTGRES_URL" ||
		key == "MONGO_URI" || key == "MONGODB_URI"
}

func setEnv(env []string, name, value string) []string {
	prefix := name + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// lookPath finds name on the PATH of the environment the program gets, not
// this process's own.
func lookPath(name string, env []string, dir string) (string, error) {
	notFound := &Error{Code: CodeNotFound, Path: name, Message: name + ": command not found"}
	if strings.Contains(name, "/") {
		p := name
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if executable(p) {
			return p, nil
		}
		return "", notFound
	}
	pathVar := ""
	for _, item := range env {
		if v, ok := strings.CutPrefix(item, "PATH="); ok {
			pathVar = v
		}
	}
	for _, d := range filepath.SplitList(pathVar) {
		if d == "" || !filepath.IsAbs(d) {
			continue
		}
		if p := filepath.Join(d, name); executable(p) {
			return p, nil
		}
	}
	return "", notFound
}

func executable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0
}

func startError(name string, err error) error {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return &Error{Code: CodeDenied, Path: name, Message: name + ": permission denied"}
	case errors.Is(err, fs.ErrNotExist):
		return &Error{Code: CodeNotFound, Path: name, Message: name + ": command not found"}
	}
	return err
}

// stream reads the merged output until every writer closed it or the
// process is released, keeping it and passing it on in whole runes.
func (p *process) stream(out OutputFunc) {
	defer close(p.readDone)
	buf := make([]byte, 32<<10)
	var carry []byte
	for {
		n, err := p.read.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			p.mu.Lock()
			p.output.write(chunk)
			p.mu.Unlock()
			if out != nil {
				data := append(carry, chunk...)
				cut := completeRunes(data)
				if cut > 0 {
					out(Output{ID: p.id, Delta: strings.ToValidUTF8(string(data[:cut]), "\uFFFD")})
				}
				carry = append([]byte(nil), data[cut:]...)
			}
		}
		if err != nil {
			if out != nil && len(carry) > 0 {
				out(Output{ID: p.id, Delta: strings.ToValidUTF8(string(carry), "\uFFFD")})
			}
			return
		}
	}
}

func (p *process) wait() {
	_ = p.cmd.Wait()
	select {
	case <-p.readDone:
	case <-time.After(drainGrace):
	}
	p.mu.Lock()
	p.exited = true
	p.ended = time.Now()
	p.code = -1
	if state := p.cmd.ProcessState; state != nil {
		if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			p.signal = unix.SignalName(status.Signal())
			if p.signal == "" {
				p.signal = status.Signal().String()
			}
		} else {
			p.code = state.ExitCode()
		}
	}
	if p.timer != nil {
		p.timer.Stop()
	}
	p.mu.Unlock()
	close(p.done)
}

func (p *process) isDone() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// stop ends the process group: SIGTERM, SIGKILL after killGrace.
func (p *process) stop() {
	if !p.isDone() {
		p.signalGroup(syscall.SIGTERM)
		select {
		case <-p.done:
		case <-time.After(killGrace):
		}
	}
	p.killLeftovers()
	<-p.done
}

// release kills whatever of the group is left and closes the output pipe,
// which also ends a reader held open by a stray child.
func (p *process) release() {
	p.killLeftovers()
	<-p.done
	_ = p.read.Close()
}

func (p *process) killLeftovers() {
	// A redirected background child closes our pipe before its shell exits.
	// Pipe EOF is not evidence that the process group is empty.
	if p.cmd.Process != nil && syscall.Kill(-p.cmd.Process.Pid, 0) == nil {
		p.signalGroup(syscall.SIGKILL)
	}
}

func (p *process) signalGroup(sig syscall.Signal) {
	if p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, sig)
	}
}

func (p *process) snapshot() Exit {
	p.mu.Lock()
	defer p.mu.Unlock()
	exit := Exit{
		ID:        p.id,
		Running:   !p.exited,
		TimedOut:  p.timedOut,
		Killed:    p.killed,
		Output:    p.output.String(),
		Truncated: p.output.dropped,
	}
	if p.exited {
		exit.Code, exit.Signal = p.code, p.signal
		exit.Elapsed = p.ended.Sub(p.started)
	} else {
		exit.Elapsed = time.Since(p.started)
	}
	return exit
}

// retained keeps the head of a process's output and a rolling tail of its
// last bytes, dropping the middle once both are full.
type retained struct {
	head, tail int // capacities
	first      []byte
	last       []byte
	dropped    bool
}

func (r *retained) write(p []byte) {
	if room := r.head - len(r.first); room > 0 {
		n := min(room, len(p))
		r.first = append(r.first, p[:n]...)
		p = p[n:]
	}
	if len(p) == 0 {
		return
	}
	if r.tail <= 0 {
		r.dropped = true
		return
	}
	r.last = append(r.last, p...)
	if over := len(r.last) - r.tail; over > 0 {
		r.dropped = true
		r.last = append(r.last[:0], r.last[over:]...)
	}
}

func (r *retained) String() string {
	if !r.dropped {
		return strings.ToValidUTF8(string(r.first)+string(r.last), "\uFFFD")
	}
	first := r.first[:completeRunes(r.first)]
	skip := 0
	for skip < len(r.last) && skip < utf8.UTFMax-1 && !utf8.RuneStart(r.last[skip]) {
		skip++
	}
	return strings.ToValidUTF8(string(first)+string(r.last[skip:]), "\uFFFD")
}
