package demo

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

func compareAnswerEvents(question, answer, model string) []protocol.SessionEvent {
	return []protocol.SessionEvent{
		event(1, protocol.EvUser, protocol.UserData{Text: question}),
		event(2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}),
		event(3, protocol.EvText, protocol.TextData{Message: "answer-1", Delta: answer}),
		event(4, protocol.EvAnswer, protocol.AnswerData{Via: model}),
		event(5, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}),
	}
}

func addCompareFixtures(f *fixtures) {
	question := i18n.T("What should I measure before changing the desktop shell?")
	answers := []string{
		i18n.T("## Start with a baseline\n\nMeasure idle processor use, memory and frame time with the same windows open. Change one thing, repeat the measurement, and keep the version that feels better without spending more at rest."),
		i18n.T("## Follow the frames\n\n1. Record frame times while opening a menu and scrolling.\n2. Compare the slowest frames, not just the average.\n3. Leave the pointer still and check whether the shell keeps drawing.\n\nA smooth desktop should also know when to stop."),
		i18n.T("## Keep the workload fixed\n\n| Measure | Why it matters |\n| --- | --- |\n| Frame time | Shows pauses during interaction |\n| Idle wakeups | Shows background work |\n| Memory | Shows the cost of keeping it ready |\n\nUse the same palette, outputs and power profile for both runs."),
	}
	models := []string{protocol.RouteAuto, protocol.RouteFast, protocol.RouteSmart}
	source := protocol.SessionInfo{ID: "ask-compare", Kind: protocol.KindAsk, Model: models[0], Title: i18n.T("Compare desktop measurements"), Origin: protocol.OriginApp, State: protocol.StateIdle, Mode: protocol.ApprovalsReadOnly, Memory: protocol.MemoryNone, Created: Epoch, Updated: Epoch, Turns: 1, LastSeq: 6}
	result := protocol.CompareStartResult{ID: "compare-demo", Source: source.ID, Seq: 5}
	f.events[source.ID] = compareAnswerEvents(question, answers[0], models[0])
	for index, model := range models {
		lane := source
		lane.ID, lane.Model, lane.Parent, lane.Origin, lane.Temporary, lane.LastSeq = fmt.Sprintf("ask-compare-lane-%d", index+1), model, source.ID, "compare", true, 5
		f.snapshot.Sessions = append(f.snapshot.Sessions, lane)
		f.events[lane.ID] = compareAnswerEvents(question, answers[index], model)
		result.Lanes = append(result.Lanes, protocol.CompareLane{Session: lane.ID, Model: model})
	}
	f.events[source.ID] = append(f.events[source.ID], event(6, protocol.EvCompare, result))
	f.snapshot.Sessions = append(f.snapshot.Sessions, source)
}

