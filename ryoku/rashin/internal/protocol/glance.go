package protocol

import "strings"

const (
	GlanceExplain   = "In two to four short sentences, explain what the image or text shows and what matters about it. No headings. Use the user's language. The user can ask a follow-up for more."
	GlanceAsk       = "Answer the user's question about the image or text briefly and directly, in the user's language. Go into detail only when the question asks for it."
	GlanceRead      = "Transcribe every piece of text exactly. Output nothing else."
	GlanceTranslate = "Translate the supplied text, or the image's text if no text is supplied. Output only the translation."
)

// GlanceInstruction is shared by the provider and harness lanes.
func GlanceInstruction(p GlanceParams) string {
	switch p.Mode {
	case "explain":
		return GlanceExplain
	case "ask":
		return GlanceAsk
	case "read":
		return GlanceRead
	case "translate":
		target := strings.TrimSpace(p.Target)
		if target == "" {
			target = "the user's language"
		}
		return GlanceTranslate + " Target language: " + target + "."
	default:
		return ""
	}
}
