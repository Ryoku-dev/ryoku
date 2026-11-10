package harness

import (
	"context"
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// fakeConfigOptions is an agent's option list in the shapes agents send: a
// mode it lists nowhere else, models under a named group, a plain select and
// an option that is not a select at all.
func fakeConfigOptions(chosen map[string]string) []map[string]any {
	return []map[string]any{
		{"id": "mode", "name": "Mode", "category": "mode", "type": "select", "currentValue": chosen["mode"],
			"options": []map[string]string{{"value": "default", "name": "Default"}, {"value": "plan", "name": "Plan", "description": "Plans before it edits"}}},
		{"id": "model", "name": "Model", "category": "model", "type": "select", "currentValue": chosen["model"],
			"options": []map[string]any{{"group": "main", "name": "Main", "options": []map[string]string{
				{"value": "big", "name": "Big", "description": "big"}, {"value": "small", "name": "Small"},
			}}}},
		{"id": "thinking", "name": "Thinking", "category": "thought_level", "type": "select", "currentValue": chosen["thinking"],
			"options": []map[string]string{{"value": "auto", "name": "Auto"}, {"value": "high", "name": "High"}}},
		{"id": "sandbox", "name": "Sandbox", "type": "boolean", "currentValue": true},
	}
}

// Starting a session with no prompt is enough to learn what the agent will
// use: its model by name, its modes, and every other choice it offers.
func TestStartingAnAgentReportsItsModelModesAndOptions(t *testing.T) {
	r, sink := helperRunner(t, "options", protocol.ApprovalsAsk, nil)
	var started []protocol.HarnessFact
	r.reportCapabilities = func(capabilities protocol.HarnessCapabilities) { started = capabilities.Started }
	if err := r.Start(context.Background(), sink); err != nil {
		t.Fatal(err)
	}
	model := lastOf[protocol.ChoiceData](t, sink, protocol.EvModel)
	wantModels := []protocol.Choice{{ID: "big", Name: "Big"}, {ID: "small", Name: "Small"}}
	if model.Current != "big" || !reflect.DeepEqual(model.Options, wantModels) {
		t.Fatalf("the model reads %#v", model)
	}
	mode := lastOf[protocol.ChoiceData](t, sink, protocol.EvMode)
	if mode.Current != "default" || len(mode.Options) != 2 || mode.Options[1].Description != "Plans before it edits" {
		t.Fatalf("the mode reads %#v", mode)
	}
	options := lastOf[protocol.OptionsData](t, sink, protocol.EvOptions).Options
	want := []protocol.SessionOption{{
		ID: "thinking", Name: "Thinking", Category: "thought_level", Current: "auto",
		Choices: []protocol.Choice{{ID: "auto", Name: "Auto"}, {ID: "high", Name: "High"}},
	}}
	if !reflect.DeepEqual(options, want) {
		t.Fatalf("the options read %#v", options)
	}
	if sink.info.Model != "big" || sink.info.Mode != "default" || !sink.info.Alive {
		t.Fatalf("the session reads %#v", sink.info)
	}
	wantStarted := []protocol.HarnessFact{{Label: "Model", Value: "Big"}, {Label: "Thinking", Value: "Auto"}}
	if !reflect.DeepEqual(started, wantStarted) {
		t.Fatalf("what it started with reads %#v", started)
	}
}

// Setting an option goes to the agent and comes back as the new state, said
// once: the model list is not sent again because another option changed.
func TestSettingAnOptionChangesItAndRepeatsNothingElse(t *testing.T) {
	r, sink := helperRunner(t, "options", protocol.ApprovalsAsk, nil)
	ctx := context.Background()
	if err := r.Start(ctx, sink); err != nil {
		t.Fatal(err)
	}
	if err := r.SetOption(ctx, "thinking", "high", sink); err != nil {
		t.Fatal(err)
	}
	if options := lastOf[protocol.OptionsData](t, sink, protocol.EvOptions).Options; options[0].Current != "high" {
		t.Fatalf("thinking reads %q", options[0].Current)
	}
	if models := countOf(sink, protocol.EvModel); models != 1 {
		t.Fatalf("the model list was reported %d times, want once", models)
	}
	if err := r.SetOption(ctx, "sandbox", "off", sink); err == nil {
		t.Fatal("an option the agent does not offer as a choice was set")
	}
	if err := r.SetMode(ctx, "plan", sink); err != nil {
		t.Fatal(err)
	}
	if mode := lastOf[protocol.ChoiceData](t, sink, protocol.EvMode); mode.Current != "plan" || sink.info.Mode != "plan" {
		t.Fatalf("the mode reads %#v, the session %q", mode, sink.info.Mode)
	}
	if err := r.SetModel(ctx, "small", sink); err != nil {
		t.Fatal(err)
	}
	if models := countOf(sink, protocol.EvModel); models != 2 || sink.info.Model != "small" {
		t.Fatalf("after one model change the list was reported %d times and the session reads %q", models, sink.info.Model)
	}
}

// An agent may answer a set with nothing. The value is then taken as set.
func TestAnAgentThatAnswersASetWithNothingIsTakenAtItsWord(t *testing.T) {
	r, sink := helperRunner(t, "options-silent", protocol.ApprovalsAsk, nil)
	ctx := context.Background()
	if err := r.Start(ctx, sink); err != nil {
		t.Fatal(err)
	}
	if err := r.SetOption(ctx, "thinking", "high", sink); err != nil {
		t.Fatal(err)
	}
	if options := lastOf[protocol.OptionsData](t, sink, protocol.EvOptions).Options; len(options) != 1 || options[0].Current != "high" {
		t.Fatalf("the options read %#v", options)
	}
	if err := r.SetMode(ctx, "plan", sink); err != nil {
		t.Fatal(err)
	}
	if mode := lastOf[protocol.ChoiceData](t, sink, protocol.EvMode); mode.Current != "plan" {
		t.Fatalf("the mode reads %#v", mode)
	}
}
