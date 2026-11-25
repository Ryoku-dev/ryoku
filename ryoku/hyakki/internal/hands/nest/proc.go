package nest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// proc is one process the desktop started, in a process group of its own,
// killed by the kernel if the daemon dies.
type proc struct {
	cmd    *exec.Cmd
	log    *tail
	exited chan struct{}
	err    error
}

// spawn starts path with exactly args in dir. A compositor's output is kept
// in a short tail for the error a failed start gives; a program's goes
// nowhere.
func spawn(path string, args, env []string, dir string, logged bool) (*proc, error) {
	p := &proc{exited: make(chan struct{})}
	p.cmd = &exec.Cmd{
		Path: path, Args: args, Env: env, Dir: dir,
		SysProcAttr: &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL},
		WaitDelay:   time.Second,
	}
	if logged {
		p.log = &tail{}
		p.cmd.Stdout, p.cmd.Stderr = p.log, p.log
	}
	if err := p.cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		p.err = p.cmd.Wait()
		close(p.exited)
	}()
	return p, nil
}

func (p *proc) ended() bool {
	select {
	case <-p.exited:
		return true
	default:
		return false
	}
}

// signal reaches the process's group while the process is still running.
func (p *proc) signal(sig syscall.Signal) {
	if !p.ended() {
		_ = syscall.Kill(-p.cmd.Process.Pid, sig)
	}
}

// exitError names the program that ended and why; call it once exited is
// closed.
func (p *proc) exitError() error {
	<-p.exited
	reason := "it ended"
	if p.err != nil {
		reason = p.err.Error()
	}
	last := ""
	if p.log != nil {
		last = p.log.last()
	}
	return fmt.Errorf("nest: %s exited: %s%s", p.cmd.Args[0], reason, last)
}

// waitAll waits up to limit for every process to exit and says whether they
// all did.
func waitAll(procs []*proc, limit time.Duration) bool {
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	for _, p := range procs {
		select {
		case <-p.exited:
		case <-deadline.C:
			return false
		}
	}
	return true
}

// sweep kills every process of this user whose environment holds dir as its
// runtime dir: what a desktop's programs started and left in another
// process group, or what a killed daemon left behind. It repeats until a
// pass finds none, so a process forking during the sweep is caught.
func sweep(dir string) {
	needle := []byte("XDG_RUNTIME_DIR=" + dir + "\x00")
	self, uid := os.Getpid(), os.Getuid()
	for range 10 {
		entries, err := os.ReadDir("/proc")
		if err != nil {
			return
		}
		found := 0
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil || pid == self {
				continue
			}
			info, err := os.Stat("/proc/" + entry.Name())
			if err != nil {
				continue
			}
			if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != uid {
				continue
			}
			environ, err := os.ReadFile("/proc/" + entry.Name() + "/environ")
			if err != nil || !(bytes.HasPrefix(environ, needle) || bytes.Contains(environ, append([]byte{0}, needle...))) {
				continue
			}
			if syscall.Kill(pid, syscall.SIGKILL) == nil {
				found++
			}
		}
		if found == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func isSocket(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode()&os.ModeSocket != 0
}

// tail keeps the last 2 KiB written to it.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

const tailSize = 2048

func (t *tail) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, b...)
	if over := len(t.buf) - tailSize; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(b), nil
}

// last is the last non-empty line written, as ": line", or "" when nothing
// was.
func (t *tail) last() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(t.buf)), "\n")
	line := strings.TrimSpace(lines[len(lines)-1])
	if line == "" {
		return ""
	}
	if len(line) > 200 {
		line = line[:200]
	}
	return ": " + line
}
