package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoget/internal/httpdl"
	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

func (e *Engine) work(ctx context.Context, t *task) error {
	e.mu.Lock()
	rec := t.record
	rec.Sel = append([]int(nil), t.Sel...)
	e.mu.Unlock()
	if err := os.MkdirAll(rec.Dir, 0o755); err != nil {
		return err
	}
	if rec.Kind == "torrent" {
		return e.runTorrent(ctx, t, rec)
	}
	var m *media.Media
	if !rec.Known || rec.Kind == "media" {
		mm, err := e.parts.Resolver.Resolve(ctx, rec.URL)
		if err != nil {
			return err
		}
		m = mm
		if m.Kind == plan.KindPage || m.Kind == "" {
			return perr(proto.CodeUnsupported, i18n.T("This page has nothing to download."))
		}
		e.adopt(ctx, t, m)
		e.mu.Lock()
		rec = t.record
		e.mu.Unlock()
		if m.Kind == plan.KindTorrent {
			return e.runTorrent(ctx, t, rec)
		}
	}
	if rec.Kind == string(plan.KindMedia) {
		return e.runMedia(ctx, t, rec, m)
	}
	return e.runFile(ctx, t, rec, m)
}

// adopt takes what the resolver learned about the link into the task.
func (e *Engine) adopt(ctx context.Context, t *task, m *media.Media) {
	thumb := ""
	if m.Thumb != "" && e.parts.Thumb != nil {
		thumb = e.parts.Thumb(ctx, m.Thumb)
	}
	e.with(t, func() {
		t.Known = true
		t.Kind = string(m.Kind)
		e.placeByKind(t)
		if m.Title != "" {
			t.Title = m.Title
		} else if m.Name != "" {
			t.Title = m.Name
		}
		if m.Source.Name != "" || m.Source.ID != "" {
			t.Source = proto.Source{ID: m.Source.ID, Name: m.Source.Name}
		}
		if m.Mime != "" {
			t.Mime = m.Mime
		}
		if m.Size > 0 {
			t.Size = m.Size
		}
		if thumb != "" {
			t.Thumb = thumb
		}
		if m.Kind != plan.KindMedia && m.URL != "" && m.URL != t.URL {
			t.FetchURL = m.URL
		}
		if m.Kind != plan.KindMedia && len(m.Headers) > 0 {
			t.Headers = m.Headers
		}
		if t.Format != nil {
			for _, f := range m.Formats {
				if f.ID == t.Format.ID {
					t.Format.Label = f.Label
				}
			}
		}
	})
}

func toHeader(m map[string]string) http.Header {
	if len(m) == 0 {
		return nil
	}
	h := http.Header{}
	for k, v := range m {
		h.Set(k, v)
	}
	return h
}

func (e *Engine) runFile(ctx context.Context, t *task, rec record, m *media.Media) error {
	link := rec.FetchURL
	if link == "" {
		link = rec.URL
	}
	headers := toHeader(rec.Headers)
	name := rec.Name
	if name == "" && m != nil {
		name = httpdl.Sanitize(m.Name)
	}
	pick := func(name string) {
		name = httpdl.Unique(rec.Dir, name, e.nameTaken(rec.Dir, t.ID))
		t.Name = name
		t.Path = filepath.Join(rec.Dir, name)
		if t.Title == "" || t.Title == provisionalTitle(t.URL) {
			t.Title = name
		}
	}
	dst := ""
	if rec.Name != "" {
		dst = filepath.Join(rec.Dir, rec.Name)
	} else if name != "" {
		e.with(t, func() { pick(name); dst = t.Path })
	}
	// the first answer names the file when nothing did yet, before any byte lands
	named := func(info *httpdl.Info) (string, error) {
		path := dst
		e.with(t, func() {
			if info.Size >= 0 {
				t.Size = info.Size
			}
			if info.Mime != "" {
				t.Mime = info.Mime
			}
			t.Resumable = info.Ranges
			if path == "" {
				pick(info.Name)
				path = t.Path
			}
		})
		return path, nil
	}
	e.setState(t, proto.StateActive, "fetch")
	part := plan.Part{Kind: plan.PartHTTP, URL: link, Headers: headers, Role: "file"}
	final, err := e.dl.FetchFile(ctx, part, dst, named, t.prog.single())
	if err != nil {
		return err
	}
	e.with(t, func() {
		t.Path = final
		t.Name = filepath.Base(final)
	})
	return nil
}

func finalName(pl *plan.Plan) string {
	n := httpdl.Sanitize(pl.Name)
	if n == "" {
		n = "download"
	}
	if c := strings.TrimPrefix(pl.Container, "."); c != "" && filepath.Ext(n) == "" {
		n += "." + c
	}
	return n
}

