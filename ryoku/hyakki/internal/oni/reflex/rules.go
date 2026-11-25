package reflex

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	i18n "ryoku-i18n"
)

// verdict is what one clause comes to: an action, an honest refusal line
// with no action, or a verb still waiting for its object.
type verdict struct {
	act    *Action
	refuse string
	verb   string
}

// How far a clause reaches over the cuts after it. A note or a reminder
// keeps every "and" and every command word that follows it in its own
// sentence; a search, a timer or a named routine keeps the command words
// but lets "and" start a new clause.
const (
	absorbNone = iota
	absorbVerbs
	absorbJoints
)

// rule is one line of the table: an anchored, case-insensitive pattern
// over a cleaned clause, the confidence its action carries, how far it
// absorbs, whether its object may swallow anything (open and search), and
// what it plans. plan returns ok false to decline, and the table moves on.
type rule struct {
	re     *regexp.Regexp
	conf   float64
	absorb int
	greedy bool
	plan   func(m []string, cat Catalog) (verdict, bool)
}

func rx(s string) *regexp.Regexp { return regexp.MustCompile(`(?i)^(?:` + s + `)$`) }

func act(i Intent, arg string) (verdict, bool) {
	return verdict{act: &Action{Intent: i, Arg: arg}}, true
}

func fixed(i Intent, arg string) func([]string, Catalog) (verdict, bool) {
	return func([]string, Catalog) (verdict, bool) { return act(i, arg) }
}

func refuse(line string) func([]string, Catalog) (verdict, bool) {
	return func([]string, Catalog) (verdict, bool) { return verdict{refuse: line}, true }
}

// Lines Oni says. Each is one sentence of at most five words.
const (
	lineCantHere   = "Can't do that here."
	lineNotFound   = "I can't find that."
	lineInSettings = "Turn that on in Settings."
	lineTooLong    = "That's too long."
	lineAlready    = "Already on it."
	lineEnough     = "That's enough for now."
	lineMany       = "On it."
)

// Bounds on what an action may carry.
const (
	maxQuery   = 200
	maxNote    = 500
	maxName    = 80
	maxAddress = 512
)

// Settings that may change by voice: each only narrows what Oni does or
// changes how it looks. Nothing here turns on listening, wake, screen
// reading, guides, decisions or hands; those take Settings and a Press.
var flips = []string{
	"sounds=off", "sounds=on",
	"reduceMotion=on", "reduceMotion=off",
	"hidden=on",
	"voice=off",
	"reflex=off",
}

// Flips returns every SettingFlip argument a plan can carry.
func Flips() []string { return append([]string(nil), flips...) }

// Number words and units a timer or reminder may be said with. The
// reminder parser does the real reading; these only decide whether a
// clause names a time at all.
const (
	numWord   = `\d+(?:[.,]\d+)?|a|an|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty|thirty|forty|fifty|sixty|ninety|half|couple|few`
	clockWord = `one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve`
	weekday   = `monday|tuesday|wednesday|thursday|friday|saturday|sunday`
)

var (
	shortSpan = regexp.MustCompile(`(?i)\b(?:` + numWord + `)\b(?:[\s-]+[\w-]+){0,3}?[\s-]*(?:seconds?|secs?|minutes?|mins?|hours?|hrs?)\b|\b\d+\s*(?:s|sec|secs|m|min|mins|h|hr|hrs)\b`)
	longSpan  = regexp.MustCompile(`(?i)\b(?:` + numWord + `)\b(?:[\s-]+[\w-]+){0,3}?[\s-]*(?:days?|weeks?)\b`)
	clockAt   = regexp.MustCompile(`(?i)\bat (?:\d{1,2}(?::\d{2})?(?:\s?[ap]\.?m\.?)?|noon|midnight|(?:` + clockWord + `)(?: thirty| fifteen| forty[- ]five)?(?: o'?clock)?)\b|\b\d{1,2}(?::\d{2})?\s?[ap]\.?m\b|\b\d{1,2}:\d{2}\b`)
	dayWord   = regexp.MustCompile(`(?i)\b(?:tomorrow|tonight|this (?:morning|afternoon|evening)|in the (?:morning|afternoon|evening)|(?:` + weekday + `))\b`)
)

func timed(c string) bool { return shortSpan.MatchString(c) }

func dated(c string) bool {
	return shortSpan.MatchString(c) || longSpan.MatchString(c) || clockAt.MatchString(c) || dayWord.MatchString(c)
}

