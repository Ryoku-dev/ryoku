.pragma library

var title = "Rashin";
var lead = "Ask, work with agents and use Theia to read the screen or work with highlighted text. Rashin is optional and runs nothing while it is off.";

var sections = [
    { id: "rashin", title: "Rashin" },
    { id: "theia", title: "Theia" },
    { id: "theia-reading", title: "Reading" },
    { id: "theia-writing", title: "Writing" },
    { id: "theia-translate", title: "Translate" },
    { id: "theia-listening", title: "Listening" },
    { id: "theia-quiet", title: "Quiet places" },
    { id: "theia-engine", title: "Model" },
    { id: "status", title: "Status" },
    { id: "open", title: "Open Rashin" }
];

function sight(key, fallback) {
    return { store: "sight", key: key, fallback: fallback };
}

function featureOn(id) {
    return { bind: sight("features." + id, false), is: [true] };
}

var toneOptions = [
    { label: "Formal", value: "tone:formal" }, { label: "Confident", value: "tone:confident" },
    { label: "Friendly", value: "tone:friendly" }, { label: "Diplomatic", value: "tone:diplomatic" },
    { label: "Direct", value: "tone:direct" }, { label: "Empathetic", value: "tone:empathetic" },
    { label: "Assertive", value: "tone:assertive" }, { label: "Constructive", value: "tone:constructive" },
    { label: "Engaging", value: "tone:engaging" },
    { label: "Exciting", value: "tone:exciting" }, { label: "Informative", value: "tone:informative" },
    { label: "Inspirational", value: "tone:inspirational" }, { label: "Objective", value: "tone:objective" },
    { label: "Persuasive", value: "tone:persuasive" }, { label: "Academic", value: "tone:academic" },
    { label: "Casual", value: "tone:casual" }, { label: "Compassionate", value: "tone:compassionate" },
    { label: "Cooperative", value: "tone:cooperative" }, { label: "Fluent", value: "tone:fluent" },
    { label: "Professional", value: "tone:professional" }, { label: "Witty", value: "tone:witty" },
    { label: "Personable", value: "tone:personable" }, { label: "Warm", value: "tone:warm" }
];

var features = [
    { id: "inplace", section: "theia", label: "In place", detail: "Select text in any field and tap Right Ctrl three times to translate or fix it where it is.", shows: ["inplace"] },
    { id: "summarize", section: "theia-reading", label: "Summarize", detail: "Key points, a one-line TL;DR or the main point of long text.", shows: ["panel", "bubble"] },
    { id: "deepread", section: "theia-reading", label: "Deep read", detail: "Fact check, assumptions, key ideas, contrasts and related reading.", shows: ["panel"] },
    { id: "math", section: "theia-reading", label: "Math", detail: "Read equations as LaTeX and solve problems step by step.", shows: ["panel"] },
    { id: "learn", section: "theia-reading", label: "Learn", detail: "Turn what you read into a short quiz.", shows: ["panel"] },
    { id: "readaloud", section: "theia-listening", label: "Read aloud", detail: "Hear answers and summaries in a voice on this computer.", shows: ["panel"] },
    { id: "proofread", section: "theia-writing", label: "Fix writing", detail: "Spelling and grammar fixes you accept one by one or all at once.", shows: ["panel", "bubble", "inplace"] },
    { id: "rewrite", section: "theia-writing", label: "Rewrite", detail: "Rephrase, shorten, expand or simplify what you highlighted.", shows: ["panel", "bubble", "inplace"] },
    { id: "tone", section: "theia-writing", label: "Tone", detail: "Make text more formal, friendly, confident and more.", shows: ["panel", "bubble", "inplace"] },
    { id: "fluent", section: "theia-writing", label: "Fluent English", detail: "Smooth out phrasing so it reads naturally, with a simple English option.", shows: ["panel", "inplace"] },
    { id: "humanize", section: "theia-writing", label: "Sound like you", detail: "Rewrite stiff, generated-sounding text so it reads like a person wrote it.", shows: ["panel"] },
    { id: "aicheck", section: "theia-writing", label: "Machine-writing check", detail: "A rough read of how machine-written text sounds, with the phrases that give it away.", shows: ["panel"] },
    { id: "inclusive", section: "theia-writing", label: "Inclusive language", detail: "Flags wording that may exclude or hurt people, with kinder alternatives.", shows: ["panel"] },
    { id: "reactions", section: "theia-writing", label: "How it reads", detail: "How a reader is likely to take your message.", shows: ["panel"] },
    { id: "reply", section: "theia-writing", label: "Reply", detail: "Three ready replies to a message you highlighted.", shows: ["panel", "bubble"] },
    { id: "voice", section: "theia-writing", label: "My voice", detail: "Teach Rashin how you write so rewrites sound like you.", shows: ["settings", "panel"] },
    { id: "translatestyle", section: "theia-translate", label: "Translation style", detail: "Choose natural or literal, formal or casual, and see it translated back.", shows: ["panel"] },
    { id: "dictionary", section: "theia-translate", label: "Dictionary", detail: "Meanings of a word in the sense it is used, with examples.", shows: ["panel"] },
    { id: "overimage", section: "theia-translate", label: "Translate on the picture", detail: "Show the translation over the text in a captured picture.", shows: ["panel"] },
    { id: "quiet", section: "theia-quiet", label: "Quiet places", detail: "Apps where Theia never appears, and a Not now that keeps it away for a while.", shows: ["look", "highlight", "inplace", "panel"] },
    { id: "save", section: "theia", label: "Save to Rashin", detail: "Save answers and excerpts to a Rashin collection.", shows: ["panel"] }
];

