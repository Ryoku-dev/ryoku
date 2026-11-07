import QtQuick
import shell.services
import shell.barkit as Pill

// Kairos quick-settings slider: a leading glyph and an accent-filled track.
Item {
    id: fader

    property string glyph: ""
    property string icon: ""
    property real value: 0
    property bool dragging: false
    property real live: 0
    signal moved(real v)
    signal committed(real v)

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    readonly property color accent: Theme.primary
    function dim(a) { return Qt.rgba(fader.ink.r, fader.ink.g, fader.ink.b, a) }

    readonly property real frac: Math.max(0, Math.min(1, fader.value))
    readonly property real shown: fader.dragging ? fader.live : fader.frac
    function fracAt(mx) { return Math.max(0, Math.min(1, mx / Math.max(1, track.width))) }

    implicitHeight: 34

    Pill.GlyphIcon {
        anchors { left: parent.left; verticalCenter: parent.verticalCenter }
        width: 20
        height: 20
        visible: fader.glyph.length > 0
        name: fader.glyph
        color: fader.dim(0.85)
    }
    Pill.MaterialIcon {
        anchors { left: parent.left; verticalCenter: parent.verticalCenter }
        width: 20
        height: 20
        visible: fader.icon.length > 0
        text: fader.icon
        color: fader.dim(0.85)
        font.pixelSize: 19
    }

    Item {
        id: track
        anchors {
            left: parent.left; leftMargin: 34
            right: parent.right; verticalCenter: parent.verticalCenter
        }
        height: 26

        Rectangle {
            anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter }
            height: 8
            radius: 4
            color: fader.dim(0.16)
        }
        Rectangle {
            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
            width: track.width * fader.shown
            height: 8
            radius: 4
            color: fader.accent
        }
        MouseArea {
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            preventStealing: true
            onPressed: (m) => {
                fader.dragging = true;
                fader.live = fader.fracAt(m.x);
                fader.moved(fader.live);
            }
            onPositionChanged: (m) => {
                if (!fader.dragging) return;
                fader.live = fader.fracAt(m.x);
                fader.moved(fader.live);
            }
            onReleased: (m) => {
                var v = fader.fracAt(m.x);
                fader.dragging = false;
                fader.committed(v);
            }
            onCanceled: fader.dragging = false
        }
    }
}
