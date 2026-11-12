import QtQuick
import QtQuick.Controls
import Ryoku.Ui.Singletons

// A colour control that shows the colour, not just a hex string. A live swatch
// previews the value; clicking it opens a visual picker (no need to know a code);
// the mono field still takes an exact hex for power users. `value` in, `chosen`
// out -- the host owns persistence. The picker is Ryoku's own paper-and-ink
// surface, not the platform ColorDialog (which reads as a grey web widget).
Rectangle {
    id: root

    property string value: ""
    signal chosen(string v)

    // a hex is valid enough to paint the swatch and seed the picker.
    readonly property bool validHex: /^#?[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$/.test(root.value.trim())
    readonly property string norm: root.validHex ? (root.value.trim()[0] === "#" ? root.value.trim() : "#" + root.value.trim()) : "#000000"

    function hexOf(c) {
        return "#" + [c.r, c.g, c.b].map(function (x) {
            var s = Math.round(x * 255).toString(16);
            return s.length === 1 ? "0" + s : s;
        }).join("").toUpperCase();
    }

    implicitHeight: 30
    color: "transparent"
    radius: Tokens.radius
    border.width: hexIn.activeFocus ? 2 : Tokens.border
    border.color: hexIn.activeFocus ? Tokens.ink : Tokens.line

    Row {
        anchors.fill: parent
        anchors.leftMargin: 5
        anchors.rightMargin: 8
        spacing: Tokens.s2

        // the swatch: a live preview, and the entry to the visual picker.
        Rectangle {
            id: swatch
            anchors.verticalCenter: parent.verticalCenter
            width: 20
            height: 20
            radius: Tokens.radius
            color: root.validHex ? root.norm : "transparent"
            border.width: Tokens.border
            border.color: Tokens.lineStrong
            Text {
                anchors.centerIn: parent
                visible: !root.validHex
                text: "?"
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: 11
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
            TapHandler { onTapped: dlg.open() }
        }

        TextInput {
            id: hexIn
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - swatch.width - Tokens.s2
            verticalAlignment: Text.AlignVCenter
            clip: true
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: 12
            selectByMouse: true
            text: root.value.toUpperCase()
            onEditingFinished: root.chosen(text.trim())
        }
    }

    // ── the visual picker: pick by eye, paper-and-ink. HSV state seeds from the
    // current value on open; OK writes back a #RRGGBB hex so the stored value
    // keeps the config's own syntax. ───────────────────────────────────────
    Popup {
        id: dlg
        parent: Overlay.overlay
        anchors.centerIn: parent
        modal: true
        width: 296
        padding: Tokens.s4
        closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside

        property real hh: 0
        property real ss: 1
        property real vv: 1
        readonly property color cur: Qt.hsva(dlg.hh, dlg.ss, dlg.vv, 1)
        readonly property string curHex: root.hexOf(dlg.cur)

        onAboutToShow: {
            var c = root.norm;
            dlg.hh = c.hsvHue < 0 ? 0 : c.hsvHue;
            dlg.ss = c.hsvSaturation;
            dlg.vv = c.hsvValue;
            hexEdit.text = dlg.curHex;
        }

        background: Rectangle {
            color: Tokens.paper
            border.width: Tokens.border
            border.color: Tokens.lineStrong
        }

        Overlay.modal: Rectangle { color: Qt.rgba(0, 0, 0, 0.5) }

        contentItem: Column {
            spacing: Tokens.s3

            // saturation / value field: hue base under white->clear (x = sat)
            // and clear->black (y = value) washes; drag to set sat + value.
            Item {
                id: sv
                width: parent.width
                height: 176
                Rectangle {
                    anchors.fill: parent
                    color: Qt.hsva(dlg.hh, 1, 1, 1)
                }
                Rectangle {
                    anchors.fill: parent
                    gradient: Gradient {
                        orientation: Gradient.Horizontal
                        GradientStop { position: 0; color: "#ffffff" }
                        GradientStop { position: 1; color: "transparent" }
                    }
                }
                Rectangle {
                    anchors.fill: parent
                    gradient: Gradient {
                        GradientStop { position: 0; color: "transparent" }
                        GradientStop { position: 1; color: "#000000" }
                    }
                }
                Rectangle {
                    border.width: 2
                    border.color: dlg.vv > 0.5 ? "#000000" : "#ffffff"
                    color: "transparent"
                    width: 12
                    height: 12
                    radius: 6
                    x: dlg.ss * sv.width - 6
                    y: (1 - dlg.vv) * sv.height - 6
                }
                MouseArea {
                    anchors.fill: parent
                    function set(mx, my) {
                        dlg.ss = Math.max(0, Math.min(1, mx / width));
                        dlg.vv = Math.max(0, Math.min(1, 1 - my / height));
                        hexEdit.text = dlg.curHex;
                    }
                    onPressed: (e) => set(e.x, e.y)
                    onPositionChanged: (e) => { if (pressed) set(e.x, e.y); }
                }
            }

            // hue rail: the full wheel unrolled, drag to set hue.
            Item {
                id: hue
                width: parent.width
                height: 16
                Rectangle {
                    anchors.fill: parent
                    radius: Tokens.radius
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
                    x: Math.max(0, Math.min(hue.width - width, dlg.hh * hue.width - 2))
                    color: "#ffffff"
                    border.width: 1
                    border.color: "#000000"
                }
                MouseArea {
                    anchors.fill: parent
                    function set(mx) {
                        dlg.hh = Math.max(0, Math.min(1, mx / width));
                        hexEdit.text = dlg.curHex;
                    }
                    onPressed: (e) => set(e.x)
                    onPositionChanged: (e) => { if (pressed) set(e.x); }
                }
            }

            // hex + swatch: type a code or read the picked one.
            Row {
                width: parent.width
                spacing: Tokens.s2
                Rectangle {
                    width: 28
                    height: 28
                    anchors.verticalCenter: parent.verticalCenter
                    radius: Tokens.radius
                    color: dlg.cur
                    border.width: Tokens.border
                    border.color: Tokens.lineStrong
                }
                Rectangle {
                    width: parent.width - 28 - Tokens.s2
                    height: 28
                    anchors.verticalCenter: parent.verticalCenter
                    color: "transparent"
                    radius: Tokens.radius
                    border.width: hexEdit.activeFocus ? 2 : Tokens.border
                    border.color: hexEdit.activeFocus ? Tokens.ink : Tokens.line
                    TextInput {
                        id: hexEdit
                        anchors.fill: parent
                        anchors.leftMargin: Tokens.s2
                        verticalAlignment: Text.AlignVCenter
                        clip: true
                        color: Tokens.ink
                        font.family: Tokens.mono
                        font.pixelSize: 13
                        selectByMouse: true
                        onEditingFinished: {
                            var m = /^#?([0-9a-fA-F]{6})$/.exec(text.trim());
                            if (m) {
                                var c = Qt.color("#" + m[1]);
                                dlg.hh = c.hsvHue < 0 ? 0 : c.hsvHue;
                                dlg.ss = c.hsvSaturation;
                                dlg.vv = c.hsvValue;
                                text = dlg.curHex;
                            }
                        }
                    }
                }
            }

            // OK / Cancel
            Row {
                anchors.right: parent.right
                spacing: Tokens.s2
                component PickBtn: Rectangle {
                    id: pb
                    property string label: ""
                    property bool primary: false
                    signal act()
                    width: Math.max(64, txt.implicitWidth + Tokens.s4)
                    height: 28
                    radius: Tokens.radius
                    color: pb.primary ? Tokens.bone : (ph.hovered ? Tokens.tint10 : "transparent")
                    border.width: Tokens.border
                    border.color: pb.primary ? Tokens.bone : Tokens.line
                    Text {
                        id: txt
                        anchors.centerIn: parent
                        text: I18n.tr(pb.label)
                        color: pb.primary ? Tokens.inkOnBone : Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        font.weight: Font.DemiBold
                    }
                    HoverHandler { id: ph; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: pb.act() }
                }
                PickBtn {
                    label: I18n.tr("Cancel")
                    onAct: dlg.close()
                }
                PickBtn {
                    label: I18n.tr("OK")
                    primary: true
                    onAct: {
                        root.chosen(dlg.curHex);
                        dlg.close();
                    }
                }
            }
        }
    }
}