// Words a setting may not be turned on by voice.
const guarded = `voice|listening|the microphone|microphone|mic|open mic|wake word|wake|screen reading|inspect|inspection|guides?|decisions|hands|memory|hints|reflexes|quick commands`

const (
	thing   = `(?: it| this| that| the window| this window| the app| this app)?`
	media   = `music|song|track|video|playback|media|podcast`
	soundOf = `volume|sound|music|audio`
)

// openVerbs start a clause that opens an app or a site; siteVerbs lean
// towards a site.
const (
	openVerbs = `open(?: up)?|launch|start(?: up)?|run|pull up|fire up|bring up|load|go to|visit|take me to|navigate to|head to|browse to`
	siteVerbs = `go to|visit|take me to|navigate to|head to|browse to`
	waitVerbs = `open(?: up)?|launch|start(?: up)?|pull up|fire up|bring up|go to|visit|take me to|navigate to|search(?: the web| online)?(?: for)?|look up`
)

var siteVerb = rx(siteVerbs)

// table is the rules, first match wins. Every line is written for Hyakki;
// the order puts refusals and narrow commands before the wide open and
// search lines.
var table = []rule{
	// Turning a guarded feature on is a Settings job.
	{re: rx(`(?:turn|switch|flip) on (?:the |your )?(?:` + guarded + `)|(?:turn|switch|flip) (?:the |your )?(?:` + guarded + `) on|(?:enable|activate|start) (?:the |your )?(?:` + guarded + `)`),
		conf: 0.9, plan: refuse(lineInSettings)},

	// Settings that narrow or only change looks.
	{re: rx(`(?:turn|switch|shut) (?:off|down) your (?:sounds?|noises?|sound effects)|turn your (?:sounds?|noises?|sound effects) off|mute yourself|stop making (?:sounds|noises)|no more (?:sounds|noises)`),
		conf: 0.9, plan: fixed(SettingFlip, "sounds=off")},
	{re: rx(`(?:turn|switch) on your (?:sounds?|sound effects)|turn your (?:sounds?|sound effects) (?:back )?on`),
		conf: 0.9, plan: fixed(SettingFlip, "sounds=on")},
	{re: rx(`(?:reduce|tone down|calm down) (?:your )?(?:motion|movement|animations?)|(?:turn|switch) on reduced? motion|move less`),
		conf: 0.9, plan: fixed(SettingFlip, "reduceMotion=on")},
	{re: rx(`(?:turn|switch) off reduced? motion`),
		conf: 0.9, plan: fixed(SettingFlip, "reduceMotion=off")},
	{re: rx(`hide(?: yourself)?|go hide`),
		conf: 0.9, plan: fixed(SettingFlip, "hidden=on")},
	{re: rx(`stop listening|(?:turn|switch) off (?:the )?(?:voice|listening|microphone|mic)|(?:turn|switch) (?:the )?(?:voice|listening|microphone|mic) off`),
		conf: 0.9, plan: fixed(SettingFlip, "voice=off")},
	{re: rx(`(?:turn|switch) off (?:your )?(?:quick commands|reflexes)|(?:turn|switch) (?:your )?(?:quick commands|reflexes) off`),
		conf: 0.9, plan: fixed(SettingFlip, "reflex=off")},

	// Tabs: only a new tab, through the browser's own launcher.
	{re: rx(`(?:close|shut) (?:this |the |that |my |a )?(?:current )?tab|(?:go to |switch to |move to |jump to )?(?:the )?(?:next|previous|last|other) tab|switch tabs?|(?:reopen|restore) (?:the |that |my )?(?:last |closed )*tab`),
		conf: 0.9, plan: refuse(lineCantHere)},
	{re: rx(`(?:open|start|make|give me|create)(?: up)? (?:a |another |one more )?(?:new )?(?:browser )?tab|new (?:browser )?tab`),
		conf: 0.95, plan: fixed(NewTab, "")},

	// Timers and reminders.
	{re: rx(`(?:cancel|stop|clear|kill|turn off|delete|remove) (the |my |that |this |all |all my |all the |all of my )?(timers?|reminders?|timers and reminders|reminders and timers)`),
		conf: 0.9, plan: func(m []string, _ Catalog) (verdict, bool) {
			all := strings.HasPrefix(strings.ToLower(m[1]), "all")
			switch w := strings.ToLower(m[2]); {
			case strings.Contains(w, " and "):
				return act(CancelTimer, CancelAll)
			case strings.HasPrefix(w, "timer"):
				return act(CancelTimer, CancelTimers)
			case w == "reminder" && !all:
				return act(CancelTimer, CancelReminder)
			default:
				return act(CancelTimer, CancelReminders)
			}
		}},
	{re: rx(`(?:list|show|read|tell me|what are)(?: me)? (?:all )?(?:my |the )?(?:reminders|timers|reminders and timers|timers and reminders)|what (?:reminders|timers) do i have|do i have any (?:reminders|timers)|any (?:reminders|timers)`),
		conf: 0.9, plan: fixed(ListReminders, "")},
	{re: rx(`(?:(?:set|start|put on|make|give me)(?: me)? )?(?:a |an |the )?(?:timer|countdown)(?: for| of)? .+|(?:(?:set|start|put on|make|give me)(?: me)? )?.+ (?:timer|countdown)|time me(?: for)? .+|count down(?: from| for)? .+`),
		conf: 0.9, absorb: absorbVerbs, plan: func(m []string, _ Catalog) (verdict, bool) {
			if !timed(m[0]) {
				return verdict{}, false
			}
			return act(SetTimer, m[0])
		}},
	{re: rx(`remind me .+|(?:set|create|add|make) (?:a |an |me a )?reminder .+|reminder .+`),
		conf: 0.9, absorb: absorbJoints, plan: func(m []string, _ Catalog) (verdict, bool) {
			if !dated(m[0]) {
				return verdict{}, false
			}
			return act(SetReminder, m[0])
		}},

	// Notes go to memory; the whole rest of the sentence is the note.
	{re: rx(`(?:keep in mind|remember|note|don't forget|do not forget) that:? (.+)|remember this:? (.+)|(?:make|take) a note(?: that)?:? (.+)|note: (.+)|jot down(?: that)?:? (.+)`),
		conf: 0.9, absorb: absorbJoints, plan: func(m []string, _ Catalog) (verdict, bool) {
			for _, g := range m[1:] {
				if g = strings.TrimSpace(g); g != "" {
					if len(g) > maxNote {
						return verdict{refuse: lineTooLong}, true
					}
					return act(RememberNote, g)
				}
			}
			return verdict{}, false
		}},

	// Routines.
	{re: rx(`(?:run|do|start|play|replay) (?:my |the )?routine`),
		conf: 0.9, plan: fixed(RunRoutine, "")},
	{re: rx(`(?:run|do|start|replay) (?:my |the )?routine (?:called |named )?(.+)|(?:run|do|start|replay) (?:my |the )?(.+) routine`),
		conf: 0.9, absorb: absorbVerbs, plan: func(m []string, _ Catalog) (verdict, bool) {
			name := strings.TrimSpace(m[1] + m[2])
			if !printable(name) || len(name) > maxName {
				return verdict{}, false
			}
			return act(RunRoutine, name)
		}},

	// The focused window.
	{re: rx(`(?:snap|move|put|tile|send|push)` + thing + `(?: to| over to| on)?(?: the)? left(?: side| half)?|left half`),
		conf: 0.9, plan: fixed(SnapLeft, "")},
	{re: rx(`(?:snap|move|put|tile|send|push)` + thing + `(?: to| over to| on)?(?: the)? right(?: side| half)?|right half`),
		conf: 0.9, plan: fixed(SnapRight, "")},
	{re: rx(`(?:maximi[sz]e|full ?screen)` + thing + `|make (?:it|this|that|the window|this window) (?:full ?screen|maximi[sz]ed)`),
		conf: 0.9, plan: fixed(Maximize, "")},
	{re: rx(`minimi[sz]e` + thing + `|hide (?:this|that|the) (?:window|app)`),
		conf: 0.9, plan: fixed(Minimize, "")},

	// Sound and media.
	{re: rx(`(?:turn|crank|pump|bring|put)(?: the)? (?:` + soundOf + `|it) up|(?:turn|crank|pump) up(?: the)? (?:` + soundOf + `)|volume up|louder|(?:make it|a bit|a little) louder|(?:raise|increase|boost)(?: the)? (?:volume|sound)|more volume`),
		conf: 0.9, plan: fixed(VolumeUp, "")},
	{re: rx(`(?:turn|bring|put)(?: the)? (?:` + soundOf + `|it) down|turn down(?: the)? (?:` + soundOf + `)|volume down|quieter|softer|(?:make it|a bit|a little) (?:quieter|softer)|(?:lower|decrease|reduce)(?: the)? (?:volume|sound)|less volume`),
		conf: 0.9, plan: fixed(VolumeDown, "")},
	{re: rx(`mute(?: it| the sound| the audio| the volume| sound| audio| everything| the music)?|(?:turn|switch) off (?:the )?(?:sound|audio)|(?:turn|switch) (?:the )?(?:sound|audio) off`),
		conf: 0.9, plan: fixed(Mute, MuteOn)},
	{re: rx(`unmute(?: it| the sound| the audio| the volume| sound| audio| everything| the music)?|(?:turn|switch) (?:the )?(?:sound|audio) (?:back )?on|(?:turn|switch) on (?:the )?(?:sound|audio)`),
		conf: 0.9, plan: fixed(Mute, MuteOff)},
	{re: rx(`play ?pause|play or pause|play/pause|toggle (?:play(?:back)?|the music|music)`),
		conf: 0.9, plan: fixed(PlayPause, MediaToggle)},
	{re: rx(`(?:pause|stop)(?: the| my| this)? (?:` + media + `)|pause(?: it| that| this)?|hold the music`),
		conf: 0.9, plan: fixed(PlayPause, MediaPause)},
	{re: rx(`(?:play|resume|unpause|continue|start)(?: the| my)? (?:` + media + `)|(?:resume|unpause)(?: it| that| this)?|play`),
		conf: 0.9, plan: fixed(PlayPause, MediaPlay)},

	// Screenshots go to the installed screenshot tool.
	{re: rx(`(?:take|grab|make|snap|get|capture)(?: me)? (?:a |an )?(?:screenshot|screen ?shot|screen grab|screen capture|picture of (?:the|my) screen)|(?:screenshot|screen ?shot)(?: this| that| it| the screen| my screen)?|(?:capture|grab) (?:the|my) screen`),
		conf: 0.9, plan: fixed(Screenshot, "")},

	// Oni itself.
	{re: rx(`(?:go|move|hop|jump|sit|get|head)(?: yourself| over)?(?: to| into| in)? (?:the )?(upper|top|lower|bottom) (left|right)(?: corner)?`),
		conf: 0.9, plan: func(m []string, _ Catalog) (verdict, bool) {
			v := "t"
			if w := strings.ToLower(m[1]); w == "bottom" || w == "lower" {
				v = "b"
			}
			return act(SelfMove, v+strings.ToLower(m[2])[:1])
		}},
	{re: rx(`(?:go|move|head|get)(?: yourself)?(?: back)? home|(?:go|move|head|get) back to your (?:home|corner|spot|place)|go back to where you were`),
		conf: 0.9, plan: fixed(SelfHome, "")},
	{re: rx(`(?:make yourself|get|grow|be) (?:a bit |a little |much )?(?:bigger|larger)|grow(?: yourself)?`),
		conf: 0.9, plan: fixed(SelfSize, SizeBigger)},
	{re: rx(`(?:make yourself|get|shrink|be) (?:a bit |a little |much )?(?:smaller|tinier)|shrink(?: yourself)?`),
		conf: 0.9, plan: fixed(SelfSize, SizeSmaller)},

	// A web search; the words are data, never flags.
	{re: rx(`(?:search(?: the web| online)?(?: for)?|look up|web search(?: for)?) (.+?)(?: online| on the web| on the internet)?|(?:look for|find) (.+) (?:online|on the web|on the internet)`),
		conf: 0.85, absorb: absorbVerbs, greedy: true, plan: func(m []string, _ Catalog) (verdict, bool) {
			q := strings.TrimLeft(strings.TrimSpace(m[1]+m[2]), "-")
			if !printable(q) || !wordy(q) || hollow[strings.ToLower(q)] {
				return verdict{}, false
			}
			if len(q) > maxQuery {
				return verdict{refuse: lineTooLong}, true
			}
			return act(WebSearch, q)
		}},

	// Open an app or a site.
	{re: rx(`(` + openVerbs + `)(?: the| my)? (.+?)(?: (app|application|program|website|web site|site|page|homepage))?`),
		conf: 0.9, greedy: true, plan: func(m []string, cat Catalog) (verdict, bool) {
			return open(m[1], m[2], strings.ToLower(m[3]), cat)
		}},

	// A verb on its own waits for its object.
	{re: rx(waitVerbs), plan: func(m []string, _ Catalog) (verdict, bool) {
		return verdict{verb: strings.ToLower(m[0])}, true
	}},
}

