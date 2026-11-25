package reflex

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// catalog is a fake app catalog: exact names and aliases score 1, anything
// else goes through Best over the names.
type catalog struct {
	names   []string
	aliases map[string]string
}

func (c catalog) Resolve(spoken string) (string, string, float64, bool) {
	if n, ok := c.aliases[plain(spoken)]; ok {
		return n, "run-" + plain(n), 1, true
	}
	i, score, ok := Best(spoken, c.names)
	if !ok {
		return "", "", score, false
	}
	return c.names[i], "run-" + plain(c.names[i]), score, true
}

var testCat = catalog{
	names:   []string{"Firefox", "Files", "Terminal", "Calculator", "Calendar", "Text Editor", "YouTuber", "Tables"},
	aliases: map[string]string{"browser": "Firefox", "webbrowser": "Firefox"},
}

var testNow = time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)

type parseCase struct {
	say  string
	ok   bool
	want []Action // Confidence compared only when non-zero
	line string   // "" leaves the line unchecked
}

func a(i Intent, arg string) Action { return Action{Intent: i, Arg: arg} }

var parseCases = []parseCase{
	// Apps and sites.
	{say: "open firefox", ok: true, want: []Action{{OpenApp, "Firefox", 1}}, line: "Opening it."},
	{say: "Can you please open Firefox for me?", ok: true, want: []Action{a(OpenApp, "Firefox")}},
	{say: "open the browser", ok: true, want: []Action{{OpenApp, "Firefox", 1}}},
	{say: "launch firefx", ok: true, want: []Action{a(OpenApp, "Firefox")}},
	{say: "open youtube", ok: true, want: []Action{{OpenSite, "https://www.youtube.com", 0.9}}},
	{say: "go to wikipedia", ok: true, want: []Action{a(OpenSite, "https://www.wikipedia.org")}},
	{say: "open example dot com", ok: true, want: []Action{a(OpenSite, "https://example.com")}},
	{say: "go to docs.example.org", ok: true, want: []Action{a(OpenSite, "https://docs.example.org")}},
	{say: "open qwzx", ok: true, line: "I can't find that."},
	{say: "open it", ok: false},
	{say: "open the youtube app", ok: true, want: []Action{a(OpenApp, "YouTuber")}},

	// The careful-intent fixture and other narration open nothing.
	{say: "I insert a table", ok: false},
	{say: "I open the tables every morning", ok: false},
	{say: "she opens firefox", ok: false},
	{say: "what time is it", ok: false},

	// Search words are data, kept whole.
	{say: "search for salt and pepper", ok: true, want: []Action{a(WebSearch, "salt and pepper")}, line: "Searching now."},
	{say: "look up how to open a jar", ok: true, want: []Action{a(WebSearch, "how to open a jar")}},
	{say: "search for --help", ok: true, want: []Action{a(WebSearch, "help")}},
	{say: "search for cats and open firefox", ok: true, want: []Action{a(WebSearch, "cats"), a(OpenApp, "Firefox")}},

	// Several clauses.
	{say: "open firefox and turn it up", ok: true, want: []Action{a(OpenApp, "Firefox"), a(VolumeUp, "")}, line: "On it."},
	{say: "open files, then take a screenshot", ok: true, want: []Action{a(OpenApp, "Files"), a(Screenshot, "")}},
	{say: "Open firefox. Mute.", ok: true, want: []Action{a(OpenApp, "Firefox"), a(Mute, MuteOn)}},
	{say: "open firefox and tell me a joke", ok: false},

	// Tabs: a new tab only.
	{say: "open a new tab", ok: true, want: []Action{a(NewTab, "")}, line: "New tab."},
	{say: "new tab", ok: true, want: []Action{a(NewTab, "")}},
	{say: "close this tab", ok: true, line: "Can't do that here."},
	{say: "next tab", ok: true, line: "Can't do that here."},
	{say: "switch to the previous tab", ok: true, line: "Can't do that here."},

	// Timers and reminders; a reminder with no time is not claimed.
	{say: "set a timer for 7 minutes", ok: true, want: []Action{a(SetTimer, "set a timer for 7 minutes")}},
	{say: "five minute timer", ok: true, want: []Action{a(SetTimer, "five minute timer")}},
	{say: "set a timer", ok: false},
	{say: "remind me to stretch", ok: false},
	{say: "remind me", ok: false},
	{say: "remind me at 5pm to call the bank", ok: true, want: []Action{a(SetReminder, "remind me at 5pm to call the bank")}},
	{say: "remind me tomorrow to water the plants and open the windows", ok: true,
		want: []Action{a(SetReminder, "remind me tomorrow to water the plants and open the windows")}},
	{say: "remind me in 3 days to renew it", ok: true, want: []Action{a(SetReminder, "remind me in 3 days to renew it")}},
	{say: "cancel the timer", ok: true, want: []Action{a(CancelTimer, CancelTimers)}},
	{say: "cancel the reminder", ok: true, want: []Action{a(CancelTimer, CancelReminder)}},
	{say: "cancel all my reminders", ok: true, want: []Action{a(CancelTimer, CancelReminders)}},
	{say: "cancel timers and reminders", ok: true, want: []Action{a(CancelTimer, CancelAll)}},
	{say: "list my reminders", ok: true, want: []Action{a(ListReminders, "")}},

	// Notes keep their own words, "and" included.
	{say: "remember that the spare key is under the mat and the code is 4", ok: true,
		want: []Action{a(RememberNote, "the spare key is under the mat and the code is 4")}},
	{say: "Make a note: Buy Salt and Pepper", ok: true, want: []Action{a(RememberNote, "Buy Salt and Pepper")}},
	{say: "remember to buy milk", ok: false},
	{say: "remember that " + strings.Repeat("x", maxNote+1), ok: true, line: "That's too long."},

	// Routines.
	{say: "run my routine", ok: true, want: []Action{a(RunRoutine, "")}},
	{say: "run my morning routine", ok: true, want: []Action{a(RunRoutine, "morning")}},
	{say: "play my routine", ok: true, want: []Action{a(RunRoutine, "")}},

	// The focused window.
	{say: "snap it to the left", ok: true, want: []Action{a(SnapLeft, "")}},
	{say: "move this window to the right half", ok: true, want: []Action{a(SnapRight, "")}},
	{say: "maximize", ok: true, want: []Action{a(Maximize, "")}},
	{say: "minimise this window", ok: true, want: []Action{a(Minimize, "")}},

	// Sound and media.
	{say: "turn it up", ok: true, want: []Action{a(VolumeUp, "")}},
	{say: "a bit quieter please", ok: true, want: []Action{a(VolumeDown, "")}},
	{say: "mute", ok: true, want: []Action{a(Mute, MuteOn)}},
	{say: "turn the sound off", ok: true, want: []Action{a(Mute, MuteOn)}},
	{say: "unmute", ok: true, want: []Action{a(Mute, MuteOff)}},
	{say: "pause the music", ok: true, want: []Action{a(PlayPause, MediaPause)}},
	{say: "start the music", ok: true, want: []Action{a(PlayPause, MediaPlay)}},
	{say: "play pause", ok: true, want: []Action{a(PlayPause, MediaToggle)}},
	{say: "take a screenshot", ok: true, want: []Action{a(Screenshot, "")}},

	// Oni itself.
	{say: "hop over to the upper left corner", ok: true, want: []Action{a(SelfMove, "tl")}},
	{say: "go to the bottom right", ok: true, want: []Action{a(SelfMove, "br")}},
	{say: "go home", ok: true, want: []Action{a(SelfHome, "")}},
	{say: "make yourself smaller", ok: true, want: []Action{a(SelfSize, SizeSmaller)}},

	// Settings: narrowing or looks only; widening is a Settings job.
	{say: "turn off your sounds", ok: true, want: []Action{a(SettingFlip, "sounds=off")}},
	{say: "stop listening", ok: true, want: []Action{a(SettingFlip, "voice=off")}},
	{say: "hide", ok: true, want: []Action{a(SettingFlip, "hidden=on")}},
	{say: "turn on open mic", ok: true, line: "Turn that on in Settings."},
	{say: "turn the microphone on", ok: true, line: "Turn that on in Settings."},
	{say: "start listening", ok: true, line: "Turn that on in Settings."},
	{say: "enable hands", ok: true, line: "Turn that on in Settings."},

	// Said to somebody else.
	{say: "tell Sam to open firefox", ok: false},
	{say: "ask him to turn it up", ok: false},
	{say: "honey, open the window", ok: false},
	{say: "mom open firefox", ok: false},
	{say: "open firefox, dad", ok: false},
	{say: "Sam, open firefox", ok: false},
	{say: "okay, open firefox", ok: true, want: []Action{a(OpenApp, "Firefox")}},
	{say: "louder, louder", ok: true, want: []Action{a(VolumeUp, "")}},

	// A verb on its own waits; Parse plans nothing for it.
	{say: "open", ok: true},
	{say: "search for", ok: true},
}

