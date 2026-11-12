pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons
import "../kit"
import "." as S

// android's large-screen settings: categories with search on the left, the chosen page on the right
PanelWindow {
    id: root

    required property var targetScreen
    property bool open: false
    signal closeRequested()

    property string route: "island"
    property string routeKey: ""
    property bool confirmReset: false
    // the card only starts to appear once the window has put out its first frame, so the opening never
    // spends its first steps waiting on the compositor
    property bool shown: false

    function mix(a, b, amount) { return Theme.blend(Md.alpha(a, amount), b) }
    function swatch(c) { return Theme.inkOn(Md.base, c, 7) }

    readonly property bool stockTiles: JSON.stringify(island.tiles) === JSON.stringify(island.defaultTiles)
    readonly property var categories: [
        { id: "island", group: 0, desc: I18n.tr("How big the clock island is, where it floats and what it shows."), icon: "crop_16_9", title: I18n.tr("Island"), hue: root.swatch(Theme.primary),
          summary: I18n.tr("%1%, %2 px from the top").arg(Math.round(island.scale * 100)).arg(Math.round(island.topGap)) },
        { id: "clock", group: 0, desc: I18n.tr("The time the island reads."), icon: "schedule", title: I18n.tr("Clock"), hue: root.swatch(Theme.secondary),
          summary: I18n.tr("%1, %2").arg(island.clock12h ? I18n.tr("12-hour") : I18n.tr("24-hour"))
              .arg(island.clockSeconds ? I18n.tr("seconds on") : I18n.tr("seconds off")) },
        { id: "music", group: 0, desc: I18n.tr("The now-playing bubble that grows beside the island."), icon: "music_note", title: I18n.tr("Music"), hue: root.swatch(Theme.tertiary),
          summary: island.music ? I18n.tr("On, beside the clock") : I18n.tr("Off") },
        { id: "weather", group: 0, desc: I18n.tr("The weather quick settings shows, and where it is for."), icon: "partly_cloudy_day", title: I18n.tr("Weather"),
          hue: root.swatch(root.mix(Theme.primary, Theme.tertiary, 0.5)),
          summary: I18n.tr("%1, %2").arg(Config.weatherUnit === "fahrenheit" ? "\u00B0F" : "\u00B0C")
              .arg(Config.weatherLocation.length > 0 ? Config.weatherLocation : I18n.tr("automatic location")) },
        { id: "tiles", group: 1, desc: I18n.tr("The tiles at the top of quick settings."), icon: "grid_view", title: I18n.tr("Quick settings"),
          hue: root.swatch(root.mix(Theme.secondary, Theme.tertiary, 0.5)),
          summary: root.stockTiles ? I18n.tr("Default tiles") : I18n.tr("%1 tiles, arranged by you").arg(island.tiles.length) },
        { id: "reset", group: 2, icon: "restart_alt", title: I18n.tr("Reset island settings"), hue: Md.error,
          summary: I18n.tr("Put the island back the way it shipped"), desc: "" }
    ]
    readonly property var entries: [
        { page: "island", key: "scale", title: I18n.tr("Island size"), words: "scale bigger smaller zoom" },
        { page: "island", key: "topGap", title: I18n.tr("Top offset"), words: "position gap margin height edge" },
        { page: "island", key: "islandOsd", title: I18n.tr("Volume and brightness"), words: "osd level sound show in the island" },
        { page: "island", key: "islandNotifications", title: I18n.tr("Show notifications in the island"), words: "banner heads-up alerts popup" },
        { page: "island", key: "tray", title: I18n.tr("App tray"), words: "system tray caret icons" },
        { page: "clock", key: "clock12h", title: I18n.tr("12-hour clock"), words: "24 hour time format am pm" },
        { page: "clock", key: "clockSeconds", title: I18n.tr("Show seconds"), words: "time tick" },
        { page: "music", key: "music", title: I18n.tr("Music island"), words: "now playing media player song" },
        { page: "weather", key: "unit", title: I18n.tr("Temperature unit"), words: "celsius fahrenheit degrees" },
        { page: "weather", key: "showLocation", title: I18n.tr("Show location"), words: "place city name" },
        { page: "weather", key: "location", title: I18n.tr("Weather location"), words: "automatic city search place network" },
        { page: "tiles", key: "editTiles", title: I18n.tr("Edit tiles"), words: "add remove move resize pencil shade" },
        { page: "tiles", key: "resetTiles", title: I18n.tr("Reset tiles"), words: "default layout shade" },
        { page: "reset", key: "", title: I18n.tr("Reset island settings"), words: "default restore" }
    ]
    function category(id) {
        for (var i = 0; i < root.categories.length; i++)
            if (root.categories[i].id === id)
                return root.categories[i];
        return root.categories[0];
    }
    function paneOf(id) {
        return id === "clock" ? clockPane : id === "music" ? musicPane : id === "weather" ? weatherPane
            : id === "tiles" ? tilesPane : islandPane;
    }

    function go(id, key) {
        if (id === "reset") {
            root.confirmReset = true;
            card.forceActiveFocus();
            return;
        }
        root.route = id;
        root.routeKey = key;
        var pane = root.paneOf(id);
        Qt.callLater(function () { pane.reveal(key); });
    }
    function dismiss() {
        if (root.confirmReset)
            root.confirmReset = false;
        else if (nav.query.length > 0)
            nav.query = "";
        else
            root.closeRequested();
        card.forceActiveFocus();
    }

    onOpenChanged: {
        root.confirmReset = false;
        nav.query = "";
        root.routeKey = "";
        if (!root.open || Md.still)
            root.shown = root.open;
    }

    S.IslandSettings { id: island }

    screen: root.targetScreen
    visible: root.open || card.opacity > 0.001
    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-kairos-settings"
    WlrLayershell.keyboardFocus: root.open ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    Connections {
        target: card.Window.window
        enabled: root.open && !root.shown
        function onFrameSwapped() { root.shown = true; }
    }

    MouseArea {
        anchors.fill: parent
        enabled: root.open
        acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
        onPressed: root.closeRequested()
    }

    RectangularShadow {
        anchors.fill: card
        radius: card.radius
        blur: 44
        spread: 2
        offset: Qt.vector2d(0, 14)
        color: Qt.rgba(0, 0, 0, 0.62)
        opacity: card.opacity
        scale: card.scale
    }

    Rectangle {
        id: card
        anchors.centerIn: parent
        width: Math.min(880, root.width - 48)
        height: Math.min(640, root.height - 96)
        radius: Md.rXxl
        color: Md.base
        border.width: 1
        border.color: Md.outlineSoft
        clip: true

        // only opacity and scale move, on one texture while they do, so no text is laid out or drawn per frame
        opacity: root.shown ? 1 : 0
        scale: root.shown ? 1 : 0.94
        layer.enabled: card.opacity < 0.999 || Math.abs(card.scale - 1) > 0.001
        layer.smooth: true
        Behavior on opacity {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
        Behavior on scale {
            enabled: !Md.still
            Spring {}
        }

        focus: root.open
        Keys.onPressed: function (e) {
            if (e.key === Qt.Key_Escape) {
                root.dismiss();
                e.accepted = true;
            } else if (!root.confirmReset && e.text.length === 1 && e.text >= " " && !(e.modifiers & Qt.ControlModifier)) {
                nav.searchField.forceActiveFocus();
                nav.query += e.text;
                e.accepted = true;
            }
        }
        MouseArea { anchors.fill: parent }

        S.SettingsNav {
            id: nav
            anchors { top: parent.top; bottom: parent.bottom; left: parent.left; margins: 16 }
            width: 332
            categories: root.categories
            entries: root.entries
            route: root.route
            routeKey: root.routeKey
            onPicked: function (page, key) { root.go(page, key); }
            onSearchLeft: root.dismiss()
        }

        Item {
            id: detail
            anchors { top: parent.top; bottom: parent.bottom; left: nav.right; right: parent.right; leftMargin: 16; rightMargin: 20 }

            S.DetailPane {
                id: islandPane
                anchors.fill: parent
                current: root.route === "island"
                title: root.category("island").title
                description: root.category("island").desc
                S.IslandPage { width: parent.width; cfg: island }
            }
            S.DetailPane {
                id: clockPane
                anchors.fill: parent
                current: root.route === "clock"
                title: root.category("clock").title
                description: root.category("clock").desc
                S.ClockPage { width: parent.width; cfg: island; live: root.open }
            }
            S.DetailPane {
                id: musicPane
                anchors.fill: parent
                current: root.route === "music"
                title: root.category("music").title
                description: root.category("music").desc
                S.MusicPage { width: parent.width; cfg: island }
            }
            S.DetailPane {
                id: weatherPane
                anchors.fill: parent
                current: root.route === "weather"
                title: root.category("weather").title
                description: root.category("weather").desc
                S.WeatherPage {
                    width: parent.width
                    cfg: island
                    onFieldLeft: card.forceActiveFocus()
                }
            }
            S.DetailPane {
                id: tilesPane
                anchors.fill: parent
                current: root.route === "tiles"
                title: root.category("tiles").title
                description: root.category("tiles").desc
                S.TilesPage { width: parent.width; cfg: island }
            }
        }

        S.ResetDialog {
            anchors.fill: parent
            open: root.confirmReset
            onCancelled: root.confirmReset = false
            onConfirmed: {
                island.reset();
                root.confirmReset = false;
            }
        }
    }
}