// Words that are never an app, a site or a search on their own.
var hollow = map[string]bool{"": true, "the": true, "a": true, "an": true, "it": true, "this": true, "that": true, "my": true, "something": true, "one": true, "for": true, "up": true, "online": true}

// siteConf is the confidence of a site named or spelled out.
const siteConf = 0.9

// open resolves "open X": a spelled-out address first, then an app whose
// name or alias matches exactly (unless the words lean to a site), then the
// site table, then an app that sounds close enough. A site therefore beats
// a near app name, and an exact app name beats a site.
func open(verb, target, kind string, cat Catalog) (verdict, bool) {
	target = strings.TrimSpace(target)
	if hollow[strings.ToLower(target)] || !printable(target) {
		return verdict{}, false
	}
	lim := limits()
	siteFirst := siteVerb.MatchString(verb) || kind == "website" || kind == "web site" || kind == "site" || kind == "page" || kind == "homepage"
	appOnly := kind == "app" || kind == "application" || kind == "program"
	if !appOnly {
		if u, ok := spelled(target); ok {
			return verdict{act: &Action{Intent: OpenSite, Arg: u, Confidence: siteConf}}, true
		}
	}
	var name string
	var score float64
	var found bool
	if cat != nil {
		name, _, score, found = cat.Resolve(target)
		found = found && name != "" && printable(name) && len(name) <= maxName
	}
	exact := found && score >= lim.ExactAppMin
	// "open firefox and tell me a joke" is not one app; only an exact
	// name may hold a joining word.
	if !exact && jointCut.MatchString(target) {
		return verdict{}, false
	}
	if exact && !siteFirst {
		return verdict{act: &Action{Intent: OpenApp, Arg: name, Confidence: score}}, true
	}
	if !appOnly {
		if u, ok := sites[plain(target)]; ok {
			return verdict{act: &Action{Intent: OpenSite, Arg: u, Confidence: siteConf}}, true
		}
	}
	if found && score >= lim.FuzzyMin {
		return verdict{act: &Action{Intent: OpenApp, Arg: name, Confidence: score}}, true
	}
	return verdict{refuse: lineNotFound}, true
}