func TestParse(t *testing.T) {
	for _, c := range parseCases {
		p, ok := Parse(c.say, testNow, time.UTC, testCat)
		if ok != c.ok {
			t.Errorf("%q: ok %v, want %v (plan %+v)", c.say, ok, c.ok, p)
			continue
		}
		if !ok {
			if len(p.Actions) != 0 || p.Line != "" {
				t.Errorf("%q: declined with a plan %+v", c.say, p)
			}
			continue
		}
		if len(p.Actions) != len(c.want) {
			t.Errorf("%q: actions %+v, want %+v", c.say, p.Actions, c.want)
			continue
		}
		for i, w := range c.want {
			g := p.Actions[i]
			if g.Intent != w.Intent || g.Arg != w.Arg || (w.Confidence != 0 && g.Confidence != w.Confidence) {
				t.Errorf("%q: action %d %+v, want %+v", c.say, i, g, w)
			}
			if g.Confidence <= 0 || g.Confidence > 1 {
				t.Errorf("%q: confidence %v", c.say, g.Confidence)
			}
		}
		if c.line != "" && p.Line != c.line {
			t.Errorf("%q: line %q, want %q", c.say, p.Line, c.line)
		}
		if len(p.Actions) > 0 && p.Line == "" {
			t.Errorf("%q: actions without a line", c.say)
		}
	}
}

