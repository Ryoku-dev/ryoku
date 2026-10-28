pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons

// First-boot hint: a one-shot `qs -c keys-hint` toast the session launches
// exactly once, pointing a new user at Kit. The quiet top-centre card uses ink,
// key caps and hairlines, never accent. It does not take keyboard focus or time
// out; dismissing it quits the process and releases the launch flock.
ShellRoot {
    id: app
    Component.onCompleted: Quickshell.watchFiles = !Quickshell.shellDir.startsWith(
        (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/quickshell/")

    property bool closing: false
    function dismiss() {
        if (app.closing)
            return;
        app.closing = true;
        quitTimer.start();
    }
    Timer {
        id: quitTimer
        interval: Tokens.durSmall + 30
        onTriggered: Qt.quit()
    }

    // A hairline mono cap, the legend's key vocabulary.
    component Cap: Rectangle {
        property string text: ""
        implicitHeight: 22
        implicitWidth: Math.max(22, capLabel.implicitWidth + 14)
        radius: Tokens.radius
        color: "transparent"
        border.width: Tokens.border
        border.color: Tokens.line
        Text {
            id: capLabel
            anchors.centerIn: parent
            text: parent.text
            color: Tokens.inkDim
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
        }
    }

    // A shortcut line: Super + <key> + what it does, in the same cap vocabulary.
    component Hint: Row {
        id: hint
        property string keyCap: ""
        property string label: ""
        spacing: Tokens.s3
        Cap { anchors.verticalCenter: parent.verticalCenter; text: "Super" }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "+"; color: Tokens.inkFaint
            font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
        }
        Cap { anchors.verticalCenter: parent.verticalCenter; text: hint.keyCap }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: hint.label; color: Tokens.inkDim
            font.family: Tokens.ui; font.pixelSize: Tokens.fBody
        }
    }

    Variants {
        model: Quickshell.screens

        PanelWindow {
            id: win
            required property var modelData
            screen: modelData
            color: "transparent"
            exclusiveZone: 0
            WlrLayershell.namespace: "ryoku-keys-hint"
            WlrLayershell.layer: WlrLayer.Overlay
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
            anchors { top: true; left: true; right: true }
            implicitHeight: 210

            // Show only on the monitor the user is on when Ryoku first comes up.
            readonly property bool onFocused: {
                var fo = Wm.focusedOutput;
                return fo ? (modelData ? fo === modelData.name : false) : true;
            }
            visible: win.onFocused

            property real appear: 0
            NumberAnimation on appear {
                running: true
                from: 0
                to: 1
                duration: Tokens.durNormal
                easing.type: Easing.OutCubic
            }
            readonly property real shown: app.closing ? 0 : appear

            // Two stacked cards, appearing together: the Kit pointer and the
            // sidebar / settings shortcuts. One X dismisses the whole set.
            Column {
                id: stack
                anchors.horizontalCenter: parent.horizontalCenter
                y: Tokens.s5 + (win.shown - 1) * 14
                opacity: win.shown
                spacing: Tokens.s3
                Behavior on opacity { NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic } }
                Behavior on y { NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic } }

                // both cards take the wider content's width so they stack flush.
                readonly property real cardW: Math.max(kRow.implicitWidth, hintCol.implicitWidth) + Tokens.s5 * 2

                // ── card 1: Kit + dismiss ──
                Rectangle {
                    width: stack.cardW
                    height: 54
                    radius: Tokens.radius * 1.5
                    color: Tokens.paperLift
                    border.width: Tokens.border
                    border.color: Tokens.line

                    Row {
                        id: kRow
                        anchors.centerIn: parent
                        spacing: Tokens.s3

                        Rectangle {
                            anchors.verticalCenter: parent.verticalCenter
                            width: 12; height: 1; color: Tokens.ink
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: UiPreferences.brandMark("\u529b"); color: Tokens.ink
                            font.family: Tokens.jp; font.pixelSize: Tokens.fBody
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("Press"); color: Tokens.ink
                            font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                        }
                        Cap { anchors.verticalCenter: parent.verticalCenter; text: "Super" }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: "+"; color: Tokens.inkFaint
                            font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                        }
                        Cap { anchors.verticalCenter: parent.verticalCenter; text: "K" }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("to open Kit"); color: Tokens.inkDim
                            font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                        }

                        Item { width: Tokens.s2; height: 1 }

                        Rectangle {
                            anchors.verticalCenter: parent.verticalCenter
                            width: 28; height: 28
                            radius: Tokens.radius
                            color: closeHover.hovered ? Tokens.tint10 : "transparent"
                            border.width: Tokens.border
                            border.color: closeHover.hovered ? Tokens.lineStrong : Tokens.line
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                            Text {
                                anchors.centerIn: parent
                                text: "\u2715"; color: Tokens.inkDim
                                font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            }
                            HoverHandler { id: closeHover }
                            TapHandler { onTapped: app.dismiss() }
                        }
                    }
                }

                // ── card 2: the sidebar + settings shortcuts ──
                Rectangle {
                    width: stack.cardW
                    height: hintCol.implicitHeight + Tokens.s4 * 2
                    radius: Tokens.radius * 1.5
                    color: Tokens.paperLift
                    border.width: Tokens.border
                    border.color: Tokens.line

                    Column {
                        id: hintCol
                        anchors.centerIn: parent
                        spacing: Tokens.s2
                        Hint { keyCap: "Esc"; label: I18n.tr("for the sidebar") }
                        Hint { keyCap: ","; label: I18n.tr("for settings") }
                    }
                }
            }
        }
    }
}