// sites are the sites Oni opens by name, keyed by the name with case,
// spaces and punctuation gone.
var sites = map[string]string{
	"youtube":       "https://www.youtube.com",
	"wikipedia":     "https://www.wikipedia.org",
	"github":        "https://github.com",
	"reddit":        "https://www.reddit.com",
	"gmail":         "https://mail.google.com",
	"netflix":       "https://www.netflix.com",
	"twitch":        "https://www.twitch.tv",
	"stackoverflow": "https://stackoverflow.com",
	"duckduckgo":    "https://duckduckgo.com",
}

var hostSaid = regexp.MustCompile(`^(?:https?://)?((?:www\.)?[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*\.(?:com|org|net|io|dev|edu|gov|app|tv|me|co|uk|de|fr|jp|ca|au|info))/?$`)

// spelled reads an address said aloud ("example dot com") or typed
// ("example.com") into an https address.
func spelled(target string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(target))
	t = strings.ReplaceAll(t, " dot ", ".")
	m := hostSaid.FindStringSubmatch(t)
	if m == nil {
		return "", false
	}
	u := "https://" + m[1]
	return u, SafeAddress(u)
}

// SafeAddress reports whether u is an address a plan may carry: https
// only, a plain host name with no user part, no spaces, quotes or control
// characters, and bounded in length.
func SafeAddress(u string) bool {
	rest, ok := strings.CutPrefix(u, "https://")
	if !ok || len(u) > maxAddress || rest == "" {
		return false
	}
	host, path, _ := strings.Cut(rest, "/")
	if host == "" || !strings.Contains(host, ".") || strings.Contains(host, "..") {
		return false
	}
	for _, r := range host {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-') {
			return false
		}
	}
	for _, r := range path {
		if r <= ' ' || r == 0x7f || r == '"' || r == '\'' || r == '\\' || r == '<' || r == '>' || r == '`' {
			return false
		}
	}
	return true
}

