pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../../kit"
import ".." as Q

// one of the hover glance's chips, drawn the way the glance draws it
Item {
    id: chip

    property var editor
    property string tileId: ""

    readonly property int at: chip.editor.chipView.indexOf(chip.tileId)
    readonly property bool held: chip.editor.dragKind === "chip" && chip.editor.dragId === chip.tileId
    readonly property bool tracking: chip.held && chip.editor.tracking
    readonly property bool picked: chip.at >= 0 && chip.editor.pickedKind === "chip" && chip.editor.picked === chip.tileId
    readonly property bool preview: chip.editor.dragKind === "tile" && chip.editor.dragId === chip.tileId
    readonly property real wantX: chip.at >= 0 ? chip.editor.chipX(chip.at) : -1
    readonly property real wantY: chip.editor.chipY
    property real restX: 0
    property real restY: 0
    property bool jump: true

    // a removed chip fades where it was dropped, an added one appears in its slot
    function settle() {
        if (chip.at < 0) {
            if (chip.held) {
                chip.restX = chip.editor.gx;
                chip.restY = chip.editor.gy;
            }
            return;
        }
        chip.jump = chip.opacity < 0.05;
        chip.restX = chip.wantX;
        chip.restY = chip.wantY;
        chip.jump = false;
    }
    onWantXChanged: chip.settle()
    onWantYChanged: chip.settle()
    Component.onCompleted: chip.settle()

    x: chip.held ? chip.editor.gx : chip.restX
    y: chip.held ? chip.editor.gy : chip.restY
    width: chip.editor.chipSize
    height: chip.editor.chipSize
    z: chip.held ? 12 : chip.picked ? 5 : 4
    scale: chip.held ? 1.12 : chip.at >= chip.editor.glanceMax ? 0.7 : 1
    opacity: chip.at < 0 ? 0
        : chip.at >= chip.editor.glanceMax ? 0.3
        : chip.preview ? 0.6
        : spec.available ? 1 : 0.38
    visible: opacity > 0.01

    Behavior on x {
        enabled: !Md.still && !chip.tracking && !chip.jump
        Spring {}
    }
    Behavior on y {
        enabled: !Md.still && !chip.tracking && !chip.jump
        Spring {}
    }
    Behavior on scale {
        enabled: !Md.still
        Spring { speed: "fast" }
    }
    Behavior on opacity {
        enabled: !Md.still
        NumberAnimation { duration: Md.fast }
    }

    Q.TileSpec { id: spec; tileId: chip.tileId }

    Accessible.role: Accessible.Button
    Accessible.name: spec.label

    Rectangle {
        id: body
        anchors.fill: parent
        radius: grab.pressed ? Md.rMd : (spec.active ? Md.rLg : width / 2)
        color: spec.active ? Md.accent : Md.containerHigh
        Behavior on radius {
            enabled: !Md.still
            Spring { speed: "fast" }
        }
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }
    }
    Icon {
        anchors.centerIn: parent
        name: spec.icon
        size: 22
        filled: spec.active
        color: spec.active ? Md.onAccent : Md.ink
    }
    Rectangle {
        anchors.fill: parent
        radius: body.radius
        color: spec.active ? Md.onAccent : Md.ink
        opacity: grab.pressed ? 0.14 : grab.containsMouse ? 0.08 : 0
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
    }
    Rectangle {
        anchors.fill: parent
        anchors.margins: -4
        radius: body.radius + 4
        color: "transparent"
        border.width: 2
        border.color: Md.accent
        opacity: chip.picked && !chip.held ? 1 : 0
        visible: opacity > 0.01
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
    }

    Grab {
        id: grab
        enabled: chip.at >= 0 && !chip.preview
        space: chip.editor
        onTapped: chip.editor.tapChip(chip.tileId)
        onLifted: (at) => chip.editor.liftChip(chip.tileId, at, chip.x, chip.y)
        onDragged: (at) => chip.editor.dragTo(at)
        onDropped: (at) => chip.editor.drop(at)
        onCancelled: chip.editor.cancelDrag()
    }

    Rectangle {
        id: badge
        x: chip.width - 16
        y: -6
        width: 22
        height: 22
        radius: 11
        color: Md.containerHighest
        border.width: 2
        border.color: Md.base
        opacity: chip.picked && !chip.held ? 1 : 0
        visible: opacity > 0.01
        scale: badgeHit.pressed ? 0.88 : 1
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
        Behavior on scale {
            enabled: !Md.still
            Spring { speed: "fast" }
        }

        Icon {
            anchors.centerIn: parent
            name: "close"
            size: 14
            color: Md.ink
        }
        MouseArea {
            id: badgeHit
            anchors.fill: parent
            anchors.margins: -6
            enabled: chip.picked
            cursorShape: Qt.PointingHandCursor
            onClicked: chip.editor.removeChip(chip.tileId)
        }

        Accessible.role: Accessible.Button
        Accessible.name: I18n.tr("Remove from hover")
    }
}
