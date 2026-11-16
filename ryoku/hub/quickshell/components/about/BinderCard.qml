import QtQuick
import Lithic
import ".."

// Both binder sizes draw the same live card, never a saved thumbnail.
Item {
    id: root
    required property var entry
    required property string cardsStoreRoot
    property bool interactive: true
    signal chosen()
    height: width * 880 / 630
    activeFocusOnTab: root.interactive
    Accessible.role: root.interactive ? Accessible.Button : Accessible.Graphic
    Accessible.name: root.entry.name
    Keys.onReturnPressed: if (root.interactive) root.chosen()
    Keys.onEnterPressed: if (root.interactive) root.chosen()
    Keys.onSpacePressed: if (root.interactive) root.chosen()

    readonly property bool pointed: pointer.containsMouse && !Lithic.reduceMotion
    Chase { id: tiltX; target: root.pointed ? (pointer.mouseX / root.width - 0.5) * 2 : 0 }
    Chase { id: tiltY; target: root.pointed ? (pointer.mouseY / root.height - 0.5) * 2 : 0 }
    transform: [
        Rotation { origin.x: root.width / 2; origin.y: root.height / 2; axis { x: 0; y: 1; z: 0 } angle: tiltX.value * 7 },
        Rotation { origin.x: root.width / 2; origin.y: root.height / 2; axis { x: 1; y: 0; z: 0 } angle: -tiltY.value * 5 }
    ]
    TradingCard {
        traded: root.entry
        cardsStoreRoot: root.cardsStoreRoot
        transformOrigin: Item.TopLeft
        scale: root.width / designWidth
        foil: Qt.vector2d(tiltX.value, tiltY.value)
        live: root.pointed
    }
    CutRect {
        anchors.fill: parent
        anchors.margins: -4
        cut: 10
        fill: "transparent"
        strokeWidth: 2
        strokeColor: Lithic.gold
        visible: root.activeFocus
    }
    MouseArea {
        id: pointer
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: root.interactive ? Qt.PointingHandCursor : Qt.ArrowCursor
        onClicked: if (root.interactive) root.chosen()
    }
}
