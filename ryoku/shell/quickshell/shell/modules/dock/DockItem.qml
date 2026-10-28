pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import shell.services
import "../../components"

// One dock island: a frosted pill carrying an app icon, its running-window
// indicator and an active tint. It fills the slot the band positions for it and
// is grown by the band's cursor-tracked magnify. Left click activates the app,
// middle click launches a fresh instance, right click pins/unpins. Only a pinned
// island is draggable; the drag, the ghost and the live reorder live on the
// band, so the DragHandler here just relays the gesture. A zero-client island
// that launches an app bounces once as launch feedback.
Item {
    id: item

    required property var band
    required property real alongCenter
    required property int pinIndex
    required property string className
    // Position across the whole band (pins then running), for the ledger index.
    required property int ordinal
    readonly property bool pinned: item.pinIndex >= 0

    readonly property bool horizontal: item.band.horizontal
    readonly property string edge: item.band.edge
    readonly property real iconSize: item.band.iconSize

    // Live window count drives the running indicator and the launch bounce (a
    // click that starts from zero clients is the one that launches).
    readonly property int count: Dock.countFor(item.className)
    readonly property bool isActive: Dock.activeClass === item.className
    // Emphasis is inversion, never the accent: the running mark on the active
    // island is dark ink on its bone plate, otherwise bone ink on the dark one.
    readonly property color indColor: item.isActive ? Theme.inverseOnSurface : Theme.onSurface

    // Continuous magnify: distance from the cursor along the band, gaussian-ish
    // falloff, gated on the dock's magnify key (the band folds in reduce-motion).
    // A separate one-shot bounce multiplies in on launch.
    //
    // `hovered` is part of the gate, not decoration: cursorAlong only ever takes
    // a value from a point INSIDE the band, so on exit it keeps the last position
    // and whatever icon sat there would stay grown until the next hover landed
    // somewhere else.
    readonly property real mag: {
        if (!item.band.magnify || !item.band.hovered)
            return 1;
        const t = Math.max(0, 1 - Math.abs(item.alongCenter - item.band.cursorAlong) / item.band.reach);
        return 1 + (item.band.maxScale - 1) * t * t;
    }
    property real bounce: 1

    // Static per-item chrome for every form except islands (whose pill lives in
    // the magnified `content` below). A child of the item, not the transformed
    // content, so a rail underline, ledger cell or tanzaku strip stays a fixed
    // part of the sheet while only the icon rises under the cursor.
    DockItemChrome {
        anchors.fill: parent
        item: item
        band: item.band
        visible: item.band.style !== "islands"
    }

    Item {
        id: content
        anchors.fill: parent

        // Pop-in: a freshly created island scales and fades up from the dock's
        // edge. `shown` flips true on completion so the Behaviors animate the
        // entrance; under reduce-motion the Behaviors are off and it just appears.
        property bool shown: false
        Component.onCompleted: content.shown = true
        opacity: content.shown ? 1 : 0
        Behavior on opacity {
            enabled: item.band.animate
            NumberAnimation { duration: Motion.rowFade; easing.type: Motion.rowRevealCurve }
        }

        // Pop-in scale from the dock's outer edge, on its own channel so it
        // animates even when magnify is off.
        scale: content.shown ? 1 : 0.3
        transformOrigin: item.horizontal
            ? (item.edge === "bottom" ? Item.Bottom : Item.Top)
            : (item.edge === "right" ? Item.Right : Item.Left)
        Behavior on scale { enabled: item.band.animate; NumberAnimation { duration: Motion.standard; easing.type: Easing.OutBack } }

        // Magnify and the launch bounce, both anchored on the dock's outer edge so
        // an island grows into the headroom, never off the screen edge. Two
        // channels: magnify smooths under a Behavior; the bounce is driven straight
        // by its spring, so it stays crisp and still works with magnify off.
        transform: [
            Scale {
                origin.x: item.horizontal ? content.width / 2 : (item.edge === "right" ? content.width : 0)
                origin.y: item.horizontal ? (item.edge === "bottom" ? content.height : 0) : content.height / 2
                xScale: item.mag
                yScale: item.mag
                Behavior on xScale { enabled: item.band.magnify && item.band.animate; NumberAnimation { duration: Motion.hover; easing.type: Motion.easeStandard } }
                Behavior on yScale { enabled: item.band.magnify && item.band.animate; NumberAnimation { duration: Motion.hover; easing.type: Motion.easeStandard } }
            },
            Scale {
                origin.x: item.horizontal ? content.width / 2 : (item.edge === "right" ? content.width : 0)
                origin.y: item.horizontal ? (item.edge === "bottom" ? content.height : 0) : content.height / 2
                xScale: item.bounce
                yScale: item.bounce
            }
        ]

        Rectangle {
            id: pill
            visible: item.band.style === "islands"
            anchors.fill: parent
            radius: item.band.radius
            readonly property real frostA: item.band.frost ? 0.6 : 0.94
            // Active inverts to a bone plate at the SAME frost alpha (frost
            // behaviour unchanged); inactive keeps the frosted container.
            color: item.isActive
                ? Qt.rgba(Theme.inverseSurface.r, Theme.inverseSurface.g, Theme.inverseSurface.b, pill.frostA)
                : Qt.rgba(Theme.surfaceContainer.r, Theme.surfaceContainer.g, Theme.surfaceContainer.b, pill.frostA)
            border.width: 1
            border.color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.12)

            RectangularShadow {
                anchors.fill: parent
                radius: parent.radius
                blur: 12
                spread: 0
                offset: item.horizontal
                    ? Qt.vector2d(0, item.edge === "bottom" ? -2 : 2)
                    : Qt.vector2d(item.edge === "right" ? -2 : 2, 0)
                color: Qt.rgba(Theme.shadow.r, Theme.shadow.g, Theme.shadow.b, 0.5)
                visible: item.band.shadow && !Perf.shadowsDisabled
                z: -1
            }
        }

        Image {
            id: dockIcon
            anchors.centerIn: parent
            width: item.iconSize
            height: item.iconSize
            visible: source !== "" && status === Image.Ready
            source: Dock.iconFor(item.className)
            // Size the pixmap for the magnified icon so a grown icon never softens.
            sourceSize.width: Math.round(item.iconSize * item.band.maxScale)
            sourceSize.height: Math.round(item.iconSize * item.band.maxScale)
            smooth: true
            mipmap: true
            asynchronous: true
            // seal draws an idle app as a bone silhouette (desaturate + tint to
            // ink, the same MultiEffect DitherImage falls back to); a running app
            // keeps its real colour, so colour is the only "alive" mark.
            layer.enabled: item.band.style === "seal" && item.count === 0
            layer.effect: MultiEffect {
                saturation: -1
                colorization: 1
                colorizationColor: item.band.bone
            }
        }

        SymbolIcon {
            anchors.centerIn: parent
            visible: !dockIcon.visible
            name: "view-app-grid"
            size: item.iconSize
            color: item.indColor
        }

        // Running indicator on the dock's outer edge: up to three 4 px dots, or a
        // single 14x4 bar once four or more windows are open.
        Row {
            spacing: 3
            visible: item.band.style === "islands" && item.count >= 1 && item.count <= 3
            anchors.horizontalCenter: item.horizontal ? parent.horizontalCenter : undefined
            anchors.verticalCenter: item.horizontal ? undefined : parent.verticalCenter
            anchors.bottom: (item.horizontal && item.edge === "bottom") ? parent.bottom : undefined
            anchors.top: (item.horizontal && item.edge === "top") ? parent.top : undefined
            anchors.left: (!item.horizontal && item.edge === "left") ? parent.left : undefined
            anchors.right: (!item.horizontal && item.edge === "right") ? parent.right : undefined
            anchors.margins: 3
            Repeater {
                model: item.count
                delegate: Rectangle { width: 4; height: 4; radius: 2; color: item.indColor }
            }
        }
        Rectangle {
            visible: item.band.style === "islands" && item.count >= 4
            width: item.horizontal ? 14 : 4
            height: item.horizontal ? 4 : 14
            radius: 2
            color: item.indColor
            anchors.horizontalCenter: item.horizontal ? parent.horizontalCenter : undefined
            anchors.verticalCenter: item.horizontal ? undefined : parent.verticalCenter
            anchors.bottom: (item.horizontal && item.edge === "bottom") ? parent.bottom : undefined
            anchors.top: (item.horizontal && item.edge === "top") ? parent.top : undefined
            anchors.left: (!item.horizontal && item.edge === "left") ? parent.left : undefined
            anchors.right: (!item.horizontal && item.edge === "right") ? parent.right : undefined
            anchors.margins: 3
        }
    }

    // A short spring on launch: only a click that starts an app (zero clients, or
    // a middle-click new instance) bounces, and reduce-motion drops it.
    function launchBounce() {
        if (Perf.reduceMotion)
            return;
        bounceAnim.restart();
    }
    SequentialAnimation {
        id: bounceAnim
        NumberAnimation { target: item; property: "bounce"; to: 1.18; duration: Motion.hover; easing.type: Easing.OutBack }
        NumberAnimation { target: item; property: "bounce"; to: 1; duration: Motion.fast; easing.type: Easing.OutBack }
    }

    HoverHandler {
        cursorShape: Qt.PointingHandCursor
    }

    TapHandler {
        acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
        onSingleTapped: (eventPoint, button) => {
            if (button === Qt.RightButton) {
                const g = item.mapToGlobal(item.width / 2, item.height / 2);
                Dock.openMenu(item.className, item.pinned, item.count, g.x, g.y, item.band.screenName, item.band.edge, item.band.reservedDepth);
                return;
            }
            if (button === Qt.MiddleButton) {
                Dock.launch(item.className);
                item.launchBounce();
                return;
            }
            if (item.count === 0)
                item.launchBounce();
            Dock.activate(item.className);
        }
    }

    // Only a pinned island reorders; the gesture is relayed to the band, which
    // owns the ghost and the live swap.
    DragHandler {
        id: drag
        enabled: item.pinned
        acceptedButtons: Qt.LeftButton
        target: null
        grabPermissions: PointerHandler.CanTakeOverFromAnything
        onActiveChanged: {
            if (drag.active)
                item.band.beginDrag(item.pinIndex, item.className);
            else
                item.band.endDrag();
        }
        onCentroidChanged: if (drag.active) item.band.dragMove(drag.centroid.scenePosition);
    }
}
