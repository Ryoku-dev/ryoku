package teams

import (
	"maps"
	"slices"
	"strings"

	i18n "ryoku-i18n"

	registry "ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/protocol"
)

// taskEngine is the harness, model and options a task starts with: its
// member's engine through the teams defaults (protocol.ResolveEngine, with
// the registry's default harness last), then the model asked for this task
// alone and its options, each one over the member's.
func (s *Service) taskEngine(member protocol.Member, task protocol.TeamTask) (string, string, map[string]string) {
	settings := s.settings()
	id, model, options := protocol.ResolveEngine(member, settings, s.defaultHarness(settings))
	if task.Model != "" {
		model = task.Model
	}
	options = maps.Clone(options)
	if len(task.Options) > 0 {
		if options == nil {
			options = make(map[string]string, len(task.Options))
		}
		maps.Copy(options, task.Options)
	}
	return id, model, options
}

// checkEngine says what is wrong with a member's engine. Its harness must
// be one this computer knows, installed or not; kept lists the harnesses
// the team already ran on, which stay accepted after their definition went.
func checkEngine(m protocol.Member, settings protocol.Settings, kept map[string]bool) error {
	if m.Harness != "" && !kept[m.Harness] && !registry.Known(m.Harness, settings) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("No harness %s on this computer.", m.Harness))
	}
	if !protocol.ValidEngineModel(m.Model) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A model id has at most %d characters and no spaces.", protocol.EngineModelMax))
	}
	if !protocol.ValidEngineOptions(m.Options) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A harness takes at most %d options, each with an id and a value.", protocol.EngineOptionsMax))
	}
	return nil
}

// engineWords says an engine the way the ledger reads: the harness by its
// name and the model by its id, "its default model" when it names none.
func (s *Service) engineWords(id, model string) string {
	name := i18n.T("the teams default harness")
	if id != "" {
		name = registry.Name(id, s.settings())
	}
	if model == "" {
		model = i18n.T("its default model")
	}
	return i18n.Tf("%s, %s", name, model)
}

// memberEngineWords says a member's engine with its options, as a team
// change records it.
func (s *Service) memberEngineWords(m protocol.Member) string {
	words := s.engineWords(m.Harness, m.Model)
	if len(m.Options) == 0 {
		return words
	}
	parts := make([]string, 0, len(m.Options))
	for _, option := range slices.Sorted(maps.Keys(m.Options)) {
		parts = append(parts, option+" "+m.Options[option])
	}
	return i18n.Tf("%s (%s)", words, strings.Join(parts, ", "))
}
