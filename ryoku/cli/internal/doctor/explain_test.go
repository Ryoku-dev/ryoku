package doctor

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	wm "ryoku-wm"
)

func TestAIDiagnoseRoundTrip(t *testing.T) {
	var gotAuth, gotPath, gotModel, gotSystem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		var req aiRequest
		_ = json.Unmarshal(body, &req)
		gotModel = req.Model
		if len(req.Messages) > 0 {
			gotSystem = req.Messages[0].Role
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"Root cause: swap in @. Fix: ryoku doctor."}}]}`)
	}))
	defer srv.Close()

	out, err := aiDiagnose(srv.URL, "test-key", "test-model", "the report")
	if err != nil {
		t.Fatalf("aiDiagnose: %v", err)
	}
	if out != "Root cause: swap in @. Fix: ryoku doctor." {
		t.Errorf("answer = %q", out)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("auth header = %q, want Bearer test-key", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotModel != "test-model" {
		t.Errorf("model = %q, want test-model", gotModel)
	}
	if gotSystem != "system" {
		t.Errorf("first message role = %q, want system (the task framing)", gotSystem)
	}
}

func TestAISystemPromptMatchesHost(t *testing.T) {
	t.Run("Arch wording stays unchanged", func(t *testing.T) {
		t.Setenv("RYOKU_HOST_PKGMGR", "pacman")
		t.Setenv("RYOKU_HOST_INIT", "systemd")

		comp := "a Wayland compositor"
		if n := wm.Detect().Name; n != "" {
			comp = "the " + n + " Wayland compositor"
		}
		want := "You are the diagnostic brain for Ryoku, an Arch-based Linux distro running " + comp +
			" (btrfs root with snapper; the `ryoku` CLI manages updates, snapshots, and " +
			"config). You are given a `ryoku doctor` report: deterministic findings plus system state (btrfs, " +
			"packages, services, journal errors, and hardware: GPU and backlight). Name the single most likely root " +
			"cause and give the exact, safe fix, preferring precise shell commands. When the cause is hardware, " +
			"firmware, or BIOS, say so plainly: software cannot fix it, so tell the user what to change. Never give a " +
			"destructive command without a clear warning. Be brief: lead with the cause, then the fix."
		if got := aiSystemPrompt(); got != want {
			t.Errorf("Arch prompt changed:\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("Void names its real host stack", func(t *testing.T) {
		t.Setenv("RYOKU_HOST_PKGMGR", "xbps")
		t.Setenv("RYOKU_HOST_INIT", "runit")

		got := aiSystemPrompt()
		if !strings.Contains(got, "Void Linux with XBPS, runit and Turnstile, no snapshot integration") {
			t.Errorf("Void prompt does not describe the host stack: %q", got)
		}
		for _, wrong := range []string{"Arch-based", "snapper", "btrfs"} {
			if strings.Contains(got, wrong) {
				t.Errorf("Void prompt must not mention %q: %q", wrong, got)
			}
		}
	})
}

func TestAIDiagnoseAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"Invalid API Key"}}`)
	}))
	defer srv.Close()

	_, err := aiDiagnose(srv.URL, "bad", "m", "r")
	if err == nil || !strings.Contains(err.Error(), "Invalid API Key") {
		t.Errorf("expected the API error surfaced, got %v", err)
	}
}

func TestAIKeyFromEnv(t *testing.T) {
	t.Setenv("RYOKU_AI_KEY", "  sk-xyz  ")
	if got := aiKey(); got != "sk-xyz" {
		t.Errorf("aiKey from env = %q, want sk-xyz (trimmed)", got)
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("RYOKU_AI_URL", "")
	if got := envOr("RYOKU_AI_URL", defaultAIEndpoint); got != defaultAIEndpoint {
		t.Errorf("envOr empty = %q, want default", got)
	}
	t.Setenv("RYOKU_AI_URL", "https://openrouter.ai/api/v1")
	if got := envOr("RYOKU_AI_URL", defaultAIEndpoint); got != "https://openrouter.ai/api/v1" {
		t.Errorf("envOr set = %q", got)
	}
}
