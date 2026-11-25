package textcalls

// Derived from oh-my-pi (MIT), packages/ai/src/dialect/coercion.ts@c7fff67

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"ryoku-rashin/internal/llm"
)

// shape is what the scanner needs of one tool: its schema, its properties
// and their order (positional Pythonic arguments map onto it).
type shape struct {
	props object
}

func buildShapes(tools []llm.Tool) map[string]*shape {
	shapes := make(map[string]*shape, len(tools))
	for _, t := range tools {
		sh := &shape{}
		if v, err := parseRelaxed(string(t.Parameters)); err == nil {
			sh.props = properties(v)
		}
		shapes[t.Name] = sh
	}
	return shapes
}

func (s *shape) prop(key string) any {
	if s == nil {
		return nil
	}
	v, _ := s.props.get(key)
	return v
}

func properties(schema any) object {
	o, _ := schema.(object)
	p, _ := o.get("properties")
	props, _ := p.(object)
	return props
}

func items(schema any) any {
	o, _ := schema.(object)
	v, _ := o.get("items")
	return v
}

// schemaTypes collects the JSON types a schema admits, through anyOf, oneOf,
// allOf, enum and const.
func schemaTypes(schema any, out map[string]bool, depth int) map[string]bool {
	if out == nil {
		out = map[string]bool{}
	}
	o, ok := schema.(object)
	if !ok || depth > 8 {
		return out
	}
	t, hasType := o.get("type")
	switch t := t.(type) {
	case string:
		out[t] = true
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok {
				out[s] = true
			}
		}
	}
	if !hasType {
		if e, ok := o.get("enum"); ok {
			if list, ok := e.([]any); ok {
				for _, v := range list {
					out[jsonType(v)] = true
				}
			}
		}
		if c, ok := o.get("const"); ok {
			out[jsonType(c)] = true
		}
	}
	for _, k := range []string{"anyOf", "oneOf", "allOf"} {
		if b, ok := o.get(k); ok {
			if list, ok := b.([]any); ok {
				for _, sub := range list {
					schemaTypes(sub, out, depth+1)
				}
			}
		}
	}
	return out
}

func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case number:
		return "number"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []any:
		return "array"
	}
	return "object"
}

// stringOnly reports a schema whose only non-null type is string: its
// values are read verbatim.
func stringOnly(schema any) bool {
	types := schemaTypes(schema, nil, 0)
	delete(types, "null")
	return len(types) == 1 && types["string"]
}

// decodeArg turns one text-form argument value into a JSON value. A string
// schema keeps the text; a typed schema repairs JSON; with no schema only
// valid JSON decodes, so a path stays a path.
func decodeArg(raw string, schema any) any {
	if schema != nil && stringOnly(schema) {
		return raw
	}
	t := strings.TrimSpace(raw)
	if t == "" {
		return raw
	}
	if schema == nil && !json.Valid([]byte(t)) {
		return raw
	}
	if v, err := parseRelaxed(t); err == nil {
		return v
	}
	return raw
}

// Coerce fixes the argument types of one call against the tool's JSON
// schema: "5" for an integer, "true" for a boolean, a JSON string for an
// array or object, a lone value for an array, a number for a string. Input
// that is not a JSON object comes back unchanged.
func Coerce(args string, schema json.RawMessage) string {
	v, err := parseRelaxed(strings.TrimSpace(args))
	if err != nil {
		return args
	}
	o, ok := v.(object)
	if !ok {
		return args
	}
	s, err := parseRelaxed(string(schema))
	if err != nil {
		return marshal(o)
	}
	return marshal(coerceObject(o, properties(s), 0))
}

func coerceObject(o object, props object, depth int) object {
	if len(props) == 0 || depth > 8 {
		return o
	}
	out := make(object, 0, len(o))
	for _, m := range o {
		if p, ok := props.get(m.key); ok {
			m.val = coerce(m.val, p, depth+1)
		}
		out = append(out, m)
	}
	return out
}

func coerce(v any, schema any, depth int) any {
	if depth > 8 {
		return v
	}
	types := schemaTypes(schema, nil, 0)
	if len(types) == 0 {
		return v
	}
	numeric := types["integer"] || types["number"]
	switch x := v.(type) {
	case string:
		if types["string"] {
			return x
		}
		t := strings.TrimSpace(x)
		if numeric {
			if n, ok := canonicalNumber(t); ok {
				return integral(n, types)
			}
		}
		if types["boolean"] {
			switch strings.ToLower(t) {
			case "true", "yes":
				return true
			case "false", "no":
				return false
			}
		}
		if types["null"] && (t == "null" || t == "None" || t == "") {
			return nil
		}
		if types["array"] || types["object"] {
			if d, err := parseRelaxed(t); err == nil {
				_, isArr := d.([]any)
				_, isObj := d.(object)
				if isArr && types["array"] || isObj && types["object"] {
					return coerce(d, schema, depth+1)
				}
			}
		}
		if types["array"] {
			return []any{coerce(x, items(schema), depth+1)}
		}
		return x
	case number:
		if numeric {
			return integral(x, types)
		}
		if types["string"] {
			return string(x)
		}
		if types["boolean"] && (x == "0" || x == "1") {
			return x == "1"
		}
		if types["array"] {
			return []any{coerce(x, items(schema), depth+1)}
		}
		return x
	case bool:
		if types["boolean"] {
			return x
		}
		if types["string"] {
			return strconv.FormatBool(x)
		}
		if types["array"] {
			return []any{coerce(x, items(schema), depth+1)}
		}
		return x
	case []any:
		if types["array"] {
			it := items(schema)
			out := make([]any, len(x))
			for i, e := range x {
				out[i] = coerce(e, it, depth+1)
			}
			return out
		}
		if types["string"] {
			return marshal(x)
		}
		return x
	case object:
		if types["object"] {
			return coerceObject(x, properties(schema), depth+1)
		}
		if types["string"] {
			return marshal(x)
		}
		if types["array"] {
			return []any{coerce(x, items(schema), depth+1)}
		}
		return x
	}
	return v
}

// integral writes a whole number like 3.0 as 3 where only integers fit.
func integral(n number, types map[string]bool) number {
	if !types["integer"] || types["number"] {
		return n
	}
	if jsonNumber(string(n)) && !strings.ContainsAny(string(n), ".eE") {
		return n
	}
	f, err := strconv.ParseFloat(string(n), 64)
	if err != nil || f < math.MinInt64 || f >= 9223372036854775808.0 || f != math.Trunc(f) {
		return n
	}
	return number(strconv.FormatInt(int64(f), 10))
}

// RepairJSON parses relaxed JSON (single quotes, unquoted keys, comments,
// Python literals, stray commas, bad escapes, raw control characters) and
// returns it as compact JSON. Truncated input and trailing text fail.
func RepairJSON(s string) (string, error) {
	v, err := parseRelaxed(strings.TrimSpace(s))
	if err != nil {
		return "", err
	}
	return marshal(v), nil
}
