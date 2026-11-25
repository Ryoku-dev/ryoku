package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

var avoidCursesWords = map[string][]string{
	"apeshit":      {"crazy", "mad", "insane", "wild"},
	"arse":         {"bum", "buttocks", "backside", "bottom", "rump", "posterior"},
	"arses":        {"bums", "buttocks", "backsides", "bottoms", "rumps", "posteriors"},
	"arsed":        {"bothered"},
	"arsehole":     {"bumhole"},
	"ass":          {"butt", "buttocks", "backside", "bottom", "rump", "posterior", "tuchus", "tush"},
	"asses":        {"butts", "buttocks", "backsides", "bottoms", "rumps", "posteriors", "tuchuses", "tushes"},
	"asshole":      {"butthole"},
	"bullshit":     {"bullcrap", "bulldust", "lie", "lies"},
	"bullshitted":  {"bullcrapped", "lied"},
	"bullshitting": {"bullcrapping", "lying"},
	"bullshitter":  {"liar"},
	"chickenshit":  {"gutless", "cowardly"},
	"cock":         {"pee-pee", "willy", "penis", "phallus", "member"},
	"cocks":        {"pee-pees", "willies", "penises", "phalluses", "members"},
	"cunt":         {"vagina"},
	"cunts":        {"vaginas"},
	"dick":         {"pee-pee", "penis"},
	"dicks":        {"pee-pees", "penises"},
	"dickhead":     {"jerk", "idiot"},
	"dichheads":    {"jerks", "idiots"},
	"dumbass":      {"idiot", "fool"},
	"dumbasses":    {"idiots", "fools"},
	"fart":         {"gas", "wind", "break wind"},
	"farts":        {"gas", "wind", "breaks wind"},
	"farted":       {"broke wind", "broken wind"},
	"farting":      {"breaking wind"},
	"fuck":         {"fudge", "screw", "damn", "hoot"},
	"fucks":        {"screws"},
	"fucked":       {"screwed"},
	"fucking":      {"screwing"},
	"fucker":       {"jerk"},
	"fuckers":      {"jerks"},
	"horseshit":    {"nonsense"},
	"piss":         {"pee", "urine", "urinate"},
	"pisses":       {"pees", "urinates"},
	"pissed":       {"peed", "urinated"},
	"pissing":      {"peeing", "urinating"},
	"pisser":       {"toilet", "bathroom", "restroom", "washroom"},
	"shit":         {"crap", "poo", "poop", "feces", "dung", "damn", "hoot"},
	"shits":        {"craps", "poos", "poops"},
	"shitted":      {"crapped", "pooed", "pooped"},
	"shitting":     {"crapping", "pooing", "pooping"},
	"shitless":     {"witless"},
	"shitload":     {"crapload", "shedload", "shirtload", "load", "tons", "pile"},
	"shitloads":    {"craploads", "shedloads", "shirtloads", "loads", "tons", "piles"},
	"shitty":       {"shirty", "crappy", "inferior"},
	"shittier":     {"crappier", "shirtier"},
	"shittiest":    {"crappiest", "shirtiest"},
	"tit":          {"boob", "breast"},
	"tits":         {"boobs", "breasts"},
	"titty":        {"boob", "breast"},
	"titties":      {"boobs", "breasts"},
	"turd":         {"poo", "poop", "feces", "dung"},
	"turds":        {"poos", "poops", "feces", "dung"},
	"twat":         {"vagina"},
	"wanker":       {"jerk"},
	"whore":        {"prostitute"},
}
var avoidCursesMorphemes = [][2]string{{"arse", ""}, {"ass", "a**"}, {"cock", "c**k"}, {"cunt", "c**t"}, {"dick", "d**k"}, {"fuck", "f**k"}, {"piss", "p**s"}, {"shit", "sh*t"}, {"wank", "w**k"}}

type AvoidCurses struct{ expression expr.Expr }

func NewAvoidCurses() *AvoidCurses {
	return &AvoidCurses{expression: expr.NewSequenceExpr().ThenSwear()}
}
func (r *AvoidCurses) Expr() expr.Expr { return r.expression }
func (r *AvoidCurses) Description() string {
	return "Flags offensive language and offers various ways to censor or replace with euphemisms."
}
func (r *AvoidCurses) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AvoidCurses) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) != 1 {
		return nil
	}
	span := toks[0].Span
	raw := src[span.Start:span.End]
	normalized := strings.ToLower(string(raw))
	var suggestions []lint.Suggestion
	for _, m := range avoidCursesMorphemes {
		if strings.Contains(normalized, m[0]) {
			suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(strings.ReplaceAll(normalized, m[0], strings.Repeat("*", len(m[0]))), raw))
			if m[1] != "" {
				suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(strings.ReplaceAll(normalized, m[0], m[1]), raw))
			}
		}
	}
	for _, replacement := range avoidCursesWords[normalized] {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(replacement, raw))
	}
	if len(suggestions) == 0 {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Message = "Try to avoid offensive language."
	result.Suggestions = suggestions
	return &result
}
