import QtQuick
import Quickshell
import Quickshell.Wayland
import Lithic as L
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services

// The administrator password card for every style that does not draw its own.
// The daemon is the polkit agent; this only shows its prompt and sends back the
// typed answer or the cancel.
PanelWindow {
    id: root

    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property real cardWidth: 420
    property real reveal: 0
    readonly property string fieldLabel: {
        const p = Polkit.prompt.trim().replace(/:\s*$/, "");
        return p !== "" ? p : I18n.tr("Password");
    }

    screen: ShellState.screens.find(scr => scr.name === Wm.focusedOutput)
        || ShellState.screens[0] || Quickshell.screens[0]
    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-polkit"
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive

    function trySubmit() {
        if (!Polkit.busy)
            Polkit.submit(field.text);
    }

    function reset() {
        field.text = "";
        field.forceActiveFocus();
    }

    Component.onCompleted: {
        root.reveal = 1;
        root.reset();
    }
    Behavior on reveal {
        NumberAnimation {
            duration: Motion.dur(220)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: L.Lithic.curve
        }
    }

    Connections {
        target: Polkit
        function onRefreshed() { root.reset(); }
    }

    Binding {
        target: L.Lithic
        property: "reduceMotion"
        value: Motion.reduce
    }

    Rectangle {
        anchors.fill: parent
        color: L.Lithic.scrim
        opacity: root.reveal
        MouseArea { anchors.fill: parent }
    }

    Item {
        id: stage
        anchors.fill: parent
        focus: true
        Keys.onEscapePressed: Polkit.cancel()

        Item {
            id: logical
            anchors.centerIn: parent
            width: root.cardWidth
            height: card.height
            scale: root.s * (0.96 + 0.04 * root.reveal)
            opacity: root.reveal

            L.CutRect {
                id: plate
                anchors.fill: parent
                fill: L.Lithic.stone4
                cut: L.LithicTokens.cut.large
                strokeWidth: L.LithicTokens.size.hairline
                strokeColor: L.Lithic.vein
            }

            Column {
                id: card
                x: 28
                width: parent.width - 56
                topPadding: 28
                bottomPadding: 28
                spacing: 16

                L.Label {
                    width: parent.width
                    text: I18n.tr("Administrator access")
                    typeRole: "sentence"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }

                L.Label {
                    width: parent.width
                    text: Polkit.message !== "" ? Polkit.message
                        : I18n.tr("An application is asking for administrator access.")
                    typeRole: "detail"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }

                Item {
                    width: parent.width
                    height: visible ? 78 : 0
                    visible: Polkit.fingerprint !== ""
                    FingerprintScan {
                        anchors.centerIn: parent
                        sizePx: 68
                        accent: L.Lithic.gold
                        ink: L.Lithic.fg
                        phase: Polkit.fingerprint === "fail" ? "fail" : "scanning"
                    }
                }

                L.Label {
                    width: parent.width
                    visible: Polkit.info !== ""
                    text: Polkit.info
                    typeRole: "detail"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }

                L.TextField {
                    id: field
                    width: parent.width
                    label: root.fieldLabel
                    secret: !Polkit.echo
                    errorText: Polkit.error
                    enabled: !Polkit.busy
                    onAccepted: root.trySubmit()
                }

                Row {
                    spacing: 12
                    anchors.right: parent.right
                    layoutDirection: Qt.RightToLeft

                    L.Plaque {
                        kind: "gold"
                        text: I18n.tr("Authenticate")
                        busy: Polkit.busy
                        onClicked: root.trySubmit()
                    }
                    L.Plaque {
                        kind: "quiet"
                        text: I18n.tr("Cancel")
                        onClicked: Polkit.cancel()
                    }
                }
            }
        }
    }
}
