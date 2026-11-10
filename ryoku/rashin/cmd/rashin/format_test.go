package main

import (
	"bytes"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestTOONUniformObjectArray(t *testing.T) {
	value := struct {
		Items []struct {
			Path string `json:"path"`
			Line int    `json:"line"`
		} `json:"items"`
	}{Items: []struct {
		Path string `json:"path"`
		Line int    `json:"line"`
	}{{"a.go", 3}, {"b.go", 9}}}
	var output bytes.Buffer
	if err := encodeTOON(&output, value); err != nil {
		t.Fatal(err)
	}
	want := "items[2]{path,line}:\n  a.go,3\n  b.go,9\n"
	if output.String() != want {
		t.Fatalf("TOON output:\n%s\nwant:\n%s", output.String(), want)
	}
}
func TestHumanRendererDereferencesResults(t *testing.T) {
	result := &protocol.PeekResult{Path: "main.go", Start: 3, End: 4, Text: "func main() {}"}
	var output bytes.Buffer
	printHuman(&output, result)
	if output.String() != "main.go:3-4\nfunc main() {}\n" {
		t.Fatalf("human output = %q", output.String())
	}
}
func TestTOONQuotesAmbiguousStrings(t *testing.T) {
	value := struct {
		Values []string `json:"values"`
	}{Values: []string{"123", "true", "a,b", "main.go"}}
	var output bytes.Buffer
	if err := encodeTOON(&output, value); err != nil {
		t.Fatal(err)
	}
	want := "values[4]: \"123\",\"true\",\"a,b\",main.go\n"
	if output.String() != want {
		t.Fatalf("TOON output = %q, want %q", output.String(), want)
	}
}
