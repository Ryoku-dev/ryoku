package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// upstream serves the official Material Symbols SVGs.
	upstream = "https://raw.githubusercontent.com/google/material-design-icons"
	// commit pins the upstream tree so -check reproduces the same bytes.
	commit          = "10ecc14790612ca8fd164e3f4ffe9c16fc50d5b7"
	listName        = "list.txt"
	outputName      = "symbols.go"
	generatedHeader = "Written by go run ./cmd/lithic-symbols from symbols/list.txt. Do not edit by hand."
)

func main() {
	dir := flag.String("dir", "symbols", "path to the symbols package")
	cache := flag.String("cache", defaultCache(), "directory that keeps fetched SVGs between runs")
	check := flag.Bool("check", false, "report a stale symbols.go without writing")
	flag.Parse()
	if err := run(*dir, *cache, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func defaultCache() string {
	if info, err := os.Stat("/dev/shm"); err == nil && info.IsDir() {
		return "/dev/shm/lithic-symbols"
	}
	return filepath.Join(os.TempDir(), "lithic-symbols")
}

func run(dir, cache string, check bool) error {
	entries, err := readList(filepath.Join(dir, listName))
	if err != nil {
		return err
	}
	source := &fetcher{
		cache:  filepath.Join(cache, commit),
		client: &http.Client{Timeout: 30 * time.Second},
	}
	output, err := generate(entries, source)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, outputName)
	if check {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, output) {
			return fmt.Errorf("stale generated file: %s", path)
		}
		return nil
	}
	if err := os.WriteFile(path, output, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

type symbol struct {
	ident string
	doc   string
	data  []byte
}

type variant struct {
	file  string
	ident string
	doc   string
}

func generate(entries []entry, source *fetcher) ([]byte, error) {
	var symbols []symbol
	for _, item := range entries {
		variants := []variant{{item.name + "_24px.svg", goName(item.name), "the " + item.name + " symbol"}}
		if item.fill {
			variants = append(variants, variant{item.name + "_fill1_24px.svg", goName(item.name) + "Fill", "the filled " + item.name + " symbol"})
		}
		for _, each := range variants {
			svg, err := source.svg(item.name, each.file)
			if err != nil {
				return nil, err
			}
			data, err := convert(svg)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", each.file, err)
			}
			symbols = append(symbols, symbol{ident: each.ident, doc: each.doc, data: data})
		}
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i].ident < symbols[j].ident })
	for index := 1; index < len(symbols); index++ {
		if symbols[index].ident == symbols[index-1].ident {
			return nil, fmt.Errorf("two list entries both generate %s", symbols[index].ident)
		}
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "// %s\n\npackage symbols\n", generatedHeader)
	for _, item := range symbols {
		fmt.Fprintf(&out, "\n// %s is %s.\nvar %s = []byte{", item.ident, item.doc, item.ident)
		for index, value := range item.data {
			if index%16 == 0 {
				out.WriteString("\n")
			}
			fmt.Fprintf(&out, "0x%02x, ", value)
		}
		out.WriteString("\n}\n")
	}
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format %s: %w", outputName, err)
	}
	return formatted, nil
}

// convert turns one Material Symbols SVG into IconVG data.
func convert(svg []byte) ([]byte, error) {
	document, err := parseDocument(svg)
	if err != nil {
		return nil, err
	}
	paths := make([][]segment, 0, len(document.paths))
	for _, data := range document.paths {
		segments, err := parsePath(data)
		if err != nil {
			return nil, err
		}
		paths = append(paths, segments)
	}
	return encode(document.viewBox, paths)
}

// goName spells a snake_case symbol name as an exported Go identifier.
func goName(name string) string {
	var out strings.Builder
	for _, part := range strings.Split(name, "_") {
		out.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return out.String()
}
