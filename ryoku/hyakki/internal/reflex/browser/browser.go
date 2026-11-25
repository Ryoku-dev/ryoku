// Package browser is a reflex's only way into a web browser: it hands the
// browser an address through the browser's own launcher (its desktop
// entry's Exec line), the way a link opened from another app reaches it, so
// a running browser opens a new tab. Nothing here sends a key or touches a
// browser window; closing or switching tabs is not offered.
package browser

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ryoku-rashin/internal/reflex/launch"
)

// ErrNoBrowser means no web browser is in the catalog.
var ErrNoBrowser = errors.New("browser: no web browser installed")

// BlankPage is what a new tab opens.
const BlankPage = "about:blank"

// DefaultSearch is the search address a spoken query is put into; %s is
// the escaped query.
const DefaultSearch = "https://duckduckgo.com/?q=%s"

// Browser opens tabs in one catalog browser.
type Browser struct {
	l      *launch.Launcher
	id     string
	search string
}

// New returns a Browser for the catalog entry id. search is a search
// address with %s for the query; empty means DefaultSearch.
func New(l *launch.Launcher, id, search string) (*Browser, error) {
	e, ok := l.Catalog().ByID(id)
	if !ok || !e.Browser() {
		return nil, ErrNoBrowser
	}
	if search == "" {
		search = DefaultSearch
	}
	if !strings.Contains(search, "%s") || !launch.AllowedURL(strings.Replace(search, "%s", "q", 1)) {
		return nil, errors.New("browser: search address must be https with %s")
	}
	return &Browser{l: l, id: id, search: search}, nil
}

// ID is the browser's desktop file ID.
func (b *Browser) ID() string { return b.id }

// NewTab opens a blank tab.
func (b *Browser) NewTab(ctx context.Context) (launch.Record, error) {
	return b.l.OpenURL(ctx, b.id, BlankPage)
}

// Open opens an https address in a new tab.
func (b *Browser) Open(ctx context.Context, address string) (launch.Record, error) {
	return b.l.OpenURL(ctx, b.id, address)
}

// Search opens a search for words in a new tab.
func (b *Browser) Search(ctx context.Context, words string) (launch.Record, error) {
	words = strings.TrimSpace(words)
	if words == "" {
		return launch.Record{}, launch.ErrBadURL
	}
	return b.l.OpenURL(ctx, b.id, strings.Replace(b.search, "%s", url.QueryEscape(words), 1))
}

// Default picks the browser from cat: the first default handler for
// https, then http, named in the mimeapps.list files (most important
// first) that is a browser in the catalog; else the first catalog entry
// that says it is a browser.
func Default(cat *launch.Catalog, mimeapps []string) (launch.Entry, bool) {
	for _, scheme := range []string{"x-scheme-handler/https", "x-scheme-handler/http"} {
		for _, file := range mimeapps {
			for _, id := range defaults(file, scheme) {
				if e, ok := cat.ByID(id); ok && e.Browser() {
					return e, true
				}
			}
		}
	}
	for _, e := range cat.Entries() {
		if e.Browser() {
			return e, true
		}
	}
	return launch.Entry{}, false
}

// maxMimeappsBytes bounds how much of one mimeapps.list is read.
const maxMimeappsBytes = 256 << 10

// defaults reads the [Default Applications] value for mime in one file.
func defaults(file, mime string) []string {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(io.LimitReader(f, maxMimeappsBytes))
	in := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			in = line == "[Default Applications]"
			continue
		}
		if !in {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != mime {
			continue
		}
		var ids []string
		for _, id := range strings.Split(value, ";") {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
		return ids
	}
	return nil
}

// Mimeapps are the mimeapps.list files for env, most important first:
// $XDG_CONFIG_HOME, each of $XDG_CONFIG_DIRS, then the applications
// directories.
func Mimeapps(env []string) []string {
	var files []string
	config := lookup(env, "XDG_CONFIG_HOME")
	if config == "" {
		if h := lookup(env, "HOME"); h != "" {
			config = filepath.Join(h, ".config")
		}
	}
	if filepath.IsAbs(config) {
		files = append(files, filepath.Join(config, "mimeapps.list"))
	}
	dirs := lookup(env, "XDG_CONFIG_DIRS")
	if dirs == "" {
		dirs = "/etc/xdg"
	}
	for _, d := range strings.Split(dirs, ":") {
		if filepath.IsAbs(d) {
			files = append(files, filepath.Join(d, "mimeapps.list"))
		}
	}
	for _, d := range launch.Dirs(env) {
		files = append(files, filepath.Join(d, "mimeapps.list"))
	}
	return files
}

func lookup(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if k, v, ok := strings.Cut(env[i], "="); ok && k == key {
			return v
		}
	}
	return ""
}
