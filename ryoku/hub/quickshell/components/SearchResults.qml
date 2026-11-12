import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

// search hits grouped by page, each with its breadcrumb
Item {
    id: root

    required property var hub
    property var results: []
    property string query: ""
    property var nearby: []
    property int cursor: 0

    signal chosen(var entry)

    // rows in reading order: group heads and hits
    readonly property var lines: {
        var order = [], groups = {};
        for (var i = 0; i < root.results.length; i++) {
            var e = root.results[i];
            var g = e.groupKey;
            if (!groups[g]) {
                groups[g] = [];
                order.push(g);
            }
            groups[g].push(e);
        }
        var out = [], ordinal = 0;
        for (var j = 0; j < order.length; j++) {
            var hits = groups[order[j]];
            out.push({ head: true, title: hits[0].groupTitle, ordinal: -1 });
            for (var k = 0; k < hits.length; k++)
                out.push({ head: false, entry: hits[k], ordinal: ordinal++ });
        }
        return out;
    }
    readonly property var hits: root.lines.filter(l => !l.head).map(l => l.entry)

    function move(delta) {
        if (root.hits.length === 0)
            return;
        root.cursor = Math.max(0, Math.min(root.hits.length - 1, root.cursor + delta));
        for (var i = 0; i < root.lines.length; i++) {
            if (root.lines[i].ordinal === root.cursor) {
                list.positionViewAtIndex(i, ListView.Contain);
                break;
            }
        }
    }

    function activate() {
        if (root.cursor >= 0 && root.cursor < root.hits.length)
            root.chosen(root.hits[root.cursor]);
    }

    onResultsChanged: root.cursor = 0

    function escaped(s) {
        return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
    }

    // the label with the query's words underlined where they match
    function marked(text) {
        var words = root.query.toLowerCase().split(/[\s,.;:\/()&+-]+/).filter(w => w.length > 0);
        var lower = String(text).toLowerCase();
        var spans = [];
        for (var i = 0; i < words.length; i++) {
            var at = lower.indexOf(words[i]);
            if (at >= 0)
                spans.push([at, at + words[i].length]);
        }
        spans.sort((a, b) => a[0] - b[0]);
        var out = "", pos = 0;
        for (var j = 0; j < spans.length; j++) {
            if (spans[j][0] < pos)
                continue;
            out += root.escaped(text.slice(pos, spans[j][0])) + "<u>" + root.escaped(text.slice(spans[j][0], spans[j][1])) + "</u>";
            pos = spans[j][1];
        }
        return out + root.escaped(text.slice(pos));
    }

    ListView {
        id: list
        anchors.fill: parent
        visible: root.results.length > 0
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        spacing: LithicTokens.size.rowListGap
        model: root.lines

        delegate: Loader {
            id: line
            required property var modelData
            required property int index
            width: ListView.view.width
            sourceComponent: modelData.head ? headLine : hitLine

            Component {
                id: headLine
                Label {
                    topPadding: line.index === 0 ? 0 : LithicTokens.space.s3
                    leftPadding: LithicTokens.space.s3
                    text: line.modelData.title
                    typeRole: "label"
                    inkRole: "fg2"
                }
            }

            Component {
                id: hitLine
                T.AbstractButton {
                    id: hit
                    readonly property var entry: line.modelData.entry
                    readonly property bool current: line.modelData.ordinal === root.cursor
                    height: 48
                    hoverEnabled: true
                    focusPolicy: Qt.NoFocus
                    Accessible.role: Accessible.Button
                    Accessible.name: hit.entry.display
                    onClicked: root.chosen(hit.entry)

                    background: CutRect {
                        cut: LithicTokens.cut.small
                        fill: hit.down ? Lithic.pressRow(Lithic.stone1)
                            : hit.hovered || hit.current ? Lithic.hoverPlane(Lithic.stone1) : "transparent"
                        ChiselColorBehavior on fill {
                            duration: hit.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
                        }
                    }

                    contentItem: Item {
                        Column {
                            anchors.left: parent.left
                            anchors.leftMargin: LithicTokens.space.s3
                            anchors.right: openGlyph.left
                            anchors.rightMargin: LithicTokens.space.s2
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: LithicTokens.size.settingsDetailGap
                            Label {
                                width: parent.width
                                text: root.marked(hit.entry.display)
                                textFormat: Text.StyledText
                                typeRole: "row"
                            }
                            Label {
                                width: parent.width
                                text: hit.entry.crumb
                                typeRole: "detail"
                            }
                        }
                        Glyph {
                            id: openGlyph
                            visible: hit.entry.kind === "handoff"
                            anchors.right: parent.right
                            anchors.rightMargin: LithicTokens.space.s3
                            anchors.verticalCenter: parent.verticalCenter
                            name: "open_in_new"
                            size: LithicTokens.size.plaqueIconSmall
                            ink: Lithic.fg2
                        }
                    }
                }
            }
        }
    }

    ScrollRail {
        flickable: list
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }

    Column {
        visible: root.results.length === 0 && root.query.trim().length > 0
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            width: parent.width
            leftPadding: LithicTokens.space.s3
            rightPadding: LithicTokens.space.s3
            text: I18n.tr("No settings match “%1”").replace("%1", root.query.trim())
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Label {
            visible: root.nearby.length > 0
            leftPadding: LithicTokens.space.s3
            text: I18n.tr("Nearby pages")
            typeRole: "label"
            inkRole: "fg2"
        }
        Repeater {
            model: root.nearby
            delegate: PageRow {
                required property var modelData
                width: parent.width
                text: modelData.display
                summary: modelData.crumb
                onClicked: root.chosen(modelData)
            }
        }
    }
}
