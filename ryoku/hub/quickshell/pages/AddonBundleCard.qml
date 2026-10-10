pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Rectangle {
    id: card

    required property var bundle
    property bool busy: false

    signal openRequested()
    signal repairRequested()
    signal removeRequested()

    readonly property int installedCount: Number(card.bundle.installedCount || 0)
    readonly property int totalCount: Number(card.bundle.totalCount
        || ((card.bundle.metadata || {}).items || []).length)
    readonly property int missingCount: Math.max(0, totalCount - installedCount)
    readonly property string summary: card.bundle.summary || card.bundle.description
        || I18n.tr("Installed add-on bundle.")

    height: 168
    radius: Tokens.radius
    color: hover.hovered ? Tokens.tint5 : "transparent"
    border.width: Tokens.border
    border.color: hover.hovered ? Tokens.lineStrong : Tokens.line

    Behavior on color { ColorAnimation { duration: Tokens.snap } }
    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
    HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }

    Item {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: actionBar.top
        z: 1
        TapHandler { onTapped: card.openRequested() }
    }

    Rectangle {
        id: iconPlate
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.margins: Tokens.s4
        width: 40
        height: 40
        radius: Tokens.radius
        color: card.missingCount === 0 ? Tokens.bone : "transparent"
        border.width: card.missingCount === 0 ? 0 : Tokens.border
        border.color: Tokens.lineStrong

        Text {
            anchors.centerIn: parent
            text: "inventory_2"
            color: card.missingCount === 0 ? Tokens.inkOnBone : Tokens.inkDim
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
            text: card.bundle.name || card.bundle.id
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
            width: componentLabel.implicitWidth + Tokens.s3 * 2
            height: 22
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            Text {
                id: componentLabel
                anchors.centerIn: parent
                text: (card.totalCount === 1
                    ? I18n.tr("%1 component") : I18n.tr("%1 components"))
                    .arg(card.totalCount)
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny
            }
        }
        Rectangle {
            visible: String(card.bundle.version || "") !== ""
            width: bundleVersion.implicitWidth + Tokens.s3 * 2
            height: 22
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            Text {
                id: bundleVersion
                anchors.centerIn: parent
                text: I18n.tr("Version %1").arg(card.bundle.version || "")
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

        Item {
            anchors.left: parent.left
            anchors.right: actions.left
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            z: 1
            TapHandler { onTapped: card.openRequested() }
        }

        Rectangle {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            width: statusLabel.implicitWidth + Tokens.s3 * 2
            height: 22
            radius: Tokens.radius
            color: card.missingCount === 0 ? Tokens.bone : "transparent"
            border.width: card.missingCount === 0 ? 0 : Tokens.border
            border.color: Tokens.line
            Text {
                id: statusLabel
                anchors.centerIn: parent
                text: card.missingCount === 0
                    ? I18n.tr("Complete") : I18n.tr("%1 missing").arg(card.missingCount)
                color: card.missingCount === 0 ? Tokens.inkOnBone : Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny
            }
        }

        Row {
            id: actions
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Btn {
                visible: card.missingCount > 0
                compact: true
                text: card.busy ? I18n.tr("WORKING") : I18n.tr("REPAIR")
                armed: !card.busy
                onAct: card.repairRequested()
            }
            Btn {
                visible: card.installedCount > 0
                compact: true
                text: card.busy ? I18n.tr("WORKING") : I18n.tr("REMOVE")
                armed: !card.busy
                onAct: card.removeRequested()
            }
        }
    }
}
