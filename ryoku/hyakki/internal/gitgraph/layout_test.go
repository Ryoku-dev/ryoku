package gitgraph

import (
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func commit(hash string, parents ...string) protocol.GitCommit {
	return protocol.GitCommit{Hash: hash, Parents: parents}
}

func TestLinearHistoryUsesOneLane(t *testing.T) {
	layout := New()
	rows := layout.Append([]protocol.GitCommit{
		commit("a", "b"),
		commit("b", "c"),
		commit("c"),
	})

	if len(rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rows))
	}
	for index, row := range rows {
		if row.Lane != 0 || row.Color != 0 || row.Width != 1 {
			t.Errorf("row %d position = lane %d color %d width %d, want 0, 0, 1", index, row.Lane, row.Color, row.Width)
		}
	}
	if len(rows[0].Above) != 0 {
		t.Errorf("tip above = %#v, want empty", rows[0].Above)
	}
	if !reflect.DeepEqual(rows[1].Above, []Segment{{From: 0, To: 0, Color: 0, Kind: Straight}}) {
		t.Errorf("middle above = %#v", rows[1].Above)
	}
	if !rows[2].IsRoot || len(rows[2].Segments) != 0 {
		t.Errorf("root = %#v, want root with no lower segments", rows[2])
	}
	if layout.Lanes() != 0 {
		t.Errorf("boundary lanes = %d, want 0", layout.Lanes())
	}
}

func TestSimpleMergeOpensAndConvergesSideLane(t *testing.T) {
	rows := New().Append([]protocol.GitCommit{
		commit("m", "a", "b"),
		commit("a", "c"),
		commit("b", "c"),
		commit("c"),
	})

	wantMerge := []Segment{
		{From: 0, To: 0, Color: 0, Kind: Straight},
		{From: 0, To: 1, Color: 1, Kind: MergeOut},
	}
	if !reflect.DeepEqual(rows[0].Segments, wantMerge) {
		t.Errorf("merge segments = %#v, want %#v", rows[0].Segments, wantMerge)
	}
	if !rows[0].IsMerge || rows[0].Width != 2 {
		t.Errorf("merge row = %#v", rows[0])
	}
	wantConvergence := []Segment{
		{From: 0, To: 0, Color: 0, Kind: Straight},
		{From: 1, To: 0, Color: 1, Kind: ConvergeIn},
	}
	if !reflect.DeepEqual(rows[2].Segments, wantConvergence) {
		t.Errorf("convergence segments = %#v, want %#v", rows[2].Segments, wantConvergence)
	}
}

func TestOctopusMergeOpensEveryExtraParent(t *testing.T) {
	rows := New().Append([]protocol.GitCommit{
		commit("m", "a", "b", "c"),
		commit("a"),
		commit("b"),
		commit("c"),
	})

	want := []Segment{
		{From: 0, To: 0, Color: 0, Kind: Straight},
		{From: 0, To: 1, Color: 1, Kind: MergeOut},
		{From: 0, To: 2, Color: 2, Kind: MergeOut},
	}
	if !reflect.DeepEqual(rows[0].Segments, want) {
		t.Errorf("octopus segments = %#v, want %#v", rows[0].Segments, want)
	}
	if rows[0].Width != 3 || !rows[0].IsMerge {
		t.Errorf("octopus row = %#v", rows[0])
	}
}

func TestSharedParentConvergesToLeftmostLane(t *testing.T) {
	rows := New().Append([]protocol.GitCommit{
		commit("a", "p"),
		commit("b", "p"),
		commit("p"),
	})

	parent := rows[2]
	if parent.Lane != 0 || parent.Color != 0 {
		t.Errorf("parent position = lane %d color %d, want leftmost lane and its color", parent.Lane, parent.Color)
	}
	want := []Segment{
		{From: 0, To: 0, Color: 0, Kind: Straight},
		{From: 1, To: 0, Color: 1, Kind: ConvergeIn},
	}
	if !reflect.DeepEqual(rows[1].Segments, want) {
		t.Errorf("branch convergence = %#v, want %#v", rows[1].Segments, want)
	}
}

