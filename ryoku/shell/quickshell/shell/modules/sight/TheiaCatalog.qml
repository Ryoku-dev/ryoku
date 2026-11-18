pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons

QtObject {
    id: root

    property RyokuUi.ShellSetting _settings: RyokuUi.ShellSetting {
        path: "sight"
        fallback: ({})
    }

    readonly property bool on: option("on", true) !== false
    readonly property var actions: [
        { id: "explain", feature: "", group: "core", label: I18n.tr("Explain"), hint: I18n.tr("Understand what you are looking at"), icon: "lightbulb", view: "thread", needs: "any", replaces: false },
        { id: "text", feature: "", group: "core", label: I18n.tr("Text"), hint: I18n.tr("Read and copy text from the picture"), icon: "text_fields", view: "text", needs: "image", replaces: true },
        { id: "translate", feature: "", group: "core", label: I18n.tr("Translate"), hint: I18n.tr("Read it in another language"), icon: "translate", view: "thread", needs: "any", replaces: true },
        { id: "ask", feature: "", group: "core", label: I18n.tr("Ask"), hint: I18n.tr("Ask Rashin about what you selected"), icon: "chat_bubble", view: "thread", needs: "any", replaces: false },
        { id: "summarize", feature: "summarize", group: "reading", label: I18n.tr("Summarize"), hint: I18n.tr("Get the key points or a short summary"), icon: "summarize", view: "summary", needs: "any", replaces: true },
        { id: "deepread", feature: "deepread", group: "reading", label: I18n.tr("Deep read"), hint: I18n.tr("Check claims, ideas and assumptions"), icon: "manage_search", view: "lenses", needs: "any", replaces: false },
        { id: "solve", feature: "math", group: "reading", label: I18n.tr("Solve"), hint: I18n.tr("Read equations and solve them step by step"), icon: "function", view: "solve", needs: "any", replaces: false },
        { id: "learn", feature: "learn", group: "reading", label: I18n.tr("Learn"), hint: I18n.tr("Turn what you read into a short quiz"), icon: "school", view: "quiz", needs: "any", replaces: false },
        { id: "proofread", feature: "proofread", group: "writing", label: I18n.tr("Fix writing"), hint: I18n.tr("Check spelling and grammar"), icon: "spellcheck", view: "issues", needs: "text", replaces: true },
        { id: "rewrite", feature: "rewrite", group: "writing", label: I18n.tr("Rewrite"), hint: I18n.tr("Rephrase, shorten, expand or simplify"), icon: "edit_note", view: "rewrite", needs: "text", replaces: true },
        { id: "tone", feature: "tone", group: "writing", label: I18n.tr("Tone"), hint: I18n.tr("Change how your words come across"), icon: "tune", view: "rewrite", needs: "text", replaces: true },
        { id: "fluent", feature: "fluent", group: "writing", label: I18n.tr("Fluent"), hint: I18n.tr("Make English phrasing read naturally"), icon: "auto_fix_high", view: "rewrite", needs: "text", replaces: true },
        { id: "humanize", feature: "humanize", group: "writing", label: I18n.tr("Sound like you"), hint: I18n.tr("Make stiff writing sound more natural"), icon: "person_edit", view: "rewrite", needs: "text", replaces: true },
        { id: "aicheck", feature: "aicheck", group: "writing", label: I18n.tr("Machine check"), hint: I18n.tr("Get a rough guide to machine-written phrasing"), icon: "radar", view: "score", needs: "text", replaces: false },
        { id: "inclusive", feature: "inclusive", group: "writing", label: I18n.tr("Inclusive"), hint: I18n.tr("Find wording that may exclude people"), icon: "diversity_3", view: "issues", needs: "text", replaces: true },
        { id: "reactions", feature: "reactions", group: "writing", label: I18n.tr("How it reads"), hint: I18n.tr("See how a reader might take your message"), icon: "visibility", view: "thread", needs: "text", replaces: false },
        { id: "reply", feature: "reply", group: "writing", label: I18n.tr("Reply"), hint: I18n.tr("Choose from three replies to a message"), icon: "reply", view: "rewrite", needs: "text", replaces: false },
        { id: "define", feature: "dictionary", group: "translate", label: I18n.tr("Dictionary"), hint: I18n.tr("Find a word's meaning in this context"), icon: "dictionary", view: "define", needs: "text", replaces: false }
    ]

    readonly property var tones: [
        { id: "formal", label: I18n.tr("Formal") },
        { id: "confident", label: I18n.tr("Confident") },
        { id: "friendly", label: I18n.tr("Friendly") },
        { id: "diplomatic", label: I18n.tr("Diplomatic") },
        { id: "direct", label: I18n.tr("Direct") },
        { id: "empathetic", label: I18n.tr("Empathetic") },
        { id: "assertive", label: I18n.tr("Assertive") },
        { id: "constructive", label: I18n.tr("Constructive") },
        { id: "engaging", label: I18n.tr("Engaging") },
        { id: "exciting", label: I18n.tr("Exciting") },
        { id: "informative", label: I18n.tr("Informative") },
        { id: "inspirational", label: I18n.tr("Inspirational") },
        { id: "objective", label: I18n.tr("Objective") },
        { id: "persuasive", label: I18n.tr("Persuasive") },
        { id: "academic", label: I18n.tr("Academic") },
        { id: "casual", label: I18n.tr("Casual") },
        { id: "compassionate", label: I18n.tr("Compassionate") },
        { id: "cooperative", label: I18n.tr("Cooperative") },
        { id: "fluent", label: I18n.tr("Fluent") },
        { id: "professional", label: I18n.tr("Professional") },
        { id: "witty", label: I18n.tr("Witty") },
        { id: "personable", label: I18n.tr("Personable") },
        { id: "warm", label: I18n.tr("Warm") }
    ]

    readonly property var lenses: [
        { id: "factcheck", label: I18n.tr("Fact check"), hint: I18n.tr("Which claims hold up and which need checking") },
        { id: "assumptions", label: I18n.tr("Assumptions"), hint: I18n.tr("What the writer takes for granted") },
        { id: "concepts", label: I18n.tr("Key ideas"), hint: I18n.tr("The ideas you need to understand this") },
        { id: "contrasts", label: I18n.tr("Contrasts"), hint: I18n.tr("Where ideas agree or differ") },
        { id: "related", label: I18n.tr("Related reading"), hint: I18n.tr("Titles and topics worth looking up next") },
        { id: "passages", label: I18n.tr("Key passages"), hint: I18n.tr("The passages that matter most and why") },
        { id: "section", label: I18n.tr("This section"), hint: I18n.tr("How this part fits into the whole") }
    ]

    readonly property var pinned: validatedIds(option("pinned", []), 2, true)
    readonly property var bubble: validatedIds(option("bubble", ["translate", "explain", "ask"]), 4, false)

    function option(key, fallback) {
        const parts = String(key).split(".");
        let value = _settings.value;
        for (let i = 0; i < parts.length; ++i) {
            if (!value || typeof value !== "object" || value[parts[i]] === undefined)
                return fallback;
            value = value[parts[i]];
        }
        return value === undefined || value === null ? fallback : value;
    }

    function action(id) {
        for (let i = 0; i < actions.length; ++i)
            if (actions[i].id === id) return actions[i];
        return null;
    }

    function feature(id) {
        const features = option("features", ({}));
        return on && features && features[id] === true;
    }

    function enabled(id) {
        const row = action(id);
        return !!row && (row.group === "core" ? on : feature(row.feature));
    }

    function available(card) {
        if (!card) return [];
        const image = card.kind === "image";
        const text = image || String(card.text || "").trim().length > 0;
        return actions.filter(function(row) {
            return root.enabled(row.id) && (row.needs === "any" || (row.needs === "image" && image) || (row.needs === "text" && text));
        });
    }

    function validatedIds(ids, maximum, extraOnly) {
        const result = [];
        if (!ids || typeof ids !== "object" || typeof ids.length !== "number")
            return result;
        for (let i = 0; i < ids.length && result.length < maximum; ++i) {
            const id = ids[i];
            const row = action(id);
            if (!row || !enabled(id) || (extraOnly && row.group === "core") || result.indexOf(id) >= 0)
                continue;
            result.push(id);
        }
        return result;
    }

    function setPinned(ids) {
        return ShellSettings.patch("sight.pinned", validatedIds(ids, 2, true));
    }

    function setBubble(ids) {
        return ShellSettings.patch("sight.bubble", validatedIds(ids, 4, false));
    }
}
