pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Wayland
import Caelestia.Config
import caelestia.services

Item {
    id: root

    required property string address
    property bool active: true
    /// Shown until the first frame arrives, and whenever there is no capture.
    property url fallbackIcon: ""
    property real fallbackScale: 0.5
    property real sourceAspect: 16 / 9

    // The window layer hands each window the compositor's toplevel handle, which is
    // what a capture is taken from.
    readonly property var toplevel: Kwin.windowList.find(w => w.address === root.address)?.toplevel ?? null
    readonly property bool capturing: root.active && GlobalConfig.bar.livePreviews && !!root.toplevel
    readonly property bool hasStream: root.capturing && capture.hasContent

    CachingIconImage {
        anchors.centerIn: parent
        implicitSize: Math.min(root.width, root.height) * root.fallbackScale
        source: root.fallbackIcon
        visible: !root.hasStream
        z: 0
    }

    ScreencopyView {
        id: capture

        readonly property real fitted: root.sourceAspect > (root.width / Math.max(1, root.height)) ? root.width / root.sourceAspect : root.height

        anchors.centerIn: parent
        height: fitted
        width: fitted * root.sourceAspect
        captureSource: root.capturing ? root.toplevel : null
        live: false
        visible: root.hasStream
        z: 1
    }
}
