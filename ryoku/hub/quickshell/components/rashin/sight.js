.pragma library

var languages = [
    { code: "eng", label: "English" },
    { code: "jpn", label: "Japanese" },
    { code: "kor", label: "Korean" },
    { code: "chi_sim", label: "Chinese (Simplified)" },
    { code: "chi_tra", label: "Chinese (Traditional)" },
    { code: "deu", label: "German" },
    { code: "fra", label: "French" },
    { code: "spa", label: "Spanish" },
    { code: "ita", label: "Italian" },
    { code: "por", label: "Portuguese" },
    { code: "rus", label: "Russian" },
    { code: "ara", label: "Arabic" },
    { code: "hin", label: "Hindi" },
    { code: "vie", label: "Vietnamese" },
    { code: "tha", label: "Thai" },
    { code: "tur", label: "Turkish" },
    { code: "pol", label: "Polish" },
    { code: "nld", label: "Dutch" },
    { code: "ukr", label: "Ukrainian" }
];

function targetOptions(langs, tr) {
    return [{ label: tr("Your language"), value: "" }].concat(langs.map(l => {
        var label = tr(l.name);
        return { label: label + (l.native && l.native !== label ? " · " + l.native : ""), value: l.code };
    }));
}

function targetCode(value, langs) {
    var code = value === "zh-Hans" ? "zh_CN" : value === "zh-Hant" ? "zh_TW" : value;
    return langs.some(l => l.code === code) ? code : "";
}

function missingLanguages(selected, installed) {
    return selected.filter((code, index) => selected.indexOf(code) === index
        && installed.indexOf(code) < 0 && languages.some(l => l.code === code));
}

function modelSuggestions(payload, query) {
    var data = typeof payload === "string" ? JSON.parse(payload) : payload;
    var seen = Object.create(null);
    var search = String(query || "").toLowerCase();
    return (data.routes || []).concat((data.models || []).filter(m => m.available === true))
        .filter(m => {
            if (!m.id || seen[m.id])
                return false;
            seen[m.id] = true;
            return !search || (m.id + " " + (m.name || "") + " " + (m.provider || "")).toLowerCase().indexOf(search) >= 0;
        }).slice(0, 100).map(m => ({ id: m.id, name: m.name ? m.name + " · " + m.id : m.id }));
}

function modelUnavailable(payload, id, providers) {
    var data = typeof payload === "string" ? JSON.parse(payload) : payload;
    if (!id || (data.routes || []).some(route => route.id === id))
        return "";
    var model = (data.models || []).find(m => m.id === id);
    if (model && model.available === true)
        return "";
    var provider = (providers || []).find(p => p.id === (model ? model.provider : id.split("/")[0]));
    return provider && provider.configured === false ? "no-key" : "unavailable";
}

function modelIdentity(payload, id, providers) {
    var data = typeof payload === "string" ? JSON.parse(payload) : payload;
    var model = (data.models || []).find(m => m.id === id);
    var providerID = model && model.provider || id.split("/")[0];
    var provider = (providers || []).find(p => p.id === providerID);
    return { model: model && model.name || id, provider: provider && provider.name || providerID };
}

var featureIDs = [
    "inplace", "summarize", "deepread", "math", "learn", "readaloud", "proofread",
    "rewrite", "tone", "fluent", "humanize", "aicheck", "inclusive", "reactions",
    "reply", "voice", "translatestyle", "dictionary", "overimage", "quiet", "save"
];

var rashinFeatureIDs = [
    "compare", "research", "schedules", "prompts", "enhance", "askwriting",
    "analyze", "audio", "video", "share", "quote", "history", "searchsteps",
    "mindmap", "artifacts", "collections", "profile", "setupcards",
    "mcp", "diskcare"
];

function defaults() {
    var values = {
        on: true, enabled: true, tapKey: "Alt_R", highlightKey: "Alt_L",
        highlight: true, mode: "explain", target: "", ocrLanguages: ["eng"],
        localOnly: false, engine: "ask", model: "",
        bubble: ["translate", "explain", "ask"], pinned: [],
        "summarize.style": "points",
        "deepread.lenses": ["factcheck", "assumptions", "concepts", "contrasts", "related", "passages", "section"],
        "learn.count": 5, "readaloud.voice": "", "readaloud.rate": 1.0,
        "proofread.engine": "auto", "proofread.dialect": "us", "proofread.off": [],
        "rewrite.variant": "rephrase", "rewrite.audience": "general",
        "tone.favorites": ["formal", "friendly", "confident", "direct", "empathetic"],
        "voice.use": true, "translate.style": "natural", "translate.simpler": false,
        "translate.back": false, "inplace.key": "Control_R", "inplace.action": "auto",
        "inplace.paste": true, "quiet.apps": [], "quiet.snooze": 0
    };
    featureIDs.forEach(function (id) { values["features." + id] = false; });
    return values;
}

function readFeatures(payload) {
    var settings = payload && payload.settings ? payload.settings : payload;
    if (!settings || typeof settings !== "object" || Array.isArray(settings))
        throw new Error("Rashin settings could not be read");
    var source = settings.features || {};
    if (typeof source !== "object" || Array.isArray(source))
        throw new Error("Rashin features could not be read");
    var values = {};
    rashinFeatureIDs.forEach(function (id) { values[id] = source[id] === true; });
    return values;
}

function once(done) {
    var finished = false;
    return function (ok, error) {
        if (finished)
            return;
        finished = true;
        if (done)
            done(ok, error || "");
    };
}
