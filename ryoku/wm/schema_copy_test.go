package wm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Settings shows every provider row's label, description and option labels to
// people who never read a config file, so none may carry a dash for a pause, a
// command, a file path, a raw config token, a key chord the user can rebind or
// display-server jargon, and all of it is in the British spelling the rest of
// Ryoku's interface uses.
var rawCopy = regexp.MustCompile("\u2014|\u2013|`|\\bryoku-[a-z]|~/|\\$HOME|[a-z]+_[a-z]+|" +
	`(?i)\b(super|alt|ctrl|mod) ?\+ ?(tab|[a-z0-9]\b)|\b(x11|xwayland|wayland)\b`)
var usSpelling = regexp.MustCompile(`(?i)\b(colors?|colored|center(ed|s)?|behaviors?|gray|favorites?|` +
	`(maxim|minim|custom|organ|recogn|optim|initial|priorit|synchron|normal|personal|visual|final|author)iz(e|ed|es|ing))\b`)

func TestProviderRowCopyIsPlain(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("*", "schema*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no provider schema files: %v", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var rows []struct {
			Key       string            `json:"key"`
			Label     string            `json:"label"`
			Desc      string            `json:"desc"`
			Opts      []any             `json:"opts"`
			OptLabels map[string]string `json:"optLabels"`
		}
		if err := json.Unmarshal(b, &rows); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, r := range rows {
			shown := []string{r.Label, r.Desc}
			for _, l := range r.OptLabels {
				shown = append(shown, l)
			}
			// an option with no label shows its value
			for _, o := range r.Opts {
				if v, ok := o.(string); ok && r.OptLabels[v] == "" {
					shown = append(shown, v)
				}
			}
			for _, s := range shown {
				if m := rawCopy.FindString(s); m != "" {
					t.Errorf("%s %s: %q shows %q", f, r.Key, s, m)
				}
				if m := usSpelling.FindString(s); m != "" {
					t.Errorf("%s %s: %q spells %q the US way", f, r.Key, s, m)
				}
			}
			if r.Label != "" && strings.ToUpper(r.Label[:1]) != r.Label[:1] {
				t.Errorf("%s %s: label %q does not start with a capital", f, r.Key, r.Label)
			}
		}
	}
}
