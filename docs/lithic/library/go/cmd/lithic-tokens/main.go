package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const generatedHeader = "Written by go run ./cmd/lithic-tokens from tokens.json. Do not edit by hand."

type document map[string]any

type staleError struct {
	files []string
}

func (e *staleError) Error() string {
	return "stale generated files: " + strings.Join(e.files, ", ")
}

func main() {
	root := flag.String("root", "../..", "path to docs/lithic")
	check := flag.Bool("check", false, "report stale outputs without writing")
	flag.Parse()
	if err := run(*root, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string, check bool) error {
	tokens, err := readTokens(filepath.Join(root, "tokens.json"))
	if err != nil {
		return err
	}
	outputs, err := generate(tokens)
	if err != nil {
		return err
	}
	paths := sortedKeys(outputs)
	if check {
		var stale []string
		for _, name := range paths {
			path := filepath.Join(root, filepath.FromSlash(name))
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, outputs[name]) {
				stale = append(stale, name)
			}
		}
		if len(stale) != 0 {
			return &staleError{files: stale}
		}
		return nil
	}
	for _, name := range paths {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, outputs[name], 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func readTokens(path string) (document, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var tokens document
	if err := decoder.Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, fmt.Errorf("decode %s: trailing data", path)
	}
	return tokens, nil
}

func generate(tokens document) (map[string][]byte, error) {
	qml, err := qmlOutput(tokens)
	if err != nil {
		return nil, err
	}
	css, err := cssOutput(tokens)
	if err != nil {
		return nil, err
	}
	javascript, err := jsOutput(tokens)
	if err != nil {
		return nil, err
	}
	goSource, err := goOutput(tokens)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"library/go/tokens/tokens.go":         goSource,
		"library/qml/Lithic/LithicTokens.qml": qml,
		"library/web/tokens.css":              css,
		"library/web/tokens.js":               javascript,
	}, nil
}

func qmlOutput(tokens document) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "// %s\n", generatedHeader)
	out.WriteString("pragma Singleton\n\nimport QtQuick\n\nQtObject {\n")
	varGroups := map[string]bool{
		"alpha": true, "dye": true, "edition": true, "minerals": true, "palette": true, "signature": true, "tone": true,
	}
	for _, key := range sortedKeys(tokens) {
		if key == "sizeNotes" {
			continue
		}
		if qmlSignalHandlerName(key) {
			return nil, fmt.Errorf("qml: token key %s is a signal-handler name", key)
		}
		value := tokens[key]
		if varGroups[key] {
			encoded, err := compactJSON(qmlObjectValue(value))
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&out, "    readonly property var %s: (%s)\n", key, encoded)
			continue
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("qml: top-level %s is not an object", key)
		}
		if err := writeQMLObject(&out, key, object, 1, []string{key}); err != nil {
			return nil, err
		}
	}
	out.WriteString("}\n")
	return out.Bytes(), nil
}

func writeQMLObject(out *bytes.Buffer, name string, object map[string]any, depth int, path []string) error {
	indent := strings.Repeat("    ", depth)
	fmt.Fprintf(out, "%sreadonly property QtObject %s: QtObject {\n", indent, name)
	for _, key := range sortedKeys(object) {
		value := object[key]
		childPath := append(append([]string{}, path...), key)
		if qmlSignalHandlerName(key) {
			return fmt.Errorf("qml: token key %s is a signal-handler name", strings.Join(childPath, "."))
		}
		if child, ok := value.(map[string]any); ok {
			if err := writeQMLObject(out, key, child, depth+1, childPath); err != nil {
				return err
			}
			continue
		}
		propertyType := qmlPropertyType(value, childPath)
		encoded, _ := compactJSON(qmlValue(value))
		fmt.Fprintf(out, "%s    readonly property %s %s: %s\n", indent, propertyType, key, encoded)
	}
	fmt.Fprintf(out, "%s}\n", indent)
	return nil
}

