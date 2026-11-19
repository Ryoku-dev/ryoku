package i18n

import (
	"embed"
	"encoding/json"
	"sync"
)

// langs.json is the one language table; compiling it in keeps a Go program's
// idea of which languages exist identical to sync.py's and the Hub's, with no
// file to find at runtime. It is a few kilobytes, unlike the catalogs.
//
//go:embed langs.json
var langsFS embed.FS

// Language is one row of the table.
type Language struct {
	Code   string `json:"code"`   // catalog file name, e.g. "pt_BR"
	Name   string `json:"name"`   // English name, e.g. "Portuguese (Brazil)"
	Native string `json:"native"` // the language's own name, for a picker
	Google string `json:"google"` // target code for the keyless Google engine
	Dir    string `json:"dir"`    // "ltr" or "rtl"
	Locale string `json:"locale"` // the glibc locale offered with it
}

var (
	langsOnce sync.Once
	langs     []Language
)

// Languages is the table, in picker order (English first).
func Languages() []Language {
	langsOnce.Do(func() {
		b, err := langsFS.ReadFile("langs.json")
		if err != nil {
			langs = []Language{{Code: "en", Name: "English", Native: "English", Dir: "ltr"}}
			return
		}
		var doc struct {
			Languages []Language `json:"languages"`
		}
		if json.Unmarshal(b, &doc) != nil || len(doc.Languages) == 0 {
			langs = []Language{{Code: "en", Name: "English", Native: "English", Dir: "ltr"}}
			return
		}
		langs = doc.Languages
	})
	return langs
}

// Find returns the row for a catalog code.
func Find(code string) (Language, bool) {
	for _, l := range Languages() {
		if l.Code == code {
			return l, true
		}
	}
	return Language{}, false
}

// IsRTL reports whether a code's script runs right to left.
func IsRTL(code string) bool {
	l, ok := Find(code)
	return ok && l.Dir == "rtl"
}
