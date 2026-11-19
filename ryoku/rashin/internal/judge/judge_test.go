package judge

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"ryoku-rashin/internal/judge/question"
	"ryoku-rashin/internal/llm"
)

type stubClient struct {
	requests []llm.Request
	replies  []string
}

func (c *stubClient) Stream(_ context.Context, r llm.Request) (llm.Stream, error) {
	c.requests = append(c.requests, r)
	reply := ""
	if len(c.replies) > 0 {
		reply = c.replies[0]
		c.replies = c.replies[1:]
	}
	return &stubStream{reply: reply}, nil
}

type stubStream struct {
	reply string
	done  bool
}

func (s *stubStream) Next() (llm.Delta, error) {
	if s.done {
		return llm.Delta{}, io.EOF
	}
	s.done = true
	return llm.Delta{Text: s.reply}, nil
}
func (*stubStream) Final() llm.Final { return llm.Final{} }
func (*stubStream) Close() error     { return nil }

func TestJudgeConstrainedSchemaRetryAndCache(t *testing.T) {
	c := &stubClient{replies: []string{`{"answer":"maybe"}`, `{"answer":"YES"}`}}
	j := New(c, nil)
	j.cache = newCache(t.TempDir() + "/answers.db")
	defer j.Close()
	got, err := j.Ask(context.Background(), "Does this happen?", question.YesNo)
	if err != nil {
		t.Fatal(err)
	}
	if answer := question.Answer(got.JSON); answer != question.Yes {
		t.Fatalf("answer %q", answer)
	}
	if len(c.requests) != 2 || !c.requests[0].Light || len(c.requests[0].Extra) == 0 {
		t.Fatalf("judge request not constrained: %#v", c.requests)
	}
	var body map[string]any
	if err := json.Unmarshal(c.requests[0].Extra["response_format"], &body); err != nil || body["type"] != "json_schema" {
		t.Fatalf("response_format: %#v %v", body, err)
	}
	again, err := j.Ask(context.Background(), "Does this happen?", question.YesNo)
	if err != nil || question.Answer(again.JSON) != question.Yes || len(c.requests) != 2 {
		t.Fatalf("cache missed: %#v, %v, calls=%d", again, err, len(c.requests))
	}
}

func TestJudgeRejectsInvalidSchemaAnswer(t *testing.T) {
	c := &stubClient{replies: []string{`{"answer":false}`, `not json`}}
	j := New(c, nil)
	j.cache = newCache(t.TempDir() + "/answers.db")
	defer j.Close()
	if _, err := j.Ask(context.Background(), "Does this happen?", question.YesNo); err == nil {
		t.Fatal("judge accepted invalid answer")
	}
}
