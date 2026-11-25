package dict

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"ryoku-rashin/internal/grammar/text"
)

type noun [2]string

type IrregularNouns struct {
	nouns []noun
}

//go:embed irregular_nouns.json
var irregularNounsJSON string

// uncachedIrregularNouns produces the original copy of the irregular noun table.
func uncachedIrregularNouns() *IrregularNouns {
	nouns, err := IrregularNounsFromJSONFile(irregularNounsJSON)
	if err != nil {
		panic(fmt.Sprintf("Failed to load irregular noun table: %v", err))
	}
	return nouns
}

var curatedIrregularNouns = sync.OnceValue(uncachedIrregularNouns)

func NewIrregularNouns() *IrregularNouns {
	return &IrregularNouns{}
}

func IrregularNounsFromJSONFile(source string) (*IrregularNouns, error) {
	var values []any
	if err := json.Unmarshal([]byte(source), &values); err != nil {
		panic(fmt.Sprintf("Failed to parse irregular nouns JSON: %v", err))
	}
	if values == nil {
		panic("Failed to parse irregular nouns JSON: expected an array")
	}
	nouns := make([]noun, 0, len(values))
	for _, value := range values {
		entry, ok := value.([]any)
		if !ok || len(entry) != 2 {
			continue
		}
		singular, singularOK := entry[0].(string)
		plural, pluralOK := entry[1].(string)
		if singularOK && pluralOK {
			nouns = append(nouns, noun{singular, plural})
		}
	}
	return &IrregularNouns{nouns: nouns}, nil
}

func CuratedIrregularNouns() *IrregularNouns {
	return curatedIrregularNouns()
}

func (n *IrregularNouns) GetPluralForSingularChars(singular []rune) (string, bool) {
	for _, entry := range n.nouns {
		if text.EqStr(singular, entry[0]) {
			return entry[1], true
		}
	}
	return "", false
}

func (n *IrregularNouns) GetPluralForSingular(singular string) (string, bool) {
	for _, entry := range n.nouns {
		if irregularStringsEqualFoldASCII(entry[0], singular) {
			return entry[1], true
		}
	}
	return "", false
}

func (n *IrregularNouns) GetSingularForPlural(plural string) (string, bool) {
	for _, entry := range n.nouns {
		if irregularStringsEqualFoldASCII(entry[1], plural) {
			return entry[0], true
		}
	}
	return "", false
}

func (n *IrregularNouns) GetSingularForPluralChars(plural []rune) (string, bool) {
	for _, entry := range n.nouns {
		if text.EqStr(plural, entry[1]) {
			return entry[0], true
		}
	}
	return "", false
}

func irregularStringsEqualFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		left, right := a[i], b[i]
		if left >= 'A' && left <= 'Z' {
			left += 'a' - 'A'
		}
		if right >= 'A' && right <= 'Z' {
			right += 'a' - 'A'
		}
		if left != right {
			return false
		}
	}
	return true
}
