package prompts

import "ryoku-rashin/internal/protocol"

func BuiltIns() []protocol.Prompt {
	return []protocol.Prompt{
		{ID: "builtin:apology", Name: "Apology", Kind: "template", BuiltIn: true, Text: "Write a sincere apology to {{recipient}} about the following. Take responsibility without making excuses. Write in {{lang}}.\n\n{{content}}"},
		{ID: "builtin:thank-you", Name: "Thank-you note", Kind: "template", BuiltIn: true, Text: "Write a warm, specific thank-you note to {{recipient}} for the following, in {{lang}}.\n\n{{content}}"},
		{ID: "builtin:email-reply", Name: "Email reply", Kind: "template", BuiltIn: true, Text: "Draft a clear email reply in {{lang}}. Address the points in this message without inventing commitments.\n\n{{content}}"},
		{ID: "builtin:follow-up", Name: "Polite follow-up", Kind: "template", BuiltIn: true, Text: "Write a polite follow-up to {{recipient}} about this, in {{lang}}. Keep it short and give one clear next step.\n\n{{content}}"},
		{ID: "builtin:agenda", Name: "Meeting agenda", Kind: "template", BuiltIn: true, Text: "Create a meeting agenda about the following for {{duration}} minutes. Include the goal, timed topics and decisions needed. Write in {{lang}}.\n\n{{content}}"},
		{ID: "builtin:outline", Name: "Outline", Kind: "prompt", BuiltIn: true, Text: "Make a structured outline from this material, with a clear order and useful headings. Write in {{lang}}.\n\n{{content}}"},
		{ID: "builtin:tldr", Name: "TL;DR", Kind: "prompt", BuiltIn: true, Text: "Summarize this in one short paragraph in {{lang}}. Keep the key point and anything the reader needs to do.\n\n{{content}}"},
		{ID: "builtin:announcement", Name: "Announcement", Kind: "template", BuiltIn: true, Text: "Write a concise announcement for {{audience}} in {{lang}}. Say what changed, who it affects and what happens next.\n\n{{content}}"},
		{ID: "builtin:project-update", Name: "Project update", Kind: "template", BuiltIn: true, Text: "Turn these notes into a project update in {{lang}} with Progress, Blockers and Next steps. Do not invent dates or achievements.\n\n{{content}}"},
		{ID: "builtin:blog-intro", Name: "Blog post intro", Kind: "template", BuiltIn: true, Text: "Write a grounded introduction for a blog post about {{topic}}, for {{audience}}, in {{lang}}. Use these notes and avoid stock opening lines.\n\n{{content}}"},
		{ID: "builtin:social-post", Name: "Social post", Kind: "template", BuiltIn: true, Text: "Write a short social post about this for {{platform}} in {{lang}}. Keep it specific, with no invented facts.\n\n{{content}}"},
		{ID: "builtin:cover-letter", Name: "Cover letter", Kind: "template", BuiltIn: true, Text: "Draft a cover letter for {{role}} at {{company}} in {{lang}}. Use only the experience and details below, and avoid generic praise.\n\n{{content}}"},
		{ID: "builtin:explain-new", Name: "Explain like I'm new", Kind: "prompt", BuiltIn: true, Text: "Explain this in {{lang}} for someone new to the subject. Define unfamiliar terms and use one concrete example.\n\n{{content}}"},
		{ID: "builtin:pros-cons", Name: "Pros and cons", Kind: "prompt", BuiltIn: true, Text: "Compare the pros and cons of the following in {{lang}}. State assumptions and uncertainties, then suggest how to decide.\n\n{{content}}"},
	}
}
