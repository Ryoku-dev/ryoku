package profile

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"ryoku-rashin/internal/oni/characters"
	"ryoku-rashin/internal/oni/spec"
)

// Default is the shipped oni, read fresh from the program each call so a
// caller's changes never reach the next reset.
func Default() Loaded { return load(characters.FS, ShippedID, true) }

// Load reads the user character id from fsys, whose top level holds one
// folder per character. The shipped id is always rejected here: user folders
// cannot shadow it.
func Load(fsys fs.FS, id string) (Loaded, Report) {
	l := load(fsys, id, false)
	return l, l.Report
}

// List is the shipped oni followed by every character folder in userDir, in
// name order, each with its report; rejected ones are listed as invalid.
func List(userDir string) []Loaded {
	out := []Loaded{Default()}
	if userDir == "" {
		return out
	}
	entries, err := os.ReadDir(userDir)
	if err != nil {
		return out
	}
	fsys := os.DirFS(userDir)
	n := 0
	for _, e := range entries {
		if !e.IsDir() && e.Type()&fs.ModeSymlink == 0 {
			continue
		}
		if n == MaxCharacters {
			var r Report
			r.reject(ReasonTooMany, "more than %d characters", MaxCharacters)
			out = append(out, Loaded{Profile: Profile{ID: e.Name()}, Report: r})
			continue
		}
		n++
		l, _ := Load(fsys, e.Name())
		out = append(out, l)
	}
	return out
}

type loader struct {
	fsys    fs.FS
	id      string
	shipped bool
	r       Report
	sizes   map[string]int64
	files   map[string][]byte
}

func load(fsys fs.FS, id string, shipped bool) Loaded {
	l := &loader{fsys: fsys, id: id, shipped: shipped, sizes: map[string]int64{}, files: map[string][]byte{}}
	p, ok := l.profile()
	if !ok || len(l.r.Errors) > 0 {
		return Loaded{Profile: Profile{ID: id}, Report: Report{Errors: l.r.Errors}}
	}
	return Loaded{Profile: p, Report: l.r, Valid: true, Files: l.files}
}

func (l *loader) profile() (Profile, bool) {
	var p Profile
	if !validID(l.id) {
		l.r.reject(ReasonBadID, "%q is not a lowercase id of at most %d letters, digits and dashes", l.id, MaxIDLen)
		return p, false
	}
	if l.id == ShippedID && !l.shipped {
		l.r.reject(ReasonShadowsShipped, "%q is the shipped character and cannot be replaced", l.id)
		return p, false
	}
	switch l.lstat(l.id) {
	case entryMissing:
		l.r.reject(ReasonMissingProfile, "no folder %q", l.id)
		return p, false
	case entrySymlink:
		l.r.reject(ReasonSymlink, "the character folder %q is a symlink", l.id)
		return p, false
	case entryFile:
		l.r.reject(ReasonBadPath, "%q is not a folder", l.id)
		return p, false
	case entryUnreadable:
		return p, false
	}
	data, found := l.read(ProfileFile, ProfileFile, MaxProfileBytes)
	if found && data != nil {
		l.files[ProfileFile] = data
	}
	if !found {
		if len(l.r.Errors) == 0 {
			l.r.reject(ReasonMissingProfile, "%s has no profile.json", l.id)
		}
		return p, false
	}
	if data == nil {
		return p, false
	}
	if !utf8.Valid(data) {
		l.r.reject(ReasonBadJSON, "profile.json: not UTF-8")
		return p, false
	}
	top := scanJSON("profile.json", data, keys, &l.r)
	if top == nil {
		return p, false
	}
	if v, ok := top["version"].(json.Number); !ok || v.String() != strconv.Itoa(Version) {
		l.r.reject(ReasonBadVersion, "profile.json: version %v, this build reads %d", top["version"], Version)
	}
	if len(l.r.Errors) > 0 || !decodeStrict("profile.json", data, &p, &l.r) {
		return p, false
	}
	if p.ID != l.id {
		l.r.reject(ReasonIDMismatch, "profile.json says %q in folder %q", p.ID, l.id)
		return p, false
	}
	l.name(&p)
	l.artwork(&p)
	l.animations(&p)
	l.personality(&p)
	l.voice(&p)
	p.Shipped = l.shipped
	return p, true
}

func (l *loader) name(p *Profile) {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "":
		l.r.reject(ReasonBadValue, "name is empty")
	case utf8.RuneCountInString(p.Name) > MaxNameRunes:
		l.r.reject(ReasonTooLarge, "name is longer than %d characters", MaxNameRunes)
	default:
		if why, bad := unsafeRune(p.Name, false); bad {
			l.r.reject(ReasonUnsafeText, "name contains %s", why)
		}
	}
}

