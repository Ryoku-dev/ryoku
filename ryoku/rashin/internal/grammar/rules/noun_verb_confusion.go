package rules

import "ryoku-rashin/internal/grammar/lint"

type NounVerbConfusion struct{ *mergeLintersLinter }

func NewNounVerbConfusion() *NounVerbConfusion {
	return &NounVerbConfusion{&mergeLintersLinter{rules: []lint.Linter{NewNounInsteadOfVerb(), NewVerbInsteadOfNoun()}, description: "Handles common confusions between related nouns and verbs (e.g., 'advice/advise', 'breath/breathe')"}}
}

var nounVerbConfusionNounVerbPairs = [][2]string{{"advice", "advise"}, {"belief", "believe"}, {"breath", "breathe"}, {"descent", "descend"}, {"effect", "affect"}, {"emphasis", "emphasize"}, {"intent", "intend"}, {"weight", "weigh"}}
