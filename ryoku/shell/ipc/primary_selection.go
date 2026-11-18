package main

import (
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type primarySelectionState struct {
	mu      sync.Mutex
	watcher *clipWatcher
	changed time.Time
	changes chan struct{}
}

func (p *primarySelectionState) set(w *clipWatcher) {
	if w != nil {
		w.primaryChanged = p.notifyChange
	}
	p.mu.Lock()
	p.watcher = w
	p.mu.Unlock()
}
func (p *primarySelectionState) notifyChange() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.changed = time.Now()
	if p.changes != nil {
		select {
		case p.changes <- struct{}{}:
		default:
		}
	}
}
func (p *primarySelectionState) notifications() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.changes == nil {
		p.changes = make(chan struct{}, 1)
	}
	return p.changes
}
func (p *primarySelectionState) lastChange() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.changed
}
func (p *primarySelectionState) current() *clipWatcher {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.watcher
}

func primaryTextMime(offer *clipOffer) string {
	if offer == nil {
		return ""
	}
	fallback := ""
	for _, mime := range offer.types {
		if mime == passwordManagerHint {
			return ""
		}
		if mime == "text/plain;charset=utf-8" {
			fallback = mime
		}
	}
	if fallback != "" {
		return fallback
	}
	for _, mime := range offer.types {
		if mime == "text/plain" {
			return mime
		}
	}
	return ""
}

func (p *primarySelectionState) read() (text string, err error) {
	defer func() {
		if err != nil {
			log.Printf("ryoku-shell: sight primary selection: %v", err)
			if errors.Is(err, os.ErrDeadlineExceeded) {
				err = errors.New("timeout")
			} else {
				err = errors.New("no-selection")
			}
		}
	}()
	w := p.current()
	if w == nil {
		return "", errors.New("no-selection")
	}
	w.mu.Lock()
	mime := primaryTextMime(w.offers[w.primaryID])
	if mime == "" {
		w.mu.Unlock()
		return "", errors.New("no-selection")
	}
	fds := []int{-1, -1}
	if err := unix.Pipe2(fds, unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		w.mu.Unlock()
		return "", err
	}
	// The owner gets a blocking write end; only our bounded receiver is nonblocking.
	if err := unix.SetNonblock(fds[1], false); err != nil {
		w.mu.Unlock()
		unix.Close(fds[0])
		unix.Close(fds[1])
		return "", err
	}
	err = w.send(w.primaryID, dataOfferReceiveRequest, wlEncodeString(mime), fds[1])
	w.mu.Unlock()
	unix.Close(fds[1])
	defer unix.Close(fds[0])
	if err != nil {
		return "", err
	}
	data, err := readClipPipe(fds[0], clipMaxEntryBytes+1, clipTransferTimeout)
	if err != nil {
		return "", err
	}
	if len(data) == 0 || len(data) > clipMaxEntryBytes {
		return "", errors.New("no-selection")
	}
	return string(data), nil
}
