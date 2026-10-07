package main

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// prowl.go surfaces Prowl's code-intelligence state for Rashin's local tools.
// Query data comes from the gateway so the console, quick lane, and harnesses
// all observe the same live index.

type ProwlReport struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Repo      string `json:"repo,omitempty"`
	Indexed   bool   `json:"indexed"`
	Files     int    `json:"files,omitempty"`
	Symbols   int    `json:"symbols,omitempty"`
	Doctor    *struct {
		Errors int `json:"errors"`
		Warns  int `json:"warns"`
		Infos  int `json:"infos"`
	} `json:"doctor,omitempty"`
	Hotspots []ProwlHotspot `json:"hotspots,omitempty"`
	Savings  *ProwlSavings  `json:"savings,omitempty"`
}

// ProwlSavings is what the index actually bought this machine: measured
// answer tokens against the reading cost they replaced. It is the number the
// dashboard leads with, because files and symbols are trivia next to it.
type ProwlSavings struct {
	Queries      int   `json:"queries"`
	AnswerTokens int64 `json:"answerTokens"`
	SavedTokens  int64 `json:"savedTokens"`
}

type ProwlHotspot struct {
	File string `json:"file"`
	In   int    `json:"in"`
}

type ProwlHit struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// findProwl resolves the shipped Prowl binary. RYOKU_PROWL_BIN lets tests and
// development deployments choose an explicit build.
func findProwl() (string, bool) {
	if v := os.Getenv("RYOKU_PROWL_BIN"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v, true
		}
	}
	if p, err := exec.LookPath("prowl"); err == nil {
		return p, true
	}
	return "", false
}

// prowlRepo picks the repo the code index answers on: a dev checkout that
// carries a .prowl index (the deploy-recorded checkout, honouring
// RYOKU_RASHIN_REPO and ~/.local/state/ryoku/repo, then the conventional
// locations), else the vault's config mirror when it carries one. So the prowl
// MCP server and search_code work on a packaged box with no checkout too.
func prowlRepo() string {
	cands := []string{}
	if repo := recordedCheckout(); repo != "" {
		cands = append(cands, repo)
	}
	cands = append(cands,
		filepath.Join(home(), "Work", "ryoku-arch"),
		filepath.Join(home(), "ryoku-arch"),
	)
	for _, cand := range cands {
		if dirExists(filepath.Join(cand, ".prowl")) {
			return cand
		}
	}
	if m := sourceMirrorDir(); dirExists(filepath.Join(m, ".prowl")) {
		return m
	}
	return ""
}

// prowlMCPServers exposes prowl's code intelligence to the hermes session
// as an MCP server, but only when prowl and an indexed Ryoku checkout are both
// present (otherwise the session opens with no extra servers, as before). The
// server runs in the repo so it finds the .prowl index; the agent then gets
// find_symbol, find_references, blast_radius and the rest over MCP.
func prowlMCPServers() []any {
	bin, ok := findProwl()
	repo := prowlRepo()
	if !ok || repo == "" {
		return []any{}
	}
	return []any{map[string]any{
		"name":    "prowl",
		"command": "sh",
		"args": []any{"-c",
			"cd " + shQuote(repo) + " && exec " + shQuote(bin) + " serve --mcp-surface core"},
		"env": []any{},
	}}
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func prowlVersion(bin string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(firstLine(string(out)), "prowl "))
}

// Prowl reports are cached because several console panels poll the same state.
var prowlCache struct {
	mu   sync.Mutex
	at   time.Time
	data ProwlReport
}

func ProwlReportNow() ProwlReport {
	prowlCache.mu.Lock()
	defer prowlCache.mu.Unlock()
	if time.Since(prowlCache.at) < 60*time.Second {
		return prowlCache.data
	}
	rep := buildProwlReport()
	prowlCache.at, prowlCache.data = time.Now(), rep
	return rep
}

func buildProwlReport() ProwlReport {
	var rep ProwlReport
	bin, ok := findProwl()
	if !ok {
		return rep
	}
	rep.Installed = true
	rep.Version = prowlVersion(bin)
	repo := prowlRepo()
	if repo == "" {
		return rep
	}
	rep.Repo = repo

	query := url.Values{"repo": []string{repo}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var status struct {
		Counts struct {
			Files   int `json:"files"`
			Symbols int `json:"symbols"`
		} `json:"counts"`
		Savings struct {
			Queries      int64 `json:"queries"`
			AnswerTokens int64 `json:"answer_tokens"`
			SavedTokens  int64 `json:"saved_tokens"`
		} `json:"savings"`
	}
	if err := prowlGatewayJSON(ctx, http.MethodGet, "/api/code/status?"+query.Encode(), nil, &status); err != nil {
		return rep
	}
	rep.Files = status.Counts.Files
	rep.Symbols = status.Counts.Symbols
	rep.Indexed = rep.Files > 0
	if status.Savings.Queries > 0 || status.Savings.SavedTokens > 0 {
		rep.Savings = &ProwlSavings{
			Queries:      int(status.Savings.Queries),
			AnswerTokens: status.Savings.AnswerTokens,
			SavedTokens:  status.Savings.SavedTokens,
		}
	}
	if !rep.Indexed {
		return rep
	}

	var overview struct {
		Hotspots []ProwlHotspot `json:"hotspots"`
	}
	if err := prowlGatewayJSON(ctx, http.MethodGet, "/api/code/overview?"+query.Encode(), nil, &overview); err == nil {
		if len(overview.Hotspots) > 5 {
			overview.Hotspots = overview.Hotspots[:5]
		}
		rep.Hotspots = overview.Hotspots
	}

	var doctor struct {
		Findings []struct {
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	if err := prowlGatewayJSON(ctx, http.MethodGet, "/api/code/doctor?"+query.Encode(), nil, &doctor); err == nil {
		counts := &struct {
			Errors int `json:"errors"`
			Warns  int `json:"warns"`
			Infos  int `json:"infos"`
		}{}
		for _, finding := range doctor.Findings {
			switch strings.ToLower(finding.Severity) {
			case "error":
				counts.Errors++
			case "warn", "warning":
				counts.Warns++
			default:
				counts.Infos++
			}
		}
		rep.Doctor = counts
	}
	return rep
}

// ProwlSearch runs a compact content search over the indexed repo.
func ProwlSearch(query string) []ProwlHit {
	repo := prowlRepo()
	if repo == "" || strings.TrimSpace(query) == "" {
		return nil
	}
	params := url.Values{
		"q":     []string{query},
		"repo":  []string{repo},
		"limit": []string{"20"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var hits []struct {
		File      string `json:"file"`
		StartLine int    `json:"start_line"`
		Snippet   string `json:"snippet"`
	}
	if err := prowlGatewayJSON(ctx, http.MethodGet, "/api/code/search?"+params.Encode(), nil, &hits); err != nil {
		return nil
	}
	res := make([]ProwlHit, 0, len(hits))
	for _, hit := range hits {
		text := strings.TrimSpace(hit.Snippet)
		if len(text) > 160 {
			text = text[:160]
		}
		res = append(res, ProwlHit{File: hit.File, Line: hit.StartLine, Text: text})
	}
	return res
}
