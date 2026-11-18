import QtQuick
import "lib/hittest.js" as Hit

// The eight grip targets on the captured region; the design's selection draws
// the grips themselves from hotRole and downRole. Centres are computed from the
// global selection and placed only by the monitor that actually contains them,
// so a selection spanning two heads has one grip per edge rather than a
// duplicate pair at the seam.
Item {
    id: grips

    required property var globalRect
    required property int sx
    required property int sy
    property real u: 1

    property string hotRole: ""
    property string downRole: ""

    signal started(string role, real gx, real gy)
    signal moved(real gx, real gy)
    signal ended()

    readonly property real hit: 22 * grips.u

    readonly property var cursors: ({
        "tl": Qt.SizeFDiagCursor, "br": Qt.SizeFDiagCursor,
        "tr": Qt.SizeBDiagCursor, "bl": Qt.SizeBDiagCursor,
        "t": Qt.SizeVerCursor, "b": Qt.SizeVerCursor,
        "l": Qt.SizeHorCursor, "r": Qt.SizeHorCursor
    })

    Repeater {
        model: Hit.handleRoles()

        Item {
            id: grip
            required property var modelData
            readonly property string role: modelData
            readonly property bool hot: ma.containsMouse && grip.visible
            onHotChanged: {
                if (grip.hot) grips.hotRole = grip.role;
                else if (grips.hotRole === grip.role) grips.hotRole = "";
            }
            readonly property var centre: grips.globalRect
                ? Hit.handleCenter(grips.globalRect, grip.role)
                : null
            readonly property real lx: centre ? centre.x - grips.sx : 0
            readonly property real ly: centre ? centre.y - grips.sy : 0

            visible: centre !== null
                && lx >= 0 && lx <= grips.width
                && ly >= 0 && ly <= grips.height

            x: lx - grips.hit / 2
            y: ly - grips.hit / 2
            width: grips.hit
            height: grips.hit

            MouseArea {
                id: ma
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: grips.cursors[grip.role]
                onPressed: {
                    grips.downRole = grip.role;
                    grips.started(grip.role, grip.lx + grips.sx, grip.ly + grips.sy);
                }
                onPositionChanged: (m) => {
                    if (pressed) grips.moved(m.x + grip.x + grips.sx, m.y + grip.y + grips.sy);
                }
                onReleased: {
                    grips.downRole = "";
                    grips.ended();
                }
                onCanceled: grips.downRole = ""
            }
        }
    }
}
