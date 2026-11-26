package main

// Foreign bytes cross this boundary exactly once before any host image decoder
// or Qt sees them. The worker has neither a host filesystem nor credentials.
import (
	"bytes"
	"context"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const mediaOutputLimit = 32 << 20
const mediaFrameLimit = 96
const mediaPixelLimit int64 = 96_000_000
const mediaDeadline = 30 * time.Second

type inspectedMedia struct {
	Data      []byte `json:"data"`
	Error     string `json:"error,omitempty"`
	Poster    []byte `json:"poster"`
	Extension string `json:"extension"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Frames    int    `json:"frames"`    // image frames; the legacy video branch reports its one decoded poster, not video frame coverage
	Durations []int  `json:"durations"` // milliseconds, including zero-duration frames
}

// The init hook also works in the package test executable; tests therefore
// exercise the very same subprocess boundary rather than an in-process bypass.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "__card-media-worker" {
		if os.Getenv("RYOKU_CARD_MEDIA_WORKER") != "1" {
			os.Exit(2)
		}
		if err := runMediaWorker(os.Args[2:]); err != nil {
			_ = json.NewEncoder(os.Stdout).Encode(inspectedMedia{Error: err.Error()})
		}
		os.Exit(0)
	}
}

// runtimeELF binds individual resolved executables/libraries, never /usr, /etc,
// a host root, library directory, home, /run, or the host's loader cache.
func runtimeELF(path string, files map[string]string) error {
	if _, exists := files[path]; exists {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	st, err := os.Stat(resolved)
	if err != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("invalid media runtime file")
	}
	f, err := elf.Open(resolved)
	if err != nil {
		return fmt.Errorf("media runtime must be ELF: %s", filepath.Base(path))
	}
	defer f.Close()
	files[path] = resolved
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			b, e := io.ReadAll(io.LimitReader(p.Open(), 4096))
			if e != nil {
				return e
			}
			if e = runtimeELF(strings.TrimRight(string(b), "\x00"), files); e != nil {
				return e
			}
		}
	}
	libs, err := f.ImportedLibraries()
	if err != nil {
		return err
	}
	dirs := []string{}
	for _, tag := range []elf.DynTag{elf.DT_RPATH, elf.DT_RUNPATH} {
		entries, e := f.DynString(tag)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			for _, dir := range strings.Split(entry, ":") {
				dir = strings.ReplaceAll(strings.ReplaceAll(dir, "${ORIGIN}", filepath.Dir(resolved)), "$ORIGIN", filepath.Dir(resolved))
				if filepath.IsAbs(dir) {
					dirs = append(dirs, filepath.Clean(dir))
				}
			}
		}
	}
	dirs = append(dirs, filepath.Dir(resolved), "/usr/lib", "/lib", "/lib64", "/usr/lib64", "/lib/x86_64-linux-gnu", "/usr/lib/x86_64-linux-gnu")
	for _, name := range libs {
		if filepath.Base(name) != name {
			return fmt.Errorf("unsafe runtime library name")
		}
		found := false
		for _, dir := range dirs {
			candidate := filepath.Join(dir, name)
			if _, e := os.Stat(candidate); e == nil {
				if e = runtimeELF(candidate, files); e != nil {
					return e
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing media runtime library %s", name)
		}
	}
	return nil
}

func mediaSandboxArgs(executable string, tools []string) ([]string, error) {
	files := map[string]string{}
	if err := runtimeELF(executable, files); err != nil {
		return nil, err
	}
	args := []string{"--unshare-all", "--unshare-user", "--disable-userns", "--die-with-parent", "--new-session", "--cap-drop", "ALL", "--clearenv", "--setenv", "PATH", "/tools", "--setenv", "HOME", "/nonexistent", "--setenv", "LANG", "C", "--setenv", "GOMAXPROCS", "2", "--setenv", "GOMEMLIMIT", "384MiB", "--setenv", "RYOKU_CARD_MEDIA_WORKER", "1", "--size", "67108864", "--tmpfs", "/tmp", "--dir", "/dev", "--dev-bind", "/dev/null", "/dev/null", "--chdir", "/tmp"}
	toolPaths := map[string]string{}
	for _, name := range tools {
		path, err := exec.LookPath(name)
		if err != nil {
			return nil, fmt.Errorf("card media inspection requires %s; install the card media runtime", name)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
		if err = runtimeELF(path, files); err != nil {
			return nil, err
		}
		toolPaths[name] = path
	}
	// Bubblewrap creates absent parent directories for file bind destinations.
	for destination, source := range files {
		args = append(args, "--ro-bind", source, destination)
	}
	args = append(args, "--ro-bind", executable, "/worker")
	args = append(args, "--dir", "/tools")
	for name, source := range toolPaths {
		args = append(args, "--symlink", source, "/tools/"+name)
	}
	args = append(args, "--remount-ro", "/", "--", "/worker", "__card-media-worker")
	return args, nil
}

func inspectMedia(ctx context.Context, raw []byte, allowVideo bool) (inspectedMedia, error) {
	var out inspectedMedia
	kind, err := sniffPicture(raw)
	if err != nil {
		return out, err
	}
	if len(raw) > maxPictureBytes {
		return out, errPictureTooLarge
	}
	if kind.video && !allowVideo {
		return out, errors.New("video is not supported in image imports")
	}
	if err = mediaPreflight(raw, kind); err != nil {
		return out, err
	}
	tools := []string{}
	if kind.ext == "webp" {
		tools = []string{"dwebp", "cwebp"}
	}
	if kind.video || kind.ext == "bmp" || kind.ext == "avif" {
		tools = []string{"ffmpeg", "ffprobe"}
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return out, errors.New("card media inspection requires bubblewrap; refusing unsandboxed decoding")
	}
	executable, err := os.Executable()
	if err != nil {
		return out, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return out, err
	}
	args, err := mediaSandboxArgs(executable, tools)
	if err != nil {
		return out, err
	}
	if kind.video {
		args = append(args, "video")
	}
	ctx, cancel := context.WithTimeout(ctx, mediaDeadline)
	defer cancel()
	cmd := mediaCommand(ctx, bwrap, args)
	cmd.Stdin = bytes.NewReader(raw)
	result, err := readLimitedCommand(cmd, mediaOutputLimit*2)
	if ctx.Err() != nil {
		return out, fmt.Errorf("card media inspection timed out: %w", ctx.Err())
	}
	if err != nil {
		return out, fmt.Errorf("isolated card media inspection failed (corrupt media, resource limit or unavailable sandbox): %w: %s", err, cmd.Stderr)
	}
	if err = json.Unmarshal(result, &out); err != nil {
		return out, errors.New("invalid isolated media result")
	}
	if len(out.Error) > 4096 {
		return inspectedMedia{}, errors.New("invalid isolated media diagnostic")
	}
	if out.Error != "" {
		return inspectedMedia{}, fmt.Errorf("card media validation: %s", out.Error)
	}
	if len(out.Data)+len(out.Poster) > mediaOutputLimit || len(out.Data) == 0 || len(out.Poster) == 0 || !validPictureSize(out.Width, out.Height) || out.Frames < 1 || out.Frames > mediaFrameLimit || len(out.Durations) != out.Frames {
		return inspectedMedia{}, errors.New("isolated media result exceeds limits")
	}
	if err = validateMediaResult(out, kind.video && allowVideo); err != nil {
		return inspectedMedia{}, err
	}
	// This response comes only from this child process's pipe, not a sidecar or
	// manifest supplied by the uploader. No persistent claimed-clean bit exists.
	return out, nil
}

// Independently re-derive bounds/control facts from the child's returned bytes.
// A worker's metadata is never authority for dimensions, formats or budgets.
func validateMediaResult(out inspectedMedia, allowVideo bool) error {
	fail := func() error { return errors.New("isolated media output failed canonical structure checks") }
	if out.Frames < 1 || out.Frames > mediaFrameLimit || len(out.Durations) != out.Frames {
		return fail()
	}
	if len(out.Data) > maxPictureBytes || len(out.Poster) > maxPictureBytes {
		return errPictureTooLarge
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out.Poster))
	if err != nil || format != "png" || cfg.Width != out.Width || cfg.Height != out.Height || !validPictureSize(cfg.Width, cfg.Height) {
		return fail()
	}
	kind, err := sniffPicture(out.Data)
	if err != nil || kind.ext != out.Extension {
		return fail()
	}
	if kind.video {
		if !allowVideo || kind.ext != "webm" {
			return fail()
		}
		return nil
	}
	if kind.moving && len(out.Data) > maxMovingBytes {
		return movingTooLarge(int64(len(out.Data)))
	}
	if err = mediaPreflight(out.Data, kind); err != nil {
		return err
	}
	switch kind.ext {
	case "png", "jpg":
		cfg, _, err = image.DecodeConfig(bytes.NewReader(out.Data))
		if err != nil || cfg.Width != out.Width || cfg.Height != out.Height || out.Frames != 1 || out.Durations[0] != 0 {
			return fail()
		}
	case "gif":
		gif, err := inspectGIF(out.Data)
		if err != nil || gif.width != out.Width || gif.height != out.Height || len(gif.frames) != out.Frames {
			return fail()
		}
		for i, frame := range gif.frames {
			if frame.delay != out.Durations[i] {
				return fail()
			}
		}
	case "webp":
		info, err := parseWebP(out.Data)
		if err != nil || info.Width != out.Width || info.Height != out.Height || len(info.Frames) != out.Frames {
			return fail()
		}
		chunks, _ := webpChunks(out.Data[12:])
		index := 0
		for _, chunk := range chunks {
			if chunk.kind == "ANMF" {
				if out.Durations[index] != webp24(chunk.body[12:15]) {
					return fail()
				}
				index++
			}
		}
		if index == 0 && (out.Frames != 1 || out.Durations[0] != 0) {
			return fail()
		}
	default:
		return fail()
	}
	return nil
}

type mediaDiagnostic struct {
	data []byte
}

func (d *mediaDiagnostic) Write(p []byte) (int, error) {
	if remaining := 4096 - len(d.data); remaining > 0 {
		d.data = append(d.data, p[:min(len(p), remaining)]...)
	}
	return len(p), nil
}

func (d *mediaDiagnostic) String() string {
	return strings.TrimSpace(string(d.data))
}

// Shared by the real import boundary and executable containment fixtures.
func mediaCommand(ctx context.Context, bwrap string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bwrap, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	cmd.Stderr = &mediaDiagnostic{}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = time.Second
	return cmd
}

func runMediaWorker(args []string) error {
	// Linux rlimits are inherited by every decoder subprocess. No optional
	// systemd unit or user cgroup delegation is required.
	for resource, limit := range map[int]uint64{syscall.RLIMIT_CPU: 20, syscall.RLIMIT_FSIZE: mediaOutputLimit, syscall.RLIMIT_NOFILE: 64, syscall.RLIMIT_CORE: 0, syscall.RLIMIT_AS: 4 << 30, syscall.RLIMIT_DATA: 512 << 20, unix.RLIMIT_NPROC: 32} {
		if err := syscall.Setrlimit(resource, &syscall.Rlimit{Cur: limit, Max: limit}); err != nil {
			return err
		}
	}
	raw, err := readBounded(os.Stdin, maxPictureBytes)
	if err != nil {
		return err
	}
	video := len(args) == 1 && args[0] == "video"
	out, err := canonicalMedia(raw, video)
	if err != nil {
		return err
	}
	if len(out.Data)+len(out.Poster) > mediaOutputLimit {
		return errPictureTooLarge
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
