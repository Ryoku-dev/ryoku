package rules

import (
	_ "embed"
	"encoding/json"
	"ryoku-rashin/internal/grammar/expr"
)

//go:embed proper_noun_rules.json
var properNounCapitalizationLintersData []byte

type properNounCapitalizationLintersRuleEntry struct {
	Canonical   []string `json:"canonical"`
	Description string   `json:"description"`
}

func properNounCapitalizationLintersReadEntries(data []byte) map[string]properNounCapitalizationLintersRuleEntry {
	var entries map[string]properNounCapitalizationLintersRuleEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		panic(err)
	}
	return entries
}

// properNounCapitalizationLintersProperNounsExpr matches every registered proper noun in any capitalization.
func properNounCapitalizationLintersProperNounsExpr() *expr.LongestMatchOf {
	result := expr.NewLongestMatchOf()
	for _, entry := range properNounCapitalizationLintersReadEntries(properNounCapitalizationLintersData) {
		for _, name := range entry.Canonical {
			result.Add(expr.NewFixedPhraseFromPhrase(name))
		}
	}
	return result
}
