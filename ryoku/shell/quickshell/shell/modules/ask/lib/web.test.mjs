import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const W = require("./web.js");
const F = require("./format.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

const sources = [
    { id: "suggest", name: "Suggestions", scope: "web", glyph: "search", typing: true },
    { id: "web", name: "Web", scope: "web", glyph: "travel_explore", typing: false },
    { id: "dictionary", name: "Dictionary", scope: "knowledge", glyph: "dictionary", typing: true },
    { id: "wikipedia", name: "Wikipedia", scope: "knowledge", glyph: "menu_book", typing: true },
    { id: "github", name: "GitHub", scope: "code", glyph: "code", typing: false },
    { id: "npm", name: "npm", scope: "code", glyph: "deployed_code", typing: false },
    { id: "youtube", name: "YouTube", scope: "video", glyph: "smart_display", typing: false }
];
const engines = [
    { id: "keenable", name: "Keenable", sources: ["web"], state: "ok", until: "" },
    { id: "ddg-lite", name: "DuckDuckGo", sources: ["web"], state: "paused", until: "2026-11-18T15:20:00-05:00" },
    { id: "bing-rss", name: "Bing", sources: ["web"], state: "ok", until: "" },
    { id: "ddg-ac", name: "DuckDuckGo", sources: ["suggest"], state: "paused", until: "2026-11-18T15:20:00-05:00" },
    { id: "github", name: "GitHub", sources: ["github"], state: "paused", until: "2026-11-18T15:05:00-05:00" },
    { id: "freedictionary", name: "FreeDictionaryAPI", sources: ["dictionary"], state: "ok", until: "" }
];
const page = (key, extra) => ({ key, kind: "page", title: key, url: "https://x.test/" + key, ...extra });
const hint = text => ({ key: text, kind: "suggestion", title: text, url: "" });
const sense = (key, text) => ({ key, kind: "definition", title: "tbh", snippet: text, url: "https://en.wiktionary.org/wiki/tbh", meta: { partOfSpeech: "phrase", attribution: "Wiktionary, CC BY-SA 4.0" } });

// the full-search trigger
equal(W.ask("paused", { text: "nvi", fullText: "" }), { mode: "suggest", text: "nvi" }, "a rest in typing asks for suggestions");
equal(W.ask("paused", { text: " nvidia ", fullText: "nvidia" }), null, "a rest on text already searched asks nothing");
equal(W.ask("paused", { text: "", fullText: "nvidia" }), { mode: "suggest", text: "" }, "an emptied field clears the search");
equal(W.ask("still", { text: "nvidia wayland", fullText: "" }), { mode: "full", text: "nvidia wayland" }, "stillness runs the full search");
equal(W.ask("still", { text: "nv", fullText: "" }), null, "stillness needs three characters");
equal(W.ask("still", { text: "nvi", fullText: "" }), { mode: "full", text: "nvi" }, "three characters are enough");
equal(W.ask("still", { text: "nvidia", fullText: "nvidia" }), null, "stillness never repeats a full search");
equal(W.ask("still", { text: "nvidia", fullText: "", choosing: true }), null, "stillness waits while the hand moves through the list");
equal(W.ask("enter", { text: "nv", fullText: "" }), { mode: "full", text: "nv" }, "Enter runs the full search at any length");
equal(W.ask("enter", { text: "  ", fullText: "" }), null, "Enter on nothing asks nothing");
equal(W.ask("picked", { text: "nvi", picked: "nvidia drivers", fullText: "" }), { mode: "full", text: "nvidia drivers" }, "a picked suggestion is searched in full");
equal(W.ask("opened", { text: "tbh", fullText: "" }), { mode: "full", text: "tbh" }, "a view pushed with a query searches at once");
equal(W.ask("scope", { text: "nvidia", fullText: "nvidia" }), { mode: "full", text: "nvidia" }, "a new scope repeats the full search");
equal(W.ask("scope", { text: "nvid", fullText: "nvidia" }), { mode: "suggest", text: "nvid" }, "a new scope while typing stays with suggestions");

// the two timers, as the view runs them: each keystroke restarts both
function type(keys) {
    const asked = [];
    let fullText = "";
    let text = "";
    let suggestAt = Infinity;
    let stillAt = Infinity;
    const fire = (event, at) => {
        const want = W.ask(event, { text, fullText });
        if (!want)
            return;
        if (want.mode === "full")
            fullText = want.text;
        asked.push([at, want.mode, want.text]);
    };
    const until = at => {
        for (;;) {
            const next = Math.min(suggestAt, stillAt);
            if (next === Infinity || next > at)
                return;
            if (next === suggestAt) {
                suggestAt = Infinity;
                fire("paused", next);
            } else {
                stillAt = Infinity;
                fire("still", next);
            }
        }
    };
    for (const [at, value] of keys) {
        until(at);
        if (value === "\n") {
            suggestAt = stillAt = Infinity;
            fire("enter", at);
            continue;
        }
        text = value;
        suggestAt = at + W.SUGGEST_MS;
        stillAt = at + W.STILL_MS;
    }
    until(Infinity);
    return asked;
}
equal(type([[0, "n"], [100, "nv"], [200, "nvi"], [500, "nvid"], [1500, "nvidi"], [2600, "nvidia"]]),
    [[370, "suggest", "nvi"], [670, "suggest", "nvid"], [1670, "suggest", "nvidi"], [2770, "suggest", "nvidia"], [3800, "full", "nvidia"]],
    "text still being extended only ever gets suggestions; the full search waits for 1.2 s of stillness");
equal(type([[0, "nvidia"], [300, "\n"]]), [[170, "suggest", "nvidia"], [300, "full", "nvidia"]], "Enter does not wait");
equal(type([[0, "nvidia"], [80, "\n"]]), [[80, "full", "nvidia"]], "Enter before the first rest sends one query");
equal(type([[0, "hi"]]), [[170, "suggest", "hi"]], "two characters never search in full on their own");

equal(W.langTag("en_US.UTF-8", "en"), "en-US", "the locale goes whole");
equal(W.langTag("pt_BR", "pt_BR"), "pt-BR", "a regional catalog is the locale's own language");
equal(W.langTag("en_US", "pl"), "pl", "a chosen language that is not the locale's goes alone");
equal(W.langTag("C", ""), "en", "no locale is English");
equal(W.langTag("de", ""), "de", "a bare language stays bare");

equal(W.scopes(sources), ["all", "web", "knowledge", "code", "video"], "scopes follow the fixed order and only list what exists");
equal(W.scopes([{ id: "x", scope: "maps" }, { id: "y", scope: "web" }]), ["all", "web", "maps"], "an unknown scope goes last");
equal(W.sourcesIn(sources, "code").map(s => s.id), ["github", "npm"], "a scope holds its sources");
equal(W.sourcesIn(sources, "youtube").map(s => s.id), ["youtube"], "a source id is a scope of one");
equal(W.sourcesIn(sources, "all").length, 7, "all holds every source");
equal(W.feeding(engines, "web").map(e => e.id), ["keenable", "ddg-lite", "bing-rss"], "the web lane's engines, in the daemon's order");

const states = {
    "web": { state: "done", items: [page("w1", { meta: { engines: ["keenable", "ddg-lite"] } }), page("w2", { meta: { engines: ["keenable"] } }), page("w3"), page("w4"), page("w5")], more: true, rank: 1 },
    "dictionary": { state: "done", items: [sense("s1", "Initialism of to be honest."), sense("s2", "Thurawal.")], rank: 9 },
    "wikipedia": { state: "loading", items: [] },
    "github": { state: "error", items: [], error: "Timed out" },
    "npm": { state: "waiting", items: [page("n1")], rank: 2 },
    "youtube": { state: "paused", items: [], until: "2026-11-18T15:40:00-05:00" }
};

const all = W.progress(sources, states, "all", engines);
equal([all.total, all.settled, all.failed, all.loading, all.waiting, all.paused, all.found], [7, 4, 1, 1, 1, 1, 7],
    "progress counts what settled, failed, is out, is held and is paused, and what was found");
equal(all.ticks.map(t => t.id + "=" + t.state),
    ["suggest=idle", "web:keenable=done", "web:ddg-lite=paused", "web:bing-rss=idle", "dictionary=done", "wikipedia=loading", "github=error", "npm=waiting", "youtube=paused"],
    "one tick per source, and one per engine of the web lane: filled when it found a row, paused while blocked");
equal(W.progress(sources, { "web": { state: "loading" } }, "web", engines).ticks.map(t => t.state), ["idle", "loading", "paused", "loading"], "while the lane is out its free engines are out");
equal(W.progress(sources, { "web": { state: "error" } }, "web", engines).ticks.map(t => t.state), ["idle", "error", "paused", "error"], "a failed lane fails its free engines");
equal(W.progress(sources, states, "web", []).ticks.map(t => t.id), ["suggest", "web"], "with no engine list the lane is one tick");
equal(W.progress(sources, states, "knowledge", engines).fraction, 0.5, "a scope is as far as its sources");
equal(W.progress([], {}, "all", engines).fraction, 0, "no sources is no progress");

// the section order
const ids = sections => sections.map(s => s.id);
const typed = {
    "suggest": { state: "done", items: [hint("tbh meaning"), hint("TBH"), hint("tbh slang"), hint("tbh full form"), hint("tbh and idk"), hint("tbh app"), hint("tbh ice cream")] },
    "dictionary": states.dictionary,
    "wikipedia": { state: "done", items: [page("TBH")], rank: 3 },
    "web": states.web
};
const whileTyping = W.plan({ sources, states: typed, scope: "all", cap: 3, query: "tbh", typing: true, flying: false });
equal(ids(whileTyping), ["suggestions", "definition", "wikipedia"], "while typing: suggestions, the definition, then what answers as you type; nothing from an older full search");
equal(whileTyping[0].rows.map(r => r.kind === "query" ? "=" + r.text : r.item.title), ["=tbh", "tbh meaning", "tbh slang", "tbh full form", "tbh and idk", "tbh app"],
    "the typed text leads its suggestions, is not repeated among them, and five follow");
equal([whileTyping[1].rows.length, whileTyping[1].total, whileTyping[1].more, whileTyping[1].source.id], [1, 2, true, "dictionary"], "all shows one sense and knows there are more");
equal(ids(W.plan({ sources, states: {}, scope: "all", query: "tbh", typing: true })), ["suggestions"], "before anything answers the typed text is already there to search");
equal(W.plan({ sources, states: typed, scope: "all", query: "", typing: false }).map(s => s.id), ["definition", "web", "wikipedia"], "an empty field has no suggestions");

const ranked = {
    "suggest": typed.suggest,
    "web": { state: "done", items: [page("w1")], rank: 1 },
    "dictionary": { state: "done", items: [sense("s1", "Initialism of to be honest.")], rank: 0 },
    "wikipedia": { state: "done", items: [page("k1")], rank: 2 },
    "github": { state: "done", items: [page("g1")], rank: 5 },
    "npm": { state: "done", items: [], more: true, rank: 0 },
    "youtube": { state: "done", items: [page("y1")], rank: 2 }
};
equal(ids(W.plan({ sources, states: ranked, scope: "all", cap: 3, query: "tbh", typing: false, flying: false })), ["definition", "web", "github", "wikipedia", "youtube"],
    "a full search: the definition, the web lane whatever its rank, then the rest by rank with ties in the daemon's order; a source with no rows is not listed");
equal(ids(W.plan({ sources, states: ranked, scope: "all", query: "tbh", typing: false, flying: true })), ["definition", "web", "github", "wikipedia", "youtube"], "suggestions leave once a full search has a row");
equal(ids(W.plan({ sources, states: { "suggest": typed.suggest, "web": { state: "loading", items: [] } }, scope: "all", query: "tbh", typing: false, flying: true })), ["suggestions"],
    "a full search with nothing in yet keeps the suggestions in sight");
equal(ids(W.plan({ sources, states: { "suggest": typed.suggest, "web": { state: "done", items: [] } }, scope: "all", query: "tbh", typing: false, flying: false })), [], "a full search that found nothing lists nothing");
equal(ids(W.plan({ sources, states: ranked, scope: "code", cap: 3, query: "tbh", typing: false })), ["github"], "a scope keeps the daemon's order");

const grouped = W.plan({ sources, states, scope: "all", cap: 3, query: "tbh", typing: false, flying: true });
equal(grouped.map(s => [s.id, s.rows.length, s.total, s.more]), [["definition", 1, 2, true], ["web", 3, 5, true], ["npm", 1, 1, false]], "all caps each source and keeps a held source's last rows");
equal(grouped[1].rows[0].id, "web:w1", "a row id joins the source and the item key");
const inWeb = W.plan({ sources, states, scope: "web", cap: 3, query: "tbh", typing: false });
equal(inWeb.map(s => [s.id, s.rows.length, s.more]), [["web", 5, true]], "a scope shows every row and keeps the source's more flag");
equal(W.plan({ sources, states, scope: "dictionary", query: "tbh", typing: false }).map(s => [s.id, s.rows.length, s.more]), [["definition", 2, false]], "the dictionary on its own shows every sense");

// every web engine failed or is paused
const blocked = { ...ranked, "web": { state: "paused", items: [], until: "2026-11-18T15:20:00-05:00" } };
const stuck = W.plan({ sources, states: blocked, scope: "all", cap: 3, query: "tbh", typing: false, flying: false });
equal(ids(stuck), ["definition", "fallback", "github", "wikipedia", "youtube"], "a paused lane leaves two ways on in its place");
equal([stuck[1].title, stuck[1].state, stuck[1].until, stuck[1].rows.map(r => r.target)], ["Web", "paused", "2026-11-18T15:20:00-05:00", ["ask", "browser"]], "ask Hyakki, then the browser, and until when");
const broken = W.plan({ sources, states: { "web": { state: "error", items: [], error: "No engine answered" } }, scope: "all", query: "tbh", typing: false, flying: false });
equal([ids(broken), broken[0].state, broken[0].error], [["fallback"], "error", "No engine answered"], "a failed lane does the same with its reason");
equal(ids(W.plan({ sources, states: blocked, scope: "all", query: "tbh more", typing: true })), ["suggestions", "definition", "wikipedia"], "typing on does not show the ways on: no web engine runs for it");
equal(ids(W.plan({ sources, states: blocked, scope: "code", query: "tbh", typing: false })), ["github"], "another scope is not the lane's business");
const gone = W.plan({ sources: [], states: {}, scope: "all", query: "tbh", typing: true, available: false });
equal([ids(gone), gone[0].state, gone[0].rows.map(r => r.target)], [["fallback"], "unavailable", ["ask", "browser"]], "with no search service the same two ways stay");
equal(W.plan({ sources: [], states: {}, scope: "all", query: "", available: false }), [], "and nothing without a query");

equal(W.failures(sources, states, "all"), [{ id: "github", name: "GitHub", error: "Timed out" }], "failed sources are named with their reason");
equal(W.failures(sources, states, "web"), [], "a scope only reports its own failures");

equal(W.paused(sources, engines, "all"), [{ id: "github", name: "GitHub", until: "2026-11-18T15:05:00-05:00" }, { id: "ddg-lite", name: "DuckDuckGo", until: "2026-11-18T15:20:00-05:00" }],
    "resting engines are named once each, the soonest back first");
equal(W.paused(sources, engines, "code").map(e => e.name), ["GitHub"], "a scope only names the engines behind it");
equal(W.paused(sources, [], "all"), [], "no engine list, nothing to name");
equal(W.foundBy(states.web.items[0], engines), ["Keenable", "DuckDuckGo"], "a merged row names the engines that found it");
equal(W.foundBy(page("x", { meta: { engines: ["searx"] } }), engines), ["searx"], "an engine with no name shows its id");
equal(W.foundBy(page("x"), engines), [], "a row from elsewhere names none");

equal([
    W.downloadable({ kind: "video", url: "https://y.test/watch?v=1" }),
    W.downloadable({ kind: "page", url: "https://x.test/a.iso" }),
    W.downloadable({ kind: "page", url: "https://x.test/a.tar.zst?sig=1" }),
    W.downloadable({ kind: "page", url: "https://x.test/readme" }),
    W.downloadable({ kind: "package", url: "https://x.test/p", meta: { download: "https://x.test/p.tgz" } }),
    W.downloadable(null)
], [true, true, true, false, true, false], "videos, files and results with a download link can be downloaded");
equal(W.downloadUrl({ url: "https://x.test/p", meta: { download: "https://x.test/p.tgz" } }), "https://x.test/p.tgz", "a download link in meta wins");
equal(W.downloadUrl({ url: "https://x.test/a.iso", meta: {} }), "https://x.test/a.iso", "else the result's own address");

equal(W.altLinks({ url: "https://a.test", meta: { discussion: "https://news.test/item?id=1", homepage: "https://a.test", repository: "git+ssh://x", documentation: "https://docs.test" } }),
    [{ id: "discussion", url: "https://news.test/item?id=1" }, { id: "documentation", url: "https://docs.test" }], "alt links are web addresses other than the result itself");

equal(W.facts({ meta: { stars: 12345, language: "Go", forks: 3 } }, F.count), [{ glyph: "star", text: "12k" }, { glyph: "", text: "Go" }], "a repository shows stars and language");
equal(W.facts({ meta: { score: 12, answers: 3, accepted: true } }, F.count), [{ glyph: "arrow_upward", text: "12" }, { glyph: "check", text: "3" }], "an accepted question marks its answers");
equal(W.facts({ meta: { points: 321, comments: 45 } }, F.count), [{ glyph: "arrow_upward", text: "321" }, { glyph: "forum", text: "45" }], "a thread shows points and comments");
equal(W.facts({ meta: { version: "1.2.3", weeklyDownloads: 1500000 } }, F.count), [{ glyph: "", text: "1.2.3", mono: true }, { glyph: "download", text: "1.5M" }], "a package shows its version and downloads");
equal(W.facts({ meta: { durationText: "12:34", views: -1 } }, F.count), [{ glyph: "", text: "12:34" }], "a video shows its running time");
equal(W.facts({}, F.count), [], "no meta, no facts");

equal(W.browserSearchUrl("a b&c"), "https://duckduckgo.com/?q=a%20b%26c", "the browser search address is encoded");
