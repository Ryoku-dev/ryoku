package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ThisTypeOfThing struct{ expression expr.Expr }

func NewThisTypeOfThing() *ThisTypeOfThing {
	return &ThisTypeOfThing{expression: expr.NewSequenceExprWordSet([]string{"this", "these", "that", "those"}).TWs().Then(expr.NewSequenceExprWordSet([]string{"kind", "kinds", "sort", "sorts", "type", "types"}).TWs()).TAco("of").TWs().ThenAnyOf([]expr.Expr{expr.NewWordSet([]string{"thing", "things"}), expr.NewSequenceExpr().ThenKindWhere(func(k doc.TokenKind) bool { return k.IsPluralNoun() && !k.IsSingularNoun() })})}
}
func (r *ThisTypeOfThing) Expr() expr.Expr { return r.expression }
func (r *ThisTypeOfThing) Description() string {
	return "Checks that the parts of `this/these type(s) of thing(s)` agree in grammatical number"
}
func (r *ThisTypeOfThing) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThisTypeOfThing) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	det, typ, thing := &t[0], &t[2], &t[len(t)-1]
	detCh, typCh := det.GetCh(s), typ.GetCh(s)
	detPlural := !text.EqAnyIgnoreASCIICaseStr(detCh, []string{"this", "that"})
	typePlural, thingPlural := typ.Kind.IsPluralNoun(), thing.Kind.IsPluralNoun()
	if detPlural == typePlural && typePlural == thingPlural {
		return nil
	}
	proximal := text.EqAnyIgnoreASCIICaseStr(detCh, []string{"this", "these"})
	var bad *doc.Token
	replacement := ""
	switch {
	case detPlural == typePlural:
		bad = thing
		if thingPlural {
			replacement = "thing"
		} else {
			replacement = "things"
		}
	case detPlural == thingPlural:
		bad = typ
		if len(typCh) == 0 {
			return nil
		}
		switch typCh[0] {
		case 'k', 'K':
			replacement = "kind"
		case 's', 'S':
			replacement = "sort"
		case 't', 'T':
			replacement = "type"
		default:
			return nil
		}
		if !typePlural {
			replacement += "s"
		}
	default:
		bad = det
		if proximal {
			if detPlural {
				replacement = "this"
			} else {
				replacement = "these"
			}
		} else {
			if detPlural {
				replacement = "that"
			} else {
				replacement = "those"
			}
		}
	}
	return &lint.Lint{Span: bad.Span, LintKind: lint.Agreement, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, bad.GetCh(s))}, Message: "The grammatical number of the determiner and the two nouns must agree.", Priority: 127}
}
