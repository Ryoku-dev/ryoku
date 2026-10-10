pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

Rectangle {
    id: row

    property var host: null
    signal tapped()
    signal connect()

    readonly property string alias: host ? (host.alias || "") : ""
    readonly property string state: alias.length > 0 ? Remotes.stateOf(alias) : "unknown"
    readonly property var reach: alias.length > 0 ? Remotes.reachOf(alias) : null
    readonly property var health: alias.length > 0 ? Remotes.healthOf(alias) : null
    readonly property bool selected: alias.length > 0 && Remotes.selectedAlias === alias

    implicitHeight: 68
    color: selected ? Tokens.bone : "transparent"
    antialiasing: false

    Rectangle {
        anchors.bottom: parent.bottom
        width: parent.width
        height: 1
        color: row.selected ? Tokens.inkOnBone : Tokens.lineSoft
        opacity: 0.35
    }

    Column {
        anchors.left: parent.left
        anchors.leftMargin: Tokens.s3
        anchors.right: chart.left
        anchors.rightMargin: Tokens.s2
        anchors.verticalCenter: parent.verticalCenter
        spacing: 3
        Text {
            width: parent.width
            text: row.alias
            elide: Text.ElideRight
            color: row.selected ? Tokens.inkOnBone : Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: 12
            font.weight: Font.Medium
        }
        Text {
            width: parent.width
            text: (row.host && row.host.user ? row.host.user + "@" : "")
                + (row.host ? (row.host.hostName || row.alias) : row.alias)
            elide: Text.ElideRight
            color: row.selected ? Tokens.inkOnBone : Tokens.inkMuted
            opacity: row.selected ? 0.72 : 1
            font.family: Tokens.mono
            font.pixelSize: 9
        }
    }

    MetricSparkline {
        id: chart
        anchors.right: status.left
        anchors.rightMargin: Tokens.s3
        anchors.verticalCenter: parent.verticalCenter
        width: 76
        height: 30
        values: Remotes.series(row.alias, "cpu")
        fixedMax: 100
        stroke: row.selected ? Tokens.inkOnBone : Tokens.inkMuted
    }

    Column {
        id: status
        anchors.right: open.left
        anchors.rightMargin: Tokens.s3
        anchors.verticalCenter: parent.verticalCenter
        width: 54
        spacing: 3
        Text {
            anchors.right: parent.right
            text: I18n.tr(row.state.toUpperCase())
            color: row.selected ? Tokens.inkOnBone : Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: 8
        }
        Text {
            anchors.right: parent.right
            text: row.reach && row.reach.up ? row.reach.rttMs + " ms" : "-"
            color: row.selected ? Tokens.inkOnBone : Tokens.inkFaint
            opacity: row.selected ? 0.72 : 1
            font.family: Tokens.mono
            font.pixelSize: 8
        }
    }

    Btn {
        id: open
        z: 1
        anchors.right: parent.right
        anchors.rightMargin: Tokens.s2
        anchors.verticalCenter: parent.verticalCenter
        compact: true
        text: I18n.tr("CONNECT")
        primary: row.selected
        onAct: row.connect()
    }

    MouseArea {
        anchors.fill: parent
        z: 0
        cursorShape: Qt.PointingHandCursor
        onClicked: row.tapped()
    }
}
