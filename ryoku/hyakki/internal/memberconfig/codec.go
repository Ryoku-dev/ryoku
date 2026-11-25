// Package memberconfig encodes canonical protocol members, never vault values.
package memberconfig

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

const Mask = "<vault-reference>"

// Bounds are shared by ordinary canonical saves and YAML. Four MiB of scalar
// data accommodates all 64 x 16 KiB argv and environment values together.
// The JSON-escaped YAML must also fit in 15 MiB, reserving 1 MiB of MaxLine
// for Get/Save framing, identifiers, revisions and timestamps.
const (
	MaxBytes        = protocol.MaxLine - (1 << 20)
	MaxContentBytes = 4 << 20
	MaxScalarBytes  = 64 << 10
	MaxKeyBytes     = 1024
	MaxNodes        = 8192
	MaxDepth        = 32
)

// Snapshot stores only canonical configuration and vault references, not YAML.
type Snapshot struct {
	Member protocol.Member `json:"member"`
	At     time.Time       `json:"at"`
}

type document struct {
	Version int             `json:"version"`
	Team    string          `json:"team"`
	Member  protocol.Member `json:"member"`
}

// Build the exact JSON-tag schema once, including every nested typed struct.
// Map keys are data, not schema names, and deliberately remain case-sensitive.
type schema struct {
	kind    reflect.Kind
	fields  []schemaField
	names   map[string]*schema
	element *schema
}

type schemaField struct {
	index     int
	name      string
	omitEmpty bool
	value     *schema
}

var documentSchema = makeSchema(reflect.TypeOf(document{}))

func makeSchema(t reflect.Type) *schema {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	s := &schema{kind: t.Kind()}
	switch t.Kind() {
	case reflect.Struct:
		s.names = make(map[string]*schema, t.NumField())
		for i := range t.NumField() {
			field := t.Field(i)
			name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
			if field.PkgPath != "" || name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			child := makeSchema(field.Type)
			s.fields = append(s.fields, schemaField{index: i, name: name, omitEmpty: strings.Contains(","+options+",", ",omitempty,"), value: child})
			s.names[name] = child
		}
	case reflect.Map, reflect.Slice, reflect.Array:
		s.element = makeSchema(t.Elem())
	}
	return s
}

func bad(message string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T(message))
}

func Encode(team string, snapshot Snapshot, current uint64) (protocol.MemberConfigDocument, error) {
	if len(team) > MaxKeyBytes {
		return protocol.MemberConfigDocument{}, bad("Member configuration team ID is too long.")
	}
	if _, err := canonicalBounds(snapshot.Member); err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	text, err := encodeDocument(team, snapshot.Member, MaxBytes)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	return protocol.MemberConfigDocument{YAML: text, Revision: snapshot.Member.Revision, CurrentRevision: current, Updated: snapshot.At}, nil
}

func encodeDocument(team string, member protocol.Member, wireLimit int) (string, error) {
	if member.Runtime != nil {
		runtime := *member.Runtime
		runtime.SecretEnv = make(map[string]string, len(member.Runtime.SecretEnv))
		for key := range member.Runtime.SecretEnv {
			runtime.SecretEnv[key] = Mask
		}
		if runtime.Credential != "" {
			runtime.Credential = Mask
		}
		member.Runtime = &runtime
	}
	// JSON tags are the single schema for forms and YAML; do not duplicate Member.
	raw, err := json.Marshal(document{Version: 1, Team: team, Member: member})
	if err != nil {
		return "", err
	}
	var shape any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&shape); err != nil {
		return "", err
	}
	node := encodeNode(shape)
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	text := out.String()
	if err := checkWireTextLimit(text, wireLimit); err != nil {
		return "", err
	}
	return text, nil
}

// Match encoding/json's string escaping without allocating another large copy.
// MaxBytes excludes framing, so this bound holds for both response results and
// request params (including RawMessage's normal HTML escaping).
func checkWireText(text string) error {
	return checkWireTextLimit(text, MaxBytes)
}

func checkWireTextLimit(text string, limit int) error {
	if len(text) > limit {
		return bad("Member YAML exceeds its 15 MiB wire-safe document limit.")
	}
	if len(text)*6+2 <= limit {
		return nil
	}
	size := 2
	for _, r := range text {
		switch {
		case r == '"' || r == '\\' || r == '\b' || r == '\f' || r == '\n' || r == '\r' || r == '\t':
			size += 2
		case r < 0x20 || r == '<' || r == '>' || r == '&' || r == '\u2028' || r == '\u2029':
			size += 6
		default:
			size += utf8.RuneLen(r)
		}
		if size > limit {
			return bad("Member YAML exceeds its 15 MiB JSON-escaped wire limit; shorten the configuration.")
		}
	}
	return nil
}

// encodeNode keeps integers exact, including revisions larger than 2^53.
func encodeNode(value any) *yaml.Node {
	switch v := value.(type) {
	case json.Number:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: string(v)}
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
			if key == "<<" {
				keyNode.Style = yaml.DoubleQuotedStyle
			}
			n.Content = append(n.Content, keyNode, encodeNode(v[key]))
		}
		return n
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range v {
			n.Content = append(n.Content, encodeNode(item))
		}
		return n
	default:
		n := new(yaml.Node)
		_ = n.Encode(v)
		// Literal-block indentation can multiply newline-heavy runtime values
		// past MaxLine. Quoted multiline strings keep full argv/env support.
		if text, ok := v.(string); ok && strings.ContainsAny(text, "\r\n") {
			n.Style = yaml.DoubleQuotedStyle
		}
		return n
	}
}

