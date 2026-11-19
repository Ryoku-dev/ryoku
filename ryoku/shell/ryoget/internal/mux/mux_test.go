package mux

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ryoget/internal/plan"
)

func needFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " not installed")
		}
	}
}

func gen(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	full := append([]string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y"}, args...)
	full = append(full, out)
	if b, err := exec.Command("ffmpeg", full...).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot make %s: %v %s", name, err, b)
	}
	return out
}

type probe struct {
	Streams []struct {
		Type  string `json:"codec_type"`
		Codec string `json:"codec_name"`
	} `json:"streams"`
	Format struct {
		Name string            `json:"format_name"`
		Tags map[string]string `json:"tags"`
	} `json:"format"`
}

func probeFile(t *testing.T, path string) probe {
	t.Helper()
	b, err := exec.Command("ffprobe", "-v", "error", "-show_streams", "-show_format", "-of", "json", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	var p probe
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func (p probe) kinds() string {
	var out []string
	for _, s := range p.Streams {
		out = append(out, s.Type+":"+s.Codec)
	}
	return strings.Join(out, " ")
}

func TestFinish(t *testing.T) {
	needFFmpeg(t)
	dir := t.TempDir()
	video := gen(t, dir, "v.part0.mp4", "-f", "lavfi", "-i", "testsrc=size=64x48:rate=10:duration=1", "-c:v", "mpeg4", "-an")
	audio := gen(t, dir, "a.part1.m4a", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "aac", "-vn")
	muxed := gen(t, dir, "m.ts", "-f", "lavfi", "-i", "testsrc=size=64x48:rate=10:duration=1", "-f", "lavfi", "-i", "sine=duration=1",
		"-c:v", "mpeg4", "-c:a", "aac", "-shortest", "-f", "mpegts")

	cases := []struct {
		name   string
		pl     plan.Plan
		inputs []string
		want   string
		format string
	}{
		{"merge", plan.Plan{Name: "Clip: one.mp4", Container: "mp4", Post: plan.PostMerge,
			Parts: []plan.Part{{Role: "audio"}, {Role: "video"}}}, []string{audio, video}, "video:mpeg4 audio:aac", "mov"},
		{"remux", plan.Plan{Name: "Stream.mp4", Container: "mp4", Post: plan.PostRemux, Parts: []plan.Part{{}}},
			[]string{muxed}, "video:mpeg4 audio:aac", "mov"},
		{"mute", plan.Plan{Name: "Quiet.mkv", Container: "mkv", Post: plan.PostMute, Parts: []plan.Part{{}}},
			[]string{muxed}, "video:mpeg4", "matroska"},
		{"audio copy", plan.Plan{Name: "Song.m4a", Container: "m4a", Post: plan.PostAudio, Parts: []plan.Part{{}}},
			[]string{muxed}, "audio:aac", "mov"},
		{"transcode", plan.Plan{Name: "Song.mp3", Container: "mp3", Post: plan.PostTranscode, Parts: []plan.Part{{}}},
			[]string{audio}, "audio:mp3", "mp3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := filepath.Join(dir, c.pl.Name)
			if err := Finish(context.Background(), "", &c.pl, c.inputs, out); err != nil {
				if c.name == "transcode" && strings.Contains(err.Error(), "libmp3lame") {
					t.Skip("ffmpeg built without libmp3lame")
				}
				t.Fatal(err)
			}
			p := probeFile(t, out)
			if p.kinds() != c.want || !strings.Contains(p.Format.Name, c.format) {
				t.Fatalf("%s: %s in %s", c.name, p.kinds(), p.Format.Name)
			}
			if title := p.Format.Tags["title"]; title != strings.TrimSuffix(c.pl.Name, filepath.Ext(c.pl.Name)) {
				t.Fatalf("title tag = %q", title)
			}
			left, _ := filepath.Glob(filepath.Join(dir, ".*ryoget-mux"))
			if len(left) > 0 {
				t.Fatalf("temp left: %v", left)
			}
		})
	}
	for _, in := range []string{video, audio, muxed} {
		if _, err := os.Stat(in); err != nil {
			t.Fatalf("input removed: %v", err)
		}
	}
}

func TestFinishFailureLeavesNothing(t *testing.T) {
	needFFmpeg(t)
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.mp4")
	os.WriteFile(junk, []byte("not media"), 0o644)
	out := filepath.Join(dir, "out.mp4")
	err := Finish(context.Background(), "", &plan.Plan{Name: "out.mp4", Container: "mp4", Post: plan.PostRemux, Parts: []plan.Part{{}}}, []string{junk}, out)
	if err == nil || !strings.HasPrefix(err.Error(), "ffmpeg: ") {
		t.Fatalf("err = %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("dir = %v", entries)
	}
}

func TestNoFFmpeg(t *testing.T) {
	err := Finish(context.Background(), "/nonexistent/ffmpeg", &plan.Plan{Post: plan.PostRemux, Parts: []plan.Part{{}}}, []string{"x"}, "y")
	if !errors.Is(err, ErrNoFFmpeg) {
		t.Fatalf("err = %v", err)
	}
}
