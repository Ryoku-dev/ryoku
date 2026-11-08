pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

// QML view of the daemon `tray` topic. The StatusNotifier host lives in
// ryoku-shell (tray.go), so QML never speaks D-Bus itself: the bus streams a
// full {items:[...]} frame on every change, and click intents ride back as bus
// calls. The icon precedence chain (item themed path,
// then theme name, then the nearest-24px ARGB->RGBA pixmap, then the generic
// executable fallback) is resolved server-side, so each item already carries a
// ready `iconPath` (a file) or an `iconName` (a theme name). Contract 04 sec 3.2
// (system_tray, system_tray_item).
Singleton {
    id: root

    property var items: []

    function apply(frame) {
        root.items = frame && Array.isArray(frame.items) ? frame.items : [];
    }

    // Click intents. Left click asks the item to act (SNI Activate). Right click
    // shows the item's menu: the shell renders the item's own dbusmenu tree (the
    // published `menu`) in-shell and drives it through menuEvent, because the SNI
    // ContextMenu method asks the app to raise its menu at a screen point, which a
    // Wayland client cannot honour (it lands anywhere, or the app has no menu and
    // raises its window instead). contextMenu stays only as the fallback for an
    // item that exposes no dbusmenu. aboutToShow lets the item populate a lazily
    // built menu before it is drawn; menuEvent clicks a menu entry by id.
    function activate(service, x, y) { root.send("tray.activate", { service: service, x: x, y: y }); }
    function contextMenu(service, x, y) { root.send("tray.contextMenu", { service: service, x: x, y: y }); }
    function scroll(service, delta, orientation) { root.send("tray.scroll", { service: service, delta: delta, orientation: orientation }); }
    function aboutToShow(service) { root.send("tray.aboutToShow", { service: service }); }
    function menuEvent(service, id) { root.send("tray.menuEvent", { service: service, item: id }); }

    function send(method, args) {
        Bus.call(method, args);
    }

    BusTopic {
        id: topicFeed
        topic: "tray"
        onUpdated: root.apply(topicFeed.data)
    }

    Connections {
        target: Bus
        function onConnectedChanged() {
            if (!Bus.connected)
                root.items = [];
        }
    }
}
