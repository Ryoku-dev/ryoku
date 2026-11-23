package ytdlp

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// yt-dlp needs a JavaScript runtime for YouTube's player, tried in this order
var jsRuntimes = []string{"deno", "node", "bun"}

// jsProbeTimeout bounds the --help probe, and tests shrink it
var jsProbeTimeout = 20 * time.Second

// jsKey names one build of the binary, so an in-place upgrade is probed again
type jsKey struct {
	path string
	mod  int64
	size int64
}

var (
	jsMu    sync.Mutex
	jsCache = map[jsKey][]string{}
)

// jsRuntimeArgs asks each build of bin once, since an older yt-dlp stops on --js-runtimes.
// ported from prism-download-manager crates/prism-video/src/lib.rs and probe.rs (MIT)
func jsRuntimeArgs(ctx context.Context, bin string) []string {
	key := jsKey{path: bin}
	if st, err := os.Stat(bin); err == nil {
		key.mod, key.size = st.ModTime().UnixNano(), st.Size()
	}
	jsMu.Lock()
	defer jsMu.Unlock()
	if args, ok := jsCache[key]; ok {
		return args
	}
	known, done := knowsJSRuntimes(ctx, bin)
	if !known {
		if done {
			jsCache[key] = nil
		}
		return nil
	}
	var args []string
	for _, name := range jsRuntimes {
		if p, err := exec.LookPath(name); err == nil {
			args = []string{"--js-runtimes", name + ":" + p}
			break
		}
	}
	jsCache[key] = args
	return args
}

// knowsJSRuntimes also reports whether the probe finished, since a timed-out answer says nothing about the version.
func knowsJSRuntimes(ctx context.Context, bin string) (known, done bool) {
	ctx, cancel := context.WithTimeout(ctx, jsProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--ignore-config", "--help")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 3 * time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return false, false
	}
	if err != nil {
		return false, true
	}
	return bytes.Contains(out.Bytes(), []byte("--js-runtimes")), true
}
