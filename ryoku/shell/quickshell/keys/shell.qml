pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Ryoku.Ui.Singletons

// Kit is a one-shot `qs -c keys` app bound to Super+K. It maps a full-screen
// layer-shell overlay on every monitor; only the focused output carries the card
// and keyboard. The legend comes from `ryoku-hub keybinds`, the same catalogue
// the Hub reads, so shortcuts stay in sync. Quitting releases the launch flock
// and leaves no idle process.
ShellRoot {
    id: app
    Component.onCompleted: Quickshell.watchFiles = !Quickshell.shellDir.startsWith(
        (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/quickshell/")

    property var categories: []
    property bool loaded: false
    property bool closing: false
    property string requestedTab: Quickshell.env("RYOKU_KIT_TAB")
    property string requestedRail: Quickshell.env("RYOKU_KIT_RAIL")
    property int requestRevision: 0

    function openKit(tab, rail) {
        app.requestedTab = tab;
        app.requestedRail = rail;
        app.requestRevision++;
    }

    IpcHandler {
        target: "kit"

        function open(tab: string, rail: string): void {
            app.openKit(tab, rail);
        }
    }

    Process {
        running: true
        command: ["ryoku-hub", "keybinds"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const o = JSON.parse(this.text);
                    // Kit answers "what can I press", so disabled rows with no
                    // chord stay in the Hub where they can be rebound.
                    const cats = (o && o.categories) ? o.categories : [];
                    app.categories = cats.map(function (c) {
                        return {
                            name: c.name,
                            binds: (c.binds || []).filter(function (b) { return !b.disabled; })
                        };
                    }).filter(function (c) { return c.binds.length > 0; });
                } catch (e) {
                    app.categories = [];
                }
                app.loaded = true;
            }
        }
    }

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

    Variants {
        model: Quickshell.screens

        PanelWindow {
            id: win
            required property var modelData
            screen: modelData
            color: "transparent"
            exclusiveZone: 0
            WlrLayershell.namespace: "ryoku-keys"
            WlrLayershell.layer: WlrLayer.Overlay
            anchors { top: true; bottom: true; left: true; right: true }

            // root.screen is briefly null while an output tears down; guard the
            // read so keyboard focus never latches on a stale value.
            readonly property bool onFocused: {
                var fo = Wm.focusedOutput;
                return fo ? (modelData ? fo === modelData.name : false) : true;
            }
            WlrLayershell.keyboardFocus: (win.onFocused && !app.closing)
                ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

            property real appear: 0
            NumberAnimation on appear {
                running: true
                from: 0
                to: 1
                duration: Tokens.reduceMotion ? 0 : Tokens.durNormal
                easing.type: Easing.OutCubic
            }

            FocusScope {
                anchors.fill: parent
                focus: true
                opacity: app.closing ? 0 : 1
                Behavior on opacity {
                    enabled: !Tokens.reduceMotion
                    NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                }

                // Dim the desktop so the frosted card reads on any wallpaper.
                Rectangle {
                    anchors.fill: parent
                    color: Tokens.paper
                    opacity: 0.42 * win.appear
                }
                // Click-out closes.
                MouseArea {
                    anchors.fill: parent
                    onClicked: app.dismiss()
                }

                Kit {
                    anchors.fill: parent
                    visible: win.onFocused
                    interactive: win.onFocused
                    categories: app.categories
                    loaded: app.loaded
                    appear: win.appear
                    requestedTab: app.requestedTab
                    requestedRail: app.requestedRail
                    requestRevision: app.requestRevision
                    onRequestClose: app.dismiss()
                }
            }
        }
    }
}