func Decode(text, team, id string, expected uint64, current protocol.Member) (protocol.Member, error) {
	if len(text) == 0 || len(text) > MaxBytes {
		return protocol.Member{}, bad("Member YAML must be between 1 byte and 15 MiB.")
	}
	if err := checkWireText(text); err != nil {
		return protocol.Member{}, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(text))
	var node yaml.Node
	if err := decoder.Decode(&node); err != nil {
		return protocol.Member{}, bad("Invalid YAML syntax; check indentation and scalar quoting.")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return protocol.Member{}, bad("Member YAML must contain exactly one document.")
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return protocol.Member{}, bad("Member YAML must be a versioned mapping.")
	}
	limits := resourceLimits{}
	shape, err := decodeNode(node.Content[0], 0, &limits, documentSchema)
	if err != nil {
		return protocol.Member{}, err
	}
	raw, err := json.Marshal(shape)
	if err != nil {
		return protocol.Member{}, bad("Member YAML contains an unsupported scalar.")
	}
	var doc document
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&doc); err != nil {
		return protocol.Member{}, bad("Member YAML has an unknown field or a value of the wrong type; use the current document's field names and types.")
	}
	if doc.Version != 1 {
		return protocol.Member{}, bad("Member YAML requires version: 1.")
	}
	if doc.Team != team || doc.Member.ID != id {
		return protocol.Member{}, bad("The team and member IDs are immutable.")
	}
	if expected == 0 || current.Revision != expected || doc.Member.Revision != expected {
		return protocol.Member{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The member changed; reload its current configuration before saving."))
	}
	if err := PreserveMasks(&doc.Member, current); err != nil {
		return protocol.Member{}, err
	}
	if err := CheckBounds(doc.Member); err != nil {
		return protocol.Member{}, err
	}
	return doc.Member, nil
}

func decodeNode(node *yaml.Node, depth int, limits *resourceLimits, typed *schema) (any, error) {
	if err := limits.add(depth, node.Value, false); err != nil {
		return nil, err
	}
	if node.Anchor != "" || node.Kind == yaml.AliasNode {
		return nil, bad("YAML anchors and aliases are not supported in member configuration.")
	}
	switch node.Kind {
	case yaml.MappingNode:
		if node.Tag != "!!map" {
			return nil, bad("Custom YAML tags are not supported.")
		}
		result := make(map[string]any, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" {
				return nil, bad("YAML mapping keys must be strings; merge keys are not supported.")
			}
			if err := limits.add(depth+1, key.Value, true); err != nil {
				return nil, err
			}
			var child *schema
			if typed != nil {
				switch typed.kind {
				case reflect.Struct:
					var known bool
					child, known = typed.names[key.Value]
					if !known {
						return nil, bad("Member YAML field names must exactly match the current document, including case.")
					}
				case reflect.Map:
					child = typed.element
				}
			}
			if _, duplicate := result[key.Value]; duplicate {
				return nil, bad("Member YAML contains a duplicate field.")
			}
			value, err := decodeNode(node.Content[i+1], depth+1, limits, child)
			if err != nil {
				return nil, err
			}
			result[key.Value] = value
		}
		return result, nil
	case yaml.SequenceNode:
		if node.Tag != "!!seq" {
			return nil, bad("Custom YAML tags are not supported.")
		}
		result := make([]any, 0, len(node.Content))
		var child *schema
		if typed != nil && (typed.kind == reflect.Slice || typed.kind == reflect.Array) {
			child = typed.element
		}
		for _, item := range node.Content {
			value, err := decodeNode(item, depth+1, limits, child)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str":
			return node.Value, nil
		case "!!null":
			return nil, nil
		case "!!bool":
			if node.Value != "true" && node.Value != "false" {
				return nil, bad("Use true or false for YAML booleans.")
			}
			return node.Value == "true", nil
		case "!!int":
			if _, err := strconv.ParseInt(node.Value, 10, 64); err != nil {
				if _, err := strconv.ParseUint(node.Value, 10, 64); err != nil {
					return nil, bad("Use bounded decimal integers in member YAML.")
				}
			}
			return json.Number(node.Value), nil
		}
	}
	return nil, bad("Unsupported YAML construct; use mappings, lists, strings, booleans and decimal integers.")
}

func PreserveMasks(member *protocol.Member, current protocol.Member) error {
	if member.Runtime == nil {
		return nil
	}
	for key, name := range member.Runtime.SecretEnv {
		if name != Mask {
			continue
		}
		if current.Runtime == nil || current.Runtime.SecretEnv[key] == "" {
			return bad("A masked environment reference has no current vault binding; name a vault entry or delete the field.")
		}
		member.Runtime.SecretEnv[key] = current.Runtime.SecretEnv[key]
	}
	if member.Runtime.Credential == Mask {
		if current.Runtime == nil || current.Runtime.Credential == "" {
			return bad("A masked credential has no current vault binding; name a vault entry or delete the field.")
		}
		member.Runtime.Credential = current.Runtime.Credential
	}
	return nil
}
