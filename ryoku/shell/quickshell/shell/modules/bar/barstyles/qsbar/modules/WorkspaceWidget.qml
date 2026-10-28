import Ryoku.Ui.Singletons
import QtQuick
import QtQuick.Shapes
import shell.services

Item {
    id: wsWidget
    required property var root

    // Workspace cells intentionally stay dense without a local surface. A widget
    // border needs its own breathing room, though: make that padding part of the
    // real implicit width so the border, drag geometry and split points all use
    // the same measurement instead of reconstructing an approximate visual edge.
    readonly property int borderHorizontalPadding: root.widgetHasBorder("G2") ? 6 : 0
    implicitWidth: wsRow.implicitWidth + 2 * borderHorizontalPadding
    implicitHeight: 28
    readonly property color contentColor: root.widgetContentColor("G2", root.seal)

    // The focused workspace name only when it is a real workspace beyond the
    // persist range, else "". Renotifies on value change only, so switching
    // between in-range workspaces keeps the model stable and the delegate
    // Behaviors animating.
    readonly property bool dynamicModel: Wm.workspaceModel === "dynamic"
    readonly property string extraWs: {
        if (root.workspaceMode === "active" || wsWidget.dynamicModel) return ""
        var n = root.workspaceMode === "5" ? 5 : 10
        var f = Wm.focusedWorkspace
        var num = f ? Number(f.name) : 0
        return (num > n) ? f.name : ""
    }

    // A dynamic model follows the live workspace list with no invented slots; a
    // fixed model shows a stable numbered row plus the focused-beyond-range one.
    readonly property var workspaceList: {
        if (wsWidget.dynamicModel) {
            var live = []
            var lw = Wm.workspaces
            for (var k = 0; k < lw.length; k++) if (!lw[k].special) live.push(lw[k].name)
            return live
        }
        if (root.workspaceMode === "active") {
            var ids = {}
            var ws = Wm.workspaces
            for (var i = 0; i < ws.length; i++) if (!ws[i].special) ids[ws[i].name] = true
            if (Wm.focusedWorkspace && !Wm.focusedWorkspace.special) ids[Wm.focusedWorkspace.name] = true
            return Object.keys(ids).sort(function(a, b) { return Number(a) - Number(b) })
        }
        var n = root.workspaceMode === "5" ? 5 : 10
        var list = []; for (var j = 1; j <= n; j++) list.push(String(j))
        if (extraWs !== "") list.push(extraWs)
        return list
    }

    // ── PACMAN style state: pacman rests on the focused cell, pellet glyphs
    //    mark occupied cells, dimmed dots the empty ones. On a focus change a
    //    runner pacman travels from the old cell to the new one, chomping as it
    //    goes; the destination pellet is eaten (fades/shrinks) as it arrives. ──
    readonly property string focusedWorkspaceKey: Wm.focusedWorkspace
        && !Wm.focusedWorkspace.special ? Wm.focusedWorkspace.name : ""

    property string pacmanLastFocusedWorkspaceKey: ""
    property string pacmanTargetWorkspaceKey: ""
    property bool pacmanTraveling: false
    property real pacmanMouthClosure: 0
    property int pacmanTravelDirection: 1
    property int pacmanTravelSteps: 1
    property real pacmanTravelFromX: 0
    property real pacmanTravelTargetX: 0
    property real pacmanTravelX: 0
    property real pacmanEatProgress: 0
    readonly property int pacmanTravelDuration: Math.min(720, 320 + pacmanTravelSteps * 100)
    readonly property int pacmanBiteCount: Math.max(3, Math.min(5, pacmanTravelSteps + 2))
    readonly property int pacmanBiteHalfDuration: Math.max(60,
        Math.round(pacmanTravelDuration / (pacmanBiteCount * 2)))
    readonly property real pacmanMaxMouthClosure: 0.82
    readonly property int pacmanEatDuration: 240
    readonly property int pacmanEatLeadIn: Math.max(0, pacmanTravelDuration - pacmanEatDuration)

    function pacmanCell(key) {
        for (var i = 0; i < wsRepeater.count; i++) {
            var it = wsRepeater.itemAt(i)
            if (it && it.wsKey === key) return it
        }
        return null
    }
    function pacmanCellIndex(key) {
        for (var i = 0; i < wsRepeater.count; i++) {
            var it = wsRepeater.itemAt(i)
            if (it && it.wsKey === key) return i
        }
        return -1
    }
    function pacmanCenterX(key) {
        var it = pacmanCell(key)
        return it ? wsRow.x + it.x + it.width / 2 : -1
    }
    function finishPacmanTravel() {
        pacmanTraveling = false
        pacmanMouthClosure = 0
        pacmanEatProgress = 0
        pacmanTargetWorkspaceKey = ""
    }
    function resetPacmanTravel() {
        pacmanTravel.stop()
        finishPacmanTravel()
        pacmanLastFocusedWorkspaceKey = focusedWorkspaceKey
    }
    function beginPacmanTravel(sourceKey, targetKey) {
        if (root.workspaceStyle !== "pacman" || focusedWorkspaceKey !== targetKey) {
            resetPacmanTravel()
            return
        }
        // Reduce-motion snaps to the focused cell; running the travel state
        // machine while throttled stranded the runner.
        if (Motion.reduce) {
            resetPacmanTravel()
            return
        }
        // Resolve the positioner before measuring so freshly rebound delegates at
        // x=0 are not mistaken for a zero-distance transition.
        if (typeof wsRow.forceLayout === "function") wsRow.forceLayout()
        var sourceX = pacmanTraveling ? pacmanTravelX : pacmanCenterX(sourceKey)
        var targetX = pacmanCenterX(targetKey)
        if (sourceX < 0 || targetX < 0 || sourceX === targetX) {
            finishPacmanTravel()
            return
        }
        pacmanTravel.stop()
        pacmanTravelFromX = sourceX
        pacmanTravelTargetX = targetX
        pacmanTravelX = sourceX
        pacmanTravelDirection = targetX >= sourceX ? 1 : -1
        var sourceIndex = pacmanCellIndex(sourceKey)
        var targetIndex = pacmanCellIndex(targetKey)
        pacmanTravelSteps = sourceIndex >= 0 && targetIndex >= 0
            ? Math.max(1, Math.abs(targetIndex - sourceIndex)) : 1
        pacmanTargetWorkspaceKey = targetKey
        pacmanEatProgress = 0
        pacmanMouthClosure = 0
        pacmanTraveling = true
        pacmanTravel.restart()
    }
    function observePacmanFocus() {
        var targetKey = focusedWorkspaceKey
        if (targetKey === "") return
        if (root.workspaceStyle !== "pacman" || pacmanLastFocusedWorkspaceKey === "") {
            resetPacmanTravel()
            pacmanLastFocusedWorkspaceKey = targetKey
            return
        }
        if (targetKey === pacmanLastFocusedWorkspaceKey) return
        var sourceKey = pacmanLastFocusedWorkspaceKey
        pacmanLastFocusedWorkspaceKey = targetKey
        Qt.callLater(function() { wsWidget.beginPacmanTravel(sourceKey, targetKey) })
    }

    onFocusedWorkspaceKeyChanged: observePacmanFocus()
    Component.onCompleted: pacmanLastFocusedWorkspaceKey = focusedWorkspaceKey

    Connections {
        target: root
        function onWorkspaceStyleChanged() { wsWidget.resetPacmanTravel() }
    }

    // Entering Power Saver mid-travel clears any in-flight runner, so a throttled
    // frame cannot strand it. This is what toggling the power profile did by hand.
    Connections {
        target: Motion
        function onReduceChanged() { if (Motion.reduce) wsWidget.resetPacmanTravel() }
    }

    // right-click anywhere opens the workspace panel
    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.RightButton
        cursorShape: Qt.PointingHandCursor
        onClicked: root.workspaceVisible = !root.workspaceVisible
    }

    Row {
        id: wsRow
        anchors.centerIn: parent
        z: 1
        spacing: root.workspaceStyle === "rings" ? 3
               : root.workspaceStyle === "aurora" ? 4
               : root.workspaceStyle === "pacman" ? 2
               : 5

        Repeater {
            id: wsRepeater
            model: wsWidget.workspaceList

            delegate: Item {
                id: wsCell
                required property var modelData
                readonly property string wsKey: String(modelData)

                // hover feedback works in every style (the old code scaled the
                // default-only `dot`, invisible in numbers/magic)
                Behavior on scale { NumberAnimation { duration: 120 } }

                readonly property bool isFocused: Wm.focusedWorkspace !== null
                                               && Wm.focusedWorkspace.name === wsKey

                readonly property bool isOccupied: {
                    var w = Wm.workspaceByName(wsKey)
                    return !!w && w.occupied && !isFocused
                }

                readonly property bool isEmpty: !isFocused && !isOccupied

                implicitWidth: root.workspaceStyle === "numbers" ? 22
                             : root.workspaceStyle === "kanji"   ? 22
                             : root.workspaceStyle === "magic"   ? (isFocused ? 20 : 18)
                             : root.workspaceStyle === "rings"   ? 20
                             : root.workspaceStyle === "aurora"  ? (isFocused ? 34 : 12)
                             : root.workspaceStyle === "pacman"  ? 22
                             : (isFocused ? 32 : 16)
                implicitHeight: 28

                Behavior on implicitWidth {
                    NumberAnimation { duration: 200; easing.type: Easing.OutCubic }
                }

                // ── DEFAULT style: glow + dot ──
                // glow - alle states, nur opacity variiert
                Rectangle {
                    visible: root.workspaceStyle === "default"
                    anchors.centerIn: parent
                    width:  isFocused ? 34 : 16
                    height: isFocused ? 16 : 16
                    radius: isFocused ?  8 :  8
                    color: isFocused
                        ? Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.20)
                        : isOccupied
                        ? Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.18)
                        : Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.06)

                    Behavior on width { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }
                    Behavior on color { ColorAnimation { duration: 200 } }
                }

                // pill / kreis
                Rectangle {
                    id: dot
                    visible: root.workspaceStyle === "default"
                    anchors.centerIn: parent
                    width:  isFocused  ? 26 : 8
                    height: 8
                    radius: 4
                    color:  isFocused
                        ? wsWidget.contentColor
                        : isOccupied
                        ? wsWidget.contentColor
                        : Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.25)

                    Behavior on width { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }
                    Behavior on color { ColorAnimation { duration: 200 } }
                }

                // ── NUMBERS style: a digit on the shared 6px V2 button shape. ──
                Rectangle {
                    visible: root.workspaceStyle === "numbers"
                    anchors.centerIn: parent
                    width:  20
                    height: 20
                    radius: root.panelButtonRadius
                    color: isFocused  ? Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.30)
                         : isOccupied ? Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.12)
                                      : Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.04)
                    Behavior on color { ColorAnimation { duration: 200 } }
                    Text {
                        anchors.centerIn: parent
                        text: wsKey
                        // focused = the only BRIGHT digit (lightened seal + bold + bigger);
                        // others dimmed so the active workspace is unmistakable
                        color: isFocused  ? wsWidget.contentColor
                             : isOccupied ? Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.5)
                                          : Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.28)
                        font.family: root.mono
                        font.pixelSize: isFocused ? 13 : 12
                        font.weight: isFocused ? Font.Bold : Font.Normal
                    }
                }

                // ── MAGIC style: the 3 ORIGINAL sparkle glyphs (filled / hollow / dot),
                //    all forced into ONE font (Adwaita Mono has all three) so they share
                //    a metric → no cross-font fallback misalignment ──
                Text {
                    visible: root.workspaceStyle === "magic"
                    anchors.centerIn: parent
                    anchors.verticalCenterOffset: isFocused ? 0 : 1   // active lifted vs occupied/empty
                    text: isFocused  ? String.fromCodePoint(0x2726)    // ✦ filled four-point star (active)
                         : isOccupied ? String.fromCodePoint(0x2727)    // ✧ hollow four-point star (occupied)
                                      : String.fromCodePoint(0x00B7)    // · middle dot (empty)
                    color: isFocused  ? wsWidget.contentColor
                         : isOccupied ? Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.7)
                                      : Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.3)
                    font.family: "Adwaita Mono"   // all 3 sparkle glyphs live here → one consistent metric
                    font.pixelSize: isFocused ? 22 : 18
                    renderType: Text.NativeRendering   // crisp hinted raster (default QtRendering softens small symbols)
                    Behavior on color { ColorAnimation { duration: 200 } }
                }

                // ── KANJI style: 一-十 numerals (waybar V7.2a format-icons), same
                //    focused/occupied/empty treatment as the other styles; ids >10
                //    fall back to the Arabic number ──
                Text {
                    visible: root.workspaceStyle === "kanji"
                    anchors.centerIn: parent
                    text: UiPreferences.showJapaneseGloss
                        && Number(wsKey) >= 1 && Number(wsKey) <= 10
                            ? ["一","二","三","四","五","六","七","八","九","十"][Number(wsKey) - 1]
                            : wsKey
                    color: isFocused  ? wsWidget.contentColor
                         : isOccupied ? Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.7)
                                      : Qt.rgba(wsWidget.contentColor.r, wsWidget.contentColor.g, wsWidget.contentColor.b, 0.3)
                    font.family: "Noto Sans CJK JP"
                    font.pixelSize: isFocused ? 15 : 13
                    font.weight: Font.Normal
                    renderType: Text.NativeRendering
                    Behavior on color { ColorAnimation { duration: 200 } }
                }

                // ── FRAME style: stable numeral cells with one shared moving
                //    outline behind the focused workspace. The persisted token
                //    stays "rings" so existing V2 caches migrate without a reset. ──
                Text {
                    id: frameLabel
                    visible: root.workspaceStyle === "rings"
                    anchors.centerIn: parent
                    text: wsKey
                    color: wsWidget.contentColor
                    opacity: wsMa.containsMouse ? 1.0
                        : isFocused ? 1.0
                        : isOccupied ? 0.64
                        : 0.24
                    font.family: root.mono
                    font.pixelSize: 12
                    font.weight: Font.Normal
                    font.hintingPreference: Font.PreferNoHinting
                    renderType: Text.QtRendering

                    Behavior on opacity {
                        NumberAnimation { duration: 160; easing.type: Easing.OutCubic }
                    }
                }

                // ── AURORA style: one flat light streak, not Default's thick
                //    pill-with-halo motif and not the source gradient. Inactive
                //    markers are strict squares before radius is applied, so every
                //    occupied/empty state renders as a true circle. ──
                Item {
                    id: auroraMark
                    visible: root.workspaceStyle === "aurora"
                    anchors.centerIn: parent
                    width: isFocused ? 32 : 10
                    height: 16

                    Behavior on width {
                        NumberAnimation { duration: 200; easing.type: Easing.OutCubic }
                    }

                    Rectangle {
                        anchors.centerIn: parent
                        width: isFocused ? 28 : (isOccupied ? 6 : 4)
                        height: isFocused ? 3 : (isOccupied ? 6 : 4)
                        radius: height / 2
                        color: wsWidget.contentColor
                        opacity: wsMa.containsMouse ? 1.0
                            : isFocused ? 0.92
                            : isOccupied ? 0.62
                            : 0.18
                        antialiasing: true

                        Behavior on width {
                            NumberAnimation { duration: 200; easing.type: Easing.OutCubic }
                        }
                        Behavior on height {
                            NumberAnimation { duration: 200; easing.type: Easing.OutCubic }
                        }
                        Behavior on opacity {
                            NumberAnimation { duration: 160; easing.type: Easing.OutCubic }
                        }
                    }
                }

                // ── PACMAN style: focused pacman / occupied pellet / empty dot.
                //    The travelling runner overlay (below) rides between cells;
                //    the focused cell hides its glyph while a runner is in flight
                //    so the runner appears to become the resting pacman. ──
                PacmanMarker {
                    visible: root.workspaceStyle === "pacman"
                    anchors.centerIn: parent
                    focused: wsCell.isFocused && !(wsWidget.pacmanTraveling
                        && wsCell.wsKey === wsWidget.pacmanTargetWorkspaceKey)
                    occupied: wsCell.isOccupied
                    hovered: wsMa.containsMouse
                    eatProgress: wsWidget.pacmanTraveling
                        && wsCell.wsKey === wsWidget.pacmanTargetWorkspaceKey
                        ? wsWidget.pacmanEatProgress : 0
                    eatDirection: wsWidget.pacmanTravelDirection
                    activeColor: wsWidget.contentColor
                    occupiedColor: wsWidget.contentColor
                    emptyColor: wsWidget.contentColor
                    hoverColor: wsWidget.contentColor
                }

                MouseArea {
                    id: wsMa
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    hoverEnabled: true
                    onClicked: root.gotoWorkspace(wsKey)
                    onEntered: wsCell.scale = root.workspaceStyle === "rings" ? 1.0
                        : root.workspaceStyle === "pacman" ? 1.0
                        : root.workspaceStyle === "aurora" ? 1.04 : 1.15
                    onExited:  wsCell.scale = 1.0
                }
            }
        }
    }

    // A single constant-size frame travels between fixed cells. Keeping its
    // geometry unchanged avoids texture resampling and numeral overlap while
    // preserving a smooth transition without rebuilding the delegates.
    Item {
        id: frameMotion
        anchors.fill: parent
        z: 0
        visible: root.workspaceStyle === "rings" && targetIndex >= 0

        readonly property int targetIndex: {
            var focused = Wm.focusedWorkspace
            if (!focused || focused.special) return -1
            return wsWidget.workspaceList.indexOf(focused.name)
        }
        readonly property real targetLeft: targetIndex >= 0
            ? wsRow.x + targetIndex * (20 + wsRow.spacing) + 1 : 0
        property real animatedX: targetLeft

        Behavior on animatedX {
            NumberAnimation {
                duration: 190
                easing.type: Easing.OutCubic
            }
        }

        Shape {
            id: frameShape
            x: frameMotion.animatedX
            y: 5
            width: 18
            height: 18
            antialiasing: true
            preferredRendererType: Shape.CurveRenderer
            layer.enabled: true
            layer.samples: Perf.msaa
            layer.smooth: true
            layer.mipmap: true
            layer.textureSize: Qt.size(Math.ceil(width * 4), height * 4)

            readonly property real r: 5

            ShapePath {
                strokeColor: wsWidget.contentColor
                strokeWidth: 1
                fillColor: "transparent"
                capStyle: ShapePath.FlatCap
                joinStyle: ShapePath.RoundJoin
                startX: frameShape.r
                startY: 0.5
                PathLine { x: frameShape.width - frameShape.r; y: 0.5 }
                PathQuad {
                    x: frameShape.width - 0.5; y: frameShape.r
                    controlX: frameShape.width - 0.5; controlY: 0.5
                }
                PathLine { x: frameShape.width - 0.5; y: frameShape.height - frameShape.r }
                PathQuad {
                    x: frameShape.width - frameShape.r; y: frameShape.height - 0.5
                    controlX: frameShape.width - 0.5; controlY: frameShape.height - 0.5
                }
                PathLine { x: frameShape.r; y: frameShape.height - 0.5 }
                PathQuad {
                    x: 0.5; y: frameShape.height - frameShape.r
                    controlX: 0.5; controlY: frameShape.height - 0.5
                }
                PathLine { x: 0.5; y: frameShape.r }
                PathQuad {
                    x: frameShape.r; y: 0.5
                    controlX: 0.5; controlY: 0.5
                }
            }
        }
    }

    // The travelling pacman that chomps between cells. It rides above the row in
    // the same coordinate space as the cells (wsRow.x + cell.x), so its resting
    // position lines up with the focused cell's marker once travel finishes.
    Item {
        id: pacmanRunner
        visible: wsWidget.pacmanTraveling && root.workspaceStyle === "pacman"
        z: 4
        x: wsWidget.pacmanTravelX - width / 2
        y: Math.round((parent.height - height) / 2)
        width: 22
        height: 18

        Item {
            id: pacmanRunnerVisual
            anchors.fill: parent
            transform: Scale {
                origin.x: pacmanRunnerVisual.width / 2
                origin.y: pacmanRunnerVisual.height / 2
                xScale: wsWidget.pacmanTravelDirection
            }

            Text {
                anchors.centerIn: parent
                text: String.fromCodePoint(0xF0BAF)
                color: wsWidget.contentColor
                font.family: "JetBrainsMono Nerd Font"
                font.pixelSize: 14
                font.weight: Font.Bold
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
                renderType: Text.NativeRendering
            }

            Canvas {
                id: pacmanMouthFill
                anchors.centerIn: parent
                width: 16
                height: width
                property real closure: wsWidget.pacmanMouthClosure
                property color fillColor: wsWidget.contentColor

                onClosureChanged: requestPaint()
                onFillColorChanged: requestPaint()
                onPaint: {
                    var context = getContext("2d")
                    var centerX = width / 2
                    var centerY = height / 2
                    var radius = Math.min(width, height) * 0.38
                    var angle = 0.70 * Math.max(0, Math.min(1, closure))
                    context.clearRect(0, 0, width, height)
                    if (angle <= 0.001) return
                    context.fillStyle = String(fillColor)
                    context.beginPath()
                    context.moveTo(centerX, centerY)
                    context.arc(centerX, centerY, radius, -angle, angle, false)
                    context.closePath()
                    context.fill()
                }
            }
        }
    }

    SequentialAnimation {
        id: pacmanTravel
        running: false

        ParallelAnimation {
            NumberAnimation {
                target: wsWidget
                property: "pacmanTravelX"
                from: wsWidget.pacmanTravelFromX
                to: wsWidget.pacmanTravelTargetX
                duration: wsWidget.pacmanTravelDuration
                easing.type: Easing.InOutSine
            }

            SequentialAnimation {
                loops: wsWidget.pacmanBiteCount
                NumberAnimation {
                    target: wsWidget
                    property: "pacmanMouthClosure"
                    from: 0
                    to: wsWidget.pacmanMaxMouthClosure
                    duration: wsWidget.pacmanBiteHalfDuration
                    easing.type: Easing.InOutSine
                }
                NumberAnimation {
                    target: wsWidget
                    property: "pacmanMouthClosure"
                    from: wsWidget.pacmanMaxMouthClosure
                    to: 0
                    duration: wsWidget.pacmanBiteHalfDuration
                    easing.type: Easing.InOutSine
                }
            }

            SequentialAnimation {
                PauseAnimation { duration: wsWidget.pacmanEatLeadIn }
                NumberAnimation {
                    target: wsWidget
                    property: "pacmanEatProgress"
                    from: 0
                    to: 1
                    duration: wsWidget.pacmanEatDuration
                    easing.type: Easing.InCubic
                }
            }
        }

        ScriptAction { script: wsWidget.finishPacmanTravel() }
    }

}
