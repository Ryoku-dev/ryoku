package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"ryoku-rashin/internal/llm"
)

// errNoJSON is a reply that held no object of the expected shape.
var errNoJSON = errors.New("memory: the model's reply was not the expected JSON")

// learnReply is what the learner asks for. Events are the model's list of
// dated and counted things the user did, written before it decides what to
// keep; they are never stored.
type learnReply struct {
	Summary string       `json:"summary"`
	Events  []learnEvent `json:"events"`
	Ops     []learnOp    `json:"ops"`
}

// learnEvent is one thing the user did or will do, and when.
type learnEvent struct {
	What string `json:"what"`
	When string `json:"when"`
}

// The most events a reply lists, and the longest one may be; the prompts
// say the same numbers.
const (
	learnEventsMax    = 20
	learnEventCharMax = 160
)

// learnOp is one operation the learner proposes.
type learnOp struct {
	Op         string `json:"op"`
	Scope      string `json:"scope"`
	Text       string `json:"text"`
	Kind       string `json:"kind"`
	Explicit   bool   `json:"explicit"`
	Sensitive  bool   `json:"sensitive"`
	Supersedes ids    `json:"supersedes"`
	Evidence   string `json:"evidence"`
}

// ids accepts the ids a model lists as strings or as bare numbers, which
// small models write for "0".."n-1".
type ids []string

func (i *ids) UnmarshalJSON(raw []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return err
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		var text string
		if json.Unmarshal(value, &text) == nil {
			out = append(out, strings.TrimSpace(text))
			continue
		}
		var number json.Number
		if err := json.Unmarshal(value, &number); err != nil {
			return err
		}
		out = append(out, number.String())
	}
	*i = out
	return nil
}

// importReply is what the import splitter asks for.
type importReply struct {
	Facts []importFact `json:"facts"`
}

// Portions adapted from mem0ai/mem0 mem0-ts/src/oss/src/prompts/index.ts @
// f1791daa86d4 (Apache-2.0, Copyright 2023 Taranjeet Singh and the
// mem0ai/mem0 contributors). Modified by Ryoku: an item also carries its
// scope and whether it is sensitive.

// importFact is one fact of an import. Small models write a bare string or
// an object keyed fact or text; each is accepted.
type importFact struct {
	Text      string
	Scope     string
	Sensitive bool
}

func (f *importFact) UnmarshalJSON(raw []byte) error {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		*f = importFact{Text: text}
		return nil
	}
	var item struct {
		Text      string `json:"text"`
		Fact      string `json:"fact"`
		Scope     string `json:"scope"`
		Sensitive bool   `json:"sensitive"`
	}
	if err := strictDecode(raw, &item); err != nil {
		return err
	}
	if item.Text == "" {
		item.Text = item.Fact
	}
	*f = importFact{Text: item.Text, Scope: item.Scope, Sensitive: item.Sensitive}
	return nil
}

// Portions adapted from openai/codex codex-rs/memories/write/src/phase1_output.rs
// @ b75ec48a2c90 (Apache-2.0, Copyright 2025 OpenAI). Modified by Ryoku:
// Rashin's operations, after a bounded list of the user's events;
// "additionalProperties": false as there.
var learnSchema = `{"type":"object","additionalProperties":false,"required":["summary","events","ops"],"properties":{` +
	`"summary":{"type":"string"},` +
	`"events":{"type":"array","maxItems":` + strconv.Itoa(learnEventsMax) + `,"items":{"type":"object","additionalProperties":false,"required":["what","when"],"properties":{` +
	`"what":{"type":"string","maxLength":` + strconv.Itoa(learnEventCharMax) + `},` +
	`"when":{"type":"string"}}}},` +
	`"ops":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["op","scope","text","kind","explicit","sensitive","supersedes","evidence"],"properties":{` +
	`"op":{"type":"string","enum":["add","supersede"]},` +
	`"scope":{"type":"string","enum":["personal","project"]},` +
	`"text":{"type":"string"},` +
	`"kind":{"type":"string","enum":["permanent","durable","ephemeral","correction"]},` +
	`"explicit":{"type":"boolean"},` +
	`"sensitive":{"type":"boolean"},` +
	`"supersedes":{"type":"array","items":{"type":"string"}},` +
	`"evidence":{"type":"string"}}}}}}`

const importSchema = `{"type":"object","additionalProperties":false,"required":["facts"],"properties":{` +
	`"facts":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["text","scope","sensitive"],"properties":{` +
	`"text":{"type":"string"},` +
	`"scope":{"type":"string","enum":["personal","project"]},` +
	`"sensitive":{"type":"boolean"}}}}}}`

// jsonFormat asks for replies that follow schema. Only a local model server
// gets it: the gateway hands it upstream as it is, and several hosted
// providers refuse a request that carries one.
func jsonFormat(local bool, name, schema string) map[string]json.RawMessage {
	if !local {
		return nil
	}
	format := `{"type":"json_schema","json_schema":{"name":` + strconv.Quote(name) + `,"strict":true,"schema":` + schema + `}}`
	return map[string]json.RawMessage{"response_format": json.RawMessage(format)}
}

// complete runs one request and returns the whole reply text.
func complete(ctx context.Context, client llm.Client, request llm.Request) (string, llm.Final, error) {
	stream, err := client.Stream(ctx, request)
	if err != nil {
		return "", llm.Final{}, err
	}
	defer stream.Close()
	var text strings.Builder
	for {
		delta, err := stream.Next()
		if err == io.EOF {
			return text.String(), stream.Final(), nil
		}
		if err != nil {
			return "", llm.Final{}, err
		}
		text.WriteString(delta.Text)
	}
}

// Portions adapted from mem0ai/mem0 mem0/memory/utils.py @ f1791daa86d4
// (Apache-2.0, Copyright 2023 Taranjeet Singh and the mem0ai/mem0
// contributors). Modified by Ryoku: reasoning blocks go through stripThink.
var fencedJSON = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

// extractJSON finds the object in a model's reply: inside a code fence, or
// from the first { to the last }.
func extractJSON(reply string) string {
	reply = stripThink(reply)
	if match := fencedJSON.FindStringSubmatch(reply); match != nil {
		return match[1]
	}
	start, end := strings.Index(reply, "{"), strings.LastIndex(reply, "}")
	if start >= 0 && end > start {
		return reply[start : end+1]
	}
	return reply
}

// strictDecode decodes one JSON value, refusing fields the shape does not
// name and anything after the value.
func strictDecode(raw []byte, into any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("trailing data")
	}
	return nil
}

// parseReply reads a model's reply into into, tolerant of fences and
// reasoning around the object and strict about the object itself.
func parseReply(reply string, into any) error {
	if err := strictDecode([]byte(extractJSON(reply)), into); err != nil {
		return fmt.Errorf("%w: %v", errNoJSON, err)
	}
	return nil
}
