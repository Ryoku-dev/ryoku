package discipline

import (
	"testing"

	"ryoku-rashin/internal/oni/window"
)

func TestPrivateWindows(t *testing.T) {
	cases := []struct {
		app, title string
		want       bool
	}{
		// password managers, secret stores and one-time code apps
		{"org.keepassxc.KeePassXC", "", true},
		{"com.bitwarden.desktop", "", true},
		{"1Password", "", true},
		{"Proton Pass", "", true},
		{"proton-pass", "", true},
		{"org.gnome.World.Secrets", "", true},
		{"org.gnome.seahorse.Application", "", true},
		{"org.kde.kwalletmanager5", "", true},
		{"com.belmoussaoui.Authenticator", "", true},
		// polkit agents and other credential prompts
		{"polkit-gnome-authentication-agent-1", "", true},
		{"org.kde.polkit-kde-authentication-agent-1", "", true},
		{"lxqt-policykit-agent", "", true},
		{"hyprpolkitagent", "", true},
		{"pinentry-qt", "", true},
		{"org.kde.ksshaskpass", "", true},
		{"gcr-prompter", "", true},
		// lock screens
		{"hyprlock", "", true},
		{"swaylock", "", true},
		{"kscreenlocker_greet", "", true},
		// prompts known only by their title
		{"", "Authentication Required", true},
		{"", "Unlock Login Keyring", true},
		{"", "Enter passphrase for key", true},
		{"org.mozilla.firefox", "Change your password - Mozilla Firefox", true},
		// ordinary windows, including one whose id holds "lock"
		{"org.gnome.clocks", "Clocks", false},
		{"org.gnome.TextEditor", "notes", false},
		{"kitty", "shell", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := Private(window.Window{App: c.app, Title: c.title}); got != c.want {
			t.Errorf("Private(%q, %q) = %v, want %v", c.app, c.title, got, c.want)
		}
	}
}

func TestSelfWindows(t *testing.T) {
	cases := []struct {
		app  string
		self []string
		want bool
	}{
		{hyakki, self, true},
		{"dev.ryoku.hyakki", self, true},
		{"dev.ryoku.Hyakki.Settings", self, false},
		{"", self, false},
		{"", []string{""}, false},
		{"org.gnome.TextEditor", nil, false},
		{hyakki, nil, false},
	}
	for _, c := range cases {
		if got := Self(window.Window{App: c.app}, c.self); got != c.want {
			t.Errorf("Self(%q, %q) = %v, want %v", c.app, c.self, got, c.want)
		}
	}
}
