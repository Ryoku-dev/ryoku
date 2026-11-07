pragma ComponentBehavior: Bound
import QtQuick
import "Singletons"

// A compact HSV colour picker for a set of named ink roles, in the desktop-menu
// idiom and sharing MenuColorPicker's anatomy (an SV square, a hue rail, a hex
// field and preset swatches). Unlike MenuColorPicker, which is bound to the
// desktop Config's <scope>Color keys, this one is store-agnostic: a caller wires
// `readColor`/`writeColor` to drive any set of palette roles, keeping the
// write on its own settings path.
// A row of role swatches picks which role the square edits, mirroring the
// gradient A/B stops. Edits write on drag so the widget recolours in place.
Item {
    id: pick

    // [{ key, label, fallback }] -- the roles this picker cycles between.
    property var roles: []
    // read(key, fallbackHex) -> "#RRGGBB"; write(key, hex) -> void.
    property var readColor: null
    property var writeColor: null
    property bool showHexEntry: true

    property int index: 0
    readonly property var role: (pick.roles && pick.roles.length > index) ? pick.roles[index] : null

    property real hh: 0
    property real ss: 1
    property real vv: 1
    readonly property color cur: Qt.hsva(pick.hh, pick.ss, pick.vv, 1)
    readonly property string curHex: pick.hexOf(pick.cur)

    readonly property var presets: [
        pick.hexOf(Theme.accent), "#FFFFFF", "#C8CDD4", "#111318",
        pick.hexOf(Theme.brand), pick.hexOf(Theme.gold), "#7FBBB3"
    ]

    width: parent ? parent.width : 0
    implicitHeight: col.implicitHeight

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
    function curValue() {
        if (!pick.role || !pick.readColor)
            return pick.presets[0];
        return pick.readColor(pick.role.key, pick.role.fallback);
    }
    function seedFromRole() {
        const v = pick.curValue();
        pick.seed((v && String(v).length > 0) ? Qt.color(v) : Qt.color(pick.presets[0]));
    }
    function put(hex) {
        if (pick.role && pick.writeColor)
            pick.writeColor(pick.role.key, hex);
    }
    function selectRole(i) { pick.index = i; pick.seedFromRole(); }

    onRolesChanged: pick.seedFromRole()
    onVisibleChanged: if (visible) pick.seedFromRole()
    Component.onCompleted: pick.seedFromRole()

    Column {
        id: col
        width: parent.width
        spacing: Theme.s2

        // role swatches: tap to pick which ink the square edits.
        Flow {
            visible: pick.roles.length > 1
            width: parent.width
            spacing: Theme.s2
            Repeater {
                model: pick.roles
                Rectangle {
                    id: chip
                    required property var modelData
                    required property int index
                    width: Theme.s5
                    height: Theme.s5
                    radius: Theme.menuTileRadius
                    readonly property string val: pick.readColor ? String(pick.readColor(chip.modelData.key, chip.modelData.fallback)) : ""
                    color: chip.val.length > 0 ? chip.val : Theme.tile
                    border.width: pick.index === chip.index ? 2 : 1
                    border.color: pick.index === chip.index ? Theme.ink : Theme.line
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: pick.selectRole(chip.index)
                    }
                }
            }
        }

        Text {
            visible: pick.role !== null
            text: pick.role ? pick.role.label : ""
            color: Theme.inkDim
            font.family: Theme.font
            font.pixelSize: Theme.fMicro
            font.weight: Font.DemiBold
            font.letterSpacing: Theme.trackMark
        }

        // saturation / value square for the current hue.
        Item {
            id: sv
            width: parent.width
            height: 96
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
                    pick.put(pick.curHex);
                }
                onPressed: (e) => set(e.x, e.y)
                onPositionChanged: (e) => { if (pressed) set(e.x, e.y); }
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
                function set(mx) { pick.hh = Math.max(0, Math.min(1, mx / width)); pick.put(pick.curHex); }
                onPressed: (e) => set(e.x)
                onPositionChanged: (e) => { if (pressed) set(e.x); }
            }
        }

        // hex readout / entry for the current role.
        Rectangle {
            visible: pick.showHexEntry
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
                        pick.put(pick.curHex);
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
                        onClicked: { pick.seed(Qt.color(sw.modelData)); pick.put(pick.curHex); }
                    }
                }
            }
        }
    }
}
