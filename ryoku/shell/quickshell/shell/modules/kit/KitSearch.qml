pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../ask/bar"
import "../ask/look"
import "lib/sections.js" as Sections
import "lib/search.js" as Search

// All of Kit (Ctrl+F): one query answered by every section, grouped by section
// in rail order, with the selection set out beside the list. Enter opens the
// row in its own section. It answers the same contract a section does.
Item {
    id: root

    required property real s
    property bool active: false
    property string query: ""
    property var args: ({})
    property real maximumHeight: 480 * s
    // {id: Source}, {id: label}: the frame's
    property var sources: ({})
    property var labels: ({})

    readonly property string title: I18n.tr("All of Kit")
    readonly property string placeholder: I18n.tr("Search every section")
    readonly property bool ownsText: false
    readonly property var pages: []
    property string pageId: ""

    readonly property bool asked: root.query.trim().length > 0
    readonly property var found: {
        const out = {};
        for (var i = 0; i < Sections.ORDER.length; i++) {
            const id = Sections.ORDER[i];
            const source = root.sources[id];
            // revision is read so a catalogue that finishes loading answers again
            out[id] = source && root.asked && source.revision >= 0 ? (source.searchAll(root.query) || []) : [];
        }
        return out;
    }
    readonly property var groups: Search.group(Sections.ORDER, root.labels, root.found, root.query, {
        limit: 5,
        more: function(count, label) { return I18n.tr("%1 more in %2").arg(count).arg(label); }
    })
    readonly property var counts: {
        const out = {};
        for (var id in root.found)
            out[id] = root.found[id].length;
        return out;
    }
    readonly property int total: Search.total(root.groups)
    readonly property var row: list.current
    readonly property string chosen: root.row ? String(root.row.section || "") : ""

    readonly property string status: !root.asked ? I18n.tr("Shortcuts, commands, tools, your phone and downloads")
        : root.total === 0 ? I18n.tr("Nothing in Kit matches \u201c%1\u201d").arg(root.query.trim())
        : root.total === 1 ? I18n.tr("1 result")
        : root.groups.length === 1 ? I18n.tr("%1 results in 1 section").arg(root.total)
        : I18n.tr("%1 results in %2 sections").arg(root.total).arg(root.groups.length)
    readonly property var primaryAction: root.row
        ? { label: I18n.tr("Open in %1").arg(String(root.labels[root.chosen] || "")), hint: "Enter" } : null
    readonly property var secondaryAction: null
    readonly property var actions: []
    readonly property string actionNote: ""

    signal openRequested(string section, string page, string query, var args)
    signal closeRequested()
    signal hud(string text)
    signal alert(string text)
    signal queryRequested(string text)
    signal menuRequested()

    function activate() {
        const target = Search.target(root.row);
        if (target)
            root.openRequested(target.section, target.page, target.query, target.args);
    }
    function activateSecondary() {}
    function act(id) {}
    function handleEscape() { return false; }
    function move(delta) { list.move(delta); }
    function jumpSection(delta) { list.jumpSection(delta); }
    function page(delta) { list.page(delta); }

    onQueryChanged: list.selectFirst()

    ResultList {
        id: list
        width: Math.round(parent.width * 0.46)
        height: parent.height
        s: root.s
        sections: root.groups
        maximumHeight: root.height
        onActivated: row => {
            const target = Search.target(row);
            if (target)
                root.openRequested(target.section, target.page, target.query, target.args);
        }
    }

    Text {
        x: Math.round(22 * root.s)
        y: Math.round(18 * root.s)
        width: list.width - x * 2
        visible: list.count === 0
        text: root.asked ? I18n.tr("Nothing in Kit matches \u201c%1\u201d").arg(root.query.trim()) : I18n.tr("Type to search every section at once")
        color: Lithic.fg2
        font.family: Look.sans
        font.pixelSize: Math.round(13.5 * root.s)
        wrapMode: Text.WordWrap
        maximumLineCount: 3
        elide: Text.ElideRight
        renderType: Text.NativeRendering
    }

    // the selection, at reading size
    Shape {
        x: list.width + Math.round(10 * root.s)
        y: Math.round(10 * root.s)
        width: parent.width - x - Math.round(10 * root.s)
        height: Math.min(parent.height - y * 2, pane.implicitHeight + Math.round(44 * root.s))
        visible: root.row !== null
        s: root.s
        cutSize: Look.cutCard
        fill: Look.card
        strokeColor: Look.cardStroke
        strokeWidth: Qt.colorEqual(Look.cardStroke, "transparent") ? 0 : 1

        Column {
            id: pane
            x: Math.round(22 * root.s)
            y: Math.round(22 * root.s)
            width: parent.width - x * 2
            spacing: Math.round(8 * root.s)

            Text {
                width: parent.width
                text: root.row ? String(root.row.title || "") : ""
                color: Lithic.fg
                font.family: Look.sans
                font.pixelSize: Math.round(17 * root.s)
                font.weight: Font.Medium
                wrapMode: Text.WordWrap
                maximumLineCount: 3
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
            Text {
                width: parent.width
                visible: text.length > 0
                text: root.row ? String(root.row.subtitle || "") : ""
                color: Lithic.fg2
                font.family: Look.sans
                font.pixelSize: Math.round(13 * root.s)
                wrapMode: Text.WordWrap
                maximumLineCount: 4
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
            Text {
                width: parent.width
                visible: text.length > 0
                topPadding: Math.round(4 * root.s)
                text: root.row ? String(root.row.trail || "") : ""
                color: Lithic.fg
                font.family: Look.mono
                font.pixelSize: Math.round(12 * root.s)
                wrapMode: Text.WrapAnywhere
                maximumLineCount: 6
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
        }
    }
}
