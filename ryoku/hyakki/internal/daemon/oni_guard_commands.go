package daemon

import (
	"regexp"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/guard"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
)

var guardDuration = regexp.MustCompile(`^(?:keep me off|block) (.+) for ([0-9]+|one|two|three|four|five|six|seven|eight|nine|ten|fifteen|twenty|thirty|forty|sixty) (minute|minutes|hour|hours)$`)
var guardUntilApp = regexp.MustCompile(`^(?:keep me off|block) (.+) until i close (.+)$`)
var guardNumberWords = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "fifteen": 15, "twenty": 20, "thirty": 30, "forty": 40, "sixty": 60}

func guardVoiceCommand(text string) (protocol.OniFocusParams, bool) {
	t := strings.Trim(strings.ToLower(strings.TrimSpace(text)), ".!?")
	switch t {
	case "stop guarding":
		return protocol.OniFocusParams{Action: protocol.OniFocusStop}, true
	case "let me back", "ok let me back", "okay let me back":
		return protocol.OniFocusParams{Action: protocol.OniFocusBack}, true
	}
	if m := guardDuration.FindStringSubmatch(t); m != nil {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			n = guardNumberWords[m[2]]
			if n == 0 {
				return protocol.OniFocusParams{}, false
			}
		}
		if strings.HasPrefix(m[3], "hour") {
			if n > 8 {
				n = 481
			} else {
				n *= 60
			}
		}
		return protocol.OniFocusParams{Action: protocol.OniFocusArm, Words: []string{m[1]}, Minutes: n}, true
	}
	if m := guardUntilApp.FindStringSubmatch(t); m != nil {
		return protocol.OniFocusParams{Action: protocol.OniFocusArm, Words: []string{m[1]}, UntilApp: m[2]}, true
	}
	return protocol.OniFocusParams{}, false
}

func (d *Daemon) voiceGuardRoute(u route.Utterance, source string, gen uint64) bool {
	settings := protocol.EffectiveOni(d.Settings())
	if !feature.Decide(feature.Request{Feature: feature.FeedGuard}, settings).Allow {
		return false
	}
	p, ok := guardVoiceCommand(u.Text)
	if !ok {
		return false
	}
	v := &d.oni.voice
	v.mu.Lock()
	live := gen == v.gen && v.turn.Accept(u.Turn)
	v.mu.Unlock()
	if !live {
		return true
	}
	line := i18n.T(route.ReflexNeedsPressLine)
	if p.Action == protocol.OniFocusStop || u.Turn != 0 && (u.Source == route.Chord || u.Source == route.Toggle) {
		if p.Action == protocol.OniFocusArm && (p.UntilApp == "" && (p.Minutes < 1 || p.Minutes > 480) || len(p.UntilApp) > protocol.OniFocusAppMax) {
			line = i18n.T("Choose a focus guard duration of at most eight hours.")
		} else if res, err := d.oniFocusCall(p); err == nil && res.Refused == "" {
			line = i18n.T("The focus guard is off.")
			if res.Armed {
				line = i18n.T("The focus guard is on. Each close still needs its own app card.")
			}
		} else {
			line = i18n.T("The focus guard could not start on this desktop.")
		}
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if gen == v.gen && v.turn.Accept(u.Turn) {
		d.voiceSayLocked(settings, u.Turn, source, line)
		d.voiceRestLocked(u.Turn)
	}
	return true
}

func (d *Daemon) guardOpenRefusal(text string) string {
	g := &d.oni.guard
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.armed {
		return ""
	}
	hit := guard.Match("", text, g.words)
	if hit.Reach == guard.None {
		return ""
	}
	return i18n.Tf("You asked me to stay off %s while the focus guard is on.", hit.Feed)
}

func (d *Daemon) guardActionRefusal(a reflex.Action) string {
	switch a.Intent {
	case reflex.OpenApp, reflex.OpenSite, reflex.WebSearch:
		return d.guardOpenRefusal(a.Arg)
	}
	return ""
}

// guardReflexCatalog checks the requested app name before catalogue matching
// can replace it with a similar but unrelated installed app. The parser only
// resolves app-opening targets here, not note contents or ordinary questions.
type guardReflexCatalog struct {
	daemon  *Daemon
	catalog reflex.Catalog
	line    string
}

func (c *guardReflexCatalog) Resolve(spoken string) (string, string, float64, bool) {
	if line := c.daemon.guardOpenRefusal(spoken); line != "" {
		c.line = line
		return "", "", 0, false
	}
	if c.catalog == nil {
		return "", "", 0, false
	}
	return c.catalog.Resolve(spoken)
}

func (d *Daemon) guardDecidedRequestRefusal(a reflex.Action, text string) string {
	switch a.Intent {
	case reflex.OpenApp, reflex.OpenSite, reflex.WebSearch:
		return d.guardOpenRefusal(text)
	}
	return ""
}
