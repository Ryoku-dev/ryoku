pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.components.effects
import caelestia.services

/// The card a utilities-drawer modal puts over its scrim: click-outside dismissal,
/// a scrim that tracks the drawer's deform animation, and the scale-in animation.
/// Content declared on an instance is laid out in the card's padded column, so a
/// modal only has to declare its own title, body and buttons.
Item {
    id: root

    /// Whether the modal is showing. Callers create this component only while it is,
    /// so it is read once, at creation, to seed the opening animation.
    required property bool open
    required property matrix4x4 deformMatrix
    /// Where the instance's own content is laid out.
    default property alias content: cardLayout.data

    /// The user clicked outside the card.
    signal dismissed()

    anchors.fill: parent

    DrawerScrim {
        deformMatrix: root.deformMatrix
    }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        onClicked: root.dismissed()
    }

    StyledRect {
        anchors.centerIn: parent
        radius: Tokens.rounding.extraLarge
        color: Colours.palette.m3surfaceContainerHigh

        scale: 0
        Component.onCompleted: scale = Qt.binding(() => root.open ? 1 : 0)

        width: Math.min(parent.width - Tokens.padding.extraLargeIncreased, implicitWidth)
        implicitWidth: cardLayout.implicitWidth + Tokens.padding.extraExtraLarge
        implicitHeight: cardLayout.implicitHeight + Tokens.padding.extraExtraLarge

        // Swallows clicks that would otherwise reach the dismiss area behind the card.
        MouseArea {
            anchors.fill: parent
        }

        Elevation {
            anchors.fill: parent
            radius: parent.radius
            z: -1
            level: 3
        }

        ColumnLayout {
            id: cardLayout

            anchors.fill: parent
            anchors.margins: Tokens.padding.large * 1.5
            spacing: Tokens.spacing.medium
        }

        Behavior on scale {
            Anim {}
        }
    }
}
