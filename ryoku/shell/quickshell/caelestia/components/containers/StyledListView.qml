import QtQuick
import caelestia.components

ListView {
    id: root

    property bool doneFakeFlick
    property bool enableFakeFlick: true

    maximumFlickVelocity: 3000

    rebound: Transition {
        onRunningChanged: {
            const canScroll = root.interactive && (root.contentHeight > root.height || root.contentWidth > root.width);
            if (!running && !root.doneFakeFlick && root.enableFakeFlick && canScroll) {
                root.doneFakeFlick = true;
                root.flick(1, 1);
                root.flick(-1, -1);
                Qt.callLater(() => root.cancelFlick());
            }
        }

        Anim {
            properties: "x,y"
        }
    }

    Timer {
        running: root.doneFakeFlick
        interval: 10
        onTriggered: root.doneFakeFlick = false
    }
}