func qmlPropertyType(value any, path []string) string {
	switch value := value.(type) {
	case string:
		if strings.HasPrefix(value, "#") {
			return "color"
		}
		return "string"
	case json.Number:
		if path[0] == "alpha" || path[0] == "contrast" || strings.Contains(value.String(), ".") {
			return "real"
		}
		return "int"
	case []any, map[string]any:
		return "var"
	default:
		return "var"
	}
}

func qmlValue(value any) any {
	color, ok := value.(string)
	if !ok || len(color) != 9 || color[0] != '#' {
		return value
	}
	return "#" + color[7:9] + color[1:7]
}

func qmlObjectValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		object := make(map[string]any, len(value))
		for key, child := range value {
			object[key] = qmlObjectValue(child)
		}
		return object
	case []any:
		list := make([]any, len(value))
		for i, child := range value {
			list[i] = qmlObjectValue(child)
		}
		return list
	default:
		return qmlValue(value)
	}
}

func qmlSignalHandlerName(key string) bool {
	return len(key) > 2 && key[0] == 'o' && key[1] == 'n' && key[2] >= 'A' && key[2] <= 'Z'
}

func cssOutput(tokens document) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "/* %s */\n", generatedHeader)
	out.WriteString(":root {\n")
	fonts, err := objectAt(tokens, "font")
	if err != nil {
		return nil, err
	}
	for _, key := range sortedKeys(fonts) {
		families, ok := fonts[key].([]any)
		if !ok {
			return nil, fmt.Errorf("css: font.%s is not a list", key)
		}
		fallback := "sans-serif"
		if key == "mono" {
			fallback = "monospace"
		}
		var names []string
		for _, family := range families {
			name, ok := family.(string)
			if !ok {
				return nil, fmt.Errorf("css: font.%s contains a non-string", key)
			}
			names = append(names, strconv.Quote(name))
		}
		names = append(names, fallback)
		fmt.Fprintf(&out, "  --lithic-font-%s: %s;\n", kebab(key), strings.Join(names, ", "))
	}
	for _, group := range []string{"space", "cut", "breakpoint", "size"} {
		values, err := objectAt(tokens, group)
		if err != nil {
			return nil, err
		}
		for _, key := range sortedKeys(values) {
			number, ok := values[key].(json.Number)
			if !ok {
				return nil, fmt.Errorf("css: %s.%s is not a number", group, key)
			}
			unit := "px"
			if group == "size" && unitlessSize(key) {
				unit = ""
			}
			fmt.Fprintf(&out, "  --lithic-%s-%s: %s%s;\n", group, kebab(key), number.String(), unit)
		}
	}
	types, err := objectAt(tokens, "type")
	if err != nil {
		return nil, err
	}
	for _, style := range sortedKeys(types) {
		entry, ok := types[style].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("css: type.%s is not an object", style)
		}
		family, _ := entry["family"].(string)
		fmt.Fprintf(&out, "  --lithic-type-%s-family: var(--lithic-font-%s);\n", kebab(style), kebab(family))
		for _, key := range []string{"line", "size", "weight"} {
			number, ok := entry[key].(json.Number)
			if !ok {
				return nil, fmt.Errorf("css: type.%s.%s is not a number", style, key)
			}
			unit := ""
			if key != "weight" {
				unit = "px"
			}
			fmt.Fprintf(&out, "  --lithic-type-%s-%s: %s%s;\n", kebab(style), kebab(key), number.String(), unit)
		}
	}
	motion, err := objectAt(tokens, "motion")
	if err != nil {
		return nil, err
	}
	for _, key := range sortedKeys(motion) {
		if key == "curve" {
			curve, ok := motion[key].([]any)
			if !ok || len(curve) != 4 {
				return nil, errors.New("css: motion.curve must contain four numbers")
			}
			parts := make([]string, len(curve))
			for i, value := range curve {
				number, ok := value.(json.Number)
				if !ok {
					return nil, errors.New("css: motion.curve must contain four numbers")
				}
				parts[i] = number.String()
			}
			fmt.Fprintf(&out, "  --lithic-motion-curve: cubic-bezier(%s);\n", strings.Join(parts, ", "))
			continue
		}
		number, ok := motion[key].(json.Number)
		if !ok {
			return nil, fmt.Errorf("css: motion.%s is not a number", key)
		}
		fmt.Fprintf(&out, "  --lithic-motion-%s: %sms;\n", kebab(key), number.String())
	}
	for _, group := range []string{"alpha", "contrast"} {
		values, err := objectAt(tokens, group)
		if err != nil {
			return nil, err
		}
		for _, key := range sortedKeys(values) {
			number, ok := values[key].(json.Number)
			if !ok {
				return nil, fmt.Errorf("css: %s.%s is not a number", group, key)
			}
			fmt.Fprintf(&out, "  --lithic-%s-%s: %s;\n", group, kebab(key), number.String())
		}
	}
	out.WriteString("}\n")
	return out.Bytes(), nil
}

