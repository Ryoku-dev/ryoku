// Package scan finds text that must never reach a model as memory: text
// that reads as an instruction, hides characters, carries memory's own fence
// tags or holds a secret. It needs only the standard library and NFKC, so
// the daemon and the app's previews share it.
package scan

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Threat is one hit: the pattern's id, and whether it is a secret rather
// than an instruction.
type Threat struct {
	ID     string
	Secret bool
}

// Invisible is the id of a hit on hidden characters.
const Invisible = "invisible_unicode"

// Portions adapted from NousResearch/hermes-agent tools/threat_patterns.py:17-159
// @ fe6649291e60 (MIT, Copyright (c) 2025 Nous Research). Modified by Ryoku:
// RE2 syntax with repeats capped at 1000, the hardcoded-secret lookahead is a
// check after the match, the agent's own paths and ids are Rashin's and the
// vault's, memory's own fence tags are refused, more hidden characters are
// caught and the whole text is scanned.
const (
	filler    = `(?:\w+\s+){0,8}`
	secretVar = `\$\{?\w*(?:KEY|TOKEN|SECRET|PASSWORD|CREDENTIAL)S?\b`
	modify    = `(update|modify|edit|write|change|append|add\s+to)\s+[^\n]{0,1000}`
)

// Where a pattern applies, cumulatively: all everywhere, context adds role
// and command-and-control hijacks for recalled text, strict adds the
// aggressive checks for memory writes.
const (
	all = iota
	context
	strict
)

type pattern struct {
	re    *regexp.Regexp
	id    string
	scope int
}

var patterns = func() []pattern {
	raw := []struct {
		pattern string
		id      string
		scope   int
	}{
		{`ignore\s+` + filler + `(previous|all|above|prior)\s+` + filler + `instructions`, "prompt_injection", all},
		{`<\s*/?\s*(memory-context|user_card|memory_metadata)`, "memory_tag", all},
		{`system\s+prompt\s+override`, "sys_prompt_override", all},
		{`disregard\s+` + filler + `(your|all|any)\s+` + filler + `(instructions|rules|guidelines)`, "disregard_rules", all},
		{`act\s+as\s+(if|though)\s+` + filler + `you\s+` + filler + `(have\s+no|don't\s+have)\s+` + filler + `(restrictions|limits|rules)`, "bypass_restrictions", all},
		{`<!--[^>]{0,512}(?:ignore|override|system|secret|hidden)[^>]{0,512}-->`, "html_comment_injection", all},
		{`<\s*div\s+style\s*=\s*["'][^>]{0,1000}display\s*:\s*none`, "hidden_div", all},
		{`translate\s+[^\n]{0,512}\s+into\s+\w+(?:[\s-]+\w+){0,2}\s+and\s+(execute|run|eval)\b`, "translate_execute", all},
		{`do\s+not\s+` + filler + `tell\s+` + filler + `the\s+user`, "deception_hide", all},

		{`you\s+are\s+` + filler + `now\s+(?:a|an|the)\s+`, "role_hijack", context},
		{`pretend\s+` + filler + `(you\s+are|to\s+be)\s+`, "role_pretend", context},
		{`output\s+` + filler + `(system|initial)\s+prompt`, "leak_system_prompt", context},
		{`(respond|answer|reply)\s+without\s+` + filler + `(restrictions|limitations|filters|safety)`, "remove_filters", context},
		{`you\s+have\s+been\s+` + filler + `(updated|upgraded|patched)\s+to`, "fake_update", context},
		{`\bname\s+yourself\s+\w+`, "identity_override", context},
		{`register\s+(as\s+)?a?\s*node`, "c2_node_registration", context},
		{`(heartbeat|beacon|check[\s\-]?in)\s+(to|with)\s+`, "c2_heartbeat", context},
		{`pull\s+(down\s+)?(?:new\s+)?task(?:ing|s)?\b`, "c2_task_pull", context},
		{`connect\s+to\s+the\s+network\b`, "c2_network_connect", context},
		{`you\s+must\s+(?:\w+\s+){0,3}(register|connect|report|beacon)\b`, "forced_action", context},
		{`only\s+use\s+one[\s\-]?liners?\b`, "anti_forensic_oneliner", context},
		{`never\s+` + filler + `(?:create|write)\s+` + filler + `(?:script|file)\s+` + filler + `disk`, "anti_forensic_disk", context},
		{`unset\s+\w*(?:CLAUDE|CODEX|RASHIN|RYOKU|AGENT|OPENAI|ANTHROPIC)\w*`, "env_var_unset_agent", context},
		{`\b(?:cobalt\s*strike|sliver|havoc|mythic|metasploit|brainworm)\b`, "known_c2_framework", context},
		{`\bc2\s+(?:server|channel|infrastructure|beacon)\b`, "c2_explicit", context},
		{`\bcommand\s+and\s+control\b`, "c2_explicit_long", context},

		{`curl\s+[^\n]{0,1000}` + secretVar, "exfil_curl", all},
		{`wget\s+[^\n]{0,1000}` + secretVar, "exfil_wget", all},
		{`cat\s+[^\n]{0,1000}(\.env|credentials|\.netrc|\.pgpass|\.npmrc|\.pypirc)`, "read_secrets", all},
		{`(send|post|upload|transmit)\s+[^\n]{0,1000}\s+(to|at)\s+https?://`, "send_to_url", strict},
		{`(include|output|print|share)\s+` + filler + `(conversation|chat\s+history|previous\s+messages|full\s+context|entire\s+context)`, "context_exfil", strict},

		{`authorized_keys`, "ssh_backdoor", strict},
		{`(?:\b(?:echo|cat|cp|mv|dd|tee|install|printf|rsync|scp|ln|append|add|write|sed|chmod|chown|truncate|rm|touch|curl|wget|git)\b|\bopen\s*\(|>>?)[^\n]{0,512}(?:\$HOME/\.ssh|~/\.ssh)`, "ssh_access", strict},
		{`(?:\$HOME|~)/\.local/share/ryoku/vault|(?:\$HOME|~)/\.local/state/ryoku/rashin/master\.key`, "vault_access", strict},
		{modify + `(?:AGENTS\.md|CLAUDE\.md|\.cursorrules|\.clinerules)`, "agent_config_mod", strict},
		{modify + `(?:\.config/ryoku/rashin\.json|ryoku/vault/)`, "rashin_config_mod", strict},
	}
	out := make([]pattern, len(raw))
	for index, item := range raw {
		out[index] = pattern{re: regexp.MustCompile(`(?i)` + item.pattern), id: item.id, scope: item.scope}
	}
	return out
}()

