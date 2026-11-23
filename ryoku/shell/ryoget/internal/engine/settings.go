package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"ryoget/internal/paths"
	"ryoget/internal/proto"
)

func DefaultSettings() proto.Settings {
	return proto.Settings{
		Dir:           paths.DownloadDir(),
		MaxActive:     3,
		Connections:   8,
		MinSplitMiB:   1,
		VideoQuality:  "best",
		AudioFormat:   "best",
		SeedRatio:     1.0,
		SeedMinutes:   30,
		Seed:          true,
		DHT:           true,
		Notify:        true,
		ResumeOnStart: true,
		YtdlpFallback: true,
		KeepDays:      7,
		CoverArt:      true,
	}
}

var (
	videoQualities = map[string]bool{"best": true, "2160": true, "1440": true, "1080": true, "720": true, "480": true, "360": true}
	audioFormats   = map[string]bool{"best": true, "mp3": true, "opus": true, "m4a": true}
)

func clamp[T int | int64 | float64](v, lo, hi T) T { return max(lo, min(hi, v)) }

// Normalize clamps every field into range so a hand-edited file or a sloppy
// patch can never wedge the engine.
func Normalize(s proto.Settings) proto.Settings {
	d := DefaultSettings()
	s.Dir = paths.Expand(s.Dir)
	if !filepath.IsAbs(s.Dir) {
		s.Dir = d.Dir
	}
	s.Dir = filepath.Clean(s.Dir)
	s.MaxActive = clamp(s.MaxActive, 1, 16)
	s.Connections = clamp(s.Connections, 1, 16)
	s.MinSplitMiB = clamp(s.MinSplitMiB, 1, 1024)
	s.DownLimit = clamp(s.DownLimit, 0, 1<<30)
	s.UpLimit = clamp(s.UpLimit, 0, 1<<30)
	if !videoQualities[s.VideoQuality] {
		s.VideoQuality = d.VideoQuality
	}
	if !audioFormats[s.AudioFormat] {
		s.AudioFormat = d.AudioFormat
	}
	s.SeedRatio = clamp(s.SeedRatio, 0, 100)
	s.SeedMinutes = clamp(s.SeedMinutes, 0, 7*24*60)
	s.KeepDays = clamp(s.KeepDays, 0, 3650)
	if s.Port != 0 {
		s.Port = clamp(s.Port, 1024, 65535)
	}
	return s
}

// LoadSettings reads the file over the defaults, so a missing key keeps its default.
func LoadSettings(path string) (proto.Settings, error) {
	s := DefaultSettings()
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return DefaultSettings(), err
	}
	return Normalize(s), nil
}

func SaveSettings(path string, s proto.Settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(b, '\n'))
}

// ApplyPatch overlays the patch's keys on cur; unknown keys are refused so a
// typo does not silently do nothing.
func ApplyPatch(cur proto.Settings, patch json.RawMessage) (proto.Settings, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(patch, &keys); err != nil {
		return cur, perr(proto.CodeInvalid, "settings patch is not an object")
	}
	var known map[string]json.RawMessage
	b, _ := json.Marshal(cur)
	_ = json.Unmarshal(b, &known)
	for k, v := range keys {
		if _, ok := known[k]; !ok {
			return cur, perr(proto.CodeInvalid, "unknown setting "+k)
		}
		known[k] = v
	}
	merged, _ := json.Marshal(known)
	next := cur
	dec := json.NewDecoder(bytes.NewReader(merged))
	if err := dec.Decode(&next); err != nil {
		return cur, perr(proto.CodeInvalid, err.Error())
	}
	return Normalize(next), nil
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
