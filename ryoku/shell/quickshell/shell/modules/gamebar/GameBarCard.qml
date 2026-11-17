pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../../components"
import Lithic
import "look"

Item {
    id: root

    property real s: 1
    property string title: ""
    property string cardId: ""
    property var host: null
    property bool resizable: false
    property bool minimalWhenPinned: false
    property real preferredHeight: 0

    readonly property bool pinned: root.cardId.length > 0 && GameBarStore.isPinned(root.cardId)
    readonly property bool minimal: root.minimalWhenPinned && root.pinned && !GameBarStore.open
    readonly property string iconGlyph: root.host ? root.host.glyphFor(root.cardId) : ""

    default property alias body: bodyArea.data

    readonly property real space1: LithicTokens.space.s1 * root.s
    readonly property real space2: LithicTokens.space.s2 * root.s
    readonly property real space3: LithicTokens.space.s3 * root.s
    readonly property real space4: LithicTokens.space.s4 * root.s
    readonly property real grab: root.minimal ? 0 : 8 * root.s
    readonly property real pad: root.minimal ? 0 : 8 * root.s
    readonly property real head: 32 * root.s
    readonly property real gripSize: 18 * root.s

    property string dragMode: ""

    DragHandler {
        id: drag
        target: null
        acceptedButtons: Qt.LeftButton
        dragThreshold: 3
        onActiveChanged: {
            if (!root.host)
                return;
            if (active) {
                const p = drag.centroid.pressPosition;
                const edge = root.grab + root.gripSize;
                root.dragMode = root.resizable && !root.minimal
                    && p.x >= root.width - edge && p.y >= root.height - edge
                    ? "resize" : "move";
                if (root.dragMode === "resize") {
                    root.host.beginResize(root.cardId);
                    root.host.resizeBy(root.cardId, drag.translation.x, drag.translation.y);
                } else {
                    root.host.beginDrag(root.cardId);
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

    Item {
        id: logical
        anchors.fill: parent
        readonly property real lithicScale: root.s

        Item {
            id: frame
            anchors.fill: parent
            anchors.margins: root.minimal ? 0 : root.grab

            CutRect {
                anchors.fill: parent
                visible: !root.minimal
                fill: GameBarTone.pane
                strokeColor: GameBarTone.paneStroke
                strokeWidth: LithicTokens.size.hairline
                cut: GameBarTone.paneCut
            }

            Item {
                id: headArea
                visible: !root.minimal
                x: root.pad
                y: root.pad
                width: frame.width - root.pad * 2
                height: root.head

                GameBarGlyph {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    size: 18
                    ink: GameBarTone.ink2
                    symbol: root.host ? root.host.symbolFor(root.cardId) : ""
                    brand: root.iconGlyph
                }

                Row {
                    id: headActions
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 4 * root.s

                    IconPlaque {
                        small: true
                        kind: root.pinned ? "tonal" : "quiet"
                        glyph: "push_pin"
                        Accessible.name: root.pinned ? I18n.tr("Unpin card") : I18n.tr("Pin card")
                        onClicked: if (root.cardId.length > 0) GameBarStore.togglePinned(root.cardId)
                    }

                    IconPlaque {
                        small: true
                        kind: "quiet"
                        glyph: "close"
                        Accessible.name: I18n.tr("Close card")
                        onClicked: if (root.cardId.length > 0) GameBarStore.closeCard(root.cardId)
                    }
                }

                Label {
                    anchors.left: parent.left
                    anchors.leftMargin: 26 * root.s
                    anchors.right: headActions.left
                    anchors.rightMargin: 8 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.title
                    typeRole: "title"
                }
            }

            Rectangle {
                visible: !root.minimal
                x: root.pad
                y: root.pad + root.head + 3 * root.s
                width: frame.width - root.pad * 2
                height: LithicTokens.size.hairline
                color: GameBarTone.line
            }

            Item {
                id: content
                x: root.minimal ? 0 : root.pad
                y: root.minimal ? 0 : root.pad + root.head + 6 * root.s
                width: frame.width - (root.minimal ? 0 : root.pad * 2)
                height: frame.height - y - (root.minimal ? 0 : root.pad)

                Item {
                    id: bodyArea
                    anchors.fill: parent
                }
            }

            Item {
                visible: root.resizable && !root.minimal
                width: root.gripSize
                height: root.gripSize
                x: frame.width - width
                y: frame.height - height
                opacity: gripPointer.containsMouse || root.dragMode === "resize" ? 1 : 0.5

                Glyph {
                    anchors.centerIn: parent
                    name: "south_east"
                    size: 14
                    ink: GameBarTone.ink3
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
}
