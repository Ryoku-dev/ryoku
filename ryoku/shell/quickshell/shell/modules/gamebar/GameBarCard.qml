pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../../components"
import "look"

// The card frame: an 8px grab margin, an 18px panel with a 1px outline, a
// 30px title bar and a 12px content area. The widget drags from its surface;
// the body is the default property.
Item {
    id: root

    property real s: 1
    property string title: ""
    property string cardId: ""
    property var host: null
    property bool resizable: false
    property bool minimalWhenPinned: false

    readonly property bool pinned: root.cardId.length > 0 && GameBarStore.isPinned(root.cardId)
    readonly property bool minimal: root.minimalWhenPinned && root.pinned && !GameBarStore.open
    readonly property string iconGlyph: root.host ? root.host.glyphFor(root.cardId) : ""

    default property alias body: bodyArea.data

    readonly property real grab: root.minimal ? 0 : 8 * root.s
    readonly property real pad: 6 * root.s
    readonly property real barHeight: 30 * root.s
    readonly property real grip: 18 * root.s

    property string dragMode: ""

    DragHandler {
        id: drag
        target: null
        acceptedButtons: Qt.LeftButton
        // Qt's default start distance reads as a dead beat before the card
        // answers; three pixels still keeps a button click a click.
        dragThreshold: 3
        onActiveChanged: {
            if (!root.host)
                return;
            if (active) {
                const p = drag.centroid.pressPosition;
                const edge = root.grab + root.grip;
                root.dragMode = root.resizable && !root.minimal
                    && p.x >= root.width - edge && p.y >= root.height - edge
                    ? "resize" : "move";
                if (root.dragMode === "resize") {
                    root.host.beginResize(root.cardId);
                    root.host.resizeBy(root.cardId, drag.translation.x, drag.translation.y);
                } else {
                    root.host.beginDrag(root.cardId);
                    // The threshold distance is already behind the pointer, so
                    // take it now instead of waiting for the next move.
                    root.host.dragBy(root.cardId, drag.translation.x, drag.translation.y);
                }
            } else {
                if (root.dragMode === "resize")
                    root.host.endResize(root.cardId);
                else if (root.dragMode === "move")
                    root.host.endDrag(root.cardId);
                root.dragMode = "";
            }
        }
        onTranslationChanged: if (active && root.host) {
            if (root.dragMode === "resize")
                root.host.resizeBy(root.cardId, drag.translation.x, drag.translation.y);
            else if (root.dragMode === "move")
                root.host.dragBy(root.cardId, drag.translation.x, drag.translation.y);
        }
    }

    Rectangle {
        id: frame
        anchors.fill: parent
        anchors.margins: root.grab
        radius: root.minimal ? 0 : GameBarLook.radiusWindow * root.s
        color: root.minimal ? "transparent" : GameBarLook.panel
        border.width: root.minimal ? 0 : 1
        border.color: GameBarLook.border

        Item {
            id: titleBar
            visible: !root.minimal
            x: root.pad
            y: root.pad
            width: parent.width - root.pad * 2
            height: root.barHeight

            Row {
                id: titleRow
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: 0

                Item {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 20 * root.s
                    height: 20 * root.s

                    MaterialIcon {
                        anchors.fill: parent
                        visible: root.iconGlyph.length === 0
                        text: root.host ? root.host.symbolFor(root.cardId) : ""
                        color: GameBarLook.fg
                        font.pixelSize: 20 * root.s
                    }

                    GlyphIcon {
                        anchors.fill: parent
                        visible: root.iconGlyph.length > 0
                        name: root.iconGlyph
                        color: GameBarLook.fg
                        stroke: 1.6
                    }
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    leftPadding: 4 * root.s
                    text: root.title
                    color: GameBarLook.fg
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.small * root.s
                    elide: Text.ElideRight
                }
            }

            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * root.s

                GameBarIconButton {
                    s: root.s
                    glyph: "pin"
                    toggled: root.pinned
                    onClicked: if (root.cardId.length > 0) GameBarStore.togglePinned(root.cardId)
                }

                GameBarIconButton {
                    s: root.s
                    glyph: "close"
                    onClicked: if (root.cardId.length > 0) GameBarStore.closeCard(root.cardId)
                }
            }
        }

        Item {
            id: content
            x: root.minimal ? 0 : root.pad
            y: root.minimal ? 0 : titleBar.y + titleBar.height + 1
            width: parent.width - (root.minimal ? 0 : root.pad * 2)
            height: parent.height - y - (root.minimal ? 0 : root.pad)

            Rectangle {
                id: contentFill
                anchors.fill: parent
                radius: root.minimal ? 0 : GameBarLook.radiusSmall * root.s
                color: root.minimal ? "transparent" : GameBarLook.content
            }

            Item {
                id: bodyArea
                anchors.fill: parent
                anchors.margins: root.minimal ? 0 : 8 * root.s
            }
        }

        Item {
            id: gripArea
            visible: root.resizable && !root.minimal
            width: root.grip
            height: root.grip
            x: frame.width - width
            y: frame.height - height
            opacity: gripPointer.containsMouse || root.dragMode === "resize" ? 0.9 : 0.4

            GlyphIcon {
                anchors.fill: parent
                name: "grip"
                color: GameBarLook.onLayer
                stroke: 1.6
            }

            MouseArea {
                id: gripPointer
                anchors.fill: parent
                hoverEnabled: true
                acceptedButtons: Qt.NoButton
                cursorShape: Qt.SizeFDiagCursor
            }
        }
    }
}