func planSig(pl *plan.Plan) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s", pl.Name, pl.Container, pl.Post)
	for _, p := range pl.Parts {
		fmt.Fprintf(h, "|%s,%s,%s,%v", p.Kind, p.Role, p.Ext, p.Args)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (e *Engine) runMedia(ctx context.Context, t *task, rec record, m *media.Media) error {
	s := e.Settings()
	opts := media.Options{Mode: rec.Mode, Quality: s.VideoQuality, AudioFormat: s.AudioFormat, Items: rec.Sel, CoverArt: s.CoverArt}
	if rec.Format != nil {
		opts.Format = rec.Format.ID
	}
	pl, err := e.parts.Resolver.Plan(m, opts)
	if err != nil {
		if items := pageItems(m, rec, err); len(items) > 0 {
			return e.expand(ctx, t, items)
		}
		return err
	}
	if len(pl.Parts) == 0 {
		return perr(proto.CodeUnavailable, i18n.T("Nothing to download in this post."))
	}
	folder := slices.ContainsFunc(pl.Parts, func(p plan.Part) bool { return p.Role == "item" })
	name := finalName(pl)
	if folder {
		if name = httpdl.Sanitize(pl.Name); name == "" {
			name = "download"
		}
	}
	e.with(t, func() {
		t.Name = name
		t.Path = filepath.Join(rec.Dir, name)
	})
	tmp := filepath.Join(rec.Dir, tempPrefix+t.ID)
	sig := planSig(pl)
	sigFile := filepath.Join(tmp, "plan")
	if old, err := os.ReadFile(sigFile); err != nil || string(old) != sig {
		_ = os.RemoveAll(tmp)
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(sigFile, []byte(sig), 0o644); err != nil {
		return err
	}
	hints := make([]int64, len(pl.Parts))
	for i, p := range pl.Parts {
		hints[i] = -1
		if p.Size > 0 {
			hints[i] = p.Size
		}
	}
	pps := t.prog.setParts(hints)
	e.setState(t, proto.StateActive, "fetch")
	inputs := make([]string, 0, len(pl.Parts))
	for i, part := range pl.Parts {
		ext := strings.TrimPrefix(part.Ext, ".")
		if ext == "" {
			ext = "bin"
		}
		dst := filepath.Join(tmp, fmt.Sprintf("part%d.%s", i, ext))
		if folder {
			item := httpdl.Sanitize(part.Name)
			if item == "" {
				item = fmt.Sprintf("%02d.%s", i+1, ext)
			}
			dst = filepath.Join(tmp, "items", item)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
		}
		inputs = append(inputs, dst)
		if fi, err := os.Stat(dst); err == nil && fi.Mode().IsRegular() {
			pps[i].complete(fi.Size())
			continue
		}
		f := e.parts.Fetchers[part.Kind]
		if f == nil {
			return perr(proto.CodeUnsupported, i18n.T("This kind of stream cannot be downloaded yet."))
		}
		if err := f.Fetch(ctx, part, dst, pps[i]); err != nil {
			// the cover picture is a nicety, the audio lands without it
			if part.Role == "cover" && ctx.Err() == nil {
				_ = os.Remove(dst)
				pps[i].complete(0)
				inputs = inputs[:len(inputs)-1]
				pl.Parts = slices.Delete(slices.Clone(pl.Parts), i, i+1)
				continue
			}
			return err
		}
	}
	if folder {
		return e.publishFolder(t, rec, name, filepath.Join(tmp, "items"), tmp)
	}
	src := inputs[0]
	if pl.Post != plan.PostNone || len(inputs) > 1 {
		if e.parts.Finisher == nil {
			return perr(proto.CodeUnsupported, i18n.T("Joining streams needs FFmpeg."))
		}
		stage := "merge"
		if len(inputs) == 1 {
			stage = "remux"
		}
		e.setState(t, proto.StateMerging, stage)
		out := filepath.Join(tmp, "out."+strings.TrimPrefix(filepath.Ext(name), "."))
		_ = os.Remove(out)
		if err := e.parts.Finisher.Finish(ctx, pl, inputs, out); err != nil {
			return err
		}
		src = out
	}
	final, err := httpdl.Publish(src, filepath.Join(rec.Dir, name))
	if err != nil {
		return err
	}
	_ = os.RemoveAll(tmp)
	e.with(t, func() {
		t.Path = final
		t.Name = filepath.Base(final)
		if fi, err := os.Stat(final); err == nil {
			t.Size = fi.Size()
		}
	})
	return nil
}

// publishFolder moves a finished multi-item post out of the temp dir in one
// rename, under a folder name nothing else uses.
func (e *Engine) publishFolder(t *task, rec record, name, items, tmp string) error {
	e.mu.Lock()
	final := filepath.Join(rec.Dir, httpdl.Unique(rec.Dir, name, e.nameTaken(rec.Dir, t.ID)))
	e.mu.Unlock()
	if err := os.Rename(items, final); err != nil {
		return err
	}
	_ = os.RemoveAll(tmp)
	var size int64
	_ = filepath.WalkDir(final, func(_ string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				size += fi.Size()
			}
		}
		return nil
	})
	e.with(t, func() {
		t.Path = final
		t.Name = filepath.Base(final)
		t.Size = size
	})
	return nil
}

