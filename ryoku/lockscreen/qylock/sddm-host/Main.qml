import QtQuick
import QtQuick.Window

Item {
    id: host
    width: Screen.width
    height: Screen.height

    readonly property bool clock24h: {
        const value = typeof config !== "undefined" ? config.clock24h : undefined
        if (value === undefined || value === null || String(value).length === 0)
            return true
        return value === true || String(value).toLowerCase() === "true"
    }

    Loader {
        id: theme
        anchors.fill: parent
        focus: true
        source: "theme/Main.qml"
        onLoaded: {
            if (item.clock24h !== undefined)
                item.clock24h = host.clock24h
            if (item.clockLocale !== undefined)
                item.clockLocale = Qt.locale()
            Qt.callLater(() => forceActiveFocus(Qt.OtherFocusReason))
        }
    }

    // The Wayland greeter cannot report or change the layout from QML, so this
    // names what weston loaded and the key it switches with.
    Item {
        id: layoutHint
        property var codes: []
        property string chord: "Alt+Shift"
        readonly property real unit: Math.max(1, Screen.height / 768)
        readonly property var chords: ({
            "alt_shift_toggle": "Alt+Shift", "ctrl_shift_toggle": "Ctrl+Shift",
            "win_space_toggle": "Super+Space", "ctrl_space_toggle": "Ctrl+Space",
            "alt_space_toggle": "Alt+Space", "caps_toggle": "Caps Lock",
            "shift_caps_toggle": "Shift+Caps Lock", "alt_caps_toggle": "Alt+Caps Lock",
            "ctrl_alt_toggle": "Ctrl+Alt", "menu_toggle": "Menu"
        })

        function field(text, name) {
            const m = text.match(new RegExp('Option\\s+"' + name + '"\\s+"([^"]*)"', "i"))
            return m ? m[1] : ""
        }

        function load(text) {
            const list = field(text, "XkbLayout").split(",").map(s => s.trim().toUpperCase()).filter(s => s.length > 0)
            const grp = field(text, "XkbOptions").split(",").filter(s => s.indexOf("grp:") === 0)
            if (grp.length > 0)
                chord = chords[grp[0].slice(4)] || ""
            codes = list
        }

        visible: codes.length > 1
        z: 1000000
        anchors.left: parent.left
        anchors.bottom: parent.bottom
        anchors.margins: 24 * unit
        width: hintText.implicitWidth + 24 * unit
        height: hintText.implicitHeight + 14 * unit

        Component.onCompleted: {
            const req = new XMLHttpRequest()
            req.open("GET", "file:///etc/X11/xorg.conf.d/00-keyboard.conf")
            req.onreadystatechange = () => {
                if (req.readyState === XMLHttpRequest.DONE && req.responseText.length > 0)
                    layoutHint.load(req.responseText)
            }
            req.send()
        }

        Rectangle {
            anchors.fill: parent
            radius: 6 * layoutHint.unit
            color: "#99000000"
        }
        Text {
            id: hintText
            anchors.centerIn: parent
            color: "#ffffff"
            opacity: 0.85
            font.pixelSize: 14 * layoutHint.unit
            text: layoutHint.codes.join("  \u00b7  ") + (layoutHint.chord.length > 0 ? "   |   " + layoutHint.chord : "")
        }
    }

    // Weston does not advertise the cursor-shape protocol used by Qt on the
    // desktop. Keep the login pointer in the scene so every selected skin has
    // one even when the legacy cursor surface cannot be scanned out.
    HoverHandler {
        id: pointer
        cursorShape: Qt.BlankCursor
    }

    Canvas {
        width: 26 * scaleFactor
        height: 26 * scaleFactor
        z: 2000000
        visible: pointer.hovered
        x: pointer.point.position.x
        y: pointer.point.position.y
        antialiasing: true

        readonly property real scaleFactor: Math.max(1, Screen.height / 768)

        onPaint: {
            var ctx = getContext("2d")
            ctx.reset()
            ctx.scale(scaleFactor, scaleFactor)
            ctx.beginPath()
            ctx.moveTo(0, 0)
            ctx.lineTo(0, 16)
            ctx.lineTo(4, 12.5)
            ctx.lineTo(6.5, 19)
            ctx.lineTo(9, 18)
            ctx.lineTo(6.5, 11.5)
            ctx.lineTo(11, 11.5)
            ctx.closePath()
            ctx.fillStyle = "#ffffff"
            ctx.fill()
            ctx.lineWidth = 1.2
            ctx.strokeStyle = "#000000"
            ctx.stroke()
        }
    }
}