func (l *loader) artwork(p *Profile) {
	a := &p.Artwork
	if an := a.Anchor; an != nil && (!unitFraction(an.X) || !unitFraction(an.Y)) {
		l.r.reject(ReasonBadValue, "artwork.anchor is fractions of the artwork, each from 0 to 1")
		return
	}
	silhouette := func(reason string) {
		l.r.fallback("artwork", reason, UsedSilhouette)
		a.Kind, a.Spec, a.Image, a.Fallback, a.Anchor = ArtworkSilhouette, "", "", "", nil
	}
	switch a.Kind {
	case "":
		if a.Spec != "" || a.Image != "" || a.Fallback != "" || a.Anchor != nil {
			l.r.reject(ReasonBadValue, "artwork has no kind")
			return
		}
		silhouette(FallbackMissing)
	case ArtworkParametric:
		if a.Spec != SpecFile || a.Image != "" {
			l.r.reject(ReasonBadValue, "parametric artwork draws from %q and has no image", SpecFile)
			return
		}
		if a.Fallback == "" {
			l.r.fallback("artwork.fallback", FallbackMissing, UsedSilhouette)
		} else if !l.svg("artwork.fallback", a.Fallback) {
			l.r.fallback("artwork.fallback", FallbackAssetMissing, UsedSilhouette)
			a.Fallback = ""
		}
	case ArtworkSVG:
		if a.Spec != "" || a.Fallback != "" {
			l.r.reject(ReasonBadValue, "svg artwork has only an image")
			return
		}
		if a.Image == "" {
			silhouette(FallbackMissing)
		} else if !l.svg("artwork.image", a.Image) {
			silhouette(FallbackAssetMissing)
		}
	default:
		l.r.reject(ReasonBadValue, "artwork kind %q is not %s or %s", a.Kind, ArtworkParametric, ArtworkSVG)
		return
	}
	if len(a.Outfits) > MaxOutfits {
		l.r.reject(ReasonTooMany, "more than %d outfits", MaxOutfits)
		return
	}
	for _, name := range sortedKeys(a.Outfits) {
		if !validID(name) {
			l.r.reject(ReasonBadValue, "outfit name %q is not an id", name)
			continue
		}
		slot := "artwork.outfits." + name
		if !l.svg(slot, a.Outfits[name]) {
			l.r.fallback(slot, FallbackAssetMissing, UsedBaseArtwork)
			delete(a.Outfits, name)
		}
	}
}

func (l *loader) animations(p *Profile) {
	parametric := p.Artwork.Kind == ArtworkParametric
	poses := func(group string, set map[string]string, ids []string, used string) {
		for _, id := range sortedKeys(set) {
			if !slices.Contains(ids, id) {
				l.r.reject(ReasonUnknownSlot, "animations.%s: %q is not in the motion spec", group, id)
			}
		}
		for _, id := range ids {
			slot := "animations." + group + "." + id
			v, ok := set[id]
			switch {
			case !ok || v == "":
				l.r.fallback(slot, FallbackMissing, used)
				delete(set, id)
			case v == PoseFromSpec:
				if !parametric {
					l.r.reject(ReasonBadValue, "%s: %q poses need parametric artwork", slot, PoseFromSpec)
				}
			case !l.svg(slot, v):
				l.r.fallback(slot, FallbackAssetMissing, used)
				delete(set, id)
			}
		}
	}
	poses("states", p.Animations.States, spec.StateIDs, UsedStaticPose)
	poses("gestures", p.Animations.Gestures, spec.GestureIDs, UsedBasePose)
	for _, id := range sortedKeys(p.Animations.Motifs) {
		notes := p.Animations.Motifs[id]
		switch {
		case !slices.Contains(spec.MotifIDs, id):
			l.r.reject(ReasonUnknownSlot, "animations.motifs: %q is not in the motion spec", id)
		case len(notes) == 0:
			l.r.reject(ReasonBadValue, "animations.motifs.%s has no notes", id)
		case len(notes) > MaxMotifNotes:
			l.r.reject(ReasonTooMany, "animations.motifs.%s has more than %d notes", id, MaxMotifNotes)
		case slices.ContainsFunc(notes, func(n int) bool { return n < 0 || n > 127 }):
			l.r.reject(ReasonBadValue, "animations.motifs.%s has a note outside MIDI 0-127", id)
		}
	}
}

