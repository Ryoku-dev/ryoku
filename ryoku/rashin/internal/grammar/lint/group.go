package lint

import (
	"container/list"
	"context"
	"encoding/json"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"sort"
	"sync"
)

type groupRule struct {
	name       string
	linter     Linter
	expression ExprLinter
	sentence   bool
}
type LintGroup struct {
	Config              FlatConfig
	rules               [3]map[string]groupRule
	ClashingLinterNames []string
	caches              [2]unitCache
}

func NewLintGroup() *LintGroup {
	out := &LintGroup{Config: NewFlatConfig()}
	for phase := range out.rules {
		out.rules[phase] = make(map[string]groupRule)
	}
	return out
}
func (g *LintGroup) ContainsKey(name string) bool {
	for _, rules := range g.rules {
		if _, ok := rules[name]; ok {
			return true
		}
	}
	return false
}
func (g *LintGroup) add(r groupRule) bool {
	if g.ContainsKey(r.name) {
		g.ClashingLinterNames = append(g.ClashingLinterNames, r.name)
		return false
	}
	phase := 0
	if r.expression != nil {
		phase = 1
		if r.sentence {
			phase = 2
		}
	}
	if g.rules[phase] == nil {
		g.rules[phase] = make(map[string]groupRule)
	}
	g.rules[phase][r.name] = r
	return true
}
func (g *LintGroup) Add(name string, l Linter) bool { return g.add(groupRule{name: name, linter: l}) }
func (g *LintGroup) AddChunkExprLinter(name string, l ExprLinter) bool {
	return g.add(groupRule{name: name, expression: l})
}
func (g *LintGroup) AddSentenceExprLinter(name string, l ExprLinter) bool {
	return g.add(groupRule{name: name, expression: l, sentence: true})
}
func (g *LintGroup) MergeFrom(other *LintGroup) {
	g.Config.MergeFrom(other.Config)
	for phase, incoming := range other.rules {
		keys := sortedRuleKeys(incoming)
		for _, key := range keys {
			if g.ContainsKey(key) {
				g.ClashingLinterNames = append(g.ClashingLinterNames, key)
				break
			}
		}
		if g.rules[phase] == nil {
			g.rules[phase] = make(map[string]groupRule)
		}
		for _, key := range keys {
			g.rules[phase][key] = incoming[key]
		}
	}
}
func (g *LintGroup) IterKeys() []string {
	var keys []string
	for _, rules := range g.rules {
		keys = append(keys, sortedRuleKeys(rules)...)
	}
	return keys
}
func sortedRuleKeys(rules map[string]groupRule) []string {
	keys := make([]string, 0, len(rules))
	for key := range rules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func (g *LintGroup) SetAllRulesTo(enabled *bool) {
	for _, key := range g.IterKeys() {
		if enabled == nil {
			g.Config.UnsetRuleEnabled(key)
		} else {
			g.Config.SetRuleEnabled(key, *enabled)
		}
	}
}
func (g *LintGroup) AllDescriptions() map[string]string {
	out := map[string]string{}
	for _, rules := range g.rules {
		for k, v := range rules {
			if v.linter != nil {
				out[k] = v.linter.Description()
			} else {
				out[k] = v.expression.Description()
			}
		}
	}
	return out
}
func (g *LintGroup) WithLintConfig(c FlatConfig) *LintGroup { g.Config = c; return g }
func (g *LintGroup) Clone() *LintGroup {
	out := NewLintGroup()
	out.Config = g.Config.Clone()
	for phase, rules := range g.rules {
		for k, v := range rules {
			out.rules[phase][k] = v
		}
	}
	out.ClashingLinterNames = append([]string(nil), g.ClashingLinterNames...)
	return out
}
func (g *LintGroup) OrganizedLints(d *doc.Document) map[string][]Lint {
	out, _ := g.OrganizedLintsContext(context.Background(), d)
	return out
}
func (g *LintGroup) OrganizedLintsContext(ctx context.Context, d *doc.Document) (map[string][]Lint, error) {
	out := map[string][]Lint{}
	for _, key := range sortedRuleKeys(g.rules[0]) {
		r := g.rules[0][key]
		if !g.Config.IsRuleEnabled(key) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out[key] = r.linter.Lint(d)
	}
	config, _ := json.Marshal(g.Config)
	configKey := string(config)
	for phase := range 2 {
		keys := sortedRuleKeys(g.rules[phase+1])
		enabled := keys[:0]
		for _, key := range keys {
			if g.Config.IsRuleEnabled(key) {
				enabled = append(enabled, key)
			}
		}
		keys = enabled
		units := d.IterChunks()
		if phase == 1 {
			units = d.IterSentences()
		}
		initialized := false
		for _, unit := range units {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if len(unit) == 0 {
				continue
			}
			if !initialized {
				for _, key := range keys {
					if _, exists := out[key]; !exists {
						out[key] = nil
					}
				}
				initialized = true
			}
			start, end := unit[0].Span.Start, unit[len(unit)-1].Span.End
			cacheKey := unitCacheKey{content: string(d.GetSource()[start:end]), config: configKey}
			cached, ok := g.caches[phase].get(cacheKey)
			if !ok {
				cached = nil
				for _, key := range keys {
					r := g.rules[phase+1][key]
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					values := RunOnChunk(r.expression, unit, d.GetSource())
					for i := range values {
						values[i].Span.Start -= start
						values[i].Span.End -= start
					}
					if len(values) != 0 {
						if cached == nil {
							cached = make(map[string][]Lint)
						}
						cached[key] = values
					}
				}
				g.caches[phase].put(cacheKey, cached)
			}
			for key, values := range cached {
				for _, value := range values {
					value = cloneLint(value)
					value.Span.Start += start
					value.Span.End += start
					out[key] = append(out[key], value)
				}
			}
		}
	}
	return out, ctx.Err()
}
func (g *LintGroup) Lint(d *doc.Document) []Lint {
	m := g.OrganizedLints(d)
	var out []Lint
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, m[key]...)
	}
	return out
}
func (g *LintGroup) Description() string { return "A collection of linters that can be run as one." }