// First match: the narrow lines come before the wide open line.
func TestFirstMatchOrder(t *testing.T) {
	cases := []struct {
		say  string
		want Intent
	}{
		{"start the music", PlayPause},            // media before open
		{"play my routine", RunRoutine},           // routine before media
		{"run my routine", RunRoutine},            // routine before open
		{"start a new tab", NewTab},               // tab before open
		{"start a timer for 2 minutes", SetTimer}, // timer before open
		{"mute yourself", SettingFlip},            // flip before mute
		{"hide this window", Minimize},            // window before nothing
	}
	for _, c := range cases {
		p, ok := Parse(c.say, testNow, nil, testCat)
		if !ok || len(p.Actions) != 1 || p.Actions[0].Intent != c.want {
			t.Errorf("%q: %+v %v, want %s", c.say, p, ok, c.want)
		}
	}
	// A guarded noun is refused before the open line could start it.
	if p, ok := Parse("start the microphone", testNow, nil, testCat); !ok || len(p.Actions) != 0 {
		t.Errorf("start the microphone: %+v %v", p, ok)
	}
}

// An exact app name fires before the site table; a site beats an app that
// only sounds close.
func TestExactAppBeatsSiteBeatsFuzzy(t *testing.T) {
	near := catalog{names: []string{"YouTuber"}}
	p, _ := Parse("open youtube", testNow, nil, near)
	if len(p.Actions) != 1 || p.Actions[0].Intent != OpenSite {
		t.Fatalf("near app: %+v, want the site", p)
	}
	if s := Soundslike("youtube", "YouTuber"); s < limits().FuzzyMin {
		t.Fatalf("fixture too far apart to prove anything: %v", s)
	}
	exact := catalog{names: []string{"YouTube"}}
	p, _ = Parse("open youtube", testNow, nil, exact)
	if len(p.Actions) != 1 || p.Actions[0] != (Action{OpenApp, "YouTube", 1}) {
		t.Fatalf("exact app: %+v, want the app", p)
	}
	if p.Actions[0].Confidence < limits().ExactAppMin {
		t.Fatal("exact app under exactAppMin")
	}
	p, _ = Parse("go to youtube", testNow, nil, exact)
	if len(p.Actions) != 1 || p.Actions[0].Intent != OpenSite {
		t.Fatalf("go to: %+v, want the site", p)
	}
	p, _ = Parse("open the youtube website", testNow, nil, exact)
	if len(p.Actions) != 1 || p.Actions[0].Intent != OpenSite {
		t.Fatalf("website: %+v, want the site", p)
	}
	// With no catalog at all an app cannot be found.
	p, _ = Parse("open firefox", testNow, nil, nil)
	if len(p.Actions) != 0 || p.Line != "I can't find that." {
		t.Fatalf("no catalog: %+v", p)
	}
}

