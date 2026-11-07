import QtQuick
import shell.services
import shell.barkit as Pill

// A quick-settings tile: accent disc, title over a status line, optional chevron.
Rectangle {
    id: tile

    property string glyph: ""
    property string icon: ""
    property string title: ""
    property string subtitle: ""
    property bool chevron: false
    signal clicked()

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    readonly property color accent: Theme.primary
    function dim(a) { return Qt.rgba(tile.ink.r, tile.ink.g, tile.ink.b, a) }

    radius: 20
    color: hover.hovered ? tile.dim(0.10) : tile.dim(0.06)
    Behavior on color {
        enabled: !Motion.reduce
        ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
    }

    Rectangle {
        id: disc
        anchors { left: parent.left; leftMargin: 10; verticalCenter: parent.verticalCenter }
        width: 46
        height: 46
        radius: 23
        color: tile.accent

        Pill.GlyphIcon {
            anchors.centerIn: parent
            width: 22
            height: 22
            visible: tile.glyph.length > 0
            name: tile.glyph
            color: tile.fill
        }
        Pill.MaterialIcon {
            anchors.centerIn: parent
            visible: tile.icon.length > 0
            text: tile.icon
            color: tile.fill
            font.pixelSize: 21
        }
    }

    Column {
        anchors {
            left: disc.right; leftMargin: 12
            right: parent.right; rightMargin: tile.chevron ? 40 : 14
            verticalCenter: parent.verticalCenter
        }
        spacing: 1

        Text {
            width: parent.width
            text: tile.title
            color: tile.ink
            font.family: Theme.fontPrimary
            font.pixelSize: 15
            font.weight: Font.DemiBold
            elide: Text.ElideRight
        }
        Text {
            width: parent.width
            visible: tile.subtitle.length > 0
            text: tile.subtitle
            color: tile.dim(0.6)
            font.family: Theme.fontPrimary
            font.pixelSize: 12
            elide: Text.ElideRight
        }
    }

    Pill.MaterialIcon {
        anchors { right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
        visible: tile.chevron
        text: "chevron_right"
        color: tile.dim(0.5)
        font.pixelSize: 20
    }

    HoverHandler { id: hover }
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: tile.clicked()
    }
}