// printable reports whether s holds no control characters.
func printable(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) < 0
}

// wordy reports whether s holds a letter or a digit.
func wordy(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
}

// line is what Oni says before it does a.
func line(a Action) string {
	switch a.Intent {
	case OpenApp, OpenSite:
		return i18n.T("Opening it.")
	case WebSearch:
		return i18n.T("Searching now.")
	case NewTab:
		return i18n.T("New tab.")
	case SnapLeft, SnapRight:
		return i18n.T("Snapping it.")
	case Maximize:
		return i18n.T("Maximizing.")
	case Minimize:
		return i18n.T("Minimizing.")
	case VolumeUp:
		return i18n.T("Louder.")
	case VolumeDown:
		return i18n.T("Quieter.")
	case Mute:
		if a.Arg == MuteOff {
			return i18n.T("Unmuted.")
		}
		return i18n.T("Muted.")
	case PlayPause:
		switch a.Arg {
		case MediaPlay:
			return i18n.T("Playing.")
		case MediaPause:
			return i18n.T("Paused.")
		}
		return i18n.T("Okay.")
	case Screenshot:
		return i18n.T("Taking a screenshot.")
	case RememberNote:
		return i18n.T("Noting that.")
	case RunRoutine:
		return i18n.T("Running your routine.")
	case SetTimer:
		return i18n.T("Setting a timer.")
	case SetReminder:
		return i18n.T("Setting a reminder.")
	case CancelTimer:
		return i18n.T("Cancelling.")
	case ListReminders:
		return i18n.T("Checking.")
	case SelfMove:
		return i18n.T("Moving.")
	case SelfSize:
		return i18n.T("Resizing.")
	case SelfHome:
		return i18n.T("Going home.")
	case SettingFlip:
		return i18n.T("Changing that.")
	}
	return i18n.T(lineMany)
}

