package protocol

import (
	"strings"
	"testing"
)

func TestGlanceInstruction(t *testing.T) {
	for _, c := range []struct {
		name   string
		params GlanceParams
		want   string
	}{
		{
			name:   "explain names the answer language",
			params: GlanceParams{Mode: "explain", Language: "English"},
			want:   "In two to four short sentences, explain what the image or text shows and what matters about it. No headings. Answer in English. The user can ask a follow-up for more.",
		},
		{
			name:   "ask names the answer language",
			params: GlanceParams{Mode: "ask", Language: "Japanese"},
			want:   "Answer the user's question about the image or text briefly and directly. Go into detail only when the question asks for it. Answer in Japanese.",
		},
		{
			name:   "explain names highlighted text as text",
			params: GlanceParams{Mode: "explain", Language: "English", Text: "Bonjour tout le monde"},
			want:   "In two to four short sentences, explain what the text says and what matters about it. No headings. Answer in English. The user can ask a follow-up for more.",
		},
		{
			name:   "explain names a capture as an image even with its text",
			params: GlanceParams{Mode: "explain", Language: "English", Text: "Bonjour", Images: []Attachment{{Path: "/tmp/box.png"}}},
			want:   "In two to four short sentences, explain what the image shows and what matters about it. No headings. Answer in English. The user can ask a follow-up for more.",
		},
		{
			name:   "ask names highlighted text as text",
			params: GlanceParams{Mode: "ask", Language: "English", Text: " Bonjour "},
			want:   "Answer the user's question about the text briefly and directly. Go into detail only when the question asks for it. Answer in English.",
		},
		{
			name:   "read ignores language choices",
			params: GlanceParams{Mode: "read", Language: "English", Source: "Japanese", Target: "French"},
			want:   "Transcribe every piece of text exactly. Output nothing else.",
		},
		{
			name:   "translate names source and target",
			params: GlanceParams{Mode: "translate", Language: "English", Source: "Japanese", Target: "French"},
			want:   "Translate the supplied text, or the image's text if no text is supplied, from Japanese into French. Output only the translation.",
		},
		{
			name:   "translate names only the target",
			params: GlanceParams{Mode: "translate", Target: "French"},
			want:   "Translate the supplied text, or the image's text if no text is supplied, into French. Output only the translation.",
		},
		{
			name:   "translate defaults target to the answer language",
			params: GlanceParams{Mode: "translate", Language: "English"},
			want:   "Translate the supplied text, or the image's text if no text is supplied, into English. Output only the translation.",
		},
		{
			name:   "translate keeps source with the default target",
			params: GlanceParams{Mode: "translate", Language: "English", Source: "Japanese"},
			want:   "Translate the supplied text, or the image's text if no text is supplied, from Japanese into English. Output only the translation.",
		},
		{
			name:   "explain without a language keeps the fallback wording",
			params: GlanceParams{Mode: "explain"},
			want:   "In two to four short sentences, explain what the image or text shows and what matters about it. No headings. Answer in the user's language. The user can ask a follow-up for more.",
		},
		{
			name:   "ask without a language keeps the fallback wording",
			params: GlanceParams{Mode: "ask"},
			want:   "Answer the user's question about the image or text briefly and directly. Go into detail only when the question asks for it. Answer in the user's language.",
		},
		{
			name:   "translate without languages keeps the fallback wording",
			params: GlanceParams{Mode: "translate"},
			want:   "Translate the supplied text, or the image's text if no text is supplied, into the user's language. Output only the translation.",
		},
		{
			name:   "translate trims language names",
			params: GlanceParams{Mode: "translate", Language: " English ", Source: " Japanese ", Target: " \t"},
			want:   "Translate the supplied text, or the image's text if no text is supplied, from Japanese into English. Output only the translation.",
		},
		{
			name:   "unknown mode has no instruction",
			params: GlanceParams{Mode: "unknown", Language: "English"},
			want:   "",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := GlanceInstruction(c.params); got != c.want {
				t.Errorf("GlanceInstruction = %q, want %q", got, c.want)
			}
		})
	}
}

