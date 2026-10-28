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
