package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"
)

type Progress func(string)

type boundedOutput struct {
	data []byte
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if room := (1 << 20) - len(b.data); room > 0 {
		if len(p) > room {
			p = p[:room]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

func run(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if name == "yt-dlp" {
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			switch strings.ToLower(key) {
			case "no_proxy", "http_proxy", "https_proxy", "all_proxy", "ftp_proxy":
				continue
			}
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr boundedOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		detail := strings.TrimSpace(string(stderr.data))
		if len(detail) > 2000 {
			detail = detail[len(detail)-2000:]
		}
		if detail != "" {
			return "", fmt.Errorf("%s: %w: %s", name, err, detail)
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return strings.TrimSpace(string(stdout.data)), nil
}

func engines() error {
	if _, err := exec.LookPath("ryoku-dictate"); err != nil {
		return fmt.Errorf("%s", i18n.T("The voice engine ryoku-dictate is missing. Open Settings > Voice typing and download a voice model."))
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("%s", i18n.T("Install ffmpeg to read recordings."))
	}
	return nil
}

func transcribe(ctx context.Context, wav string) (string, error) {
	text, err := run(ctx, 10*time.Minute, "ryoku-dictate", "--selftest", wav)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("%s: %w", i18n.T("Could not transcribe the recording. Download a voice model in Settings > Voice typing, or check the selected model"), err)
	}
	return text, nil
}

// Audio converts a recording to mono PCM and transcribes it on demand.
func Audio(ctx context.Context, path string, progress Progress) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err := engines(); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "rashin-audio-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	wav := filepath.Join(dir, "recording.wav")
	progress(i18n.T("Preparing the recording"))
	if _, err := run(ctx, 10*time.Minute, "ffmpeg", "-nostdin", "-v", "error", "-y", "-i", path, "-vn", "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", "-f", "wav", wav); err != nil {
		return "", fmt.Errorf("%s: %w", i18n.T("Could not read the recording"), err)
	}
	progress(i18n.T("Transcribing the recording"))
	text, err := transcribe(ctx, wav)
	if err == nil && text == "" {
		err = fmt.Errorf("%s", i18n.T("No speech was found in the recording."))
	}
	return LimitTranscript(text), err
}

// AcceptsVideo checks unfamiliar sites without downloading the video.
func AcceptsVideo(ctx context.Context, raw string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	proxy, err := startVideoProxy(ctx, raw)
	if err != nil {
		return false, err
	}
	defer proxy.Close()
	_, err = proxy.run(ctx, 15*time.Second, "--no-playlist", "--simulate", "--", raw)
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	return err == nil, nil
}

// Video reads captions, or transcribes short audio segments when captions are absent.
func Video(ctx context.Context, raw, language string, progress Progress) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	proxy, err := startVideoProxy(ctx, raw)
	if err != nil {
		return "", err
	}
	defer proxy.Close()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", fmt.Errorf("%s", i18n.T("Install yt-dlp to read video links."))
	}
	dir, err := os.MkdirTemp("", "rashin-video-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	language = languageCode(language)
	progress(i18n.T("Looking for subtitles"))
	_, subtitlesErr := proxy.run(ctx, 2*time.Minute, "--no-playlist", "--skip-download", "--write-subs", "--write-auto-subs", "--sub-langs", language+".*,en.*", "--sub-format", "vtt", "-o", filepath.Join(dir, "video.%(ext)s"), "--", raw)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.vtt"))
	if err != nil {
		return "", err
	}
	sort.SliceStable(files, func(i, j int) bool {
		return strings.HasPrefix(filepath.Base(files[i]), "video."+language) && !strings.HasPrefix(filepath.Base(files[j]), "video."+language)
	})
	for _, file := range files {
		text, err := readCaptions(file)
		if err != nil {
			return "", err
		}
		if text != "" {
			return LimitTranscript(text), nil
		}
	}
	if err := engines(); err != nil {
		return "", err
	}
	progress(i18n.T("No subtitles found, downloading the audio"))
	_, err = proxy.run(ctx, 10*time.Minute, "--no-playlist", "--match-filter", "duration <= 5400", "-x", "--audio-format", "wav", "-o", filepath.Join(dir, "audio.%(ext)s"), "--", raw)
	if err != nil {
		if subtitlesErr != nil {
			return "", fmt.Errorf("%s: %w (%s: %v)", i18n.T("Could not read the video audio"), err, i18n.T("Subtitles"), subtitlesErr)
		}
		return "", err
	}
	if _, err := os.Stat(filepath.Join(dir, "audio.wav")); err != nil {
		return "", fmt.Errorf("%s", i18n.T("No video audio was downloaded. Recordings longer than 90 minutes are not supported."))
	}
	progress(i18n.T("Preparing the video audio"))
	_, err = run(ctx, 10*time.Minute, "ffmpeg", "-nostdin", "-v", "error", "-i", filepath.Join(dir, "audio.wav"), "-vn", "-t", "5400", "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", "-f", "segment", "-segment_time", "60", filepath.Join(dir, "part-%03d.wav"))
	if err != nil {
		return "", err
	}
	parts, err := filepath.Glob(filepath.Join(dir, "part-*.wav"))
	if err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("%s", i18n.T("The video has no readable audio."))
	}
	var transcript strings.Builder
	for i, part := range parts {
		progress(i18n.Tf("Transcribing video audio, %d of %d", i+1, len(parts)))
		text, err := transcribe(ctx, part)
		if err != nil {
			return "", err
		}
		if text == "" {
			continue
		}
		fmt.Fprintf(&transcript, "[%s] %s\n", timestamp(i*60), text)
	}
	if transcript.Len() == 0 {
		return "", fmt.Errorf("%s", i18n.T("No speech was found in the video."))
	}
	return LimitTranscript(strings.TrimSpace(transcript.String())), nil
}

func readCaptions(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if err != nil {
		return "", err
	}
	if len(data) > 8<<20 {
		return "", fmt.Errorf("%s", i18n.T("The subtitle file is too large to read."))
	}
	return VTTText(string(data)), nil
}

func languageCode(language string) string {
	language = strings.ToLower(strings.Split(strings.Split(strings.Split(language, ".")[0], "_")[0], "-")[0])
	if len(language) >= 2 && len(language) <= 3 {
		for _, c := range language {
			if c < 'a' || c > 'z' {
				return "en"
			}
		}
		return language
	}
	return "en"
}