// Lead words that are only manners or filler, and the ways of asking that
// put the command after them.
var (
	polite     = regexp.MustCompile(`(?i)^(?:(?:please|kindly|just|so|ok|okay|alright|all right|hey|um|uh|er|now|then|and|also|oh|well)[,.!]?\s+|(?:could|can|will|would) (?:you|u)(?: please)?\s+|i(?:'d| would) like (?:you )?to\s+|i (?:want|need) (?:you )?to\s+|i wanna\s+|go ahead and\s+|let's\s+)+`)
	politeTail = regexp.MustCompile(`(?i)(?:[,\s]+(?:please|for me|now|right now|thanks|thank you|okay|ok))+$`)
	// A clause that opens on a person or a thing is told, not asked: "I
	// insert a table", "she opens the door".
	narrated = regexp.MustCompile(`(?i)^(?:i|i'm|im|i'll|i've|i'd|we|we're|we'll|he|he's|she|she's|they|they're|it|it's|you|you're|my|our|his|her|their|there|there's)\b`)
)

const edge = " \t\r\n.,!?;:\"“”‘’"

// clean gathers whitespace, straightens apostrophes and trims stray marks
// from both ends.
func clean(s string) string {
	s = strings.NewReplacer("’", "'", "‘", "'").Replace(s)
	return strings.Trim(strings.Join(strings.Fields(s), " "), edge)
}

// command is a clause with its manners taken off.
func command(c string) string {
	c = clean(c)
	c = polite.ReplaceAllString(c, "")
	c = politeTail.ReplaceAllString(c, "")
	return strings.Trim(c, edge)
}

// judge runs one clause down the table. ok false means no line claims it.
func judge(clause string, cat Catalog) (verdict, *rule, bool) {
	c := command(clause)
	if c == "" || narrated.MatchString(c) {
		return verdict{}, nil, false
	}
	for i := range table {
		r := &table[i]
		m := r.re.FindStringSubmatch(c)
		if m == nil {
			continue
		}
		v, ok := r.plan(m, cat)
		if !ok {
			continue
		}
		if v.act != nil && v.act.Confidence == 0 {
			v.act.Confidence = r.conf
		}
		return v, r, true
	}
	return verdict{}, nil, false
}

// People a few words are said to rather than to Oni.
const kin = `mom|mum|mommy|mama|dad|daddy|papa|honey|hon|babe|baby|sweetie|sweetheart|darling|dear|love|guys|you guys|everyone|everybody|kids|son|sis|bro|dude|buddy|mate|man|folks|grandma|grandpa|granny`

