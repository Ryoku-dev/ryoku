package rashin

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// toon renders a result the way the rashin CLI does in TOON: arrays of
// alike structs as one header and a row each, which costs a model far fewer
// tokens than JSON for team and template listings.
func toon(value any) string {
	return strings.Join(toonValue(reflect.ValueOf(value), ""), "\n")
}

func toonValue(value reflect.Value, indent string) []string {
	value, ok := deref(value)
	if !ok {
		return []string{indent + "null"}
	}
	switch value.Kind() {
	case reflect.Struct:
		return toonStruct(value, indent)
	case reflect.Map:
		keys := value.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface()) })
		var lines []string
		for _, key := range keys {
			lines = append(lines, toonField(fmt.Sprint(key.Interface()), value.MapIndex(key), indent)...)
		}
		return lines
	}
	return []string{indent + toonScalar(value)}
}

func toonStruct(value reflect.Value, indent string) []string {
	var lines []string
	for _, f := range toonFields(value) {
		lines = append(lines, toonField(f.name, f.value, indent)...)
	}
	return lines
}

type toonPart struct {
	name  string
	value reflect.Value
}

// toonFields are a struct's exported fields under their JSON names, empty
// omitempty fields left out.
func toonFields(value reflect.Value) []toonPart {
	var parts []toonPart
	typ := value.Type()
	for i := range value.NumField() {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name, omit := jsonName(f)
		if name == "-" || omit && value.Field(i).IsZero() {
			continue
		}
		parts = append(parts, toonPart{name, value.Field(i)})
	}
	return parts
}

func toonField(name string, value reflect.Value, indent string) []string {
	value, ok := deref(value)
	if !ok {
		return []string{indent + name + ": null"}
	}
	if value.Type() == reflect.TypeOf(time.Time{}) {
		return []string{indent + name + ": " + toonScalar(value)}
	}
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		return toonSlice(name, value, indent)
	case reflect.Struct, reflect.Map:
		return append([]string{indent + name + ":"}, toonValue(value, indent+"  ")...)
	}
	return []string{indent + name + ": " + toonScalar(value)}
}

func toonSlice(name string, value reflect.Value, indent string) []string {
	length := value.Len()
	if length == 0 {
		return []string{fmt.Sprintf("%s%s[0]:", indent, name)}
	}
	if header, ok := uniformFields(value); ok {
		lines := []string{fmt.Sprintf("%s%s[%d]{%s}:", indent, name, length, strings.Join(header, ","))}
		for i := range length {
			row, _ := deref(value.Index(i))
			var cells []string
			for _, f := range toonFields(row) {
				cells = append(cells, toonScalar(f.value))
			}
			lines = append(lines, indent+"  "+strings.Join(cells, ","))
		}
		return lines
	}
	cells := make([]string, 0, length)
	for i := range length {
		item, ok := deref(value.Index(i))
		if ok && slices.Contains([]reflect.Kind{reflect.Struct, reflect.Map, reflect.Slice, reflect.Array}, item.Kind()) {
			lines := []string{fmt.Sprintf("%s%s[%d]:", indent, name, length)}
			for j := range length {
				lines = append(lines, toonValue(value.Index(j), indent+"  ")...)
			}
			return lines
		}
		cells = append(cells, toonScalar(item))
	}
	return []string{fmt.Sprintf("%s%s[%d]: %s", indent, name, length, strings.Join(cells, ","))}
}

// uniformFields is the field names shared by every element when all are
// structs with the same fields present and none of them nested.
func uniformFields(value reflect.Value) ([]string, bool) {
	var header []string
	for i := range value.Len() {
		row, ok := deref(value.Index(i))
		if !ok || row.Kind() != reflect.Struct || row.Type() == reflect.TypeOf(time.Time{}) {
			return nil, false
		}
		var names []string
		for _, f := range toonFields(row) {
			inner, present := deref(f.value)
			if present && inner.Type() != reflect.TypeOf(time.Time{}) && slices.Contains([]reflect.Kind{reflect.Struct, reflect.Map, reflect.Slice, reflect.Array}, inner.Kind()) {
				return nil, false
			}
			names = append(names, f.name)
		}
		if i == 0 {
			header = names
		} else if !slices.Equal(header, names) {
			return nil, false
		}
	}
	return header, true
}

func deref(value reflect.Value) (reflect.Value, bool) {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return value, false
		}
		value = value.Elem()
	}
	return value, value.IsValid()
}

func jsonName(f reflect.StructField) (string, bool) {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name, false
	}
	name, options, _ := strings.Cut(tag, ",")
	if name == "" {
		name = f.Name
	}
	return name, slices.Contains(strings.Split(options, ","), "omitempty")
}

func toonScalar(value reflect.Value) string {
	value, ok := deref(value)
	if !ok {
		return "null"
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
		safe := !strings.ContainsFunc(value, func(r rune) bool { return r <= 0x20 || strings.ContainsRune(",:{}[]\"\\", r) })
		_, numberErr := strconv.ParseFloat(value, 64)
		if safe && numberErr != nil && value != "true" && value != "false" && value != "null" {
			return value
		}
	}
	// HTML escaping would turn every < and & into \u003c and \u0026
	var encoded strings.Builder
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return strings.TrimSuffix(encoded.String(), "\n")
}
