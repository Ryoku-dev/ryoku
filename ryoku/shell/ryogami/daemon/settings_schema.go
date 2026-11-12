package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
)

type settingSpec struct {
	Key          string      `json:"key"`
	Type         string      `json:"type"`
	Default      interface{} `json:"default"`
	DefaultSmall interface{} `json:"defaultSmall,omitempty"`
	Options      []string    `json:"options,omitempty"`
	Min          *float64    `json:"min,omitempty"`
	Max          *float64    `json:"max,omitempty"`
	Step         *float64    `json:"step,omitempty"`
	Store        string      `json:"store,omitempty"`
	StoreKey     string      `json:"-"`
}

// Boxed so an unset bound stays distinguishable from a deliberate zero.
func fp(f float64) *float64 { return &f }

var schemaIndex = buildSchemaIndex()

func buildSchemaIndex() map[string]*settingSpec {
	m := make(map[string]*settingSpec, len(settingsSchema))
	for i := range settingsSchema {
		m[settingsSchema[i].Key] = &settingsSchema[i]
	}
	return m
}

func specFor(key string) (*settingSpec, bool) {
	if s, ok := schemaIndex[key]; ok {
		return s, true
	}
	// A per-output key such as display.fillModes.DP-1 takes its "<output>" pattern's spec.
	if i := strings.LastIndexByte(key, '.'); i > 0 && i < len(key)-1 {
		if s, ok := schemaIndex[key[:i+1]+"<output>"]; ok {
			return s, true
		}
	}
	return nil, false
}

var settingsRevision = schemaRevision()

func schemaRevision() uint32 {
	b, err := json.Marshal(settingsSchema)
	if err != nil {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write(b)
	return h.Sum32()
}

// An unknown key is rejected too, so a typo never persists.
func validateSetting(key string, value interface{}) error {
	spec, ok := specFor(key)
	if !ok {
		return fmt.Errorf("%s: unknown setting", key)
	}
	if err := spec.validate(value); err != nil {
		return err
	}
	return validateDocValue(key, value)
}

func (s *settingSpec) validate(value interface{}) error {
	switch s.Type {
	case "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected a boolean", s.Key)
		}
	case "number":
		n, ok := toNumber(value)
		if !ok {
			return fmt.Errorf("%s: expected a number", s.Key)
		}
		if s.Min != nil && n < *s.Min {
			return fmt.Errorf("%s: %v is below the minimum %v", s.Key, n, *s.Min)
		}
		if s.Max != nil && n > *s.Max {
			return fmt.Errorf("%s: %v is above the maximum %v", s.Key, n, *s.Max)
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: expected a string", s.Key)
		}
	case "enum":
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s: expected one of %v", s.Key, s.Options)
		}
		if !containsStr(s.Options, str) {
			return fmt.Errorf("%s: %q is not one of %v", s.Key, str, s.Options)
		}
	case "list":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("%s: expected a list", s.Key)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("%s: expected an object", s.Key)
		}
	default:
		return fmt.Errorf("%s: unhandled type %q", s.Key, s.Type)
	}
	return nil
}

// Values arrive as float64 from JSON, int from Go callers, or json.Number.
func toNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
