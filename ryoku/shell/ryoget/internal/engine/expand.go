package engine

import (
	"context"
	"errors"
	"slices"

	"ryoget/internal/media"
	"ryoget/internal/proto"
)

// pageItems returns the selected items of a post when the planner refused them
// because they are pages of their own (a set's tracks, a thread's videos, a
// playlist's entries). nil when the refusal was about something else.
func pageItems(m *media.Media, rec record, err error) []media.Item {
	if m == nil || rec.Expanded || len(m.Items) == 0 || !errors.Is(err, media.ErrUnsupported) {
		return nil
	}
	picked := rec.Sel
	if len(picked) == 0 {
		picked = make([]int, len(m.Items))
		for i := range picked {
			picked[i] = i
		}
	}
	var out []media.Item
	for _, i := range picked {
		if i < 0 || i >= len(m.Items) {
			return nil
		}
		it := m.Items[i]
		// an item that is the pasted link itself would resolve to the same post
		// again; one equal to Media.URL is fine (a playlist's first video is both)
		if it.URL == "" || it.URL == rec.URL {
			return nil
		}
		out = append(out, it)
	}
	return out
}

// expand turns the running task into the first item and queues one task per
// further item right after it, each resolved on its own with the same mode and
// folder; their task events announce them.
func (e *Engine) expand(ctx context.Context, t *task, items []media.Item) error {
	e.mu.Lock()
	at := slices.Index(e.order, t.ID) + 1
	var added []*task
	for _, it := range items[1:] {
		n := &task{record: record{Task: proto.Task{
			ID:      newID(),
			URL:     it.URL,
			Source:  t.Source,
			Kind:    "file",
			Title:   itemTitle(it),
			Dir:     t.Dir,
			Size:    -1,
			ETA:     -1,
			State:   proto.StateQueued,
			Created: now(),
			Pieces:  emptyPieces,
			Files:   []proto.File{},
			Mode:    t.Mode,
		}, Expanded: true, Sort: t.Sort, Referer: it.Referer}}
		e.tasks[n.ID] = n
		added = append(added, n)
	}
	ids := make([]string, len(added))
	for i, n := range added {
		ids[i] = n.ID
	}
	e.order = slices.Insert(e.order, at, ids...)
	first := items[0]
	t.URL, t.Title, t.Expanded = first.URL, itemTitle(first), true
	t.Known, t.Kind, t.Sel, t.Format = false, "file", nil, nil
	t.FetchURL, t.Headers, t.Name, t.Path, t.Thumb, t.Size = "", nil, "", "", "", -1
	t.Referer = first.Referer
	// a checksum named the pasted link's file, not an item's
	t.Checksum = ""
	e.dirty()
	e.emitTask(t)
	for _, n := range added {
		e.emitTask(n)
	}
	e.schedule()
	e.mu.Unlock()
	return e.work(ctx, t)
}

func itemTitle(it media.Item) string {
	if it.Title != "" {
		return it.Title
	}
	return provisionalTitle(it.URL)
}
