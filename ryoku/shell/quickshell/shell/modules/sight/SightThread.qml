pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../services/lib/markdown.js" as Md

Column {
    id: root
    property var turns: []
    property real s: 1
    property var keyTarget: null
    spacing: 13 * s
    property bool ready: false
    ListModel { id: rows }
    function sync() {
        while (rows.count > turns.length) rows.remove(rows.count - 1);
        for (var i = 0; i < turns.length; ++i) {
            const row = { question: String(turns[i].question || ""), answer: String(turns[i].answer || "") };
            if (i >= rows.count) rows.append(row);
            else {
                if (rows.get(i).question !== row.question) rows.setProperty(i, "question", row.question);
                if (rows.get(i).answer !== row.answer) rows.setProperty(i, "answer", row.answer);
            }
        }
    }
    onTurnsChanged: if (ready) sync()
    Component.onCompleted: { ready = true; sync(); }
    function htmlEscape(text) { return String(text).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;"); }
    function inline(text) {
        return String(text).split(/(`[^`]*`)/g).map(function(part) {
            if (part.length > 1 && part[0] === "`" && part[part.length - 1] === "`")
                return '<span style="font-family:' + root.htmlEscape(Lithic.font.mono) + ';font-size:' + (12 * root.s) + 'px">' + root.htmlEscape(part.slice(1, -1)) + '</span>';
            return root.htmlEscape(part.replace(/!\[([^\]]*)\]\([^)]*\)/g, "$1"))
                .replace(/\*\*([^*]+)\*\*/g, "<b>$1</b>")
                .replace(/__([^_]+)__/g, "<b>$1</b>")
                .replace(/\*([^*]+)\*/g, "<i>$1</i>")
                .replace(/\[([^\]]+)\]\(([^)]+)\)/g, function(all, label, url) {
                    const safe = Md.safeUrl(url.replace(/&amp;/g, "&"));
                    return safe ? '<a href="' + root.htmlEscape(safe) + '" style="color:' + Lithic.fg.toString() + '">' + label + '</a>' : label;
                });
        }).join("");
    }
    function blocks(text) {
        const out = [];
        const lines = String(text || "").split("\n");
        var fence = "", buffer = [], prose = [];
        function flushProse() { if (prose.length) { out.push({ kind: "prose", text: prose.join("\n") }); prose = []; } }
        for (var i = 0; i < lines.length; ++i) {
            const marker = lines[i].match(/^\s*(```+|~~~+)/);
            if (marker) {
                if (!fence) { flushProse(); fence = marker[1][0]; buffer = []; }
                else if (marker[1][0] === fence) { out.push({ kind: "code", text: buffer.join("\n") }); fence = ""; buffer = []; }
                else buffer.push(lines[i]);
            } else if (fence) buffer.push(lines[i]);
            else if (!lines[i].trim()) flushProse();
            else {
                const list = lines[i].match(/^\s*(?:[-*+]\s+|\d+[.)]\s+)(.*)$/);
                const heading = lines[i].match(/^#{1,6}\s+(.+?)\s*#*$/);
                if (list) { flushProse(); out.push({ kind: "list", text: /^\s*\d/.test(lines[i]) ? lines[i].trim() : list[1] }); }
                else if (heading) { flushProse(); out.push({ kind: "prose", text: "**" + heading[1] + "**" }); }
                else prose.push(lines[i]);
            }
        }
        if (fence) out.push({ kind: "code", text: buffer.join("\n") });
        flushProse();
        return out;
    }
    Repeater {
        model: rows
        delegate: Column {
            id: turn
            required property string question
            required property string answer
            width: root.width
            spacing: 8 * root.s
            property bool ready: false
            ListModel { id: parts }
            function syncBlocks() {
                if (!ready) return;
                const next = root.blocks(answer);
                while (parts.count > next.length) parts.remove(parts.count - 1);
                for (var i = 0; i < next.length; ++i) {
                    if (i >= parts.count) parts.append(next[i]);
                    else if (parts.get(i).kind !== next[i].kind || parts.get(i).text !== next[i].text) parts.set(i, next[i]);
                }
            }
            onAnswerChanged: syncBlocks()
            Component.onCompleted: { ready = true; syncBlocks(); }
            Label { width: parent.width; visible: turn.question.length > 0; text: turn.question; typeRole: "label"; inkRole: "fg2"; wrapMode: Text.Wrap; elide: Text.ElideNone }
            Repeater {
                model: parts
                delegate: Item {
                    id: block
                    required property string kind
                    required property string text
                    readonly property bool code: kind === "code"
                    readonly property bool list: kind === "list"
                    width: turn.width
                    height: prose.implicitHeight + (code ? 20 * root.s : 0)
                    CutRect { anchors.fill: parent; visible: block.code; cut: 7; fill: Lithic.field }
                    CutRect { visible: block.list && !/^\d+[.)]\s/.test(block.text); x: 0; y: 8 * root.s; width: 3 * root.s; height: width; fill: Lithic.fg2 }
                    TextEdit {
                        id: prose
                        x: block.code ? 13 * root.s : block.list ? 8 * root.s : 0
                        y: block.code ? 10 * root.s : 0
                        width: parent.width - x - (block.code ? 13 * root.s : 0)
                        text: block.code ? '<pre style="margin:0;line-height:167%;font-family:' + root.htmlEscape(Lithic.font.mono) + ';font-size:' + (12 * root.s) + 'px">' + root.htmlEscape(block.text) + '</pre>'
                            : '<div style="line-height:143%">' + root.inline(block.text).replace(/\n/g, "<br>") + '</div>'
                        textFormat: TextEdit.RichText
                        color: Lithic.fg
                        font.family: block.code ? Lithic.font.mono : Lithic.type.body.family
                        font.pixelSize: (block.code ? 12 : 14) * root.s
                        wrapMode: TextEdit.Wrap; readOnly: true; selectByMouse: true
                        selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
                        Keys.forwardTo: root.keyTarget ? [root.keyTarget] : []
                        onLinkActivated: url => { const safe = Md.safeUrl(url); if (safe) Spawn.runApp(["xdg-open", safe], { label: "browser" }); }
                    }
                }
            }
        }
    }
}
