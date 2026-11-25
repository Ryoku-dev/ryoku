package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Source is one capture source the mic chooser can offer: its node name
// (what Start takes), its human description, and whether it is a monitor of
// an output rather than a microphone.
type Source struct {
	Name    string
	Desc    string
	Monitor bool
}

// pactlSource is the part of one `pactl -f json list sources` entry read
// here. Monitors are told by the monitor-of-sink fields (pactl versions
// name it differently and print "n/a" or null for none) or by the
// device.class property.
type pactlSource struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	MonitorOfSink *string        `json:"monitor_of_sink"`
	MonitorSource *string        `json:"monitor_source"`
	Properties    map[string]any `json:"properties"`
}

func (p pactlSource) monitor() bool {
	for _, s := range []*string{p.MonitorOfSink, p.MonitorSource} {
		if s != nil && *s != "" && *s != "n/a" {
			return true
		}
	}
	class, _ := p.Properties["device.class"].(string)
	return class == "monitor"
}

// Sources lists the microphones: every capture source that is not a
// monitor of an output.
func Sources(ctx context.Context, run Runner) ([]Source, error) {
	all, err := AllSources(ctx, run)
	if err != nil {
		return nil, err
	}
	mics := all[:0]
	for _, s := range all {
		if !s.Monitor {
			mics = append(mics, s)
		}
	}
	return mics, nil
}

// AllSources lists every capture source, monitors included and marked, in
// the order the sound server gives. A source with no usable name is
// skipped; one Start would refuse is skipped too.
func AllSources(ctx context.Context, run Runner) ([]Source, error) {
	out, err := guarded(run)(ctx, "pactl", "-f", "json", "list", "sources")
	if err != nil {
		return nil, err
	}
	var raw []pactlSource
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("audio: pactl source list: %w", err)
	}
	list := make([]Source, 0, len(raw))
	for _, p := range raw {
		if !validNode(p.Name) {
			continue
		}
		desc := strings.TrimSpace(p.Description)
		if desc == "" {
			desc = p.Name
		}
		list = append(list, Source{Name: p.Name, Desc: desc, Monitor: p.monitor()})
	}
	return list, nil
}

// DefaultSource is the sound server's default capture source name, "" when
// it names none Start would accept.
func DefaultSource(ctx context.Context, run Runner) (string, error) {
	out, err := guarded(run)(ctx, "pactl", "get-default-source")
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(out))
	if !validNode(name) {
		return "", nil
	}
	return name, nil
}

// guarded checks every call against the allowlist before run sees it.
func guarded(run Runner) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if !Allowed(name, args...) {
			return nil, refuse(name, args)
		}
		if run == nil {
			return nil, fmt.Errorf("audio: no runner for %s", name)
		}
		return run(ctx, name, args...)
	}
}
