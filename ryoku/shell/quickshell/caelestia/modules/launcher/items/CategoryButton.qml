import QtQuick
import Caelestia.Config
import caelestia.components
import caelestia.services

Item {
    id: catRoot

    required property var modelData
    required property int index
    required property bool selected

    signal clicked()

    implicitHeight: 40

    StyledRect {
        anchors.fill: parent
        radius: Tokens.rounding.medium
        color: Colours.palette.m3onSurface
        opacity: catRoot.selected ? 0.10 : 0

        Behavior on opacity {
            Anim {
                type: Anim.DefaultEffects
            }
        }
    }

    StateLayer {
        id: catHover

        anchors.fill: parent
        radius: Tokens.rounding.medium
        acceptedButtons: Qt.LeftButton
        onClicked: catRoot.clicked()
    }

    Row {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: Tokens.padding.medium
        anchors.rightMargin: Tokens.padding.medium
        spacing: Tokens.spacing.medium

        MaterialIcon {
            anchors.verticalCenter: parent.verticalCenter
            text: catRoot.modelData?.icon ?? "apps"
            color: catRoot.selected ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
            fontStyle: Tokens.font.icon.medium
        }

        StyledText {
            anchors.verticalCenter: parent.verticalCenter
            text: catRoot.modelData?.name ?? ""
            color: catRoot.selected ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
            font: Tokens.font.label.large
            elide: Text.ElideRight
            width: catRoot.width - Tokens.padding.medium * 2 - 24 - Tokens.spacing.medium
        }
    }
}
