//go:build live

package media

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

// The ROM pages hand out the file through script behind an invisible reCAPTCHA.
func TestLiveROMPagesNeedBrowser(t *testing.T) {
	env := &plan.Env{HTTP: &http.Client{Timeout: 45 * time.Second}}
	for _, raw := range []string{
		"https://www.romsgames.net/gameboy-rom-pokemon-blue-version/",
		"https://www.romsgames.net/gameboy-rom-mario-yoshi/",
	} {
		m, err := Resolve(t.Context(), raw, env, false)
		var pe *proto.Error
		if !errors.Is(err, ErrBrowserCheck) || !errors.As(err, &pe) || pe.Code != proto.CodeBrowserCheck {
			t.Fatalf("%s: %+v, %v", raw, m, err)
		}
		t.Logf("%s: %q, %s", raw, m.Title, pe.Message)
	}
}

// Reddit answers anonymous post page loads with a script check.
func TestLiveRedditPostNeedsBrowser(t *testing.T) {
	env := &plan.Env{HTTP: &http.Client{Timeout: 45 * time.Second}}
	_, err := Resolve(t.Context(), "https://www.reddit.com/r/videos/comments/6rrwyj/that_small_heart_attack/", env, false)
	if !errors.Is(err, ErrBrowserCheck) {
		t.Fatalf("err = %v", err)
	}
}
