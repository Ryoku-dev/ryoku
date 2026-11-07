import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane

    readonly property string layoutName: String(Wm.keyboardLayout || "Keyboard")
    readonly property string code: shortCode(layoutName)

    property bool present: (Wm.keyboardLayouts || []).length > 1

    implicitWidth: present ? label.implicitWidth + 2 * K.Style.hitPad : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    function shortCode(name) {
        const value = String(name || "").trim();
        const match = value.match(/[A-Za-z]{2,}/);
        const raw = match
            ? match[0].slice(0, match[0].length <= 3 ? 3 : 2)
            : value.slice(0, 3);
        if (raw.length === 0)
            return "Kb";
        const lower = raw.toLowerCase();
        return lower.charAt(0).toUpperCase() + lower.slice(1);
    }

    K.Label {
        id: label
        anchors.centerIn: parent
        scale: press.pressScale
        text: root.code
        role: "tag"
        tone: "dim"
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: Wm.cycleKeyboardLayout()
        onRightClicked: Wm.cycleKeyboardLayout()
    }

    K.Hint {
        target: root
        text: root.layoutName
        shown: root.present && press.hovered
    }
}