func jsOutput(tokens document) ([]byte, error) {
	outputTokens := make(document, len(tokens))
	for key, value := range tokens {
		if key != "sizeNotes" {
			outputTokens[key] = value
		}
	}
	encoded, err := json.MarshalIndent(outputTokens, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("javascript: %w", err)
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "// %s\nexport default ", generatedHeader)
	out.Write(encoded)
	out.WriteString(";\n")
	return out.Bytes(), nil
}

func goOutput(tokens document) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "// %s\n", generatedHeader)
	out.WriteString("package tokens\n\nimport \"time\"\n\n")
	out.WriteString("type Type struct {\nFamily string\nWeight int\nSize float32\nLine float32\n}\n\n")
	for _, group := range []string{"space", "cut", "breakpoint", "size"} {
		values, err := objectAt(tokens, group)
		if err != nil {
			return nil, err
		}
		out.WriteString("const (\n")
		for _, key := range sortedKeys(values) {
			number, ok := values[key].(json.Number)
			if !ok {
				return nil, fmt.Errorf("go: %s.%s is not a number", group, key)
			}
			fmt.Fprintf(&out, "%s%s = %s\n", pascal(group), pascal(key), number.String())
		}
		out.WriteString(")\n\n")
	}
	motion, err := objectAt(tokens, "motion")
	if err != nil {
		return nil, err
	}
	out.WriteString("const (\n")
	for _, key := range sortedKeys(motion) {
		if key == "curve" {
			continue
		}
		number, ok := motion[key].(json.Number)
		if !ok {
			return nil, fmt.Errorf("go: motion.%s is not a number", key)
		}
		fmt.Fprintf(&out, "Motion%s time.Duration = %s * time.Millisecond\n", pascal(key), number.String())
	}
	out.WriteString(")\n\n")
	curve, ok := motion["curve"].([]any)
	if !ok || len(curve) != 4 {
		return nil, errors.New("go: motion.curve must contain four numbers")
	}
	out.WriteString("var MotionCurve = [4]float32{")
	for i, value := range curve {
		if i > 0 {
			out.WriteString(", ")
		}
		number, ok := value.(json.Number)
		if !ok {
			return nil, errors.New("go: motion.curve must contain four numbers")
		}
		out.WriteString(number.String())
	}
	out.WriteString("}\n\n")
	for _, group := range []string{"alpha", "contrast"} {
		values, err := objectAt(tokens, group)
		if err != nil {
			return nil, err
		}
		out.WriteString("const (\n")
		for _, key := range sortedKeys(values) {
			number, ok := values[key].(json.Number)
			if !ok {
				return nil, fmt.Errorf("go: %s.%s is not a number", group, key)
			}
			fmt.Fprintf(&out, "%s%s float32 = %s\n", pascal(group), pascal(key), number.String())
		}
		out.WriteString(")\n\n")
	}
	fonts, err := objectAt(tokens, "font")
	if err != nil {
		return nil, err
	}
	out.WriteString("var (\n")
	for _, key := range sortedKeys(fonts) {
		values, ok := fonts[key].([]any)
		if !ok {
			return nil, fmt.Errorf("go: font.%s is not a list", key)
		}
		fmt.Fprintf(&out, "Font%s = []string{", pascal(key))
		for i, value := range values {
			if i > 0 {
				out.WriteString(", ")
			}
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("go: font.%s contains a non-string", key)
			}
			out.WriteString(strconv.Quote(text))
		}
		out.WriteString("}\n")
	}
	out.WriteString(")\n\n")
	types, err := objectAt(tokens, "type")
	if err != nil {
		return nil, err
	}
	out.WriteString("var (\n")
	for _, key := range sortedKeys(types) {
		entry, ok := types[key].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("go: type.%s is not an object", key)
		}
		family, ok := entry["family"].(string)
		if !ok {
			return nil, fmt.Errorf("go: type.%s.family is not a string", key)
		}
		weight, _ := entry["weight"].(json.Number)
		size, _ := entry["size"].(json.Number)
		line, _ := entry["line"].(json.Number)
		fmt.Fprintf(&out, "Type%s = Type{Family: %q, Weight: %s, Size: %s, Line: %s}\n", pascal(key), family, weight.String(), size.String(), line.String())
	}
	out.WriteString(")\n\n")
	for _, group := range []string{"signature", "minerals", "palette"} {
		values, err := objectAt(tokens, group)
		if err != nil {
			return nil, err
		}
		for _, key := range sortedKeys(values) {
			entries, ok := values[key].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("go: %s.%s is not an object", group, key)
			}
			writeGoStringMap(&out, pascal(group)+pascal(key), entries)
		}
	}
	for _, group := range []string{"tone", "dye"} {
		values, err := objectAt(tokens, group)
		if err != nil {
			return nil, err
		}
		for _, key := range sortedKeys(values) {
			entries, ok := values[key].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("go: %s.%s is not an object", group, key)
			}
			if err := writeGoFloatMap(&out, pascal(group)+pascal(key), entries); err != nil {
				return nil, fmt.Errorf("go: %s.%s: %w", group, key, err)
			}
		}
	}
	edition, err := objectAt(tokens, "edition")
	if err != nil {
		return nil, err
	}
	writeGoStringMap(&out, "Edition", edition)
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated Go: %w", err)
	}
	return formatted, nil
}

