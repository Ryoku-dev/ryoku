package doctor

import (
	"strings"
	"testing"
)

// The consumer-visible bug: lib32 packages installed while [multilib] is off
// make every `pacman -Syu` abort with "could not satisfy dependencies" naming
// the lib32 pins (#265). The reconciler must recognize exactly that state and
// heal it by activating the stock stanza, never by editing an active config.

func TestHasMultilibSection(t *testing.T) {
	cases := []struct {
		name string
		conf string
		want bool
	}{
		{"active section", "[options]\nCheckSpace\n\n[multilib]\nInclude = /etc/pacman.d/mirrorlist\n", true},
		{"commented stock stanza is not enabled", "[options]\n#CacheDir = /var/cache/pacman/pkg/\n\n#[multilib]\n#Include = /etc/pacman.d/mirrorlist\n", false},
		{"no section at all", "[options]\nILoveCandy\n\n[core]\nServer = x\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasMultilibSection(c.conf); got != c.want {
				t.Errorf("hasMultilibSection = %v, want %v", got, c.want)
			}
		})
	}
}

func TestEnableMultilibSection(t *testing.T) {
	t.Run("uncomments the stock stanza in place", func(t *testing.T) {
		conf := "[options]\nHoldPkg = pacman\n\n#[multilib]\n#Include = /etc/pacman.d/mirrorlist\n"
		out, ok := enableMultilibSection(conf)
		if !ok {
			t.Fatal("enableMultilibSection refused the stock config")
		}
		if !strings.Contains(out, "\n[multilib]\nInclude = /etc/pacman.d/mirrorlist\n") {
			t.Errorf("stanza not activated:\n%s", out)
		}
		if strings.Contains(out, "#[multilib]") {
			t.Errorf("the commented header survived:\n%s", out)
		}
		if !strings.HasPrefix(out, "[options]\nHoldPkg = pacman") {
			t.Errorf("the rest of the config was rewritten:\n%s", out)
		}
	})

	t.Run("appends when no stanza exists", func(t *testing.T) {
		conf := "[options]\n\n[core]\nServer = x\n"
		out, ok := enableMultilibSection(conf)
		if !ok {
			t.Fatal("enableMultilibSection refused a config without the stanza")
		}
		if !hasMultilibSection(out) {
			t.Errorf("the appended config does not parse as enabled:\n%s", out)
		}
		if !strings.HasPrefix(out, "[options]") {
			t.Errorf("the original content was not preserved:\n%s", out)
		}
	})

	t.Run("an empty config is reported, not guessed at", func(t *testing.T) {
		if _, ok := enableMultilibSection("  \n"); ok {
			t.Error("enableMultilibSection accepted an empty config")
		}
	})
}
