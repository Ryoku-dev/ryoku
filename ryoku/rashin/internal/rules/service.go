package rules

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type Deps struct {
	Client   llm.Client
	Settings func() protocol.Settings
	Judge    engine.Judge
}

type Service struct {
	deps     Deps
	mu       sync.Mutex
	turns    map[string]int
	fired    map[string]map[string]int
	pending  map[string][]string
	hydrated map[string]bool
}

var _ engine.ContextProvider = (*Service)(nil)
var _ engine.StreamGuards = (*Service)(nil)

func New(deps Deps) *Service {
	return &Service{deps: deps, turns: make(map[string]int), fired: make(map[string]map[string]int), pending: make(map[string][]string), hydrated: make(map[string]bool)}
}

func (s *Service) enabled() bool {
	return s.deps.Settings != nil && s.deps.Settings().Features[protocol.FeatureRules]
}

var ruleName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ruleFile(ref protocol.RuleRef) (string, error) {
	if !ruleName.MatchString(ref.Name) || len(ref.Name) > 96 {
		return "", protocol.Errorf(protocol.CodeBadRequest, "rule name must be lowercase kebab-case")
	}
	root := paths.RulesDir()
	if ref.Project != "" {
		if !filepath.IsAbs(ref.Project) || filepath.Clean(ref.Project) != ref.Project {
			return "", protocol.Errorf(protocol.CodeBadRequest, "project must be an absolute folder")
		}
		root = paths.ProjectRulesDir(ref.Project)
	}
	return filepath.Join(root, ref.Name+".md"), nil
}

func (s *Service) List(ctx context.Context, project string) (protocol.LearningRulesResult, error) {
	if project != "" && (!filepath.IsAbs(project) || filepath.Clean(project) != project) {
		return protocol.LearningRulesResult{}, protocol.Errorf(protocol.CodeBadRequest, "project must be an absolute folder")
	}
	out := protocol.LearningRulesResult{Rules: []protocol.Rule{}}
	groups := []struct{ dir, project string }{{dir: paths.RulesDir()}}
	if project != "" {
		groups = append([]struct{ dir, project string }{{paths.ProjectRulesDir(project), project}}, groups...)
	}
	for _, group := range groups {
		entries, err := os.ReadDir(group.dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return out, err
		}
		for _, entry := range entries {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".md")
			if !ruleName.MatchString(name) {
				continue
			}
			path := filepath.Join(group.dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				out.Rules = append(out.Rules, protocol.Rule{Name: name, Project: group.project, Path: path, Error: err.Error()})
				continue
			}
			r := decode(name, group.project, path, string(data))
			if info, e := entry.Info(); e == nil {
				r.Updated = info.ModTime()
			}
			out.Rules = append(out.Rules, r)
		}
	}
	sort.SliceStable(out.Rules, func(i, j int) bool {
		if out.Rules[i].Project != out.Rules[j].Project {
			return out.Rules[i].Project != ""
		}
		return out.Rules[i].Name < out.Rules[j].Name
	})
	return out, nil
}

func (s *Service) Save(ctx context.Context, p protocol.RuleSaveParams) (protocol.Rule, error) {
	r := p.Rule
	path, err := ruleFile(protocol.RuleRef{Name: r.Name, Project: r.Project})
	if err != nil {
		return r, err
	}
	r.Body = strings.TrimSpace(r.Body)
	if r.Body == "" || strings.TrimSpace(r.Description) == "" {
		return r, protocol.Errorf(protocol.CodeBadRequest, "a rule needs a description and guidance")
	}
	if len(r.Body) > 32<<10 || len(r.Description) > 512 {
		return r, protocol.Errorf(protocol.CodeBadRequest, "rule is too large")
	}
	if len(r.Condition) == 0 && strings.TrimSpace(r.Question) == "" && !r.AlwaysApply {
		return r, protocol.Errorf(protocol.CodeBadRequest, "a rule needs a condition, question or alwaysApply")
	}
	c := compile(r, false)
	if c.rule.Error != "" {
		return r, protocol.Errorf(protocol.CodeBadRequest, "%s", c.rule.Error)
	}
	if ctx.Err() != nil {
		return r, ctx.Err()
	}
	data := encode(r)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return r, err
	}
	if !p.Overwrite {
		file, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(openErr, os.ErrExist) {
			return r, protocol.Errorf(protocol.CodeConflict, "rule %q already exists", r.Name)
		}
		if openErr != nil {
			return r, openErr
		}
		n, writeErr := file.Write([]byte(data))
		closeErr := file.Close()
		if writeErr != nil {
			return r, writeErr
		}
		if closeErr != nil {
			return r, closeErr
		}
		if n != len(data) {
			return r, fmt.Errorf("short write saving rule %q", r.Name)
		}
	} else {
		info, statErr := os.Lstat(path)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return r, statErr
		}
		if statErr == nil && !info.Mode().IsRegular() {
			return r, protocol.Errorf(protocol.CodeBadRequest, "rule path is not a regular file")
		}
		tmp, tmpErr := os.CreateTemp(filepath.Dir(path), ".rule-*")
		if tmpErr != nil {
			return r, tmpErr
		}
		defer os.Remove(tmp.Name())
		if err = tmp.Chmod(0600); err != nil {
			tmp.Close()
			return r, err
		}
		n, writeErr := tmp.Write([]byte(data))
		closeErr := tmp.Close()
		if writeErr != nil {
			return r, writeErr
		}
		if closeErr != nil {
			return r, closeErr
		}
		if n != len(data) {
			return r, fmt.Errorf("short write saving rule %q", r.Name)
		}
		if err = os.Rename(tmp.Name(), path); err != nil {
			return r, err
		}
	}
	r.Path = path
	r.Updated = time.Now().UTC()
	return r, nil
}

func (s *Service) Delete(ctx context.Context, ref protocol.RuleRef) error {
	path, err := ruleFile(ref)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return protocol.Errorf(protocol.CodeNotFound, "rule %q not found", ref.Name)
	}
	return err
}

func (s *Service) discovered(ctx context.Context, project string) []*compiled {
	listed, err := s.List(ctx, project)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []*compiled
	for _, r := range listed.Rules {
		if seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		out = append(out, compile(r, false))
	}
	for _, r := range builtinRules() {
		if !seen[r.Name] {
			out = append(out, compile(r, true))
		}
	}
	return out
}

func (s *Service) Blocks(ctx context.Context, tc engine.TurnContext) []engine.Block {
	if !s.enabled() {
		return nil
	}
	var lines []string
	s.mu.Lock()
	if notices := s.pending[tc.Session.ID]; len(notices) > 0 {
		lines = append(lines, notices...)
		delete(s.pending, tc.Session.ID)
	}
	s.mu.Unlock()
	for _, c := range s.discovered(ctx, tc.Project) {
		r := c.rule
		if !r.Enabled || r.Error != "" || c.streams() || c.judged() {
			continue
		}
		if r.AlwaysApply {
			lines = append(lines, "### "+r.Name+"\n"+r.Body)
		} else if r.Description != "" {
			lines = append(lines, "- "+r.Name+" ("+strings.Join(r.Globs, ", ")+"): "+r.Description)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	budget := 16000
	if tc.Profile.Compact() {
		budget = 2000
	}
	return []engine.Block{{Name: engine.BlockRules, Text: strings.Join(lines, "\n\n"), Budget: budget}}
}
