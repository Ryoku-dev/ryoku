pragma ComponentBehavior: Bound

import QtQuick
import "popouts"
import shell.services

// One Ryoku-owned surface riding the shared Popout. Credential prompts, voice,
// capture, and small rail cards mount on demand.
Popout {
    id: root

    property var record: null
    property string anchor: "top"
    property bool menuOpen: false
    // Live "voice opened in its inactive state" flag from the manager record
    // (the static config record does not carry it). Gates the dictation capture
    // and the surface's off note.
    property bool off: false
    // The trigger centre the manager derives for the surface (its owning bar
    // widget or the screen centre for an IPC open).
    property real triggerAlong: -1

    signal requestClose()

    readonly property real fallbackMinWidth: 200
    readonly property real minWidth: record && record.minWidth ? record.minWidth : fallbackMinWidth
    readonly property string kind: record && record.kind ? record.kind : ""
    // stays true through the close melt so the body tears down only once flush.
    readonly property bool effectiveOpen: menuOpen || prog > 0.004

    edge: anchor.indexOf("top") === 0 ? "top"
        : anchor.indexOf("bottom") === 0 ? "bottom"
        : anchor
    align: (anchor.indexOf("-left") >= 0) ? "start"
         : (anchor.indexOf("-right") >= 0) ? "end"
         : "center"
    fullSpan: record && record.fullSpan === true
    hoverOpen: false
    pinned: root.menuOpen
    // A keybind open centres the surface on the screen midpoint (triggerAlong).
    // A folder style remaps side anchors to top corners, where align (start/end)
    // must win so the surface hugs that corner instead of the centre.
    alongCenter: root.align === "center" ? root.triggerAlong : -1

    // Surfaces size to their content at the monitor UI scale (unlike the fixed
    // reference-pixel menus). A full-span sidebar fills the frame top-to-bottom.
    openW: Math.max(root.minWidth * root.s, body.item ? body.item.implicitWidth : 0)
    // A full-span sidebar fills the frame top-to-bottom; other surfaces size to
    // content.
    openH: root.fullSpan ? root.height
        : (body.item ? body.item.implicitHeight : 0)

    // Card popouts float off the frame lip.
    edgeGap: (root.kind === "music" || root.kind === "bluetooth" || root.kind === "battery" || root.kind === "network" || root.kind === "voice" || root.kind === "sysmon" || root.kind === "audio" || root.kind === "screenshot") ? 10 * root.s : 0

    readonly property bool sideMenu: root.edge === "left" || root.edge === "right"

    // Side surfaces slide; top and bottom surfaces grow diagonally.
    transitions: [
        Transition {
            to: "open"
            NumberAnimation {
                property: "prog"
                duration: root.sideMenu ? Motion.menuSlide : Motion.diagonal
                easing.type: root.sideMenu ? Motion.menuSlideCurve : Motion.diagonalCurve
            }
        },
        Transition {
            from: "open"
            NumberAnimation {
                property: "prog"
                duration: root.sideMenu ? Motion.menuSlide : Motion.diagonal
                easing.type: root.sideMenu ? Motion.menuSlideCurve : Motion.diagonalCurve
            }
        }
    ]

    Loader {
        id: body
        // Bodies mount only while open or melting closed, so inactive capture
        // and monitoring processes do no work.
        active: root.effectiveOpen
        width: root.openW
        height: root.openH
        sourceComponent: root.kind === "voice" ? voiceBody
            : root.kind === "keyring" ? keyringBody
            : root.kind === "polkit" ? polkitBody
            : root.kind === "music" ? musicBody
            : root.kind === "bluetooth" ? bluetoothBody
            : root.kind === "battery" ? batteryBody
            : root.kind === "network" ? networkBody
            : root.kind === "sysmon" ? sysmonBody
            : root.kind === "audio" ? audioBody
            : root.kind === "screenshot" ? captureBody
            : null
    }


    Component {
        id: voiceBody
        VoicePopout {
            s: root.s
            off: root.off
            capture: root.menuOpen && !root.off
            open: root.effectiveOpen
            onCloseRequested: root.requestClose()
        }
    }
    Component {
        id: polkitBody
        PolkitPopout {
            s: root.s
            open: root.effectiveOpen
            onCloseRequested: root.requestClose()
        }
    }
    Component {
        id: keyringBody
        KeyringPopout {
            s: root.s
            open: root.effectiveOpen
            onCloseRequested: root.requestClose()
        }
    }
    Component {
        id: musicBody
        MusicPopout {
            s: root.s
            open: root.effectiveOpen
        }
    }
    Component {
        id: bluetoothBody
        BluetoothPopout {
            s: root.s
            open: root.effectiveOpen
        }
    }
    Component {
        id: batteryBody
        BatteryPopout {
            s: root.s
            open: root.effectiveOpen
        }
    }
    Component {
        id: networkBody
        NetworkPopout {
            s: root.s
            open: root.effectiveOpen
        }
    }
    Component {
        id: sysmonBody
        SysMonitorPopout {
            s: root.s
            open: root.effectiveOpen
        }
    }
    Component {
        id: audioBody
        AudioPopout {
            s: root.s
            open: root.effectiveOpen
        }
    }
    Component {
        id: captureBody
        CapturePopout {
            s: root.s
            open: root.effectiveOpen
            onRequestClose: root.requestClose()
        }
    }
}