var (
	toldVia  = regexp.MustCompile(`(?i)^(?:(?:hey|ok|okay|so|now|please)[,]?\s+)*(?:tell|ask|let)\s+(\S+)`)
	kinLead  = regexp.MustCompile(`(?i)^(?:hey |oi |yo )?(?:` + kin + `)\b[,!:]?\s`)
	kinTail  = regexp.MustCompile(`(?i),\s*(?:` + kin + `)$`)
	vocative = regexp.MustCompile(`^([\p{L}'-]+)\s*,\s*(.+)$`)
	fillers  = map[string]bool{"please": true, "ok": true, "okay": true, "alright": true, "right": true, "well": true, "so": true, "now": true, "yes": true, "yeah": true, "yep": true, "no": true, "hey": true, "um": true, "uh": true, "oh": true, "then": true, "also": true, "and": true, "first": true, "next": true, "and then": true}
)

// elsewhere reports whether text is said to somebody else: "tell Sam to
// open it", "honey, open the window", "open the window, mom", or "Sam,
// open the window". The caller takes Oni's own name off first (Unaddress).
func elsewhere(text string) bool {
	if m := toldVia.FindStringSubmatch(text); m != nil && !strings.EqualFold(strings.Trim(m[1], edge), "me") {
		return true
	}
	if kinLead.MatchString(text) || kinTail.MatchString(text) {
		return true
	}
	if m := vocative.FindStringSubmatch(text); m != nil && !fillers[strings.ToLower(m[1])] {
		if _, _, ok := judge(m[1], nil); !ok {
			if _, _, ok := judge(m[2], nil); ok {
				return true
			}
		}
	}
	return false
}

// Unaddress takes a leading or trailing call of one of names off text
// ("hey Oni, open files" or "open files, Oni") and reports whether one was
// there. Matching ignores case.
func Unaddress(text string, names []string) (string, bool) {
	t := clean(text)
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		for _, lead := range []string{"hey ", "hi ", "ok ", "okay ", "yo ", ""} {
			p := lead + n
			if len(t) < len(p) || !strings.EqualFold(t[:len(p)], p) {
				continue
			}
			rest := t[len(p):]
			if r := []rune(rest); len(r) > 0 && (unicode.IsLetter(r[0]) || unicode.IsDigit(r[0])) {
				continue
			}
			return strings.Trim(rest, edge+"-"), true
		}
		if len(t) > len(n) && strings.EqualFold(t[len(t)-len(n):], n) {
			before := strings.TrimRight(t[:len(t)-len(n)], " ")
			if strings.HasSuffix(before, ",") {
				return strings.Trim(before, edge), true
			}
		}
	}
	return t, false
}

// Parse plans one utterance. ok is false when no line of the table claims
// it, when a clause in it is claimed by none, or when it is said to
// somebody else; the router then takes the slow path. A plan holds at most
// the spec's reflex.maxActions actions, each said once. A timer or a
// reminder keeps its whole clause as Arg for the reminder parser, which
// reads it against the time and zone the caller hands in; Parse itself
// reads neither the clock nor the zone.
func Parse(text string, _ time.Time, _ *time.Location, cat Catalog) (Plan, bool) {
	p, _, ok := plan(text, cat)
	return p, ok
}

// plan is Parse that also returns a verb left waiting at the end.
func plan(text string, cat Catalog) (Plan, string, bool) {
	text = clean(text)
	if text == "" || elsewhere(text) {
		return Plan{}, "", false
	}
	clauses := Split(text)
	var p Plan
	var refusal, verb string
	for i, c := range clauses {
		v, _, ok := judge(c, cat)
		if !ok {
			return Plan{}, "", false
		}
		switch {
		case v.act != nil:
			if !held(p.Actions, *v.act) && len(p.Actions) < limits().MaxActions {
				p.Actions = append(p.Actions, *v.act)
			}
		case v.refuse != "":
			if refusal == "" {
				refusal = v.refuse
			}
		case v.verb != "" && i == len(clauses)-1:
			verb = v.verb
		}
	}
	switch {
	case len(p.Actions) == 1:
		p.Line = line(p.Actions[0])
	case len(p.Actions) > 1:
		p.Line = i18n.T(lineMany)
	case refusal != "":
		p.Line = i18n.T(refusal)
	case verb == "":
		return Plan{}, "", false
	}
	return p, verb, true
}

// held reports whether acts already holds the same intent and argument.
func held(acts []Action, a Action) bool {
	for _, b := range acts {
		if b.Intent == a.Intent && b.Arg == a.Arg {
			return true
		}
	}
	return false
}
