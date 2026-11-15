import QtQuick
import Lithic

// a value that follows its target smoothly and at once: each frame it closes a share of the
// gap that depends only on the time passed, so it never overshoots, never runs away, and
// looks the same at 60 Hz or 165 Hz. It runs only while it has a gap to close; with reduced
// motion it jumps
QtObject {
    id: root

    property real target: 0
    // how fast the gap closes, per second: 28 closes 90% of it in about 80 ms
    property real rate: 28
    property real epsilon: 0.001
    property real value: 0

    // stand at v at once, as a first placement should
    function snap(v) {
        root.target = v;
        root.value = v;
    }

    onTargetChanged: {
        if (Lithic.reduceMotion)
            root.value = root.target;
    }

    property FrameAnimation frames: FrameAnimation {
        running: Math.abs(root.target - root.value) > root.epsilon && !Lithic.reduceMotion
        onTriggered: {
            var step = 1 - Math.exp(-Math.min(0.1, frameTime) * root.rate);
            var next = root.value + (root.target - root.value) * step;
            root.value = Math.abs(root.target - next) <= root.epsilon ? root.target : next;
        }
    }
}