func TestGlanceActionInstructions(t *testing.T) {
	for _, c := range []struct {
		mode    string
		key     string
		value   string
		want    string
		rewrite bool
	}{
		{mode: "summarize", want: "three to seven concise Markdown bullets"},
		{mode: "summarize", key: "style", value: "points", want: "three to seven concise Markdown bullets"},
		{mode: "summarize", key: "style", value: "tldr", want: "one sentence"},
		{mode: "summarize", key: "style", value: "main", want: "one short paragraph"},
		{mode: "deepread", want: "Holds up, Unclear or Doubtful"},
		{mode: "deepread", key: "lens", value: "factcheck", want: "anything you cannot verify"},
		{mode: "deepread", key: "lens", value: "assumptions", want: "unstated assumptions"},
		{mode: "deepread", key: "lens", value: "concepts", want: "key ideas in plain words"},
		{mode: "deepread", key: "lens", value: "contrasts", want: "similarities and differences"},
		{mode: "deepread", key: "lens", value: "related", want: "Never invent titles or URLs"},
		{mode: "deepread", key: "lens", value: "passages", want: "passages exactly"},
		{mode: "deepread", key: "lens", value: "section", want: "sections fit together"},
		{mode: "solve", want: "numbered steps, then a final **Answer:** line"},
		{mode: "solve", key: "form", value: "steps", want: "numbered steps, then a final **Answer:** line"},
		{mode: "solve", key: "form", value: "latex", want: "Reply with one ```latex fenced block only"},
		{mode: "learn", want: "Make 5 multiple-choice questions"},
		{mode: "learn", key: "count", value: "3", want: "Make 3 multiple-choice questions"},
		{mode: "learn", key: "count", value: "5", want: "Make 5 multiple-choice questions"},
		{mode: "learn", key: "count", value: "8", want: "Make 8 multiple-choice questions"},
		{mode: "define", want: `"senses":[{"part":"","meaning":"","example":"","here":true}]`},
		{mode: "rewrite", want: "Rephrase the wording", rewrite: true},
		{mode: "rewrite", key: "variant", value: "rephrase", want: "Rephrase the wording", rewrite: true},
		{mode: "rewrite", key: "variant", value: "shorter", want: "Shorten the wording", rewrite: true},
		{mode: "rewrite", key: "variant", value: "longer", want: "Expand the wording", rewrite: true},
		{mode: "rewrite", key: "variant", value: "simpler", want: "Simplify the wording", rewrite: true},
		{mode: "rewrite", key: "audience", value: "general", want: "general reader", rewrite: true},
		{mode: "rewrite", key: "audience", value: "expert", want: "expert, keeping precise terms", rewrite: true},
		{mode: "rewrite", key: "audience", value: "esl", want: "reader learning the language", rewrite: true},
		{mode: "tone", want: "tone that is formal", rewrite: true},
		{mode: "fluent", want: "natural, idiomatic phrasing", rewrite: true},
		{mode: "fluent", key: "audience", value: "general", want: "natural, idiomatic phrasing", rewrite: true},
		{mode: "fluent", key: "audience", value: "esl", want: "familiar words and short sentences", rewrite: true},
		{mode: "humanize", want: "personal and natural rather than stiff or formulaic", rewrite: true},
		{mode: "reactions", want: "how they may feel, what lands, what may misfire, and one practical suggestion"},
		{mode: "reply", want: "three ready-to-send replies", rewrite: true},
		{mode: "aicheck", want: "rough guide, not proof of authorship"},
	} {
		t.Run(c.mode+"/"+c.key+"="+c.value, func(t *testing.T) {
			p := GlanceParams{Mode: c.mode, Text: "Bonjour", Language: "Japanese"}
			if c.key != "" {
				p.Options = map[string]string{c.key: c.value}
			}
			got := GlanceInstruction(p)
			if !strings.Contains(got, c.want) {
				t.Errorf("instruction %q does not contain %q", got, c.want)
			}
			if !strings.Contains(got, "the text") || strings.Contains(got, "the image") {
				t.Errorf("instruction does not name the carried text: %q", got)
			}
			if c.rewrite {
				if strings.Contains(got, "Japanese") || !strings.Contains(got, "own language") {
					t.Errorf("rewrite must keep the text's language: %q", got)
				}
			} else if c.value != "latex" && !strings.Contains(got, "Japanese") {
				t.Errorf("instruction must name the answer language: %q", got)
			}
			if strings.Contains(got, "\u2014") || strings.Contains(got, "AI") {
				t.Errorf("instruction contains forbidden wording: %q", got)
			}
			p.Images = []Attachment{{Path: "/tmp/capture.png"}}
			got = GlanceInstruction(p)
			if !strings.Contains(got, "the image") || strings.Contains(got, "image or text") {
				t.Errorf("instruction does not name the carried image: %q", got)
			}
		})
	}
}

func TestGlanceTones(t *testing.T) {
	tones := []string{
		"formal", "confident", "friendly", "diplomatic", "direct", "empathetic",
		"assertive", "constructive", "engaging", "exciting", "informative", "inspirational",
		"objective", "persuasive", "academic", "casual", "compassionate", "cooperative",
		"fluent", "professional", "witty", "personable", "warm",
	}
	for _, mode := range []string{"tone", "reply"} {
		for _, tone := range tones {
			t.Run(mode+"/"+tone, func(t *testing.T) {
				got := GlanceInstruction(GlanceParams{Mode: mode, Text: "Hello", Options: map[string]string{"tone": tone}})
				if !strings.Contains(got, "tone that is "+tone) {
					t.Errorf("instruction did not apply tone %q: %q", tone, got)
				}
			})
		}
	}
}

