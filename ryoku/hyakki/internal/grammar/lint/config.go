package lint

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

//go:embed default_config.json
var defaultConfigJSON []byte

type FlatConfig map[string]*bool

func NewFlatConfig() FlatConfig { return FlatConfig{} }
func (c FlatConfig) Clone() FlatConfig {
	out := NewFlatConfig()
	for k, v := range c {
		if v == nil {
			out[k] = nil
		} else {
			x := *v
			out[k] = &x
		}
	}
	return out
}
func (c FlatConfig) HasRule(k string) bool           { _, ok := c[k]; return ok }
func (c FlatConfig) SetRuleEnabled(k string, v bool) { c[k] = &v }
func (c FlatConfig) UnsetRuleEnabled(k string)       { delete(c, k) }
func (c FlatConfig) SetRuleEnabledIfUnset(k string, v bool) {
	if !c.HasRule(k) {
		c.SetRuleEnabled(k, v)
	}
}
func (c FlatConfig) IsRuleEnabled(k string) bool { v := c[k]; return v != nil && *v }
func (c FlatConfig) Clear() {
	for k := range c {
		c[k] = nil
	}
}
func (c FlatConfig) MergeFrom(other FlatConfig) {
	for k, v := range other {
		if v != nil {
			c.SetRuleEnabled(k, *v)
		}
	}
}
func (c FlatConfig) FillWithCurated() {
	defaults := NewCuratedFlatConfig()
	for k, v := range defaults {
		if c[k] == nil {
			c.SetRuleEnabled(k, *v)
		}
	}
}
func NewCuratedFlatConfig() FlatConfig {
	s := CuratedStructuredConfig()
	c, ok := s.ToFlatConfig()
	if !ok {
		panic("invalid embedded Harper configuration")
	}
	return c
}

type SettingKind uint8

const (
	Bool SettingKind = iota
	OneOfMany
	Group
)

type Setting struct {
	Kind               SettingKind
	Name               string
	State              bool
	Names, Labels      []string
	Choice             *int
	Label, Description string
	Child              *StructuredConfig
}
type StructuredConfig struct{ Settings []Setting }

func (s Setting) Validate() bool {
	switch s.Kind {
	case Bool:
		return true
	case OneOfMany:
		return len(s.Names) == len(s.Labels) && (s.Choice == nil || *s.Choice >= 0 && *s.Choice < len(s.Names))
	case Group:
		return s.Child != nil && s.Child.Validate()
	}
	return false
}
func (s StructuredConfig) Validate() bool {
	for _, v := range s.Settings {
		if !v.Validate() {
			return false
		}
	}
	return true
}
func (s StructuredConfig) ToFlatConfig() (FlatConfig, bool) {
	if !s.Validate() {
		return nil, false
	}
	out := NewFlatConfig()
	for _, v := range s.Settings {
		switch v.Kind {
		case Bool:
			out.SetRuleEnabled(v.Name, v.State)
		case OneOfMany:
			for i, name := range v.Names {
				out.SetRuleEnabled(name, v.Choice != nil && *v.Choice == i)
			}
		case Group:
			child, _ := v.Child.ToFlatConfig()
			out.MergeFrom(child)
		}
	}
	return out, true
}
func (s *StructuredConfig) CopyFromFlatConfig(c FlatConfig) {
	for i := range s.Settings {
		v := &s.Settings[i]
		switch v.Kind {
		case Bool:
			v.State = c.IsRuleEnabled(v.Name)
		case OneOfMany:
			v.Choice = nil
			for j, name := range v.Names {
				if c.IsRuleEnabled(name) {
					choice := j
					v.Choice = &choice
					break
				}
			}
		case Group:
			v.Child.CopyFromFlatConfig(c)
		}
	}
}
func CuratedStructuredConfig() StructuredConfig {
	var h HumanReadableStructuredConfig
	if err := json.Unmarshal(defaultConfigJSON, &h); err != nil {
		panic(err)
	}
	s, ok := h.ToStructuredConfig()
	if !ok {
		panic("invalid embedded Harper configuration")
	}
	return s
}

type HumanReadableSetting struct {
	Kind        SettingKind
	Name        *string
	State       bool
	Names       []string
	Labels      []string
	Label       *string
	Description string
	Child       *HumanReadableStructuredConfig
}
type HumanReadableStructuredConfig struct {
	Settings []HumanReadableSetting `json:"settings"`
}

