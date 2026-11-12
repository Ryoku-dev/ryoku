pragma Singleton

import QtQuick
import Quickshell
import shell.services

// every kairos surface sits on the island's black body whatever the palette, light presets too
Singleton {
    id: md

    function alpha(c, a) { return Qt.rgba(c.r, c.g, c.b, a) }
    function tone(inkAmount, accentAmount) {
        return Theme.blend(md.alpha(md.accent, accentAmount),
            Theme.blend(md.alpha(md.ink, inkAmount), md.base));
    }

    readonly property color base: Theme.shadow
    readonly property color ink: Theme.ink(md.base, 7)
    readonly property color inkMuted: Theme.blend(md.alpha(md.ink, 0.74), md.base)
    readonly property color inkFaint: Theme.blend(md.alpha(md.ink, 0.52), md.base)
    readonly property color accent: Theme.inkOn(md.base, Theme.primary, 3)
    readonly property color onAccent: Theme.inkOn(md.accent, Theme.onPrimary, 4.5)
    readonly property color container: md.tone(0.08, 0.05)
    readonly property color containerHigh: md.tone(0.13, 0.08)
    readonly property color containerHighest: md.tone(0.19, 0.10)
    readonly property color accentContainer: md.tone(0.04, 0.30)
    readonly property color onAccentContainer: Theme.inkOn(md.accentContainer, md.ink, 7)
    readonly property color outline: md.alpha(md.ink, 0.22)
    readonly property color outlineSoft: md.alpha(md.ink, 0.12)
    readonly property color error: Theme.inkOn(md.base, Theme.error, 3)
    readonly property color onError: Theme.inkOn(md.error, Theme.onError, 4.5)

    readonly property int rXs: 4
    readonly property int rSm: 8
    readonly property int rMd: 12
    readonly property int rLg: 16
    readonly property int rXl: 24
    readonly property int rXxl: 32
    readonly property int seam: 2

    readonly property string font: Theme.fontPrimary
    readonly property string mono: Theme.mono

    readonly property bool still: Motion.reduce
    readonly property int fast: Motion.fast
    readonly property int standard: Motion.standard
    readonly property int morph: Motion.morph
    readonly property int spatial: Motion.spatial
}