var rows = [
    { id: "rashin-enabled", section: "rashin", label: "Use Rashin", detail: "Off. Rashin runs nothing at all until you turn it on", keywords: ["rashin", "assistant", "agent", "on", "off", "install", "remove", "opt in"], control: "custom", activatable: true },
    { id: "rashin-app-features", section: "rashin", label: "Rashin app features", detail: "Choose optional features in the Rashin app", keywords: ["rashin", "features", "compare", "research", "schedules", "prompts", "enhance", "askwriting", "analyze", "audio", "video", "share", "quote", "history", "searchsteps", "mindmap", "artifacts", "collections", "profile", "setupcards", "mcp", "diskcare"], control: "handoff", command: ["rashin", "open", "--tab", "settings", "--view", "features"], actionText: "Open in Rashin" },
    { id: "theia-on", section: "theia", label: "Theia", detail: "Point at anything on screen or highlight any text, and Rashin explains, translates, fixes or answers right there.", keywords: ["theia", "sight", "screen", "selection", "master", "gestures"], control: "custom", activatable: true, bind: sight("on", true), "default": true },
    { id: "sight-enabled", section: "theia", label: "Tap Right Alt three times", detail: "Then draw a box to explain, read or translate that part of the screen", keywords: ["theia", "sight", "screen", "tap", "shortcut", "explain", "translate", "ocr"], control: "toggle", bind: sight("enabled", true), "default": true },
    { id: "theia-o-tapKey", section: "theia", label: "Look key", detail: "Tap this key three times to choose part of the screen", keywords: ["theia", "sight", "look", "tap", "gesture", "key"], control: "picker", options: [], bind: sight("tapKey", "Alt_R"), "default": "Alt_R", when: { bind: sight("enabled", true), is: [true] } },
    { id: "sight-highlight", section: "theia", label: "Hold Left Alt while highlighting", detail: "Release the key to show Translate, Explain and Ask beside the highlighted text", keywords: ["theia", "sight", "text", "highlight", "hold", "translate", "explain", "ask"], control: "toggle", bind: sight("highlight", true), "default": true },
    { id: "theia-o-highlightKey", section: "theia", label: "Highlight key", detail: "Hold this key while selecting text", keywords: ["theia", "sight", "highlight", "hold", "gesture", "key"], control: "picker", options: [], bind: sight("highlightKey", "Alt_L"), "default": "Alt_L", when: { bind: sight("highlight", true), is: [true] } },
    { id: "sight-mode", section: "theia", label: "Start with", detail: "What appears beside the box the moment you release it", keywords: ["theia", "sight", "screen", "explain", "text", "translate", "ask"], control: "segmented", options: [{ label: "Explain", value: "explain" }, { label: "Text", value: "text" }, { label: "Translate", value: "translate" }, { label: "Ask", value: "ask" }], bind: sight("mode", "explain"), "default": "explain" },
    { id: "theia-o-bubble", section: "theia", label: "Highlight actions", detail: "Choose up to four actions and their order beside highlighted text", keywords: ["theia", "sight", "bubble", "highlight", "actions", "order"], control: "custom", bind: sight("bubble", ["translate", "explain", "ask"]), "default": ["translate", "explain", "ask"], when: { bind: sight("highlight", true), is: [true] } },
    { id: "theia-o-pinned", section: "theia", label: "Pinned tabs", detail: "Keep up to two extra tools beside Explain, Text, Translate and Ask", keywords: ["theia", "sight", "pin", "tabs", "panel"], control: "custom", bind: sight("pinned", []), "default": [], when: { bind: sight("on", true), is: [true] } },
    { id: "sight-shortcut", section: "theia", label: "Shortcuts", detail: "Choose the keys for screen boxes, highlighted text and in-place changes", keywords: ["theia", "sight", "screen", "tap", "hold", "shortcut", "keybind"], control: "custom", activatable: true },
    { id: "sight-status", section: "theia", label: "Theia status", detail: "Off", keywords: ["theia", "sight", "screen", "gestures", "keyboard", "access", "watching"] },
    { id: "theia-o-inplace.key", section: "theia", label: "In-place key", detail: "Tap this key three times with text selected", keywords: ["theia", "sight", "in place", "gesture", "key"], control: "picker", options: [], bind: sight("inplace.key", "Control_R"), "default": "Control_R", when: featureOn("inplace") },
    { id: "theia-o-inplace.action", section: "theia", label: "In-place action", detail: "What to do with selected text", keywords: ["theia", "sight", "in place", "replace", "action"], control: "picker", options: [{ label: "Fix or translate", value: "auto" }, { label: "Translate", value: "translate" }, { label: "Fix writing", value: "proofread" }, { label: "Rewrite", value: "rewrite" }, { label: "Fluent English", value: "fluent" }].concat(toneOptions), bind: sight("inplace.action", "auto"), "default": "auto", when: featureOn("inplace") },
    { id: "theia-o-inplace.paste", section: "theia", label: "Paste the result", detail: "Turn off to type the result instead", keywords: ["theia", "sight", "paste", "type", "clipboard", "replace"], control: "toggle", bind: sight("inplace.paste", true), "default": true, when: featureOn("inplace"), more: true },
    { id: "theia-o-summarize.style", section: "theia-reading", label: "Summary style", detail: "How summaries start", keywords: ["theia", "sight", "summary", "tldr", "points"], control: "segmented", options: [{ label: "Key points", value: "points" }, { label: "TL;DR", value: "tldr" }, { label: "Main point", value: "main" }], bind: sight("summarize.style", "points"), "default": "points", when: featureOn("summarize") },
    { id: "theia-o-deepread.lenses", section: "theia-reading", label: "Reading lenses", detail: "Choose which ways of reading appear in Deep read", keywords: ["theia", "sight", "fact check", "assumptions", "concepts", "contrasts", "related", "passages", "section"], control: "custom", bind: sight("deepread.lenses", ["factcheck", "assumptions", "concepts", "contrasts", "related", "passages", "section"]), "default": ["factcheck", "assumptions", "concepts", "contrasts", "related", "passages", "section"], when: featureOn("deepread") },
    { id: "theia-o-learn.count", section: "theia-reading", label: "Quiz length", detail: "Questions in each quiz", keywords: ["theia", "sight", "learn", "quiz", "questions"], control: "segmented", options: [{ label: "3", value: 3 }, { label: "5", value: 5 }, { label: "8", value: 8 }], bind: sight("learn.count", 5), "default": 5, when: featureOn("learn") },
    { id: "theia-o-proofread.engine", section: "theia-writing", label: "Writing checker", detail: "English on this computer, other languages with your model", keywords: ["theia", "sight", "proofread", "local", "grammar"], control: "segmented", options: [{ label: "Automatic", value: "auto" }, { label: "This computer", value: "local" }, { label: "Model", value: "model" }], bind: sight("proofread.engine", "auto"), "default": "auto", when: featureOn("proofread") },
    { id: "theia-o-proofread.dialect", section: "theia-writing", label: "English dialect", detail: "Spelling used by the writing checker", keywords: ["theia", "sight", "proofread", "spelling", "dialect"], control: "picker", options: [{ label: "United States", value: "us" }, { label: "United Kingdom", value: "uk" }, { label: "Canada", value: "ca" }, { label: "Australia", value: "au" }, { label: "India", value: "in" }], bind: sight("proofread.dialect", "us"), "default": "us", when: featureOn("proofread") },
    { id: "theia-o-proofread.off", section: "theia-writing", label: "Writing checks", detail: "Choose the kinds of writing issues to check", keywords: ["theia", "sight", "proofread", "grammar", "categories"], control: "custom", bind: sight("proofread.off", []), "default": [], when: featureOn("proofread"), more: true },
    { id: "theia-o-proofread.words", section: "theia-writing", label: "Your words", detail: "Words accepted by the checker on this computer", keywords: ["theia", "sight", "proofread", "dictionary", "spelling", "local"], control: "custom", when: featureOn("proofread"), more: true },
    { id: "theia-o-rewrite.variant", section: "theia-writing", label: "Rewrite style", detail: "How rewrites start", keywords: ["theia", "sight", "rewrite", "shorter", "longer", "simpler"], control: "picker", options: [{ label: "Rephrase", value: "rephrase" }, { label: "Shorter", value: "shorter" }, { label: "Longer", value: "longer" }, { label: "Simpler", value: "simpler" }], bind: sight("rewrite.variant", "rephrase"), "default": "rephrase", when: featureOn("rewrite") },
    { id: "theia-o-rewrite.audience", section: "theia-writing", label: "Writing audience", detail: "Who the rewrite is for", keywords: ["theia", "sight", "rewrite", "audience", "english learners"], control: "segmented", options: [{ label: "General", value: "general" }, { label: "Expert", value: "expert" }, { label: "English learners", value: "esl" }], bind: sight("rewrite.audience", "general"), "default": "general", when: featureOn("rewrite") },
    { id: "theia-o-tone.favorites", section: "theia-writing", label: "Favourite tones", detail: "Choose the tone chips offered for rewrites", keywords: ["theia", "sight", "tone", "formal", "friendly", "confident"], control: "custom", bind: sight("tone.favorites", ["formal", "friendly", "confident", "direct", "empathetic"]), "default": ["formal", "friendly", "confident", "direct", "empathetic"], when: featureOn("tone") },
    { id: "theia-o-voice", section: "theia-writing", label: "Your writing voice", detail: "Add samples and notes about how you write", keywords: ["theia", "sight", "my voice", "writing", "samples"], control: "custom", when: featureOn("voice") },
    { id: "theia-o-voice.use", section: "theia-writing", label: "Use my voice", detail: "Use your writing voice by default in rewrites", keywords: ["theia", "sight", "voice", "rewrite", "default"], control: "toggle", bind: sight("voice.use", true), "default": true, when: featureOn("voice") },
    { id: "sight-target", section: "theia-translate", label: "Translate into", detail: "The language used for translated screen text", keywords: ["theia", "sight", "screen", "translate", "language"], control: "picker", options: [], bind: sight("target", ""), "default": "" },
    { id: "sight-languages", section: "theia-translate", label: "Text recognition languages", detail: "Missing languages install when enabled. Turning one off keeps its files", keywords: ["theia", "sight", "screen", "ocr", "text", "tesseract", "languages", "install"], control: "custom", bind: sight("ocrLanguages", ["eng"]), "default": ["eng"] },
    { id: "theia-o-translate.style", section: "theia-translate", label: "Translation style", detail: "How translations start", keywords: ["theia", "sight", "translate", "natural", "literal", "formal", "casual"], control: "picker", options: [{ label: "Natural", value: "natural" }, { label: "Literal", value: "literal" }, { label: "Formal", value: "formal" }, { label: "Casual", value: "casual" }], bind: sight("translate.style", "natural"), "default": "natural", when: featureOn("translatestyle") },
    { id: "theia-o-translate.simpler", section: "theia-translate", label: "Simpler words", detail: "Prefer simpler wording in translations", keywords: ["theia", "sight", "translate", "simple", "words"], control: "toggle", bind: sight("translate.simpler", false), "default": false, when: featureOn("translatestyle") },
    { id: "theia-o-translate.back", section: "theia-translate", label: "Show it translated back", detail: "Translate the result back to help check its meaning", keywords: ["theia", "sight", "translate", "back translation", "meaning"], control: "toggle", bind: sight("translate.back", false), "default": false, when: featureOn("translatestyle"), more: true },
    { id: "theia-o-readaloud.voice", section: "theia-listening", label: "Reading voice", detail: "Choose a voice installed on this computer", keywords: ["theia", "sight", "read aloud", "voice", "espeak", "piper"], control: "custom", bind: sight("readaloud.voice", ""), "default": "", when: featureOn("readaloud") },
    { id: "theia-o-readaloud.rate", section: "theia-listening", label: "Reading speed", detail: "One is the normal speaking speed", keywords: ["theia", "sight", "read aloud", "rate", "speed"], control: "slider", min: 0.75, max: 1.5, step: 0.05, bind: sight("readaloud.rate", 1.0), "default": 1.0, when: featureOn("readaloud") },
    { id: "theia-o-quiet.apps", section: "theia-quiet", label: "Excluded apps", detail: "Theia never appears in these apps", keywords: ["theia", "sight", "quiet", "apps", "exclude", "privacy"], control: "custom", bind: sight("quiet.apps", []), "default": [], when: featureOn("quiet") },
    { id: "theia-o-quiet.snooze", section: "theia-quiet", label: "Not now lasts", detail: "How long Theia stays away after choosing Not now", keywords: ["theia", "sight", "quiet", "snooze", "not now"], control: "segmented", options: [{ label: "Until switching app", value: 0 }, { label: "15 minutes", value: 15 }, { label: "60 minutes", value: 60 }], bind: sight("quiet.snooze", 0), "default": 0, when: featureOn("quiet") },
    { id: "sight-local-only", section: "theia-engine", label: "Local models only", detail: "Only models on this computer may see your screen", keywords: ["theia", "sight", "screen", "privacy", "local", "offline", "models"], control: "toggle", bind: sight("localOnly", false), "default": false },
    { id: "sight-model", section: "theia-engine", label: "Model", detail: "Leave empty for Rashin's light model", keywords: ["theia", "sight", "screen", "model", "default"], control: "custom", bind: sight("model", ""), "default": "" },
    { id: "status-ask", section: "status", label: "Ask", detail: "How Ask picks a model for your questions", keywords: ["ask", "models", "route", "local providers", "chat"] },
    { id: "status-agent", section: "status", label: "Agent model", detail: "Default model for new Agent sessions", keywords: ["agent", "coding", "model", "local"] },
    { id: "status-index", section: "status", label: "Search index", detail: "What Rashin can search on this computer", keywords: ["index", "search", "projects", "files"] },
    { id: "rashin-open", section: "open", label: "Open Rashin", detail: "Ask, chat and work with agents in the Rashin app", keywords: ["app", "chat", "ask", "launch"], control: "handoff", command: ["rashin", "open"], actionText: "Open" },
    { id: "rashin-settings", section: "open", label: "Rashin's own settings", detail: "Models, providers, agents and indexes live in the Rashin app", keywords: ["providers", "models", "api keys", "ollama", "llama.cpp", "preferences"], control: "handoff", command: ["rashin", "open", "--tab", "settings"], actionText: "Open" }
];

features.forEach(function (feature) {
    var row = Object.assign({}, feature, {
        id: "theia-f-" + feature.id,
        feature: feature.id,
        control: "custom",
        activatable: true,
        keywords: ["theia", "sight", "rashin", feature.id, feature.label.toLowerCase()],
        bind: sight("features." + feature.id, false),
        "default": false
    });
    var firstOption = rows.findIndex(function (candidate) {
        return candidate.when && candidate.when.bind.store === row.bind.store && candidate.when.bind.key === row.bind.key;
    });
    if (firstOption >= 0)
        rows.splice(firstOption, 0, row);
    else {
        var end = rows.findIndex(function (candidate) {
            return sections.findIndex(function (section) { return section.id === candidate.section; }) > sections.findIndex(function (section) { return section.id === row.section; });
        });
        rows.splice(end < 0 ? rows.length : end, 0, row);
    }
});

var related = ["voice-typing", "default-apps", "add-ons"];
