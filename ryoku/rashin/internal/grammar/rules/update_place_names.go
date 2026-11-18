package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type UpdatePlaceNamesMapping struct {
	Year     int16
	Name     string
	OldNames []string
}
type UpdatePlaceNames struct {
	expression        *expr.LongestMatchOf
	placeNameMappings []UpdatePlaceNamesMapping
}

func NewUpdatePlaceNamesWithMappings(expression *expr.LongestMatchOf, placeNameMappings []UpdatePlaceNamesMapping) *UpdatePlaceNames {
	return &UpdatePlaceNames{expression, placeNameMappings}
}
func NewUpdatePlaceNames() *UpdatePlaceNames {
	mappings := []UpdatePlaceNamesMapping{
		{1984, "Burkina Faso", []string{"Upper Volta"}},
		{1985, "Côte d'Ivoire", []string{"Ivory Coast"}},
		{2018, "Eswatini", []string{"Swaziland"}},
		{1993, "Kata Tjuta", []string{"The Olgas"}},
		{1993, "Uluru", []string{"Ayers Rock"}},
		{1961, "Dushanbe", []string{"Stalinabad"}},
		{1979, "Beijing", []string{"Peking"}},
		{0, "Guangzhou", []string{"Canton"}},
		{1945, "Taiwan", []string{"Formosa"}},
		{1991, "Ulaanbaatar", []string{"Ulan Bator"}},
		{1945, "Gdańsk", []string{"Danzig"}},
		{1992, "Podgorica", []string{"Titograd"}},
		{1936, "Tbilisi", []string{"Tiflis"}},
		{2022, "Türkiye", []string{"Turkey"}},
		{2006, "Bengaluru", []string{"Bangalore"}},
		{1996, "Chennai", []string{"Madras"}},
		{2007, "Kochi", []string{"Cochin"}},
		{2001, "Kolkata", []string{"Calcutta"}},
		{1995, "Mumbai", []string{"Bombay"}},
		{2014, "Mysuru", []string{"Mysore"}},
		{2006, "Puducherry", []string{"Pondicherry"}},
		{1978, "Pune", []string{"Poona"}},
		{1991, "Thiruvananthapuram", []string{"Trivandrum"}},
		{2013, "CDMX", []string{"DF"}},
		{1997, "Samoa", []string{"Western Samoa"}},
		{1980, "Vanuatu", []string{"New Hebrides"}},
		{1946, "Kaliningrad", []string{"Königsberg"}},
		{1991, "Saint Petersburg", []string{"Leningrad", "Petrograd"}},
		{1961, "Volgograd", []string{"Stalingrad"}},
		{2000, "Busan", []string{"Pusan"}},
		{2018, "Chattogram", []string{"Chittagong"}},
		{1982, "Dhaka", []string{"Dacca"}},
		{1972, "Sri Lanka", []string{"Ceylon"}},
		{1989, "Cambodia", []string{"Kampuchea"}},
		{1976, "Ho Chi Minh City", []string{"Saigon"}},
		{2017, "Melaka", []string{"Malacca"}},
		{1989, "Myanmar", []string{"Burma"}},
		{1939, "Thailand", []string{"Siam"}},
		{2002, "Timor-Leste", []string{"East Timor"}},
		{1989, "Yangon", []string{"Rangoon"}},
		{1992, "Kharkiv", []string{"Kharkov"}},
		{1992, "Kyiv", []string{"Kiev"}},
		{1992, "Luhansk", []string{"Lugansk"}},
		{1992, "Lviv", []string{"Lvov"}},
		{1992, "Odesa", []string{"Odessa"}},
		{1992, "Vinnytsia", []string{"Vinnitsa"}},
		{1992, "Zaporizhzhia", []string{"Zaporozhye"}},
	}
	expression := expr.NewLongestMatchOf()
	for _, m := range mappings {
		for _, old := range m.OldNames {
			expression.Add(expr.NewFixedPhraseFromPhrase(old))
		}
	}
	return NewUpdatePlaceNamesWithMappings(expression, mappings)
}
func (r *UpdatePlaceNames) Expr() expr.Expr                  { return r.expression }
func (r *UpdatePlaceNames) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *UpdatePlaceNames) Description() string {
	return "This rule looks for deprecated place names and offers to update them."
}
func (r *UpdatePlaceNames) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	old := span.GetContentString(src)
	for _, m := range r.placeNameMappings {
		for _, n := range m.OldNames {
			if n == old {
				l := lint.NewLint()
				l.Span = span
				l.LintKind = lint.WordChoice
				l.Priority = 31
				l.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune(m.Name))}
				if m.Year > 0 {
					l.Message = fmt.Sprintf("This place has been officially known as '%s' since %d", m.Name, m.Year)
				} else {
					l.Message = fmt.Sprintf("This place is now officially known as '%s'", m.Name)
				}
				return &l
			}
		}
	}
	return nil
}
