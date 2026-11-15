import QtQuick
import Lithic
import "../../Singletons"

// Animations' preview: a window that opens and closes over and over in the effect and at the
// speed set on the page. With window animations off, or motion reduced, it simply stands
MiniDesk {
    id: root

    function v(bind, fallback) {
        Changes.revision;
        var said = Changes.value(bind);
        return said === undefined || said === null ? fallback : said;
    }
    readonly property string style: root.v({ store: "desktop", key: "appearance.windowStyle" }, "pop")
    readonly property bool moving: root.v({ store: "desktop", key: "appearance.animations" }, true) === true
        && root.v({ store: "settings", key: "theme.motion.reduce", fallback: false }, false) !== true
    readonly property real speed: Math.max(0.25, Number(root.v({ store: "settings", key: "theme.motion.scale", fallback: 1 }, 1)))
    readonly property int span: Math.round(420 / root.speed)

    // 0 is shut, 1 is open
    property real shown: 1

    SequentialAnimation {
        running: root.moving && root.visible
        loops: Animation.Infinite
        onRunningChanged: {
            if (!running)
                root.shown = 1;
        }
        PauseAnimation { duration: 1100 }
        NumberAnimation { target: root; property: "shown"; to: 0; duration: root.span; easing.type: Easing.InCubic }
        PauseAnimation { duration: 500 }
        NumberAnimation { target: root; property: "shown"; to: 1; duration: root.span; easing.type: root.style === "pop" ? Easing.OutBack : Easing.OutCubic }
    }

    MiniWindow {
        id: win
        x: (parent.width - width) / 2
        y: (parent.height - height) / 2 + (root.style === "slide" ? (1 - root.shown) * parent.height * 0.6 : 0)
        width: parent.width * 0.62
        height: parent.height * 0.66
        radius: root.v({ store: "desktop", key: "appearance.rounding" }, 0)
        opacity: root.style === "gnomed" ? Math.min(1, root.shown * 2) : root.shown
        transform: Scale {
            origin.x: win.width / 2
            origin.y: win.height / 2
            xScale: root.style === "pop" ? 0.82 + 0.18 * root.shown : root.style === "gnomed" ? 0.6 + 0.4 * root.shown : 1
            yScale: root.style === "pop" ? 0.82 + 0.18 * root.shown : root.style === "gnomed" ? Math.max(0.02, root.shown) : 1
        }
    }
}
