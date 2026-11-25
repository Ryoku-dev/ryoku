package consent

import "testing"

func TestClassifyAppSortsEveryStep(t *testing.T) {
	win := AppSubject{App: "org.gnome.TextEditor", Title: "notes.txt"}
	with := func(f func(*AppSubject)) AppSubject {
		s := win
		f(&s)
		return s
	}
	cases := []struct {
		name string
		verb string
		s    AppSubject
		want Class
		why  Reason
	}{
		{"type a line", VerbType, with(func(s *AppSubject) { s.Text = "hello" }), Safe, ReasonSafe},
		{"type two lines", VerbType, with(func(s *AppSubject) { s.Text = "hello\nworld" }), Handoff, ReasonSubmits},
		{"type a tab", VerbType, with(func(s *AppSubject) { s.Text = "a\tb" }), Refused, ReasonControlText},
		{"type a secret", VerbType, with(func(s *AppSubject) { s.Text = "4111 1111 1111 1111" }), Refused, ReasonSecret},
		{"type a code", VerbType, with(func(s *AppSubject) { s.Text = "482913" }), Refused, ReasonSecret},
		{"press Tab", VerbPress, with(func(s *AppSubject) { s.Keys = "Tab" }), Safe, ReasonSafe},
		{"press undo", VerbPress, with(func(s *AppSubject) { s.Keys = "ctrl+z" }), Safe, ReasonSafe},
		{"press Enter", VerbPress, with(func(s *AppSubject) { s.Keys = "Return" }), Handoff, ReasonSubmits},
		{"press ctrl+Enter", VerbPress, with(func(s *AppSubject) { s.Keys = "ctrl+Return" }), Handoff, ReasonSubmits},
		{"press Delete", VerbPress, with(func(s *AppSubject) { s.Keys = "Delete" }), Handoff, ReasonRiskyKey},
		{"press close", VerbPress, with(func(s *AppSubject) { s.Keys = "ctrl+w" }), Handoff, ReasonRiskyKey},
		{"press quit", VerbPress, with(func(s *AppSubject) { s.Keys = "ctrl+q" }), Handoff, ReasonRiskyKey},
		{"press alt+F4", VerbPress, with(func(s *AppSubject) { s.Keys = "alt+F4" }), Handoff, ReasonRiskyKey},
		{"press paste", VerbPress, with(func(s *AppSubject) { s.Keys = "ctrl+v" }), Refused, ReasonRefusedKey},
		{"press super", VerbPress, with(func(s *AppSubject) { s.Keys = "logo+d" }), Refused, ReasonRefusedKey},
		{"press nonsense", VerbPress, with(func(s *AppSubject) { s.Keys = "frob" }), Refused, ReasonRefusedKey},
		{"click Next", VerbClick, with(func(s *AppSubject) { s.Near = "Next" }), Safe, ReasonSafe},
		{"click Send", VerbClick, with(func(s *AppSubject) { s.Near = "Send" }), Handoff, ReasonRiskyLabel},
		{"click Delete all", VerbClick, with(func(s *AppSubject) { s.Near = "Delete all" }), Handoff, ReasonRiskyLabel},
		{"click nothing read", VerbClick, with(func(s *AppSubject) { s.Near = "" }), Handoff, ReasonUnreadable},
		{"click an icon", VerbClick, with(func(s *AppSubject) { s.Near = "→ ×" }), Handoff, ReasonUnreadable},
		{"click a PIN field", VerbClick, with(func(s *AppSubject) { s.Near = "Enter PIN" }), Refused, ReasonPrivate},
		{"scroll", VerbScroll, win, Safe, ReasonSafe},
		{"scroll by a password", VerbScroll, with(func(s *AppSubject) { s.Near = "Password" }), Refused, ReasonPrivate},
		{"wait", VerbWait, win, Safe, ReasonSafe},
		{"find files", VerbFindFiles, win, Safe, ReasonSafe},
		{"show a file", VerbShowFile, win, Safe, ReasonSafe},
		{"open an editor", VerbOpenApp, with(func(s *AppSubject) { s.Name = "Text Editor" }), Safe, ReasonSafe},
		{"open a password manager", VerbOpenApp, with(func(s *AppSubject) { s.Name = "KeePassXC" }), Refused, ReasonPrivateApp},
		{"open Hyakki", VerbOpenApp, with(func(s *AppSubject) { s.Name = "dev.ryoku.Hyakki" }), Refused, ReasonSelf},
		{"open an uninstaller", VerbOpenApp, with(func(s *AppSubject) { s.Name = "Uninstall Tool" }), Handoff, ReasonRiskyLabel},
		{"open by symbol", VerbOpenApp, with(func(s *AppSubject) { s.Name = "终端" }), Handoff, ReasonUnreadable},
		{"drag", "drag", win, Handoff, ReasonUnknownVerb},
		{"fill is a tab verb", VerbFill, win, Handoff, ReasonUnknownVerb},
	}
	for _, tc := range cases {
		c, r := ClassifyApp(tc.verb, tc.s, ClaimNormal)
		if c != tc.want || r != tc.why {
			t.Errorf("%s: ClassifyApp = %s %s, want %s %s", tc.name, c, r, tc.want, tc.why)
		}
	}
}

// Three layers keep Hands out of private surfaces: the window, the words
// read at the point or beside the focus, and the model's claim, which only
// raises.
func TestPrivateSurfacesAreRefusedAtEveryLayer(t *testing.T) {
	verbs := []string{VerbClick, VerbType, VerbPress, VerbScroll, VerbWait, VerbOpenApp, VerbFindFiles, VerbShowFile}
	surfaces := []struct {
		name string
		s    AppSubject
		why  Reason
	}{
		{"self", AppSubject{App: "dev.ryoku.Hyakki"}, ReasonSelf},
		{"password manager", AppSubject{App: "org.keepassxc.KeePassXC"}, ReasonPrivateApp},
		{"polkit agent", AppSubject{App: "polkit-gnome-authentication-agent-1"}, ReasonPrivateApp},
		{"lock screen", AppSubject{App: "hyprlock"}, ReasonPrivateApp},
		{"title names a password", AppSubject{App: "firefox", Title: "Enter your password"}, ReasonPrivate},
		{"executor found a private field", AppSubject{App: "firefox", Private: true}, ReasonPrivate},
	}
	for _, sf := range surfaces {
		for _, v := range verbs {
			s := sf.s
			s.Text, s.Keys, s.Near, s.Name = "hello", "Tab", "Next", "Files"
			for _, claim := range []string{ClaimNormal, ClaimCritical, "low"} {
				if c, r := ClassifyApp(v, s, claim); c != Refused || r != sf.why {
					t.Errorf("%s/%s/%s: %s %s, want refused %s", sf.name, v, claim, c, r, sf.why)
				}
			}
		}
	}
	safe := AppSubject{App: "org.gnome.TextEditor", Text: "hello", Keys: "Tab", Near: "Next", Name: "Files"}
	for _, v := range verbs {
		for _, claim := range []string{ClaimSensitive, ClaimCritical, "", "safe"} {
			if c, r := ClassifyApp(v, safe, claim); c != Handoff || r != ReasonClaim {
				t.Errorf("%s claimed %q: %s %s, want a handoff", v, claim, c, r)
			}
		}
		if c, _ := ClassifyApp(v, safe, ClaimNormal); c != Safe {
			t.Errorf("%s: %s, want safe", v, c)
		}
	}
}