// A catalog name that is not printable never rides an action.
type badCat struct{}

func (badCat) Resolve(string) (string, string, float64, bool) { return "Fire\nfox", "x", 1, true }

func TestCatalogNameIsChecked(t *testing.T) {
	p, _ := Parse("open firefox", testNow, nil, badCat{})
	if len(p.Actions) != 0 {
		t.Fatalf("%+v", p)
	}
}

func TestAtMostMaxActions(t *testing.T) {
	say := "open firefox and open files and open terminal and open calculator and open calendar and open text editor and take a screenshot and mute"
	p, ok := Parse(say, testNow, nil, testCat)
	if !ok || len(p.Actions) != limits().MaxActions {
		t.Fatalf("%d actions, want %d: %+v", len(p.Actions), limits().MaxActions, p)
	}
}

func TestLoopedUtterancePlansAtMostOneLaunch(t *testing.T) {
	for _, say := range []string{
		strings.Repeat("open firefox ", 43),
		strings.Repeat("open firefox and ", 43),
		strings.Repeat("Open Firefox. ", 43),
	} {
		p, _ := Parse(say, testNow, nil, testCat)
		n := 0
		for _, x := range p.Actions {
			if starts[x.Intent] {
				n++
			}
		}
		if n > 1 {
			t.Errorf("%.30q…: %d launches", say, n)
		}
	}
}

func TestSafeAddress(t *testing.T) {
	good := []string{"https://example.com", "https://www.youtube.com", "https://a.b-c.org/path?q=1"}
	bad := []string{"http://example.com", "javascript:alert(1)", "file:///etc/passwd", "data:text/html,x",
		"https://user@example.com", "https://exa mple.com", "https://example", "https://", "https://a..b.com",
		"https://example.com/\"x", "https://" + strings.Repeat("a", maxAddress) + ".com", "https://EXAMPLE.com"}
	for _, u := range good {
		if !SafeAddress(u) {
			t.Errorf("%q refused", u)
		}
	}
	for _, u := range bad {
		if SafeAddress(u) {
			t.Errorf("%q passed", u)
		}
	}
	for k, u := range sites {
		if !SafeAddress(u) {
			t.Errorf("site %s: %q unsafe", k, u)
		}
	}
	for _, c := range parseCases {
		p, _ := Parse(c.say, testNow, nil, testCat)
		for _, x := range p.Actions {
			if x.Intent == OpenSite && !SafeAddress(x.Arg) {
				t.Errorf("%q planned unsafe %q", c.say, x.Arg)
			}
		}
	}
}

func TestSelfMoveCornersArePins(t *testing.T) {
	for _, say := range []string{"jump to the upper left", "sit in the top right", "go to the lower left corner", "go to the bottom right"} {
		p, _ := Parse(say, testNow, nil, testCat)
		if len(p.Actions) != 1 || !slices.Contains([]string{"tl", "tr", "bl", "br"}, p.Actions[0].Arg) {
			t.Errorf("%q: %+v", say, p)
		}
	}
}

func TestUnaddress(t *testing.T) {
	names := []string{"Oni"}
	cases := []struct {
		in, out string
		named   bool
	}{
		{"Hey Oni, open firefox", "open firefox", true},
		{"oni open firefox", "open firefox", true},
		{"open firefox, Oni.", "open firefox", true},
		{"onion soup", "onion soup", false},
		{"open firefox", "open firefox", false},
	}
	for _, c := range cases {
		out, named := Unaddress(c.in, names)
		if out != c.out || named != c.named {
			t.Errorf("Unaddress(%q) = %q, %v", c.in, out, named)
		}
	}
	// Once Oni's name is off, the rest plans; left on, it reads as said to
	// somebody else.
	rest, _ := Unaddress("Oni, open firefox", names)
	if p, ok := Parse(rest, testNow, nil, testCat); !ok || len(p.Actions) != 1 {
		t.Errorf("unaddressed: %+v %v", p, ok)
	}
	if _, ok := Parse("Oni, open firefox", testNow, nil, testCat); ok {
		t.Error("a name left on still planned")
	}
}
