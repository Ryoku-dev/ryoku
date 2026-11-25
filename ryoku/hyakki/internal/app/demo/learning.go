package demo

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type learningFixtures struct {
	skills     []protocol.ManagedSkill
	banks      []protocol.DecisionBank
	rules      []protocol.Rule
	grievances []protocol.Grievance
}

func demoLearning(now time.Time) learningFixtures {
	root := paths.ManagedSkillsDir()
	skill := func(name, description string, bytes int64, days int, shadowed bool) protocol.ManagedSkill {
		return protocol.ManagedSkill{Name: name, Description: description, Bytes: bytes, Updated: now.AddDate(0, 0, -days),
			Shadowed: shadowed, Path: filepath.Join(root, name, "SKILL.md")}
	}
	consolidated := func(age time.Duration) *time.Time { t := now.Add(-age); return &t }
	return learningFixtures{
		skills: []protocol.ManagedSkill{
			skill("release-notes-from-git-log", i18n.T("Write release notes from the Git log since the last tag, grouped by area"), 1840, 2, false),
			skill("niri-window-rule-debug", i18n.T("Find why a niri window rule does not apply"), 2310, 5, false),
			skill("review", i18n.T("Review a diff for regressions before a commit"), 960, 9, true),
		},
		banks: []protocol.DecisionBank{
			{Project: demoHyperionRoot, Bank: demoBankName(demoHyperionRoot), Queued: 2, Consolidated: consolidated(3 * time.Hour),
				Architecture: i18n.T("## Portability\n- Everything works on systemd and runit, and on Hyprland and niri\n- QML never probes the system itself: the daemon gathers and hands over\n\n## Hyakki\n- Hyakki keeps no credentials of its own: it uses the Ryoku vault\n"),
				Product:      i18n.T("## Updates\n- A branch switch or an update never loses local work\n\n## Defaults\n- A new default comes with a toggle\n"),
				Style:        i18n.T("## Lithic\n- No lit top edge on a button, no accent bar on the left of an option\n- Type is Geologica and Martian Mono\n")},
			{Project: demoRyotunesRoot, Bank: demoBankName(demoRyotunesRoot), Consolidated: consolidated(4 * 24 * time.Hour),
				Architecture: i18n.T("- Provider sign-in survives long stretches offline\n"),
				Product:      i18n.T("- The volume control is easy to drag precisely\n- Compact mode can always be opened again\n")},
		},
		rules: []protocol.Rule{
			{Name: "no-em-dash", Project: demoHyperionRoot, Description: i18n.T("Never write an em dash"), Condition: []string{"\u2014"}, Scope: "text", Interrupt: protocol.RuleInterruptAlways, Repeat: protocol.RuleRepeatAlways, Enabled: true, Forged: true,
				Body: i18n.T("Rewrite the sentence with a comma, a colon or two sentences."), Updated: now.Add(-24 * time.Hour), Path: filepath.Join(paths.ProjectRulesDir(demoHyperionRoot), "no-em-dash.md")},
			{Name: "tests-before-done", Description: i18n.T("Run the tests before saying a change is done"), Question: i18n.T("Did the reply claim the work is done without a test run?"), Interrupt: protocol.RuleInterruptNever, Repeat: protocol.RuleRepeatAfterGap, Enabled: true,
				Body: i18n.T("Run the narrowest test that covers the change and report its result."), Updated: now.Add(-6 * 24 * time.Hour), Path: filepath.Join(paths.RulesDir(), "tests-before-done.md")},
			{Name: "plain-words", Description: i18n.T("Answer in plain words"), AlwaysApply: true,
				Body: i18n.T("Short sentences. No filler. Say what changed and why."), Updated: now.Add(-20 * 24 * time.Hour), Path: filepath.Join(paths.RulesDir(), "plain-words.md")},
		},
		grievances: []protocol.Grievance{
			{ID: 4, Tool: "edit", Report: i18n.T("edit said the text was not found, but read shows the same line with a tab instead of spaces"), Source: "builtin", Model: demoLocalID, Session: "agent-done", At: now.Add(-40 * time.Minute)},
			{ID: 3, Tool: "grep", Report: i18n.T("grep returned no matches for a pattern that find_files shows in the same folder"), Source: "builtin", Model: demoLocalID, Session: "agent-done", At: now.Add(-3 * time.Hour)},
			{ID: 2, Tool: "edit", Report: i18n.T("edit applied the replacement twice when all was false"), Source: "builtin", Model: demoFreeID, Session: "agent-working", At: now.Add(-24 * time.Hour)},
			{ID: 1, Tool: "search_issues", Report: i18n.T("The description says it takes a repo, but the call fails without owner"), Source: "mcp:github", Model: demoFreeID, Session: "agent-working", At: now.Add(-48 * time.Hour)},
		},
	}
}

