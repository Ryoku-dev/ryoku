package protocol

import (
	"encoding/json"
	"math"
	"testing"
)

func decisionFixture(t *testing.T) (DecisionSpec, DecisionResponse) {
	t.Helper()
	request := DecisionRequest{
		Model: "fixture-local", State: json.RawMessage(`"A selected fixture window"`),
		Questions: map[string]DecisionQuestion{
			"control": {Type: "choice", Criteria: json.RawMessage(`{"read":"Read the selection","stop":"Do not proceed"}`)},
			"urgency": {Type: "score", Criteria: json.RawMessage(`["Can wait","Now"]`)},
			"safe":    {Type: "noul"},
		},
	}
	spec, err := CompileDecisionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	response := DecisionResponse{Model: "fixture-local:resolved", Usage: DecisionUsage{Input: 43}, Answers: map[string]DecisionAnswer{
		"control": {Type: "choice", Choice: "read", Confidence: new(0.8), Probabilities: map[string]float64{"read": 0.9, "stop": 0.1}},
		"urgency": {Type: "score", Score: new(0.75), Confidence: new(0.5), Probabilities: map[string]float64{"0": 0.25, "1": 0.75}, Legend: map[string]string{"0": "Can wait", "1": "Now"}},
		"safe":    {Type: "noul", Noul: new(0.95)},
	}}
	return spec, response
}

func TestDecisionRejectsAnswersOutsideTheReviewedQuestions(t *testing.T) {
	for name, mutate := range map[string]func(*DecisionResponse){
		"another question":     func(r *DecisionResponse) { r.Answers["unrequested"] = r.Answers["control"]; delete(r.Answers, "safe") },
		"invented option":      func(r *DecisionResponse) { a := r.Answers["control"]; a.Choice = "delete"; r.Answers["control"] = a },
		"changed type":         func(r *DecisionResponse) { a := r.Answers["control"]; a.Type = "noul"; r.Answers["control"] = a },
		"missing distribution": func(r *DecisionResponse) { delete(r.Answers["control"].Probabilities, "stop") },
		"inconsistent confidence": func(r *DecisionResponse) {
			a := r.Answers["control"]
			a.Confidence = new(float64(1))
			r.Answers["control"] = a
		},
		"nonwinning choice":      func(r *DecisionResponse) { a := r.Answers["control"]; a.Choice = "stop"; r.Answers["control"] = a },
		"changed level":          func(r *DecisionResponse) { r.Answers["urgency"].Legend["1"] = "Delete immediately" },
		"inconsistent score":     func(r *DecisionResponse) { a := r.Answers["urgency"]; a.Score = new(0.1); r.Answers["urgency"] = a },
		"missing truth":          func(r *DecisionResponse) { a := r.Answers["safe"]; a.Noul = nil; r.Answers["safe"] = a },
		"nonfinite truth":        func(r *DecisionResponse) { a := r.Answers["safe"]; a.Noul = new(math.NaN()); r.Answers["safe"] = a },
		"generated output":       func(r *DecisionResponse) { r.Usage.Output = 1 },
		"missing measured input": func(r *DecisionResponse) { r.Usage.Input = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			spec, response := decisionFixture(t)
			if err := spec.Validate(response); err != nil {
				t.Fatalf("valid decision refused: %v", err)
			}
			mutate(&response)
			if err := spec.Validate(response); err == nil {
				t.Fatal("an unrequested or inconsistent decision was accepted")
			}
		})
	}
}

func TestDecisionRequiresFiniteTypedQuestions(t *testing.T) {
	for _, criteria := range []string{`["same","same"]`, `{"only":"one"}`, `{"a":3,"b":"text"}`, `{"a":"yes","b":"no"}`} {
		request := DecisionRequest{Model: "fixture", State: json.RawMessage(`"state"`), Questions: map[string]DecisionQuestion{"question": {Type: "choice", Criteria: json.RawMessage(criteria)}}}
		_, err := CompileDecisionRequest(request)
		valid := criteria == `{"a":"yes","b":"no"}`
		if (err == nil) != valid {
			t.Fatalf("criteria %s: %v", criteria, err)
		}
	}
	var empty DecisionSpec
	if err := empty.Validate(DecisionResponse{Model: "model", Usage: DecisionUsage{Input: 1}}); err == nil {
		t.Fatal("an uncompiled empty specification accepted an answer")
	}
}

func TestDecisionLocalRouteCannotReachLANOrAChangingHostname(t *testing.T) {
	for _, raw := range []string{
		"https://public.example/v1", "http://192.168.1.2:11435", "http://10.0.0.2:11435", "http://[fd00::1]:11435",
		"http://user:pass@127.0.0.1:11435", "http://127.0.0.1:11435/v1?next=remote", "http://127.0.0.1:11435/#route", "file:///tmp/model", "http://127.0.0.1:0",
	} {
		if address, ok := ParseDecisionAddress(raw); ok {
			t.Fatalf("unsafe local address %q admitted as %q", raw, address)
		}
	}
	for raw, want := range map[string]string{"http://localhost:11435": "http://127.0.0.1:11435/v1", "http://[::1]:11435/v1/": "http://[::1]:11435/v1"} {
		if address, ok := ParseDecisionAddress(raw); !ok || address != want {
			t.Fatalf("%q => %q, %v", raw, address, ok)
		}
	}
}
