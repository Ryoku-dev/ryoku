// Package launch opens installed applications for a reflex. The catalog is
// read from desktop entry files and bounded; an app starts from its own
// Exec line with every field code stripped and no shell, inside a
// transient user scope where the systemd user manager runs (so the app
// outlives the daemon's unit) and plainly detached otherwise.
package launch

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"ryoku-rashin/internal/oni/reflex"
)

// maxScannedFiles bounds how many files one scan opens, whatever the
// catalog's own bound, so a huge applications directory cannot stall it.
const maxScannedFiles = 4096

// Catalog is the bounded list of apps a reflex may open.
type Catalog struct {
	entries []Entry
	// Dropped counts launchable entries left out because the catalog was
	// full.
	Dropped int
}

// ScanOptions says where and how to read desktop entries.
type ScanOptions struct {
	// Dirs are applications directories, most important first; the first
	// file with a given ID wins, as the format says.
	Dirs []string
	// Max bounds the catalog (reflex.catalogMax).
	Max int
	// Desktops is XDG_CURRENT_DESKTOP split on ':', for OnlyShowIn and
	// NotShowIn.
	Desktops []string
	// LookPath resolves TryExec; nil skips the check.
	LookPath func(string) (string, error)
}

// Scan reads the desktop entries under o.Dirs into a catalog of at most
// o.Max apps. Hidden, NoDisplay and terminal entries, entries for other
// desktops, entries whose TryExec is missing, invalid Exec lines and
// commands that would run a shell or a privilege tool are left out. A
// directory that cannot be read is skipped.
func Scan(o ScanOptions) *Catalog {
	c := &Catalog{}
	seen := map[string]bool{}
	files := 0
	for _, dir := range o.Dirs {
		var ids []string
		paths := map[string]string{}
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".desktop") || !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return nil
			}
			id := strings.ReplaceAll(filepath.ToSlash(rel), "/", "-")
			ids = append(ids, id)
			paths[id] = p
			return nil
		})
		sort.Strings(ids)
		for _, id := range ids {
			if seen[id] {
				continue
			}
			if files >= maxScannedFiles {
				return c
			}
			files++
			seen[id] = true
			e, ok := readEntry(paths[id], id, o)
			if !ok {
				continue
			}
			if len(c.entries) >= o.Max {
				c.Dropped++
				continue
			}
			c.entries = append(c.entries, e)
		}
	}
	return c
}

func readEntry(p, id string, o ScanOptions) (Entry, bool) {
	f, err := os.Open(p)
	if err != nil {
		return Entry{}, false
	}
	defer f.Close()
	fields, err := parseEntry(f)
	if err != nil || fields.kind != "Application" || fields.hidden || fields.noDisplay ||
		fields.term || fields.name == "" || fields.exec == "" || !shown(fields, o.Desktops) {
		return Entry{}, false
	}
	if fields.tryExec != "" && o.LookPath != nil {
		if _, err := o.LookPath(fields.tryExec); err != nil {
			return Entry{}, false
		}
	}
	tokens, err := splitExec(fields.exec)
	if err != nil || len(tokens) == 0 || !checkCodes(tokens) {
		return Entry{}, false
	}
	e := Entry{ID: id, Name: fields.name, Categories: fields.categories, MimeTypes: fields.mimeTypes, tokens: tokens}
	argv := e.Argv()
	if len(argv) == 0 || refusedPrograms[program(argv)] || strings.HasPrefix(argv[0], "-") {
		return Entry{}, false
	}
	if stem := idStem(id); stem != "" && plainName(stem) != plainName(e.Name) {
		e.Aliases = []string{stem}
	}
	return e, true
}

// shown applies OnlyShowIn and NotShowIn.
func shown(f entryFields, desktops []string) bool {
	for _, d := range desktops {
		if slices.Contains(f.notShowIn, d) {
			return false
		}
	}
	if len(f.onlyShowIn) == 0 {
		return true
	}
	for _, d := range desktops {
		if slices.Contains(f.onlyShowIn, d) {
			return true
		}
	}
	return false
}

// idStem is the last dotted part of a reverse-domain ID, the name people
// tend to say: "org.example.Notes.desktop" answers to "Notes".
func idStem(id string) string {
	id = strings.TrimSuffix(id, ".desktop")
	if i := strings.LastIndexByte(id, '.'); i >= 0 {
		id = id[i+1:]
	}
	return id
}

func plainName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Entries returns the catalog's apps in catalog order. The slice is a copy.
func (c *Catalog) Entries() []Entry { return slices.Clone(c.entries) }

// Len is the number of apps in the catalog.
func (c *Catalog) Len() int { return len(c.entries) }

// Alias lets the app with the given ID also answer to name, e.g. "browser"
// for the default browser. It reports whether the ID is in the catalog.
func (c *Catalog) Alias(name, id string) bool {
	for i := range c.entries {
		if c.entries[i].ID == id {
			if !slices.Contains(c.entries[i].Aliases, name) {
				c.entries[i].Aliases = append(c.entries[i].Aliases, name)
			}
			return true
		}
	}
	return false
}

// ByID returns the app with the given desktop file ID.
func (c *Catalog) ByID(id string) (Entry, bool) {
	for _, e := range c.entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Lookup returns the app whose name or an alias is name, compared without
// case, spaces or punctuation. The plan's open_app argument is the catalog
// name Resolve returned, so this is an exact lookup, never a guess.
func (c *Catalog) Lookup(name string) (Entry, bool) {
	want := plainName(name)
	if want == "" {
		return Entry{}, false
	}
	for _, e := range c.entries {
		if plainName(e.Name) == want {
			return e, true
		}
	}
	for _, e := range c.entries {
		for _, a := range e.Aliases {
			if plainName(a) == want {
				return e, true
			}
		}
	}
	return Entry{}, false
}

// Resolve implements reflex.Catalog: the app whose name or alias best
// matches what was said, by reflex.Best. Each app takes part once, under
// whichever of its names sounds closest, so an app's own alias never
// crowds out its name. name is the app's catalog name and exec its
// command, for the record only.
func (c *Catalog) Resolve(spoken string) (name, exec string, score float64, ok bool) {
	names := make([]string, len(c.entries))
	for i, e := range c.entries {
		names[i] = e.Name
		best := reflex.Soundslike(spoken, e.Name)
		for _, a := range e.Aliases {
			if s := reflex.Soundslike(spoken, a); s > best {
				names[i], best = a, s
			}
		}
	}
	i, score, ok := reflex.Best(spoken, names)
	if !ok || i < 0 || i >= len(c.entries) {
		return "", "", score, false
	}
	e := c.entries[i]
	return e.Name, strings.Join(e.Argv(), " "), score, true
}

var _ reflex.Catalog = (*Catalog)(nil)
