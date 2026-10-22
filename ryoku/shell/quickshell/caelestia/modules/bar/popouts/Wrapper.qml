pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Caelestia.Config
import caelestia.components
import caelestia.components.misc
import caelestia.services
import caelestia.modules.windowinfo

Item {
    id: root

    required property ShellScreen screen
    required property real offsetScale
    required property DrawerVisibilities visibilities
    Config.screen: root.screen.name
    readonly property alias content: content
    readonly property alias winfo: winfo
    readonly property real nonAnimWidth: content.shouldBeActive ? content.implicitWidth : winfo.shouldBeActive ? winfo.implicitWidth : content.implicitWidth
    readonly property real nonAnimHeight: content.shouldBeActive ? content.implicitHeight : winfo.shouldBeActive ? winfo.implicitHeight : content.implicitHeight
    readonly property Item current: (content.item as Content)?.current ?? null
    readonly property bool isDetached: detachedMode.length > 0
    readonly property bool sidebarOpen: popoutState.sidebarOpen
    // Popouts excluded from pushing the notification column / sidebar out of the
    // way (Panels.qml) and from visually merging with the sidebar (ContentWindow.qml):
    // hover previews and context menus are transient, so shoving panels around for
    // them feels jittery. The clock popout is deliberately NOT here — the calendar
    // is a real panel-sized popout and displaces notifications like audio/network do.
    readonly property bool isDockPopout: currentName === "dockhover" || currentName === "dockcontext" || currentName === "dockbgcontext" || currentName === "greeter" || currentName === "greetercontext" || currentName === "activewindow" || currentName === "github" || currentName === "clockcontext" || currentName === "statusiconscontext" || currentName === "workspacescontext" || currentName === "traycontext" || currentName === "githubcontext"
    property alias currentName: popoutState.currentName
    property alias hasCurrent: popoutState.hasCurrent
    property alias dockModel: popoutState.dockModel
    property alias tasksModel: popoutState.tasksModel
    property real currentCenter
    // Set by ClipWrapper: whether the current bar item sits under the sidebar,
    // which can be narrower than the bar's right side now that it is resizable.
    property bool underSidebar: true
    property string detachedMode
    readonly property QtObject dummy: QtObject {}
    property int animLength: dummy.Tokens.anim.durations.expressiveDefaultSpatial
    property var animCurve: dummy.Tokens.anim.expressiveDefaultSpatial

    function setAnims(detach: bool): void {
        const type = `expressive${detach ? "Slow" : "Default"}Spatial`;
        animLength = dummy.Tokens.anim.durations[type];
        animCurve = dummy.Tokens.anim[type];
    }
    function detach(mode: string): void {
        setAnims(true);
        if (mode === "winfo") {
            detachedMode = mode;
            focus = true;
        } else {
            Hub.open("shellstyle");
            close();
        }
        setAnims(false);
    }
    function close(): void {
        hasCurrent = false;
        detachedMode = "";
    }

    implicitWidth: nonAnimWidth
    implicitHeight: nonAnimHeight
    focus: hasCurrent
    Keys.onEscapePressed: {
        if (currentName === "wirelesspassword" && content.item) {
            const passwordPopout = (content.item as Content)?.children.find(c => c.name === "wirelesspassword");
            if (passwordPopout && passwordPopout.item) {
                passwordPopout.item.closeDialog();
                return;
            }
        }
        close();
    }
    Keys.onPressed: event => {
        if (currentName === "wirelesspassword") {
            event.accepted = false;
        }
    }

    PopoutState {
        id: popoutState

        // Only attach to the sidebar when the popout would open below it anyway;
        // otherwise it would end up away from the item that opened it.
        sidebarOpen: root.visibilities.sidebar && root.underSidebar
        isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"
        onDetachRequested: mode => root.detach(mode)
    }
    FocusGrab {
        active: root.isDetached
        windows: [QsWindow.window]
        onCleared: root.close()
    }
    Binding {
        when: root.isDetached || (root.hasCurrent && root.currentName === "wirelesspassword")
        target: QsWindow.window
        property: "WlrLayershell.keyboardFocus"
        value: WlrKeyboardFocus.OnDemand
    }
    Comp {
        id: content

        shouldBeActive: root.hasCurrent && !root.detachedMode
        anchors.fill: parent
        sourceComponent: Content {
            popouts: popoutState
        }
    }
    Comp {
        id: winfo

        shouldBeActive: root.detachedMode === "winfo"
        anchors.centerIn: parent
        sourceComponent: WindowInfo {
            clientAddress: popoutState.selectedClientAddress
        }
        onShouldBeActiveChanged: {
            if (!shouldBeActive) {
                popoutState.selectedClientAddress = "";
            }
        }
    }
    Behavior on implicitWidth {
        Anim {
            duration: root.animLength
            easing: root.animCurve
        }
    }
    Behavior on implicitHeight {
        enabled: root.offsetScale < 1

        Anim {
            duration: root.animLength
            easing: root.animCurve
        }
    }

    component Comp: Loader {
        id: comp

        property bool shouldBeActive

        active: false
        opacity: 0
        states: State {
            name: "active"
            when: comp.shouldBeActive

            PropertyChanges {
                comp.opacity: 1
                comp.active: true
            }
        }
        transitions: [
            Transition {
                from: ""
                to: "active"

                SequentialAnimation {
                    PropertyAction {
                        property: "active"
                    }
                    Anim {
                        type: Anim.DefaultEffects
                        property: "opacity"
                    }
                }
            },
            Transition {
                from: "active"
                to: ""

                SequentialAnimation {
                    Anim {
                        type: Anim.DefaultEffects
                        property: "opacity"
                    }
                    PropertyAction {
                        property: "active"
                    }
                }
            }
        ]
    }
}
