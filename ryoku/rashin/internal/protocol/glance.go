package protocol

import "strings"

const GlanceRead = "Transcribe every piece of text exactly. Output nothing else."

// GlanceInstruction is shared by the provider and harness lanes.
func GlanceInstruction(p GlanceParams) string {
	language := strings.TrimSpace(p.Language)
	if language == "" {
		language = "the user's language"
	}
	// Name what the turn carries, so the model never remarks on what it lacks.
	subject, shows := "the image or text", "shows"
	switch {
	case len(p.Images) > 0:
		subject = "the image"
	case strings.TrimSpace(p.Text) != "":
		subject, shows = "the text", "says"
	}
	switch p.Mode {
	case "explain":
		return "In two to four short sentences, explain what " + subject + " " + shows + " and what matters about it. No headings. Answer in " + language + ". The user can ask a follow-up for more."
	case "ask":
		return "Answer the user's question about " + subject + " briefly and directly. Go into detail only when the question asks for it. Answer in " + language + "."
	case "read":
		return GlanceRead
	case "translate":
		target := strings.TrimSpace(p.Target)
		if target == "" {
			target = language
		}
		source := strings.TrimSpace(p.Source)
		if source != "" {
			source = "from " + source + " "
		}
		return "Translate the supplied text, or the image's text if no text is supplied, " + source + "into " + target + ". Output only the translation."
	default:
		return ""
	}
}
