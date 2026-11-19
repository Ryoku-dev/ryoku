package protocol

import "strings"

const GlanceRead = "Transcribe every piece of text exactly. Output nothing else."

// GlanceInstruction is the instruction of every Theia turn, whatever model
// answers it.
func GlanceInstruction(p GlanceParams) string {
	language := strings.TrimSpace(p.Language)
	if language == "" {
		language = "the user's language"
	}
	// name what the turn carries so the model does not remark on what it lacks
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
		instruction := "Translate the supplied text, or the image's text if no text is supplied, " + source + "into " + target + ". Output only the translation."
		switch p.Options["style"] {
		case "natural":
			instruction += " Use natural phrasing that preserves the meaning."
		case "literal":
			instruction += " Stay close to the original wording and structure without making it unreadable."
		case "formal":
			instruction += " Use formal wording appropriate to the context."
		case "casual":
			instruction += " Use relaxed, conversational wording appropriate to the context."
		}
		if p.Options["simpler"] == "on" {
			instruction += " Use simple, familiar words without losing meaning."
		}
		return instruction
	case "summarize":
		format := "three to seven concise Markdown bullets"
		switch p.Options["style"] {
		case "tldr":
			format = "one sentence"
		case "main":
			format = "one short paragraph"
		}
		return "Summarize " + subject + " in " + format + ". Keep the main point and essential facts, without adding claims. Answer in " + language + "."
	case "deepread":
		lens := "List the main factual claims as Holds up, Unclear or Doubtful, with a brief reason for each. Distinguish evidence in the source from anything you cannot verify."
		switch p.Options["lens"] {
		case "assumptions":
			lens = "Identify the unstated assumptions and what changes if they do not hold."
		case "concepts":
			lens = "Explain the key ideas in plain words, with one brief example where useful."
		case "contrasts":
			lens = "Contrast the viewpoints or ideas, naming the important similarities and differences."
		case "related":
			lens = "Suggest a few relevant readings with searchable titles and authors. Never invent titles or URLs, and say when you are unsure."
		case "passages":
			lens = "Quote a few short, important passages exactly and explain why each matters."
		case "section":
			lens = "Explain how the sections fit together and how each supports the main point."
		}
		return "Read " + subject + " closely. " + lens + " Keep it short, use Markdown, and answer in " + language + "."
	case "solve":
		if p.Options["form"] == "latex" {
			return "Transcribe the mathematical expressions in " + subject + " as LaTeX. Do not solve them or invent missing symbols. Reply with one ```latex fenced block only."
		}
		return "Solve the problem in " + subject + ". Show concise numbered steps, then a final **Answer:** line. State any missing information instead of guessing. Answer in " + language + "."
	case "learn":
		count := p.Options["count"]
		switch count {
		case "3", "5", "8":
		default:
			count = "5"
		}
		return "Make " + count + " multiple-choice questions to check understanding of " + subject + ". Use only supported facts, four distinct choices and one correct answer per question. Write questions, choices and explanations in " + language + ". Reply with JSON only, using exactly this shape: {\"questions\":[{\"question\":\"...\",\"choices\":[\"...\",\"...\",\"...\",\"...\"],\"answer\":0,\"why\":\"...\"}]}. The questions array must contain " + count + " items; answer is the zero-based correct choice index, 0 to 3."
	case "define":
		target := strings.TrimSpace(p.Target)
		translation := "Leave translation empty unless a target language different from the source language is given."
		if target != "" {
			translation = "Put the term's translation in " + target + " in translation if that differs from the source language; otherwise leave it empty."
		}
		return "Define the selected word or phrase in " + subject + ", focusing on its meaning here. Give its reading or pronunciation, relevant parts of speech, short meanings and examples; mark the sense used here with here:true. Write meanings and note in " + language + ". " + translation + " Reply with JSON only, using exactly this shape: {\"term\":\"\",\"reading\":\"\",\"senses\":[{\"part\":\"\",\"meaning\":\"\",\"example\":\"\",\"here\":true}],\"note\":\"\",\"translation\":\"\"}."
	case "rewrite":
		variant := "Rephrase"
		switch p.Options["variant"] {
		case "shorter":
			variant = "Shorten"
		case "longer":
			variant = "Expand"
		case "simpler":
			variant = "Simplify"
		}
		audience := "Write for a general reader."
		switch p.Options["audience"] {
		case "expert":
			audience = "Write for an expert, keeping precise terms."
		case "esl":
			audience = "Write for a reader learning the language, using familiar words and clear sentences."
		}
		return variant + " the wording in " + subject + " while preserving its meaning, facts and intent. Do not invent details. " + audience + " Keep the text's own language. Output only the rewritten text."
	case "tone":
		return "Rewrite the wording in " + subject + " in a tone that is " + glanceTone(p.Options["tone"]) + " without changing its meaning or adding claims. Keep the text's own language. Output only the rewritten text."
	case "fluent":
		audience := "Use natural, idiomatic phrasing."
		if p.Options["audience"] == "esl" {
			audience = "Use simple, natural phrasing with familiar words and short sentences."
		}
		return "Smooth out the wording in " + subject + " without changing the meaning or intent. " + audience + " Keep the text's own language. Output only the rewritten text."
	case "humanize":
		return "Rewrite the wording in " + subject + " so it sounds personal and natural rather than stiff or formulaic. Keep its meaning, facts and own language; do not invent experiences or add filler. Output only the rewritten text."
	case "reactions":
		return "Describe how a reader is likely to take " + subject + ". In short Markdown sections, cover how they may feel, what lands, what may misfire, and one practical suggestion. Do not claim to know anyone's thoughts. Answer in " + language + "."
	case "reply":
		return "Write three ready-to-send replies to the message in " + subject + ", in a tone that is " + glanceTone(p.Options["tone"]) + ". Keep the message's own language and avoid inventing commitments or personal details. Reply with JSON only, using exactly this shape: {\"replies\":[{\"label\":\"Short\",\"text\":\"\"},{\"label\":\"Warm\",\"text\":\"\"},{\"label\":\"Detailed\",\"text\":\"\"}]}. Make Short concise, Warm considerate and Detailed fuller; keep the labels exactly as shown."
	case "aicheck":
		return "Assess how machine-written the wording in " + subject + " sounds. This is a rough guide, not proof of authorship; style alone cannot establish who wrote it. Give a score from 0 (human-sounding) to 100 (machine-sounding), a verdict of human, mixed or machine, and quote the phrases behind your assessment. Write explanations and note in " + language + ". Reply with JSON only, using exactly this shape: {\"score\":0,\"verdict\":\"human\",\"phrases\":[{\"text\":\"\",\"why\":\"\"}],\"note\":\"\"}."
	default:
		return ""
	}
}

func glanceTone(tone string) string {
	switch tone {
	case "formal", "confident", "friendly", "diplomatic", "direct", "empathetic",
		"assertive", "constructive", "engaging", "exciting", "informative", "inspirational",
		"objective", "persuasive", "academic", "casual", "compassionate", "cooperative",
		"fluent", "professional", "witty", "personable", "warm":
		return tone
	default:
		return "formal"
	}
}

// GlanceInstructionWithVoice adds saved style notes only when the user opts in.
func GlanceInstructionWithVoice(p GlanceParams, notes string) string {
	instruction := GlanceInstruction(p)
	if instruction != "" && p.Options["voice"] == "on" {
		if notes = strings.TrimSpace(notes); notes != "" {
			instruction += "\n\nFollow these writing style notes without changing the required language or output format:\n" + notes
		}
	}
	return instruction
}
