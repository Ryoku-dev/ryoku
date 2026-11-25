// Package clips plays a recorded voice: a folder of WAV clips with a
// manifest, composed into times, dates, counts and short lines. A pack is
// read only when a line is spoken; opening one checks its manifest and the
// same license and provenance rules the profile loader applies.
package clips

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"ryoku-rashin/internal/oni/profile"
)

// Manifest is the file every pack carries.
const Manifest = "manifest.json"

// Caps on one pack.
const (
	MaxManifestBytes = 1 << 20
	MaxClipBytes     = 4 << 20
)

// Variants a clip may be recorded in. Plain is the clip itself.
const (
	Plain = ""
	Comma = "comma" // said before a pause, rising
	Final = "final" // said last, falling
	Fast  = "fast"  // said quickly inside a run
)

var (
	// ErrInvalid is a pack that may not be used.
	ErrInvalid = errors.New("clips: invalid pack")
	// ErrMissingClip is a word the pack has no clip for.
	ErrMissingClip = errors.New("clips: missing clip")
)

// Clip is one recorded word or line.
type Clip struct {
	ID       string
	Text     string            // what it says, matched by Speak
	File     string            // the plain recording
	Variants map[string]string // variant -> file
}

// Pack is an opened clip pack. It holds the manifest only; audio is read
// from the pack when PCM asks for it.
type Pack struct {
	fsys  fs.FS
	dir   string
	clips map[string]Clip
	texts map[string]string // spoken text -> id
	words int               // most words in one clip's text
}

// entry is one manifest clip: a file name, or an object with variants.
type entry struct {
	File  string `json:"file"`
	Text  string `json:"text"`
	Comma string `json:"comma"`
	Final string `json:"final"`
	Fast  string `json:"fast"`
}

func (e *entry) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, &e.File)
	}
	type plain entry
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode((*plain)(e))
}

