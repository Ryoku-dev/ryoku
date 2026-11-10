package state

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"

	"ryoku-rashin/internal/paths"
)

var ErrAppLockHeld = errors.New("Rashin is already open")

type AppLock struct {
	file *os.File
	once sync.Once
	err  error
}

func AcquireAppLock() (*AppLock, error) {
	if err := paths.EnsureDir(paths.RuntimeDir()); err != nil {
		return nil, fmt.Errorf("create Rashin runtime directory: %w", err)
	}
	if err := os.Chmod(paths.RuntimeDir(), 0o700); err != nil {
		return nil, fmt.Errorf("secure Rashin runtime directory: %w", err)
	}
	file, err := os.OpenFile(paths.AppLock(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open Rashin app lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrAppLockHeld
		}
		return nil, fmt.Errorf("lock Rashin app: %w", err)
	}
	return &AppLock{file: file}, nil
}

func (lock *AppLock) Close() error {
	if lock == nil {
		return nil
	}
	lock.once.Do(func() {
		if err := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN); err != nil {
			lock.err = err
		}
		if err := lock.file.Close(); lock.err == nil {
			lock.err = err
		}
	})
	return lock.err
}
