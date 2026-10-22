pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.utils

RowLayout {
    id: root

    property real rowScale: 1.0

    Layout.fillWidth: true
    Layout.rightMargin: Tokens.padding.extraSmall * root.rowScale
    spacing: Tokens.spacing.small * root.rowScale

    ParallelAnimation {
        running: true

        Anim {
            target: root
            property: "opacity"
            from: 0
            to: 1
            type: Anim.DefaultEffects
        }

        Anim {
            target: root
            property: "scale"
            from: 0.7
            to: 1
            type: Anim.DefaultSpatial
        }
    }
}
