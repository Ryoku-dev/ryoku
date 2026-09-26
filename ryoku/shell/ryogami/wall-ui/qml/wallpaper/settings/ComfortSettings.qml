import QtQuick
import Quickshell
import Quickshell.Io
import "../.."
import "../../components"
import Ryoku.Ui.Singletons

// Comfort: screen backlight + warm-light (night light), folded from the Hub's
// Appearance > Comfort tab. Brightness rides brightnessctl; the warm screen
// rides the ryoku-shell daemon's reactive `nightlight` topic over its control
// socket, the same stream the bar and the Hub use: `subscribe nightlight`
// pushes {on, temperature} on every change, `call nightlight.set` sends the
// intent back. No process polling, so the toggle can never show a stale
// reading over a click that just landed.
Column {
    id: root
    property var colors
    width: parent ? parent.width : 0
    spacing: 8

    property int _brightness: 100
    property bool _warm: false
    property int _temp: 4000
    readonly property string _sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    function applyNightFrame(line) {
        try {
            const f = JSON.parse(line)
            root._warm = f.on === true
            if (typeof f.temperature === "number" && f.temperature > 0)
                root._temp = f.temperature
        } catch (e) {
            // A malformed frame keeps the last good state.
        }
    }

    function setNight(on, temp) {
        nightCtl.queued += "call nightlight.set " +
            JSON.stringify({ on: on, temperature: temp }) + "\n"
        if (nightCtl.connected)
            nightCtl.flushQueued()
        else
            nightCtl.connected = true
    }

    Component.onCompleted: brProc.running = true

    property string _brBuf: ""
    Process {
        id: brProc
        command: ["brightnessctl", "-m"]
        stdout: SplitParser { splitMarker: ""; onRead: function(d) { root._brBuf += d } }
        onExited: {
            var line = (root._brBuf.trim().split("\n")[0]) || ""
            var parts = line.split(",")          // device,class,current,percent,max
            if (parts.length >= 4) {
                var p = parseInt(parts[3])
                if (!isNaN(p)) root._brightness = p
            }
            root._brBuf = ""
        }
    }

    // A subscription connection is read-mostly: writes half-close the stream,
    // so intents ride a second socket, and queued calls flush on (re)connect.
    Socket {
        id: nightSub
        path: root._sockPath
        parser: SplitParser { onRead: line => root.applyNightFrame(line) }
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe nightlight\n")
                flush()
            } else {
                nightRetry.restart()
            }
        }
        // The panel is created lazily; connect on completion so the first
        // open already has the daemon's last frame by the next tick.
        Component.onCompleted: connected = true
    }
    Timer {
        id: nightRetry
        interval: 2000
        onTriggered: if (!nightSub.connected) nightSub.connected = true
    }
    Socket {
        id: nightCtl
        path: root._sockPath
        property string queued: ""
        function flushQueued() {
            if (queued.length === 0)
                return
            write(queued)
            flush()
            queued = ""
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }

    function _setBrightness(v) { Quickshell.execDetached(["brightnessctl", "set", v + "%"]) }

    SettingsCard {
        colors: root.colors
        title: I18n.tr("Screen"); kana: "画面"
        width: parent.width

        RowInput {
            colors: root.colors
            title: I18n.tr("Brightness")
            description: I18n.tr("Display backlight level.")
            value: root._brightness
            min: 5; max: 100; suffix: "%"
            onCommit: function(v) { root._brightness = v; root._setBrightness(v) }
        }
    }

    SettingsCard {
        colors: root.colors
        title: I18n.tr("Warm light"); kana: "暖色"
        width: parent.width

        RowToggle {
            colors: root.colors
            title: I18n.tr("Warm screen")
            description: I18n.tr("Cut blue light with a night-light tint (hyprsunset).")
            checked: root._warm
            onToggle: function(v) { root._warm = v; root.setNight(v, root._temp) }
        }

        RowInput {
            colors: root.colors
            title: I18n.tr("Temperature")
            description: I18n.tr("Colour temperature in Kelvin. Lower is warmer.")
            value: root._temp
            min: 2500; max: 6500; suffix: "K"
            enabled: root._warm
            onCommit: function(v) { root._temp = v; if (root._warm) root.setNight(true, v) }
        }
    }
}
