pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Widgets
import Ryoku.Ui.Singletons
import "../kit" as K
import "lib/rows.js" as Rows
import "lib/rest.js" as Rest
import "lib/text.js" as Words

// One track: index, icon, title over its kind, and the row's live meta. The
// bead behind it marks the selection, and on Lithic's bone bead the row takes
// the plate's ink.
Item {
    id: root

    required property string key
    required property int index
    property var row: null
    // how many digits the list's numbers run to, and the column that fits them
    property int digits: 2
    property real indexWidth: 16
    property var engine: null
    property bool selected: false
    property string highlight: ""
    property bool live: false
    // 0 hidden to 1 settled, from the list's open ripple
    property real intro: 1
    // 0 hidden to 1 settled, when the row comes into view as you type
    property real entry: 1

    signal pointed(point scenePosition)
    signal activated()

    readonly property bool onBone: root.selected && K.Look.lithic
    readonly property color inkMain: root.onBone ? K.Style.inkOnBone : K.Style.ink
    readonly property color inkRest: root.onBone ? K.Style.alpha(K.Style.inkOnBone, 0.72) : K.Style.inkDim
    readonly property color inkKind: root.onBone ? K.Style.alpha(K.Style.inkOnBone, 0.66) : K.Style.inkMuted
    readonly property string kind: Rows.kindKey(root.row)
    readonly property int windowCount: root.engine && root.kind === "app"
        ? root.engine.windowsFor(root.row).length : 0
    readonly property bool playing: root.engine !== null && root.kind === "app"
        && root.engine.isPlayingApp(root.row)
    readonly property string workspace: root.engine && root.kind === "window"
        ? root.engine.workspaceFor(root.row) : ""
    readonly property string memory: root.engine && root.kind === "window"
        ? Rows.memoryLabel((root.engine.windowInfo[String(root.row.windowAddress)] || {}).bytes) : ""
    property real pinch: 0

    implicitHeight: K.Style.rowH

    function categoryLabel(key) {
        switch (key) {
        case "browser": return I18n.tr("Browser");
        case "terminal": return I18n.tr("Terminal");
        case "files": return I18n.tr("Files");
        case "media": return I18n.tr("Media");
        case "office": return I18n.tr("Office");
        case "dev": return I18n.tr("Dev");
        case "graphics": return I18n.tr("Graphics");
        case "chat": return I18n.tr("Chat");
        case "game": return I18n.tr("Game");
        case "tool": return I18n.tr("Tool");
        case "system": return I18n.tr("System");
        case "network": return I18n.tr("Net");
        default: return I18n.tr("App");
        }
    }

    function kindText() {
        const row = root.row || {};
        switch (root.kind) {
        case "app": {
            const entry = root.engine ? root.engine.appEntry(row) : null;
            return root.categoryLabel(Rest.categoryKey(entry ? entry.categories : []));
        }
        case "window":
            return root.workspace.length > 0 ? I18n.tr("Window on %1").arg(root.workspace) : I18n.tr("Window");
        case "calc":
            return I18n.tr("Calculator");
        case "web":
            return I18n.tr("Web search");
        case "file":
            return row.providerId === "recent"
                ? Rows.parentFolder(row.subtitle, Quickshell.env("HOME") || "")
                : String(row.subtitle || "");
        case "action":
            return row.description ? String(row.description)
                : row.category ? I18n.tr(String(row.category)) : String(row.subtitle || "");
        default:
            return String(row.subtitle || "");
        }
    }

    function titleMarkup() {
        const title = String(root.row && root.row.title || "");
        if (root.highlight.length === 0)
            return Words.escapeHtml(title);
        const spans = Words.matchSpans(title, root.highlight);
        let out = "";
        for (let i = 0; i < spans.length; i++) {
            const tone = spans[i].hit ? root.inkMain : root.inkRest;
            out += "<font color=\"" + String(tone) + "\">" + Words.escapeHtml(spans[i].text) + "</font>";
        }
        return out;
    }

    function pinchNow() {
        if (K.Look.microMotion && !K.Style.still)
            pinchAnimation.restart();
    }

    // a quick rise, staggered by the row's place among the rows arriving with it
    function enter(order) {
        entryAnimation.stop();
        if (!K.Look.microMotion || K.Style.still) {
            root.entry = 1;
            return;
        }
        root.entry = 0;
        entryPause.duration = Math.min(6, Math.max(0, order)) * 16;
        entryAnimation.restart();
    }

    NumberAnimation {
        id: pinchAnimation
        target: root
        property: "pinch"
        from: 0
        to: 1
        duration: 160
        easing.type: Easing.OutCubic
    }

    SequentialAnimation {
        id: entryAnimation
        PauseAnimation { id: entryPause }
        NumberAnimation {
            target: root
            property: "entry"
            to: 1
            duration: K.Style.ms(170)
            easing.type: Easing.OutCubic
        }
    }

    Item {
        id: body
        anchors.fill: parent
        opacity: root.intro * root.entry * (root.row && root.row.disabled ? 0.5 : 1)
        transform: Translate { y: (1 - root.intro) * 6 + (1 - root.entry) * 6 * K.Style.scale }

        Text {
            id: indexLabel
            anchors.left: parent.left
            anchors.leftMargin: K.Style.s3
            anchors.verticalCenter: parent.verticalCenter
            width: root.indexWidth
            text: Rows.indexLabel(root.index, K.Look.lithic, root.digits)
            color: root.onBone ? root.inkKind : K.Style.inkFaint
            font.family: K.Look.lithic ? K.Style.mono : K.Style.font
            font.pixelSize: K.Style.caption
            font.features: K.Style.tabular
            horizontalAlignment: Text.AlignRight
            renderType: Text.NativeRendering
        }

        Item {
            id: iconBox
            anchors.left: indexLabel.right
            anchors.leftMargin: K.Style.s3
            anchors.verticalCenter: parent.verticalCenter
            width: 28
            height: 28
            scale: 1 - 0.18 * root.pinch
            opacity: 1 - root.pinch

            IconImage {
                anchors.fill: parent
                visible: String(root.row && root.row.icon || "").length > 0
                source: String(root.row && root.row.icon || "")
                implicitSize: 28
            }

            K.Glyph {
                anchors.centerIn: parent
                visible: String(root.row && root.row.icon || "").length === 0
                name: Rows.glyphFor(root.row)
                size: 22
                color: root.inkMain
            }
        }

        Row {
            id: meta
            anchors.right: parent.right
            anchors.rightMargin: K.Style.s3
            anchors.verticalCenter: parent.verticalCenter
            spacing: K.Style.s2

            EqBars {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.playing
                live: root.live && root.playing
                tone: root.inkMain
            }

            Row {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.windowCount > 0
                spacing: K.Style.s1

                K.Shape {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 5
                    height: 5
                    cutSize: 1.5
                    fill: root.inkMain
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.windowCount > 1
                    text: String(root.windowCount)
                    color: root.inkKind
                    font.family: K.Look.lithic ? K.Style.mono : K.Style.font
                    font.pixelSize: K.Style.caption
                    font.features: K.Style.tabular
                    renderType: Text.NativeRendering
                }
            }

            Row {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.memory.length > 0
                spacing: K.Style.s1

                K.Glyph {
                    anchors.verticalCenter: parent.verticalCenter
                    name: "memory"
                    size: 14
                    color: root.inkKind
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.memory
                    color: root.inkKind
                    font.family: K.Look.lithic ? K.Style.mono : K.Style.font
                    font.pixelSize: K.Style.caption
                    font.features: K.Style.tabular
                    renderType: Text.NativeRendering
                }
            }

            KeyCap {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.selected
                text: I18n.tr("Enter")
                inverse: true
            }
        }

        Column {
            anchors.left: iconBox.right
            anchors.leftMargin: K.Style.s3
            anchors.right: meta.left
            anchors.rightMargin: K.Style.s2
            anchors.verticalCenter: parent.verticalCenter
            spacing: 1

            Text {
                width: parent.width
                text: root.titleMarkup()
                textFormat: Text.StyledText
                color: root.highlight.length > 0 ? root.inkRest : root.inkMain
                font.family: K.Style.font
                font.pixelSize: K.Style.body
                font.weight: Font.Medium
                elide: Text.ElideRight
                maximumLineCount: 1
                renderType: Text.NativeRendering
            }

            Text {
                width: parent.width
                visible: text.length > 0
                text: root.kindText()
                color: root.inkKind
                font.family: K.Style.font
                font.pixelSize: K.Style.caption
                elide: root.kind === "file" ? Text.ElideMiddle : Text.ElideRight
                maximumLineCount: 1
                renderType: Text.NativeRendering
            }
        }
    }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton
        cursorShape: Qt.PointingHandCursor
        onEntered: root.pointed(mapToItem(null, mouseX, mouseY))
        onPositionChanged: mouse => root.pointed(mapToItem(null, mouse.x, mouse.y))
        onClicked: root.activated()
    }
}
