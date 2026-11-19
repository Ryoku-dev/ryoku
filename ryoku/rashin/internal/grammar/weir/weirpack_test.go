package weir

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestRoundTripWeirpackBytes(t *testing.T) {
	manifest := NewWeirpackManifest()
	manifest.SetAuthor("Test Author")
	manifest.SetVersion("0.1.0")
	manifest.SetDescription("Test pack")
	manifest.SetLicense("MIT")
	pack := NewWeirpack(manifest)
	pack.AddRule("ExampleRule", "expr main test")
	data, err := pack.ToBytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := WeirpackFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]string{"author": "Test Author", "version": "0.1.0", "description": "Test pack", "license": "MIT"} {
		got, err := parsed.Manifest.required(field)
		if err != nil || got != want {
			t.Fatalf("%s=%q,%v; want %q", field, got, err, want)
		}
	}
	if got := parsed.Rules["ExampleRule"]; got != "expr main test" {
		t.Fatalf("rule=%q", got)
	}
}
func TestWeirpackManifestValidation(t *testing.T) {
	for _, s := range []string{`{}`, `{"author":1}`, `{"author":"a","version":"v","description":"d"}`, `[]`, `null`} {
		if _, err := WeirpackManifestFromReader(strings.NewReader(s)); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
func TestWeirpackNestedRuleNamesAndDuplicates(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, entry := range [][2]string{{"manifest.json", `{"author":"a","version":"v","description":"d","license":"l"}`}, {"nested/Name.weir", "expr main first"}, {"other/Name.weir", "expr main second"}, {"dictionary.dict", "dictionary"}, {"annotations.json", "annotations"}, {"ignored.txt", "ignored"}} {
		w, err := z.Create(entry[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(entry[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	p, err := WeirpackFromReader(&b)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rules) != 1 || p.Rules["Name"] != "expr main second" {
		t.Fatal(p.Rules)
	}
	if p.Dictionary == nil || *p.Dictionary != "dictionary" || p.Annotations == nil || *p.Annotations != "annotations" {
		t.Fatal("missing optional dictionary")
	}
	if old, ok := p.AddRule("Name", "replacement"); !ok || old != "expr main second" {
		t.Fatal(old, ok)
	}
	if old, ok := p.RemoveRule("Name"); !ok || old != "replacement" {
		t.Fatal(old, ok)
	}
}
func TestWeirpackRejectsDuplicateAndMissingManifest(t *testing.T) {
	for _, count := range []int{0, 2} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		for range count {
			w, err := z.Create("manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			_, err = w.Write([]byte(`{"author":"a","version":"v","description":"d","license":"l"}`))
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := WeirpackFromBytes(b.Bytes()); err == nil {
			t.Errorf("accepted manifest count %d", count)
		}
	}
}
