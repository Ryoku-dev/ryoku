pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons

// Notification popup surface (contract 07 sec 1/2.4, sec 8; contract 12 sec 1).
// A per-monitor overlay layer surface anchored to the top edge, on the left or
// right (or centred) per the position setting, reserving nothing (exclusive
// zone 0) and never taking keyboard focus. It holds the flat, newest-first popup
// list (newest at index 0) at a fixed 400 px content width with 14 px between
// cards, floating 16 px off the corner. Each card arrives from off the edge it
// is anchored to (slide + fade + a small scale settle, Motion.notifIn) and, on
// close, is flicked back out the same way (slide + shrink + fade, Motion.notifOut)
// while the cards below hold their slot until it clears, then rise. The surface
// holds its height for a card that is still leaving and unmaps Motion.notifHide
// after the list empties, so the last exit is neither clipped nor cut short; a
// popup arriving in that wait cancels the unmap.
//
// `Notifs.popups` is reassigned as a whole array on every change, which resets a
// plain view (no per-item add/remove animation). A local `cards` ListModel is
// reconciled against it incrementally so the ListView's add/remove/displaced
// transitions actually fire per card.
PanelWindow {
    id: win

    required property var modelData
    // Per-monitor UI scale (shell.json displays.ui_scale): multiplies this
    // surface's own logical sizes so one monitor's toasts shrink/grow without
    // touching the compositor scale. Default 1.0 is a byte-identical no-op.
    readonly property real us: Tokens.uiScaleFor(modelData ? modelData.name : "")
    // Base logical px: content width 400, inter-card spacing 14, corner float 16.
    // These are multiplied by `us` above (1.0 = the base), the only scale term;
    // the column does not otherwise grow with monitor or accessibility font scale.

    property string position: Config.notifications && Config.notifications.notification_position
        ? Config.notifications.notification_position : "Right"
    // popup_window_margins (contract 07 sec 8), default 0.
    property real margin: 0

    // Base breathing room off the screen corner, on top of the configurable
    // popup margin, so the toast column floats instead of sitting flush.
    readonly property real inset: 16 * win.us

    // How far off its own edge a card starts and leaves. Centred popups have no
    // edge to come from, so they only fade.
    readonly property real slideFrom: position === "Left" ? -implicitWidth
        : position === "Center" ? 0 : implicitWidth

    readonly property var popups: Notifs.popups
    property bool mapped: false
    readonly property string notificationUrl: Designs.notificationUrl
    property string failedNotificationUrl: ""
    onNotificationUrlChanged: win.failedNotificationUrl = ""

    screen: modelData
    visible: mapped && Config.barStyle !== "nacre" && !Config.styleOwnsBanners
    color: "transparent"
    // Exclusive zone 0: reserve nothing, respect other layers' zones (contract
    // 12 sec 1). ExclusionMode.Ignore would request -1 instead.
    exclusionMode: ExclusionMode.Normal
    exclusiveZone: 0
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
    WlrLayershell.namespace: "ryoku-notifications"

    anchors.top: position !== "BottomRight"
    anchors.bottom: position === "BottomRight"
    anchors.left: position === "Left"
    anchors.right: position === "Right" || position === "BottomRight"
    margins.top: inset + margin
    margins.left: inset + margin
    margins.right: inset + margin
    margins.bottom: inset + margin

    implicitWidth: 400 * win.us
    // The surface is as tall as its cards, but never shorter than a card that is
    // still leaving: the window clips, so collapsing to 1 px under the last toast
    // wiped its exit slide instead of letting it glide off the edge.
    implicitHeight: Math.max(1, list.contentHeight, exitFloor)
    // Height held for a leaving card, released once its exit has finished. This
    // must outlast the whole dismiss: the flicked card slides out over notifOut,
    // then the cards below wait that long and rise over durFastSpatial. Releasing
    // sooner would drop the surface under a card still rising and clip its foot.
    property real exitFloor: 0
    Timer {
        id: exitHold
        interval: Motion.notifOut + Tokens.durFastSpatial + 60
        onTriggered: win.exitFloor = 0
    }
    // Only follow the height once cards are already up: growing from nothing
    // wiped the first toast open top-down, which read as the card unrolling
    // rather than arriving. With cards present the ease still keeps a leaving
    // one from being clipped by the surface shrinking under it.
    Behavior on implicitHeight {
        enabled: cards.count > 1
        NumberAnimation { duration: Motion.rowReveal; easing.type: Motion.rowRevealCurve }
    }

    // Newest-first card view, reconciled from Notifs.popups by id.
    ListModel { id: cards }

    function indexOfId(id) {
        for (var i = 0; i < cards.count; i++)
            if (cards.get(i).nid === id)
                return i;
        return -1;
    }
    function sync() {
        var incoming = win.popups;
        // Drop cards whose notification is gone (this triggers the remove slide).
        for (var i = cards.count - 1; i >= 0; i--) {
            var id = cards.get(i).nid;
            var keep = false;
            for (var k = 0; k < incoming.length; k++)
                if (incoming[k].id === id) { keep = true; break; }
            if (!keep) {
                // Pin the surface at its current height for the exit's duration,
                // then let it settle; otherwise the last card leaves a 1 px
                // window that clips the slide it is in the middle of.
                win.exitFloor = Math.max(win.exitFloor, list.contentHeight);
                exitHold.restart();
                cards.remove(i);
            }
        }
        // Insert new cards and reorder to match the newest-first incoming order.
        for (var j = 0; j < incoming.length; j++) {
            var p = incoming[j];
            var cur = win.indexOfId(p.id);
            if (cur < 0)
                cards.insert(j, { nid: p.id, entry: p });
            else if (cur !== j)
                cards.move(cur, j, 1);
        }
    }

    // Map immediately on the first popup; unmap 260 ms after the list empties,
    // re-checking emptiness so a popup arriving during the wait cancels it.
    // Map before the first card lands. A ListView does not animate its initial
    // population, so syncing in the same frame the surface maps skipped the
    // arrival entirely and left the window resize as the only motion.
    onPopupsChanged: {
        if (popups.length > 0) {
            unmapTimer.stop();
            if (!mapped) {
                mapped = true;
                Qt.callLater(win.sync);
                return;
            }
        } else {
            unmapTimer.restart();
        }
        sync();
    }
    Component.onCompleted: {
        sync();
        mapped = popups.length > 0;
    }

    Timer {
        id: unmapTimer
        interval: Motion.notifHide
        onTriggered: if (win.popups.length === 0) win.mapped = false
    }

    ListView {
        id: list
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: contentHeight
        spacing: 14 * win.us
        interactive: false
        model: cards
        verticalLayoutDirection: win.position === "BottomRight" ? ListView.BottomToTop : ListView.TopToBottom

        // Physical dismiss and arrival (contract 12 sec 5, eye-candy pass): a
        // toast is flicked off the edge it lives on rather than fading in place,
        // and the stack settles up behind it instead of teleporting.
        //
        // add: the card arrives from off its own edge on the emphasized settle,
        // with a small scale settle so it reads as landing, not just sliding.
        // remove: the card slides out toward its edge on the overshoot spatial
        // curve while it shrinks and fades on the effects curve, all in
        // parallel, so it reads as flicked away instead of dissolving.
        // removeDisplaced: the cards below hold their slot until the flicked card
        // has cleared (a PauseAnimation the length of the exit), then rise on the
        // spatial curve, so the gap never collapses before the dismiss reads. It
        // also restores x/opacity/scale for a card caught mid-arrival, so an
        // interrupted card never sticks half-flicked. displaced does the same for
        // an arrival-time reorder.
        add: Transition {
            NumberAnimation { properties: "opacity"; from: 0; to: 1; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
            NumberAnimation { property: "x"; from: win.slideFrom; to: 0; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
            NumberAnimation { property: "scale"; from: 0.96; to: 1; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
        }
        displaced: Transition {
            NumberAnimation { properties: "y"; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
            NumberAnimation { properties: "opacity"; to: 1; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
            NumberAnimation { property: "x"; to: 0; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
            NumberAnimation { property: "scale"; to: 1; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
        }
        remove: Transition {
            NumberAnimation { property: "x"; to: win.slideFrom; duration: Motion.notifOut; easing.type: Motion.easeType; easing.bezierCurve: Tokens.curveDefaultSpatial }
            NumberAnimation { property: "scale"; to: 0.8; duration: Motion.notifOut; easing.type: Motion.easeType; easing.bezierCurve: Tokens.curveDefaultEffects }
            NumberAnimation { properties: "opacity"; to: 0; duration: Motion.notifOut; easing.type: Motion.easeType; easing.bezierCurve: Tokens.curveDefaultEffects }
        }
        removeDisplaced: Transition {
            SequentialAnimation {
                PauseAnimation { duration: Motion.notifOut }
                NumberAnimation { properties: "y"; duration: Tokens.durFastSpatial; easing.type: Motion.easeType; easing.bezierCurve: Tokens.curveDefaultSpatial }
            }
            NumberAnimation { properties: "opacity"; to: 1; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
            NumberAnimation { property: "x"; to: 0; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
            NumberAnimation { property: "scale"; to: 1; duration: Motion.notifIn; easing.type: Motion.easeType; easing.bezierCurve: Motion.notifInCurve }
        }

        delegate: Item {
            id: popupDelegate

            required property var entry
            readonly property int lifespanMs: {
                const ttl = Notifs.popupTtl(popupDelegate.entry);
                return ttl < 0 ? 0 : ttl;
            }
            readonly property bool storeRuns: Designs.notification !== Designs.shipped
                && Designs.notificationReady
                && win.notificationUrl !== ""
                && win.notificationUrl !== win.failedNotificationUrl
            // ListView keeps its first delegate measurement unless a later
            // Loader height is pushed through the delegate and force-laid out.
            property real shippedHeight: 0
            function syncShippedHeight() {
                const item = shippedLoader.item;
                popupDelegate.shippedHeight = item
                    ? Math.max(item.implicitHeight, item.targetImplicitHeight)
                    : 0;
            }

            width: ListView.view ? ListView.view.width : implicitWidth
            implicitHeight: storeLoader.visible && storeLoader.item
                ? storeLoader.item.implicitHeight
                : popupDelegate.shippedHeight
            height: implicitHeight
            onHeightChanged: if (ListView.view) ListView.view.forceLayout()
            transformOrigin: win.position === "Left" ? Item.TopLeft
                : win.position === "Center" ? Item.Top
                : win.position === "BottomRight" ? Item.BottomRight : Item.TopRight

            Loader {
                id: shippedLoader

                width: popupDelegate.width
                height: popupDelegate.shippedHeight
                active: !popupDelegate.storeRuns || storeLoader.status === Loader.Error
                visible: active
                onItemChanged: popupDelegate.syncShippedHeight()
                sourceComponent: Component {
                    NotificationCard {
                        width: popupDelegate.width
                        height: implicitHeight
                        notif: popupDelegate.entry
                        us: win.us
                        // Popups are compact and expand on demand; the grow/collapse scales
                        // about the anchored corner (top-right, top-left or top-centre).
                        compact: true
                        transformOrigin: popupDelegate.transformOrigin
                        // The popup drains its countdown frame over its own display lifespan;
                        // a persistent popup (ttl -1) passes 0 and draws no frame.
                        lifespanMs: popupDelegate.lifespanMs
                        // The popup has no menu to close, so it ignores actionInvoked.
                    }
                }
            }
            Connections {
                target: shippedLoader.item
                function onImplicitHeightChanged() { popupDelegate.syncShippedHeight(); }
                function onTargetImplicitHeightChanged() { popupDelegate.syncShippedHeight(); }
            }

            Loader {
                id: storeLoader

                readonly property string url: win.notificationUrl
                property string loadedUrl: ""
                property string loadedDesign: ""

                width: popupDelegate.width
                height: item ? item.implicitHeight : 0
                active: popupDelegate.storeRuns
                visible: active && status === Loader.Ready
                // Built in the same frame the card is inserted, so the column
                // lays it out at its real height. Loaded later, a taller card
                // grew after its neighbours had already been displaced and sat
                // flush against the one below.
                asynchronous: false

                function load() {
                    if (!storeLoader.active || storeLoader.url === ""
                            || storeLoader.loadedUrl === storeLoader.url)
                        return;
                    storeLoader.loadedUrl = storeLoader.url;
                    storeLoader.loadedDesign = Designs.notification;
                    storeLoader.setSource(storeLoader.url, {
                        notification: popupDelegate.entry,
                        us: win.us,
                        lifespanMs: popupDelegate.lifespanMs,
                        position: win.position,
                        showImages: Config.notifications.show_images === true
                    });
                }

                onActiveChanged: {
                    if (storeLoader.active)
                        storeLoader.load();
                    else
                        storeLoader.loadedUrl = "";
                }
                onUrlChanged: storeLoader.load()
                Component.onCompleted: storeLoader.load()
                onStatusChanged: {
                    if (storeLoader.status !== Loader.Error)
                        return;
                    const brokenUrl = storeLoader.loadedUrl;
                    if (brokenUrl === "" || brokenUrl !== win.notificationUrl
                            || brokenUrl === win.failedNotificationUrl)
                        return;
                    console.warn("notifications: the store card " + storeLoader.loadedDesign
                        + " did not load; drawing the Ryoku card");
                    win.failedNotificationUrl = brokenUrl;
                }

                Binding {
                    when: storeLoader.item !== null
                    target: storeLoader.item
                    property: "us"
                    value: win.us
                }
                Binding {
                    when: storeLoader.item !== null
                    target: storeLoader.item
                    property: "lifespanMs"
                    value: popupDelegate.lifespanMs
                }
                Binding {
                    when: storeLoader.item !== null
                    target: storeLoader.item
                    property: "position"
                    value: win.position
                }
                Binding {
                    when: storeLoader.item !== null
                    target: storeLoader.item
                    property: "showImages"
                    value: Config.notifications.show_images === true
                }
            }
        }
    }
}