func (l *loader) personality(p *Profile) {
	pe := &p.Personality
	p.Lines = l.lines(pe)
	if p.Lines == nil {
		p.Lines = neutralAcks()
	}
	const slot = "personality.promptPersona"
	if pe.PromptPersona == "" {
		l.r.fallback(slot, FallbackMissing, UsedNoPersona)
		return
	}
	data, found := l.read(slot, pe.PromptPersona, MaxPersonaBytes)
	if !found {
		l.r.fallback(slot, FallbackAssetMissing, UsedNoPersona)
		pe.PromptPersona = ""
		return
	}
	if data == nil {
		return
	}
	text := string(data)
	if why, bad := unsafeRune(text, true); bad {
		l.r.reject(ReasonUnsafeText, "%s: %s contains %s", slot, pe.PromptPersona, why)
		return
	}
	if p.Persona = strings.TrimSpace(text); p.Persona == "" {
		l.r.fallback(slot, FallbackEmpty, UsedNoPersona)
		pe.PromptPersona = ""
	}
}

// lines reads the character's voice lines, or reports the fallback and
// returns nil.
func (l *loader) lines(pe *Personality) []string {
	const slot = "personality.voiceLines"
	if pe.VoiceLines == "" {
		l.r.fallback(slot, FallbackMissing, UsedNeutralAcks)
		return nil
	}
	data, found := l.read(slot, pe.VoiceLines, MaxLinesBytes)
	if !found {
		l.r.fallback(slot, FallbackAssetMissing, UsedNeutralAcks)
		pe.VoiceLines = ""
		return nil
	}
	if data == nil {
		return nil
	}
	file := pe.VoiceLines
	if !utf8.Valid(data) {
		l.r.reject(ReasonBadJSON, "%s: not UTF-8", file)
		return nil
	}
	var lines Lines
	if scanJSON(file, data, linesKeys, &l.r) == nil || !decodeStrict(file, data, &lines, &l.r) {
		return nil
	}
	if len(lines.Acknowledge) > MaxLines {
		l.r.reject(ReasonTooMany, "%s: more than %d lines", file, MaxLines)
		return nil
	}
	out := make([]string, 0, len(lines.Acknowledge))
	for _, line := range lines.Acknowledge {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			l.r.reject(ReasonBadValue, "%s: an empty line", file)
			return nil
		case len(line) > MaxLineBytes:
			l.r.reject(ReasonTooLarge, "%s: a line longer than %d bytes", file, MaxLineBytes)
			return nil
		}
		if why, bad := unsafeRune(line, false); bad {
			l.r.reject(ReasonUnsafeText, "%s: a line contains %s", file, why)
			return nil
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		l.r.fallback(slot, FallbackEmpty, UsedNeutralAcks)
		pe.VoiceLines = ""
		return nil
	}
	return out
}

func (l *loader) voice(p *Profile) {
	v := &p.Voice
	if v.Engine == "" {
		v.Engine = VoiceSystem
	}
	if v.Speed == 0 {
		v.Speed = 1
	}
	if v.Honorific == "" {
		v.Honorific = HonorificNone
	}
	if !slices.Contains([]string{VoiceSystem, VoiceClips, VoiceKokoro}, v.Engine) {
		l.r.reject(ReasonBadValue, "voice engine %q", v.Engine)
	}
	if v.Speed < MinSpeed || v.Speed > MaxSpeed {
		l.r.reject(ReasonBadValue, "voice speed %v is outside %v-%v", v.Speed, MinSpeed, MaxSpeed)
	}
	if v.Speaker < 0 || v.Speaker > MaxSpeaker {
		l.r.reject(ReasonBadValue, "voice speaker %d is outside 0-%d", v.Speaker, MaxSpeaker)
	}
	if !slices.Contains([]string{HonorificNone, HonorificSir, HonorificMaam}, v.Honorific) {
		l.r.reject(ReasonBadValue, "voice honorific %q", v.Honorific)
	}
	switch {
	case v.Engine == VoiceClips && v.ClipPack == "":
		l.r.reject(ReasonBadValue, "the clips engine needs a clipPack")
	case v.Engine != VoiceClips && v.ClipPack != "":
		l.r.reject(ReasonBadValue, "a clipPack needs the clips engine")
	case v.ClipPack != "":
		l.clipPack(strings.TrimSuffix(v.ClipPack, "/"))
	}
}

