import QtQuick
import QtQuick.Effects

// The picker draws this blur itself so a sheet's backdrop looks the same on every compositor.
Item {
    id: root

    required property PickerState state

    readonly property bool wanted: root.state.sheet !== "" && !root.state.sceneThrough
        && root.state.sheet !== "settings" && root.state.sheet !== "browser"
    property real reveal: root.wanted ? 1 : 0
    Behavior on reveal { NumberAnimation { duration: Theme.standard; easing.type: Theme.revealEasing } }
    visible: root.reveal > 0.001 && img.status === Image.Ready
    opacity: root.reveal

    readonly property string key: {
        var map = Library.currentByOutput
        var k = map[root.state.monitor]
        if (!k) {
            for (var name in map) {
                k = map[name]
                break
            }
        }
        return k || ""
    }
    readonly property string thumb: {
        if (!root.key)
            return ""
        var e = Library.entry("wallpapers", root.key)
        return e && e.thumb ? Library.fileUrl(e.thumb) : ""
    }

    // Blurred small and scaled up: the blur hides the upscale and costs a fraction of a fullscreen pass.
    Item {
        width: 480
        height: root.width > 0 ? Math.round(480 * root.height / root.width) : 270
        scale: root.width / 480
        transformOrigin: Item.TopLeft

        Image {
            id: img
            anchors.fill: parent
            source: root.wanted || root.reveal > 0 ? root.thumb : ""
            sourceSize: Qt.size(640, 360)
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            visible: false
        }
        MultiEffect {
            anchors.fill: parent
            source: img
            autoPaddingEnabled: false
            blurEnabled: true
            blur: 1.0
            blurMax: 32
            saturation: -0.1
        }
    }
}