func (s HumanReadableSetting) Validate() bool {
	switch s.Kind {
	case Bool:
		return s.Name != nil
	case OneOfMany:
		return (s.Labels == nil || len(s.Labels) == len(s.Names)) && (s.Name == nil || slices.Contains(s.Names, *s.Name))
	case Group:
		return s.Child != nil && s.Child.Validate()
	}
	return false
}
func (s HumanReadableStructuredConfig) Validate() bool {
	for _, v := range s.Settings {
		if !v.Validate() {
			return false
		}
	}
	return true
}
func (s HumanReadableSetting) ToSetting() (Setting, bool) {
	if !s.Validate() {
		return Setting{}, false
	}
	out := Setting{Kind: s.Kind, State: s.State, Names: slices.Clone(s.Names), Labels: slices.Clone(s.Labels), Description: s.Description}
	switch s.Kind {
	case Bool:
		out.Name = *s.Name
	case OneOfMany:
		if out.Labels == nil {
			out.Labels = slices.Clone(s.Names)
		}
		if s.Name != nil {
			i := slices.Index(s.Names, *s.Name)
			out.Choice = &i
		}
	case Group:
		if s.Label != nil {
			out.Label = *s.Label
		}
		child, _ := s.Child.ToStructuredConfig()
		out.Child = &child
	}
	return out, true
}
func (s HumanReadableStructuredConfig) ToStructuredConfig() (StructuredConfig, bool) {
	if !s.Validate() {
		return StructuredConfig{}, false
	}
	out := StructuredConfig{}
	for _, v := range s.Settings {
		setting, _ := v.ToSetting()
		out.Settings = append(out.Settings, setting)
	}
	return out, true
}
func HumanReadableSettingFromSetting(s Setting) HumanReadableSetting {
	out := HumanReadableSetting{Kind: s.Kind, State: s.State, Names: slices.Clone(s.Names), Description: s.Description}
	switch s.Kind {
	case Bool:
		name := s.Name
		out.Name = &name
	case OneOfMany:
		if s.Choice != nil {
			name := s.Names[*s.Choice]
			out.Name = &name
		}
		if !slices.Equal(s.Names, s.Labels) {
			out.Labels = slices.Clone(s.Labels)
		}
	case Group:
		label := s.Label
		out.Label = &label
		child := HumanReadableStructuredConfigFromStructuredConfig(*s.Child)
		out.Child = &child
	}
	return out
}
func HumanReadableStructuredConfigFromStructuredConfig(s StructuredConfig) HumanReadableStructuredConfig {
	out := HumanReadableStructuredConfig{}
	for _, v := range s.Settings {
		out.Settings = append(out.Settings, HumanReadableSettingFromSetting(v))
	}
	return out
}
func (s *HumanReadableSetting) UnmarshalJSON(b []byte) error {
	var tagged map[string]json.RawMessage
	if err := json.Unmarshal(b, &tagged); err != nil {
		return err
	}
	if len(tagged) != 1 {
		return errors.New("a Harper setting must contain exactly one variant")
	}
	type payload struct {
		Name        *string                        `json:"name"`
		State       bool                           `json:"state"`
		Names       []string                       `json:"names"`
		Labels      []string                       `json:"labels"`
		Label       *string                        `json:"label"`
		Description string                         `json:"description"`
		Child       *HumanReadableStructuredConfig `json:"child"`
	}
	for key, raw := range tagged {
		var p payload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		var kind SettingKind
		switch key {
		case "Bool":
			kind = Bool
		case "OneOfMany":
			kind = OneOfMany
		case "Group":
			kind = Group
		default:
			return fmt.Errorf("unknown Harper setting variant %q", key)
		}
		*s = HumanReadableSetting{Kind: kind, Name: p.Name, State: p.State, Names: p.Names, Labels: p.Labels, Label: p.Label, Description: p.Description, Child: p.Child}
		return nil
	}
	return errors.New("empty Harper setting")
}
func (s HumanReadableSetting) MarshalJSON() ([]byte, error) {
	p := map[string]any{}
	key := "Bool"
	switch s.Kind {
	case Bool:
		p["name"] = s.Name
		p["state"] = s.State
		if s.Label != nil {
			p["label"] = s.Label
		}
	case OneOfMany:
		key = "OneOfMany"
		p["names"] = s.Names
		p["name"] = s.Name
		if s.Labels != nil {
			p["labels"] = s.Labels
		}
	case Group:
		key = "Group"
		p["label"] = s.Label
		p["description"] = s.Description
		p["child"] = s.Child
	}
	return json.Marshal(map[string]any{key: p})
}
