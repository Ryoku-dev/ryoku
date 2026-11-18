package dict

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrMalformedItemCount = errors.New("The provided file's item count was malformed.")

// AnnotatedWord is a dictionary spelling and the flags describing its forms.
type AnnotatedWord struct {
	Letters     []rune
	Annotations []rune
}

// ParseWordList parses a Rune word list, ignoring blank lines and comments.
func ParseWordList(source string) ([]AnnotatedWord, error) {
	body, count, err := wordListBody(source)
	if err != nil {
		return nil, err
	}
	words := make([]AnnotatedWord, 0, count)
	walkWordList(body, func(word, attributes string) {
		words = append(words, AnnotatedWord{Letters: []rune(word), Annotations: []rune(attributes)})
	})
	return words, nil
}

func wordListBody(source string) (string, int, error) {
	countText, body, terminated := strings.Cut(source, "\n")
	if terminated {
		countText = strings.TrimSuffix(countText, "\r")
	}
	count, err := strconv.ParseUint(strings.TrimPrefix(countText, "+"), 10, 64)
	if err != nil || count > uint64(^uint(0)>>1) {
		return "", 0, ErrMalformedItemCount
	}
	return body, int(count), nil
}

// walkWordList borrows line text rather than retaining a split line table or
// an annotated-word array throughout dictionary expansion.
func walkWordList(body string, visit func(word, attributes string)) {
	for body != "" {
		line, rest, terminated := strings.Cut(body, "\n")
		body = rest
		if terminated {
			line = strings.TrimSuffix(line, "\r")
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, _, _ := strings.Cut(line, "#")
		entry = strings.TrimRightFunc(entry, unicode.IsSpace)
		word, attributes, _ := strings.Cut(entry, "/")
		visit(word, attributes)
	}
}

type matcherOperatorKind uint8

const (
	matcherLiteral matcherOperatorKind = iota
	matcherOne
	matcherNone
	matcherAny
)

type matcherOperator struct {
	kind  matcherOperatorKind
	chars []rune
}

// Matcher is the simplified, position-based regex syntax used by Rune affixes.
type Matcher struct{ operators []matcherOperator }

func ParseMatcher(source string) (Matcher, error) {
	var matcher Matcher
	chars := []rune(source)
	byteIndex := 0
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		switch c {
		case '[':
			end := i + 1
			for end < len(chars) && chars[end] != ']' {
				end++
			}
			if end == len(chars) {
				return Matcher{}, fmt.Errorf("Unmatched bracket at index: %d", byteIndex)
			}
			content := chars[i+1 : end]
			kind := matcherOne
			if len(content) > 0 && content[0] == '^' {
				kind = matcherNone
				content = content[1:]
			}
			matcher.operators = append(matcher.operators, matcherOperator{kind: kind, chars: content})
			for ; i < end; i++ {
				byteIndex += utf8.RuneLen(chars[i])
			}
		case '.':
			matcher.operators = append(matcher.operators, matcherOperator{kind: matcherAny})
		default:
			matcher.operators = append(matcher.operators, matcherOperator{kind: matcherLiteral, chars: []rune{c}})
		}
		byteIndex += utf8.RuneLen(chars[i])
	}
	return matcher, nil
}
func (m Matcher) Len() int { return len(m.operators) }
func (m Matcher) Matches(chars []rune) bool {
	if len(chars) != len(m.operators) {
		return false
	}
	for i, op := range m.operators {
		if op.kind == matcherAny {
			continue
		}
		contains := false
		for _, c := range op.chars {
			if c == chars[i] {
				contains = true
				break
			}
		}
		if (op.kind == matcherNone && contains) || (op.kind != matcherNone && !contains) {
			return false
		}
	}
	return true
}
func (m Matcher) String() string {
	var out strings.Builder
	for _, op := range m.operators {
		switch op.kind {
		case matcherAny:
			out.WriteByte('.')
		case matcherLiteral:
			out.WriteRune(op.chars[0])
		default:
			out.WriteByte('[')
			if op.kind == matcherNone {
				out.WriteByte('^')
			}
			out.WriteString(string(op.chars))
			out.WriteByte(']')
		}
	}
	return out.String()
}

