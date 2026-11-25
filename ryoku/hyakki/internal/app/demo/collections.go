package demo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ryoku-rashin/internal/collections"
	"ryoku-rashin/internal/protocol"
)

type collectionFixtures struct {
	list  []protocol.Collection
	items map[string][]protocol.CollectionItem
	next  int
}

func (s *server) callCollections(method string, raw json.RawMessage) (any, bool, error) {
	if !strings.HasPrefix(method, "collections.") {
		return nil, false, nil
	}
	f := s.collections
	if f == nil {
		f = &collectionFixtures{list: []protocol.Collection{{ID: "saved", Name: "Saved", Items: 3, Updated: "2026-11-18"}, {ID: "reading", Name: "Weekend reading", Items: 1, Updated: "2026-11-17"}}, items: map[string][]protocol.CollectionItem{
			"saved": {
				{ID: "saved-answer", Collection: "saved", Kind: "answer", Title: "Why a shell wakes up", Text: "A timer can wake a desktop even when nothing changes. Listen for events and keep animation work tied to visible surfaces.", Source: "theia", Saved: "2026-11-18T10:30:00Z"},
				{ID: "saved-excerpt", Collection: "saved", Kind: "excerpt", Title: "A shorter path to the setting", Text: "Keep related settings together, use the words people search for, and show what a change will do before they make it.", URL: "https://example.com/design-notes", Source: "theia", Saved: "2026-11-17T14:20:00Z", Checked: "2026-11-18T10:00:00Z", Changed: true},
				{ID: "saved-note", Collection: "saved", Kind: "excerpt", Title: "Questions for the next review", Text: "Does this save a step? Can it be understood without a tooltip? What happens with the feature off?", Source: "theia", Saved: "2026-11-16T09:10:00Z"},
			},
			"reading": {{ID: "reading-1", Collection: "reading", Kind: "url", Title: "The shape of a good answer", Text: "Start with the answer, then give the evidence and the next useful step.", URL: "https://example.com/answers", Saved: "2026-11-17T09:00:00Z"}},
		}}
		s.collections = f
	}
	update := func(id string) {
		for i := range f.list {
			if f.list[i].ID == id {
				f.list[i].Items = len(f.items[id])
				f.list[i].Updated = time.Now().UTC().Format(time.RFC3339)
			}
		}
	}
	switch method {
	case protocol.MCollectionsList:
		return protocol.CollectionsListResult{Collections: f.list}, true, nil
	case protocol.MCollectionsSave:
		var p protocol.CollectionSaveParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		if strings.TrimSpace(p.Name) == "" {
			return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Give the collection a name.")
		}
		for i := range f.list {
			if f.list[i].ID == p.ID {
				f.list[i].Name = p.Name
				update(p.ID)
				return f.list[i], true, nil
			}
		}
		f.next++
		c := protocol.Collection{ID: fmt.Sprintf("collection-%d", f.next), Name: p.Name, Updated: time.Now().UTC().Format(time.RFC3339)}
		f.list = append(f.list, c)
		return c, true, nil
	case protocol.MCollectionsAdd:
		var p protocol.CollectionAddParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		if p.Collection == "" {
			p.Collection = "saved"
		}
		f.next++
		item := protocol.CollectionItem{ID: fmt.Sprintf("item-%d", f.next), Collection: p.Collection, Kind: p.Kind, Title: p.Title, Text: p.Text, Path: p.Path, URL: p.URL, Image: p.Image, Source: p.Source, Saved: time.Now().UTC().Format(time.RFC3339)}
		f.items[p.Collection] = append(f.items[p.Collection], item)
		update(p.Collection)
		return item, true, nil
	case protocol.MCollectionsItems:
		var p protocol.CollectionItemsParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		if p.Collection == "" {
			p.Collection = "saved"
		}
		result, err := collections.Page(f.items[p.Collection], p)
		return result, true, err
	case protocol.MCollectionsRemove:
		var p protocol.CollectionRemoveParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		for i, item := range f.items[p.Collection] {
			if item.ID == p.ID {
				f.items[p.Collection] = append(f.items[p.Collection][:i], f.items[p.Collection][i+1:]...)
				update(p.Collection)
				return protocol.Empty{}, true, nil
			}
		}
		return nil, true, notFound("saved item")
	case protocol.MCollectionsCheck:
		var p protocol.CollectionCheckParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		for i := range f.items[p.Collection] {
			item := &f.items[p.Collection][i]
			if item.ID == p.ID {
				item.Checked = time.Now().UTC().Format(time.RFC3339)
				return *item, true, nil
			}
		}
		return nil, true, notFound("saved item")
	case protocol.MCollectionsChat:
		var p protocol.CollectionChatParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		text, attachments := collections.Chat(f.items[p.Collection], p.Question)
		info := s.newSession(protocol.KindAsk, "", "", "", "", "", "", "")
		info.Title, info.Mode = "Reading a collection", protocol.ApprovalsReadOnly
		s.data.events[info.ID] = []protocol.SessionEvent{event(1, protocol.EvUser, protocol.UserData{Text: text, Images: attachments}), event(2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}), event(3, protocol.EvText, protocol.TextData{Message: "collection-answer", Delta: "These notes share a focus on doing less work, making choices clear, and keeping the user's next step close.\n\n**Why a shell wakes up** is about runtime cost. **A shorter path to the setting** is about the cost of finding something. The review questions connect both: keep the useful result, remove the unnecessary work."}), event(4, protocol.EvAnswer, protocol.AnswerData{}), event(5, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1})}
		info.LastSeq = 5
		s.replaceSession(info)
		return protocol.AskResult{Session: info, Turn: 1}, true, nil
	}
	return nil, false, nil
}
