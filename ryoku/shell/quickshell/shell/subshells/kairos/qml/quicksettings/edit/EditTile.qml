pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Ryoku.Ui.Singletons
import "../../kit"
import ".." as Q

// one tile of the editor, in the grid or the available list; the page owns where it sits
Item {
    id: cell

    property var editor
    property string tileId: ""

    readonly property var place: cell.editor.places[cell.tileId] || null
    readonly property bool inGrid: cell.place !== null && cell.place.where === "grid"
    readonly property bool held: cell.editor.dragKind === "tile" && cell.editor.dragId === cell.tileId
    readonly property bool tracking: cell.held && cell.editor.tracking
    readonly property bool picked: cell.inGrid && cell.editor.pickedKind === "tile" && cell.editor.picked === cell.tileId
    readonly property bool sizing: cell.picked && cell.editor.sizing
    readonly property bool still: !cell.editor.ready || Md.still

    x: cell.held ? cell.editor.gx : (cell.place ? cell.place.x : 0)
    y: cell.held ? cell.editor.gy : (cell.place ? cell.place.y : 0)
    width: cell.sizing ? cell.editor.sizeW : (cell.place ? cell.place.w : 0)
    height: cell.editor.tileHeight
    z: cell.held ? 10 : cell.picked ? 3 : 1
    // over the hover row it shrinks out of the way so the chip it would become shows through
    scale: !cell.held ? 1 : cell.editor.target === "chips" ? 0.5 : 1.06
    opacity: cell.held && cell.editor.target === "chips" ? 0.7 : 1

    Behavior on x {
        enabled: !cell.still && !cell.tracking
        Spring {}
    }
    Behavior on y {
        enabled: !cell.still && !cell.tracking
        Spring {}
    }
    Behavior on width {
        enabled: !cell.still && !(cell.picked && cell.editor.sizeTracking)
        Spring { speed: "fast" }
    }
    Behavior on scale {
        enabled: !Md.still
        Spring { speed: "fast" }
    }
    Behavior on opacity {
        enabled: !Md.still
        NumberAnimation { duration: Md.fast }
    }

    Q.TileSpec { id: spec; tileId: cell.tileId }

    Rectangle {
        anchors.fill: face
        anchors.topMargin: 6
        anchors.bottomMargin: -6
        radius: face.rest
        color: "black"
        opacity: cell.held ? 0.55 : 0
        visible: opacity > 0.01
        layer.enabled: visible
        layer.effect: MultiEffect {
            blurEnabled: true
            blur: 1
            blurMax: 24
            autoPaddingEnabled: true
        }
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
    }

    Tile {
        id: face
        anchors.fill: parent
        // the editor takes every press, the tile only draws
        enabled: false
        opacity: spec.available ? 1 : 0.38
        large: cell.sizing ? cell.width > cell.editor.mid : (cell.place ? cell.place.large : false)
        icon: spec.icon
        label: spec.label
        secondary: spec.secondary
        active: spec.active
        split: spec.split
    }

    Rectangle {
        anchors.fill: parent
        radius: face.rest
        color: face.ink
        opacity: grab.pressed ? 0.14 : grab.containsMouse ? 0.08 : 0
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
    }

    // drawn inside the tile: the outer column sits flush with the shade's clip
    Rectangle {
        id: ring
        anchors.fill: parent
        radius: face.rest
        color: "transparent"
        border.width: 3
        border.color: face.active ? Md.onAccent : Md.accent
        opacity: cell.picked && !cell.held ? 1 : 0
        visible: opacity > 0.01
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
    }

    Grab {
        id: grab
        space: cell.editor
        onTapped: cell.editor.tapTile(cell.tileId)
        onLifted: (at) => cell.editor.liftTile(cell.tileId, at, cell.x, cell.y)
        onDragged: (at) => cell.editor.dragTo(at)
        onDropped: (at) => cell.editor.drop(at)
        onCancelled: cell.editor.cancelDrag()
    }

    // android's resize handle: pull it past half a cell to change the size, or tap it
    Rectangle {
        id: handle
        x: cell.width - 5
        anchors.verticalCenter: parent.verticalCenter
        width: 6
        height: sizer.pressed ? 32 : 24
        radius: 3
        color: Md.accent
        opacity: ring.opacity
        visible: ring.visible
        Behavior on height {
            enabled: !Md.still
            Spring { speed: "fast" }
        }

        MouseArea {
            id: sizer
            anchors.centerIn: parent
            width: 28
            height: 56
            enabled: cell.picked
            preventStealing: true
            cursorShape: Qt.SizeHorCursor
            onPressed: function (mouse) {
                cell.editor.beginSize(cell.tileId, sizer.mapToItem(cell.editor, mouse.x, mouse.y).x);
            }
            onPositionChanged: function (mouse) {
                if (sizer.pressed)
                    cell.editor.sizeTo(sizer.mapToItem(cell.editor, mouse.x, mouse.y).x);
            }
            onReleased: cell.editor.endSize(true)
            onCanceled: cell.editor.endSize(false)
        }

        Accessible.role: Accessible.Button
        Accessible.name: cell.place && cell.place.large ? I18n.tr("Make small") : I18n.tr("Make large")
    }
}
