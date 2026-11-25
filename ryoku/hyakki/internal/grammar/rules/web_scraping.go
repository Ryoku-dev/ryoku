package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type WebScraping struct{ expression expr.Expr }

func NewWebScraping() *WebScraping {
	verbs := []string{"scrap", "scrapped", "scraps", "scrapping"}
	nouns := []string{"scrapper", "scrappers"}
	closed := expr.NewWordSet()
	var open []expr.Expr
	for _, group := range [][]string{verbs, nouns} {
		for _, scrap := range group {
			closed.Add("web" + scrap)
			open = append(open, expr.NewSequenceExprAco("web").TWsH().TAco(scrap))
		}
	}
	scrapables := []string{"article", "articles", "content", "doc", "docs", "document", "documents", "dom", "html", "info", "information", "page", "pages", "site", "sites", "text", "web", "webpage", "webpages", "website", "websites"}
	scrapWeb := expr.NewSequenceExprWordSet(verbs).TWs().ThenZeroOrMore(expr.NewSequenceExprWith(func(t doc.Token, s []rune) bool {
		return t.Kind.IsWord() && !text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), scrapables)
	}).TWs()).ThenWordSet(scrapables)
	return &WebScraping{expr.NewFirstMatchOf([]expr.Expr{expr.NewFirstMatchOf([]expr.Expr{expr.AsExpr(closed), expr.NewFirstMatchOf(open)}), scrapWeb})}
}
func (r *WebScraping) Expr() expr.Expr                  { return r.expression }
func (r *WebScraping) Description() string              { return "Corrects `scrapping` the web to `scraping`." }
func (r *WebScraping) Unit() lint.DocumentIterator      { return lint.Sentence{} }
func (r *WebScraping) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func webScrapingCorrect(scrap []rune) []rune {
	corrected := make([]rune, 0, len(scrap)+1)
	n := min(5, len(scrap))
	corrected = append(corrected, scrap[:n]...)
	if len(scrap) == 5 || len(scrap) == 6 {
		corrected = append(corrected, 'e')
		corrected = append(corrected, scrap[n:]...)
	} else {
		corrected = append(corrected, scrap[min(6, len(scrap)):]...)
	}
	return corrected
}
func (r *WebScraping) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	first := toks[0].GetCh(src)
	if len(first) == 0 {
		return nil
	}
	var replacement []rune
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	switch first[0] {
	case 'w', 'W':
		if len(toks) != 1 && len(toks) != 3 {
			return nil
		}
		if len(toks) == 1 {
			replacement = append(replacement, first[:3]...)
			replacement = append(replacement, webScrapingCorrect(first[3:])...)
		} else {
			replacement = append(replacement, first...)
			replacement = append(replacement, toks[1].GetCh(src)...)
			replacement = append(replacement, webScrapingCorrect(toks[2].GetCh(src))...)
		}
	case 's', 'S':
		span = toks[0].Span
		replacement = webScrapingCorrect(first)
	default:
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Eggcorn
	result.Message = "`Scrap` means `discard`. The word for gathering information from websites is `scrape`."
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(replacement, src[span.Start:span.End])}
	return &result
}