func demoBankName(project string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(project))
	return fmt.Sprintf("%s-%016x", filepath.Base(project), h.Sum64())
}

func (s *server) callLearning(method string, raw json.RawMessage) (any, error) {
	fx := &s.data.learning
	switch method {
	case protocol.MLearningSkills:
		return protocol.LearningSkillsResult{Root: paths.ManagedSkillsDir(), Skills: slices.Clone(fx.skills)}, nil
	case protocol.MLearningSkillDelete:
		var p protocol.LearningSkillParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		index := slices.IndexFunc(fx.skills, func(skill protocol.ManagedSkill) bool { return skill.Name == p.Name })
		if index < 0 {
			return nil, notFound("managed skill")
		}
		fx.skills = append(fx.skills[:index:index], fx.skills[index+1:]...)
		s.learningChanged(protocol.LearningSkills, "")
		return protocol.LearningSkillsResult{Root: paths.ManagedSkillsDir(), Skills: slices.Clone(fx.skills)}, nil
	case protocol.MLearningDecisions:
		var p protocol.LearningDecisionsParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		banks := make([]protocol.DecisionBank, 0, len(fx.banks))
		for _, bank := range fx.banks {
			if p.Project == "" || p.Project == bank.Project {
				banks = append(banks, bank)
			}
		}
		return protocol.LearningDecisionsResult{Banks: banks}, nil
	case protocol.MLearningDecisionDelete:
		var p protocol.DecisionDeleteParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		for i := range fx.banks {
			bank := &fx.banks[i]
			if bank.Project != p.Project {
				continue
			}
			var text *string
			switch p.Area {
			case protocol.DecisionArchitecture:
				text = &bank.Architecture
			case protocol.DecisionProduct:
				text = &bank.Product
			case protocol.DecisionStyle:
				text = &bank.Style
			default:
				return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a decision area."))
			}
			lines := strings.Split(*text, "\n")
			for j, line := range lines {
				if strings.TrimSpace(line) != strings.TrimSpace(p.Line) {
					continue
				}
				*text = strings.Join(append(lines[:j:j], lines[j+1:]...), "\n")
				s.learningChanged(protocol.LearningDecisions, p.Project)
				return *bank, nil
			}
		}
		return nil, notFound("decision")
	case protocol.MLearningRules:
		var p protocol.LearningRulesParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		rules := make([]protocol.Rule, 0, len(fx.rules))
		for _, rule := range fx.rules {
			if p.Project == "" || rule.Project == "" || rule.Project == p.Project {
				rules = append(rules, rule)
			}
		}
		slices.SortFunc(rules, func(a, b protocol.Rule) int {
			switch {
			case a.Project != "" && b.Project == "":
				return -1
			case a.Project == "" && b.Project != "":
				return 1
			default:
				return strings.Compare(a.Name, b.Name)
			}
		})
		return protocol.LearningRulesResult{Rules: rules}, nil
	case protocol.MLearningRuleSave:
		var p protocol.RuleSaveParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.Rule.Name == "" || strings.ContainsAny(p.Rule.Name, "/\\") {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Name the rule before saving it."))
		}
		for _, expression := range p.Rule.Condition {
			if _, err := regexp.Compile(expression); err != nil {
				return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", err.Error())
			}
		}
		index := slices.IndexFunc(fx.rules, func(rule protocol.Rule) bool { return rule.Name == p.Rule.Name && rule.Project == p.Rule.Project })
		if index >= 0 && !p.Overwrite {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("That rule already exists."))
		}
		folder := paths.RulesDir()
		if p.Rule.Project != "" {
			folder = paths.ProjectRulesDir(p.Rule.Project)
		}
		p.Rule.Path, p.Rule.Updated = filepath.Join(folder, p.Rule.Name+".md"), time.Now()
		if index >= 0 {
			fx.rules[index] = p.Rule
		} else {
			fx.rules = append(fx.rules, p.Rule)
		}
		s.learningChanged(protocol.LearningRules, p.Rule.Project)
		return p.Rule, nil
	case protocol.MLearningRuleDelete:
		var p protocol.RuleRef
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		index := slices.IndexFunc(fx.rules, func(rule protocol.Rule) bool { return rule.Name == p.Name && rule.Project == p.Project })
		if index < 0 {
			return nil, notFound("rule")
		}
		fx.rules = append(fx.rules[:index:index], fx.rules[index+1:]...)
		s.learningChanged(protocol.LearningRules, p.Project)
		return protocol.Empty{}, nil
	case protocol.MLearningRuleForge:
		if !s.data.snapshot.Settings.Features[protocol.FeatureRules] {
			return nil, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("Turn on rules in Settings to forge one."))
		}
		var p protocol.RuleForgeParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.Session == "" {
			return protocol.RuleForgeResult{Message: i18n.T("That conversation has no reply to check a rule against.")}, nil
		}
		if _, ok := s.session(p.Session); !ok {
			return protocol.RuleForgeResult{Message: i18n.T("That conversation has no reply to check a rule against.")}, nil
		}
		description := []rune(p.Complaint)
		if len(description) > 60 {
			description = description[:60]
		}
		body := i18n.T("Write the sentence again without an em dash.")
		if p.Feedback != "" {
			body += " " + p.Feedback
		}
		return protocol.RuleForgeResult{OK: true, Attempts: 1, Matched: i18n.T("It works now \u2014 the build is green."), Rule: protocol.Rule{
			Name: "no-em-dash-in-replies", Description: string(description), Condition: []string{"\u2014"}, Scope: "text",
			Interrupt: protocol.RuleInterruptAlways, Repeat: protocol.RuleRepeatAlways, Enabled: true, Forged: true, Body: body,
		}}, nil
	case protocol.MLearningGrievances:
		var p protocol.LearningGrievancesParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		counts := map[string]int{}
		for _, item := range fx.grievances {
			counts[item.Tool]++
		}
		tools := make([]protocol.Count, 0, len(counts))
		for _, name := range []string{"edit", "grep", "search_issues"} {
			if counts[name] > 0 {
				tools = append(tools, protocol.Count{Name: name, Count: counts[name]})
			}
		}
		grievances := make([]protocol.Grievance, 0, len(fx.grievances))
		for _, item := range fx.grievances {
			if p.Tool == "" || p.Tool == item.Tool {
				grievances = append(grievances, item)
			}
		}
		total := len(grievances)
		if p.Limit > 0 && len(grievances) > p.Limit {
			grievances = grievances[:p.Limit]
		}
		return protocol.LearningGrievancesResult{Grievances: grievances, Total: total, Tools: tools}, nil
	case protocol.MLearningGrievancesClear:
		var p protocol.GrievancesClearParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		grievances := fx.grievances[:0]
		for _, item := range fx.grievances {
			remove := len(p.IDs) > 0 && slices.Contains(p.IDs, item.ID) || len(p.IDs) == 0 && (p.Tool == "" || p.Tool == item.Tool)
			if !remove {
				grievances = append(grievances, item)
			}
		}
		fx.grievances = grievances
		s.learningChanged(protocol.LearningGrievances, "")
		return protocol.Empty{}, nil
	}
	return nil, protocol.Errorf(protocol.CodeUnsupported, "demo method %s is not implemented", method)
}

func (s *server) learningChanged(kind, project string) {
	s.queueEvent(protocol.EvtLearningChanged, protocol.LearningChanged{Kind: kind, Project: project})
}

func (s *server) managedSkill(params protocol.SkillParams) (protocol.SkillContent, error) {
	if params.File != "" && params.File != "SKILL.md" {
		return protocol.SkillContent{}, notFound("skill file")
	}
	name := strings.TrimPrefix(params.ID, protocol.ManagedSkillSet+"/")
	for _, skill := range s.data.learning.skills {
		if skill.Name == name {
			text := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s.\nRead the source before changing anything.\nMake the smallest safe change.\nCheck the result before reporting it.\n", skill.Name, skill.Description, skill.Name, skill.Description)
			return protocol.SkillContent{ID: params.ID, File: "SKILL.md", Path: skill.Path, Text: text}, nil
		}
	}
	return protocol.SkillContent{}, notFound("skill")
}
