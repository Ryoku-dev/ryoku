pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../components" as C
import "../settings" as S
import "../kit"
import "../components/Sky.js" as Sky

Item {
    id: header

    property var pane

    implicitHeight: 64

    C.Clock { id: clock }
    S.IslandSettings { id: cfg }

    Column {
        anchors { left: parent.left; leftMargin: 4; right: rightCluster.left; rightMargin: 8; verticalCenter: parent.verticalCenter }
        spacing: 0

        Text {
            text: clock.time
            color: Md.ink
            font.family: Md.mono
            font.pixelSize: 32
            font.weight: Font.Medium
        }
        Row {
            width: parent.width
            spacing: 10

            Label {
                id: dateLabel
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - (weather.visible ? weather.width + parent.spacing : 0))
                role: "label"
                color: Md.inkMuted
                text: clock.now.toLocaleDateString(Config.formatLoc, "ddd d MMMM")
            }
            Row {
                id: weather
                anchors.verticalCenter: parent.verticalCenter
                visible: Weather.available
                spacing: 4

                Icon {
                    anchors.verticalCenter: parent.verticalCenter
                    name: Sky.symbol(Weather.current ? (Number(Weather.current.code) | 0) : -1, Weather.isDay)
                    size: 16
                    filled: true
                    color: Md.inkMuted
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    role: "label"
                    color: Md.inkMuted
                    text: Math.round(Weather.tempNow) + "\u00B0"
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    visible: cfg.showLocation && text.length > 0
                    width: Math.min(implicitWidth, 110)
                    role: "label"
                    color: Md.inkMuted
                    text: Weather.city || Weather.location
                }
            }
        }
    }

    Row {
        id: rightCluster
        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
        spacing: 4

        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            visible: Battery.present
            width: batteryRow.implicitWidth + 20
            height: 32
            radius: 16
            color: Battery.low ? Md.error : Md.containerHigh

            Row {
                id: batteryRow
                anchors.centerIn: parent
                spacing: 4

                Icon {
                    anchors.verticalCenter: parent.verticalCenter
                    size: 18
                    filled: true
                    color: Battery.low ? Md.onError : Md.ink
                    name: Battery.charging ? "battery_charging_full"
                        : Battery.pct >= 95 ? "battery_full"
                        : "battery_" + Math.max(0, Math.min(6, Math.floor(Battery.pct / 14.3))) + "_bar"
                }
                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    role: "label"
                    color: Battery.low ? Md.onError : Md.ink
                    text: Battery.pct + "%"
                }
            }
        }

        Item { width: 4; height: 1 }

        IconButton {
            anchors.verticalCenter: parent.verticalCenter
            icon: "settings"
            tip: I18n.tr("Ryoku Settings")
            onClicked: {
                header.pane.close();
                Bus.run("hub open");
            }
        }
        IconButton {
            anchors.verticalCenter: parent.verticalCenter
            icon: "power_settings_new"
            tip: I18n.tr("Power menu")
            onClicked: header.pane.go("power")
        }
    }
}
