package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ryoku-rashin/internal/paths"
)

const maxLogSize = 1 << 20

type daemonLogger struct {
	mu sync.Mutex
	f  *os.File
}

func openDaemonLogger() (*daemonLogger, error) {
	if err := paths.EnsureDir(filepath.Dir(paths.DaemonLog())); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(paths.DaemonLog(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &daemonLogger{f: f}, nil
}

func (l *daemonLogger) Printf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	if stat, err := l.f.Stat(); err == nil && stat.Size() >= maxLogSize {
		l.f.Close()
		_ = os.Remove(paths.DaemonLog() + ".1")
		_ = os.Rename(paths.DaemonLog(), paths.DaemonLog()+".1")
		l.f, _ = os.OpenFile(paths.DaemonLog(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	}
	if l.f != nil {
		_, _ = fmt.Fprintf(l.f, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	}
}

func (l *daemonLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

func (d *Daemon) warn(format string, args ...any) {
	d.mu.Lock()
	if d.logger == nil {
		d.logger, _ = openDaemonLogger()
	}
	logger := d.logger
	d.mu.Unlock()
	if logger != nil {
		logger.Printf(format, args...)
	}
}
