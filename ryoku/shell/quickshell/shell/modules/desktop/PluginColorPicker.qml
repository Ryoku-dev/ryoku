pragma ComponentBehavior: Bound
import QtQuick
import "Singletons"

// A compact HSV colour picker for a plugin tile's right-click menu, mirroring
// the built-in MenuColorPicker (an SV square, a hue rail, preset swatches) but
// emitting the chosen colour rather than writing Config: the host persists it
// through ryoku-plugins-place. The wallpaper layer has no keyboard (the plugin
// menu keeps text fields in the hub), so this is mouse-only -- no hex field --
// and plugins have no per-frame setLive fast path, so it commits once, on
// release or a preset tap, instead of scrubbing live. The first swatch is the
// palette accent, so "match the system" is one tap.
Item {
    id: pick

    property string seed: ""            // current hex; "" -> the palette accent
    readonly property color accent: Scheme.accent

    signal committed(string hex)

    property real hh: 0
    property real ss: 1
    property real vv: 1
    readonly property color cur: Qt.hsva(pick.hh, pick.ss, pick.vv, 1)
    readonly property string curHex: pick.hexOf(pick.cur)

    readonly property var presets: [
        pick.hexOf(pick.accent), "#FFFFFF", "#C8CDD4", "#111318",
        pick.hexOf(Theme.brand), pick.hexOf(Theme.gold), "#7FBBB3"
    ]

    width: parent ? parent.width : 0
    implicitHeight: col.implicitHeight

    // "#RRGGBB", the syntax plugins.json stores.
    function hexOf(c) {
        return "#" + [c.r, c.g, c.b].map(function (x) {
            const s = Math.round(x * 255).toString(16);
            return s.length === 1 ? "0" + s : s;
        }).join("").toUpperCase();
    }
    function seedFrom(c) {
        pick.hh = c.hsvHue < 0 ? 0 : c.hsvHue;
        pick.ss = c.hsvSaturation;
        pick.vv = c.hsvValue;
    }
    function reseed() { pick.seedFrom((pick.seed && pick.seed.length > 0) ? Qt.color(pick.seed) : pick.accent); }
    function commit() { pick.committed(pick.curHex); }

    // re-seed when what is shown changes or the picker appears, never mid-drag
    // (the drag drives hh/ss/vv itself).
    onSeedChanged: pick.reseed()
    onVisibleChanged: if (visible) pick.reseed()
    Component.onCompleted: pick.reseed()

    Column {
        id: col
        width: parent.width
        spacing: Theme.s2

        // saturation / value square for the current hue.
        Item {
            id: sv
            width: parent.width
            height: 108
            Rectangle { anchors.fill: parent; radius: Theme.menuTileRadius; color: Qt.hsva(pick.hh, 1, 1, 1) }
            Rectangle {
                anchors.fill: parent
                radius: Theme.menuTileRadius
                gradient: Gradient {
                    orientation: Gradient.Horizontal
                    GradientStop { position: 0; color: "#ffffff" }
                    GradientStop { position: 1; color: "transparent" }
                }
            }
            Rectangle {
                anchors.fill: parent
                radius: Theme.menuTileRadius
                gradient: Gradient {
                    GradientStop { position: 0; color: "transparent" }
                    GradientStop { position: 1; color: "#000000" }
                }
            }
            Rectangle {
                width: 12
                height: 12
                radius: 6
                color: "transparent"
                border.width: 2
                border.color: pick.vv > 0.5 ? "#000000" : "#ffffff"
                x: pick.ss * sv.width - 6
                y: (1 - pick.vv) * sv.height - 6
            }
            MouseArea {
                anchors.fill: parent
                preventStealing: true
                function set(mx, my) {
                    pick.ss = Math.max(0, Math.min(1, mx / width));
                    pick.vv = Math.max(0, Math.min(1, 1 - my / height));
                }
                onPressed: (e) => set(e.x, e.y)
                onPositionChanged: (e) => { if (pressed) set(e.x, e.y); }
                onReleased: pick.commit()
            }
        }

        // hue rail.
        Item {
            id: hueRail
            width: parent.width
            height: 12
            Rectangle {
                anchors.fill: parent
                radius: 6
                gradient: Gradient {
                    orientation: Gradient.Horizontal
                    GradientStop { position: 0.000; color: "#ff0000" }
                    GradientStop { position: 0.167; color: "#ffff00" }
                    GradientStop { position: 0.333; color: "#00ff00" }
                    GradientStop { position: 0.500; color: "#00ffff" }
                    GradientStop { position: 0.667; color: "#0000ff" }
                    GradientStop { position: 0.833; color: "#ff00ff" }
                    GradientStop { position: 1.000; color: "#ff0000" }
                }
            }
            Rectangle {
                width: 4
                height: parent.height + 6
                y: -3
                x: Math.max(0, Math.min(hueRail.width - width, pick.hh * hueRail.width - 2))
                color: "#ffffff"
                border.width: 1
                border.color: "#000000"
            }
            MouseArea {
                anchors.fill: parent
                preventStealing: true
                function set(mx) { pick.hh = Math.max(0, Math.min(1, mx / width)); }
                onPressed: (e) => set(e.x)
                onPositionChanged: (e) => { if (pressed) set(e.x); }
                onReleased: pick.commit()
            }
        }

        // preset swatches, the palette accent first.
        Flow {
            width: parent.width
            spacing: Theme.s2
            Repeater {
                model: pick.presets
                Rectangle {
                    id: sw
                    required property string modelData
                    width: 20
                    height: 20
                    radius: Theme.menuTileRadius
                    color: sw.modelData
                    border.width: 1
                    border.color: Theme.line
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: { pick.seedFrom(Qt.color(sw.modelData)); pick.commit(); }
                    }
                }
            }
        }
    }
}
