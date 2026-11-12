import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../components"

// find an installed app by name; picking one hands over its launch command
Column {
    id: finder

    property string current: ""
    property color plane: Lithic.field
    property int limit: 5
    property string query: ""

    signal picked(string name, string command)

    spacing: LithicTokens.space.s2

    // every app the launcher lists, as { name, command, words }
    readonly property var catalog: {
        var src = DesktopEntries.applications ? DesktopEntries.applications.values : [];
        var out = [], seen = {};
        for (var i = 0; i < src.length; i++) {
            var e = src[i];
            if (!e || e.noDisplay)
                continue;
            var cmd = String((e.command || []).join(" ")).trim();
            if (cmd.length === 0 || seen[e.name + "\n" + cmd])
                continue;
            seen[e.name + "\n" + cmd] = true;
            out.push({ name: String(e.name || cmd), command: cmd,
                       words: [e.name, e.genericName, (e.keywords || []).join(" "), cmd].join(" ").toLowerCase() });
        }
        out.sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()));
        return out;
    }

    readonly property var matches: {
        var q = finder.query.trim().toLowerCase();
        if (q.length === 0)
            return [];
        var first = [], rest = [];
        for (var i = 0; i < finder.catalog.length; i++) {
            var a = finder.catalog[i];
            if (a.name.toLowerCase().indexOf(q) === 0)
                first.push(a);
            else if (a.words.indexOf(q) >= 0)
                rest.push(a);
        }
        return first.concat(rest).slice(0, finder.limit);
    }

    // the app a command launches, or "" when no installed app matches it
    function nameOf(command) {
        var c = String(command || "").trim();
        if (c.length === 0)
            return "";
        for (var i = 0; i < finder.catalog.length; i++)
            if (finder.catalog[i].command === c)
                return finder.catalog[i].name;
        return "";
    }

    function clear() {
        search.text = "";
    }

    Search {
        id: search
        width: parent.width
        placeholderText: I18n.tr("Find an installed app")
        accessibleName: I18n.tr("Find an installed app")
        onTextChanged: finder.query = text
    }

    Label {
        visible: finder.query.trim().length > 0 && finder.matches.length === 0
        width: parent.width
        text: I18n.tr("No installed app matches \u201c%1\u201d. Type its command below instead.").arg(finder.query.trim())
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Column {
        width: parent.width
        visible: finder.matches.length > 0

        Repeater {
            model: finder.matches
            delegate: HandoffRow {
                required property var modelData
                required property int index
                width: parent ? parent.width : 0
                first: index === 0
                plane: finder.plane
                text: modelData.name
                detail: modelData.command
                glyph: modelData.command === finder.current ? "check" : "add"
                Accessible.role: Accessible.Button
                onTriggered: finder.picked(modelData.name, modelData.command)
            }
        }
    }
}