func TestGlanceJSONShapes(t *testing.T) {
	for _, c := range []struct {
		mode  string
		shape string
	}{
		{"learn", `{"questions":[{"question":"...","choices":["...","...","...","..."],"answer":0,"why":"..."}]}`},
		{"define", `{"term":"","reading":"","senses":[{"part":"","meaning":"","example":"","here":true}],"note":"","translation":""}`},
		{"reply", `{"replies":[{"label":"Short","text":""},{"label":"Warm","text":""},{"label":"Detailed","text":""}]}`},
		{"aicheck", `{"score":0,"verdict":"human","phrases":[{"text":"","why":""}],"note":""}`},
	} {
		t.Run(c.mode, func(t *testing.T) {
			got := GlanceInstruction(GlanceParams{Mode: c.mode, Text: "Bonjour"})
			if !strings.Contains(got, "Reply with JSON only") || !strings.Contains(got, c.shape) {
				t.Errorf("instruction must require the exact JSON shape %s: %q", c.shape, got)
			}
		})
	}
}

func TestGlanceTranslationStyle(t *testing.T) {
	p := GlanceParams{Mode: "translate", Source: "French", Target: "English", Text: "Bonjour"}
	base := GlanceInstruction(p)
	for _, c := range []struct {
		style string
		want  string
	}{
		{"natural", "natural phrasing that preserves the meaning"},
		{"literal", "close to the original wording and structure"},
		{"formal", "formal wording appropriate to the context"},
		{"casual", "relaxed, conversational wording"},
	} {
		t.Run(c.style, func(t *testing.T) {
			p.Options = map[string]string{"style": c.style}
			got := GlanceInstruction(p)
			if !strings.HasPrefix(got, base) || !strings.Contains(got, c.want) {
				t.Errorf("style not applied to translation: %q", got)
			}
			p.Options["simpler"] = "on"
			got = GlanceInstruction(p)
			if !strings.Contains(got, c.want) || !strings.Contains(got, "simple, familiar words") {
				t.Errorf("simpler words must combine with the style: %q", got)
			}
		})
	}
	p.Options = map[string]string{"simpler": "on"}
	if got := GlanceInstruction(p); !strings.HasPrefix(got, base) || !strings.Contains(got, "simple, familiar words") {
		t.Errorf("simpler words must work without a style: %q", got)
	}
	p.Options = map[string]string{"style": "unknown", "simpler": "off"}
	if got := GlanceInstruction(p); got != base {
		t.Errorf("unknown options changed the legacy translation: %q", got)
	}
}

func TestGlanceDefinitionTranslation(t *testing.T) {
	p := GlanceParams{Mode: "define", Text: "Bonjour", Language: "Japanese"}
	got := GlanceInstruction(p)
	if !strings.Contains(got, "Leave translation empty") {
		t.Errorf("definition should not guess a translation target: %q", got)
	}
	p.Target = " English "
	got = GlanceInstruction(p)
	if !strings.Contains(got, "translation in English") || !strings.Contains(got, "differs from the source language") {
		t.Errorf("definition should use the requested translation target: %q", got)
	}
}

func TestGlanceVoiceNotes(t *testing.T) {
	for _, mode := range []string{"rewrite", "tone", "humanize", "reply"} {
		t.Run(mode, func(t *testing.T) {
			p := GlanceParams{Mode: mode, Text: "Bonjour", Options: map[string]string{"voice": "on"}}
			base := GlanceInstruction(p)
			got := GlanceInstructionWithVoice(p, "  Prefer short sentences and plain words. \n")
			want := base + "\n\nFollow these writing style notes without changing the required language or output format:\nPrefer short sentences and plain words."
			if got != want {
				t.Errorf("voice notes = %q, want %q", got, want)
			}
			if got := GlanceInstructionWithVoice(p, " \t\n"); got != base {
				t.Errorf("empty notes changed the instruction: %q", got)
			}
			p.Options["voice"] = "off"
			if got := GlanceInstructionWithVoice(p, "Use my style"); got != base {
				t.Errorf("disabled voice changed the instruction: %q", got)
			}
			p.Options = nil
			if got := GlanceInstructionWithVoice(p, "Use my style"); got != base {
				t.Errorf("voice must be opt-in: %q", got)
			}
		})
	}
	if got := GlanceInstructionWithVoice(GlanceParams{Mode: "unknown", Options: map[string]string{"voice": "on"}}, "Use my style"); got != "" {
		t.Errorf("voice notes accepted an unknown mode: %q", got)
	}
}

func TestGlanceOptionFallbacks(t *testing.T) {
	for _, c := range []struct {
		mode string
		key  string
	}{
		{"summarize", "style"},
		{"deepread", "lens"},
		{"solve", "form"},
		{"learn", "count"},
		{"rewrite", "variant"},
		{"rewrite", "audience"},
		{"tone", "tone"},
		{"fluent", "audience"},
		{"reply", "tone"},
	} {
		t.Run(c.mode+"/"+c.key, func(t *testing.T) {
			p := GlanceParams{Mode: c.mode, Text: "Bonjour"}
			want := GlanceInstruction(p)
			p.Options = map[string]string{c.key: "unknown"}
			if got := GlanceInstruction(p); got != want {
				t.Errorf("unknown option = %q, want default %q", got, want)
			}
		})
	}
}
