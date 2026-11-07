pragma Singleton

import QtQuick
import Quickshell
import shell.services
import Ryoku.Ui.Singletons
import "../settings" as S

Singleton {
    readonly property real scale: S.Prefs.scale
    readonly property real opacity: S.Prefs.opacity
    readonly property string font: Theme.fontPrimary
    readonly property string jp: Tokens.jp
    readonly property string mono: Tokens.mono
    readonly property string display: Tokens.display
    readonly property var tabular: ({ "tnum": 1 })

    readonly property color base: Theme.surface
    readonly property color surface: base
    readonly property color ink: Theme.inkOn(base, Tokens.ink, 7)
    readonly property color inkDim: Theme.inkOn(base, Tokens.inkDim, 4.5)
    readonly property color inkMuted: alpha(inkDim, 0.88)
    readonly property color inkFaint: alpha(inkDim, 0.68)
    readonly property color bone: Tokens.bone
    readonly property color inkOnBone: Theme.inkOn(Tokens.bone, Tokens.inkOnBone, 4.5)
    readonly property color line: alpha(ink, 0.26)
    readonly property color lineSoft: alpha(ink, 0.13)
    readonly property color lineStrong: alpha(ink, 0.42)
    readonly property color tint5: alpha(ink, 0.05)
    readonly property color tint10: alpha(ink, 0.10)
    readonly property color tint16: alpha(ink, 0.16)
    readonly property color plate: tint5
    readonly property color alert: Tokens.alert
    readonly property color accent: Theme.primary
    readonly property color rim: alpha(ink, 0.12)
    readonly property color trace: alpha(ink, 0.78)

    readonly property int barText: Math.round(13 * scale)
    readonly property int barSub: Math.round(11 * scale)
    readonly property int barIcon: Math.round(18 * scale)
    readonly property int caption: 11
    readonly property int label: 12
    readonly property int body: 13
    readonly property int title: 16
    readonly property int headline: 22
    readonly property int displaySize: 40
    readonly property int micro: 11
    readonly property int tiny: 9
    readonly property real track: Tokens.trackLabel
    readonly property int icon: 20

    readonly property int radius: Tokens.radius
    readonly property int plateRadius: 9
    readonly property int liquidRadius: Math.round(14 * scale)
    readonly property int squareRadius: Math.round(5 * scale)
    readonly property int sheetRadius: 14
    readonly property real border: 1
    readonly property int s1: 4
    readonly property int s2: 8
    readonly property int s3: 12
    readonly property int s4: 16
    readonly property int s5: 24
    readonly property int s6: 32
    readonly property int s7: 48
    readonly property int rowH: 44
    readonly property int ctlH: 26

    readonly property int barH: Math.round(36 * scale)
    readonly property real barPad: 12 * scale
    readonly property real barGap: 4 * scale
    readonly property real hitPad: 6 * scale

    readonly property bool still: Motion.reduce
    readonly property var spatialCurve: [0.38, 1.21, 0.22, 1.00, 1, 1]
    readonly property var spatialFastCurve: [0.42, 1.67, 0.21, 0.90, 1, 1]
    readonly property var effectsCurve: [0.34, 0.80, 0.34, 1.00, 1, 1]
    readonly property var enterCurve: [0.05, 0.70, 0.10, 1.00, 1, 1]
    readonly property var exitCurve: [0.30, 0.00, 0.80, 0.15, 1, 1]
    readonly property int spatial: ms(500)
    readonly property int spatialFast: ms(350)
    readonly property int effects: ms(200)
    readonly property int enter: ms(400)
    readonly property int exit: ms(200)
    readonly property int quick: ms(120)
    readonly property int snap: Tokens.snap
    readonly property int move: Tokens.move
    readonly property int swap: Tokens.swap

    function alpha(color, amount) {
        const value = Math.max(0, Math.min(1, Number(amount)));
        return Qt.rgba(color.r, color.g, color.b, color.a * value);
    }

    function ms(duration) {
        return still ? 0 : duration;
    }
}
