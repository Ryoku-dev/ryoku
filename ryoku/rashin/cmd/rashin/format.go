package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

func defaultOutputFormat() string {
	info, err := os.Stdout.Stat()
	if err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return "human"
	}
	return "toon"
}
func printFormatted(value any, format string) int {
	switch format {
	case "json":
		return printJSON(value)
	case "toon":
		if err := encodeTOON(os.Stdout, value); err != nil {
			return report(err)
		}
		return 0
	case "human":
		printHuman(os.Stdout, value)
		return 0
	default:
		return report(fmt.Errorf("%s", i18n.Tf("Unknown format: %s", format)))
	}
}

func printHuman(writer io.Writer, value any) {
	reflected := reflect.ValueOf(value)
	if reflected.IsValid() && reflected.Kind() == reflect.Pointer && !reflected.IsNil() {
		value = reflected.Elem().Interface()
	}
	switch result := value.(type) {
	case protocol.IndexesResult:
		printIndexTable(writer, result.Indexes)
	case protocol.IndexInfo:
		printIndexTable(writer, []protocol.IndexInfo{result})
	case protocol.SearchResult:
		for _, hit := range result.Hits {
			fmt.Fprintf(writer, "%s:%d-%d  %.4f", hit.Path, hit.Start, hit.End, hit.Score)
			if hit.Symbol != "" {
				fmt.Fprintf(writer, "  %s", hit.Symbol)
			}
			fmt.Fprintln(writer)
			if hit.Snippet != "" {
				for _, line := range strings.Split(hit.Snippet, "\n") {
					fmt.Fprintf(writer, "  %s\n", line)
				}
			}
		}
	case protocol.SymbolsResult:
		for _, symbol := range result.Symbols {
			fmt.Fprintf(writer, "%-24s %-10s %s:%d-%d", symbol.Name, symbol.Kind, symbol.Path, symbol.Start, symbol.End)
			if symbol.Signature != "" {
				fmt.Fprintf(writer, "  %s", symbol.Signature)
			}
			fmt.Fprintln(writer)
		}
	case protocol.DefResult:
		printDefinition(writer, result)
	case protocol.ReferencesResult:
		for _, reference := range result.References {
			fmt.Fprintf(writer, "%s:%d  %s\n", reference.Path, reference.Line, reference.Text)
		}
	case protocol.PeekResult:
		fmt.Fprintf(writer, "%s:%d-%d\n%s\n", result.Path, result.Start, result.End, result.Text)
	case protocol.Overview:
		printOverview(writer, result)
	case protocol.Impact:
		fmt.Fprint(writer, i18n.Tf("%s: %d dependents\n", result.Path, result.Total))
		for _, item := range result.Direct {
			fmt.Fprintf(writer, "  %s\n", item)
		}
		if len(result.All) > len(result.Direct) {
			fmt.Fprintln(writer, i18n.T("All:"))
			for _, item := range result.All {
				fmt.Fprintf(writer, "  %s\n", item)
			}
		}
	case protocol.WipResult:
		for _, change := range result.Changes {
			fmt.Fprintf(writer, "%-10s %-6d %s\n", change.Status, change.Dependents, change.Path)
			for _, marker := range change.Markers {
				fmt.Fprintf(writer, "  %s\n", marker)
			}
		}
	default:
		encoded, _ := json.MarshalIndent(value, "", "  ")
		fmt.Fprintln(writer, string(encoded))
	}
}
func printIndexTable(writer io.Writer, indexes []protocol.IndexInfo) {
	for _, index := range indexes {
		fmt.Fprint(writer, i18n.Tf("%-14s %-24s %-10s %6d files %6d chunks %6d vectors\n", index.ID, index.Name, index.State, index.Files, index.Chunks, index.Vectors))
		if index.Error != "" {
			fmt.Fprint(writer, i18n.Tf("  Error: %s\n", index.Error))
		}
	}
}
func printDefinition(writer io.Writer, result protocol.DefResult) {
	fmt.Fprintf(writer, "%s:%d-%d  %s %s\n", result.Symbol.Path, result.Symbol.Start, result.Symbol.End, result.Symbol.Kind, result.Symbol.Name)
	fmt.Fprintln(writer, result.Body)
	if len(result.Others) > 0 {
		fmt.Fprintln(writer, i18n.T("Other definitions:"))
		for _, symbol := range result.Others {
			fmt.Fprintf(writer, "  %s:%d-%d\n", symbol.Path, symbol.Start, symbol.End)
		}
	}
}
func printOverview(writer io.Writer, result protocol.Overview) {
	fmt.Fprint(writer, i18n.Tf("%s  %d files, %d symbols, %d chunks\n", result.Root, result.Files, result.Symbols, result.Chunks))
	fmt.Fprintln(writer, i18n.T("Languages:"))
	for _, item := range result.Languages {
		fmt.Fprintf(writer, "  %-18s %d\n", item.Name, item.Count)
	}
	printPaths(writer, i18n.T("Entry points:"), result.Entrypoints)
	printPaths(writer, i18n.T("Documentation:"), result.Docs)
	if len(result.Largest) > 0 {
		fmt.Fprintln(writer, i18n.T("Largest files:"))
		for _, item := range result.Largest {
			fmt.Fprintf(writer, "  %8d  %s\n", item.Value, item.Path)
		}
	}
	if len(result.MostImported) > 0 {
		fmt.Fprintln(writer, i18n.T("Most imported:"))
		for _, item := range result.MostImported {
			fmt.Fprintf(writer, "  %8d  %s\n", item.Value, item.Path)
		}
	}
}
func printPaths(writer io.Writer, label string, paths []string) {
	if len(paths) == 0 {
		return
	}
	fmt.Fprintln(writer, label)
	for _, item := range paths {
		fmt.Fprintf(writer, "  %s\n", item)
	}
}