type AffixEntryKind uint8

const (
	Suffix AffixEntryKind = iota
	Prefix
)

func (k AffixEntryKind) MarshalJSON() ([]byte, error) {
	if k == Suffix {
		return []byte(`"suffix"`), nil
	}
	if k == Prefix {
		return []byte(`"prefix"`), nil
	}
	return nil, fmt.Errorf("unknown affix kind: %d", k)
}
func (k *AffixEntryKind) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch value {
	case "suffix":
		*k = Suffix
	case "prefix":
		*k = Prefix
	default:
		return fmt.Errorf("unknown affix kind: %s", value)
	}
	return nil
}

type AffixReplacement struct {
	Remove, Add []rune
	Condition   Matcher
}
type HumanReadableAffixReplacement struct {
	Remove    string `json:"remove"`
	Add       string `json:"add"`
	Condition string `json:"condition"`
}

func (r AffixReplacement) ToHumanReadable() HumanReadableAffixReplacement {
	return HumanReadableAffixReplacement{Remove: string(r.Remove), Add: string(r.Add), Condition: r.Condition.String()}
}
func (r HumanReadableAffixReplacement) ToNormal() (AffixReplacement, error) {
	condition, err := ParseMatcher(r.Condition)
	if err != nil {
		return AffixReplacement{}, err
	}
	return AffixReplacement{Remove: []rune(r.Remove), Add: []rune(r.Add), Condition: condition}, nil
}
func (r *HumanReadableAffixReplacement) UnmarshalJSON(data []byte) error {
	type plain HumanReadableAffixReplacement
	return decodeRuneRequired(data, (*plain)(r), "remove", "add", "condition")
}

type MetadataExpansion struct {
	Metadata DictWordMetadata  `json:"metadata"`
	IfBase   *DictWordMetadata `json:"if_base,omitempty"`
}

func (e *MetadataExpansion) UnmarshalJSON(data []byte) error {
	type plain MetadataExpansion
	return decodeRuneRequired(data, (*plain)(e), "metadata")
}

// Expansion defines transformations and metadata for a word's derived forms.
type Expansion struct {
	Kind         AffixEntryKind
	CrossProduct bool
	Replacements []AffixReplacement
	Target       []MetadataExpansion
	BaseMetadata DictWordMetadata
}
type HumanReadableExpansion struct {
	Kind         AffixEntryKind                  `json:"kind"`
	CrossProduct bool                            `json:"cross_product"`
	Replacements []HumanReadableAffixReplacement `json:"replacements"`
	Target       []MetadataExpansion             `json:"target"`
	BaseMetadata DictWordMetadata                `json:"base_metadata"`
}

func (e *HumanReadableExpansion) UnmarshalJSON(data []byte) error {
	type plain HumanReadableExpansion
	return decodeRuneRequired(data, (*plain)(e), "kind", "cross_product", "replacements", "target", "base_metadata")
}
func (e HumanReadableExpansion) IntoNormal() (Expansion, error) {
	replacements := make([]AffixReplacement, 0, len(e.Replacements))
	for _, replacement := range e.Replacements {
		normal, err := replacement.ToNormal()
		if err != nil {
			return Expansion{}, err
		}
		replacements = append(replacements, normal)
	}
	return Expansion{Kind: e.Kind, CrossProduct: e.CrossProduct, Replacements: replacements, Target: e.Target, BaseMetadata: e.BaseMetadata}, nil
}
func (e Expansion) IntoHumanReadable() HumanReadableExpansion {
	replacements := make([]HumanReadableAffixReplacement, len(e.Replacements))
	for i, r := range e.Replacements {
		replacements[i] = r.ToHumanReadable()
	}
	return HumanReadableExpansion{Kind: e.Kind, CrossProduct: e.CrossProduct, Replacements: replacements, Target: e.Target, BaseMetadata: e.BaseMetadata}
}

type Property struct {
	Propagate bool             `json:"propagate"`
	Metadata  DictWordMetadata `json:"metadata"`
}

func (p *Property) UnmarshalJSON(data []byte) error {
	type plain Property
	return decodeRuneRequired(data, (*plain)(p), "metadata")
}

