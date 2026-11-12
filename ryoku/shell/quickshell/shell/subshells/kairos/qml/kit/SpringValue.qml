import QtQuick
import shell.services

// a damped spring stepped on every rendered frame, so it runs at the screen's own rate and a
// retarget mid-flight keeps its velocity, the way android's springs hand off
QtObject {
    id: spring

    property real target: 0
    // fast | default | slow, the M3E spatial springs (damping ratio, stiffness)
    property string speed: "default"
    property real value: spring.target
    property real velocity: 0
    readonly property bool moving: driver.running

    readonly property real ratio: spring.speed === "fast" ? 0.6 : 0.8
    readonly property real stiffness: (spring.speed === "fast" ? 800 : spring.speed === "slow" ? 200 : 380)
        / Math.max(0.05, Motion.speed)

    onTargetChanged: {
        if (Motion.reduce) {
            driver.stop();
            spring.velocity = 0;
            spring.value = spring.target;
            return;
        }
        driver.start();
    }
    function snap() {
        driver.stop();
        spring.velocity = 0;
        spring.value = spring.target;
    }

    property FrameAnimation driver: FrameAnimation {
        onTriggered: {
            // a long hitch is stepped in small pieces so a stiff spring can't blow up
            var dt = Math.min(0.05, driver.frameTime);
            var steps = Math.max(1, Math.ceil(dt / 0.004));
            var h = dt / steps;
            var k = spring.stiffness, c = 2 * spring.ratio * Math.sqrt(k);
            var x = spring.value, v = spring.velocity;
            for (var i = 0; i < steps; i++) {
                v += (-k * (x - spring.target) - c * v) * h;
                x += v * h;
            }
            if (Math.abs(x - spring.target) < 0.0005 && Math.abs(v) < 0.005) {
                x = spring.target;
                v = 0;
                driver.stop();
            }
            spring.velocity = v;
            spring.value = x;
        }
    }
}
