import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

// the system card large, over a shade, to read it whole: it fills the window's height, turns
// toward the pointer with its foil sliding, and closes on Escape, a click beside it or its
// close button
Item {
    id: root

    property bool opened: false

    function open() {
        root.opened = true;
        root.forceActiveFocus();
    }
    function close() {
        root.opened = false;
    }

    anchors.fill: parent
    visible: shade.opacity > 0
    focus: root.opened
    Keys.onEscapePressed: root.close()

    Rectangle {
        id: shade
        anchors.fill: parent
        // deeper than a dialog's scrim: nothing behind should compete with the card
        color: Lithic.alpha("#000000", Lithic.dark ? 0.82 : 0.6)
        opacity: root.opened ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
        MouseArea {
            anchors.fill: parent
            onClicked: root.close()
        }
    }

    Item {
        id: holder
        readonly property real scaleTo: Math.min((root.height - 64) / 880, (root.width - 64) / 630, 1.4)
        width: 630 * holder.scaleTo
        height: 880 * holder.scaleTo
        anchors.centerIn: parent
        opacity: shade.opacity
        scale: 0.94 + 0.06 * shade.opacity
        readonly property bool pointed: tilt.containsMouse && !Lithic.reduceMotion
        Chase {
            id: tiltX
            target: holder.pointed ? Math.max(-1, Math.min(1, (tilt.mouseX / tilt.width - 0.5) * 2)) : 0
        }
        Chase {
            id: tiltY
            target: holder.pointed ? Math.max(-1, Math.min(1, (tilt.mouseY / tilt.height - 0.5) * 2)) : 0
        }
        transform: [
            Rotation { origin.x: holder.width / 2; origin.y: holder.height / 2; axis { x: 0; y: 1; z: 0 } angle: tiltX.value * 7 },
            Rotation { origin.x: holder.width / 2; origin.y: holder.height / 2; axis { x: 1; y: 0; z: 0 } angle: -tiltY.value * 5 }
        ]
        Loader {
            active: root.visible
            sourceComponent: TradingCard {
                transformOrigin: Item.TopLeft
                scale: holder.scaleTo
                foil: Qt.vector2d(tiltX.value, tiltY.value)
                live: root.opened && holder.pointed
            }
        }
        MouseArea {
            id: tilt
            anchors.fill: parent
            hoverEnabled: true
        }
    }

    IconButton {
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: LithicTokens.space.s4
        opacity: shade.opacity
        visible: root.opened
        glyph: "close"
        accessibleName: I18n.tr("Close the card")
        onClicked: root.close()
    }
}
