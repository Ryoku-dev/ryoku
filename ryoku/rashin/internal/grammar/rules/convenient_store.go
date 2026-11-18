package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ConvenientStore struct{ expression expr.Expr }

func NewConvenientStore() *ConvenientStore {
	return &ConvenientStore{expression: expr.NewSequenceExprAco("convenient").TWs().TSet([]string{"store", "stores"})}
}
func (r *ConvenientStore) Expr() expr.Expr { return r.expression }
func (r *ConvenientStore) Description() string {
	return "Attempts to detect when `convenient store` is mistake for `convenience store`."
}
func (r *ConvenientStore) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ConvenientStore) Unit() lint.DocumentIterator      { return lint.Sentence{} }
func (r *ConvenientStore) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}

var convenientStoreNounClues = []string{"11", "24", "7", "99", "alcohol", "around", "beer", "bhd", "brands", "business", "buy", "cafes", "chain", "clerk", "coffee", "coupon", "coupons", "deli", "eleven", "family", "familymart", "groceries", "grocery", "kk", "liquor", "located", "location", "locations", "lotus", "malaysia", "mall", "market", "mart", "marts", "minimart", "ministop", "mrt", "mynews", "near", "nearby", "neighborhood", "neighbourhood", "products", "purchase", "receipt", "restaurant", "restaurants", "retail", "retailers", "sdn", "seven", "shop", "shopping", "shops", "singapore", "snack", "snacks", "sold", "speedmart", "stalls", "station", "supermarket", "supermarkets", "tesco", "traded"}
var convenientStoreAdjClues = []string{"api", "cache", "client", "database", "db", "dbms", "docs", "extension", "gui", "methods", "redis", "server", "stored", "type", "value", "yaml"}

func (r *ConvenientStore) MatchToLintWithContext(t []doc.Token, s []rune, ctx *lint.MatchContext) *lint.Lint {
	noun, adj := 0, 0
	if ctx != nil {
		for _, tokens := range [][]doc.Token{ctx.Before, ctx.After} {
			for _, tok := range tokens {
				if !tok.Kind.IsWordLike() {
					continue
				}
				ch := tok.GetCh(s)
				if text.EqAnyIgnoreASCIICaseStr(ch, convenientStoreNounClues) {
					noun++
				}
				if text.EqAnyIgnoreASCIICaseStr(ch, convenientStoreAdjClues) {
					adj++
				}
			}
		}
	}
	if adj >= noun || len(t) == 0 {
		return nil
	}
	return &lint.Lint{Span: t[0].Span, LintKind: lint.Eggcorn, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("convenience", t[0].GetCh(s))}, Message: "Did you mean `convenience store`?", Priority: 127}
}