const torrentPoll = 500 * time.Millisecond

func (e *Engine) runTorrent(ctx context.Context, t *task, rec record) error {
	tc, err := e.torrentClient()
	if err != nil {
		return err
	}
	src := rec.FetchURL
	if src == "" {
		src = rec.URL
	}
	e.setState(t, proto.StateActive, "metadata")
	tor, err := tc.Add(ctx, src, rec.Dir, rec.Sel)
	if err != nil {
		return err
	}
	tor.Resume()
	e.with(t, func() { t.tor = tor })
	select {
	case <-tor.Metadata():
	case <-ctx.Done():
		return e.torrentStop(t, tor, ctx.Err())
	}
	e.with(t, func() {
		if n := tor.Name(); n != "" {
			if t.Title == "" || t.Title == provisionalTitle(t.URL) || t.Title == t.URL {
				t.Title = n
			}
			t.Name = n
		}
		if p := tor.Path(); p != "" {
			t.Path = p
		}
		if files := tor.Files(); len(files) > 0 {
			t.Files = files
		}
	})
	e.setState(t, proto.StateActive, "fetch")
	completed := false
	for {
		if err := tor.Err(); err != nil {
			return e.torrentStop(t, tor, err)
		}
		var compC, finC <-chan struct{}
		if completed {
			finC = tor.Finished()
		} else {
			compC = tor.Completed()
		}
		select {
		case <-ctx.Done():
			return e.torrentStop(t, tor, ctx.Err())
		case <-compC:
			completed = true
			if e.Settings().Seed {
				e.setState(t, proto.StateSeeding, "seed")
				e.mu.Lock()
				e.schedule()
				e.mu.Unlock()
			}
		case <-finC:
			_ = tor.Drop(false)
			e.with(t, func() { t.tor = nil })
			return nil
		case <-time.After(torrentPoll):
			if completed && !tor.Stats().Complete {
				// a wider file selection made it incomplete again
				completed = false
				e.setState(t, proto.StateActive, "fetch")
			}
		}
	}
}

// torrentStop settles the torrent the way the stop was meant: a pause keeps it
// in the client, a cancel drops its data, a removal drops what it should.
func (e *Engine) torrentStop(t *task, tor Torrent, err error) error {
	e.mu.Lock()
	want, del, state := t.want, t.delFile, t.State
	e.mu.Unlock()
	switch want {
	case wantStopped:
		_ = tor.Drop(true)
		e.with(t, func() { t.tor = nil })
	case wantRemoved:
		_ = tor.Drop(del || state != proto.StateSeeding)
		e.with(t, func() { t.tor = nil })
	default:
		tor.Pause()
	}
	return err
}

// Resolve answers the bar's paste card.
func (e *Engine) Resolve(ctx context.Context, raw string) (proto.Resolved, error) {
	kind, link, err := classify(strings.TrimSpace(raw))
	if err != nil {
		return proto.Resolved{}, err
	}
	if kind == "torrent" {
		return e.resolveTorrent(ctx, link)
	}
	m, err := e.parts.Resolver.Resolve(media.Preview(ctx), link)
	if err != nil {
		return proto.Resolved{}, mapErr(err)
	}
	if m.Kind == plan.KindTorrent {
		src := m.URL
		if src == "" {
			src = link
		}
		r, err := e.resolveTorrent(ctx, src)
		if err == nil {
			r.URL = link
		}
		return r, err
	}
	thumb := ""
	if m.Thumb != "" && e.parts.Thumb != nil {
		thumb = e.parts.Thumb(ctx, m.Thumb)
	}
	return toResolved(link, m, thumb), nil
}

