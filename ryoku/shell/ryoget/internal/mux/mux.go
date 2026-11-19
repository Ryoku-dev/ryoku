// Package mux runs the plan's finishing step through ffmpeg: merge separate
// video and audio, remux, drop audio, extract or transcode audio. Streams are
// copied wherever the target container takes them.
package mux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ryoget/internal/plan"

	i18n "ryoku-i18n"
)

var ErrNoFFmpeg = errors.New("ffmpeg is not installed")

// muxers maps a file extension to the ffmpeg muxer that writes it.
var muxers = map[string]string{
	"mp4": "mp4", "m4a": "ipod", "m4v": "mp4", "mov": "mov",
	"webm": "webm", "mkv": "matroska", "mka": "matroska",
	"opus": "opus", "ogg": "ogg", "oga": "ogg", "mp3": "mp3", "flac": "flac",
	"ts": "mpegts", "aac": "adts",
}

// Finish turns inputs (one per plan part, same order) into out. The result is
// written beside out first and renamed into place, so out never exists half
// written.
func Finish(ctx context.Context, ffmpeg string, pl *plan.Plan, inputs []string, out string) error {
	if pl == nil || len(inputs) == 0 {
		return errors.New("mux: nothing to finish")
	}
	bin, err := lookFFmpeg(ffmpeg)
	if err != nil {
		return err
	}
	box := strings.ToLower(strings.TrimPrefix(pl.Container, "."))
	if box == "" {
		box = strings.ToLower(strings.TrimPrefix(filepath.Ext(out), "."))
	}
	muxer, ok := muxers[box]
	if !ok {
		muxer = "matroska"
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y"}
	title := strings.TrimSuffix(pl.Name, filepath.Ext(pl.Name))

	switch pl.Post {
	case plan.PostMerge:
		if len(inputs) < 2 {
			return errors.New("mux: merge needs two inputs")
		}
		v, a := roleIndex(pl, "video", 0), roleIndex(pl, "audio", 1)
		if v == a {
			a = 1 - v
		}
		args = append(args, "-i", inputs[v], "-i", inputs[a],
			"-map", "0:v:0", "-map", "1:a:0", "-c", "copy")
	case plan.PostRemux, plan.PostNone:
		args = append(args, "-i", inputs[0], "-map", "0:v?", "-map", "0:a?", "-c", "copy")
	case plan.PostMute:
		args = append(args, "-i", inputs[0], "-map", "0:v", "-c", "copy", "-an")
	case plan.PostAudio:
		args = append(args, "-i", inputs[audioInput(pl)], "-map", "0:a:0", "-vn", "-c:a", "copy")
	case plan.PostTranscode:
		enc, ok := encoders[box]
		if !ok {
			return fmt.Errorf("mux: no encoder for %s", box)
		}
		args = append(args, "-i", inputs[audioInput(pl)], "-map", "0:a:0", "-vn")
		args = append(args, enc...)
	default:
		return fmt.Errorf("mux: unknown step %q", pl.Post)
	}
	if muxer == "mp4" || muxer == "ipod" || muxer == "mov" {
		args = append(args, "-movflags", "+faststart")
	}
	if title != "" {
		args = append(args, "-metadata", "title="+title)
	}
	tmp := filepath.Join(filepath.Dir(out), "."+filepath.Base(out)+".ryoget-mux")
	args = append(args, "-f", muxer, tmp)
	defer os.Remove(tmp)
	if err := run(ctx, bin, args); err != nil {
		return err
	}
	return os.Rename(tmp, out)
}

var encoders = map[string][]string{
	"mp3":  {"-c:a", "libmp3lame", "-q:a", "2"},
	"opus": {"-c:a", "libopus", "-b:a", "160k"},
	"m4a":  {"-c:a", "aac", "-b:a", "192k"},
	"ogg":  {"-c:a", "libvorbis", "-q:a", "5"},
	"flac": {"-c:a", "flac"},
}

func roleIndex(pl *plan.Plan, role string, def int) int {
	for i, p := range pl.Parts {
		if p.Role == role {
			return i
		}
	}
	return def
}

func audioInput(pl *plan.Plan) int { return roleIndex(pl, "audio", 0) }

func lookFFmpeg(ffmpeg string) (string, error) {
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	bin, err := exec.LookPath(ffmpeg)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNoFFmpeg, i18n.T("Install FFmpeg to merge and convert media."))
	}
	return bin, nil
}

func run(ctx context.Context, bin string, args []string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return err
	}
	// muxing competes with the desktop for disk and CPU; it can wait
	syscall.Setpriority(syscall.PRIO_PROCESS, cmd.Process.Pid, 10)
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
			msg = msg[i+1:]
		}
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("ffmpeg: %s", msg)
	}
	return nil
}
