pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../kit"
import "../quicksettings" as Q
import "../quicksettings/Catalogue.js" as Catalogue
import "../IslandMetrics.js" as M
import "Sky.js" as Sky

// the island's own clock text sits over this as the shared element, so the glance leaves room for it
Item {
    id: glance

    property real reveal: 0
    property real uiScale: 1
    property var tiles: []
    property bool showLocation: true
    property bool showWeather: true
    property bool showBattery: true
    property var host: null
    signal expandRequested()
    signal settingsRequested()

    readonly property real pad: M.glancePad * glance.uiScale
    readonly property real tile: M.glanceTile * glance.uiScale
    readonly property var compact: glance.tiles.filter(function (id) { return Catalogue.ids.indexOf(id) >= 0; }).slice(0, 5)

    readonly property int code: Weather.current ? (Number(Weather.current.code) | 0) : -1
    readonly property string place: Weather.city || Weather.location
    readonly property string weatherLine: {
        if (!Weather.available)
            return Weather.status === "loading" ? I18n.tr("Getting the weather") : I18n.tr("Weather unavailable");
        if (glance.showLocation && glance.place.length > 0)
            return I18n.tr("%1 in %2").replace("%1", Weather.condition).replace("%2", glance.place);
        return Weather.condition;
    }

    Clock { id: clock }

    visible: glance.reveal > 0.01
    opacity: Math.max(0, Math.min(1, glance.reveal))

    Row {
        x: glance.pad
        y: (M.glanceClockY + 54) * glance.uiScale
        spacing: 12

        Label {
            anchors.verticalCenter: parent.verticalCenter
            role: "title"
            color: Md.inkMuted
            text: clock.now.toLocaleDateString(Config.formatLoc, "ddd d MMMM")
        }
        Row {
            anchors.verticalCenter: parent.verticalCenter
            visible: Battery.present && glance.showBattery
            spacing: 2

            Icon {
                anchors.verticalCenter: parent.verticalCenter
                size: 16
                filled: true
                color: Battery.low ? Md.error : Md.inkMuted
                name: Battery.charging ? "battery_charging_full"
                    : Battery.pct >= 95 ? "battery_full"
                    : "battery_" + Math.max(0, Math.min(6, Math.floor(Battery.pct / 14.3))) + "_bar"
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                role: "label"
                color: Battery.low ? Md.error : Md.inkMuted
                text: Battery.pct + "%"
            }
        }
    }

    Column {
        visible: glance.showWeather
        anchors { right: parent.right; rightMargin: glance.pad }
        y: (M.glanceClockY + 6) * glance.uiScale
        spacing: 2

        Row {
            anchors.right: parent.right
            spacing: 8

            Icon {
                anchors.verticalCenter: parent.verticalCenter
                name: Sky.symbol(glance.code, Weather.isDay)
                size: 32
                filled: true
                color: Md.accent
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: Weather.available ? Math.round(Weather.tempNow) + "\u00B0" : "--\u00B0"
                color: Md.ink
                font.family: Md.font
                font.pixelSize: 30 * glance.uiScale
                font.weight: Font.Medium
            }
        }
        Label {
            anchors.right: parent.right
            width: Math.min(implicitWidth, 200 * glance.uiScale)
            horizontalAlignment: Text.AlignRight
            role: "caption"
            color: Md.inkMuted
            text: glance.weatherLine
        }
    }

    Row {
        x: glance.pad
        y: M.glanceTilesY * glance.uiScale
        spacing: 8

        Repeater {
            model: glance.compact
            delegate: Item {
                id: chip
                required property var modelData
                width: glance.tile
                height: glance.tile

                Q.TileSpec { id: spec; tileId: chip.modelData; pane: glance.host }

                Accessible.role: Accessible.Button
                Accessible.name: spec.label
                Accessible.checkable: true
                Accessible.checked: spec.active
                opacity: spec.available ? 1 : 0.38
                enabled: spec.available

                // android's small tiles: a circle when off, a squircle filled with the accent when on
                Rectangle {
                    id: body
                    anchors.fill: parent
                    radius: hit.pressed ? Md.rMd : (spec.active ? Md.rLg : width / 2)
                    color: spec.active ? Md.accent : Md.containerHigh
                    Behavior on radius {
                        enabled: !Md.still
                        Spring { speed: "fast" }
                    }
                    Behavior on color {
                        enabled: !Md.still
                        ColorAnimation { duration: Md.fast }
                    }
                }
                Icon {
                    anchors.centerIn: parent
                    name: spec.icon
                    size: 22
                    filled: spec.active
                    color: spec.active ? Md.onAccent : Md.ink
                }
                StateLayer {
                    id: hit
                    radius: body.radius
                    tint: spec.active ? Md.onAccent : Md.ink
                    onActivated: spec.toggle()
                    onSecondary: spec.open()
                }
            }
        }
    }

    Row {
        anchors { right: parent.right; rightMargin: glance.pad - 4 }
        y: M.glanceTilesY * glance.uiScale
        height: glance.tile
        spacing: 4

        IconButton {
            anchors.verticalCenter: parent.verticalCenter
            icon: "settings"
            tip: I18n.tr("Island settings")
            onClicked: glance.settingsRequested()
        }
        IconButton {
            anchors.verticalCenter: parent.verticalCenter
            size: glance.tile
            variant: "tonal"
            icon: "expand_more"
            tip: I18n.tr("Quick settings")
            onClicked: glance.expandRequested()
        }
    }
}