func writeGoStringMap(out *bytes.Buffer, name string, entries map[string]any) {
	fmt.Fprintf(out, "var %s = map[string]string{\n", name)
	for _, key := range sortedKeys(entries) {
		value, _ := entries[key].(string)
		fmt.Fprintf(out, "%q: %q,\n", key, value)
	}
	out.WriteString("}\n\n")
}

func writeGoFloatMap(out *bytes.Buffer, name string, entries map[string]any) error {
	fmt.Fprintf(out, "var %s = map[string]float64{\n", name)
	for _, key := range sortedKeys(entries) {
		number, ok := entries[key].(json.Number)
		if !ok {
			return fmt.Errorf("%s is not a number", key)
		}
		fmt.Fprintf(out, "%q: %s,\n", key, number.String())
	}
	out.WriteString("}\n\n")
	return nil
}

func objectAt(tokens document, key string) (map[string]any, error) {
	value, ok := tokens[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tokens.%s is not an object", key)
	}
	return value, nil
}

func compactJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func kebab(value string) string {
	var out strings.Builder
	for i, r := range value {
		if unicode.IsUpper(r) {
			if i > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(unicode.ToLower(r))
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func pascal(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func unitlessSize(key string) bool {
	return strings.HasSuffix(key, "Count") || strings.HasSuffix(key, "Fraction") || strings.HasSuffix(key, "Ratio") || strings.HasSuffix(key, "Columns")
}