func (e *Engine) resolveTorrent(ctx context.Context, link string) (proto.Resolved, error) {
	r := proto.Resolved{
		Kind:      "torrent",
		URL:       link,
		Source:    proto.Source{ID: "torrent", Name: "BitTorrent"},
		Title:     provisionalTitle(link),
		Size:      -1,
		Resumable: true,
		Formats:   []proto.Format{},
		Items:     []proto.Item{},
		Files:     []proto.File{},
	}
	e.mu.Lock()
	e.torResolving++
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.torResolving--
		stale := e.releaseTorrents()
		e.mu.Unlock()
		if stale != nil {
			_ = stale.Close()
		}
	}()
	tc, err := e.torrentClient()
	if err != nil {
		return r, err
	}
	name, files, err := tc.Resolve(ctx, link)
	if err != nil {
		return r, mapErr(err)
	}
	if name != "" {
		r.Title, r.Name = name, name
	}
	if len(files) > 0 {
		r.Files = files
		r.Size = 0
		for _, f := range files {
			r.Size += f.Size
		}
	}
	return r, nil
}

func toResolved(link string, m *media.Media, thumb string) proto.Resolved {
	r := proto.Resolved{
		Kind:          string(m.Kind),
		URL:           link,
		Source:        proto.Source{ID: m.Source.ID, Name: m.Source.Name},
		Title:         m.Title,
		Name:          m.Name,
		Size:          m.Size,
		Mime:          m.Mime,
		Thumb:         thumb,
		Duration:      m.Duration,
		Author:        m.Author,
		Resumable:     m.Resumable,
		Formats:       make([]proto.Format, 0, len(m.Formats)),
		Items:         make([]proto.Item, 0, len(m.Items)),
		Files:         []proto.File{},
		DefaultFormat: m.Default,
		Note:          m.Note,
	}
	if r.Title == "" {
		r.Title = m.Name
	}
	for _, f := range m.Formats {
		r.Formats = append(r.Formats, proto.Format{
			ID: f.ID, Kind: f.Kind, Label: f.Label, Height: f.Height, FPS: f.FPS,
			Bitrate: f.Bitrate, Ext: f.Ext, Size: f.Size, Codec: f.Codec,
		})
	}
	for _, it := range m.Items {
		r.Items = append(r.Items, proto.Item{Title: it.Title, Thumb: it.Thumb, Kind: it.Kind, URL: it.URL})
	}
	return r
}

// mapErr turns whatever a run failed with into the wire's error codes.
func mapErr(err error) *proto.Error {
	var pe *proto.Error
	if errors.As(err, &pe) {
		return pe
	}
	msg := err.Error()
	for _, c := range []struct {
		err  error
		code string
	}{
		{media.ErrUnsupported, proto.CodeUnsupported},
		{media.ErrUnavailable, proto.CodeUnavailable},
		{media.ErrPrivate, proto.CodePrivate},
		{media.ErrRateLimited, proto.CodeRateLimited},
		{media.ErrNetwork, proto.CodeNetwork},
		{media.ErrInvalid, proto.CodeInvalid},
	} {
		if errors.Is(err, c.err) {
			// the resolvers wrap as "%w: text"; the code already says the first half
			if text, ok := strings.CutPrefix(msg, c.err.Error()+": "); ok && text != "" {
				msg = text
			}
			return perr(c.code, msg)
		}
	}
	var se *httpdl.StatusError
	if errors.As(err, &se) {
		switch se.Code {
		case http.StatusNotFound, http.StatusGone:
			return perr(proto.CodeUnavailable, msg)
		case http.StatusUnauthorized, http.StatusForbidden:
			return perr(proto.CodePrivate, msg)
		case http.StatusTooManyRequests:
			return perr(proto.CodeRateLimited, msg)
		}
		return perr(proto.CodeNetwork, msg)
	}
	var ne net.Error
	var ue *url.Error
	if errors.As(err, &ne) || errors.As(err, &ue) || errors.Is(err, context.DeadlineExceeded) {
		return perr(proto.CodeNetwork, msg)
	}
	return perr(proto.CodeInternal, msg)
}

// directResolver treats every web link as one file; it stands in until the
// media resolvers are wired.
type directResolver struct{ client *plan.Env }

func (d directResolver) Resolve(ctx context.Context, link string) (*media.Media, error) {
	info, err := httpdl.Probe(ctx, d.client.HTTP, link, nil)
	if err != nil {
		return nil, err
	}
	host := ""
	if u, err := url.Parse(link); err == nil {
		host = u.Hostname()
	}
	return &media.Media{
		Kind:      plan.KindFile,
		URL:       link,
		Source:    media.Source{ID: "file", Name: host},
		Title:     info.Name,
		Name:      info.Name,
		Size:      info.Size,
		Mime:      info.Mime,
		Resumable: info.Ranges,
	}, nil
}

func (directResolver) Plan(m *media.Media, _ media.Options) (*plan.Plan, error) {
	return &plan.Plan{Name: m.Name, Parts: []plan.Part{{Kind: plan.PartHTTP, URL: m.URL, Size: m.Size, Role: "file"}}}, nil
}
