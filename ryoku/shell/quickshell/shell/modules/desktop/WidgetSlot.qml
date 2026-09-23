pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Effects
import Qt5Compat.GraphicalEffects
import "Singletons"

// placement / shape / interaction frame for one desktop widget. measures its
// single child's natural size, pads it, positions it either at a compass
// zone (a fixed margin in from that edge) or, once dragged, at a free
// monitor pixel. draws the chosen backing (none|card|glass). carries the
// desktop interaction: drag to move (grid-snapped, with a press bump),
// right-click for the menu, and a per-widget lock that freezes everything.
// dragging persists a "free" position to the widgets Config; the menu and
// Ryoku Settings write the same file. the grip takes the press over the
// desktop catcher beneath it, so a right-click on the widget opens its
// menu, not the desktop one.
Item {
    id: slot

    property string widget: "clock"            // config prefix, for persistence
    property string anchor: "top-left"         // auto | 9 zones | free
    property real freeX: 72
    property real freeY: 64
    property bool locked: false
    property real pad: 0
    property string bg: "none"                 // none | card | glass
    property real radius: Theme.radius
    property real gridSize: 32
    property real zoneMargin: 64
    property real scaleCfg: 1                   // current Config <widget>Scale, for the resize readout
    // While an Edit widgets session is on, keep the resize bracket present (not
    // just on hover): the frame overlay above intercepts hover, so a hover-only
    // bracket would never reveal, leaving the widget un-resizable in the editor.
    property bool composing: false

    signal menuRequested(real x, real y, string widget)
    // emitted on drop with the slot's final pixel box, so the desktop layer can
    // flash the edges it landed on (and any centre line it snapped to). reported
    // one way, as a signal, like menuRequested.
    signal dropped(rect box)
    // emitted whenever a resize (the corner bracket or Ctrl+wheel) persists a
    // new scale, so the edit session can mark itself dirty.
    signal resized()

    default property alias content: holder.data

    readonly property var item: holder.children.length > 0 ? holder.children[0] : null
    readonly property real cw: slot.item ? slot.item.implicitWidth : 0
    readonly property real ch: slot.item ? slot.item.implicitHeight : 0
    // true while the hosted widget wants the keyboard (its own `editing` flag).
    // the host raises the layer's keyboard grab off this, the same way plugin
    // tiles do; the clock never exposes it, so it stays input-passive.
    readonly property bool editing: slot.visible && !!(slot.item && slot.item.editing)

    // custom ink: a bg:none widget can wear a pinned colour or an A->B sweep in
    // place of its adaptive ink. The solid colour is pushed into the widget so it
    // paints only the ink (text/marks), never a card; a gradient is layered on top
    // via the mask, but only for card-less widgets (calendar/music/aio keep their
    // card, so they take the solid colour, never the mask). Empty = adaptive.
    readonly property string inkColorA: slot.bg === "none" ? (Config[slot.widget + "Color"] || "") : ""
    readonly property string inkColorB: Config[slot.widget + "Color2"] || ""
    readonly property bool cardWidget: slot.widget === "calendar" || slot.widget === "music" || slot.widget === "aio"
    readonly property bool inkGradient: slot.inkColorA !== "" && (Config[slot.widget + "Gradient"] === true) && slot.inkColorB !== ""
    readonly property bool inkMaskOn: slot.inkGradient && !slot.cardWidget

    // drag state. while holding (dragging, or briefly after release until
    // the config write lands) the rendered position follows the drag so it
    // doesn't flicker back to the old anchor for a frame.
    property bool dragging: false
    property real dragX: 0
    property real dragY: 0
    property bool resizing: false
    property real resizeOX: 0
    property real resizeOY: 0
    property real resizeStartScale: 1
    property real resizeStartDiag: 1
    readonly property bool holding: slot.dragging || slot.resizing || guard.running

    width: Math.max(1, slot.cw + slot.pad * 2)
    height: Math.max(1, slot.ch + slot.pad * 2)

    // A slot's parent is the Loader that hosts it; before the desktop window
    // has been laid out that parent momentarily reports width/height 0. Clamping
    // a saved position against a zero parent collapses every widget to (0,0) and
    // they stack in the corner (#251), so treat a non-positive parent as "not
    // laid out yet" and pass the requested value through untouched; the binding
    // re-resolves to the clamped position once the parent has a real size.
    function clampX(v) {
        const w = slot.parent ? slot.parent.width : 0;
        if (w <= 0)
            return v;
        return Math.max(0, Math.min(v, w - slot.width));
    }
    function clampY(v) {
        const h = slot.parent ? slot.parent.height : 0;
        if (h <= 0)
            return v;
        return Math.max(0, Math.min(v, h - slot.height));
    }
    function snap(v) { return Math.round(v / slot.gridSize) * slot.gridSize; }
    function zoneX() {
        const w = slot.parent ? slot.parent.width : 0;
        if (w <= 0)
            return slot.anchor.indexOf("left") >= 0 ? slot.zoneMargin : 0;
        if (slot.anchor.indexOf("left") >= 0) return slot.zoneMargin;
        if (slot.anchor.indexOf("right") >= 0) return w - slot.width - slot.zoneMargin;
        return (w - slot.width) / 2;
    }
    function zoneY() {
        const h = slot.parent ? slot.parent.height : 0;
        if (h <= 0)
            return slot.anchor.indexOf("top") >= 0 ? slot.zoneMargin : 0;
        if (slot.anchor.indexOf("top") >= 0) return slot.zoneMargin;
        if (slot.anchor.indexOf("bottom") >= 0) return h - slot.height - slot.zoneMargin;
        return (h - slot.height) / 2;
    }

    // anchor:"auto" -> the wallpaper's calmest patch. calmSpot returns a
    // screen-normalised top-left for a box of the slot's normalised size; scale
    // it back to monitor pixels. calmSpot reads the daemon's tone map internally,
    // so this binding tracks it and re-resolves -- gliding via the x/y Behaviors
    // below -- whenever a new wallpaper publishes a fresh map. marginN is the zone
    // pixel inset against the shorter screen axis, so an auto widget stays at
    // least as far off every edge as a zoned one.
    readonly property point autoPoint: {
        // only auto slots pay for calmSpot (and subscribe to the tone map);
        // a zoned or free slot must not re-lay-out on a wallpaper change.
        if (slot.anchor !== "auto")
            return Qt.point(0, 0);
        const pw = slot.parent ? slot.parent.width : 0;
        const ph = slot.parent ? slot.parent.height : 0;
        if (pw <= 0 || ph <= 0)
            return Qt.point(slot.freeX, slot.freeY);
        const s = Scheme.calmSpot(slot.width / pw, slot.height / ph,
            slot.zoneMargin / Math.min(pw, ph));
        return Qt.point(slot.clampX(s.x * pw), slot.clampY(s.y * ph));
    }

    x: slot.holding ? slot.dragX
        : slot.anchor === "free" ? slot.clampX(slot.freeX)
        : slot.anchor === "auto" ? slot.autoPoint.x : slot.zoneX()
    y: slot.holding ? slot.dragY
        : slot.anchor === "free" ? slot.clampY(slot.freeY)
        : slot.anchor === "auto" ? slot.autoPoint.y : slot.zoneY()

    Behavior on x { enabled: !slot.holding; NumberAnimation { duration: 240; easing.type: Easing.OutCubic } }
    Behavior on y { enabled: !slot.holding; NumberAnimation { duration: 240; easing.type: Easing.OutCubic } }

    // press bump: small lift while dragging, so it feels picked up.
    scale: slot.dragging ? 1.03 : 1.0
    transformOrigin: Item.Center
    Behavior on scale { NumberAnimation { duration: 140; easing.type: Easing.OutExpo } }

    // per-widget opacity (the menu and Ryoku Settings write <widget>Opacity),
    // clamped so a widget can fade back but never vanish or lose its clicks.
    opacity: Math.max(0.2, Math.min(1, Config[slot.widget + "Opacity"]))

    Timer { id: guard; interval: 90 }

    // the tone the hosted widget's ink actually sits on: the wallpaper under
    // this slot, or the backing plate composited over it. pushed into the
    // widget, which has no way to find out where on the screen it landed.
    readonly property real underL: {
        const pw = slot.parent ? slot.parent.width : 0;
        const ph = slot.parent ? slot.parent.height : 0;
        if (pw <= 0 || ph <= 0)
            return Scheme.wallLstar;
        const l = Scheme.lstarAt(slot.x / pw, slot.y / ph, slot.width / pw, slot.height / ph);
        return slot.bg === "none" ? l : Scheme.overLstar(l, backing.color);
    }
    Binding {
        target: slot.item
        property: "underL"
        value: slot.underL
        when: slot.item !== null && slot.item.underL !== undefined
    }
    // push the pinned solid colour into the widget so it paints its own ink (never
    // a card). "" leaves the widget on its adaptive inkOn(underL).
    Binding {
        target: slot.item
        property: "inkColorA"
        value: slot.inkColorA
        when: slot.item !== null && slot.item.inkColorA !== undefined
    }

    // soft lift off the wallpaper for the backed styles.
    MultiEffect {
        source: backing
        anchors.fill: backing
        visible: !Performance.shadowsDisabled && slot.bg !== "none"
        shadowEnabled: true
        shadowColor: Qt.rgba(0, 0, 0, 0.5)
        shadowBlur: 1.0
        shadowVerticalOffset: 6
        blurMax: 32
        autoPaddingEnabled: true
    }

    Rectangle {
        id: backing
        anchors.fill: parent
        visible: slot.bg !== "none"
        radius: slot.radius
        color: slot.bg === "card" ? Qt.rgba(0, 0, 0, 0.42) : Qt.rgba(16 / 255, 16 / 255, 24 / 255, 0.26)
        border.width: 1
        border.color: slot.bg === "card" ? Qt.rgba(1, 1, 1, 0.08) : Qt.rgba(1, 1, 1, 0.16)

        // glass sheen: faint top-down highlight so the panel reads as a pane
        // of glass rather than a flat fill.
        Rectangle {
            visible: slot.bg === "glass"
            anchors.fill: parent
            radius: parent.radius
            gradient: Gradient {
                GradientStop { position: 0.0; color: Qt.rgba(1, 1, 1, 0.10) }
                GradientStop { position: 0.5; color: Qt.rgba(1, 1, 1, 0.0) }
                GradientStop { position: 1.0; color: Qt.rgba(0, 0, 0, 0.06) }
            }
        }
    }

    // interaction grip UNDER the content: left-drag on bare widget area moves
    // the tile (grid-snapped) and right-click opens the menu, while an
    // interactive widget keeps its own clicks on top. the clock has no
    // interactive children, so the whole surface still drags. a grip above the
    // content would swallow every click.
    MouseArea {
        id: grip
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        hoverEnabled: true
        cursorShape: slot.locked ? Qt.ArrowCursor : (slot.dragging ? Qt.ClosedHandCursor : Qt.OpenHandCursor)

        property bool leftDown: false
        property real grabOX: 0
        property real grabOY: 0

        onPressed: (mouse) => {
            if (mouse.button === Qt.RightButton) {
                const pr = slot.mapToItem(slot.parent, mouse.x, mouse.y);
                slot.menuRequested(pr.x, pr.y, slot.widget);
                return;
            }
            if (slot.locked)
                return;
            grip.leftDown = true;
            const p = slot.mapToItem(slot.parent, mouse.x, mouse.y);
            grip.grabOX = p.x - slot.x;
            grip.grabOY = p.y - slot.y;
        }
        onPositionChanged: (mouse) => {
            if (!grip.leftDown || slot.locked)
                return;
            const p = slot.mapToItem(slot.parent, mouse.x, mouse.y);
            const nx = p.x - grip.grabOX;
            const ny = p.y - grip.grabOY;
            if (!slot.dragging) {
                if (Math.abs(nx - slot.x) < 6 && Math.abs(ny - slot.y) < 6)
                    return;
                slot.dragging = true;
            }
            slot.dragX = slot.clampX(slot.snap(nx));
            slot.dragY = slot.clampY(slot.snap(ny));
        }
        onReleased: (mouse) => {
            if (slot.dragging) {
                Config.setFree(slot.widget, Math.round(slot.dragX), Math.round(slot.dragY));
                slot.dropped(Qt.rect(slot.dragX, slot.dragY, slot.width, slot.height));
                slot.dragging = false;
                guard.restart();
            }
            grip.leftDown = false;
        }
    }

    // lift a bare widget off the wallpaper for legibility on any backdrop. a
    // card/glass panel already gives contrast, so the shadow only applies
    // when the widget sits directly on the wallpaper.
    Item {
        id: holder
        x: slot.pad
        y: slot.pad
        width: slot.cw
        height: slot.ch
        layer.enabled: slot.inkMaskOn || (!Performance.shadowsDisabled && slot.bg === "none")
        // a layer texture drawn at a fractional Wayland scale needs linear
        // filtering or the bare-widget ink crawls, worst during the press
        // bump and the drag, when the tile sits off the pixel grid.
        layer.smooth: true
        layer.effect: slot.inkMaskOn ? recolorFx : shadowFx
    }

    // the bare-widget shadow, and the ink recolour: a gradient (or a solid, both
    // stops equal) masked by the live content's alpha via LinearGradient's source,
    // so only the glyph shapes wear the pinned colour, never the box. Auto uses the
    // plain shadow and the widget's own adaptive inkOn(underL).
    Component {
        id: shadowFx
        MultiEffect {
            shadowEnabled: true
            shadowColor: Qt.rgba(0, 0, 0, 0.5)
            shadowBlur: 0.8
            shadowVerticalOffset: 2
            blurMax: 28
            autoPaddingEnabled: true
        }
    }
    Component {
        id: recolorFx
        LinearGradient {
            start: Qt.point(0, 0)
            end: Qt.point(0, height)
            gradient: Gradient {
                GradientStop { position: 0; color: slot.inkColorA }
                GradientStop { position: 1; color: slot.inkGradient ? slot.inkColorB : slot.inkColorA }
            }
        }
    }

    // hover state for the slot and its children, so the resize handle stays
    // lit while you reach across to it.
    HoverHandler { id: slotHover }

    // scroll to scale: Ctrl + wheel anywhere on the widget resizes it, an easier
    // reach than the corner bracket. setLive keeps it smooth; the settle timer
    // does the one persisting write once scrolling stops.
    WheelHandler {
        enabled: !slot.locked
        acceptedModifiers: Qt.ControlModifier
        onWheel: event => {
            const step = event.angleDelta.y > 0 ? 1.06 : 1 / 1.06;
            const ns = Math.max(0.5, Math.min(2.5, slot.scaleCfg * step));
            Config.setLive(slot.widget + "Scale", ns);
            scalePersist.restart();
        }
    }
    Timer {
        id: scalePersist
        interval: 350
        onTriggered: { Config.set(slot.widget + "Scale", slot.scaleCfg); slot.resized(); }
    }

    // quick resize: drag the bottom-right bracket to scrub the widget's
    // scale. top-left is pinned during the resize so it grows toward the
    // cursor; on release the new scale + a pinned free position persist in
    // one write.
    Item {
        id: handle
        width: 22
        height: 22
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        opacity: (((slotHover.hovered || slot.composing) && !slot.locked && !slot.dragging) || slot.resizing) ? 1 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: 120 } }

        Rectangle {
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            width: 13
            height: 2
            radius: Theme.radius
            color: (hgrip.containsMouse || slot.resizing) ? Theme.accent : Theme.faint
            Behavior on color { ColorAnimation { duration: 100 } }
        }
        Rectangle {
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            width: 2
            height: 13
            radius: Theme.radius
            color: (hgrip.containsMouse || slot.resizing) ? Theme.accent : Theme.faint
            Behavior on color { ColorAnimation { duration: 100 } }
        }

        MouseArea {
            id: hgrip
            anchors.fill: parent
            enabled: !slot.locked
            acceptedButtons: Qt.LeftButton
            hoverEnabled: true
            cursorShape: Qt.SizeFDiagCursor

            onPressed: (mouse) => {
                const ox = slot.x;
                const oy = slot.y;
                slot.dragX = ox;
                slot.dragY = oy;
                slot.resizeOX = ox;
                slot.resizeOY = oy;
                slot.resizeStartScale = slot.scaleCfg;
                const p = hgrip.mapToItem(slot.parent, mouse.x, mouse.y);
                slot.resizeStartDiag = Math.max(1, Math.hypot(p.x - ox, p.y - oy));
                slot.resizing = true;
            }
            onPositionChanged: (mouse) => {
                if (!slot.resizing)
                    return;
                const p = hgrip.mapToItem(slot.parent, mouse.x, mouse.y);
                const diag = Math.hypot(p.x - slot.resizeOX, p.y - slot.resizeOY);
                const ns = Math.max(0.5, Math.min(2.5, slot.resizeStartScale * diag / slot.resizeStartDiag));
                Config.setLive(slot.widget + "Scale", ns);
            }
            onReleased: (mouse) => {
                if (slot.resizing) {
                    Config.setFree(slot.widget, Math.round(slot.resizeOX), Math.round(slot.resizeOY));
                    slot.resizing = false;
                    slot.resized();
                    guard.restart();
                }
            }
        }
    }

    // live size readout while resizing.
    Rectangle {
        visible: slot.resizing || scalePersist.running
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.rightMargin: 26
        anchors.bottomMargin: 26
        width: roText.implicitWidth + 16
        height: 20
        radius: Theme.radius
        color: Qt.rgba(0, 0, 0, 0.62)
        Text {
            id: roText
            anchors.centerIn: parent
            text: Math.round(slot.scaleCfg * 100) + "%"
            color: Theme.ink
            font.family: Theme.mono
            font.pixelSize: 11
            font.weight: Font.DemiBold
        }
    }
}
