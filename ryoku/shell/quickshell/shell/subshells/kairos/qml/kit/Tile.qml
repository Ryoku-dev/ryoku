import QtQuick

Item {
    id: tile

    property string icon: ""
    property string label: ""
    property string secondary: ""
    property bool active: false
    property bool available: true
    property bool large: true
    property bool split: false
    property bool hasDetail: false
    signal toggled()
    signal opened()

    readonly property color body: tile.active ? Md.accent : Md.containerHigh
    readonly property color ink: tile.active ? Md.onAccent : Md.ink
    readonly property bool pressed: whole.pressed || discHit.pressed
    readonly property real rest: tile.active ? height / 2 : Md.rXl

    implicitHeight: 72
    enabled: tile.available
    opacity: tile.available ? 1 : 0.38

    Accessible.role: Accessible.Button
    Accessible.name: tile.label
    Accessible.description: tile.secondary
    Accessible.checkable: true
    Accessible.checked: tile.active

    Rectangle {
        id: shape
        anchors.fill: parent
        radius: whole.pressed ? Md.rLg : tile.rest
        color: tile.body
        Behavior on radius {
            enabled: !Md.still
            Spring { speed: "fast" }
        }
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.standard }
        }
    }

    StateLayer {
        id: whole
        radius: shape.radius
        tint: tile.ink
        onActivated: (tile.split && tile.large) ? tile.opened() : tile.toggled()
        onSecondary: if (tile.hasDetail) tile.opened()
    }

    Rectangle {
        id: disc
        visible: tile.large
        anchors { left: parent.left; leftMargin: 12; verticalCenter: parent.verticalCenter }
        width: 48
        height: 48
        radius: discHit.pressed ? Md.rMd : (tile.split ? 24 : 0)
        color: tile.split
            ? (tile.active ? Md.alpha(Md.onAccent, 0.16) : Md.containerHighest)
            : "transparent"
        Behavior on radius {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }

        Icon {
            anchors.centerIn: parent
            name: tile.icon
            size: 22
            filled: tile.active
            color: tile.ink
        }

        StateLayer {
            id: discHit
            enabled: tile.split
            visible: tile.split
            radius: disc.radius
            tint: tile.ink
            onActivated: tile.toggled()
        }
    }

    Column {
        visible: tile.large
        anchors {
            left: disc.right; leftMargin: tile.split ? 12 : 4
            right: chevron.visible ? chevron.left : parent.right
            rightMargin: chevron.visible ? 4 : 16
            verticalCenter: parent.verticalCenter
        }
        spacing: 1

        Label {
            width: parent.width
            role: "title"
            font.pixelSize: 15
            text: tile.label
            color: tile.ink
        }
        Label {
            width: parent.width
            visible: text.length > 0
            role: "caption"
            text: tile.secondary
            color: tile.ink
            opacity: 0.78
        }
    }

    Icon {
        id: chevron
        visible: tile.large && tile.split
        anchors { right: parent.right; rightMargin: 14; verticalCenter: parent.verticalCenter }
        name: "chevron_right"
        size: 20
        color: tile.ink
        opacity: 0.8
    }

    Column {
        visible: !tile.large
        anchors.centerIn: parent
        width: parent.width - 12
        spacing: 4

        Icon {
            anchors.horizontalCenter: parent.horizontalCenter
            name: tile.icon
            size: 22
            filled: tile.active
            color: tile.ink
        }
        Label {
            width: parent.width
            horizontalAlignment: Text.AlignHCenter
            role: "caption"
            font.pixelSize: 11
            text: tile.label
            color: tile.ink
            opacity: 0.85
        }
    }
}
