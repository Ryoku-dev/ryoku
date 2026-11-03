// iNiR SDDM pixel theme — Virtual Keyboard
// Matches the shell's OnScreenKeyboard / KeyboardKey.qml visual style.
import QtQuick 2.15
import QtQuick.Layouts 1.15

Rectangle {
    id: kbd
    // Width set by parent (Main.qml anchors.left/right with margins)
    height: 290
    radius: 20
    color: Qt.rgba(kbd.bgColor.r, kbd.bgColor.g, kbd.bgColor.b, 0.96)

    signal keyClicked(string key)
    signal backspaceClicked()
    signal enterClicked()
    signal closeRequested()

    property bool showSymbols: false
    property bool isShifted: false

    // Theme colors — passed from Main.qml
    property color keyBgColor:      "#1e2022"
    property color funcBgColor:     "#141617"
    property color accentColor:     "#cba6f7"
    property color accentTextColor: "#1e1e2e"
    property color textColor:       "#cdd6f4"
    property color bgColor:         "#131315"
    // Legacy aliases kept for compatibility
    property alias btnColor: kbd.keyBgColor
    property alias activeColor: kbd.accentColor

    // ── Key depth effect (matches KeyboardKey.qml: outer border + inner face) ─
    component KeyButton: Rectangle {
        id: keyBtn
        property string label: ""
        property string shiftLabel: label.toUpperCase()
        property string symLabel: ""
        property real keyWidth: 1

        Layout.fillHeight: true
        Layout.fillWidth: true
        Layout.preferredWidth: keyWidth

        // Outer = depth border (slightly lighter = bottom shadow illusion)
        color: Qt.lighter(kbd.keyBgColor, 1.15)
        radius: 7

        // Inner face — shifts up 2px when not pressed (creates depth)
        Rectangle {
            anchors { fill: parent; bottomMargin: keyArea.pressed ? 0 : 2 }
            radius: 7
            color: keyArea.pressed ? Qt.lighter(kbd.keyBgColor, 1.2) : kbd.keyBgColor

            Text {
                anchors.centerIn: parent
                text: kbd.showSymbols && keyBtn.symLabel ? keyBtn.symLabel
                    : kbd.isShifted ? keyBtn.shiftLabel : keyBtn.label
                color: kbd.textColor
                font.pixelSize: 15
                font.weight: Font.Medium
            }
        }

        MouseArea {
            id: keyArea; anchors.fill: parent
            onClicked: {
                kbd.keyClicked(kbd.showSymbols && keyBtn.symLabel ? keyBtn.symLabel
                               : kbd.isShifted ? keyBtn.shiftLabel : keyBtn.label)
                if (kbd.isShifted) kbd.isShifted = false
            }
        }
    }

    component FuncButton: Rectangle {
        id: funcBtn
        property string icon: ""
        property string textLabel: ""
        property var action: null
        property bool active: false
        property bool isEnter: false
        property real keyWidth: isEnter ? 2.2 : 1.6

        Layout.fillHeight: true
        Layout.fillWidth: true
        Layout.preferredWidth: keyWidth

        color: (funcBtn.active || funcBtn.isEnter)
            ? (funcArea.pressed ? Qt.darker(kbd.accentColor, 1.15) : kbd.accentColor)
            : (funcArea.pressed ? Qt.lighter(kbd.funcBgColor, 1.2) : kbd.funcBgColor)
        radius: 10
        Behavior on color { ColorAnimation { duration: 100 } }

        Text {
            anchors.centerIn: parent
            text: funcBtn.icon !== "" ? funcBtn.icon : funcBtn.textLabel
            color: (funcBtn.active || funcBtn.isEnter) ? kbd.accentTextColor : kbd.textColor
            font.pixelSize: funcBtn.isEnter ? 15 : 16
            font.weight: Font.Medium
        }
        MouseArea {
            id: funcArea; anchors.fill: parent
            onClicked: if (funcBtn.action) funcBtn.action()
        }
    }

    // ── Layout ────────────────────────────────────────────────────────────────
    ColumnLayout {
        anchors { fill: parent; margins: 10 }
        spacing: 5

        // Header bar: label + close button
        RowLayout {
            Layout.fillWidth: true
            Layout.preferredHeight: 28
            spacing: 0

            Text {
                text: "Virtual Keyboard"; font.pixelSize: 11
                color: Qt.rgba(kbd.textColor.r, kbd.textColor.g, kbd.textColor.b, 0.4)
                Layout.fillWidth: true; Layout.leftMargin: 4
            }
            Rectangle {
                width: 28; height: 28; radius: 7
                color: closeHover.containsMouse
                    ? Qt.rgba(kbd.accentColor.r, kbd.accentColor.g, kbd.accentColor.b, 0.2)
                    : "transparent"
                Behavior on color { ColorAnimation { duration: 100 } }
                Text {
                    anchors.centerIn: parent; text: "✕"; font.pixelSize: 13
                    color: closeHover.containsMouse ? kbd.accentColor
                        : Qt.rgba(kbd.textColor.r, kbd.textColor.g, kbd.textColor.b, 0.5)
                    Behavior on color { ColorAnimation { duration: 100 } }
                }
                MouseArea { id: closeHover; anchors.fill: parent; hoverEnabled: true
                    onClicked: kbd.closeRequested() }
            }
        }

        // Row 1: qwertyuiop
        RowLayout { spacing: 4; Layout.fillWidth: true; Layout.fillHeight: true
            KeyButton { label: "q"; symLabel: "1" } KeyButton { label: "w"; symLabel: "2" }
            KeyButton { label: "e"; symLabel: "3" } KeyButton { label: "r"; symLabel: "4" }
            KeyButton { label: "t"; symLabel: "5" } KeyButton { label: "y"; symLabel: "6" }
            KeyButton { label: "u"; symLabel: "7" } KeyButton { label: "i"; symLabel: "8" }
            KeyButton { label: "o"; symLabel: "9" } KeyButton { label: "p"; symLabel: "0" }
        }

        // Row 2: asdfghjkl (inset)
        RowLayout { spacing: 4; Layout.fillWidth: true; Layout.fillHeight: true
            Item { Layout.preferredWidth: 0.5; Layout.fillWidth: true }
            KeyButton { label: "a"; symLabel: "@" } KeyButton { label: "s"; symLabel: "#" }
            KeyButton { label: "d"; symLabel: "$" } KeyButton { label: "f"; symLabel: "%" }
            KeyButton { label: "g"; symLabel: "&" } KeyButton { label: "h"; symLabel: "-" }
            KeyButton { label: "j"; symLabel: "+" } KeyButton { label: "k"; symLabel: "(" }
            KeyButton { label: "l"; symLabel: ")" }
            Item { Layout.preferredWidth: 0.5; Layout.fillWidth: true }
        }

        // Row 3: shift + zxcvbnm + backspace
        RowLayout { spacing: 4; Layout.fillWidth: true; Layout.fillHeight: true
            FuncButton { icon: "⇧"; active: kbd.isShifted; action: function() { kbd.isShifted = !kbd.isShifted } }
            KeyButton { label: "z"; symLabel: "*" }  KeyButton { label: "x"; symLabel: "\"" }
            KeyButton { label: "c"; symLabel: "'" }  KeyButton { label: "v"; symLabel: ":" }
            KeyButton { label: "b"; symLabel: ";" }  KeyButton { label: "n"; symLabel: "!" }
            KeyButton { label: "m"; symLabel: "?" }
            FuncButton { icon: "⌫"; action: function() { kbd.backspaceClicked() } }
        }

        // Row 4: ?123 / space / enter
        RowLayout { spacing: 4; Layout.fillWidth: true; Layout.fillHeight: true
            FuncButton { textLabel: "?123"; active: kbd.showSymbols; action: function() { kbd.showSymbols = !kbd.showSymbols } }
            KeyButton { label: ","; symLabel: ","; keyWidth: 1 }
            KeyButton { label: " "; symLabel: " "; keyWidth: 4.5 }
            KeyButton { label: "."; symLabel: "."; keyWidth: 1 }
            FuncButton { icon: "⏎"; isEnter: true; action: function() { kbd.enterClicked() } }
        }
    }
}