var (
	idPattern   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	filePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*\.wav$`)
)

// ForProfile opens the clip pack of a character loaded from fsys (the
// folder holding character folders, as profile.Load reads it).
func ForProfile(fsys fs.FS, id string, v profile.Voice) (*Pack, error) {
	if v.Engine != profile.VoiceClips || v.ClipPack == "" {
		return nil, fmt.Errorf("%w: character %q has no clip pack", ErrInvalid, id)
	}
	return Open(fsys, path.Join(id, strings.TrimSuffix(v.ClipPack, "/")))
}

// Open checks the pack in folder dir of fsys and reads its manifest. Like
// the profile loader it refuses a pack with a symlink on the way or inside,
// more than profile.MaxClipEntries entries, or no non-empty license file
// (LICENSE*, LICENCE*, COPYING*) and PROVENANCE.md. Every manifest file
// must be a non-empty .wav in the folder itself.
func Open(fsys fs.FS, dir string) (*Pack, error) {
	dir = path.Clean(dir)
	if !fs.ValidPath(dir) {
		return nil, fmt.Errorf("%w: %q is not a path inside the profile", ErrInvalid, dir)
	}
	if err := noLinks(fsys, dir); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if len(entries) > profile.MaxClipEntries {
		return nil, fmt.Errorf("%w: %q has more than %d entries", ErrInvalid, dir, profile.MaxClipEntries)
	}
	sizes := map[string]int64{}
	licensed, sourced := false, false
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: %q is a symlink", ErrInvalid, path.Join(dir, e.Name()))
		}
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 {
			continue
		}
		sizes[e.Name()] = info.Size()
		upper := strings.ToUpper(e.Name())
		switch {
		case strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "LICENCE") || strings.HasPrefix(upper, "COPYING"):
			licensed = true
		case e.Name() == "PROVENANCE.md":
			sourced = true
		}
	}
	if !licensed {
		return nil, fmt.Errorf("%w: %q has no license file", ErrInvalid, dir)
	}
	if !sourced {
		return nil, fmt.Errorf("%w: %q has no PROVENANCE.md", ErrInvalid, dir)
	}
	if n, ok := sizes[Manifest]; !ok || n > MaxManifestBytes {
		return nil, fmt.Errorf("%w: %q has no %s of at most %d bytes", ErrInvalid, dir, Manifest, MaxManifestBytes)
	}
	raw, err := fs.ReadFile(fsys, path.Join(dir, Manifest))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	var m struct {
		Version int              `json:"version"`
		Clips   map[string]entry `json:"clips"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrInvalid, Manifest, err)
	}
	if m.Version != 1 {
		return nil, fmt.Errorf("%w: %s version %d", ErrInvalid, Manifest, m.Version)
	}
	p := &Pack{fsys: fsys, dir: dir, clips: map[string]Clip{}, texts: map[string]string{}}
	for id, e := range m.Clips {
		if !idPattern.MatchString(id) {
			return nil, fmt.Errorf("%w: clip id %q", ErrInvalid, id)
		}
		c := Clip{ID: id, Text: e.Text, File: e.File, Variants: map[string]string{}}
		for v, f := range map[string]string{Comma: e.Comma, Final: e.Final, Fast: e.Fast} {
			if f != "" {
				c.Variants[v] = f
			}
		}
		for _, f := range append([]string{c.File}, mapValues(c.Variants)...) {
			n, ok := sizes[f]
			switch {
			case !filePattern.MatchString(f):
				return nil, fmt.Errorf("%w: clip %q: %q is not a .wav name in the pack", ErrInvalid, id, f)
			case !ok:
				return nil, fmt.Errorf("%w: clip %q: %q is missing or empty", ErrInvalid, id, f)
			case n > MaxClipBytes:
				return nil, fmt.Errorf("%w: clip %q: %q is over %d bytes", ErrInvalid, id, f, MaxClipBytes)
			}
		}
		if c.Text == "" {
			c.Text = strings.ReplaceAll(id, "-", " ")
		}
		key := strings.Join(words(c.Text), " ")
		if key == "" {
			return nil, fmt.Errorf("%w: clip %q says nothing", ErrInvalid, id)
		}
		if other, dup := p.texts[key]; dup {
			return nil, fmt.Errorf("%w: clips %q and %q both say %q", ErrInvalid, other, id, key)
		}
		p.texts[key] = id
		p.words = max(p.words, strings.Count(key, " ")+1)
		p.clips[id] = c
	}
	return p, nil
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// noLinks refuses a symlink anywhere on the way to dir.
func noLinks(fsys fs.FS, dir string) error {
	if dir == "." {
		return nil
	}
	at := ""
	for part := range strings.SplitSeq(dir, "/") {
		at = path.Join(at, part)
		info, err := fs.Lstat(fsys, at)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: %q is a symlink", ErrInvalid, at)
		}
		if !info.IsDir() {
			return fmt.Errorf("%w: %q is not a folder", ErrInvalid, at)
		}
	}
	return nil
}

// Has reports whether the pack holds clip id.
func (p *Pack) Has(id string) bool { _, ok := p.clips[id]; return ok }

// Clip returns clip id.
func (p *Pack) Clip(id string) (Clip, bool) { c, ok := p.clips[id]; return c, ok }

// PCM reads and joins the clips refs name (as Compose and Speak return
// them: "id" or "id.variant"), at rateHz.
func (p *Pack) PCM(refs []string, rateHz int) ([]float32, error) {
	var out []float32
	for _, ref := range refs {
		id, variant, _ := strings.Cut(ref, ".")
		c, ok := p.clips[id]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrMissingClip, id)
		}
		file := c.File
		if variant != Plain {
			if file, ok = c.Variants[variant]; !ok {
				return nil, fmt.Errorf("%w: %q has no %s variant", ErrMissingClip, id, variant)
			}
		}
		data, err := fs.ReadFile(p.fsys, path.Join(p.dir, file))
		if err != nil {
			return nil, err
		}
		pcm, rate, err := DecodeWAV(data)
		if err != nil {
			return nil, fmt.Errorf("clip %q: %w", ref, err)
		}
		out = append(out, Resample(pcm, rate, rateHz)...)
	}
	return out, nil
}
