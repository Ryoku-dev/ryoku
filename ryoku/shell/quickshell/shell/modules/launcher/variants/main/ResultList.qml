import QtQuick
import Quickshell
import Ryoku.Ui.Singletons as Ui
import "../../shared/Singletons"
import "metrics.js" as MainMetrics
import "../../../../utils/fuzzy.js" as Fuzzy
import "../../shared/lib/results.js" as Results

// The result list: ranked rows from the dispatcher. A row is a mono icon, a
// category eyebrow, the title with the matched chars highlighted, a subtitle, and
// the primary action verb on the selected row. Keyboard + pointer select; the
// primary action runs on activate.
ListView {
    id: list

    property real s: 1
    property var results: []
    property string query: ""
    property int selectedIndex: 0
    property string selectedResultKey: ""
    property bool selectionSyncing: false

    // window-position of the last hover allowed to move the selection, so rows
    // sliding under a still cursor during keyboard scroll don't steal it.
    property point lastPointer: Qt.point(-1, -1)

    signal activated(bool closeRequested)
    signal selectionLost()

    spacing: MainMetrics.gapRow * s
    clip: true
    boundsBehavior: Flickable.StopAtBounds
    model: results.length

    function applySelection(key, index) {
        selectionSyncing = true;
        selectedResultKey = key;
        selectedIndex = index;
        selectionSyncing = false;
    }

    function resetSelection() {
        applySelection("", 0);
    }

    function reconcileSelection() {
        var previousKey = selectedResultKey;
        var next = Results.reconcileSelection(
            results, previousKey, selectedIndex);
        applySelection(next.key, next.index);
        if (previousKey.length > 0 && previousKey !== next.key)
            selectionLost();
    }

    function move(delta) {
        if (results.length === 0)
            return;
        selectedIndex = Math.max(0, Math.min(results.length - 1, selectedIndex + delta));
        positionViewAtIndex(selectedIndex, ListView.Contain);
    }

    function activate() {
        if (selectedIndex < 0 || selectedIndex >= results.length)
            return;
        var row = results[selectedIndex];
        var action = row && row.actions && row.actions.length > 0
            ? row.actions[0] : null;
        if (!action || action.enabled === false || !action.execute)
            return;
        action.execute();
        list.activated(action.closeOnExecute !== false);
    }

    onResultsChanged: reconcileSelection()
    onSelectedIndexChanged: {
        if (selectionSyncing)
            return;
        var row = selectedIndex >= 0 && selectedIndex < results.length
            ? results[selectedIndex] : null;
        selectedResultKey = row ? String(row.resultKey || "") : "";
    }

    delegate: Item {
        id: row
        required property int index
        width: list.width

        readonly property var entry: list.results[index]
        readonly property bool selected: index === list.selectedIndex
        readonly property var spans: entry ? Fuzzy.highlight(entry.title, list.query) : []
        readonly property string typeLabel: entry && entry.type ? entry.type : ""
        // first row of each type group draws a small section header + hairline, so
        // results are visually grouped (apps, files, packages...) instead of one
        // undifferentiated list.
        readonly property bool sectionHead: {
            if (index === 0) return true;
            var prev = list.results[index - 1];
            return !prev || (prev.type || "") !== row.typeLabel;
        }
        readonly property real headerH: sectionHead ? MainMetrics.sectionH * list.s : 0
        readonly property bool hasIcon: entry && entry.icon && entry.icon.length > 0

        height: MainMetrics.rowHeight * list.s + headerH

        // section header
        Item {
            id: header
            visible: row.sectionHead
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            height: row.headerH

            Text {
                anchors.left: parent.left
                anchors.leftMargin: MainMetrics.padRow * list.s
                anchors.verticalCenter: parent.verticalCenter
                text: Ui.I18n.tr(row.typeLabel).toUpperCase()
                color: Theme.faint
                font.family: Theme.font
                font.pixelSize: Metrics.fontEyebrow * list.s
                font.letterSpacing: 1
            }
            Rectangle {
                anchors.right: parent.right
                anchors.left: parent.left
                anchors.leftMargin: MainMetrics.padRow * list.s + 70 * list.s
                anchors.rightMargin: MainMetrics.padRow * list.s
                anchors.verticalCenter: parent.verticalCenter
                height: 1
                color: Theme.hair
            }
        }

        Rectangle {
            id: rowBg
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: header.bottom
            anchors.bottom: parent.bottom
            radius: MainMetrics.radiusRow * list.s
            visible: row.selected || rowArea.containsMouse
            color: row.selected ? Theme.threadBg : Qt.rgba(0.94, 0.88, 0.84, 0.03)
            border.width: row.selected ? 1 : 0
            border.color: Theme.lineStrong
        }

        MouseArea {
            id: rowArea
            anchors.fill: rowBg
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onPositionChanged: (m) => {
                var g = rowArea.mapToItem(null, m.x, m.y);
                if (g.x !== list.lastPointer.x || g.y !== list.lastPointer.y) {
                    list.lastPointer = Qt.point(g.x, g.y);
                    list.selectedIndex = row.index;
                }
            }
            onClicked: { list.selectedIndex = row.index; list.activate(); }
        }

        Item {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: header.bottom
            anchors.bottom: parent.bottom
            anchors.leftMargin: MainMetrics.padRow * list.s
            anchors.rightMargin: MainMetrics.padRow * list.s

            Image {
                id: icon
                anchors.verticalCenter: parent.verticalCenter
                anchors.left: parent.left
                width: row.hasIcon ? MainMetrics.iconSize * list.s : 0
                height: MainMetrics.iconSize * list.s
                sourceSize.width: Math.round(MainMetrics.iconSize * 2 * list.s)
                sourceSize.height: Math.round(MainMetrics.iconSize * 2 * list.s)
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                smooth: true
                visible: row.hasIcon
                source: row.hasIcon ? row.entry.icon : ""
            }

            Column {
                anchors.verticalCenter: parent.verticalCenter
                anchors.left: icon.right
                anchors.leftMargin: row.hasIcon ? 11 * list.s : 0
                anchors.right: verb.left
                anchors.rightMargin: 10 * list.s
                spacing: 1 * list.s

                Text {
                    width: parent.width
                    textFormat: Text.StyledText
                    text: row.entry ? list.markup(row.entry.title, row.spans) : ""
                    color: row.selected ? Theme.bright : Theme.cream
                    font.family: Theme.font
                    font.pixelSize: Metrics.fontTitle * list.s
                    font.weight: row.selected ? Font.DemiBold : Font.Normal
                    elide: Text.ElideRight
                }
                Text {
                    visible: row.entry && row.entry.subtitle && row.entry.subtitle.length > 0
                    width: parent.width
                    text: row.entry ? row.entry.subtitle : ""
                    color: row.selected ? Theme.iconDim : Theme.faint
                    font.family: Theme.font
                    font.pixelSize: Metrics.fontSubtitle * list.s
                    elide: Text.ElideRight
                }
            }

            Text {
                id: verb
                anchors.verticalCenter: parent.verticalCenter
                anchors.right: parent.right
                text: row.entry && row.entry.actions && row.entry.actions.length > 0 ? row.entry.actions[0].name : ""
                color: Theme.vermLit
                font.family: Theme.font
                font.pixelSize: 11 * list.s
                visible: row.selected
            }
        }
    }

    // Wrap the matched spans in the accent color for the title's StyledText. Spans
    // come from Fuzzy.highlight (subsequence positions over the title).
    function markup(title, spans) {
        if (!spans || spans.length === 0)
            return list.escapeHtml(title);
        var out = "";
        var last = 0;
        for (var i = 0; i < spans.length; i++) {
            var sp = spans[i];
            if (sp.start > last)
                out += list.escapeHtml(title.slice(last, sp.start));
            out += "<font color=\"" + Theme.vermLit + "\">" + list.escapeHtml(title.slice(sp.start, sp.start + sp.len)) + "</font>";
            last = sp.start + sp.len;
        }
        if (last < title.length)
            out += list.escapeHtml(title.slice(last));
        return out;
    }

    function escapeHtml(s) {
        return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
    }
}
