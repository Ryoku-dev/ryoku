import QtQuick
import QtQuick.Effects
import caelestia.components
import caelestia.services

RectangularShadow {
    property int level
    property real dp: [0, 1, 3, 6, 8, 12][level]

    // Qt 6.10 compat: per-corner radii are not exposed by RectangularShadow yet.
    // Declaring them keeps callers working; the shadow itself uses `radius`.
    property real topLeftRadius: -1
    property real topRightRadius: -1
    property real bottomLeftRadius: -1
    property real bottomRightRadius: -1

    color: level === 0 ? "transparent" : Qt.alpha(Colours.palette.m3shadow, 0.7)
    blur: (dp * 5) ** 0.7
    spread: -dp * 0.3 + (dp * 0.1) ** 2
    offset.y: dp / 2

    Behavior on dp {
        Anim {
            type: Anim.SlowEffects
        }
    }
}
