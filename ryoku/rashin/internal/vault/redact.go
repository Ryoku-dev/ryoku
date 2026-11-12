package vault

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"slices"
	"sort"
	"strings"

	"go.etcd.io/bbolt"

	"ryoku-rashin/internal/protocol"
)

// minRedacted is the shortest value Redact replaces; a shorter one would
// match ordinary words and say more about the value than it hides.
const minRedacted = 6

// valueForms are the ways a value commonly comes back inside text: as it
// is, in standard and URL-safe base64 with and without padding, escaped for
// a URL query and a URL path, and escaped inside a JSON string with and
// without HTML escaping. Each appears once.
func valueForms(value string) []string {
	if value == "" {
		return nil
	}
	raw := []byte(value)
	forms := []string{
		value,
		base64.StdEncoding.EncodeToString(raw),
		base64.RawStdEncoding.EncodeToString(raw),
		base64.URLEncoding.EncodeToString(raw),
		base64.RawURLEncoding.EncodeToString(raw),
		url.QueryEscape(value),
		url.PathEscape(value),
	}
	for _, html := range []bool{true, false} {
		var b bytes.Buffer
		encoder := json.NewEncoder(&b)
		encoder.SetEscapeHTML(html)
		if encoder.Encode(value) == nil {
			quoted := strings.TrimSuffix(b.String(), "\n")
			forms = append(forms, quoted[1:len(quoted)-1])
		}
	}
	unique := forms[:0]
	for _, form := range forms {
		if form != "" && !slices.Contains(unique, form) {
			unique = append(unique, form)
		}
	}
	return unique
}

// named is one form of a value and the name it is replaced with.
type named struct{ form, name string }

// replaceForms replaces every form in text with its name, longest first so a
// form holding another is replaced whole.
func replaceForms(text string, forms []named) string {
	sort.SliceStable(forms, func(i, j int) bool { return len(forms[i].form) > len(forms[j].form) })
	for _, n := range forms {
		text = strings.ReplaceAll(text, n.form, "‹"+n.name+"›")
	}
	return text
}

// Scrub replaces every form of one value in text with ‹name›, whatever the
// value's length.
func Scrub(text, value, name string) string {
	if text == "" || value == "" {
		return text
	}
	var forms []named
	for _, form := range valueForms(value) {
		forms = append(forms, named{form, name})
	}
	return replaceForms(text, forms)
}

// ValuesScrubber returns a scrubber that replaces every form of each value
// with ‹its name›, as Redact does for the vault's own values: values maps
// each value to its name, and a value too short to tell from an ordinary
// word is left alone. The forms are made once, for every text it scrubs.
func ValuesScrubber(values map[string]string) func(string) string {
	var forms []named
	for value, name := range values {
		if len(value) < minRedacted {
			continue
		}
		for _, form := range valueForms(value) {
			forms = append(forms, named{form, name})
		}
	}
	return formsScrubber(forms)
}

// formsScrubber replaces forms in each text it is given, longest first.
func formsScrubber(forms []named) func(string) string {
	if len(forms) == 0 {
		return func(text string) string { return text }
	}
	sort.SliceStable(forms, func(i, j int) bool { return len(forms[i].form) > len(forms[j].form) })
	return func(text string) string {
		if text == "" {
			return text
		}
		return replaceForms(text, forms)
	}
}

// Redact replaces every form of every current credential value in text with
// ‹NAME›. It opens the values once per call. Without the key no value has
// left the vault, so the text is returned as it is.
func (v *Vault) Redact(text string) string {
	if text == "" {
		return text
	}
	return v.Scrubber()(text)
}

// Scrubber opens every current credential value once and returns what
// Redact does with them, for a pass over many texts. It holds the values,
// so it is dropped when the pass ends. Without the key it changes nothing.
func (v *Vault) Scrubber() func(string) string {
	var forms []named
	aead := v.cipher()
	if v.db != nil && aead != nil {
		_ = v.db.View(func(tx *bbolt.Tx) error {
			sealed := tx.Bucket(bucketValues)
			return tx.Bucket(bucketCredentials).ForEach(func(_, raw []byte) error {
				var c protocol.Credential
				if json.Unmarshal(raw, &c) != nil {
					return nil
				}
				value, err := unseal(aead, c.ID, c.Version, sealed.Get(valueKey(c.ID, c.Version)))
				if err != nil || len(value) < minRedacted {
					return nil
				}
				for _, form := range valueForms(value) {
					forms = append(forms, named{form, c.Name})
				}
				return nil
			})
		})
	}
	return formsScrubber(forms)
}
