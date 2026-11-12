import QtQuick
import Quickshell.Widgets
import shell.services as Ryoku
import "../kit"

Item {
    id: cell

    property var entry: null
    property bool selected: false
    // keystroke-to-result never animates
    property bool animate: true
    signal activated()
    signal secondary(real x, real y)

    readonly property bool moving: cell.animate && !Md.still

    Rectangle {
        id: plate
        anchors.fill: parent
        anchors.margins: 3
        radius: cell.selected ? Md.rXl : height / 2
        scale: cell.selected ? 1 : 0.82
        color: cell.selected ? Md.accentContainer : Md.alpha(Md.accentContainer, 0)
        Behavior on radius {
            enabled: cell.moving
            Spring { speed: "fast" }
        }
        Behavior on scale {
            enabled: cell.moving
            Spring { speed: "fast" }
        }
        Behavior on color {
            enabled: cell.moving
            ColorAnimation { duration: Md.fast }
        }
    }

    IconImage {
        id: glyph
        anchors.horizontalCenter: parent.horizontalCenter
        y: 10
        implicitSize: 56
        asynchronous: true
        source: Ryoku.Icons.path((cell.entry && cell.entry.icon) || "", "application-x-executable")
    }

    // an app whose icon the theme can't find still gets a tile, like the pixel launcher does
    Rectangle {
        anchors.centerIn: glyph
        width: 52
        height: 52
        radius: 26
        visible: String(glyph.source) === "" || glyph.status === Image.Error
        color: Md.containerHighest

        Label {
            anchors.centerIn: parent
            role: "title"
            font.pixelSize: 22
            elide: Text.ElideNone
            text: cell.entry && cell.entry.name ? cell.entry.name.charAt(0).toUpperCase() : ""
        }
    }

    Label {
        anchors { left: parent.left; right: parent.right; top: glyph.bottom; topMargin: 6 }
        leftPadding: 6
        rightPadding: 6
        role: "caption"
        horizontalAlignment: Text.AlignHCenter
        maximumLineCount: 1
        text: cell.entry ? cell.entry.name : ""
        color: cell.selected ? Md.onAccentContainer : Md.ink
        font.weight: cell.selected ? Font.Medium : Font.Normal
    }

    StateLayer {
        anchors.margins: 3
        radius: plate.radius
        ring: false
        activeFocusOnTab: false
        tint: cell.selected ? Md.onAccentContainer : Md.ink
        onActivated: cell.activated()
        onSecondary: cell.secondary(mouseX + 3, mouseY + 3)
    }
}
