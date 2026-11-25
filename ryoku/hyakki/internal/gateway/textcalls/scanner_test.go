package textcalls

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"ryoku-rashin/internal/llm"
)

var testTools = []llm.Tool{
	{Name: "read", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"limit":{"type":"integer"},"flags":{"type":"array","items":{"type":"boolean"}},"nested":{"type":"object","properties":{"count":{"type":"integer"}}},"enabled":{"type":"boolean"}}}`)},
	{Name: "write", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}}}`)},
}

type observed struct {
	text, reasoning string
	starts, calls   []llm.ToolCall
	broken          int
	fabricated      int
}

func scanFor(t *testing.T, dialect, input string, chunk int, cut, thinking bool) observed {
	t.Helper()
	id := 0
	s := ForDialect(dialect, Options{Tools: testTools, Thinking: thinking, NewID: func() string {
		id++
		return "call_" + string(rune('0'+id))
	}})
	if s == nil {
		t.Fatalf("no scanner for %s", dialect)
	}
	var out observed
	consume := func(events []Event) {
		for _, e := range events {
			out.text += e.Text
			out.reasoning += e.Reasoning
			if e.Start != nil {
				out.starts = append(out.starts, *e.Start)
			}
			if e.Call != nil {
				out.calls = append(out.calls, *e.Call)
			}
			if e.Broken != "" {
				out.broken++
			}
			if e.Fabricated {
				out.fabricated++
			}
		}
	}
	for i := 0; i < len(input); i += chunk {
		consume(s.Feed(input[i:min(i+chunk, len(input))]))
	}
	consume(s.Flush(cut))
	return out
}

func TestScannerDialectsAndFragments(t *testing.T) {
	cases := []struct {
		name, dialect, input, text, reasoning string
		calls                                 []llm.ToolCall
		broken, fabricated                    int
		thinking                              bool
	}{
		{"xml typed and Unicode", XMLFn, `Before <tool_call><function=read><parameter=path>日本/notes</parameter><parameter=limit>"12"</parameter><parameter=flags>["true",false]</parameter><parameter=enabled>true</parameter></function></tool_call> after`, "Before  after", "", []llm.ToolCall{{Name: "read", Arguments: `{"path":"日本/notes","limit":12,"flags":[true,false],"enabled":true}`}}, 0, 0, false},
		{"xml missing parameter closer", XMLFn, `<tool_call><function=read><parameter=path>first<parameter=limit>3</parameter></function></tool_call>`, "", "", []llm.ToolCall{{Name: "read", Arguments: `{"path":"first","limit":3}`}}, 0, 0, false},
		{"xml name attribute", XMLFn, "<function=write><parameter name=\"path\">notes</parameter><parameter name=\"content\">hello\n世界</parameter></function>", "", "", []llm.ToolCall{{Name: "write", Arguments: `{"path":"notes","content":"hello\n世界"}`}}, 0, 0, false},
		{"xml multiple functions", XMLFn, `<tool_call><function=read><parameter=path>a</parameter></function><function=read><parameter=path>b</parameter></function></tool_call>`, "", "", []llm.ToolCall{{Name: "read", Arguments: `{"path":"a"}`}, {Name: "read", Arguments: `{"path":"b"}`}}, 0, 0, false},
		{"JSON nested and escaped marker", Hermes, `A<tool_call>{name:'read',arguments:{path:'a',nested:{count:'7'},content:'</tool_call>',flags:['true','false'],},}</tool_call>B`, "AB", "", []llm.ToolCall{{Name: "read", Arguments: `{"path":"a","nested":{"count":7},"content":"</tool_call>","flags":[true,false]}`}}, 0, 0, false},
		{"JSON arguments encoded as string", Hermes, `<tool_call>{"name":"read","arguments":"{path:'x',limit:'4'}"}</tool_call>`, "", "", []llm.ToolCall{{Name: "read", Arguments: `{"path":"x","limit":4}`}}, 0, 0, false},
		{"JSON missing wrapper closer", Hermes, `<tool_call>{"name":"read","arguments":{"path":"ok"}} prose`, " prose", "", []llm.ToolCall{{Name: "read", Arguments: `{"path":"ok"}`}}, 0, 0, false},
		{"GLM healed value", GLM, `<tool_call>read<arg_key>path</arg_key><arg_value>hello <arg_key>limit</arg_key><arg_value>5</arg_value></tool_call>`, "", "", []llm.ToolCall{{Name: "read", Arguments: `{"path":"hello","limit":5}`}}, 0, 0, false},
		{"GLM malformed key", GLM, `<tool_call>read<arg_key>path<arg_value>x</arg_value></tool_call>`, "", "", nil, 1, 0, false},
		{"Pythonic call list and nested", Pythonic, `<|tool_call_start|>[read(path="\u65e5本", nested={"count":"3"}, flags=[True,False]),write(path="p", content="x,y") ]<|tool_call_end|>`, "", "", []llm.ToolCall{{Name: "read", Arguments: `{"path":"日本","nested":{"count":3},"flags":[true,false]}`}, {Name: "write", Arguments: `{"path":"p","content":"x,y"}`}}, 0, 0, false},
		{"Pythonic bare positional", Pythonic, `[read("notes", 5)]`, "", "", []llm.ToolCall{{Name: "read", Arguments: `{"path":"notes","limit":5}`}}, 0, 0, false},
		{"Pythonic prose brackets", Pythonic, `[not a call] just text`, `[not a call] just text`, "", nil, 0, 0, false},
		{"thinking and text", Hermes, `<think>plan carefully</think>Answer`, "Answer", "plan carefully", nil, 0, 0, true},
		{"fabricated result", Hermes, `before<tool_response>fabricated</tool_response>after`, "before", "", nil, 0, 1, false},
		{"truncated JSON", Hermes, `<tool_call>{"name":"read","arguments":{"path":"partial`, "", "", nil, 1, 0, false},
		{"truncated XML", XMLFn, `<tool_call><function=write><parameter=path>foo</parameter><parameter=content>unfinished`, "", "", nil, 1, 0, false},
		{"truncated Pythonic", Pythonic, `[read(path="not done`, "", "", nil, 1, 0, false},
		{"malformed args cannot run", Hermes, `<tool_call>{"name":"read","arguments":"{broken:"}</tool_call>`, "", "", nil, 1, 0, false},
		{"trailing garbage inside call cannot run", Hermes, `<tool_call>{"name":"read","arguments":{"path":"x"}} nope</tool_call>`, "", "", nil, 1, 0, false},
	}
	for _, tc := range cases {
		for _, chunk := range []int{1, 2, 7, 4096} {
			t.Run(tc.name+"/chunk="+strconv.Itoa(chunk), func(t *testing.T) {
				got := scanFor(t, tc.dialect, tc.input, chunk, false, tc.thinking)
				if got.text != tc.text || got.reasoning != tc.reasoning || got.broken != tc.broken || got.fabricated != tc.fabricated {
					t.Errorf("visible text/reason/broken/fabricated = %#v, want %q %q %d %d", got, tc.text, tc.reasoning, tc.broken, tc.fabricated)
				}
				if len(got.calls) != len(tc.calls) || (tc.broken == 0 && len(got.starts) != len(tc.calls)) {
					t.Fatalf("starts/calls = %#v, %#v; want %#v", got.starts, got.calls, tc.calls)
				}
				for i, want := range tc.calls {
					if got.calls[i].Name != want.Name || got.calls[i].Arguments != want.Arguments || got.starts[i].Name != want.Name || got.starts[i].ID != got.calls[i].ID || got.calls[i].ID == "" {
						t.Errorf("call[%d] = %#v start %#v; want %#v", i, got.calls[i], got.starts[i], want)
					}
				}
			})
		}
	}
}

