package ytdlp

import (
	"bytes"
	"context"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// yt-dlp needs a JavaScript runtime for YouTube's player, tried in this order
var jsRuntimes = []string{"deno", "node", "bun"}

var (
	jsMu    sync.Mutex
	jsCache = map[string][]string{}
)

// jsRuntimeArgs asks bin once per run, since an older yt-dlp stops on --js-runtimes.
// ported from prism-download-manager crates/prism-video/src/lib.rs and probe.rs (MIT)
func jsRuntimeArgs(ctx context.Context, bin string) []string {
	jsMu.Lock()
	defer jsMu.Unlock()
	if args, ok := jsCache[bin]; ok {
		return args
	}
	if !knowsJSRuntimes(ctx, bin) {
		if ctx.Err() == nil {
			jsCache[bin] = nil
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
	jsCache[bin] = args
	return args
}

func knowsJSRuntimes(ctx context.Context, bin string) bool {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--ignore-config", "--help")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 3 * time.Second
	if cmd.Run() != nil {
		return false
	}
	return bytes.Contains(out.Bytes(), []byte("--js-runtimes"))
}
