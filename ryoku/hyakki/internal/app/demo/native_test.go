package demo

import (
	"encoding/json"
	"slices"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestDemoModelsUseNativeIDsAndOfferLocalFirst(t *testing.T) {
	fx := makeFixtures()
	if got := fx.snapshot.Providers[0]; got.ID != protocol.ProviderRashinModels || !got.OnDemand || got.Running {
		t.Fatalf("first provider = %+v", got)
	}
	if got := fx.snapshot.Local.Models[0]; got.ID != "lfm3-omni-2b" || got.State != protocol.LocalInstalled || !got.Tools || !got.Vision || !got.Audio {
		t.Fatalf("managed model = %+v", got)
	}
	for _, info := range fx.snapshot.Sessions {
		if info.Kind == protocol.KindAgent && !protocol.ValidModel(info.Model) {
			t.Errorf("agent %s model %q", info.ID, info.Model)
		}
		if events := fx.events[info.ID]; len(events) > 0 && info.Kind == protocol.KindAgent && info.LastSeq != events[len(events)-1].Seq {
			t.Errorf("agent %s last sequence %d, transcript ends at %d", info.ID, info.LastSeq, events[len(events)-1].Seq)
		}
	}
	for _, team := range fx.teams.teams {
		for _, member := range team.Members {
			if !protocol.ValidModel(member.Model) {
				t.Errorf("team %s member %s model %q", team.ID, member.ID, member.Model)
			}
		}
	}
	for _, task := range fx.teams.tasks {
		if !protocol.ValidModel(task.Model) {
			t.Errorf("team task %s model %q", task.ID, task.Model)
		}
	}
	for _, tasks := range fx.tasks {
		for _, task := range tasks.Tasks {
			if !protocol.ValidModel(task.Model) {
				t.Errorf("work task %s model %q", task.ID, task.Model)
			}
		}
	}
}

func TestDemoLocalCancelKeepsPartialDownloadAndBroadcasts(t *testing.T) {
	s := &server{data: makeFixtures()}
	initial := s.data.snapshot.Local.Models[2].Files[0].Done
	call(t, s, protocol.MLocalDownload, protocol.LocalModelParams{ID: "gemma-3-4b-it"})
	call(t, s, protocol.MLocalCancel, protocol.LocalModelParams{ID: "gemma-3-4b-it"})
	model := s.data.snapshot.Local.Models[2]
	if model.State != protocol.LocalPaused || model.Files[0].Done != initial {
		t.Fatalf("cancel lost the partial download: %+v", model)
	}
	for _, event := range s.takeEvents() {
		if event.Event != protocol.EvtLocalChanged {
			t.Fatalf("unexpected local event %q", event.Event)
		}
	}
	probe := call(t, s, protocol.MModelsProbe, protocol.ModelProbeParams{Model: demoLocalID}).(protocol.ModelProbeResult)
	if !probe.OK || probe.Model.ID != demoLocalID || len(probe.Options) != 1 || probe.Options[0].ID != protocol.OptionPlan {
		t.Fatalf("managed model probe = %+v", probe)
	}
	paid := call(t, s, protocol.MModelsProbe, protocol.ModelProbeParams{Model: demoPaidID}).(protocol.ModelProbeResult)
	if !paid.OK || len(paid.Options) != 2 || paid.Options[1].ID != protocol.OptionEffort {
		t.Fatalf("pinned paid model probe = %+v", paid)
	}
	call(t, s, protocol.MLocalRemove, protocol.LocalModelParams{ID: "lfm3-omni-2b"})
	models := call(t, s, protocol.MModelsList, protocol.ModelsParams{}).(protocol.ModelsResult)
	if slices.ContainsFunc(models.Models, func(model protocol.ModelInfo) bool { return model.ID == demoLocalID }) {
		t.Fatal("removed model remains available to the model picker")
	}
	if got := call(t, s, protocol.MModelsProbe, protocol.ModelProbeParams{Model: demoLocalID}).(protocol.ModelProbeResult); got.OK {
		t.Fatal("removed model still passes the agent probe")
	}
}

func TestDemoStopBroadcastsStoppingBeforeStopped(t *testing.T) {
	s := &server{data: makeFixtures()}
	call(t, s, protocol.MLocalStart, protocol.LocalModelParams{ID: "lfm3-omni-2b"})
	s.takeEvents()
	runtime := call(t, s, protocol.MLocalStop, protocol.LocalModelParams{}).(protocol.LocalRuntime)
	if runtime.State != protocol.RuntimeStopped || len(runtime.Loaded) != 0 {
		t.Fatalf("stop result = %+v", runtime)
	}
	var states []string
	for _, event := range s.takeEvents() {
		if event.Event != protocol.EvtLocalChanged {
			continue
		}
		var result protocol.LocalResult
		if err := json.Unmarshal(event.Data, &result); err != nil {
			t.Fatal(err)
		}
		states = append(states, result.Runtime.State)
	}
	if len(states) != 2 || states[0] != protocol.RuntimeStopping || states[1] != protocol.RuntimeStopped {
		t.Fatalf("stop states = %v", states)
	}
}

func TestDemoLearningListsAndRemovesWithFeaturesOff(t *testing.T) {
	s := &server{data: makeFixtures()}
	rules := call(t, s, protocol.MLearningRules, protocol.LearningRulesParams{}).(protocol.LearningRulesResult)
	if len(rules.Rules) != 3 || rules.Rules[0].Name != "no-em-dash" {
		t.Fatalf("project rule missing from Learning: %+v", rules.Rules)
	}
	grievances := call(t, s, protocol.MLearningGrievances, protocol.LearningGrievancesParams{}).(protocol.LearningGrievancesResult)
	if grievances.Total != 4 || len(grievances.Tools) != 3 || grievances.Tools[0].Count != 2 {
		t.Fatalf("grievance counts = %+v", grievances)
	}
	call(t, s, protocol.MLearningSkillDelete, protocol.LearningSkillParams{Name: "review"})
	call(t, s, protocol.MLearningGrievancesClear, protocol.GrievancesClearParams{IDs: []int64{4}})
	if skills := call(t, s, protocol.MLearningSkills, protocol.Empty{}).(protocol.LearningSkillsResult); slices.ContainsFunc(skills.Skills, func(skill protocol.ManagedSkill) bool { return skill.Name == "review" }) {
		t.Fatal("removed skill remains")
	}
	if remaining := call(t, s, protocol.MLearningGrievances, protocol.LearningGrievancesParams{}).(protocol.LearningGrievancesResult); remaining.Total != 3 {
		t.Fatalf("cleared grievances total = %d", remaining.Total)
	}
	queued := s.takeEvents()
	if len(queued) != 2 {
		t.Fatalf("learning change events = %d", len(queued))
	}
	for _, event := range queued {
		if event.Event != protocol.EvtLearningChanged {
			t.Fatalf("learning mutation broadcast %q", event.Event)
		}
	}
	params, _ := json.Marshal(protocol.RuleForgeParams{Session: "agent-done", Complaint: "No em dash"})
	if _, err := s.call(protocol.MLearningRuleForge, params); err == nil || protocol.AsError(err).Code != protocol.CodeDisabled {
		t.Fatalf("forge without opt-in error = %v", err)
	}
}

func TestDemoNativeAgentTranscriptShowsPlanTaskQuestionAndCompaction(t *testing.T) {
	fx := makeFixtures()
	var plan, agents, question, notice bool
	for _, events := range fx.events {
		for _, event := range events {
			switch event.Kind {
			case protocol.EvPlan:
				plan = true
			case protocol.EvTool:
				var tool protocol.ToolData
				if json.Unmarshal(event.Data, &tool) == nil && tool.Title == "task" && len(tool.Agents) > 0 && protocol.ValidModel(tool.Agents[0].Model) {
					agents = true
				}
			case protocol.EvPermission:
				var card protocol.PermissionData
				if json.Unmarshal(event.Data, &card) == nil && card.Kind == protocol.PermissionQuestion && len(card.Options) == 2 {
					question = true
				}
			case protocol.EvNotice:
				var note protocol.NoticeData
				if json.Unmarshal(event.Data, &note) == nil && note.Text != "" {
					notice = true
				}
			}
		}
	}
	if !plan || !agents || !question || !notice {
		t.Fatalf("missing native transcript: plan=%t task=%t question=%t compaction=%t", plan, agents, question, notice)
	}
}
