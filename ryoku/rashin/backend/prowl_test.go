package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestProwlRepoPrecedence(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_DATA_HOME", filepath.Join(h, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(h, ".local", "state"))
	t.Setenv("RYOKU_RASHIN_VAULT", "") // VaultDir derives from XDG_DATA_HOME
	t.Setenv("RYOKU_RASHIN_REPO", "")

	// No checkout, no mirror: nothing to report.
	if r := prowlRepo(); r != "" {
		t.Fatalf("want empty, got %q", r)
	}

	// A dev checkout that carries an index wins.
	checkout := filepath.Join(h, "checkout")
	if err := os.MkdirAll(filepath.Join(checkout, ".prowl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_RASHIN_REPO", checkout)
	if r := prowlRepo(); r != checkout {
		t.Fatalf("dev checkout should win, got %q", r)
	}

	// With no checkout in the resolution but an indexed mirror, the mirror wins.
	t.Setenv("RYOKU_RASHIN_REPO", "")
	mirror := sourceMirrorDir()
	if err := os.MkdirAll(filepath.Join(mirror, ".prowl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if r := prowlRepo(); r != mirror {
		t.Fatalf("mirror should win with no checkout, got %q", r)
	}
}

func TestFindProwlUsesOverrideAndCurrentBinaryName(t *testing.T) {
	homeDir := isolateProwlGatewayTest(t)
	bin := filepath.Join(homeDir, "custom-prowl")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_PROWL_BIN", bin)
	if got, ok := findProwl(); !ok || got != bin {
		t.Fatalf("findProwl() = %q, %v", got, ok)
	}

	t.Setenv("RYOKU_PROWL_BIN", "")
	pathDir := filepath.Join(homeDir, "path")
	if err := os.MkdirAll(pathDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pathBin := filepath.Join(pathDir, "prowl")
	if err := os.WriteFile(pathBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)
	if got, ok := findProwl(); !ok || got != pathBin {
		t.Fatalf("findProwl() from PATH = %q, %v", got, ok)
	}
}

func TestFindProwlDoesNotResolveAnotherBinaryName(t *testing.T) {
	isolateProwlGatewayTest(t)
	t.Setenv("RYOKU_PROWL_BIN", "")
	t.Setenv("PATH", t.TempDir())
	if got, ok := findProwl(); ok || got != "" {
		t.Fatalf("findProwl() = %q, %v, want not found", got, ok)
	}
}

func TestProwlReportAndSearchUseGatewayCodeRoutes(t *testing.T) {
	homeDir := isolateProwlGatewayTest(t)
	repo := filepath.Join(homeDir, "checkout")
	if err := os.MkdirAll(filepath.Join(repo, ".prowl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_RASHIN_REPO", repo)
	bin := filepath.Join(homeDir, "prowl")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho prowl v9.1.2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_PROWL_BIN", bin)
	writeGatewayToken(t, "token")

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("repo"); got != repo {
			t.Errorf("repo = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/code/status":
			_, _ = w.Write([]byte(`{"counts":{"files":24,"symbols":90},"savings":{"queries":3,"answer_tokens":40,"saved_tokens":900}}`))
		case "/api/code/overview":
			_, _ = w.Write([]byte(`{"hotspots":[{"file":"core.go","in":12}]}`))
		case "/api/code/doctor":
			_, _ = w.Write([]byte(`{"findings":[{"severity":"error"},{"severity":"warning"},{"severity":"info"}]}`))
		case "/api/code/search":
			if r.URL.Query().Get("q") != "needle" {
				t.Errorf("search q = %q", r.URL.Query().Get("q"))
			}
			_, _ = w.Write([]byte(`[{"file":"main.go","start_line":7,"snippet":"matching line"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer gateway.Close()
	pointGatewayAt(t, gateway.URL)

	report := buildProwlReport()
	if !report.Installed || !report.Indexed || report.Files != 24 || report.Symbols != 90 {
		t.Fatalf("report = %+v", report)
	}
	if report.Version != "v9.1.2" || report.Savings == nil || report.Savings.SavedTokens != 900 {
		t.Fatalf("report metadata = %+v", report)
	}
	if report.Doctor == nil || report.Doctor.Errors != 1 || report.Doctor.Warns != 1 || report.Doctor.Infos != 1 {
		t.Fatalf("doctor = %+v", report.Doctor)
	}
	if len(report.Hotspots) != 1 || report.Hotspots[0].File != "core.go" {
		t.Fatalf("hotspots = %+v", report.Hotspots)
	}

	hits := ProwlSearch("needle")
	if len(hits) != 1 || hits[0].File != "main.go" || hits[0].Line != 7 || hits[0].Text != "matching line" {
		t.Fatalf("hits = %+v", hits)
	}
}
