pragma Singleton

import QtQuick
import shell.services

// The Game Bar overlay follows Ryoku's rounding, type scale and
// matugen roles, so it tracks the wallpaper instead of a fixed palette.
QtObject {
    id: root

    function alpha(c, a) {
        return Qt.rgba(c.r, c.g, c.b, a);
    }

    readonly property color bar: Theme.surfaceContainer
    readonly property color panel: Theme.surfaceContainerLow
    readonly property color content: Theme.surfaceContainer
    readonly property color raised: Theme.surfaceContainerHigh
    readonly property color raisedHover: Theme.surfaceContainerHighest
    readonly property color border: Theme.outlineVariant
    readonly property color fg: Theme.onSurface
    readonly property color onLayer: Theme.onSurfaceVariant
    readonly property color subtext: Theme.outline
    readonly property color toggled: Theme.secondaryContainer
    readonly property color toggledInk: Theme.onSecondaryContainer
    readonly property color accent: Theme.primary
    readonly property color accentInk: Theme.onPrimary
    readonly property color danger: Theme.error
    readonly property color onDanger: Theme.onError

    readonly property color hover: root.alpha(Theme.onSurface, 0.06)

    // Hub -> Game Bar -> Corner rounding owns the base; the smaller radii keep
    // their distance from it so the strip, frame and chips stay in proportion.
    readonly property int radiusWindow: Config.gamebar.radius
    readonly property int radiusVeryLarge: Config.gamebar.radius + 8
    readonly property int radiusNormal: Config.gamebar.radius + 3
    readonly property int radiusSmall: Math.max(0, Config.gamebar.radius - 2)
    readonly property int radiusVerySmall: Math.max(0, Config.gamebar.radius - 6)

    readonly property int smallest: 10
    readonly property int smaller: 12
    readonly property int smallie: 13
    readonly property int small: 15
    readonly property int huge: 22

    readonly property string face: Theme.fontPrimary
}
