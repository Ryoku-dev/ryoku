import QtQuick

Item {
    id: row

    property string headline: ""
    property string supporting: ""
    property string icon: ""
    property real iconOpacity: 1
    property string position: "only"
    property bool selected: false
    property bool clickable: true
    property bool wrapSupporting: false
    default property alias trailing: trailingRow.data
    signal clicked()
    signal secondary()

    readonly property real outer: Md.rXl
    readonly property real inner: Md.rXs
    readonly property bool topOuter: row.position === "only" || row.position === "first"
    readonly property bool bottomOuter: row.position === "only" || row.position === "last"
    readonly property color body: row.selected ? Md.accentContainer : Md.container
    readonly property color text: row.selected ? Md.onAccentContainer : Md.ink

    implicitWidth: 320
    implicitHeight: Math.max(row.supporting.length > 0 ? 68 : 56, texts.implicitHeight + 24)

    Rectangle {
        id: shape
        anchors.fill: parent
        topLeftRadius: row.topOuter ? row.outer : row.inner
        topRightRadius: row.topOuter ? row.outer : row.inner
        bottomLeftRadius: row.bottomOuter ? row.outer : row.inner
        bottomRightRadius: row.bottomOuter ? row.outer : row.inner
        color: row.body
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }
    }

    StateLayer {
        enabled: row.clickable
        topLeftRadius: shape.topLeftRadius
        topRightRadius: shape.topRightRadius
        bottomLeftRadius: shape.bottomLeftRadius
        bottomRightRadius: shape.bottomRightRadius
        tint: row.text
        onActivated: row.clicked()
        onSecondary: row.secondary()
    }

    Rectangle {
        id: disc
        visible: row.icon.length > 0
        anchors { left: parent.left; leftMargin: 14; verticalCenter: parent.verticalCenter }
        width: 40
        height: 40
        radius: row.selected ? Md.rMd : 20
        color: row.selected ? Md.accent : Md.containerHighest
        Behavior on radius {
            enabled: !Md.still
            Spring { speed: "fast" }
        }

        Icon {
            anchors.centerIn: parent
            name: row.icon
            size: 20
            filled: row.selected
            opacity: row.iconOpacity
            color: row.selected ? Md.onAccent : Md.ink
        }
    }

    Column {
        id: texts
        anchors {
            left: disc.visible ? disc.right : parent.left
            leftMargin: disc.visible ? 14 : 18
            right: trailingRow.left
            rightMargin: trailingRow.width > 0 ? 12 : 18
            verticalCenter: parent.verticalCenter
        }
        spacing: 2

        Label {
            width: parent.width
            role: "title"
            font.pixelSize: 15
            text: row.headline
            color: row.text
        }
        Label {
            width: parent.width
            visible: text.length > 0
            role: "caption"
            text: row.supporting
            color: row.selected ? row.text : Md.inkMuted
            opacity: row.selected ? 0.8 : 1
            wrapMode: row.wrapSupporting ? Text.Wrap : Text.NoWrap
            maximumLineCount: row.wrapSupporting ? 3 : 1
        }
    }

    Row {
        id: trailingRow
        anchors { right: parent.right; rightMargin: 14; verticalCenter: parent.verticalCenter }
        spacing: 8
    }
}
