import QtQuick
import Lithic
import "../../Singletons"

// Window behaviour's preview: three tiled windows with the gaps between them and at the
// screen's edge set on the page, in the corners Window style gives them
MiniDesk {
    id: root

    readonly property real scaleBy: 0.4
    function v(key, fallback) {
        Changes.revision;
        var said = Changes.value({ store: "desktop", key: "appearance." + key });
        return said === undefined || said === null ? fallback : said;
    }
    property real outer: root.v("gapsOut", 0) * root.scaleBy
    property real inner: root.v("gapsIn", 0) * root.scaleBy
    Behavior on outer { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
    Behavior on inner { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }

    // the room the bar takes along the desk's top
    readonly property real barRoom: 9
    readonly property real half: (root.width - 2 * root.outer - root.inner) / 2
    readonly property real tall: root.height - root.barRoom - 2 * root.outer

    // the bar the windows keep clear of
    Rectangle {
        x: 6
        y: 3
        width: parent.width - 12
        height: 3
        radius: 1.5
        color: Lithic.alpha(Lithic.fg, 0.35)
    }

    MiniWindow {
        x: root.outer
        y: root.barRoom + root.outer
        width: root.half
        height: root.tall
        scaleBy: root.scaleBy
        radius: root.v("rounding", 0)
        border: root.v("borderSize", 0)
        active: true
    }
    MiniWindow {
        x: root.outer + root.half + root.inner
        y: root.barRoom + root.outer
        width: root.half
        height: (root.tall - root.inner) / 2
        scaleBy: root.scaleBy
        radius: root.v("rounding", 0)
        border: root.v("borderSize", 0)
        borderColor: Lithic.fg3
        active: false
    }
    MiniWindow {
        x: root.outer + root.half + root.inner
        y: root.barRoom + root.outer + (root.tall + root.inner) / 2
        width: root.half
        height: (root.tall - root.inner) / 2
        scaleBy: root.scaleBy
        radius: root.v("rounding", 0)
        border: root.v("borderSize", 0)
        borderColor: Lithic.fg3
        active: false
    }
}
