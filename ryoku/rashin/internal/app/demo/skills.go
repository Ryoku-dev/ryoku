package demo

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the demo's built-in sets: the four real ones with a few of their real skills each, and the openspec command missing
type demoSkillSet struct {
	set    protocol.SkillSet
	skills []demoSkill
}

type demoSkill struct {
	name, description string
	manual            bool
	// what SKILL.md says under its frontmatter, cut short
	body string
}

func demoSkillsRoot() string {
	return filepath.Join(paths.DataHome(), "ryoku", "rashin", "skills", "3c9e41a7d0b2")
}

var demoSkillSets = []demoSkillSet{
	{
		set: protocol.SkillSet{
			ID: "superpowers", Name: "Superpowers", By: "Jesse Vincent", URL: "https://github.com/obra/superpowers",
			License: "MIT", Commit: "22f0b463e7a5",
			Summary: "A way of working for coding agents: design before building, write the test first, find the root cause, and prove the work before calling it done.",
		},
		skills: []demoSkill{
			{
				name:        "brainstorming",
				description: "You MUST use this before any creative work - creating features, building components, adding functionality, modifying behavior, or planning anything new, in software or out of it (a talk, a business, a renovation).",
				body: "# Brainstorming\n\nYou are very good at building. You build what you believe your human\npartner wants, and that belief is usually thinner than it feels. This\nskill is for finding out what they actually want, and why, before\nanything gets built.\n\n## First: What Do You Actually Know?\n\n" +
					"- **Everything that matters is in the request** (\"make the icon cornflower\n  blue\"). This is a quick, clear task. Do it, and say what you did.\n- **Something non-trivial is missing.** Start the conversation.\n\nBefore you start implementing, you must, in order:\n\n1. Lay out the approaches in a message of their own, and have your\n   human partner pick one.\n2. Write the design doc, short or full.\n3. Get their approval of that doc.\n4. Invoke the `superpowers:writing-plans` skill.\n",
			},
			{
				name:        "systematic-debugging",
				description: "Use when encountering any bug, test failure, or unexpected behavior, before proposing fixes",
				body:        "# Systematic Debugging\n\n## Overview\n\n**Core principle:** ALWAYS find root cause before attempting fixes. Symptom fixes are failure.\n\n## The Iron Law\n\n```\nNO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST\n```\n\nIf you haven't completed Phase 1, you cannot propose fixes.\n\n## When to Use\n\nUse for ANY technical issue:\n- Test failures\n- Bugs in production\n- Unexpected behavior\n- Build failures\n",
			},
			{
				name:        "test-driven-development",
				description: "Use when implementing any feature or bugfix, before writing implementation code",
				body:        "# Test-Driven Development (TDD)\n\n## Overview\n\nWrite the test first. Watch it fail. Write minimal code to pass.\n\n**Core principle:** If you didn't watch the test fail, you don't know if it tests the right thing.\n\n## The Iron Law\n\n```\nNO PRODUCTION CODE WITHOUT A FAILING TEST FIRST\n```\n\nWrite code before the test? Delete it. Start over.\n",
			},
			{
				name:        "verification-before-completion",
				description: "Use when about to claim work is complete, fixed, or passing, before committing or creating PRs - requires running verification commands and confirming output before making any success claims; evidence before assertions always",
				body:        "# Verification Before Completion\n\n## Overview\n\n**Core principle:** Evidence before claims, always.\n\n## The Gate Function\n\n1. IDENTIFY: What command proves this claim?\n2. RUN: Execute the FULL command (fresh, complete)\n3. READ: Full output, check exit code, count failures\n4. VERIFY: Does output confirm the claim?\n",
			},
			{
				name:        "writing-plans",
				description: "Use when you have a spec or requirements for a multi-step task, before touching code",
				body:        "# Writing Plans\n\n## Overview\n\nWrite implementation plans for an engineer who has not seen this codebase or this spec. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent commits.\n\n**Save plans to:** `docs/superpowers/plans/YYYY-MM-DD-<feature-name>.md`\n",
			},
		},
	},
	{
		set: protocol.SkillSet{
			ID: "openspec", Name: "OpenSpec", By: "Fission AI", URL: "https://github.com/Fission-AI/OpenSpec",
			License: "MIT", Commit: "d12c28177767",
			Summary: "Plans each change in the project as a proposal, specs, a design and tasks, then builds it and archives it, through the openspec command.",
			Needs:   []protocol.SkillNeed{{Command: "openspec", Install: "npm install -g @fission-ai/openspec"}},
		},
		skills: []demoSkill{
			{
				name:        "openspec-apply-change",
				description: "Implement tasks from an OpenSpec change. Use when the user wants to start implementing, continue implementation, or work through tasks. Also use when the user says \"openspec apply\", \"opsx apply\", or \"openspec implement\".",
				body:        "Implement tasks from an OpenSpec change.\n\n**Project check:** These steps expect a project that already uses OpenSpec.\n\n1. Run `openspec status --change \"<name>\" --json`.\n2. Read the tasks and work through them in order.\n3. Mark each task done as it lands.\n",
			},
			{
				name:        "openspec-archive-change",
				description: "Archive a completed OpenSpec change in the experimental workflow. Use when the user wants to finalize and archive a change after implementation is complete. Also use when the user says \"openspec archive\" or \"opsx archive\".",
				body:        "Archive a completed change in the experimental workflow.\n\nRun `openspec archive \"<name>\"` once every task is checked, then show the user what moved into the main specs.\n",
			},
			{
				name:        "openspec-explore",
				description: "Enter OpenSpec explore mode - a thinking partner for exploring ideas, investigating problems, and clarifying requirements in a project that uses OpenSpec. Use when the user wants to think through something before or during an OpenSpec change. Also use when the user says \"openspec explore\" or \"opsx explore\".",
				body:        "Enter explore mode. Think deeply. Visualize freely. Follow the conversation wherever it goes.\n\n**IMPORTANT: Explore mode is for thinking, not implementing.** You may read files, search code and run read-only commands, but you must never write code or implement features here.\n",
			},
			{
				name:        "openspec-propose",
				description: "Propose a new OpenSpec change with all artifacts generated in one step. Use when the user wants to quickly describe what they want to build and get a complete proposal with design, specs, and tasks ready for implementation. Also use when the user says \"openspec propose\" or \"opsx propose\".",
				body:        "Propose a new change - create the change and generate all artifacts in one step.\n\nWith the default spec-driven schema that is:\n- proposal.md (what & why)\n- `specs/<capability-path>/spec.md` (what the system must do)\n- design.md (how)\n- tasks.md (implementation steps)\n",
			},
		},
	},
	{
		set: protocol.SkillSet{
			ID: "speckit", Name: "Spec Kit", By: "GitHub", URL: "https://github.com/github/spec-kit",
			License: "MIT", Commit: "979b0a3c1277",
			Summary: "Spec-driven development: a constitution, a specification, a plan and tasks, each written into the project before the code that follows them.",
			Needs:   []protocol.SkillNeed{{Command: "bash", Present: true}, {Command: "git", Present: true}},
		},
		skills: []demoSkill{
			{
				name:        "speckit-constitution",
				description: "Create or update the project constitution from interactive or provided principle inputs.",
				body:        "## User Input\n\n```text\n$ARGUMENTS\n```\n\nYou are updating the project constitution at `.specify/memory/constitution.md`. Fill every placeholder with the principles the user gives, and keep the version line honest.\n",
			},
			{
				name:        "speckit-specify",
				description: "Create or update the feature specification from a natural language feature description.",
				body:        "## User Input\n\n```text\n$ARGUMENTS\n```\n\nThe text the user typed after the command **is** the feature description. Write the specification from it, focusing on **what** users need and **why**, never on how it is built.\n",
			},
			{
				name:        "speckit-plan",
				description: "Execute the implementation planning workflow using the plan template to generate design artifacts.",
				body:        "## Outline\n\n1. Run `.specify/scripts/bash/setup-plan.sh --json` from the repo root.\n2. Read the feature spec and the constitution.\n3. Fill the plan template: technical context, constitution check, research, data model, contracts.\n",
			},
			{
				name:        "speckit-tasks",
				description: "Generate an actionable, dependency-ordered tasks.md for the feature based on available design artifacts.",
				body:        "## Outline\n\nGenerate `tasks.md` from the plan, ordered by dependency. Mark tasks that can run in parallel with `[P]`, and give each one the exact file it touches.\n",
			},
			{
				name:        "speckit-implement",
				description: "Execute the implementation plan by processing and executing all tasks defined in tasks.md",
				body:        "## Outline\n\n1. Run `.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks`.\n2. Work through `tasks.md` phase by phase, tests before code.\n3. Mark each finished task `[X]` in the file.\n",
			},
		},
	},
	{
		set: protocol.SkillSet{
			ID: "egm", Name: "Elephant and Goldfish", By: "Google", URL: "https://github.com/google/elephant-goldfish-model",
			License: "Apache-2.0", Commit: "84145300b9e0",
			Summary: "Design with an elephant, review with a goldfish: one agent drafts with the whole context, reviewers read the draft fresh. Each starts when you ask for it.",
		},
		skills: []demoSkill{
			{
				name: "egm-brainstorm", manual: true,
				description: "Interactive ideation and brainstorming skill for the Elephant & Goldfish Model (EGM). Helps turn half-baked concepts into structured hypotheses via clarifying questions, review, and iterative refinement. Produces an RFC summary for human review or more detailed design with egm-draft.",
				body:        "# EGM Brainstorm: Interactive Multi-Lens Ideation & Refinement\n\n## Overview\n\nThis skill acts as the upstream conversational brainstorming engine for the\nElephant-Goldfish Model (EGM). It turns \"half-baked\" or raw concepts into\nhypotheses by combining **interactive clarifying questions** with an automated\n**Goldfish review**.\n",
			},
			{
				name: "egm-draft", manual: true,
				description: "Drafts a design document section-by-section using the concepts brief from egm-brainstorm (or user notes). This is the second step in the Elephant-Goldfish (EGM) feature flow. Use when you are ready to write a design doc after aligning on requirements + solution.",
				body:        "# Design Drafting (Document Construction)\n\nThis skill guides the user through drafting a technical design document based on\nthe aligned problem and requirements from the brainstorm phase.\n\n1. Write the draft to `design_draft.md` in the working directory.\n2. Tell the user the path and ask for review.\n",
			},
			{
				name: "egm-resolve", manual: true,
				description: "The Elephant resolution skill. Ingests flat deficiency reports from concurrent Goldfish reviewers, applies automated fixes for PROPOSE FIX items via UI multiple-choice gates, and presents interactive negotiation/re-sync checkpoints.",
				body:        "# Goldfish Review: Resolve\n\n## Overview\n\nThis skill instructs the agent on how to operate within the primary Elephant\ncontext window to parse flat deficiency reports from concurrent Goldfish\nreviewers. It defends accurate design choices, applies automated textual\nrepairs, and manages interactive negotiation checkpoints.\n",
			},
			{
				name: "egm-review", manual: true,
				description: "Orchestrates the EGM Goldfish Review Protocol. Used as part of the EGM process to perform design document reviews. Executes five review phases: Initial Zoom Out Review, Comprehension Review, Design Critique, Intermediate Zoom Out Review, and Implementation Readiness.",
				body:        "# Review, Validation & Automated Remediation\n\nReviews and validates a design document through specialized reviewer personas.\n\n## Delegation\n\nA Goldfish runs in a fresh context with no conversation history. Spawn each\nsubagent with your harness's subagent tool and wait for its report.\n",
			},
		},
	},
}

