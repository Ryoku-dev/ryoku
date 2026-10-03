import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import shell.services

Item {
    id: rootMod
    required property var root

    implicitWidth: logo.implicitWidth + logoPadding
    implicitHeight: 28

    readonly property string tooltipText: Config.launcherTarget === "quick" ? I18n.tr("Quick settings") : I18n.tr("Control center")
    readonly property real logoPadding: 12
    readonly property color contentColor: root.widgetContentColor("G1", root.seal)

    // animated wave phase
    property real phase: 0
    NumberAnimation on phase {
        from: 0; to: 2 * Math.PI
        duration: 2600; loops: Animation.Infinite
        // gate: only animate while hovered or control panel open - otherwise the
        // Canvas repainted 24/7 via onPhaseChanged even when nobody looks
        running: ma.containsMouse || root.controlVisible
    }

    // ── hover-wave container (invisible surface, clips the wave to the logo area) ──
    Rectangle {
        id: pill
        anchors.centerIn: parent
        width: logo.implicitWidth + rootMod.logoPadding
        height: root.pillH
        color: "transparent"
        border.width: 0
        clip: true

        Canvas {
            id: wave
            anchors.fill: parent
            // only present while active (hovered or control panel open); fully
            // gone when idle. Fades so it appears/disappears smoothly.
            opacity: (ma.containsMouse || root.controlVisible) ? 0.55 : 0
            visible: opacity > 0.001
            Behavior on opacity { NumberAnimation { duration: 220; easing.type: Easing.OutCubic } }

            onPaint: {
                var ctx = getContext("2d")
                ctx.clearRect(0, 0, width, height)
                var cy = height / 2
                var amp = 3.0
                var k = (2 * Math.PI) / width * 2   // two cycles across the width

                function drawWave(phaseOff, alpha) {
                    ctx.beginPath()
                    for (var x = 0; x <= width; x += 2) {
                        var y = cy + Math.sin(x * k + rootMod.phase + phaseOff) * amp
                        if (x === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y)
                    }
                    // fade the ends so the wave doesn't hard-clip at the pill edge
                    var r = Math.round(rootMod.contentColor.r * 255)
                    var g = Math.round(rootMod.contentColor.g * 255)
                    var b = Math.round(rootMod.contentColor.b * 255)
                    var lit = "rgba(" + r + "," + g + "," + b + "," + alpha + ")"
                    var grad = ctx.createLinearGradient(0, 0, width, 0)
                    grad.addColorStop(0,    "rgba(" + r + "," + g + "," + b + ",0)")
                    grad.addColorStop(0.16, lit)
                    grad.addColorStop(0.84, lit)
                    grad.addColorStop(1,    "rgba(" + r + "," + g + "," + b + ",0)")
                    ctx.strokeStyle = grad
                    ctx.lineWidth = 1.5
                    ctx.lineCap = "round"
                    ctx.stroke()
                }

                drawWave(0,       0.45)
                drawWave(Math.PI, 0.22)
            }

            Connections {
                target: rootMod
                function onPhaseChanged() { wave.requestPaint() }
            }
            Connections {
                target: root
                function onSealChanged() { wave.requestPaint() }
                function onWidgetColorStylesChanged() { wave.requestPaint() }
            }
            Component.onCompleted: requestPaint()
        }
    }

    // Ryoku brand mark by default; the chosen wordmark or icon option otherwise,
    // themed with the seal. Resolvers live in Theme so the picker and bar agree.
    Text {
        id: logo
        anchors.centerIn: parent
        anchors.horizontalCenterOffset: root.launcherLogoMode === "icon"
            ? root.launcherLogoIconXOffset(root.launcherLogoIcon) : 0
        text: root.launcherLogoMode === "icon"
            ? root.launcherLogoIconGlyph(root.launcherLogoIcon)
            : root.launcherLogoTextLabel(root.launcherLogoText)
        color: root.seal
        renderType: Text.NativeRendering
        font.family: root.launcherLogoMode === "icon"
            ? root.launcherLogoIconFont(root.launcherLogoIcon) : root.mono
        font.pixelSize: root.launcherLogoMode === "icon"
            ? root.launcherLogoIconSize(root.launcherLogoIcon) : 12
        font.weight: Font.Bold
        font.letterSpacing: root.launcherLogoMode === "icon" ? 0 : 2
        Behavior on color { ColorAnimation { duration: 200 } }
    }

    TooltipMixin { id: tip; root: rootMod.root; owner: rootMod; text: rootMod.tooltipText }

    MouseArea {
        id: ma
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onEntered: tip.show()
        onExited:  { tip.hide() }
        onClicked: {
            tip.hide()
            if (Config.launcherTarget === "quick")
                ShellState.requestSurfaceActive("sidebar-left", undefined)
            else
                root.controlVisible = !root.controlVisible
        }
    }
}