func encodeTOON(writer io.Writer, value any) error {
	lines, err := toonValue(reflect.ValueOf(value), "")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(writer, strings.Join(lines, "\n"))
	return err
}
func toonValue(value reflect.Value, indent string) ([]string, error) {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return []string{indent + "null"}, nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Struct:
		return toonStruct(value, indent)
	case reflect.Map:
		return toonMap(value, indent)
	default:
		return []string{indent + toonScalar(value)}, nil
	}
}
func toonStruct(value reflect.Value, indent string) ([]string, error) {
	var lines []string
	typ := value.Type()
	for i := range value.NumField() {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, omit := jsonField(field)
		if name == "-" || omit && value.Field(i).IsZero() {
			continue
		}
		encoded, err := toonField(name, value.Field(i), indent)
		if err != nil {
			return nil, err
		}
		lines = append(lines, encoded...)
	}
	return lines, nil
}
func toonMap(value reflect.Value, indent string) ([]string, error) {
	keys := value.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface()) })
	var lines []string
	for _, key := range keys {
		encoded, err := toonField(fmt.Sprint(key.Interface()), value.MapIndex(key), indent)
		if err != nil {
			return nil, err
		}
		lines = append(lines, encoded...)
	}
	return lines, nil
}
func toonField(name string, value reflect.Value, indent string) ([]string, error) {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return []string{indent + name + ": null"}, nil
		}
		value = value.Elem()
	}
	if value.Type() == reflect.TypeOf(time.Time{}) {
		return []string{indent + name + ": " + toonScalar(value)}, nil
	}
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		return toonSlice(name, value, indent)
	case reflect.Struct, reflect.Map:
		children, err := toonValue(value, indent+"  ")
		if err != nil {
			return nil, err
		}
		return append([]string{indent + name + ":"}, children...), nil
	default:
		return []string{indent + name + ": " + toonScalar(value)}, nil
	}
}
func toonSlice(name string, value reflect.Value, indent string) ([]string, error) {
	length := value.Len()
	if length == 0 {
		return []string{fmt.Sprintf("%s%s[0]:", indent, name)}, nil
	}
	element := value.Index(0)
	for element.Kind() == reflect.Pointer || element.Kind() == reflect.Interface {
		if element.IsNil() {
			break
		}
		element = element.Elem()
	}
	if element.IsValid() && element.Kind() == reflect.Struct {
		fields := toonFieldNames(element)
		uniform := true
		for i := 1; i < length; i++ {
			candidate := value.Index(i)
			for candidate.Kind() == reflect.Pointer || candidate.Kind() == reflect.Interface {
				if candidate.IsNil() {
					uniform = false
					break
				}
				candidate = candidate.Elem()
			}
			if !uniform || candidate.Kind() != reflect.Struct || !reflect.DeepEqual(fields, toonFieldNames(candidate)) {
				uniform = false
				break
			}
		}
		if uniform {
			header := fmt.Sprintf("%s%s[%d]{%s}:", indent, name, length, strings.Join(fields, ","))
			lines := []string{header}
			for i := range length {
				row := value.Index(i)
				for row.Kind() == reflect.Pointer || row.Kind() == reflect.Interface {
					row = row.Elem()
				}
				values := make([]string, 0, len(fields))
				typ := row.Type()
				for j := range row.NumField() {
					field := typ.Field(j)
					if field.PkgPath != "" {
						continue
					}
					fieldName, omit := jsonField(field)
					if fieldName == "-" || omit && row.Field(j).IsZero() {
						continue
					}
					values = append(values, toonScalar(row.Field(j)))
				}
				lines = append(lines, indent+"  "+strings.Join(values, ","))
			}
			return lines, nil
		}
	}
	primitive := true
	values := make([]string, length)
	for i := range length {
		item := value.Index(i)
		for item.Kind() == reflect.Interface || item.Kind() == reflect.Pointer {
			if item.IsNil() {
				break
			}
			item = item.Elem()
		}
		if item.IsValid() && (item.Kind() == reflect.Struct || item.Kind() == reflect.Map || item.Kind() == reflect.Slice || item.Kind() == reflect.Array) {
			primitive = false
			break
		}
		values[i] = toonScalar(item)
	}
	if primitive {
		return []string{fmt.Sprintf("%s%s[%d]: %s", indent, name, length, strings.Join(values, ","))}, nil
	}
	lines := []string{fmt.Sprintf("%s%s[%d]:", indent, name, length)}
	for i := range length {
		child, err := toonValue(value.Index(i), indent+"  ")
		if err != nil {
			return nil, err
		}
		lines = append(lines, child...)
	}
	return lines, nil
}
func toonFieldNames(value reflect.Value) []string {
	var names []string
	typ := value.Type()
	for i := range value.NumField() {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, omit := jsonField(field)
		if name != "-" && !(omit && value.Field(i).IsZero()) {
			names = append(names, name)
		}
	}
	return names
}
func jsonField(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name, false
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = field.Name
	}
	return name, len(parts) > 1 && slicesContain(parts[1:], "omitempty")
}
func slicesContain(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func toonScalar(value reflect.Value) string {
	if !value.IsValid() {
		return "null"
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "null"
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.String:
		return toonString(value.String())
	case reflect.Bool:
		return strconv.FormatBool(value.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(value.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'g', -1, value.Type().Bits())
	}
	encoded, _ := json.Marshal(value.Interface())
	return string(encoded)
}
func toonString(value string) string {
	if value != "" {
		safe := true
		for _, r := range value {
			if r <= 0x20 || strings.ContainsRune(",:{}[]\"\\", r) {
				safe = false
				break
			}
		}
		_, numberErr := strconv.ParseFloat(value, 64)
		if safe && numberErr != nil && value != "true" && value != "false" && value != "null" {
			return value
		}
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
