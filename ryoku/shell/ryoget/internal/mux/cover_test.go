package mux

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ryoget/internal/plan"
)

// coverStream reports the attached picture ffprobe finds in path: its codec and size.
func coverStream(t *testing.T, path string) (string, int, int) {
	t.Helper()
	b, err := exec.Command("ffprobe", "-v", "error", "-show_streams", "-of", "json", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	var p struct {
		Streams []struct {
			Codec       string `json:"codec_name"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	for _, s := range p.Streams {
		if s.Disposition.AttachedPic == 1 {
			return s.Codec, s.Width, s.Height
		}
	}
	return "", 0, 0
}

func coverPlan(post plan.Post, box string) *plan.Plan {
	return &plan.Plan{Name: "Song." + box, Container: box, Post: post, Parts: []plan.Part{
		{Kind: plan.PartHTTP, Role: "audio"}, {Kind: plan.PartHTTP, Role: "cover", Ext: "cover"},
	}}
}

func TestFinishEmbedsCover(t *testing.T) {
	needFFmpeg(t)
	dir := t.TempDir()
	tone := []string{"-f", "lavfi", "-i", "sine=frequency=440:duration=1"}
	mp3 := gen(t, dir, "a.mp3", append(tone, "-c:a", "libmp3lame")...)
	m4a := gen(t, dir, "a.m4a", append(tone, "-c:a", "aac")...)
	opus := gen(t, dir, "a.opus", append(tone, "-c:a", "libopus")...)
	jpg := gen(t, dir, "c.jpg", "-f", "lavfi", "-i", "color=c=red:s=64x48:d=1", "-frames:v", "1")
	// thumbnails often arrive as webp under a name that says nothing
	webp := gen(t, dir, "c.webp", "-f", "lavfi", "-i", "color=c=blue:s=40x30:d=1", "-frames:v", "1", "-c:v", "libwebp")
	unnamed := filepath.Join(dir, "part1.cover")
	if b, err := os.ReadFile(webp); err == nil {
		os.WriteFile(unnamed, b, 0o644)
	}

	for _, c := range []struct {
		name, audio, cover string
		post               plan.Post
		box                string
		w, h               int
	}{
		{"mp3", mp3, jpg, plan.PostAudio, "mp3", 64, 48},
		{"m4a", m4a, unnamed, plan.PostAudio, "m4a", 40, 30},
		{"opus", opus, jpg, plan.PostAudio, "opus", 64, 48},
		{"to-mp3", m4a, jpg, plan.PostTranscode, "mp3", 64, 48},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out."+c.box)
			if err := Finish(context.Background(), "", coverPlan(c.post, c.box), []string{c.audio, c.cover}, out); err != nil {
				t.Fatal(err)
			}
			codec, w, h := coverStream(t, out)
			if codec != "mjpeg" || w != c.w || h != c.h {
				t.Fatalf("cover = %s %dx%d, want mjpeg %dx%d", codec, w, h, c.w, c.h)
			}
			if p := probeFile(t, out); len(p.Streams) == 0 || p.Streams[0].Type != "audio" {
				t.Fatalf("streams = %+v", p.Streams)
			}
			left, _ := filepath.Glob(filepath.Join(filepath.Dir(out), ".*"))
			if len(left) != 0 {
				t.Fatalf("scratch files left: %v", left)
			}
		})
	}

	t.Run("broken picture", func(t *testing.T) {
		bad := filepath.Join(dir, "bad.cover")
		os.WriteFile(bad, []byte("<html>not a picture</html>"), 0o644)
		out := filepath.Join(t.TempDir(), "out.mp3")
		if err := Finish(context.Background(), "", coverPlan(plan.PostAudio, "mp3"), []string{mp3, bad}, out); err != nil {
			t.Fatal(err)
		}
		if codec, _, _ := coverStream(t, out); codec != "" {
			t.Fatalf("a broken picture became a %s cover", codec)
		}
	})
}

// coverTags reports all format and stream tags in path, keys lowercased as Ogg comments come upper case, and its picture's tags.
func coverTags(t *testing.T, path string) (map[string]string, map[string]string) {
	t.Helper()
	b, err := exec.Command("ffprobe", "-v", "error", "-show_streams", "-show_format", "-of", "json", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	var p struct {
		Streams []struct {
			Tags        map[string]string `json:"tags"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	all, pic := map[string]string{}, map[string]string{}
	for k, v := range p.Format.Tags {
		all[strings.ToLower(k)] = v
	}
	for _, s := range p.Streams {
		for k, v := range s.Tags {
			all[strings.ToLower(k)] = v
			if s.Disposition.AttachedPic == 1 {
				pic[strings.ToLower(k)] = v
			}
		}
	}
	return all, pic
}

func TestCoverKeepsSourceTags(t *testing.T) {
	needFFmpeg(t)
	dir := t.TempDir()
	mp3 := gen(t, dir, "a.mp3", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "libmp3lame",
		"-metadata", "artist=Some Artist", "-metadata", "album=Some Album")
	jpg := gen(t, dir, "c.jpg", "-f", "lavfi", "-i", "color=c=red:s=64x48:d=1", "-frames:v", "1")
	out := filepath.Join(t.TempDir(), "out.opus")
	if err := Finish(context.Background(), "", coverPlan(plan.PostTranscode, "opus"), []string{mp3, jpg}, out); err != nil {
		t.Fatal(err)
	}
	if codec, w, h := coverStream(t, out); codec != "mjpeg" || w != 64 || h != 48 {
		t.Fatalf("cover = %s %dx%d, want mjpeg 64x48", codec, w, h)
	}
	tags, _ := coverTags(t, out)
	if tags["artist"] != "Some Artist" || tags["album"] != "Some Album" {
		t.Fatalf("tags = %v, want the source artist and album", tags)
	}
}

func TestCoverInPatternFolder(t *testing.T) {
	needFFmpeg(t)
	src := t.TempDir()
	mp3 := gen(t, src, "a.mp3", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "libmp3lame")
	jpg := gen(t, src, "c.jpg", "-f", "lavfi", "-i", "color=c=red:s=64x48:d=1", "-frames:v", "1")
	// image2 would read %d as a frame number without -update
	dir := filepath.Join(t.TempDir(), "ep%d")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.mp3")
	if err := Finish(context.Background(), "", coverPlan(plan.PostAudio, "mp3"), []string{mp3, jpg}, out); err != nil {
		t.Fatal(err)
	}
	if codec, w, h := coverStream(t, out); codec != "mjpeg" || w != 64 || h != 48 {
		t.Fatalf("cover = %s %dx%d, want mjpeg 64x48", codec, w, h)
	}
}

func TestCoverIsFrontCover(t *testing.T) {
	needFFmpeg(t)
	dir := t.TempDir()
	tone := []string{"-f", "lavfi", "-i", "sine=frequency=440:duration=1"}
	mp3 := gen(t, dir, "a.mp3", append(tone, "-c:a", "libmp3lame")...)
	flac := gen(t, dir, "a.flac", append(tone, "-c:a", "flac")...)
	jpg := gen(t, dir, "c.jpg", "-f", "lavfi", "-i", "color=c=red:s=64x48:d=1", "-frames:v", "1")
	for _, c := range []struct{ box, audio string }{{"mp3", mp3}, {"flac", flac}} {
		t.Run(c.box, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out."+c.box)
			if err := Finish(context.Background(), "", coverPlan(plan.PostAudio, c.box), []string{c.audio, jpg}, out); err != nil {
				t.Fatal(err)
			}
			if _, pic := coverTags(t, out); pic["comment"] != "Cover (front)" {
				t.Fatalf("picture tags = %v, want comment Cover (front)", pic)
			}
		})
	}
}

func TestPictureBlock(t *testing.T) {
	needFFmpeg(t)
	jpg := gen(t, t.TempDir(), "c.jpg", "-f", "lavfi", "-i", "color=c=red:s=16x8:d=1", "-frames:v", "1")
	b, err := pictureBlock(jpg)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(jpg)
	be := func(i int) int { return int(b[i])<<24 | int(b[i+1])<<16 | int(b[i+2])<<8 | int(b[i+3]) }
	// type, mime length, mime, description length, width, height, depth, colours, data length, data
	if be(0) != 3 || be(4) != 10 || string(b[8:18]) != "image/jpeg" || be(18) != 0 || be(22) != 16 || be(26) != 8 ||
		be(38) != len(data) || len(b) != 42+len(data) {
		t.Fatalf("block header = %x", b[:42])
	}
}
