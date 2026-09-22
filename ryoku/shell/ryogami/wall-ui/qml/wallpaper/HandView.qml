pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import ".."
import "../services"

// A fanned "hand of cards" wallpaper picker. Cards splay from a shared pivot in
// a shallow arc; the selected one lifts to the centre. Only a bounded window of
// cards around the selection is ever built, so the fan costs the same whether
// the catalogue holds twelve wallpapers or twelve thousand. The whole fan reads
// off one animated scroll value, so a single easing curve drives every card's
// slide, lift, scale and depth at once.
Item {
    id: handView

    // ── shared picker contract ───────────────────────────────────────────
    property var model                       // catalogue: count + get(i) -> role object
    property var colors                      // Ryoku palette (see Colors.qml)
    property bool active: false              // owns keyboard focus and animations
    property int selectedIndex: 0            // index of the centred card

    signal selectionChanged(int index)       // user moved the selection
    signal itemActivated(var item)           // user committed (double-click / return)

    // ── fan geometry ─────────────────────────────────────────────────────
    property int cardWidth: 200              // one card's width in px
    property int cardHeight: 300             // one card's height in px
    property int cardCount: 9                // cards materialised (window size, incl. fade buffer)
    property real fanAngle: 7                // degrees of rotation added per card away from centre
    property real fanRoll: 0                 // constant rotation applied to the whole fan
    property real arch: 26                   // px the outer cards drop below the centre card
    property real spread: 96                 // horizontal px between neighbouring card centres
    property real skew: 0                    // horizontal shear of each card (parallelogram lean)
    property real cornerRadius: Style.radiusLarge
    property real tilt: 8                    // degrees the whole fan tips back (3D)
    property real perspective: 18            // how strongly distance shrinks a card (0 = flat)
    property real speed: 1.0                 // slide-animation speed multiplier (higher = quicker)
    property int ghosts: 2                   // trailing motion-blur copies per moving card
    property real bob: 4                     // idle vertical bob amplitude of the centre card (px)
    property bool backdrop: true             // dim, blurred hero behind the fan

    // ── internal state ───────────────────────────────────────────────────
    readonly property int _count: model ? model.count : 0
    readonly property int _windowCount: Math.max(1, Math.min(cardCount, _count))
    readonly property int _half: Math.floor(_windowCount / 2)
    property real _scroll: selectedIndex     // animated position the fan is centred on
    readonly property int _centerInt: Math.round(_scroll)
    readonly property int _dur: Math.max(60, Math.round(Style.animExpand / Math.max(0.1, speed)))
    readonly property real _moveDir: _scroll < selectedIndex ? 1 : (_scroll > selectedIndex ? -1 : 0)
    property real _bobY: 0

    // selection lift/scale are polish, derived from card size so they scale with it
    readonly property real _selLift: cardHeight * 0.16
    readonly property real _selGrow: 0.14
    readonly property real _baseY: height * 0.56

    clip: false
    focus: active

    // Opacity ramp that fades a card out just before it reaches the window edge,
    // so the seamless index recycling at the extremes is never seen.
    function _edgeOpacity(rel) {
        var a = Math.abs(rel)
        var full = Math.max(0.5, _half - 1.0)
        if (a <= full) return 1
        return Math.max(0, 1 - (a - full) / 0.6)
    }
    // 1 at the centre, falling to 0 one card-step out; drives lift and scale.
    function _selFactor(rel) {
        return Math.max(0, 1 - Math.abs(rel))
    }
    function _thumbOf(item) {
        if (!item) return ""
        return item.thumbnail || item.thumb || item.path || ""
    }
    function _isFav(item) {
        return !!(item && (item.favourite === true || item.favorite === true))
    }

    function _select(i) {
        var c = _count
        if (c <= 0) return
        var n = Math.max(0, Math.min(c - 1, i))
        if (n === selectedIndex) return
        selectedIndex = n
        selectionChanged(n)
    }
    function _step(d) { _select(selectedIndex + d) }
    function _activate() {
        if (!model || _count <= 0) return
        var idx = Math.max(0, Math.min(_count - 1, selectedIndex))
        itemActivated(model.get ? model.get(idx) : idx)
    }

    onSelectedIndexChanged: _scroll = selectedIndex
    onActiveChanged: if (active) forceActiveFocus()

    // Keep the selection valid when the catalogue changes underneath us.
    Connections {
        target: handView.model
        function onCountChanged() {
            var c = handView._count
            if (c <= 0) { handView.selectedIndex = 0; handView._scroll = 0; return }
            if (handView.selectedIndex > c - 1) {
                handView.selectedIndex = c - 1
                handView._scroll = c - 1
            }
        }
    }

    Behavior on _scroll { NumberAnimation { id: scrollAnim; duration: handView._dur; easing.type: Easing.OutCubic } }
    Behavior on spread { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
    Behavior on arch { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
    Behavior on fanAngle { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
    Behavior on fanRoll { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
    Behavior on tilt { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
    Behavior on skew { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
    Behavior on perspective { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
    Behavior on cardWidth { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
    Behavior on cardHeight { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
    Behavior on cornerRadius { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }

    // Gentle idle float of the centre card, paused while sliding.
    SequentialAnimation {
        running: handView.active && handView.bob > 0 && !scrollAnim.running
        loops: Animation.Infinite
        NumberAnimation { target: handView; property: "_bobY"; from: 0; to: handView.bob; duration: 1400; easing.type: Easing.InOutSine }
        NumberAnimation { target: handView; property: "_bobY"; from: handView.bob; to: 0; duration: 1400; easing.type: Easing.InOutSine }
        onStopped: handView._bobY = 0
    }

    // Dim, softly blurred hero drawn from the selected card, sitting behind the
    // fan. One image plus one effect, only while there is a selection to show.
    Item {
        id: backdropLayer
        anchors.fill: parent
        visible: handView.backdrop && backdropImage.source != ""
        z: -10

        Image {
            id: backdropImage
            anchors.fill: parent
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            cache: true
            smooth: true
            sourceSize.width: 320
            sourceSize.height: 320
            source: {
                var it = (handView.model && handView._count > 0)
                    ? handView.model.get(Math.max(0, Math.min(handView._count - 1, handView.selectedIndex)))
                    : null
                var t = handView._thumbOf(it)
                return t ? ImageService.fileUrl(t) : ""
            }
            layer.enabled: backdropLayer.visible
            layer.effect: MultiEffect {
                blurEnabled: true
                blur: 1.0
                blurMax: 48
                autoPaddingEnabled: false
            }
        }
        Rectangle {
            anchors.fill: parent
            color: handView.colors ? handView.colors.background : "#000000"
            opacity: 0.72
        }
    }

    // The fan plane. A back-axis tilt gives the arc a little 3D depth for free.
    Item {
        id: fanLayer
        anchors.fill: parent
        transform: Rotation {
            origin.x: fanLayer.width / 2
            origin.y: fanLayer.height * 0.72
            axis { x: 1; y: 0; z: 0 }
            angle: handView.tilt
        }

        Repeater {
            model: handView._windowCount

            delegate: Item {
                id: card
                required property int index

                readonly property int catIdx: handView._centerInt - handView._half + index
                readonly property bool inRange: catIdx >= 0 && catIdx < handView._count
                readonly property var item: (inRange && handView.model) ? handView.model.get(catIdx) : null
                readonly property real rel: catIdx - handView._scroll
                readonly property real sel: handView._selFactor(rel)
                readonly property bool isSelected: catIdx === handView.selectedIndex
                readonly property bool hovered: cardMouse.containsMouse
                readonly property real distScale: 1 / (1 + Math.abs(rel) * (handView.perspective / 100))
                readonly property string thumbUrl: item ? ImageService.fileUrl(handView._thumbOf(item)) : ""

                width: handView.cardWidth
                height: handView.cardHeight

                x: fanLayer.width / 2 + rel * handView.spread - width / 2
                y: handView._baseY + handView.arch * rel * rel
                   - handView._selLift * sel - handView._bobY * sel - height

                visible: inRange && opacity > 0.02
                opacity: handView._edgeOpacity(rel)
                Behavior on opacity { NumberAnimation { duration: handView._dur; easing.type: Easing.OutCubic } }

                z: Math.round(1000 * sel) + Math.round(120 - Math.abs(rel) * 12) + (hovered ? 500 : 0)

                transform: [
                    Scale {
                        origin.x: card.width / 2
                        origin.y: card.height
                        xScale: card.distScale * (1 + handView._selGrow * card.sel) * (card.hovered ? 1.04 : 1)
                        yScale: card.distScale * (1 + handView._selGrow * card.sel) * (card.hovered ? 1.04 : 1)
                    },
                    Rotation {
                        origin.x: card.width / 2
                        origin.y: card.height
                        axis { x: 0; y: 0; z: 1 }
                        angle: card.rel * handView.fanAngle + handView.fanRoll
                    },
                    Matrix4x4 {
                        matrix: Qt.matrix4x4(1, handView.skew * 0.01, 0, 0,
                                             0, 1, 0, 0,
                                             0, 0, 1, 0,
                                             0, 0, 0, 1)
                    }
                ]

                // Drop shadow: a soft dark plate offset beneath the card.
                Rectangle {
                    anchors.fill: parent
                    anchors.topMargin: 6 + 8 * card.sel
                    radius: handView.cornerRadius
                    color: handView.colors ? handView.colors.shadow : "#000000"
                    opacity: 0.35 * card.opacity
                    z: -2
                }

                // Trailing motion ghosts, drawn behind and only while sliding.
                Repeater {
                    model: (handView.ghosts > 0 && scrollAnim.running && card.thumbUrl != "")
                        ? Math.min(4, handView.ghosts) : 0
                    delegate: Image {
                        required property int index
                        anchors.fill: parent
                        source: card.thumbUrl
                        fillMode: Image.PreserveAspectCrop
                        cache: true
                        asynchronous: true
                        smooth: true
                        sourceSize.width: Math.ceil(handView.cardWidth)
                        sourceSize.height: Math.ceil(handView.cardHeight)
                        x: handView._moveDir * (index + 1) * (handView.spread * 0.12)
                        opacity: 0.16 * (1 - (index + 1) / (Math.min(4, handView.ghosts) + 1))
                        z: -1
                    }
                }

                // Rounded-corner mask for the thumbnail.
                Rectangle {
                    id: cardMask
                    anchors.fill: parent
                    radius: handView.cornerRadius
                    color: "white"
                    visible: false
                    layer.enabled: card.visible
                }

                // Placeholder while the thumbnail decodes / when absent.
                Rectangle {
                    anchors.fill: parent
                    radius: handView.cornerRadius
                    visible: thumbImage.status !== Image.Ready
                    color: handView.colors
                        ? Qt.rgba(handView.colors.surfaceVariant.r, handView.colors.surfaceVariant.g, handView.colors.surfaceVariant.b, 0.9)
                        : Qt.rgba(0.06, 0.06, 0.06, 0.9)
                }

                Image {
                    id: thumbImage
                    anchors.fill: parent
                    source: card.thumbUrl
                    fillMode: Image.PreserveAspectCrop
                    smooth: true
                    asynchronous: true
                    cache: true
                    sourceSize.width: Math.ceil(handView.cardWidth * 1.15)
                    sourceSize.height: Math.ceil(handView.cardHeight * 1.15)
                    opacity: status === Image.Ready ? 1 : 0
                    Behavior on opacity { NumberAnimation { duration: Style.animNormal; easing.type: Easing.OutCubic } }
                    layer.enabled: card.visible
                    layer.smooth: true
                    layer.effect: MultiEffect {
                        maskEnabled: true
                        maskSource: cardMask
                        maskThresholdMin: 0.4
                        maskSpreadAtMin: 0.4
                    }
                }

                // Darken the shoulder cards so the centre reads as chosen.
                Rectangle {
                    anchors.fill: parent
                    radius: handView.cornerRadius
                    color: "#000000"
                    opacity: (1 - card.sel) * 0.42 * (card.hovered ? 0.4 : 1)
                    Behavior on opacity { NumberAnimation { duration: handView._dur } }
                }

                // Accent frame on the selected card.
                Rectangle {
                    anchors.fill: parent
                    radius: handView.cornerRadius
                    color: "transparent"
                    border.width: card.isSelected ? Style.borderThick : Style.borderThin
                    border.color: card.isSelected
                        ? (handView.colors ? handView.colors.primary : Style.fallbackAccent)
                        : Qt.rgba(0, 0, 0, 0.4)
                    Behavior on border.color { ColorAnimation { duration: Style.animFast } }
                }

                // Favourite pip, top-right of the centred card.
                Rectangle {
                    anchors.top: parent.top
                    anchors.right: parent.right
                    anchors.margins: 8
                    width: 22; height: 22; radius: 11
                    visible: handView._isFav(card.item) && card.sel > 0.5
                    color: Qt.rgba(0, 0, 0, 0.6)
                    Text {
                        anchors.centerIn: parent
                        text: "\u2665"
                        font.pixelSize: 12
                        color: handView.colors ? handView.colors.primary : Style.fallbackAccent
                    }
                }

                MouseArea {
                    id: cardMouse
                    anchors.fill: parent
                    hoverEnabled: true
                    enabled: card.inRange && handView.active
                    onClicked: handView._select(card.catIdx)
                    onDoubleClicked: {
                        handView._select(card.catIdx)
                        handView.itemActivated(card.item)
                    }
                }
            }
        }
    }

    // Wheel scrolls the selection; vertical or horizontal, either works.
    WheelHandler {
        enabled: handView.active
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        onWheel: function (ev) {
            var d = ev.angleDelta.y !== 0 ? ev.angleDelta.y : ev.angleDelta.x
            if (d > 0) handView._step(-1)
            else if (d < 0) handView._step(1)
        }
    }

    Keys.enabled: active
    Keys.onLeftPressed: _step(-1)
    Keys.onRightPressed: _step(1)
    Keys.onReturnPressed: _activate()
    Keys.onEnterPressed: _activate()
    Keys.onSpacePressed: _activate()
}
