package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// `ryoku doctor --explain` = the reasoning layer over the deterministic
// reconcilers. ships the diagnostic report to the user's own cloud model,
// prints back the root-cause analysis + fix steps. strictly advisory and
// read-only: it never runs anything, so a wrong answer can only mislead, not
// break the box (cognition stays separate from actuation). provider, key, and
// model are the user's; nothing leaves the box without a key.
//
// any OpenAI-compatible chat endpoint works. defaults aim at Groq (fast, free
// tier); OpenRouter's free models work via RYOKU_AI_URL + RYOKU_AI_MODEL.

const (
	defaultAIEndpoint = "https://api.groq.com/openai/v1"
	defaultAIModel    = "llama-3.3-70b-versatile"
)

// aiSystemPrompt frames the system for the model, naming the compositor from the
// detected provider so the advice fits the box actually running. short on
// purpose -- the report is the data.
func aiSystemPrompt() string {
	comp := "a Wayland compositor"
	if n := wm.Detect().Name; n != "" {
		comp = "the " + n + " Wayland compositor"
	}

	app := host.Default()
	manager, _ := app.PackageManager()
	if manager == host.XBPS {
		initSystem, _ := app.Init()
		initDescription := string(initSystem)
		if initSystem == host.Runit {
			initDescription = "runit and Turnstile"
		}
		return "You are the diagnostic brain for Ryoku on Void Linux with XBPS, " + initDescription +
			", a btrfs root protected by snapper and limine-snapper-sync, with snapshots restored from the Limine boot menu, running " +
			comp + ". The `ryoku` CLI manages updates and config. " +
			"You are given a `ryoku doctor` report: deterministic findings plus system state (packages, " +
			"services, journal errors, and hardware: GPU and backlight). Name the single most likely root " +
			"cause and give the exact, safe fix, preferring precise shell commands. When the cause is hardware, " +
			"firmware, or BIOS, say so plainly: software cannot fix it, so tell the user what to change. Never give a " +
			"destructive command without a clear warning. Be brief: lead with the cause, then the fix."
	}

	return "You are the diagnostic brain for Ryoku, an Arch-based Linux distro running " + comp +
		" (btrfs root with snapper; the `ryoku` CLI manages updates, snapshots, and " +
		"config). You are given a `ryoku doctor` report: deterministic findings plus system state (btrfs, " +
		"packages, services, journal errors, and hardware: GPU and backlight). Name the single most likely root " +
		"cause and give the exact, safe fix, preferring precise shell commands. When the cause is hardware, " +
		"firmware, or BIOS, say so plainly: software cannot fix it, so tell the user what to change. Never give a " +
		"destructive command without a clear warning. Be brief: lead with the cause, then the fix."
}

func aiKey() string {
	if k := strings.TrimSpace(os.Getenv("RYOKU_AI_KEY")); k != "" {
		return k
	}
	b, err := os.ReadFile(filepath.Join(sys.Xdg("XDG_CONFIG_HOME", ".config"), "ryoku", "ai-key"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func explainSetupHelp() {
	p := func(f string, a ...any) { fmt.Printf(f+"\n", a...) }
	fmt.Println()
	p(i18n.T("  %s reasons over the report with a cloud model."), sys.Brand("ryoku doctor --explain"))
	p("  %s", sys.Dim(i18n.T("It needs a free API key (your own, opt-in: nothing is sent without it).")))
	fmt.Println()
	p("  %s %s", sys.Bold("Groq"), sys.Dim(i18n.T("(recommended, fast and free)")))
	p(i18n.T("    1. get a key:  %s"), sys.Brand("https://console.groq.com/keys"))
	p("    2. export RYOKU_AI_KEY=...   %s", sys.Dim(i18n.T("or write it to ~/.config/ryoku/ai-key")))
	fmt.Println()
	p("  %s %s", sys.Bold("OpenRouter"), sys.Dim(i18n.T("(free models)")))
	p("    export RYOKU_AI_KEY=...      %s", sys.Dim(i18n.T("from https://openrouter.ai/keys")))
	p("    export RYOKU_AI_URL=https://openrouter.ai/api/v1")
	p("    export RYOKU_AI_MODEL=meta-llama/llama-3.3-70b-instruct:free")
	fmt.Println()
	p("  %s", sys.Dim(i18n.Tf("Default model: %s (override with RYOKU_AI_MODEL).", defaultAIModel)))
}

func explainFindings(findings []finding) error {
	key := aiKey()
	if key == "" {
		explainSetupHelp()
		if path, err := writeReport("", findings); err == nil {
			fmt.Printf(i18n.T("\nThe full report is at %s; you can also paste it into any assistant yourself.\n"), path)
		}
		return nil
	}
	endpoint := envOr("RYOKU_AI_URL", defaultAIEndpoint)
	model := envOr("RYOKU_AI_MODEL", defaultAIModel)
	report := gatherReport(findings)

	fmt.Fprintf(os.Stderr, i18n.T("\n  %s asking %s to diagnose %s\n"), sys.Brand("➜"), sys.Bold(model), sys.Dim(i18n.T("(advisory, read-only; nothing runs)")))
	answer, err := aiDiagnose(endpoint, key, model, report)
	if err != nil {
		path, _ := writeReport("", findings)
		return fmt.Errorf(i18n.T("AI request failed: %v\n    The report is at %s; paste it into any assistant."), err, path)
	}
	fmt.Printf("\n%s\n\n%s\n", sys.Brand("➜ "+i18n.T("AI diagnosis")), answer)
	return nil
}

type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiRequest struct {
	Model    string      `json:"model"`
	Messages []aiMessage `json:"messages"`
}

type aiResponse struct {
	Choices []struct {
		Message aiMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// aiDiagnose POSTs the report to an OpenAI-compatible chat-completions
// endpoint and returns the assistant's text. OpenAI's wire format is the
// de-facto standard, so the same code serves Groq, OpenRouter, and friends.
func aiDiagnose(endpoint, key, model, report string) (string, error) {
	body, err := json.Marshal(aiRequest{
		Model: model,
		Messages: []aiMessage{
			{Role: "system", Content: aiSystemPrompt()},
			{Role: "user", Content: "Diagnose this machine and tell me how to fix it:\n\n" + report},
		},
	})
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var ar aiResponse
	if err := json.Unmarshal(raw, &ar); err != nil {
		return "", fmt.Errorf(i18n.T("unexpected response (HTTP %d)"), resp.StatusCode)
	}
	if ar.Error != nil && ar.Error.Message != "" {
		return "", fmt.Errorf("%s", ar.Error.Message)
	}
	if len(ar.Choices) == 0 || strings.TrimSpace(ar.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf(i18n.T("no answer returned (HTTP %d)"), resp.StatusCode)
	}
	return strings.TrimSpace(ar.Choices[0].Message.Content), nil
}