func TestScannerCutNeverRunsOpenCall(t *testing.T) {
	for _, dialect := range []string{XMLFn, Hermes, Pythonic, GLM} {
		var input string
		switch dialect {
		case XMLFn:
			input = `<tool_call><function=read><parameter=path>x`
		case Hermes:
			input = `<tool_call>{"name":"read","arguments":{"path":"x"}}`
		case Pythonic:
			input = `[read(path="x"`
		case GLM:
			input = `<tool_call>read<arg_key>path</arg_key><arg_value>x</arg_value>`
		}
		got := scanFor(t, dialect, input, 1, true, false)
		if len(got.calls) != 0 || got.fabricated != 0 {
			t.Errorf("%s cut produced calls: %#v", dialect, got)
		}
	}
}

func TestRepairJSONAndCoercion(t *testing.T) {
	cases := []struct{ input, want string }{
		{`{path:'a',limit:+3, flags:[True,False,],}`, `{"path":"a","limit":3,"flags":[true,false]}`},
		{"{\"text\":\"raw\nline\", /* comment */ count: 0x10}", `{"text":"raw\nline","count":16}`},
		{`{path: src/main.go // comment
}`, `{"path":"src/main.go"}`},
		{`{'message':'it\'s ok',}`, `{"message":"it's ok"}`},
	}
	for _, tc := range cases {
		got, err := RepairJSON(tc.input)
		if err != nil || got != tc.want {
			t.Errorf("RepairJSON(%q) = %q, %v; want %q", tc.input, got, err, tc.want)
		}
	}
	for _, malformed := range []string{`{"a":`, `{"a":1}extra`, `{"x":NaN}`, `{"x":1e}`} {
		if _, err := RepairJSON(malformed); err == nil {
			t.Errorf("accepted %q", malformed)
		}
	}
	args := `{"path":27,"limit":"3.0","enabled":"yes","flags":["true","false"],"nested":{"count":"8"}}`
	want := `{"path":"27","limit":3,"enabled":true,"flags":[true,false],"nested":{"count":8}}`
	if got := Coerce(args, testTools[0].Parameters); got != want {
		t.Errorf("Coerce = %s; want %s", got, want)
	}
}

