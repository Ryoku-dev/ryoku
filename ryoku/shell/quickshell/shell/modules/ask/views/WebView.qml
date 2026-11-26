pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import ".."
import "../bar"
import "../look"
import "web"
import "../lib/web.js" as Web
import "../lib/format.js" as Format

// Web search in place. While you type, only suggestions, the dictionary and
// title lookups answer; the full search (every source, the web engines among
// them) runs on Enter, on a picked suggestion, or once the hand has rested. The
// strip under the field scopes the search and shows each source arriving.
// args: {query} searches at once.
Item {
    id: root

    required property real s
    property bool active: false
    property string query: ""
    property var args: ({})
    property real maximumHeight: 480 * s
    property SearchClient search: null
    property DownloadsClient downloads: null

    readonly property string title: I18n.tr("Web")
    readonly property string placeholder: I18n.tr("Search the web, code, packages and videos")
    readonly property real preferredWidth: root.detailOpen ? Math.round(1000 * root.s) : 0

    property string scope: "all"
    property bool detailOpen: false
    // what the last full search ran for; any other text is still being typed
    property string fullText: ""
    // the args.query already searched
    property string openedFor: ""
    // a list that a rest just replaced is not yet what Enter was aimed at
    property real settleAt: 0
    property real now: Date.now()

    readonly property string text: root.query.trim()
    readonly property bool typing: root.text !== root.fullText
    readonly property bool available: root.search !== null && root.search.state !== "unavailable"
    readonly property var sources: root.search ? root.search.sources : []
    readonly property var engines: root.search ? root.search.engines : []
    readonly property var answers: root.search ? root.search.states : ({})
    readonly property bool flying: root.search ? !root.search.settled : false
    readonly property var progress: Web.progress(root.sources, root.answers, root.scope, root.engines)
    readonly property var failures: Web.failures(root.sources, root.answers, root.scope)
    // while typing only the engines that answer as you type are worth a word
    readonly property var resting: Web.paused(root.typing ? root.sources.filter(source => source.typing === true) : root.sources, root.engines, root.scope)
        .filter(engine => !(Date.parse(engine.until) <= root.now))

    readonly property var scopeNames: ({
        "all": I18n.tr("All"), "web": I18n.tr("Web"), "knowledge": I18n.tr("Knowledge"), "code": I18n.tr("Code"),
        "linux": I18n.tr("Linux"), "video": I18n.tr("Video"), "discuss": I18n.tr("Discussion")
    })
    readonly property var scopes: {
        const ids = Web.scopes(root.sources);
        const out = ids.map(id => ({ id: id, label: root.scopeNames[id] || id }));
        // one source opened on its own sits at the end of the strip
        if (ids.indexOf(root.scope) < 0) {
            const own = root.sources.filter(source => source.id === root.scope);
            if (own.length > 0)
                out.push({ id: root.scope, label: String(own[0].name || own[0].id) });
        }
        return out;
    }

    readonly property var plan: Web.plan({
        sources: root.sources, states: root.answers, scope: root.scope, cap: 3, query: root.text,
        typing: root.typing, flying: root.flying, available: root.available
    })
    readonly property var sections: root.plan.map(section => ({
        id: section.id,
        kind: section.kind,
        // a source that is the scope itself is named by the strip already
        title: section.kind === "fallback" ? (section.title.length > 0 ? section.title : I18n.tr("Web"))
            : section.kind === "source" && String(section.source.id) === root.scope ? "" : section.title,
        name: section.source ? String(section.source.name || section.source.id) : "",
        scope: section.source ? String(section.source.id) : "",
        note: root.sectionNote(section),
        more: section.more === true,
        failed: section.kind === "fallback",
        rows: section.rows.map((entry, index) => root.resultRow(entry, index === section.rows.length - 1))
    }))
    readonly property bool stranded: root.sections.some(section => section.failed)
    readonly property int found: root.sections.reduce((sum, section) => sum + (section.kind === "source" || section.kind === "definition" ? section.rows.length : 0), 0)

    readonly property var current: list.current
    readonly property string status: {
        if (!root.available || root.text.length === 0)
            return "";
        const parts = [];
        if (!root.typing && root.flying)
            parts.push(I18n.tr("%1 of %2 sources").arg(root.progress.settled).arg(root.progress.total));
        else if (!root.typing && root.found > 0)
            parts.push(I18n.tr("%1 results").arg(root.found));
        for (var i = 0; i < root.resting.length && i < 2; i++) {
            const until = root.clock(root.resting[i].until);
            parts.push(until.length > 0 ? I18n.tr("%1 paused until %2").arg(root.resting[i].name).arg(until)
                : I18n.tr("%1 paused").arg(root.resting[i].name));
        }
        const silent = root.failures.filter(failure => !(root.stranded && failure.id === Web.LANE));
        if (!root.typing && silent.length > 2)
            parts.push(I18n.tr("%1 sources did not answer").arg(silent.length));
        else if (!root.typing && silent.length > 0)
            parts.push(I18n.tr("%1 did not answer").arg(silent.map(failure => failure.name).join(", ")));
        return parts.join(" \u00b7 ");
    }

    readonly property var primaryAction: {
        const row = root.current;
        if (!row)
            return root.text.length === 0 ? null : { label: root.typing ? I18n.tr("Search") : I18n.tr("Search in the browser") };
        if (row.kind === "query" || row.kind === "suggestion")
            return { label: I18n.tr("Search") };
        if (row.kind === "fallback")
            return { label: I18n.tr("Search in the browser") };
        return { label: root.link(row).length > 0 ? I18n.tr("Open in the browser") : I18n.tr("Copy") };
    }
    readonly property var secondaryAction: {
        const row = root.current;
        if (row && row.kind === "definition")
            return { label: I18n.tr("Copy definition"), glyph: "content_copy", hint: "Ctrl+Enter" };
        return root.link(row).length > 0 ? { label: I18n.tr("Copy link"), glyph: "content_copy", hint: "Ctrl+Enter" } : null;
    }
    readonly property var actions: {
        const out = [];
        const row = root.current;
        const item = row && row.item && row.kind !== "suggestion" ? row.item : null;
        if (item) {
            out.push({ id: "detail", label: root.detailOpen ? I18n.tr("Hide details") : I18n.tr("Show details"), glyph: "side_navigation", hint: "Ctrl+I", section: "" });
            if (Web.downloadable(item))
                out.push({ id: "download", label: I18n.tr("Download"), glyph: "download", hint: "Ctrl+D", section: "" });
            const links = Web.altLinks(item);
            for (var i = 0; i < links.length; i++)
                out.push({ id: "alt:" + links[i].id, label: root.altName(links[i].id), glyph: "open_in_new", hint: "", section: I18n.tr("Also") });
            const section = root.sectionOf(row);
            if (section && section.more && section.name.length > 0)
                out.push({ id: "all-from", label: I18n.tr("Show all from %1").arg(section.name), glyph: "unfold_more", hint: "Ctrl+M", section: I18n.tr("Also") });
        }
        if (root.scopes.length > 1) {
            out.push({ id: "scope-next", label: I18n.tr("Next scope"), glyph: "arrow_forward", hint: "Ctrl+Tab", section: I18n.tr("Scope") });
            out.push({ id: "scope-previous", label: I18n.tr("Previous scope"), glyph: "arrow_back", hint: "Ctrl+Shift+Tab", section: I18n.tr("Scope") });
        }
        if (root.text.length > 0 && row && !(row.kind === "fallback" && row.target === "browser"))
            out.push({ id: "browser", label: I18n.tr("Search in the browser"), glyph: "open_in_browser", hint: "Ctrl+B", section: I18n.tr("Scope") });
        return out;
    }

    signal pushRequested(string view, var args)
    signal closeRequested()
    signal hud(string text)
    signal alert(string text)
    signal menuRequested()
    signal queryRequested(string text)

    // with the detail open the list and the pane share one height
    readonly property real paneHeight: Math.min(root.maximumHeight - strip.height, Math.max(list.implicitHeight, Math.round(400 * root.s)))
    implicitHeight: strip.height + (!list.visible ? blank.implicitHeight : root.detailOpen ? root.paneHeight : list.implicitHeight)

    function altName(id) {
        return id === "discussion" ? I18n.tr("Open the discussion")
            : id === "repository" ? I18n.tr("Open the repository")
            : id === "homepage" ? I18n.tr("Open the home page")
            : I18n.tr("Open the documentation");
    }

    function sectionOf(row) {
        if (!row)
            return null;
        for (var i = 0; i < root.sections.length; i++) {
            if (root.sections[i].rows.some(member => member.id === row.id))
                return root.sections[i];
        }
        return null;
    }

    function sectionNote(section) {
        if (section.kind === "source")
            return section.total > section.rows.length ? I18n.tr("%1 of %2").arg(section.rows.length).arg(section.total) : "";
        if (section.kind !== "fallback")
            return "";
        if (section.state === "unavailable")
            return I18n.tr("Not available");
        if (section.state !== "paused")
            return I18n.tr("No engine answered");
        const until = root.clock(section.until);
        return until.length > 0 ? I18n.tr("Paused until %1").arg(until) : I18n.tr("Paused");
    }

    function clock(stamp) {
        const value = Date.parse(String(stamp || ""));
        return isNaN(value) ? "" : new Date(value).toLocaleTimeString(Qt.locale(), Locale.ShortFormat);
    }

    function dateText(stamp) {
        const value = Date.parse(String(stamp || ""));
        if (isNaN(value))
            return "";
        const date = new Date(value);
        return Qt.locale().toString(date, Date.now() - value > 300 * 86400000 ? "yyyy" : "d MMM");
    }

    // a definition opens its entry; the credit's own address stands in for one
    function link(row) {
        const item = row ? row.item : null;
        if (!item || row.kind === "suggestion")
            return "";
        const own = String(item.url || "");
        return own.length > 0 || row.kind !== "definition" ? own : String(item.meta && item.meta.attributionUrl || "");
    }

    function words(value) {
        return (Array.isArray(value) ? value : []).map(entry => String(entry || "").trim()).filter(entry => entry.length > 0);
    }

    function resultRow(entry, last) {
        if (entry.kind === "query")
            return { id: entry.id, kind: "query", last: last, size: "one", glyph: "search", glyphTone: "accent", title: entry.text };
        if (entry.kind === "fallback") {
            return {
                id: entry.id, kind: "fallback", target: entry.target, last: last, size: "one",
                glyph: "open_in_browser", title: I18n.tr("Search in the browser"), hint: ""
            };
        }
        const item = entry.item;
        if (entry.kind === "suggestion")
            return { id: entry.id, kind: "suggestion", item: item, source: entry.source, last: last, size: "one", glyph: "search", title: String(item.title || "") };
        const meta = item.meta || ({});
        if (entry.kind === "definition") {
            const examples = root.words(meta.examples);
            return {
                id: entry.id, kind: "definition", item: item, source: entry.source, last: last, size: "answer",
                glyph: String(entry.source.glyph || "dictionary"),
                title: String(item.title || ""),
                part: String(meta.partOfSpeech || ""),
                body: String(item.snippet || "").replace(/\s+/g, " "),
                example: examples.length > 0 ? I18n.tr("\u201c%1\u201d").arg(examples[0]) : "",
                credit: String(meta.attribution || "")
            };
        }
        const facts = Web.facts(item, Format.count);
        const by = Web.foundBy(item, root.engines);
        // the merged lane says quietly which engine found a row; beside the
        // detail pane the title needs the room and the pane says it
        if (by.length > 0 && facts.length < 2 && !root.detailOpen)
            facts.push({ glyph: "", text: by.length > 1 ? I18n.tr("%1 +%2").arg(by[0]).arg(by.length - 1) : by[0], mono: true });
        const site = String(item.site || "");
        const snippet = String(item.snippet || "").replace(/\s+/g, " ");
        return {
            id: entry.id, kind: "result", item: item, source: entry.source, last: last, size: "two",
            glyph: String(entry.source.glyph || "public"),
            image: String(item.thumb || ""), imageWide: item.kind === "video",
            title: String(item.title || item.url || ""),
            subtitle: site.length > 0 && snippet.length > 0 ? site + " \u00b7 " + snippet : site + snippet,
            facts: facts,
            trail: facts.length < 2 ? root.dateText(item.date) : ""
        };
    }

    function detailFacts(row) {
        if (!row || !row.item)
            return [];
        const meta = row.item.meta || ({});
        if (row.kind === "definition") {
            const out = [];
            if (String(meta.partOfSpeech || "").length > 0)
                out.push({ label: I18n.tr("Part of speech"), value: String(meta.partOfSpeech) });
            const tags = root.words(meta.tags);
            if (tags.length > 0)
                out.push({ label: I18n.tr("Tags"), value: tags.join(", ") });
            return out;
        }
        const by = Web.foundBy(row.item, root.engines);
        return by.length > 0 ? [{ label: I18n.tr("Found by"), value: by.join(", ") }] : [];
    }

    function open(url) {
        if (String(url || "").length === 0)
            return;
        Spawn.runApp(["xdg-open", String(url)], { label: "browser" });
        root.closeRequested();
    }

    function copy(text) {
        Quickshell.clipboardText = String(text);
        root.hud(I18n.tr("Copied"));
    }

    function stepScope(delta) {
        if (root.scopes.length === 0)
            return;
        var at = 0;
        for (var i = 0; i < root.scopes.length; i++) {
            if (root.scopes[i].id === root.scope)
                at = i;
        }
        root.scope = root.scopes[(at + delta + root.scopes.length) % root.scopes.length].id;
    }

    function move(delta) { list.move(delta); }
    function jumpSection(delta) { list.jumpSection(delta); }
    function page(delta) { list.page(delta); }

    function handleEscape() {
        if (!root.detailOpen)
            return false;
        root.detailOpen = false;
        return true;
    }

    // request asks the daemon for what this moment calls for (lib/web.js ask).
    function request(event, picked) {
        if (!root.search || !root.active)
            return;
        const want = Web.ask(event, { text: root.text, fullText: root.fullText, picked: picked || "", choosing: list.held });
        if (!want)
            return;
        if (want.mode === "full") {
            pause.stop();
            still.stop();
            root.fullText = want.text;
            root.settleAt = event === "still" ? Date.now() + 700 : 0;
        }
        root.search.search(want.text, root.scope, want.mode);
        if (want.text !== root.text)
            root.queryRequested(want.text);
    }

    function activate() {
        const row = root.current;
        if (!row) {
            if (root.typing)
                root.request("enter");
            else if (root.text.length > 0)
                root.open(Web.browserSearchUrl(root.text));
        } else if (row.kind === "query") {
            root.request("enter");
        } else if (row.kind === "suggestion") {
            root.request("picked", row.title);
        } else if (row.kind === "fallback") {
            root.open(Web.browserSearchUrl(root.text));
        } else if (Date.now() < root.settleAt) {
            return;
        } else if (root.link(row).length > 0) {
            root.open(root.link(row));
        } else {
            root.copy(String(row.item.snippet || row.item.title || ""));
        }
    }

    function activateSecondary() {
        const row = root.current;
        if (row && row.kind === "definition")
            root.copy(row.title + ": " + row.body);
        else if (root.link(row).length > 0)
            root.copy(root.link(row));
    }

    function act(id) {
        const row = root.current;
        const item = row && row.item ? row.item : null;
        if (id === "scope-next") {
            root.stepScope(1);
        } else if (id === "scope-previous") {
            root.stepScope(-1);
        } else if (id === "browser") {
            root.open(Web.browserSearchUrl(root.text));
        } else if (!item) {
            return;
        } else if (id === "detail") {
            root.detailOpen = !root.detailOpen;
        } else if (id === "download") {
            if (!root.downloads || !Web.downloadable(item))
                return;
            root.downloads.add({ url: Web.downloadUrl(item) }, function(task, error) {
                if (error)
                    root.alert(root.downloads.reason(error));
                else
                    root.hud(I18n.tr("Download started"));
            });
        } else if (id === "all-from") {
            const section = root.sectionOf(row);
            if (section && section.scope.length > 0)
                root.scope = section.scope;
        } else if (id.indexOf("alt:") === 0) {
            const links = Web.altLinks(item);
            for (var i = 0; i < links.length; i++) {
                if ("alt:" + links[i].id === id)
                    root.open(links[i].url);
            }
        }
    }

    function wantDetail() {
        const row = root.current;
        if (root.detailOpen && root.search && row && row.kind === "result" && row.item && row.source)
            root.search.requestDetail(row.source.id, row.item.key);
    }

    // arrive settles what the view owes the daemon when it comes on top: a
    // pushed query is searched at once, text it kept is picked up where it was.
    function arrive() {
        if (!root.active || !root.search)
            return;
        const pushed = String(root.args && root.args.query || "").trim();
        if (pushed.length > 0 && pushed === root.text && root.openedFor !== pushed) {
            root.openedFor = pushed;
            root.request("opened");
        } else if (root.text.length > 0 && (root.search.query !== root.text || root.search.scope !== root.scope)) {
            if (root.typing) {
                pause.restart();
                still.restart();
            } else {
                root.request("scope");
            }
        }
    }

    onActiveChanged: {
        if (!root.active || !root.search) {
            pause.stop();
            still.stop();
            return;
        }
        root.now = Date.now();
        root.search.start();
        Qt.callLater(root.arrive);
    }
    onArgsChanged: Qt.callLater(root.arrive)
    onTextChanged: {
        list.selectFirst();
        if (root.text.length === 0) {
            pause.stop();
            still.stop();
            root.fullText = "";
            root.request("paused");
        } else if (root.typing) {
            pause.restart();
            still.restart();
        } else if (root.search && (root.search.query !== root.text || root.search.mode !== "full")) {
            // back on text already searched: its results, not the suggestions left behind
            pause.stop();
            still.stop();
            root.request("scope");
        }
    }
    onScopeChanged: {
        list.selectFirst();
        root.request("scope");
        if (root.typing && root.text.length > 0)
            still.restart();
    }
    onCurrentChanged: lookCloser.restart()
    onDetailOpenChanged: root.wantDetail()

    // suggestions wait for the hand to pause; the full search for it to rest
    Timer { id: pause; interval: Web.SUGGEST_MS; onTriggered: root.request("paused") }
    Timer { id: still; interval: Web.STILL_MS; onTriggered: root.request("still") }
    Timer { id: lookCloser; interval: 140; onTriggered: root.wantDetail() }
    // a pause that ran out leaves the footer on its own
    Timer {
        readonly property real soonest: root.resting.length > 0 ? Date.parse(root.resting[0].until) : NaN
        interval: isNaN(soonest) ? 60000 : Math.max(1000, soonest - root.now + 500)
        running: root.active && !isNaN(soonest)
        onTriggered: root.now = Date.now()
    }

    ScopeStrip {
        id: strip
        width: parent.width
        s: root.s
        scopes: root.scopes
        current: root.scope
        sources: root.sources
        engines: root.engines
        answers: root.answers
        flying: root.flying
        visible: root.available && root.sources.length > 0
        height: visible ? implicitHeight : 0
        onPicked: scope => root.scope = scope
    }

    ResultList {
        id: list
        anchors.top: strip.bottom
        width: root.detailOpen ? Math.round(parent.width * 0.46) : parent.width
        height: root.detailOpen ? root.paneHeight : implicitHeight
        visible: count > 0
        s: root.s
        sections: root.sections
        maximumHeight: Math.max(0, root.maximumHeight - strip.height)
        rowDelegate: WebRow {}
        onActivated: root.activate()
        onMenuRequested: root.menuRequested()
    }

    DetailPane {
        id: pane

        readonly property var meta: root.current && root.current.kind === "definition" && root.current.item.meta ? root.current.item.meta : ({})

        anchors.top: strip.bottom
        anchors.left: list.right
        anchors.right: parent.right
        height: root.paneHeight
        visible: root.detailOpen && list.visible
        s: root.s
        item: root.current && root.current.item && root.current.kind !== "suggestion" ? root.current.item : null
        detail: root.search ? root.search.detail : null
        extra: root.detailFacts(root.current)
        examples: root.words(pane.meta.examples).map(example => I18n.tr("\u201c%1\u201d").arg(example))
        credit: String(pane.meta.attribution || "")
        creditUrl: String(pane.meta.attributionUrl || root.link(root.current))
        onLinkActivated: url => root.open(url)
    }

    // nothing to list: say why, in one or two lines
    Item {
        id: blank
        anchors.top: strip.bottom
        width: parent.width
        visible: !list.visible
        implicitHeight: blankColumn.implicitHeight + Math.round(30 * root.s)
        height: implicitHeight

        Column {
            id: blankColumn
            x: Math.round(18 * root.s)
            y: Math.round(13 * root.s)
            width: parent.width - x * 2
            spacing: Math.round(4 * root.s)

            Text {
                width: parent.width
                text: !root.available ? I18n.tr("Web search is not available")
                    : root.text.length > 0 ? I18n.tr("Nothing found for \u201c%1\u201d").arg(root.text)
                    : root.sources.length > 0 ? I18n.tr("Suggestions and definitions appear as you type. Enter searches every source.")
                    : I18n.tr("Starting the search service")
                color: Lithic.fg
                font.family: Look.sans
                font.pixelSize: Math.round(14 * root.s)
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }
            Text {
                width: parent.width
                visible: text.length > 0
                text: !root.available ? I18n.tr("The search service did not start.")
                    : root.text.length > 0 ? root.failures.map(failure => failure.name + ": " + failure.error).join(" \u00b7 ")
                    : root.sources.filter(source => source.id !== Web.HINTS).map(source => String(source.name || source.id)).join(" \u00b7 ")
                color: Lithic.fg3
                font.family: Look.sans
                font.pixelSize: Math.round(12 * root.s)
                wrapMode: Text.WordWrap
                maximumLineCount: 3
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
        }
    }
}
