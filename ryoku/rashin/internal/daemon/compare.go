package daemon

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

type compareGroups struct {
	mu     sync.Mutex
	groups map[string]compareGroup
}

type compareGroup struct {
	result        protocol.CompareStartResult
	sourceSeq     int64
	replaceSource bool
	keeping       bool
}

func (d *Daemon) closeComparisons(source string) {
	d.comparisons.mu.Lock()
	var groups []compareGroup
	for id, group := range d.comparisons.groups {
		if source == "" || group.result.Source == source {
			groups = append(groups, group)
			delete(d.comparisons.groups, id)
		}
	}
	d.comparisons.mu.Unlock()
	for _, group := range groups {
		for _, lane := range group.result.Lanes {
			_ = d.manager.delete(lane.Session)
		}
		if remaining, err := d.manager.get(group.result.Source); err == nil {
			_ = d.manager.persist(remaining, protocol.EvCompare, protocol.CompareStartResult{Source: group.result.Source, Seq: group.result.Seq})
		}
	}
}

func (d *Daemon) compareStart(ctx context.Context, c *connection, p protocol.CompareStartParams) (protocol.CompareStartResult, error) {
	if !d.feature("compare") {
		return protocol.CompareStartResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Turn on Compare models in Settings first."))
	}
	models := make([]string, 0, 2)
	for _, model := range p.Models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		duplicate := false
		for _, kept := range models {
			duplicate = duplicate || model == kept
		}
		if !duplicate {
			models = append(models, model)
		}
	}
	if len(models) < 1 || len(models) > 2 {
		return protocol.CompareStartResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose one or two other models."))
	}
	source, err := d.manager.get(p.ID)
	if err != nil {
		return protocol.CompareStartResult{}, err
	}
	info := source.log.Info()
	if info.Kind != protocol.KindAsk {
		return protocol.CompareStartResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose an Ask conversation to compare."))
	}
	for _, model := range models {
		if model == info.Model {
			return protocol.CompareStartResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a different model from the original answer."))
		}
	}
	events, err := source.log.Since(0)
	if err != nil {
		return protocol.CompareStartResult{}, err
	}
	userAt, endAt, turns := -1, -1, 0
	var user protocol.UserData
	for index, event := range events {
		if event.Seq > p.Seq {
			break
		}
		switch event.Kind {
		case protocol.EvUser:
			userAt = index
			if err := json.Unmarshal(event.Data, &user); err != nil {
				return protocol.CompareStartResult{}, err
			}
		case protocol.EvTurnStart:
			turns++
		case protocol.EvTurnEnd:
			if event.Seq == p.Seq {
				endAt = index
			}
		}
	}
	if userAt < 0 || endAt < userAt || user.Run != "" {
		return protocol.CompareStartResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a finished answer to compare."))
	}
	result := protocol.CompareStartResult{Source: info.ID, Seq: p.Seq}
	cleanup := func() {
		for _, lane := range result.Lanes {
			_ = d.manager.delete(lane.Session)
		}
	}
	for index, model := range append([]string{info.Model}, models...) {
		harness := ""
		if index == 0 {
			harness = info.Harness
		}
		lane, err := d.manager.newSession(ctx, protocol.SessionNewParams{
			Kind: protocol.KindAsk, Harness: harness, Model: model, Title: info.Title, Origin: "compare",
			Temporary: true, Mode: protocol.ApprovalsReadOnly, Memory: info.Memory,
		})
		if err != nil {
			cleanup()
			return protocol.CompareStartResult{}, err
		}
		result.Lanes = append(result.Lanes, protocol.CompareLane{Session: lane.ID, Model: model})
		target, err := d.manager.get(lane.ID)
		if err != nil {
			cleanup()
			return protocol.CompareStartResult{}, err
		}
		until, count := userAt, turns-1
		if index == 0 {
			until, count = endAt+1, turns
		}
		for _, event := range events[:until] {
			if err := target.log.AppendCopy(event); err != nil {
				cleanup()
				return protocol.CompareStartResult{}, err
			}
		}
		if err := target.log.Update(func(i *protocol.SessionInfo) {
			i.Parent, i.Pinned, i.Turns = info.ID, true, count
		}, nil); err != nil {
			cleanup()
			return protocol.CompareStartResult{}, err
		}
	}
	result.ID = result.Lanes[0].Session
	d.comparisons.mu.Lock()
	source.mu.Lock()
	stopping := source.stopping
	source.mu.Unlock()
	_, exists := d.manager.Info(info.ID)
	if !d.feature("compare") || stopping || !exists {
		d.comparisons.mu.Unlock()
		cleanup()
		return protocol.CompareStartResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This comparison has ended."))
	}
	if d.comparisons.groups == nil {
		d.comparisons.groups = make(map[string]compareGroup)
	}
	// a second comparison of this answer replaces its unused temporary lanes
	var replaced []protocol.CompareLane
	for id, group := range d.comparisons.groups {
		if group.result.Source == info.ID && group.result.Seq == p.Seq {
			replaced = append(replaced, group.result.Lanes...)
			delete(d.comparisons.groups, id)
		}
	}
	d.comparisons.groups[result.ID] = compareGroup{result: result, sourceSeq: info.LastSeq}
	d.comparisons.mu.Unlock()
	for _, lane := range replaced {
		_ = d.manager.delete(lane.Session)
	}
	for _, lane := range result.Lanes[1:] {
		if _, _, err := d.manager.prompt(ctx, protocol.PromptParams{ID: lane.Session, Text: user.Text, Images: user.Images}, nil); err != nil {
			d.comparisons.mu.Lock()
			delete(d.comparisons.groups, result.ID)
			d.comparisons.mu.Unlock()
			cleanup()
			return protocol.CompareStartResult{}, err
		}
	}
	if err := d.manager.persist(source, protocol.EvCompare, result); err != nil {
		d.comparisons.mu.Lock()
		delete(d.comparisons.groups, result.ID)
		d.comparisons.mu.Unlock()
		cleanup()
		return protocol.CompareStartResult{}, err
	}
	recent, err := source.log.Since(p.Seq)
	if err != nil {
		d.comparisons.mu.Lock()
		delete(d.comparisons.groups, result.ID)
		d.comparisons.mu.Unlock()
		cleanup()
		return protocol.CompareStartResult{}, err
	}
	replaceSource, sourceSeq := true, p.Seq
	for _, event := range recent {
		replaceSource = replaceSource && event.Kind != protocol.EvUser && event.Kind != protocol.EvTurnStart
		sourceSeq = event.Seq
	}
	d.comparisons.mu.Lock()
	if _, exists := d.comparisons.groups[result.ID]; !exists || !d.feature("compare") {
		d.comparisons.mu.Unlock()
		cleanup()
		return protocol.CompareStartResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This comparison has ended."))
	}
	d.comparisons.groups[result.ID] = compareGroup{result: result, sourceSeq: sourceSeq, replaceSource: replaceSource}
	d.comparisons.mu.Unlock()
	return result, nil
}

