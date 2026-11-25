package tool

import (
	"context"
	"fmt"
	"sync"

	"ryoku-rashin/internal/llm"
)

// Registry holds every built-in tool once, by name, plus the providers of
// tools that come and go. The daemon builds it at start from the tools
// packages; each runner asks it for the set its profile offers.
type Registry struct {
	mu        sync.RWMutex
	tools     map[string]Tool
	providers []Provider
}

// NewRegistry registers tools. A second tool with a name already taken is a
// programming error and panics.
func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	r.Add(tools...)
	return r
}

// Add registers more tools; see NewRegistry.
func (r *Registry) Add(tools ...Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range tools {
		name := t.Name()
		if _, taken := r.tools[name]; taken {
			panic(fmt.Sprintf("tool: %q registered twice", name))
		}
		r.tools[name] = t
	}
}

// AddProvider registers a source of dynamic tools.
func (r *Registry) AddProvider(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, p)
}

// Get returns a registered built-in tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// For resolves the tools env.Profile offers this round: the canonical
// names of its mode and class, in table order, that are registered and
// enabled for env, then (where ProvidersJoin) the providers' tools. A
// ReadOnly profile keeps only ReadOnly tools. A compact profile never holds
// more than MaxCompactTools.
func (r *Registry) For(ctx context.Context, env Env) Set {
	p := env.Profile
	r.mu.RLock()
	builtins := r.tools
	providers := append([]Provider(nil), r.providers...)
	var picked []Tool
	for _, name := range Names(p.Mode, p.Class) {
		if t, ok := builtins[name]; ok && offered(t, env) {
			picked = append(picked, t)
		}
	}
	r.mu.RUnlock()

	if ProvidersJoin(p.Mode, p.Class) {
		seen := map[string]bool{}
		for _, t := range picked {
			seen[t.Name()] = true
		}
		for _, prov := range providers {
			for _, t := range prov.Tools(ctx, env) {
				name := t.Name()
				if seen[name] || r.builtin(name) || !offered(t, env) {
					continue
				}
				seen[name] = true
				picked = append(picked, t)
			}
		}
	}
	if p.Class == ClassCompact && len(picked) > MaxCompactTools {
		picked = picked[:MaxCompactTools]
	}
	return newSet(picked, p)
}

func (r *Registry) builtin(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

func offered(t Tool, env Env) bool {
	if env.Profile.ReadOnly && !t.ReadOnly() {
		return false
	}
	if c, ok := t.(Conditional); ok && !c.Enabled(env) {
		return false
	}
	return true
}

// Set is the tools of one round.
type Set struct {
	tools   []Tool
	byName  map[string]Tool
	profile Profile
}

func newSet(tools []Tool, p Profile) Set {
	byName := make(map[string]Tool, len(tools))
	for _, t := range tools {
		byName[t.Name()] = t
	}
	return Set{tools: tools, byName: byName, profile: p}
}

// Tools are the set's tools in the order the model sees them.
func (s Set) Tools() []Tool { return append([]Tool(nil), s.tools...) }

// Specs are the schemas for llm.Request.Tools.
func (s Set) Specs() []llm.Tool {
	specs := make([]llm.Tool, 0, len(s.tools))
	for _, t := range s.tools {
		specs = append(specs, t.Spec(s.profile))
	}
	return specs
}

// Lookup finds a tool the model called. A name outside the set is not
// found, even when registered: the model may only call what it was offered.
func (s Set) Lookup(name string) (Tool, bool) {
	t, ok := s.byName[name]
	return t, ok
}

// Names lists the set's tool names in order.
func (s Set) Names() []string {
	names := make([]string, 0, len(s.tools))
	for _, t := range s.tools {
		names = append(names, t.Name())
	}
	return names
}

// Len counts the set's tools.
func (s Set) Len() int { return len(s.tools) }
