package askuser

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine/tool"
)

// picker answers every question card with one option ID.
type picker struct {
	pick string
	got  tool.Question
}

func (p *picker) Allow(context.Context, tool.Request) (tool.Decision, error) {
	return tool.Decision{Allowed: true}, nil
}

func (p *picker) Choose(_ context.Context, q tool.Question) (string, error) {
	p.got = q
	return p.pick, nil
}

func TestQuestionCardCarriesOptionsAndRecommendation(t *testing.T) {
	p := &picker{pick: "2"}
	raw := `{"question":"Which store?","options":["SQLite","Redis",{"label":"Files"},"sqlite"],"recommended":"redis"}`
	res := askTool{}.Run(context.Background(), tool.Call{ID: "c", Args: json.RawMessage(raw)}, tool.Env{Permissions: p})
	if res.IsError {
		t.Fatalf("failed: %s", res.Text)
	}
	if len(p.got.Options) != 3 || p.got.Options[2].Name != "Files" || p.got.Recommended != "2" || p.got.Call.ID != "c" {
		t.Fatalf("card %+v", p.got)
	}
	if !strings.Contains(res.Text, "option 2: Redis") || res.Shown != "Redis" {
		t.Fatalf("answer %+v", res)
	}
}

func TestRecommendationReadsNumbersAndText(t *testing.T) {
	options := []string{"a", "b", "c"}
	for raw, want := range map[string]int{`3`: 3, `"1"`: 1, `"B"`: 2, `7`: 0, `"z"`: 0, ``: 0} {
		if got := recommendation(json.RawMessage(raw), options); got != want {
			t.Fatalf("%s: %d, want %d", raw, got, want)
		}
	}
}

func TestQuestionsOutsideTheCardLimitsAreRefused(t *testing.T) {
	for _, raw := range []string{
		`{"question":"?","options":["only one"]}`,
		`{"question":"?","options":["a","b","c","d","e","f","g"]}`,
		`{"options":["a","b"]}`,
	} {
		p := &picker{}
		res := askTool{}.Run(context.Background(), tool.Call{Args: json.RawMessage(raw)}, tool.Env{Permissions: p})
		if !res.IsError || p.got.Question != "" {
			t.Fatalf("%s asked: %+v", raw, res)
		}
	}
}

func TestDismissedCardTellsTheModelToGoOn(t *testing.T) {
	p := &picker{pick: ""}
	res := askTool{}.Run(context.Background(), tool.Call{Args: json.RawMessage(`{"question":"?","options":["a","b"]}`)}, tool.Env{Permissions: p})
	if res.IsError || !strings.Contains(res.Text, "dismissed") {
		t.Fatalf("%+v", res)
	}
}