// clipPack rejects a recorded voice that does not say where it came from and
// under what license.
func (l *loader) clipPack(dir string) {
	if !assetPath("voice.clipPack", dir, &l.r) {
		return
	}
	full := path.Join(l.id, dir)
	switch l.walkTo(full) {
	case entryMissing:
		l.r.reject(ReasonClipUnlicensed, "clip pack %q is missing", dir)
		return
	case entrySymlink:
		l.r.reject(ReasonSymlink, "voice.clipPack: %q", dir)
		return
	case entryFile:
		l.r.reject(ReasonBadPath, "voice.clipPack: %q is not a folder", dir)
		return
	case entryUnreadable:
		return
	}
	entries, err := fs.ReadDir(l.fsys, full)
	if err != nil {
		l.r.reject(ReasonUnreadable, "voice.clipPack: %v", err)
		return
	}
	if len(entries) > MaxClipEntries {
		l.r.reject(ReasonTooMany, "clip pack %q has more than %d entries", dir, MaxClipEntries)
		return
	}
	licensed, sourced := false, false
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			l.r.reject(ReasonSymlink, "clip pack %q: %q", dir, e.Name())
			return
		}
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 {
			continue
		}
		upper := strings.ToUpper(e.Name())
		switch {
		case strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "LICENCE") || strings.HasPrefix(upper, "COPYING"):
			licensed = true
		case e.Name() == "PROVENANCE.md":
			sourced = true
		}
	}
	if !licensed {
		l.r.reject(ReasonClipUnlicensed, "clip pack %q has no license file", dir)
	}
	if !sourced {
		l.r.reject(ReasonClipNoSource, "clip pack %q has no PROVENANCE.md", dir)
	}
}

// svg checks one SVG asset; false means it is not there and the slot falls
// back. Anything wrong with a file that is there rejects the character.
func (l *loader) svg(slot, p string) bool {
	if !strings.HasSuffix(p, ".svg") {
		l.r.reject(ReasonBadPath, "%s: %q is not an .svg file", slot, p)
		return true
	}
	data, found := l.read(slot, p, MaxSVGBytes)
	if found && data != nil {
		checkSVG(p, data, &l.r)
		l.files[p] = data
	}
	return found
}

// read reads an asset of the character. found is false only when the file is
// simply not there; data is nil when the file was rejected.
func (l *loader) read(slot, p string, max int64) (data []byte, found bool) {
	if !assetPath(slot, p, &l.r) {
		return nil, true
	}
	full := path.Join(l.id, p)
	switch l.walkTo(full) {
	case entryMissing:
		return nil, false
	case entrySymlink:
		l.r.reject(ReasonSymlink, "%s: %q", slot, p)
		return nil, true
	case entryDir:
		l.r.reject(ReasonBadPath, "%s: %q is a folder", slot, p)
		return nil, true
	case entryUnreadable:
		return nil, true
	}
	info, err := fs.Stat(l.fsys, full)
	if err != nil {
		l.r.reject(ReasonUnreadable, "%s: %v", slot, err)
		return nil, true
	}
	if info.Size() > max {
		l.r.reject(ReasonTooLarge, "%s: %q is larger than %d bytes", slot, p, max)
		return nil, true
	}
	f, err := l.fsys.Open(full)
	if err != nil {
		l.r.reject(ReasonUnreadable, "%s: %v", slot, err)
		return nil, true
	}
	defer f.Close()
	data, err = io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		l.r.reject(ReasonUnreadable, "%s: %v", slot, err)
		return nil, true
	}
	if int64(len(data)) > max {
		l.r.reject(ReasonTooLarge, "%s: %q is larger than %d bytes", slot, p, max)
		return nil, true
	}
	if _, counted := l.sizes[full]; !counted {
		l.sizes[full] = int64(len(data))
		var total int64
		for _, n := range l.sizes {
			total += n
		}
		if total > MaxAssetBytes {
			l.r.reject(ReasonTooLarge, "the character's files are larger than %d bytes", MaxAssetBytes)
			return nil, true
		}
	}
	return data, true
}

type entry int

const (
	entryFile entry = iota
	entryDir
	entryMissing
	entrySymlink
	entryUnreadable
)

// walkTo checks every element of name from the top of fsys without
// following symlinks, so no link anywhere on the way leads out.
func (l *loader) walkTo(name string) entry {
	parts := strings.Split(name, "/")
	for i := range parts {
		kind := l.lstat(strings.Join(parts[:i+1], "/"))
		if kind != entryDir && i < len(parts)-1 {
			if kind == entryFile {
				return entryMissing
			}
			return kind
		}
		if i == len(parts)-1 {
			return kind
		}
	}
	return entryMissing
}

func (l *loader) lstat(name string) entry {
	info, err := fs.Lstat(l.fsys, name)
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return entryMissing
	case err != nil:
		l.r.reject(ReasonUnreadable, "%q: %v", name, err)
		return entryUnreadable
	case info.Mode()&fs.ModeSymlink != 0:
		return entrySymlink
	case info.IsDir():
		return entryDir
	case info.Mode().IsRegular():
		return entryFile
	}
	l.r.reject(ReasonBadPath, "%q is not a regular file", name)
	return entryUnreadable
}

// neutralAcks are the shipped oni's acknowledgements, used by a character
// without lines of its own.
func neutralAcks() []string {
	data, err := fs.ReadFile(characters.FS, ShippedID+"/lines.json")
	if err != nil {
		return nil
	}
	var lines Lines
	if json.Unmarshal(data, &lines) != nil {
		return nil
	}
	return lines.Acknowledge
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