// secret finds a key, token, secret or password written as a quoted value
// of 20 or more characters; envName is a value that only names an
// environment variable (SHOUTY_SNAKE), which says where a secret lives
// without holding it.
var (
	secret  = regexp.MustCompile(`(?i)(?:api[_-]?key|token|secret|password)\s*[=:]\s*["']([A-Za-z0-9+/=_-]{20,})(["']?)`)
	envName = regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+$`)
)

// invisible are the zero-width, soft-hyphen, direction-mark and
// bidirectional characters used to hide an instruction.
const invisible = "\u00ad\u200b\u200c\u200d\u200e\u200f\u2060\u2061\u2062\u2063\u2064\ufeff\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069"

// Strict checks text about to be written to memory or read back from a
// memory file: everything Context does, plus exfiltration, persistence and
// secrets.
func Strict(text string) (Threat, bool) { return find(text, strict) }

// Context checks text about to be put in front of a model as data, such as
// a recalled fact or a carried report.
func Context(text string) (Threat, bool) { return find(text, context) }

// find returns the first threat in text for a scope: invisible characters
// are checked on the raw text, the patterns after NFKC folds full-width and
// compatibility forms. The whole text is read: RE2 runs in linear time, so a
// long text cannot hide its end.
func find(text string, scope int) (Threat, bool) {
	if strings.ContainsAny(text, invisible) {
		return Threat{ID: Invisible}, true
	}
	folded := norm.NFKC.String(text)
	for _, p := range patterns {
		if p.scope <= scope && p.re.MatchString(folded) {
			return Threat{ID: p.id}, true
		}
	}
	if scope == strict {
		for _, match := range secret.FindAllStringSubmatch(folded, -1) {
			if match[2] != "" && envName.MatchString(match[1]) {
				continue
			}
			return Threat{ID: "hardcoded_secret", Secret: true}, true
		}
	}
	return Threat{}, false
}
