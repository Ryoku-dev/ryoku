package media

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"ryoget/internal/plan"

	i18n "ryoku-i18n"
)

var itemExts = map[string]string{
	"jpg": "jpg", "jpeg": "jpg", "png": "png", "webp": "webp", "gif": "gif", "avif": "avif", "heic": "heic",
	"mp4": "mp4", "mov": "mov", "webm": "webm", "m4v": "m4v", "mp3": "mp3", "m4a": "m4a", "ogg": "ogg", "opus": "opus",
}

// itemsPlan plans the selected items of a post (all when none are selected).
// ok is false when the selection is just the item the formats belong to,
// which the format plan already covers with quality and mode.
func itemsPlan(m *Media, opts Options) (*plan.Plan, bool, error) {
	picked := opts.Items
	if len(picked) == 0 {
		picked = make([]int, len(m.Items))
		for i := range picked {
			picked[i] = i
		}
	}
	for _, i := range picked {
		if i < 0 || i >= len(m.Items) {
			return nil, true, fmt.Errorf("%w: %s", ErrInvalid, i18n.Tf("There is no item %d in this post.", i+1))
		}
	}
	if len(picked) == 1 && len(m.Formats) > 0 && picked[0] == formatsItem(m) {
		return nil, false, nil
	}
	title := firstOf(m.Title, m.Source.Name, "download")
	var parts []plan.Part
	for _, i := range picked {
		it := m.Items[i]
		ext, ok := directItem(m.URL, it)
		if !ok {
			return nil, true, fmt.Errorf("%w: %s", ErrUnsupported, i18n.Tf("Item %d is a page of its own; add its link to download it.", i+1))
		}
		name := title
		if len(m.Items) > 1 {
			name = fmt.Sprintf("%s %02d", title, i+1)
		}
		parts = append(parts, plan.Part{Kind: plan.PartHTTP, URL: it.URL, Ext: ext, Size: -1, Role: "item",
			Name: SafeName(name, ext), Headers: mergeHeaders(m.Headers, nil)})
	}
	if len(parts) == 1 {
		p := parts[0]
		p.Role, p.Name = "file", ""
		return &plan.Plan{Name: parts[0].Name, Container: p.Ext, Parts: []plan.Part{p}}, true, nil
	}
	return &plan.Plan{Name: SafeName(title, ""), Parts: parts}, true, nil
}

// formatsItem is the item m.Formats belong to: the one at m.URL (a playlist
// opened on one of its videos), else the first video.
func formatsItem(m *Media) int {
	first := -1
	for i, it := range m.Items {
		if it.URL == m.URL {
			return i
		}
		if it.Kind == "video" && first < 0 {
			first = i
		}
	}
	return first
}

// directItem tells a media file URL from a per-item page and picks the file
// extension: a known extension (also after "@", as Bluesky's CDN writes it), a
// format= query (X's image CDN), or a host other than the post's own.
func directItem(post string, it Item) (string, bool) {
	// a file a page links to is a task of its own so its host names and sizes it
	if it.Kind == "file" {
		return "", false
	}
	u, err := url.Parse(it.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	base := path.Base(u.Path)
	if i := strings.LastIndexByte(base, '@'); i >= 0 {
		if e, ok := itemExts[strings.ToLower(base[i+1:])]; ok {
			return e, true
		}
	}
	if e, ok := itemExts[strings.ToLower(strings.TrimPrefix(path.Ext(base), "."))]; ok {
		return e, true
	}
	if e, ok := itemExts[strings.ToLower(u.Query().Get("format"))]; ok {
		return e, true
	}
	pu, err := url.Parse(post)
	if err == nil && pu.Hostname() != "" && site(pu.Hostname()) == site(u.Hostname()) {
		return "", false
	}
	return defaultExt(it.Kind), true
}

func site(host string) string {
	labels := strings.Split(strings.TrimSuffix(strings.ToLower(host), "."), ".")
	if len(labels) > 2 {
		labels = labels[len(labels)-2:]
	}
	return strings.Join(labels, ".")
}

func defaultExt(kind string) string {
	switch kind {
	case "video", "gif":
		return "mp4"
	case "audio":
		return "mp3"
	}
	return "jpg"
}