func (s *server) callSkills(method string, raw json.RawMessage) (any, error) {
	switch method {
	case protocol.MSkillsList:
		return s.skillsList(), nil
	case protocol.MSkillsGet:
		var params protocol.SkillParams
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		return skillContent(params)
	}
	return nil, protocol.Errorf(protocol.CodeUnsupported, "demo method %s is not implemented", method)
}

func (s *server) skillsList() protocol.SkillsResult {
	root := demoSkillsRoot()
	result := protocol.SkillsResult{Root: root}
	for _, fixture := range demoSkillSets {
		set := fixture.set
		set.On = !slices.Contains(s.data.snapshot.Settings.Skills.Off, set.ID)
		set.Skills = make([]protocol.Skill, 0, len(fixture.skills))
		for _, skill := range fixture.skills {
			set.Skills = append(set.Skills, protocol.Skill{
				ID: set.ID + "/" + skill.name, Name: skill.name, Set: set.ID, Description: skill.description,
				Path: filepath.Join(root, set.ID, "skills", skill.name, "SKILL.md"), Manual: skill.manual,
			})
		}
		result.Sets = append(result.Sets, set)
	}
	return result
}

// SKILL.md as the daemon hands it over, frontmatter and all
func skillContent(params protocol.SkillParams) (protocol.SkillContent, error) {
	setID, name, _ := strings.Cut(params.ID, "/")
	if params.File != "" && params.File != "SKILL.md" {
		return protocol.SkillContent{}, notFound("skill file")
	}
	for _, fixture := range demoSkillSets {
		if fixture.set.ID != setID {
			continue
		}
		for _, skill := range fixture.skills {
			if skill.name != name {
				continue
			}
			front := "---\nname: " + skill.name + "\ndescription: " + strings.ReplaceAll(skill.description, "\n", " ") + "\n"
			if skill.manual {
				front += "disable-model-invocation: true\n"
			}
			return protocol.SkillContent{
				ID: params.ID, File: "SKILL.md", Path: filepath.Join(demoSkillsRoot(), setID, "skills", name, "SKILL.md"),
				Text: front + "---\n\n" + skill.body,
			}, nil
		}
	}
	return protocol.SkillContent{}, notFound("skill")
}