func TestDialectReplay(t *testing.T) {
	original := []llm.Message{
		{Role: llm.RoleSystem, Content: "Be useful."},
		{Role: llm.RoleAssistant, Content: "Checking", ToolCalls: []llm.ToolCall{{ID: "call_1", Name: "read", Arguments: `{"path":"a</parameter>&b","limit":3,"flags":[true,false]}`}}},
		{Role: llm.RoleTool, ToolCallID: "call_1", Name: "read", Content: "found", IsError: true},
	}
	for _, d := range Dialects {
		encoded := Encode(d, original, testTools)
		if len(encoded) != 3 || encoded[0].Role != llm.RoleSystem || !strings.Contains(encoded[0].Content, "<tools>") || len(encoded[1].ToolCalls) != 0 || encoded[2].Role != llm.RoleUser || !strings.Contains(encoded[2].Content, "Error: found") {
			t.Errorf("%s encoded: %#v", d, encoded)
			continue
		}
		if len(original[1].ToolCalls) != 1 || original[2].Role != llm.RoleTool {
			t.Fatal("mutated original history")
		}
		got := scanFor(t, d, RenderCall(d, original[1].ToolCalls[0], testTools), 1, false, false)
		if len(got.calls) != 1 || got.calls[0].Name != "read" || !reflect.DeepEqual(json.RawMessage(got.calls[0].Arguments), json.RawMessage(original[1].ToolCalls[0].Arguments)) {
			t.Errorf("%s replay: %#v", d, got)
		}
	}
	if ForDialect(Native, Options{}) != nil || ForDialect("bad", Options{}) != nil || len(Stops(Native)) != 0 || Detect(`hello`) != "" || Detect(`<tool_call>{}`) != Hermes {
		t.Error("unexpected dialect handling")
	}
}

func TestAutoQuotedMarkupAndCodeFences(t *testing.T) {
	const jsonCall = `<tool_call>{"name":"write","arguments":{"path":"x","content":"y"}}</tool_call>`
	const xmlCall = `<tool_call><function=read><parameter=path>/etc/passwd</parameter></function></tool_call>`
	for _, tc := range []struct {
		name, input string
	}{
		{"inline JSON example", "Hermes writes calls like `" + jsonCall + "` in its text."},
		{"fenced XML example", "```xml\n" + xmlCall + "\n```"},
		{"inline result marker", "The server answers with `<tool_response>` blocks, and then continues."},
		{"inline split ticks", "A ``" + jsonCall + "`` example."},
		{"fenced result marker", "```\n<tool_response>do not stop</tool_response>\n``` after"},
		{"triple delimiter inside fence", "```xml\nconst marker = \"```\";\n" + xmlCall + "\n```"},
		{"bare function example", "Use <function=name> to start. This is prose."},
		{"unclosed opener", "Qwen wraps each call in a <tool_call> tag. The rest of this long answer must survive."},
	} {
		for _, chunk := range []int{1, 3, 13, 4096} {
			t.Run(tc.name+"/chunk="+strconv.Itoa(chunk), func(t *testing.T) {
				got := scanFor(t, Auto, tc.input, chunk, false, false)
				if got.text != tc.input || len(got.starts) != 0 || len(got.calls) != 0 || got.fabricated != 0 || got.broken != 0 {
					t.Fatalf("scanned prose as a call or discarded text: %#v", got)
				}
			})
		}
	}
}

