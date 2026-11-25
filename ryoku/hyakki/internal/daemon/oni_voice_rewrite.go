package daemon

import (
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
)

// rewriteVoiceStyle recognizes only explicit selected-text requests. A bare
// "shorter" is not an instruction to read another application's selection.
func rewriteVoiceStyle(text string) (string, bool) {
	t := strings.Trim(strings.ToLower(strings.TrimSpace(text)), ".!?")
	styles := map[string]string{
		"rewrite this": "", "rewrite the selection": "", "rewrite selected text": "",
		"fix the grammar": protocol.OniRewriteGrammar, "fix the grammar in this": protocol.OniRewriteGrammar,
		"make this friendlier": protocol.OniRewriteFriendly, "make this more formal": protocol.OniRewriteFormal,
		"make this more casual": protocol.OniRewriteCasual, "make this more polite": protocol.OniRewritePolite,
		"make this more confident": protocol.OniRewriteConfident, "make this shorter": protocol.OniRewriteShorter,
		"make this longer": protocol.OniRewriteLonger, "make this simpler": protocol.OniRewriteSimpler,
		"turn this into an email": protocol.OniRewriteEmail, "turn this into bullets": protocol.OniRewriteBullets,
	}
	style, ok := styles[t]
	return style, ok
}

// A voice press only captures PRIMARY and prepares a disclosure. The app alone
// can review its exact text; no spoken yes, even another press, can send it.
func (d *Daemon) voiceRewriteRoute(u route.Utterance, source string, gen uint64) bool {
	settings := protocol.EffectiveOni(d.Settings())
	if !feature.Decide(feature.Request{Feature: feature.Rewrite}, settings).Allow {
		return false
	}
	style, ok := rewriteVoiceStyle(u.Text)
	if !ok {
		return false
	}
	v := &d.oni.voice
	v.mu.Lock()
	if gen != v.gen || !v.turn.Accept(u.Turn) {
		v.mu.Unlock()
		return true
	}
	if u.Turn == 0 || u.Source != route.Chord && u.Source != route.Toggle {
		d.voiceSayLocked(settings, u.Turn, source, i18n.T(route.ReflexNeedsPressLine))
		d.voiceRestLocked(u.Turn)
		v.mu.Unlock()
		return true
	}
	v.work++
	v.mu.Unlock()
	go func() {
		defer d.stateChanged()
		res, err := d.rewriteStart(nil, protocol.OniRewriteParams{Action: protocol.OniRewriteStart, Style: style})
		line := i18n.T("I couldn't prepare a rewrite. Select text in its window first.")
		if err == nil && res.Status == protocol.OniRewriteConsent {
			line = i18n.T("Open Rewrite in Hyakki to review the selected text and its route. Use its button, not a spoken yes.")
		}
		v.mu.Lock()
		defer v.mu.Unlock()
		v.work--
		if gen == v.gen && v.turn.Accept(u.Turn) {
			d.voiceSayLocked(settings, u.Turn, source, line)
			d.voiceRestLocked(u.Turn)
		}
	}()
	return true
}