func decodeRuneRequired(data []byte, dest any, fields ...string) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, field := range fields {
		value, ok := raw[field]
		if !ok {
			return fmt.Errorf("missing field %q", field)
		}
		if string(value) == "null" {
			return fmt.Errorf("null field %q", field)
		}
	}
	return json.Unmarshal(data, dest)
}

type HumanReadableAttributeList struct {
	Affixes    map[string]HumanReadableExpansion `json:"affixes"`
	Properties map[string]Property               `json:"properties"`
}

func (a *HumanReadableAttributeList) UnmarshalJSON(data []byte) error {
	type plain HumanReadableAttributeList
	return decodeRuneRequired(data, (*plain)(a), "affixes", "properties")
}

type AttributeList struct {
	affixes    map[rune]Expansion
	properties map[rune]Property
}

func ParseAttributeList(source string) (*AttributeList, error) {
	var human HumanReadableAttributeList
	if err := json.Unmarshal([]byte(source), &human); err != nil {
		return nil, err
	}
	return human.IntoNormal()
}
func (h HumanReadableAttributeList) IntoNormal() (*AttributeList, error) {
	attributes := &AttributeList{affixes: make(map[rune]Expansion, len(h.Affixes)), properties: make(map[rune]Property, len(h.Properties))}
	for flag, expansion := range h.Affixes {
		if utf8.RuneCountInString(flag) != 1 {
			return nil, errors.New("Expected affix flag to be exactly one character.")
		}
		key, _ := utf8.DecodeRuneInString(flag)
		normal, err := expansion.IntoNormal()
		if err != nil {
			return nil, err
		}
		attributes.affixes[key] = normal
	}
	for flag, property := range h.Properties {
		if utf8.RuneCountInString(flag) != 1 {
			return nil, errors.New("Expected affix flag to be exactly one character.")
		}
		key, _ := utf8.DecodeRuneInString(flag)
		attributes.properties[key] = property
	}
	return attributes, nil
}
func (a *AttributeList) IntoHumanReadable() HumanReadableAttributeList {
	human := HumanReadableAttributeList{Affixes: make(map[string]HumanReadableExpansion, len(a.affixes)), Properties: make(map[string]Property, len(a.properties))}
	for flag, expansion := range a.affixes {
		human.Affixes[string(flag)] = expansion.IntoHumanReadable()
	}
	for flag, property := range a.properties {
		human.Properties[string(flag)] = property
	}
	return human
}

func applyAffixReplacement(replacement AffixReplacement, letters []rune, kind AffixEntryKind) ([]rune, bool) {
	if replacement.Condition.Len() > len(letters) || len(replacement.Remove) > len(letters) {
		return nil, false
	}
	segment := letters[:replacement.Condition.Len()]
	if kind == Suffix {
		segment = letters[len(letters)-replacement.Condition.Len():]
	}
	if !replacement.Condition.Matches(segment) {
		return nil, false
	}
	start, end := 0, len(letters)
	if kind == Suffix {
		for i, c := range replacement.Remove {
			if letters[len(letters)-len(replacement.Remove)+i] != c {
				return nil, false
			}
		}
		end -= len(replacement.Remove)
	} else {
		for i, c := range replacement.Remove {
			if letters[i] != c {
				return nil, false
			}
		}
		start += len(replacement.Remove)
	}
	out := make([]rune, 0, end-start+len(replacement.Add))
	if kind == Prefix {
		out = append(out, replacement.Add...)
	}
	out = append(out, letters[start:end]...)
	if kind == Suffix {
		out = append(out, replacement.Add...)
	}
	return out, true
}

type conditionalAffixMetadata struct {
	letters             []rune
	metadata, condition DictWordMetadata
}
type expandedAffixWord struct {
	letters  []rune
	metadata DictWordMetadata
}

// Equal UTF-8 keys need no allocated string. Invalid runes use RuneError, just
// as conversion to string does for externally supplied annotated words.
func affixSpellingEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i, c := range a {
		other := b[i]
		if c == other {
			continue
		}
		if !utf8.ValidRune(c) {
			c = utf8.RuneError
		}
		if !utf8.ValidRune(other) {
			other = utf8.RuneError
		}
		if c != other {
			return false
		}
	}
	return true
}

