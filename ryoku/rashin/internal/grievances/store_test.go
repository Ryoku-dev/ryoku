package grievances

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestStoreRemainsClosedUntilFirstReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "reports.db")
	s := Open(path)
	defer s.Close()
	if _, err := s.List(context.Background(), protocol.LearningGrievancesParams{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(context.Background(), protocol.GrievancesClearParams{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("empty list created database: %v", err)
	}
}

func TestGrievanceAddListClearAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reports.db")
	ctx := context.Background()
	s := Open(path)
	first, err := s.Add(ctx, protocol.Grievance{Tool: "read", Report: "Returned an unrelated file"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Add(ctx, protocol.Grievance{Tool: "edit", Report: "Wrong line range"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == 0 || second.ID <= first.ID || first.At.IsZero() {
		t.Fatalf("bad IDs or clock: %#v %#v", first, second)
	}
	listed, err := s.List(ctx, protocol.LearningGrievancesParams{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if listed.Total != 2 || len(listed.Grievances) != 1 || listed.Grievances[0].ID != second.ID || len(listed.Tools) != 2 {
		t.Fatalf("wrong listing: %#v", listed)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = Open(path)
	defer s.Close()
	if err := s.Clear(ctx, protocol.GrievancesClearParams{IDs: []int64{first.ID}}); err != nil {
		t.Fatal(err)
	}
	remaining, err := s.List(ctx, protocol.LearningGrievancesParams{})
	if err != nil {
		t.Fatal(err)
	}
	if remaining.Total != 1 || remaining.Grievances[0].ID != second.ID {
		t.Fatalf("wrong remaining reports: %#v", remaining)
	}
	if err := s.Clear(ctx, protocol.GrievancesClearParams{}); err != nil {
		t.Fatal(err)
	}
	empty, err := s.List(ctx, protocol.LearningGrievancesParams{})
	if err != nil || empty.Total != 0 {
		t.Fatalf("clear all: %#v %v", empty, err)
	}
}

func TestNormalizeBoundsAndRemovesHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	tool, report := Normalize("read", strings.Repeat("x", MaxReportBytes)+" "+home+"/secret")
	if tool != "read" || len(report) > MaxReportBytes {
		t.Fatalf("normalize bounds: %q len %d", tool, len(report))
	}
	if home != "" && strings.Contains(Redact(home+"/secret"), home) {
		t.Fatal("home leaked from redaction")
	}
}