func (s *server) callCompare(method string, raw json.RawMessage) (any, bool, error) {
	if method != protocol.MCompareStart && method != protocol.MCompareKeep {
		return nil, false, nil
	}
	if !s.data.snapshot.Settings.Features["compare"] {
		return nil, true, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Turn on Compare models in Settings first."))
	}
	if method == protocol.MCompareKeep {
		var p protocol.CompareKeepParams
		if err := decode(raw, &p); err != nil {
			return nil, true, err
		}
		for source, events := range s.data.events {
			for _, compared := range events {
				if compared.Kind != protocol.EvCompare {
					continue
				}
				var result protocol.CompareStartResult
				if json.Unmarshal(compared.Data, &result) != nil || result.ID != p.ID {
					continue
				}
				if !slices.ContainsFunc(result.Lanes, func(lane protocol.CompareLane) bool { return lane.Session == p.Session }) {
					return nil, true, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose an answer from this comparison."))
				}
				info, ok := s.session(p.Session)
				if !ok {
					return nil, true, notFound("comparison")
				}
				kept := s.newSession(protocol.KindAsk, "", "", "", "", "", "", "")
				kept.Title, kept.Model, kept.Memory, kept.Mode, kept.LastSeq, kept.Turns = info.Title, info.Model, info.Memory, info.Mode, info.LastSeq, info.Turns
				s.data.events[kept.ID] = slices.Clone(s.data.events[p.Session])
				s.replaceSession(kept)
				for _, lane := range result.Lanes {
					s.deleteSession(lane.Session)
				}
				hasLaterWork := slices.ContainsFunc(events, func(event protocol.SessionEvent) bool {
					return event.Seq > result.Seq && event.Kind != protocol.EvCompare
				})
				if !hasLaterWork {
					s.deleteSession(source)
				} else {
					s.compareTell(source, protocol.CompareStartResult{Source: source, Seq: result.Seq})
				}
				return kept, true, nil
			}
		}
		return nil, true, notFound("comparison")
	}
	var p protocol.CompareStartParams
	if err := decode(raw, &p); err != nil {
		return nil, true, err
	}
	info, ok := s.session(p.ID)
	if !ok {
		return nil, true, notFound("session")
	}
	models := []string{}
	for _, model := range p.Models {
		if model = strings.TrimSpace(model); model != "" && !slices.Contains(models, model) {
			models = append(models, model)
		}
	}
	if info.Kind != protocol.KindAsk || len(models) < 1 || len(models) > 2 || slices.Contains(models, info.Model) {
		return nil, true, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose one or two other models for this Ask answer."))
	}
	events := s.data.events[p.ID]
	userAt, endAt := -1, -1
	var user protocol.UserData
	for index, item := range events {
		if item.Seq > p.Seq {
			break
		}
		if item.Kind == protocol.EvUser {
			userAt = index
			_ = json.Unmarshal(item.Data, &user)
		}
		if item.Kind == protocol.EvTurnEnd && item.Seq == p.Seq {
			endAt = index
		}
	}
	if userAt < 0 || endAt < userAt || user.Run != "" {
		return nil, true, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a finished answer to compare."))
	}
	for _, previous := range events {
		if previous.Kind == protocol.EvCompare {
			var old protocol.CompareStartResult
			if json.Unmarshal(previous.Data, &old) == nil && old.Source == p.ID && old.Seq == p.Seq {
				for _, lane := range old.Lanes {
					s.deleteSession(lane.Session)
				}
			}
		}
	}
	result := protocol.CompareStartResult{Source: info.ID, Seq: p.Seq}
	for index, model := range append([]string{info.Model}, models...) {
		lane := s.newSession(protocol.KindAsk, "", "", "", "", "", "", info.ID)
		lane.Title, lane.Model, lane.Temporary, lane.Memory, lane.Mode = info.Title, model, true, protocol.MemoryNone, protocol.ApprovalsReadOnly
		lane.Origin = "compare"
		if index == 0 {
			lane.Harness = info.Harness
		}
		copied := slices.Clone(events[:endAt+1])
		if index > 0 {
			copied = slices.Clone(events[:userAt])
			answer := i18n.T("Start by measuring the current behaviour, change one thing, then repeat the same workload. Keep your measurements alongside the result so you can explain what improved.")
			for _, next := range compareAnswerEvents(user.Text, answer, model) {
				next.Seq = int64(len(copied) + 1)
				copied = append(copied, next)
			}
		}
		lane.LastSeq, lane.Turns = int64(len(copied)), info.Turns
		s.data.events[lane.ID] = copied
		s.replaceSession(lane)
		result.Lanes = append(result.Lanes, protocol.CompareLane{Session: lane.ID, Model: model})
	}
	result.ID = result.Lanes[0].Session
	s.compareTell(info.ID, result)
	return result, true, nil
}

func (s *server) compareTell(source string, result protocol.CompareStartResult) {
	events := s.data.events[source]
	next := event(int64(len(events)+1), protocol.EvCompare, result)
	s.data.events[source] = append(events, next)
	if info, ok := s.session(source); ok {
		info.LastSeq = next.Seq
		s.replaceSession(info)
	}
	s.told = append(s.told, protocol.SessionEventData{Session: source, SessionEvent: next})
}
