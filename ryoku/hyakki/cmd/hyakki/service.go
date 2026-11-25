package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ryoku-rashin/internal/childproc"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/paths"
)

type workerSpec struct {
	name    string
	program string
	args    []string
	initial time.Duration
	period  time.Duration
	once    bool
}

func init() {
	register(&Command{Name: "service", Group: "lifecycle", Usage: "routines|usage [--once]", Summary: "Run an optional worker or refresh usage once.", Run: runService})
}

func runService(args []string) int {
	once := len(args) == 2 && args[0] == "usage" && args[1] == "--once"
	if !once && (len(args) != 1 || args[0] != "routines" && args[0] != "usage") {
		return usageError("service routines|usage [--once]")
	}
	settings, err := config.Load()
	if err != nil {
		return report(err)
	}
	if !settings.Enabled {
		return 0
	}
	program, err := os.Executable()
	if err != nil {
		return report(err)
	}
	spec := workerSpec{name: args[0], program: program, args: []string{"routines", "wake"}, initial: time.Minute, period: 30 * time.Minute, once: once}
	if spec.name == "usage" {
		spec.program = filepath.Join(filepath.Dir(program), "hyakki-usage")
		if info, err := os.Stat(spec.program); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			spec.program, err = exec.LookPath("hyakki-usage")
			if err != nil {
				return report(err)
			}
		}
		spec.args, spec.initial, spec.period = nil, 45*time.Second, 5*time.Minute
		if once {
			spec.initial = 0
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := superviseWorker(ctx, spec); err != nil {
		if once && (errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)) {
			return 0
		}
		return report(err)
	}
	return 0
}

func superviseWorker(ctx context.Context, spec workerSpec) error {
	settings, err := config.Load()
	if err != nil || !settings.Enabled {
		return err
	}
	lockName := spec.name
	if spec.once {
		lockName += "-refresh"
	}
	lock, err := workerLock(lockName)
	if err != nil {
		return err
	}
	defer lock.Close()
	changes, closeWatch, err := workerSettingsWatch()
	if err != nil {
		return err
	}
	defer closeWatch()
	// a save between the first read and installing inotify must not be missed
	settings, err = config.Load()
	if err != nil || !settings.Enabled {
		return err
	}
	timer := time.NewTimer(spec.initial)
	defer timer.Stop()
	var command *exec.Cmd
	var exited <-chan struct{}
	var jobLock *os.File
	defer func() {
		if command != nil {
			childproc.Terminate(command, exited, 2*time.Second)
		}
		if jobLock != nil {
			jobLock.Close()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case watchErr, open := <-changes:
			if watchErr != nil {
				return watchErr
			}
			if !open {
				return errors.New("Hyakki settings watch stopped; restart the optional worker")
			}
			settings, err = config.Load()
			if err != nil || !settings.Enabled {
				return err
			}
		case <-timer.C:
			if ctx.Err() != nil {
				return nil
			}
			settings, err = config.Load()
			if err != nil || !settings.Enabled {
				return err
			}
			jobLock, err = workerLock(spec.name + "-job")
			if err != nil {
				if !spec.once && (errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)) {
					timer.Reset(spec.period)
					continue
				}
				return err
			}
			command = exec.Command(spec.program, spec.args...)
			command.SysProcAttr = childproc.Attr()
			command.Stdout, command.Stderr = os.Stdout, os.Stderr
			done := make(chan struct{})
			if err := childproc.LaunchLocked(command, done); err != nil {
				command = nil
				return err
			}
			exited = done
		case <-exited:
			jobLock.Close()
			jobLock = nil
			if spec.once {
				if !command.ProcessState.Success() {
					return fmt.Errorf("Hyakki %s collector failed: %w", spec.name, &exec.ExitError{ProcessState: command.ProcessState})
				}
				return nil
			}
			if !command.ProcessState.Success() {
				fmt.Fprintf(os.Stderr, "Hyakki %s worker: %s\n", spec.name, command.ProcessState.String())
			}
			command, exited = nil, nil
			timer.Reset(spec.period)
		}
	}
}

func workerLock(name string) (*os.File, error) {
	dir := paths.RuntimeDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 || !ok || owner.Uid != uint32(os.Getuid()) {
		return nil, fmt.Errorf("optional worker requires a private owned runtime directory: %s", dir)
	}
	path := filepath.Join(dir, "service-"+name+".lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("optional worker lock must be a regular file: %s", path)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("Hyakki %s worker is already running or its lock is unavailable: %w", name, err)
	}
	return file, nil
}

func workerSettingsWatch() (<-chan error, func(), error) {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, nil, err
	}
	mask := uint32(syscall.IN_CREATE | syscall.IN_MOVED_TO | syscall.IN_CLOSE_WRITE | syscall.IN_DELETE | syscall.IN_ATTRIB | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF)
	if _, err := syscall.InotifyAddWatch(fd, filepath.Dir(paths.ConfigFile()), mask); err != nil {
		syscall.Close(fd)
		return nil, nil, err
	}
	file := os.NewFile(uintptr(fd), "hyakki-settings-watch")
	changes := make(chan error, 1)
	done := make(chan struct{})
	name := []byte(filepath.Base(paths.ConfigFile()))
	go func() {
		defer close(done)
		defer close(changes)
		emit := func(err error) {
			select {
			case changes <- err:
			default:
			}
		}
		var buffer [16 * (syscall.SizeofInotifyEvent + syscall.NAME_MAX + 1)]byte
		for {
			n, err := file.Read(buffer[:])
			if err != nil {
				emit(err)
				return
			}
			for at := 0; at+syscall.SizeofInotifyEvent <= n; {
				eventMask := binary.NativeEndian.Uint32(buffer[at+4 : at+8])
				length := int(binary.NativeEndian.Uint32(buffer[at+12 : at+16]))
				end := at + syscall.SizeofInotifyEvent + length
				if end > n {
					emit(errors.New("incomplete Hyakki settings event; restart the optional worker"))
					return
				}
				if eventMask&(syscall.IN_Q_OVERFLOW|syscall.IN_IGNORED|syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF) != 0 {
					emit(errors.New("Hyakki settings directory changed or events were lost; restart the optional worker"))
					return
				}
				if bytes.Equal(bytes.TrimRight(buffer[at+syscall.SizeofInotifyEvent:end], "\x00"), name) {
					emit(nil)
				}
				at = end
			}
		}
	}()
	return changes, func() { file.Close(); <-done }, nil
}
