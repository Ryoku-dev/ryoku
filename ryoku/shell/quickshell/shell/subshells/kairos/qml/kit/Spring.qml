import QtQuick
import shell.services

// material 3 expressive spatial motion as a per-frame curve. not SpringAnimation: qt steps that
// in whole 16 ms ticks, so on a 165 hz screen it moves every second or third frame
NumberAnimation {
    // fast | default | slow, the M3E expressive spatial tokens
    property string speed: "default"

    duration: Motion.dur(speed === "fast" ? 350 : speed === "slow" ? 650 : 500)
    easing.type: Easing.BezierSpline
    easing.bezierCurve: speed === "fast" ? [0.42, 1.67, 0.21, 0.90, 1, 1]
        : speed === "slow" ? [0.39, 1.29, 0.35, 0.98, 1, 1]
        : [0.38, 1.21, 0.22, 1.00, 1, 1]
}
