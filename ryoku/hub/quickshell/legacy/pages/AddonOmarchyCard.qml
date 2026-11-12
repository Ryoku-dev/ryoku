pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Rectangle {
    id: card

    required property var plugin
    property bool busy: false

    signal enabledRequested(bool enabled)
    signal removeRequested()

    readonly property var kinds: card.plugin.kinds || []
    readonly property string kindText: {
        const labels = card.kinds.map(kind => {
            if (kind === "bar-widget") return I18n.tr("Bar widget");
            if (kind === "bar") return I18n.tr("Bar style");
            if (kind === "panel") return I18n.tr("Panel");
            if (kind === "overlay") return I18n.tr("Overlay");
            if (kind === "menu") return I18n.tr("Menu");
            if (kind === "service") return I18n.tr("Service");
            return kind;
        });
        return labels.length > 0 ? labels.join(", ") : I18n.tr("Shell plugin");
    }
    readonly property string summary: card.plugin.description
        || I18n.tr("Installed from the Omarchy plugin market.")
    readonly property bool pluginEnabled: card.plugin.enabled === true

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
            text: "extension"
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
            text: card.plugin.name || card.plugin.id
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
            width: kindLabel.implicitWidth + Tokens.s3 * 2
            height: 22
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            Text {
                id: kindLabel
                anchors.centerIn: parent
                text: card.kindText
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny
            }
        }
        Rectangle {
            visible: String(card.plugin.version || "") !== ""
            width: omarchyVersion.implicitWidth + Tokens.s3 * 2
            height: 22
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            Text {
                id: omarchyVersion
                anchors.centerIn: parent
                text: I18n.tr("Version %1").arg(card.plugin.version || "")
                color: Tokens.inkMuted
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
                enabled: !card.busy && card.plugin.canDisable !== false
                onToggled: value => card.enabledRequested(value)
            }
        }

        Btn {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            compact: true
            text: card.busy ? I18n.tr("WORKING") : I18n.tr("REMOVE")
            armed: !card.busy
            onAct: card.removeRequested()
        }
    }
}
