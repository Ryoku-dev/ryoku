pragma ComponentBehavior: Bound
import QtQuick
import "Singletons"

// A compact HSV colour picker for the widget context menu, in the desktop-menu
// idiom and mirroring the visualiser's picker (an SV square, a hue rail, a hex
// field and preset swatches). It edits the widget's Config colour key(s): the
// base colour, or -- in gradient mode -- whichever of the two stops is
// selected. Scrubs live via setLive so the widget recolours in place, and
// persists on release with set, the same split the resize uses so a drag never
// thrashes widgets.json. The first swatch is the live wallpaper accent, so
// "match the wallpaper" is one tap.
Item {
    id: pick

    property string scope: ""          // widget id -> <scope>Color / <scope>Color2
    property bool gradient: false       // reveal and edit the second stop
    readonly property color accent: Scheme.accent

    // which stop the square/rail/hex edits: base "a" or the gradient's "b".
    property string editing: "a"
    readonly property string keyA: pick.scope + "Color"
    readonly property string keyB: pick.scope + "Color2"
    readonly property string activeKey: pick.editing === "b" ? pick.keyB : pick.keyA

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

    // "#RRGGBB", the syntax widgets.json stores.
    function hexOf(c) {
        return "#" + [c.r, c.g, c.b].map(function (x) {
            const s = Math.round(x * 255).toString(16);
            return s.length === 1 ? "0" + s : s;
        }).join("").toUpperCase();
    }
    function seed(c) {
        pick.hh = c.hsvHue < 0 ? 0 : c.hsvHue;
        pick.ss = c.hsvSaturation;
        pick.vv = c.hsvValue;
    }
    function seedFromKey() {
        const v = Config.get(pick.activeKey);
        pick.seed((v && v.length > 0) ? Qt.color(v) : pick.accent);
    }
    function selectStop(which) { pick.editing = which; pick.seedFromKey(); }
    function live() { Config.setLive(pick.activeKey, pick.curHex); }
    function commit() { Config.set(pick.activeKey, pick.curHex); }

    // re-seed on any change of what is being edited or when the picker appears,
    // never mid-drag (the drag drives hh/ss/vv itself).
    onScopeChanged: pick.seedFromKey()
    onGradientChanged: if (!pick.gradient) pick.selectStop("a")
    onVisibleChanged: if (visible) pick.seedFromKey()
    Component.onCompleted: pick.seedFromKey()

    Column {
        id: col
        width: parent.width
        spacing: Theme.s2

        // A/B stops (B only in gradient): tap to pick the stop the picker edits.
        Row {
            visible: pick.gradient
            spacing: Theme.s2
            Repeater {
                model: [{ "id": "a", "key": pick.keyA }, { "id": "b", "key": pick.keyB }]
                Rectangle {
                    id: stop
                    required property var modelData
                    width: Theme.s5
                    height: Theme.s5
                    radius: Theme.menuTileRadius
                    readonly property string val: Config.get(stop.modelData.key) || ""
                    color: stop.val.length > 0 ? stop.val : pick.accent
                    border.width: pick.editing === stop.modelData.id ? 2 : 1
                    border.color: pick.editing === stop.modelData.id ? Theme.ink : Theme.line
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: pick.selectStop(stop.modelData.id)
                    }
                }
            }
        }

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
                    pick.live();
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
                function set(mx) { pick.hh = Math.max(0, Math.min(1, mx / width)); pick.live(); }
                onPressed: (e) => set(e.x)
                onPositionChanged: (e) => { if (pressed) set(e.x); }
                onReleased: pick.commit()
            }
        }

        // hex readout / entry for the selected stop.
        Rectangle {
            width: parent.width
            height: Theme.ctlH
            radius: Theme.menuTileRadius
            color: "transparent"
            border.width: hexField.activeFocus ? 2 : 1
            border.color: hexField.activeFocus ? Theme.ink : Theme.line
            TextInput {
                id: hexField
                anchors.fill: parent
                anchors.leftMargin: Theme.s2
                anchors.rightMargin: Theme.s2
                verticalAlignment: TextInput.AlignVCenter
                clip: true
                color: Theme.ink
                font.family: Theme.mono
                font.pixelSize: Theme.fSmall
                selectByMouse: true
                maximumLength: 7
                text: pick.curHex
                onActiveFocusChanged: if (activeFocus) selectAll()
                function commitText() {
                    let t = text.trim();
                    if (t.length > 0 && t[0] !== "#") t = "#" + t;
                    if (/^#[0-9a-fA-F]{6}$/.test(t)) {
                        pick.seed(Qt.color(t));
                        pick.commit();
                    }
                    text = Qt.binding(function () { return pick.curHex; });
                }
                Keys.onReturnPressed: commitText()
                Keys.onEnterPressed: commitText()
                onEditingFinished: commitText()
            }
        }

        // preset swatches, the wallpaper accent first.
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
                        onClicked: { pick.seed(Qt.color(sw.modelData)); pick.commit(); }
                    }
                }
            }
        }
    }
}
