package settings

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

type instanceEditor struct {
	fold                       foldRow
	name, command, environment kit.TextField
	save, remove               kit.Plaque
	loaded                     bool
	problem                    string
}

const newInstance = "\x00new"

func (p *Page) layoutInstances(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	instances := e.Snapshot().Settings.Agent.Instances
	return foldSlab(gtx, th, len(instances)+1, func(gtx layout.Context, index int) layout.Dimensions {
		if index == len(instances) {
			head := foldHead{Title: i18n.T("Add a harness"), Detail: i18n.T("A name and the command that starts it.")}
			return p.layoutInstance(gtx, e, &p.fresh, protocol.HarnessInstance{}, newInstance, head)
		}
		instance := instances[index]
		editor := p.instances[instance.ID]
		if editor == nil {
			editor = new(instanceEditor)
			p.instances[instance.ID] = editor
		}
		head := foldHead{Title: instance.Name, Detail: strings.Join(instance.Command, " "), Mono: true}
		return p.layoutInstance(gtx, e, editor, instance, instance.ID, head)
	})
}

func (p *Page) layoutInstance(
	gtx layout.Context, e *env.Env, editor *instanceEditor, instance protocol.HarnessInstance, key string, head foldHead,
) layout.Dimensions {
	th := e.Theme
	create := key == newInstance
	if editor.fold.toggled(gtx) {
		if p.openInstance == key {
			p.openInstance = ""
		} else {
			p.openInstance = key
		}
	}
	return editor.fold.layout(gtx, th, p.openInstance == key, head, func(gtx layout.Context) layout.Dimensions {
		p.updateInstance(gtx, e, editor, instance, create)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(.4, func(gtx layout.Context) layout.Dimensions { return editor.name.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Flexed(.6, func(gtx layout.Context) layout.Dimensions { return editor.command.Layout(gtx, th.Theme) }),
				)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return editor.environment.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return editor.save.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if create {
							return layout.Dimensions{}
						}
						return editor.remove.Layout(gtx, th.Theme)
					}),
				)
			}),
		)
	})
}

func (p *Page) updateInstance(gtx layout.Context, e *env.Env, editor *instanceEditor, instance protocol.HarnessInstance, create bool) {
	if !editor.loaded {
		editor.loaded = true
		editor.name.SetText(instance.Name)
		editor.command.SetText(strings.Join(instance.Command, " "))
		editor.environment.SetText(environmentText(instance.Env))
	}
	editor.name.Label, editor.name.Placeholder = i18n.T("Name"), i18n.T("My agent")
	editor.name.Editor.SingleLine = true
	editor.command.Label, editor.command.Placeholder, editor.command.Mono = i18n.T("Command"), "my-agent --acp", true
	editor.command.Editor.SingleLine = true
	editor.environment.Label, editor.environment.Placeholder, editor.environment.Mono = i18n.T("Environment"), "KEY=value", true
	editor.environment.Help = i18n.T("One per line. Leave it empty to use your own environment.")
	editor.environment.Error = editor.problem
	editor.save.Kind, editor.save.Small, editor.save.Label = kit.PlaqueTonal, true, i18n.T("Save")
	if create {
		editor.save.Label = i18n.T("Add harness")
	}
	editor.remove.Kind, editor.remove.Small, editor.remove.Label = kit.PlaqueQuiet, true, i18n.T("Remove")
	if editor.save.Clicked(gtx) {
		p.saveInstance(e, editor, instance, create)
	}
	if !create && editor.remove.Clicked(gtx) {
		kept := make([]protocol.HarnessInstance, 0, len(e.Snapshot().Settings.Agent.Instances))
		for _, candidate := range e.Snapshot().Settings.Agent.Instances {
			if candidate.ID != instance.ID {
				kept = append(kept, candidate)
			}
		}
		set(e, "agent", map[string]any{"instances": kept})
		delete(p.instances, instance.ID)
		p.openInstance = ""
	}
}

func (p *Page) saveInstance(e *env.Env, editor *instanceEditor, instance protocol.HarnessInstance, create bool) {
	name := strings.TrimSpace(editor.name.Text())
	command, commandErr := splitCommandLine(editor.command.Text())
	environment, environmentErr := parseEnvironment(editor.environment.Text())
	switch {
	case name == "" || len(command) == 0:
		editor.problem = i18n.T("It needs a name and a command.")
		return
	case commandErr != nil:
		editor.problem = commandErr.Error()
		return
	case environmentErr != nil:
		editor.problem = environmentErr.Error()
		return
	}
	editor.problem = ""
	instances := append([]protocol.HarnessInstance(nil), e.Snapshot().Settings.Agent.Instances...)
	replacement := protocol.HarnessInstance{ID: instance.ID, Name: name, Command: command, Env: environment}
	if create {
		replacement.ID = instanceID(name, instances)
		instances = append(instances, replacement)
	} else {
		for index := range instances {
			if instances[index].ID == instance.ID {
				instances[index] = replacement
			}
		}
	}
	e.SetSettings(map[string]any{"agent": map[string]any{"instances": instances}}, func(err error) {
		switch {
		case err != nil:
			editor.problem = err.Error()
		case create:
			p.fresh, p.openInstance = instanceEditor{}, ""
		}
	})
}

func instanceID(name string, taken []protocol.HarnessInstance) string {
	var id strings.Builder
	for _, char := range strings.ToLower(name) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			id.WriteRune(char)
		case id.Len() > 0 && !strings.HasSuffix(id.String(), "-"):
			id.WriteByte('-')
		}
	}
	base := strings.Trim(id.String(), "-")
	if base == "" {
		base = "harness"
	}
	candidate := base
	for suffix := 2; ; suffix++ {
		used := false
		for _, instance := range taken {
			used = used || instance.ID == candidate
		}
		if !used {
			return candidate
		}
		candidate = base + "-" + strconv.Itoa(suffix)
	}
}

func environmentText(environment map[string]string) string {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+environment[key])
	}
	return strings.Join(lines, "\n")
}

func parseEnvironment(text string) (map[string]string, error) {
	result := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsAny(key, " \t") {
			return nil, errors.New(i18n.T("Each environment line is KEY=value."))
		}
		result[key] = value
	}
	return result, nil
}

// splits like a shell would, without running one
func splitCommandLine(text string) ([]string, error) {
	var result []string
	var word strings.Builder
	quote := rune(0)
	escaped := false
	flush := func() {
		if word.Len() > 0 {
			result = append(result, word.String())
			word.Reset()
		}
	}
	for _, char := range strings.TrimSpace(text) {
		if escaped {
			word.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				word.WriteRune(char)
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case ' ', '\t', '\n':
			flush()
		default:
			word.WriteRune(char)
		}
	}
	if escaped || quote != 0 {
		return nil, errors.New(i18n.T("The command has an unfinished quote or escape."))
	}
	flush()
	return result, nil
}
