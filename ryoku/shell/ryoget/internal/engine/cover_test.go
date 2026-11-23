package engine

import (
	"context"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

type coverResolver struct {
	base  string
	cover atomic.Bool
}

func (r *coverResolver) Resolve(_ context.Context, link string) (*media.Media, error) {
	return &media.Media{Kind: plan.KindMedia, URL: link, Title: "Song", Source: media.Source{ID: "fake", Name: "Fake"},
		Formats: []media.Format{{ID: "a", Label: "128 kbps"}}}, nil
}

func (r *coverResolver) Plan(m *media.Media, o media.Options) (*plan.Plan, error) {
	r.cover.Store(o.CoverArt)
	return &plan.Plan{Name: "Song", Container: "m4a", Post: plan.PostAudio, Parts: []plan.Part{
		{Kind: plan.PartHLS, URL: "hls://a", Ext: "m4a", Role: "audio"},
		{Kind: plan.PartHTTP, URL: r.base + "/missing-cover.jpg", Ext: "cover", Role: "cover"},
	}}, nil
}

type planFinisher struct{ parts, inputs atomic.Int32 }

func (f *planFinisher) Finish(ctx context.Context, pl *plan.Plan, inputs []string, out string) error {
	f.parts.Store(int32(len(pl.Parts)))
	f.inputs.Store(int32(len(inputs)))
	return catFinisher{}.Finish(ctx, pl, inputs, out)
}

func TestMediaCoverIsOptional(t *testing.T) {
	g := newGated()
	g.open()
	srv := httptest.NewServer(g)
	defer srv.Close()
	res := &coverResolver{base: srv.URL}
	fin := &planFinisher{}
	f := newFixture(t, map[string]any{"coverArt": true}, &Parts{
		Resolver: res,
		Fetchers: map[plan.PartKind]Fetcher{plan.PartHLS: fakeHLS{}},
		Finisher: fin,
	})
	tk, err := f.e.Add(proto.AddParams{URL: "https://fake.test/song", Mode: "audio"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitTask(t, f.e, tk.ID, inState(proto.StateDone, proto.StateError))
	if done.State != proto.StateDone {
		t.Fatalf("a missing cover failed the download: %+v", done.Error)
	}
	if !res.cover.Load() {
		t.Fatal("the coverArt setting did not reach the planner")
	}
	if fin.parts.Load() != 1 || fin.inputs.Load() != 1 {
		t.Fatalf("finish saw %d parts and %d inputs, want the audio alone", fin.parts.Load(), fin.inputs.Load())
	}
	if got, _ := os.ReadFile(done.Path); string(got) != "AUDIOAUDIO" {
		t.Fatalf("file = %q", got)
	}
}