// ExpandAnnotatedWord applies properties, affixes, cross-products and conditional
// metadata, retaining the original spelling as well as all derived forms.
func (a *AttributeList) ExpandAnnotatedWord(word AnnotatedWord, dest *WordMap) {
	dest.Reserve(len(word.Annotations) + 1)
	base := DictWordMetadata{OrthInfo: OrthFlagsFromLetters(word.Letters)}
	sourceID := NewCanonicalWordIdFromWordChars(word.Letters)
	var conditionalStorage [4]conditionalAffixMetadata
	conditional := conditionalStorage[:0]
	for _, flag := range word.Annotations {
		if property, ok := a.properties[flag]; ok {
			base.Merge(property.Metadata)
		}
	}
	for _, flag := range word.Annotations {
		expansion, ok := a.affixes[flag]
		if !ok {
			continue
		}
		base.Merge(expansion.BaseMetadata)
		var generatedStorage [4]expandedAffixWord
		generated := generatedStorage[:0]
		for _, replacement := range expansion.Replacements {
			letters, ok := applyAffixReplacement(replacement, word.Letters, expansion.Kind)
			if !ok {
				continue
			}
			index := 0
			for index < len(generated) && !affixSpellingEqual(generated[index].letters, letters) {
				index++
			}
			if index == len(generated) {
				generated = append(generated, expandedAffixWord{letters: letters})
			}
			for _, target := range expansion.Target {
				if target.IfBase != nil {
					conditional = append(conditional, conditionalAffixMetadata{letters: letters, metadata: target.Metadata, condition: *target.IfBase})
				} else {
					generated[index].metadata.Merge(target.Metadata)
				}
			}
		}
		if expansion.CrossProduct {
			var oppositeStorage [16]rune
			opposite := oppositeStorage[:0]
			for _, flag := range word.Annotations {
				if property, ok := a.properties[flag]; ok && (expansion.Kind == Prefix || property.Propagate) {
					opposite = append(opposite, flag)
				}
			}
			for _, flag := range word.Annotations {
				if other, ok := a.affixes[flag]; ok && other.Kind != expansion.Kind {
					opposite = append(opposite, flag)
				}
			}
			for _, generatedWord := range generated {
				a.ExpandAnnotatedWord(AnnotatedWord{Letters: generatedWord.letters, Annotations: opposite}, dest)
				metadata := dest.GetMetadataMutCanonical(NewCanonicalWordIdFromWordChars(generatedWord.letters))
				metadata.Merge(generatedWord.metadata)
				metadata.DerivedFrom.Insert(sourceID)
			}
		} else {
			for _, generatedWord := range generated {
				generatedWord.metadata.DerivedFrom.Insert(sourceID)
				if existing := dest.GetMetadataMutCanonical(NewCanonicalWordIdFromWordChars(generatedWord.letters)); existing != nil {
					existing.Merge(generatedWord.metadata)
				} else {
					dest.Insert(WordMapEntry{CanonicalSpelling: generatedWord.letters, Metadata: generatedWord.metadata, WordIds: NewWordIdPairFromWordChars(generatedWord.letters)})
				}
			}
		}
	}
	if existing := dest.GetCanonical(sourceID); existing != nil {
		base.Merge(existing.Metadata)
	}
	dest.Insert(WordMapEntry{CanonicalSpelling: word.Letters, Metadata: base, WordIds: NewWordIdPairFromWordChars(word.Letters)})
	for _, pending := range conditional {
		if base.Or(pending.condition).Equal(base) {
			dest.GetMetadataMutCanonical(NewCanonicalWordIdFromWordChars(pending.letters)).Merge(pending.metadata)
		}
	}
}
func (a *AttributeList) ExpandAnnotatedWords(words []AnnotatedWord, dest *WordMap) {
	for _, word := range words {
		a.ExpandAnnotatedWord(word, dest)
	}
}
func expandRuneFiles(wordList, attrs string) ([]WordMapEntry, error) {
	words, err := NewWordMapFromRuneFiles(wordList, attrs)
	if err != nil {
		return nil, err
	}
	return words.Iter(), nil
}
