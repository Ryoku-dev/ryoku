import QtQuick
import ".."
import "../services"

Item {
    id: sandyView

    // Shared picker contract
    property var model
    property var colors
    property bool active: false
    property int selectedIndex: 0

    signal selectionChanged(int index)
    signal itemActivated(var item)

    // Sandy composition knobs
    property real center: 0.5
    property real sliceWidth: 200
    property real sliceHeight: 260
    property real skew: 0
    property real spacing: 0.55
    property real duration: 6000
    property int strands: 5
    property real twist: 8
    property real orbit: 0.6
    property real turbulence: 0.4
    property real waist: 0.35
    property real front: 0.6
    property real arc: 0.6
    property real edgeSpeed: 0.5
    property real ringSpin: 0
    property real ringSize: 0.6
    property real ringWave: 0.15
    property real ringSoft: 0.5
    property real ringBlend: 0.0
    property real ringHold: 0.4
    property real grain: 0.3
    property real fan: 6

    // Runtime animation state
    property real _clock: 0
    property real _slide: 0

    readonly property int _count: {
        var mc = model ? model.count : 0
        if (mc <= 0)
            return 0
        var want = Math.max(1, strands * 5)
        return Math.min(48, Math.min(mc, want))
    }
    readonly property int _half: Math.floor(_count / 2)

    clip: false
    focus: active
    onActiveChanged: if (active) forceActiveFocus()

    function _hash(i) {
        var x = Math.sin(i * 127.1 + 311.7) * 43758.5453
        return x - Math.floor(x)
    }
    function _mix(a, b, t) {
        return a + (b - a) * t
    }
    function _clamp(v, lo, hi) {
        return Math.max(lo, Math.min(hi, v))
    }
    function _wrapIndex(i) {
        if (!model)
            return 0
        var n = model.count
        if (n <= 0)
            return 0
        return ((i % n) + n) % n
    }

    function _thumbOf(item) {
        if (!item)
            return ""
        var t = item.thumbnail !== undefined ? item.thumbnail
              : (item.thumb !== undefined ? item.thumb : "")
        if (!t)
            t = item.path || ""
        return t ? ImageService.fileUrl(t) : ""
    }

    function _select(idx) {
        if (!model || model.count === 0)
            return
        var c = _wrapIndex(idx)
        if (c === selectedIndex)
            return
        selectedIndex = c
        selectionChanged(c)
    }

    function _step(delta) {
        if (!model || model.count === 0 || delta === 0)
            return
        _slide = delta * (sliceWidth * spacing)
        _slideAnim.restart()
        _select(selectedIndex + delta)
    }

    function _activate() {
        if (!model || model.count === 0)
            return
        var it = model.get(selectedIndex)
        if (it)
            itemActivated(it)
    }

    // Resolve every geometric/opacity component for one card in a single pass.
    function _place(off, slot, rnd, rnd2, dist) {
        var W = width
        var H = height
        var cx = W * center
        var cy = H * 0.5

        var laneCount = Math.max(2, Math.min(strands, 6))
        // Weave the strand from the signed offset off the centre card, so the
        // selected card always rides the middle line and its neighbours braid
        // evenly above and below it. Keying the lane off `slot` (the render
        // index) instead stranded the centre card in whichever lane
        // slot % lanes happened to land it, so it hung low.
        var laneFrac = 0.5 * Math.sin(off * Math.PI / laneCount)

        var step = sliceWidth * spacing
        var sgn = off < 0 ? -1 : 1
        var comp = sgn * Math.pow(Math.abs(off), 0.85)
        var linX = cx + comp * step

        var waistF = 1 - waist * Math.exp(-(off * off) / (2 * 2.2 * 2.2))
        var arcY = arc * (off * off) * (sliceHeight * 0.012)
        var laneY = laneFrac * sliceHeight * 0.55 * waistF
        var linY = cy + arcY + laneY

        var ph = _clock * (2 * Math.PI) * (1000 / Math.max(200, duration))
               * (1 + edgeSpeed * dist) + rnd * 6.283

        var ringR = ringSize * Math.min(W, H) * 0.5
                  * (1 + ringWave * Math.sin(off * 0.6 + ph))
        var ringAng = (ringSpin * Math.PI / 180) + off * (fan * Math.PI / 180) * 0.6
        var ringX = cx + Math.sin(ringAng) * ringR
        var ringY = cy - Math.cos(ringAng) * ringR * 0.55

        var baseX = _mix(linX, ringX, ringBlend)
        var baseY = _mix(linY, ringY, ringBlend)

        var hold = 1 - ringHold * ringBlend
        var animX = orbit * sliceWidth * 0.12 * Math.cos(ph)
        var animY = orbit * sliceHeight * 0.12 * Math.sin(ph)
        var turbX = turbulence * sliceWidth * 0.18 * Math.sin(ph * 1.3 + rnd * 10.0)
        var turbY = turbulence * sliceHeight * 0.18 * Math.cos(ph * 0.9 + rnd2 * 8.0)

        var px = baseX + _slide + (animX + turbX) * hold
        var py = baseY + (animY + turbY) * hold

        var rot = skew + twist * Math.sin(ph) + fan * off * 0.35
        var frontScale = 1 + front * (1 - dist) * 0.45
        var sc = frontScale * (0.72 + 0.28 * (1 - dist))
        var z = Math.round(1000 * ((1 - dist) + front * (1 - dist)))

        var baseOp = 0.32 + 0.68 * (1 - dist)
        var grainDim = 1 - grain * rnd2 * 0.55
        var softDim = 1 - ringSoft * ringBlend * dist * 0.65
        var op = _clamp(baseOp * grainDim * softDim, 0.05, 1)

        return {
            "x": px - sliceWidth / 2,
            "y": py - sliceHeight / 2,
            "rot": rot,
            "sc": sc,
            "op": op,
            "z": z
        }
    }

    NumberAnimation {
        id: _slideAnim
        target: sandyView
        property: "_slide"
        to: 0
        duration: 220
        easing.type: Easing.OutCubic
    }

    Timer {
        interval: 16
        repeat: true
        running: sandyView.active && sandyView._count > 0
        onTriggered: sandyView._clock += 0.016
    }

    Connections {
        target: sandyView.model
        function onCountChanged() {
            var n = sandyView.model ? sandyView.model.count : 0
            if (n <= 0) {
                if (sandyView.selectedIndex !== 0)
                    sandyView.selectedIndex = 0
                return
            }
            if (sandyView.selectedIndex >= n) {
                sandyView.selectedIndex = n - 1
                sandyView.selectionChanged(n - 1)
            } else if (sandyView.selectedIndex < 0) {
                sandyView.selectedIndex = 0
                sandyView.selectionChanged(0)
            }
        }
    }

    WheelHandler {
        onWheel: function(event) {
            var d = event.angleDelta.y || event.angleDelta.x
            if (d !== 0)
                sandyView._step(d < 0 ? 1 : -1)
        }
    }

    Keys.onPressed: function(event) {
        if (event.key === Qt.Key_Left) {
            sandyView._step(-1)
            event.accepted = true
        } else if (event.key === Qt.Key_Right) {
            sandyView._step(1)
            event.accepted = true
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
            sandyView._activate()
            event.accepted = true
        }
    }

    Repeater {
        model: sandyView._count

        delegate: Item {
            id: card

            required property int index

            readonly property int slot: index
            readonly property int modelIndex: sandyView._wrapIndex(
                sandyView.selectedIndex - sandyView._half + slot)
            readonly property real off: slot - sandyView._half
            readonly property real dist: Math.min(
                1, Math.abs(off) / Math.max(1, sandyView._half))
            readonly property real rnd: sandyView._hash(modelIndex + 1)
            readonly property real rnd2: sandyView._hash(modelIndex * 2 + 7)
            readonly property var itemData: sandyView.model ? sandyView.model.get(modelIndex) : null
            readonly property bool isSelected: modelIndex === sandyView.selectedIndex
            property bool hovered: false

            readonly property var p: sandyView._place(off, slot, rnd, rnd2, dist)

            width: sandyView.sliceWidth
            height: sandyView.sliceHeight
            transformOrigin: Item.Center

            x: p.x
            y: p.y
            z: p.z
            rotation: p.rot
            scale: p.sc
            opacity: p.op

            Rectangle {
                anchors.fill: parent
                radius: 10
                color: sandyView.colors
                       ? Qt.rgba(sandyView.colors.surfaceContainer.r,
                                 sandyView.colors.surfaceContainer.g,
                                 sandyView.colors.surfaceContainer.b, 0.85)
                       : Qt.rgba(0.08, 0.09, 0.12, 0.85)
                border.width: card.isSelected ? 2 : (card.hovered ? 1.5 : 1)
                border.color: card.isSelected
                    ? (sandyView.colors ? sandyView.colors.primary : Style.fallbackAccent)
                    : Qt.rgba(1, 1, 1, card.hovered ? 0.35 : 0.12)

                Image {
                    anchors.fill: parent
                    anchors.margins: 3
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                    smooth: true
                    source: sandyView._thumbOf(card.itemData)
                    sourceSize.width: ImageService.thumbWidth
                    sourceSize.height: ImageService.thumbHeight
                }

                Rectangle {
                    anchors.fill: parent
                    anchors.margins: 3
                    visible: !card.isSelected
                    color: Qt.rgba(0, 0, 0, 0.15 + card.dist * 0.25)
                }

                Rectangle {
                    visible: card.itemData && card.itemData.favourite === true
                    width: 10
                    height: 10
                    radius: 5
                    anchors.top: parent.top
                    anchors.right: parent.right
                    anchors.margins: 7
                    color: sandyView.colors ? sandyView.colors.tertiary : "#8bceff"
                }

                Rectangle {
                    visible: card.isSelected && card.itemData
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.margins: 3
                    height: nameText.implicitHeight + 10
                    color: Qt.rgba(0, 0, 0, 0.55)

                    Text {
                        id: nameText
                        anchors.fill: parent
                        anchors.margins: 5
                        text: card.itemData
                              ? ("" + (card.itemData.name || "")).replace(/\.[^/.]+$/, "")
                              : ""
                        elide: Text.ElideRight
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                        color: "white"
                        font.family: Style.fontFamily
                        font.pixelSize: 12
                        font.weight: Font.Medium
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onEntered: card.hovered = true
                    onExited: card.hovered = false
                    onClicked: {
                        var it = card.itemData
                        sandyView._select(card.modelIndex)
                        if (it)
                            sandyView.itemActivated(it)
                    }
                }
            }
        }
    }
}
