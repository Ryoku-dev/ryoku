package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Cloudflare's bot check answers in front of the gate with a browser page several
// kilobytes long; the reader must hear what happened, not a byte count.
func TestTeamExchangeExplainsPagesFromInFrontOfTheGate(t *testing.T) {
	page := "<!DOCTYPE html><title>Just a moment...</title>" + strings.Repeat("<script>x</script>", 600)
	for _, c := range []struct {
		name      string
		challenge bool
		status    int
		want      string
	}{
		{"bot check", true, http.StatusForbidden, "protection held this request back"},
		{"error page", false, http.StatusServiceUnavailable, "could not be reached (HTTP 503)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if c.challenge {
					w.Header().Set("Cf-Mitigated", "challenge")
				}
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(page))
			}))
			defer srv.Close()
			t.Setenv("RYOKU_TEAM_URL", strings.Replace(srv.URL, "127.0.0.1", "localhost", 1))
			_, err := teamExchange(http.MethodPut, "/v1/cards/0123456789abcdef", "application/json", []byte("{}"), nil, 4096)
			if err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "exceeds") {
				t.Fatalf("error = %v, want one saying %q", err, c.want)
			}
			if c.challenge && !errors.Is(err, errTeamChallenged) {
				t.Fatalf("a bot check is not errTeamChallenged: %v", err)
			}
		})
	}
}
