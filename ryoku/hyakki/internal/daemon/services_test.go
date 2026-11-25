package daemon

import (
	"context"
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

type serviceLocal struct {
	core.Local
	downloads int
	scans     int
	links     int
}

func (*serviceLocal) Models(context.Context) protocol.LocalResult { return protocol.LocalResult{} }
func (l *serviceLocal) Download(context.Context, string) error {
	l.downloads++
	return nil
}
func (l *serviceLocal) Found(context.Context, []string) ([]protocol.LocalFoundModel, error) {
	l.scans++
	return []protocol.LocalFoundModel{{Path: "/home/test/model.gguf"}}, nil
}
func (l *serviceLocal) UseFound(path string) (protocol.LocalModel, error) {
	l.links++
	return protocol.LocalModel{Path: path}, nil
}

type serviceGateway struct {
	core.Gateway
	probes int
}

func (g *serviceGateway) Probe(context.Context, protocol.ModelProbeParams) (protocol.ModelProbeResult, error) {
	g.probes++
	return protocol.ModelProbeResult{OK: true}, nil
}

func TestModelProbeAndLocalDownloadsOnlyRunForTheApp(t *testing.T) {
	isolateDaemon(t)
	local := &serviceLocal{}
	gateway := &serviceGateway{}
	d := New(Deps{Gateway: gateway, Local: local})
	ctx := context.Background()
	probe, _ := json.Marshal(protocol.ModelProbeParams{Model: protocol.RouteAuto})
	pull, _ := json.Marshal(protocol.LocalModelParams{ID: "test"})
	link, _ := json.Marshal(protocol.LocalUseFoundParams{Path: "/home/test/model.gguf"})
	for _, call := range []struct {
		method string
		raw    json.RawMessage
	}{
		{protocol.MModelsProbe, probe},
		{protocol.MLocalDownload, pull},
		{protocol.MLocalFound, json.RawMessage(`{}`)},
		{protocol.MLocalUseFound, link},
	} {
		if _, err := d.dispatch(ctx, &connection{}, call.method, call.raw); !forbidden(err) {
			t.Fatalf("%s from CLI = %v, want forbidden", call.method, err)
		}
	}
	if local.downloads != 0 || gateway.probes != 0 || local.scans != 0 || local.links != 0 {
		t.Fatal("an unauthorized service call had an effect")
	}
	for _, call := range []struct {
		method string
		raw    json.RawMessage
	}{
		{protocol.MModelsProbe, probe},
		{protocol.MLocalDownload, pull},
		{protocol.MLocalFound, json.RawMessage(`{}`)},
		{protocol.MLocalUseFound, link},
	} {
		if _, err := d.dispatch(ctx, &connection{app: true}, call.method, call.raw); err != nil {
			t.Fatalf("%s from app: %v", call.method, err)
		}
	}
	if local.downloads != 1 || gateway.probes != 1 || local.scans != 1 || local.links != 1 {
		t.Fatalf("app requests: downloads %d, probes %d, scans %d, links %d", local.downloads, gateway.probes, local.scans, local.links)
	}
}

type serviceSkills struct{ deletes int }

func (*serviceSkills) List(context.Context) (protocol.LearningSkillsResult, error) {
	return protocol.LearningSkillsResult{}, nil
}
func (s *serviceSkills) Delete(context.Context, string) (protocol.LearningSkillsResult, error) {
	s.deletes++
	return protocol.LearningSkillsResult{}, nil
}

func TestLearningDeleteRequiresTheAppAndNotifiesReaders(t *testing.T) {
	isolateDaemon(t)
	skills := &serviceSkills{}
	d := New(Deps{ManagedSkills: skills})
	var events []protocol.LearningChanged
	stop := d.events.add(func(event string, data any) {
		if event == protocol.EvtLearningChanged {
			events = append(events, data.(protocol.LearningChanged))
		}
	})
	defer stop()
	ctx := context.Background()
	raw, _ := json.Marshal(protocol.LearningSkillParams{Name: "obsolete"})
	if _, err := d.dispatch(ctx, &connection{}, protocol.MLearningSkillDelete, raw); !forbidden(err) {
		t.Fatalf("CLI deleting a managed skill: %v", err)
	}
	if skills.deletes != 0 || len(events) != 0 {
		t.Fatal("an unauthorized delete changed managed skills")
	}
	if _, err := d.dispatch(ctx, &connection{app: true}, protocol.MLearningSkillDelete, raw); err != nil {
		t.Fatal(err)
	}
	if skills.deletes != 1 || len(events) != 1 || events[0].Kind != protocol.LearningSkills {
		t.Fatalf("managed skill deletion: calls %d, events %v", skills.deletes, events)
	}
}