func (d *Daemon) compareKeep(ctx context.Context, c *connection, p protocol.CompareKeepParams) (protocol.SessionInfo, error) {
	if !d.feature("compare") {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Turn on Compare models in Settings first."))
	}
	d.comparisons.mu.Lock()
	group, ok := d.comparisons.groups[p.ID]
	if !ok {
		d.comparisons.mu.Unlock()
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This comparison has ended."))
	}
	if group.keeping {
		d.comparisons.mu.Unlock()
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("An answer from this comparison is already being kept."))
	}
	group.keeping = true
	d.comparisons.groups[p.ID] = group
	d.comparisons.mu.Unlock()
	defer func() {
		d.comparisons.mu.Lock()
		if remaining, exists := d.comparisons.groups[p.ID]; exists {
			remaining.keeping = false
			d.comparisons.groups[p.ID] = remaining
		}
		d.comparisons.mu.Unlock()
	}()
	found := false
	for _, lane := range group.result.Lanes {
		found = found || lane.Session == p.Session
	}
	if !found {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose an answer from this comparison."))
	}
	target, err := d.manager.get(p.Session)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	target.mu.Lock()
	busy := target.running || target.finishing || target.starting
	seq := target.log.Info().LastSeq
	target.mu.Unlock()
	if busy {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Wait for this answer to finish before keeping it."))
	}
	kept, err := d.manager.fork(ctx, protocol.ForkParams{ID: p.Session, Seq: seq})
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	keptSession, err := d.manager.get(kept.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	if err := keptSession.log.Update(func(i *protocol.SessionInfo) {
		i.Origin, i.Parent, i.Updated = "app", "", time.Now()
	}, nil); err != nil {
		_ = d.manager.delete(kept.ID)
		return protocol.SessionInfo{}, err
	}
	d.comparisons.mu.Lock()
	_, exists := d.comparisons.groups[p.ID]
	valid := exists && d.feature("compare")
	if valid {
		delete(d.comparisons.groups, p.ID)
	}
	d.comparisons.mu.Unlock()
	if !valid {
		_ = d.manager.delete(kept.ID)
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This comparison has ended."))
	}
	for _, lane := range group.result.Lanes {
		if err := d.manager.delete(lane.Session); err != nil {
			return protocol.SessionInfo{}, err
		}
	}
	// later work in the source conversation must never disappear with a comparison
	if source, exists := d.manager.Info(group.result.Source); exists {
		if source.LastSeq == group.sourceSeq && group.replaceSource {
			if err := d.manager.delete(source.ID); err != nil {
				return protocol.SessionInfo{}, err
			}
		} else if remaining, err := d.manager.get(source.ID); err == nil {
			if err := d.manager.persist(remaining, protocol.EvCompare, protocol.CompareStartResult{Source: source.ID, Seq: group.result.Seq}); err != nil {
				return protocol.SessionInfo{}, err
			}
		}
	}
	return keptSession.log.Info(), nil
}
