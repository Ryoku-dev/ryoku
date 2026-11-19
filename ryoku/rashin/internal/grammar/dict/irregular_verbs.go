package dict

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

type verb [3]string

type IrregularVerbs struct {
	verbs []verb
}

//go:embed irregular_verbs.json
var irregularVerbsJSON string

// uncachedIrregularVerbs produces the original copy of the irregular verb table.
func uncachedIrregularVerbs() *IrregularVerbs {
	verbs, err := IrregularVerbsFromJSONFile(irregularVerbsJSON)
	if err != nil {
		panic(fmt.Sprintf("Failed to load irregular verb table: %v", err))
	}
	return verbs
}

var curatedIrregularVerbs = sync.OnceValue(uncachedIrregularVerbs)

func NewIrregularVerbs() *IrregularVerbs {
	return &IrregularVerbs{}
}

func IrregularVerbsFromJSONFile(source string) (*IrregularVerbs, error) {
	var values []any
	if err := json.Unmarshal([]byte(source), &values); err != nil {
		panic(fmt.Sprintf("Failed to parse irregular verbs JSON: %v", err))
	}
	if values == nil {
		panic("Failed to parse irregular verbs JSON: expected an array")
	}
	verbs := make([]verb, 0, len(values))
	for _, value := range values {
		entry, ok := value.([]any)
		if !ok || len(entry) != 3 {
			continue
		}
		lemma, lemmaOK := entry[0].(string)
		preterite, preteriteOK := entry[1].(string)
		participle, participleOK := entry[2].(string)
		if lemmaOK && preteriteOK && participleOK {
			verbs = append(verbs, verb{lemma, preterite, participle})
		}
	}
	return &IrregularVerbs{verbs: verbs}, nil
}

func CuratedIrregularVerbs() *IrregularVerbs {
	return curatedIrregularVerbs()
}

func (v *IrregularVerbs) GetPastParticipleForPreterite(preterite string) (string, bool) {
	for _, entry := range v.verbs {
		if irregularStringsEqualFoldASCII(entry[1], preterite) {
			return entry[2], true
		}
	}
	return "", false
}

func (v *IrregularVerbs) GetLemmaForPreterite(preterite string) (string, bool) {
	for _, entry := range v.verbs {
		if irregularStringsEqualFoldASCII(entry[1], preterite) {
			return entry[0], true
		}
	}
	return "", false
}

func (v *IrregularVerbs) GetPreteriteForLemma(lemma string) (string, bool) {
	for _, entry := range v.verbs {
		if irregularStringsEqualFoldASCII(entry[0], lemma) {
			return entry[1], true
		}
	}
	return "", false
}

func (v *IrregularVerbs) GetPastParticipleForLemma(lemma string) (string, bool) {
	for _, entry := range v.verbs {
		if irregularStringsEqualFoldASCII(entry[0], lemma) {
			return entry[2], true
		}
	}
	return "", false
}

func (v *IrregularVerbs) GetPastsForLemma(lemma string) ([2]string, bool) {
	for _, entry := range v.verbs {
		if irregularStringsEqualFoldASCII(entry[0], lemma) {
			return [2]string{entry[1], entry[2]}, true
		}
	}
	return [2]string{}, false
}