func TestFirstParentStaysLeftWhenSideBranchIsVisitedFirst(t *testing.T) {
	rows := New().Append([]protocol.GitCommit{
		commit("c", "merge"),
		commit("merge", "b", "f"),
		commit("f", "a"),
		commit("b", "a"),
		commit("a"),
	})

	wantLanes := []int{0, 0, 1, 0, 0}
	for index, want := range wantLanes {
		if rows[index].Lane != want {
			t.Errorf("row %s lane = %d, want %d", rows[index].Commit, rows[index].Lane, want)
		}
	}
	wantConvergence := []Segment{
		{From: 0, To: 0, Color: 0, Kind: Straight},
		{From: 1, To: 0, Color: 1, Kind: ConvergeIn},
	}
	if !reflect.DeepEqual(rows[3].Segments, wantConvergence) {
		t.Errorf("first-parent convergence = %#v, want %#v", rows[3].Segments, wantConvergence)
	}
	if !rows[4].IsRoot || rows[4].Lane != 0 {
		t.Errorf("root row = %#v, want root in lane 0", rows[4])
	}
}

func TestPageSplitIsStable(t *testing.T) {
	commits := []protocol.GitCommit{
		commit("m", "a", "b"),
		commit("a", "c"),
		commit("b", "c"),
		commit("c", "d"),
		commit("d"),
	}
	whole := New().Append(commits)

	pagedLayout := New()
	paged := append(pagedLayout.Append(commits[:2]), pagedLayout.Append(commits[2:])...)
	if !reflect.DeepEqual(paged, whole) {
		t.Errorf("paged rows differ\npaged: %#v\nwhole: %#v", paged, whole)
	}
}

func TestColorsAreDeterministic(t *testing.T) {
	commits := []protocol.GitCommit{
		commit("m", "a", "b", "c"),
		commit("x", "y"),
		commit("a"),
		commit("b"),
		commit("c"),
		commit("y"),
	}
	first := New().Append(commits)
	second := New().Append(commits)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("same input produced different layouts\nfirst: %#v\nsecond: %#v", first, second)
	}
}

func TestFreedLaneIsReusedLowestFirst(t *testing.T) {
	rows := New().Append([]protocol.GitCommit{
		commit("m", "a", "b"),
		commit("b"),
		commit("x", "y"),
		commit("a"),
		commit("y"),
	})

	if rows[2].Lane != 1 {
		t.Errorf("new tip lane = %d, want freed lane 1", rows[2].Lane)
	}
	if rows[2].Color != 2 {
		t.Errorf("new tip color = %d, want next deterministic color 2", rows[2].Color)
	}
}

func TestMissingParentRemainsAtBoundary(t *testing.T) {
	layout := New()
	rows := layout.Append([]protocol.GitCommit{commit("a", "missing")})
	if layout.Lanes() != 1 {
		t.Fatalf("boundary lanes = %d, want 1", layout.Lanes())
	}

	continued := layout.Append([]protocol.GitCommit{commit("missing")})
	if continued[0].Lane != rows[0].Lane || continued[0].Color != rows[0].Color {
		t.Errorf("continued parent = %#v, want lane %d color %d", continued[0], rows[0].Lane, rows[0].Color)
	}
	if layout.Lanes() != 0 {
		t.Errorf("boundary lanes after root = %d, want 0", layout.Lanes())
	}
}

func TestCloneCopiesBoundaryIndependently(t *testing.T) {
	layout := New()
	layout.Append([]protocol.GitCommit{commit("a", "p")})
	clone := layout.Clone()

	fromOriginal := layout.Append([]protocol.GitCommit{commit("p")})
	fromClone := clone.Append([]protocol.GitCommit{commit("p", "q")})
	if fromOriginal[0].Lane != fromClone[0].Lane || fromOriginal[0].Color != fromClone[0].Color {
		t.Errorf("clone did not preserve boundary: original %#v clone %#v", fromOriginal[0], fromClone[0])
	}
	if layout.Lanes() != 0 || clone.Lanes() != 1 {
		t.Errorf("clone states are not independent: original %d clone %d", layout.Lanes(), clone.Lanes())
	}
}
