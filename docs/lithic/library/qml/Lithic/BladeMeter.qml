import QtQuick

Blade {
    id: root

    property real value: 0
    property color bladeInk: Lithic.fg2
    property real alertAt: LithicTokens.size.bladeMeterAlertAt

    implicitHeight: LithicTokens.size.bladeTrack
    fraction: Math.max(0, Math.min(1, value))
    fill: value >= alertAt ? Lithic.amber : bladeInk
    track: Lithic.stone3

    ChiselColorBehavior on fill {
        duration: LithicTokens.motion.select
    }
}
