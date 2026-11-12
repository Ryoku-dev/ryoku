pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Rectangle {
    id: card

    required property var plugin
    property string hostText: ""
    property string updateVersion: ""
    property bool busy: false

    signal settingsRequested()
    signal enabledRequested(bool enabled)
    signal removeRequested()

    readonly property var manifest: card.plugin.manifest || ({})
    readonly property var placement: card.plugin.placement || ({})
    readonly property bool pluginEnabled: card.placement.enabled === true
    readonly property string summary: card.manifest.summary || card.manifest.description
        || I18n.tr("Installed shell add-on.")
    readonly property string host: card.placement.host
        || ((card.manifest.defaults && card.manifest.defaults.host)
            ? card.manifest.defaults.host : "framePopout")
    readonly property string glyph: card.host === "topbarGlyph" ? "view_day"
        : card.host === "sidebarCard" ? "dock_to_right"
        : card.host === "framePopout" ? "open_in_new"
        : "extension"

    height: 168
    radius: Tokens.radius
    color: hover.hovered ? Tokens.tint5 : "transparent"
    border.width: Tokens.border
    border.color: hover.hovered ? Tokens.lineStrong : Tokens.line

    Behavior on color { ColorAnimation { duration: Tokens.snap } }
    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
    HoverHandler { id: hover }

    Rectangle {
        id: iconPlate
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.margins: Tokens.s4
        width: 40
        height: 40
        radius: Tokens.radius
        color: card.pluginEnabled ? Tokens.bone : "transparent"
        border.width: card.pluginEnabled ? 0 : Tokens.border
        border.color: Tokens.lineStrong

        Text {
            anchors.centerIn: parent
            text: card.glyph
            color: card.pluginEnabled ? Tokens.inkOnBone : Tokens.inkDim
            font.family: "Material Symbols Rounded"
            font.pixelSize: 21
        }
    }

    Column {
        anchors.left: iconPlate.right
        anchors.leftMargin: Tokens.s3
        anchors.right: parent.right
        anchors.rightMargin: Tokens.s4
        anchors.top: parent.top
        anchors.topMargin: Tokens.s4
        spacing: Tokens.s1

        Text {
            width: parent.width
            text: card.manifest.name || card.plugin.id
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fRow
            font.weight: Font.Medium
            elide: Text.ElideRight
        }
        Text {
            width: parent.width
            text: card.summary
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            elide: Text.ElideRight
        }
    }

    Row {
        anchors.left: parent.left
        anchors.leftMargin: Tokens.s4
        anchors.top: iconPlate.bottom
        anchors.topMargin: Tokens.s3
        spacing: Tokens.s2

        Rectangle {
            width: hostLabel.implicitWidth + Tokens.s3 * 2
            height: 22
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            Text {
                id: hostLabel
                anchors.centerIn: parent
                text: card.hostText
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny
            }
        }
        Rectangle {
            visible: card.manifest.version
            width: versionLabel.implicitWidth + Tokens.s3 * 2
            height: 22
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            Text {
                id: versionLabel
                anchors.centerIn: parent
                text: I18n.tr("Version %1").arg(card.manifest.version || "")
                color: Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
            }
        }
        Rectangle {
            visible: card.updateVersion !== ""
            width: updateLabel.implicitWidth + Tokens.s3 * 2
            height: 22
            radius: Tokens.radius
            color: Tokens.bone
            Text {
                id: updateLabel
                anchors.centerIn: parent
                text: I18n.tr("Update %1").arg(card.updateVersion)
                color: Tokens.inkOnBone
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
            }
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: actionBar.top
        height: 1
        color: Tokens.lineSoft
    }

    Item {
        id: actionBar
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 48

        Row {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: card.pluginEnabled ? I18n.tr("Enabled") : I18n.tr("Disabled")
                color: card.pluginEnabled ? Tokens.ink : Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny
            }
            Sw {
                anchors.verticalCenter: parent.verticalCenter
                on: card.pluginEnabled
                enabled: !card.busy
                onToggled: value => card.enabledRequested(value)
            }
        }

        Row {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Btn {
                compact: true
                text: I18n.tr("SETTINGS")
                armed: !card.busy
                onAct: card.settingsRequested()
            }
            Btn {
                compact: true
                text: card.busy ? I18n.tr("WORKING") : I18n.tr("REMOVE")
                armed: !card.busy
                onAct: card.removeRequested()
            }
        }
    }
}
