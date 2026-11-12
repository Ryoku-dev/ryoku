pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../../kit"
import ".." as Q
import "../../settings" as S
import "../Catalogue.js" as Catalogue

Column {
    id: page

    property var host

    spacing: 12

    readonly property string focusedName: Wm.focusedOutput
    readonly property string ddcBus: {
        var mons = Devices.ddcMonitors || [];
        for (var i = 0; i < mons.length; i++)
            if (mons[i] && mons[i].label === page.focusedName) return mons[i].bus;
        return mons.length > 0 && mons[0] ? String(mons[0].bus || "") : "";
    }
    readonly property bool brightnessAvailable: Devices.backlightAvailable || page.ddcBus !== ""
    readonly property var ddcCache: page.host ? (page.host.ddcBrightness || {}) : {}
    readonly property real brightness: Devices.backlightAvailable ? Devices.backlightPct / 100
        : page.ddcCache[page.ddcBus] !== undefined ? page.ddcCache[page.ddcBus] / 100 : 0.5
    function setBrightness(v, final) {
        var pct = Math.max(1, Math.round(v * 100));
        if (Devices.backlightAvailable) {
            Devices.setBacklight(pct);
            return;
        }
        // every ddc write is a slow i2c round trip, so an external screen only gets where the drag lands
        if (!final || page.ddcBus === "")
            return;
        Devices.setBrightness(page.ddcBus, pct);
        var next = Object.assign({}, page.ddcCache);
        next[page.ddcBus] = pct;
        page.host.ddcBrightness = next;
    }

    readonly property var sinkAudio: (Audio.sink && Audio.sink.audio) ? Audio.sink.audio : null
    function setVolume(v) {
        if (!page.sinkAudio) return;
        if (page.sinkAudio.muted && v > 0) page.sinkAudio.muted = false;
        page.sinkAudio.volume = Math.max(0, Math.min(1, v));
    }
    function volumeIcon() {
        if (!page.sinkAudio || page.sinkAudio.muted || page.sinkAudio.volume <= 0.001) return "volume_off";
        return page.sinkAudio.volume < 0.5 ? "volume_down" : "volume_up";
    }

    readonly property var tiles: (cfg.tiles || []).filter(function (t) { return Catalogue.ids.indexOf(t.id) >= 0; })

    S.IslandSettings { id: cfg }

    Q.Header {
        width: parent.width
        pane: page.host
    }

    Row {
        width: parent.width
        spacing: 8

        Slider {
            width: parent.width - brightnessMore.width - parent.spacing
            enabled: page.brightnessAvailable
            icon: "brightness_6"
            value: page.brightness
            onMoved: (v) => page.setBrightness(v, false)
            onCommitted: (v) => page.setBrightness(v, true)
        }
        IconButton {
            id: brightnessMore
            size: 48
            variant: "tonal"
            icon: "chevron_right"
            tip: I18n.tr("Display")
            onClicked: page.host.go("display")
        }
    }

    Row {
        width: parent.width
        spacing: 8

        Slider {
            width: parent.width - soundMore.width - parent.spacing
            enabled: page.sinkAudio !== null
            icon: page.volumeIcon()
            iconClickable: true
            muted: page.sinkAudio ? page.sinkAudio.muted : false
            value: page.sinkAudio ? page.sinkAudio.volume : 0
            onMoved: (v) => page.setVolume(v)
            onIconClicked: if (page.sinkAudio) page.sinkAudio.muted = !page.sinkAudio.muted
        }
        IconButton {
            id: soundMore
            size: 48
            variant: "tonal"
            icon: "chevron_right"
            tip: I18n.tr("Sound")
            onClicked: page.host.go("sound")
        }
    }

    Q.TileGrid {
        id: grid
        width: parent.width
        tiles: page.tiles
        pane: page.host
    }

    Item {
        width: parent.width
        height: 40

        IconButton {
            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
            icon: "edit"
            tip: I18n.tr("Edit tiles")
            onClicked: page.host.go("edit")
        }
        Dots {
            anchors.centerIn: parent
            count: grid.pageCount
            current: grid.currentPage
            onPicked: (i) => grid.show(i)
        }
        IconButton {
            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
            icon: "tune"
            tip: I18n.tr("Island settings")
            onClicked: page.host.openIslandSettings()
        }
    }

    Q.MediaCard {
        width: parent.width
        visible: Media.present
    }

    Q.NotificationList {
        width: parent.width
        pane: page.host
    }

    Item { width: 1; height: 4 }
}
