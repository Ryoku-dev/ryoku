package weir

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/lint"
	"sort"
	"strings"
)

type WeirpackManifest struct{ Data map[string]any }

func NewWeirpackManifest() WeirpackManifest { return WeirpackManifest{Data: map[string]any{}} }
func WeirpackManifestFromReader(r io.Reader) (WeirpackManifest, error) {
	m := NewWeirpackManifest()
	decoder := json.NewDecoder(r)
	decoder.UseNumber()
	err := decoder.Decode(&m.Data)
	if err != nil {
		return m, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return m, err
		}
		return m, fmt.Errorf("trailing JSON after manifest")
	}
	return m, m.validate()
}
func (m WeirpackManifest) GetField(key string) (any, bool) { v, ok := m.Data[key]; return v, ok }
func (m *WeirpackManifest) SetField(key string, v any) {
	if m.Data == nil {
		m.Data = map[string]any{}
	}
	m.Data[key] = v
}
func (m WeirpackManifest) required(key string) (string, error) {
	v, ok := m.Data[key]
	if !ok {
		return "", fmt.Errorf("missing manifest field %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("invalid manifest field type for %q", key)
	}
	return s, nil
}
func (m WeirpackManifest) Author() (string, error)      { return m.required("author") }
func (m WeirpackManifest) Version() (string, error)     { return m.required("version") }
func (m WeirpackManifest) Description() (string, error) { return m.required("description") }
func (m WeirpackManifest) License() (string, error)     { return m.required("license") }
func (m *WeirpackManifest) SetAuthor(s string)          { m.SetField("author", s) }
func (m *WeirpackManifest) SetVersion(s string)         { m.SetField("version", s) }
func (m *WeirpackManifest) SetDescription(s string)     { m.SetField("description", s) }
func (m *WeirpackManifest) SetLicense(s string)         { m.SetField("license", s) }
func (m WeirpackManifest) validate() error {
	for _, k := range []string{"author", "version", "description", "license"} {
		if _, err := m.required(k); err != nil {
			return err
		}
	}
	return nil
}
func (m WeirpackManifest) WriteTo(w io.Writer) (int64, error) {
	if err := m.validate(); err != nil {
		return 0, err
	}
	data, err := json.MarshalIndent(m.Data, "", "  ")
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

type Weirpack struct {
	Rules                   map[string]string
	Dictionary, Annotations *string
	Manifest                WeirpackManifest
}

func NewWeirpack(m WeirpackManifest) *Weirpack {
	return &Weirpack{Rules: map[string]string{}, Manifest: m}
}
func (p *Weirpack) AddRule(name, rule string) (string, bool) {
	if p.Rules == nil {
		p.Rules = map[string]string{}
	}
	old, ok := p.Rules[name]
	p.Rules[name] = rule
	return old, ok
}
func (p *Weirpack) RemoveRule(name string) (string, bool) {
	old, ok := p.Rules[name]
	delete(p.Rules, name)
	return old, ok
}
func (p *Weirpack) RunTests() (map[string][]TestResult, error) {
	failures := map[string][]TestResult{}
	for name, code := range p.Rules {
		r, err := NewWeirLinter(code)
		if err != nil {
			return nil, err
		}
		if failed := r.RunTests(); len(failed) > 0 {
			failures[name] = failed
		}
	}
	return failures, nil
}
func (p *Weirpack) ToLintGroup() (*lint.LintGroup, error) {
	g := lint.NewLintGroup()
	for name, code := range p.Rules {
		r, err := NewWeirLinter(code)
		if err != nil {
			return nil, err
		}
		if r.scope == Sentence {
			g.AddSentenceExprLinter(name, r)
		} else {
			g.AddChunkExprLinter(name, r)
		}
		g.Config.SetRuleEnabled(name, true)
	}
	return g, nil
}
func (p *Weirpack) LoadDictionary() (*dict.MutableDictionary, error) {
	if p.Dictionary == nil || p.Annotations == nil {
		return nil, nil
	}
	return dict.FromRuneFiles(*p.Dictionary, *p.Annotations)
}
func WeirpackFromReader(r io.Reader) (*Weirpack, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return WeirpackFromBytes(b)
}
func WeirpackFromBytes(data []byte) (*Weirpack, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	p := NewWeirpack(NewWeirpackManifest())
	found := false
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		if name != "manifest.json" && name != "dictionary.dict" && name != "annotations.json" && !strings.HasSuffix(name, ".weir") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		contents, err := io.ReadAll(r)
		closeErr := r.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		switch name {
		case "manifest.json":
			if found {
				return nil, fmt.Errorf("duplicate manifest.json")
			}
			p.Manifest, err = WeirpackManifestFromReader(bytes.NewReader(contents))
			if err != nil {
				return nil, err
			}
			found = true
		case "dictionary.dict":
			s := string(contents)
			p.Dictionary = &s
		case "annotations.json":
			s := string(contents)
			p.Annotations = &s
		default:
			base := path.Base(name)
			if base == "." || base == "/" {
				return nil, fmt.Errorf("invalid rule file name %q", name)
			}
			p.Rules[strings.TrimSuffix(base, ".weir")] = string(contents)
		}
	}
	if !found {
		return nil, fmt.Errorf("missing manifest.json")
	}
	return p, nil
}
func (p *Weirpack) ToBytes() ([]byte, error) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	write := func(name string, data []byte) error {
		w, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	var manifest bytes.Buffer
	if _, err := p.Manifest.WriteTo(&manifest); err != nil {
		return nil, err
	}
	if err := write("manifest.json", manifest.Bytes()); err != nil {
		return nil, err
	}
	if p.Annotations != nil {
		if err := write("annotations.json", []byte(*p.Annotations)); err != nil {
			return nil, err
		}
	}
	if p.Dictionary != nil {
		if err := write("dictionary.dict", []byte(*p.Dictionary)); err != nil {
			return nil, err
		}
	}
	names := make([]string, 0, len(p.Rules))
	for name := range p.Rules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := write(name+".weir", []byte(p.Rules[name])); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (p *Weirpack) WriteTo(w io.Writer) (int64, error) {
	b, err := p.ToBytes()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(b)
	return int64(n), err
}