type unitCacheKey struct{ content, config string }
type unitCacheEntry struct {
	key    unitCacheKey
	values map[string][]Lint
}
type unitCache struct {
	entries map[unitCacheKey]*list.Element
	order   list.List
}

func (c *unitCache) get(key unitCacheKey) (map[string][]Lint, bool) {
	element, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(element)
	return element.Value.(unitCacheEntry).values, true
}
func (c *unitCache) put(key unitCacheKey, values map[string][]Lint) {
	if c.entries == nil {
		c.entries = make(map[unitCacheKey]*list.Element)
	}
	if element, ok := c.entries[key]; ok {
		element.Value = unitCacheEntry{key, values}
		c.order.MoveToFront(element)
		return
	}
	c.entries[key] = c.order.PushFront(unitCacheEntry{key, values})
	if len(c.entries) > 1000 {
		last := c.order.Back()
		delete(c.entries, last.Value.(unitCacheEntry).key)
		c.order.Remove(last)
	}
}
func cloneLint(value Lint) Lint {
	if value.Suggestions != nil {
		value.Suggestions = append([]Suggestion{}, value.Suggestions...)
		for i := range value.Suggestions {
			value.Suggestions[i].Replacement = append([]rune(nil), value.Suggestions[i].Replacement...)
		}
	}
	return value
}

// RemoveOverlaps preserves Harper's stable ordering: source start, longest end,
// then priority. Priority only breaks ties for identical spans.
func RemoveOverlaps(lints []Lint) []Lint {
	if len(lints) < 2 {
		return lints
	}
	sort.SliceStable(lints, func(i, j int) bool { return lints[i].Priority < lints[j].Priority })
	sort.SliceStable(lints, func(i, j int) bool {
		if lints[i].Span.Start != lints[j].Span.Start {
			return lints[i].Span.Start < lints[j].Span.Start
		}
		return lints[i].Span.End > lints[j].Span.End
	})
	cur := 0
	n := 0
	for _, l := range lints {
		if l.Span.Start < cur {
			continue
		}
		cur = l.Span.End
		lints[n] = l
		n++
	}
	return lints[:n]
}
func RemoveOverlapsMap(m map[string][]Lint) {
	type item struct {
		key   string
		index int
		l     Lint
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var all []item
	for _, k := range keys {
		for i, l := range m[k] {
			all = append(all, item{k, i, l})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].l.Priority < all[j].l.Priority })
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i].l.Span, all[j].l.Span
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		return a.End > b.End
	})
	removed := map[string]map[int]bool{}
	cur := 0
	for _, v := range all {
		if v.l.Span.Start < cur {
			if removed[v.key] == nil {
				removed[v.key] = map[int]bool{}
			}
			removed[v.key][v.index] = true
		} else {
			cur = v.l.Span.End
		}
	}
	for _, k := range keys {
		n := 0
		for i, l := range m[k] {
			if !removed[k][i] {
				m[k][n] = l
				n++
			}
		}
		m[k] = m[k][:n]
	}
}
func RemoveLintsOverlappingExpr(e expr.Expr, d *doc.Document, lints []Lint) []Lint {
	tokens, source := d.GetTokens(), d.GetSource()
	n := 0
	for _, l := range lints {
		overlap := false
		for cursor := range tokens {
			span, ok := e.Run(cursor, tokens, source)
			if !ok || span.Start == span.End {
				continue
			}
			start, end := tokens[span.Start].Span.Start, tokens[span.End-1].Span.End
			if l.Span.Start < end && start < l.Span.End {
				overlap = true
				break
			}
		}
		if !overlap {
			lints[n] = l
			n++
		}
	}
	return lints[:n]
}

type PooledLinter[L Linter] struct {
	pool        *sync.Pool
	description string
}

func NewPooledLinter[L Linter](ctor func() L) *PooledLinter[L] {
	p := &sync.Pool{New: func() any { return ctor() }}
	l := p.Get().(L)
	description := l.Description()
	p.Put(l)
	return &PooledLinter[L]{pool: p, description: description}
}
func (p *PooledLinter[L]) RunWithInner(callback func(L)) {
	l := p.pool.Get().(L)
	defer p.pool.Put(l)
	callback(l)
}
func (p *PooledLinter[L]) Lint(d *doc.Document) []Lint {
	var out []Lint
	p.RunWithInner(func(l L) { out = l.Lint(d) })
	return out
}
func (p *PooledLinter[L]) Description() string { return p.description }
func (p *PooledLinter[L]) Clone() *PooledLinter[L] {
	return &PooledLinter[L]{pool: p.pool, description: p.description}
}