func TestAutoOnlyEmitsOfferedTools(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		offered     []llm.Tool
	}{
		{"JSON", `<tool_call>{"name":"write","arguments":{"path":"x","content":"y"}}</tool_call>`, testTools[:1]},
		{"XML", `<tool_call><function=write><parameter=path>x</parameter></function></tool_call>`, testTools[:1]},
		{"bare function", `<function=write></function>`, testTools[:1]},
		{"GLM", `<tool_call>write<arg_key>path</arg_key><arg_value>x</arg_value></tool_call>`, testTools[:1]},
		{"Pythonic", `<|tool_call_start|>[write(path="x")]<|tool_call_end|>`, testTools[:1]},
		{"no tools", `<tool_call>{"name":"read","arguments":{"path":"x"}}</tool_call>`, nil},
		{"mixed JSON", `<tool_call>[{"name":"read","arguments":{"path":"x"}},{"name":"write","arguments":{"path":"x"}}]</tool_call>`, testTools[:1]},
	} {
		for _, chunk := range []int{1, 7, 4096} {
			t.Run(tc.name+"/chunk="+strconv.Itoa(chunk), func(t *testing.T) {
				s := ForDialect(Auto, Options{Tools: tc.offered})
				var out observed
				for i := 0; i < len(tc.input); i += chunk {
					for _, e := range s.Feed(tc.input[i:min(i+chunk, len(tc.input))]) {
						out.text += e.Text
						if e.Start != nil {
							out.starts = append(out.starts, *e.Start)
						}
						if e.Call != nil {
							out.calls = append(out.calls, *e.Call)
						}
					}
				}
				for _, e := range s.Flush(false) {
					out.text += e.Text
					if e.Start != nil {
						out.starts = append(out.starts, *e.Start)
					}
					if e.Call != nil {
						out.calls = append(out.calls, *e.Call)
					}
				}
				if out.text != tc.input || len(out.starts) != 0 || len(out.calls) != 0 {
					t.Fatalf("unoffered call was consumed or emitted: %#v", out)
				}
			})
		}
	}
}

func TestAutoValidCallsOutsideCode(t *testing.T) {
	for _, tc := range []struct {
		input, text, name string
	}{
		{"`<tool_call>example</tool_call>` <tool_call>{\"name\":\"read\",\"arguments\":{\"path\":\"x\"}}</tool_call> done", "`<tool_call>example</tool_call>`  done", "read"},
		{"A ``<tool_response>`` example. <tool_call>{\"name\":\"read\",\"arguments\":{\"path\":\"x\"}}</tool_call>", "A ``<tool_response>`` example. ", "read"},
		{"A ```<tool_response>``` example. <tool_call>{\"name\":\"read\",\"arguments\":{\"path\":\"x\"}}</tool_call>", "A ```<tool_response>``` example. ", "read"},
		{"```\n<function=read></function>\n```\n<function=read><parameter=path>x</parameter></function>", "```\n<function=read></function>\n```\n", "read"},
		{`<|tool_call_start|>[read(path="x")]<|tool_call_end|>`, "", "read"},
	} {
		for _, chunk := range []int{1, 3, 13, 4096} {
			got := scanFor(t, Auto, tc.input, chunk, false, false)
			if got.text != tc.text || len(got.calls) != 1 || got.calls[0].Name != tc.name || len(got.starts) != 1 || got.starts[0].ID != got.calls[0].ID || got.broken != 0 {
				t.Errorf("chunk=%d input=%q: %#v", chunk, tc.input, got)
			}
		}
	}
}

func TestAutoReleasesUnclosedMarkupWithinBound(t *testing.T) {
	for _, chunk := range []int{1, 13, 2048, 131072} {
		t.Run(strconv.Itoa(chunk), func(t *testing.T) {
			input := `<tool_call>{"name":"read","arguments":{"path":"` + strings.Repeat("x", maxAutoHeld+100) + `"}` + " and the answer continues"
			s := ForDialect(Auto, Options{Tools: testTools}).(*scanner)
			var got observed
			for i := 0; i < len(input); i += chunk {
				for _, e := range s.Feed(input[i:min(i+chunk, len(input))]) {
					got.text += e.Text
					if e.Start != nil {
						got.starts = append(got.starts, *e.Start)
					}
					if e.Call != nil {
						got.calls = append(got.calls, *e.Call)
					}
					if e.Broken != "" {
						got.broken++
					}
				}
				if s.raw.Len()+len(s.buf) > maxAutoHeld {
					t.Fatalf("scanner held %d bytes after feed", s.raw.Len()+len(s.buf))
				}
			}
			const continuation = "<tool_response>not a result</tool_response>"
			for _, e := range s.Feed(continuation) {
				got.text += e.Text
				if e.Fabricated {
					got.fabricated++
				}
				if e.Call != nil {
					got.calls = append(got.calls, *e.Call)
				}
			}
			if got.text == "" {
				t.Fatal("unterminated markup was held until flush")
			}
			for _, e := range s.Flush(false) {
				got.text += e.Text
				if e.Call != nil {
					got.calls = append(got.calls, *e.Call)
				}
				if e.Broken != "" {
					got.broken++
				}
			}
			if got.text != input+continuation || len(got.starts) != 0 || len(got.calls) != 0 || got.broken != 0 || got.fabricated != 0 {
				t.Fatalf("unclosed markup lost text or executed: text length=%d, starts=%d, calls=%d, broken=%d, fabricated=%d", len(got.text), len(got.starts), len(got.calls), got.broken, got.fabricated)
			}
		})
	}
}
