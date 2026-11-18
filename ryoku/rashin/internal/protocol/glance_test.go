package protocol

import "testing"

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
