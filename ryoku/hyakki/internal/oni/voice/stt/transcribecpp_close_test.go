package stt

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestDictateCloseReapsActiveDecodersAndRemovesTheirSpeech(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Fatal(err)
	}
	d, scratch := newTestDictate(t, nil, specLimits(t))
	program := "#!/bin/sh\n[ -f \"$2\" ] || exit 17\nprintf '%s\\n' \"$$\" > \"${2}.pid\"\n: > \"${2}.ready\"\nexec " + strconv.Quote(sleep) + " 30\n"
	if err := os.WriteFile(d.bin, []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := d.Transcribe(ctx, speech(time.Second), Hint{})
			results <- err
		}()
	}
	t.Cleanup(func() { cancel(); workers.Wait() })

	var ready []string
	deadline := time.Now().Add(5 * time.Second)
	for {
		ready, err = filepath.Glob(filepath.Join(scratch, "hyakki-stt-*", "speech.wav.ready"))
		if err != nil {
			t.Fatal(err)
		}
		if len(ready) == 2 {
			break
		}
		select {
		case err := <-results:
			t.Fatalf("decoder ended before the close: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("the two real decoders did not start")
		}
		time.Sleep(time.Millisecond)
	}
	pids := make([]int, len(ready))
	for i, path := range ready {
		data, err := os.ReadFile(strings.TrimSuffix(path, ".ready") + ".pid")
		if err != nil {
			t.Fatal(err)
		}
		pids[i], err = strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || pids[i] <= 0 {
			t.Fatalf("invalid decoder pid %q: %v", data, err)
		}
	}

	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	for i, path := range ready {
		wav := strings.TrimSuffix(path, ".ready")
		if _, err := os.Stat(wav); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Close returned with private speech still on disk: %s (%v)", wav, err)
		}
		if err := syscall.Kill(pids[i], 0); !errors.Is(err, syscall.ESRCH) {
			t.Errorf("Close returned before decoder %d was reaped: %v", pids[i], err)
		}
	}
	cancel()
	workers.Wait()
	for range 2 {
		if err := <-results; !errors.Is(err, context.Canceled) {
			t.Errorf("interrupted decode returned %v, want context cancellation", err)
		}
	}
	if _, err := d.Transcribe(context.Background(), speech(time.Second), Hint{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("decode admitted after Close: %v", err)
	}
}
