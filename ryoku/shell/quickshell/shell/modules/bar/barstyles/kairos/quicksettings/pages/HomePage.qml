pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Bluetooth
import shell.services
import shell.barkit as Pill
import Ryoku.Ui.Singletons
import "../kit" as K

// The quick-settings home: the two radio tiles, the weather card, the two
// fader rows and the notification list.
Column {
    id: page

    property var host

    spacing: 14

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    readonly property color accent: Theme.primary
    function dim(a) { return Qt.rgba(page.ink.r, page.ink.g, page.ink.b, a) }

    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property string wifiSubtitle: {
        if (!Network.wifiPresent) return I18n.tr("Not available");
        if (!Network.wifiRadio) return I18n.tr("Off");
        if (Network.wifiConnectivity === "Connected") return Network.activeSsid;
        return I18n.tr("Not connected");
    }
    readonly property string btSubtitle: {
        if (BluetoothPower.blocked) return I18n.tr("Blocked");
        if (!page.adapter) return I18n.tr("Not available");
        if (!page.adapter.enabled) return I18n.tr("Off");
        var devs = Bluetooth.devices ? Bluetooth.devices.values : [];
        for (var i = 0; i < devs.length; i++) {
            var d = devs[i];
            if (!d || !d.connected) continue;
            var name = BtLink.label(d);
            return (name && name !== String(d.address || "")) ? name : I18n.tr("Connected");
        }
        return I18n.tr("Not connected");
    }

    readonly property string focusedName: Wm.focusedOutput
    readonly property string ddcBus: {
        var mons = Devices.ddcMonitors || [];
        for (var i = 0; i < mons.length; i++)
            if (mons[i] && mons[i].label === page.focusedName) return mons[i].bus;
        return "";
    }
    readonly property real brightness: Devices.backlightAvailable ? Devices.backlightPct / 100 : 0.5
    function setBrightness(v) {
        var pct = Math.round(v * 100);
        if (Devices.backlightAvailable) Devices.setBacklight(pct);
        else if (page.ddcBus !== "") Devices.setBrightness(page.ddcBus, pct);
    }

    readonly property real volume: (Audio.sink && Audio.sink.audio) ? Audio.sink.audio.volume : 0
    function setVolume(v) {
        var s = Audio.sink;
        if (!s || !s.audio) return;
        if (s.audio.muted && v > 0) s.audio.muted = false;
        s.audio.volume = Math.max(0, Math.min(1, v));
    }

    // Weather: the full WMO set, day/night split, plus a mood tint (night cool
    // and dark, a hot day warm, a cold day blue, otherwise the wallpaper accent).
    readonly property int wxCode: (Weather.available && Weather.current)
        ? (Number(Weather.current.code) | 0) : -1
    readonly property bool wxDay: !Weather.available || Weather.isDay
    readonly property bool wxNight: Weather.available && !Weather.isDay
    readonly property real wxTempC: {
        var t = Number(Weather.tempNow);
        return String(Config.weatherUnit) === "fahrenheit" ? (t - 32) * 5 / 9 : t;
    }
    readonly property bool wxHot: Weather.available && page.wxDay && page.wxTempC >= 30
    readonly property bool wxCold: Weather.available && page.wxDay && page.wxTempC <= 5
    readonly property bool wxLocation: !(Config.kairos && Config.kairos.showLocation === false)

    function wxGlyph(code, day) {
        if (code < 0) return "wx-unknown";
        if (code === 0) return day ? "wx-clear-day" : "wx-clear-night";
        if (code === 1) return day ? "wx-partly-cloudy-day" : "wx-partly-cloudy-night";
        if (code === 2) return "wx-cloudy";
        if (code === 3) return "wx-overcast";
        if (code === 45) return "wx-mist";
        if (code === 48) return "wx-fog";
        if (code === 51 || code === 53 || code === 55) return "wx-drizzle";
        if (code === 56 || code === 57 || code === 66 || code === 67) return "wx-sleet";
        if (code === 61 || code === 80) return "wx-rain-light";
        if (code === 63 || code === 81) return "wx-rain";
        if (code === 65 || code === 82) return "wx-rain-heavy";
        if (code === 71 || code === 77 || code === 85) return "wx-snow-light";
        if (code === 73 || code === 86) return "wx-snow";
        if (code === 75) return "wx-snow-heavy";
        if (code === 95) return "wx-thunderstorm";
        if (code === 96 || code === 99) return "wx-hail";
        return "wx-cloudy";
    }

    readonly property color wxBase: page.wxNight ? "#161a2c"
        : page.wxHot ? "#3a1710"
        : page.wxCold ? "#122430"
        : Qt.darker(page.accent, 1.7)
    readonly property color wxTop: page.wxNight ? Qt.rgba(0.22, 0.27, 0.45, 0.85)
        : page.wxHot ? Qt.rgba(0.96, 0.47, 0.20, 0.55)
        : page.wxCold ? Qt.rgba(0.30, 0.62, 0.86, 0.45)
        : Qt.rgba(page.accent.r, page.accent.g, page.accent.b, 0.42)
    readonly property color wxBottom: page.wxNight ? Qt.rgba(0, 0, 0, 0.55)
        : page.wxHot ? Qt.rgba(0.22, 0.05, 0, 0.5)
        : page.wxCold ? Qt.rgba(0, 0.05, 0.12, 0.5)
        : Qt.rgba(0, 0, 0, 0.36)


    Row {
        width: parent.width
        spacing: 12

        Column {
            width: (page.width - 12) / 2
            spacing: 12

            K.QsTile {
                width: parent.width
                height: 70
                glyph: "wifi"
                title: I18n.tr("Wi-Fi")
                subtitle: page.wifiSubtitle
                onClicked: page.host.go("wifi")
            }
            K.QsTile {
                width: parent.width
                height: 70
                glyph: "bluetooth"
                title: I18n.tr("Bluetooth")
                subtitle: page.btSubtitle
                onClicked: page.host.go("bluetooth")
            }
        }

        Rectangle {
            id: weatherCard
            width: (page.width - 12) / 2
            height: 152
            radius: 20
            clip: true
            color: page.wxBase

            Rectangle {
                anchors.fill: parent
                radius: weatherCard.radius
                gradient: Gradient {
                    GradientStop { position: 0.0; color: page.wxTop }
                    GradientStop { position: 1.0; color: page.wxBottom }
                }
            }

            Pill.GlyphIcon {
                anchors { right: parent.right; rightMargin: -12; bottom: parent.bottom; bottomMargin: -16 }
                width: 108
                height: 108
                name: page.wxGlyph(page.wxCode, page.wxDay)
                color: Qt.rgba(page.ink.r, page.ink.g, page.ink.b, page.wxNight ? 0.20 : 0.16)
            }

            Column {
                anchors { left: parent.left; right: parent.right; top: parent.top }
                anchors.margins: 14
                spacing: 2

                Text {
                    width: parent.width
                    visible: page.wxLocation
                    text: Weather.available ? (Weather.city || Weather.location) : I18n.tr("Weather")
                    color: page.dim(0.8)
                    font.family: Theme.fontPrimary
                    font.pixelSize: 11
                    elide: Text.ElideRight
                }
                Text {
                    text: Weather.available ? (Math.round(Weather.tempNow) + "\u00B0") : "--\u00B0"
                    color: page.ink
                    font.family: Theme.mono
                    font.pixelSize: 30
                    font.weight: Font.DemiBold
                }
                Text {
                    width: parent.width
                    text: Weather.available ? Weather.condition : I18n.tr("No data")
                    color: page.dim(0.85)
                    font.family: Theme.fontPrimary
                    font.pixelSize: 12
                    elide: Text.ElideRight
                }
            }
        }
    }

    Rectangle {
        id: displayRow
        width: parent.width
        height: 78
        radius: 20
        color: page.dim(0.06)

        Item {
            id: displayHead
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 40
            Text {
                anchors { left: parent.left; leftMargin: 14; verticalCenter: parent.verticalCenter }
                text: I18n.tr("Display")
                color: page.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 14
                font.weight: Font.DemiBold
            }
            Pill.MaterialIcon {
                anchors { right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
                text: "chevron_right"
                color: page.dim(0.5)
                font.pixelSize: 20
            }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: page.host.go("display")
            }
        }

        K.QsFader {
            anchors { left: parent.left; leftMargin: 14; right: parent.right; rightMargin: 14; bottom: parent.bottom; bottomMargin: 4 }
            icon: "light_mode"
            value: page.brightness
            onMoved: (v) => page.setBrightness(v)
        }
    }

    Rectangle {
        id: soundRow
        width: parent.width
        height: 78
        radius: 20
        color: page.dim(0.06)

        Item {
            id: soundHead
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 40
            Text {
                anchors { left: parent.left; leftMargin: 14; verticalCenter: parent.verticalCenter }
                text: I18n.tr("Sound")
                color: page.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 14
                font.weight: Font.DemiBold
            }
            Pill.MaterialIcon {
                anchors { right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
                text: "chevron_right"
                color: page.dim(0.5)
                font.pixelSize: 20
            }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: page.host.go("sound")
            }
        }

        K.QsFader {
            anchors { left: parent.left; leftMargin: 14; right: parent.right; rightMargin: 14; bottom: parent.bottom; bottomMargin: 4 }
            icon: "volume_up"
            value: page.volume
            onMoved: (v) => page.setVolume(v)
        }
    }

    Rectangle {
        id: notifCard
        width: parent.width
        height: notifColumn.implicitHeight + 28
        radius: 20
        color: page.dim(0.06)

        Column {
            id: notifColumn
            anchors { left: parent.left; right: parent.right; top: parent.top }
            anchors.margins: 14
            spacing: 8

            Row {
                width: parent.width
                spacing: 8
                Text {
                    id: notifTitle
                    text: I18n.tr("Notifications")
                    color: page.dim(0.6)
                    font.family: Theme.fontPrimary
                    font.pixelSize: 12
                }
                Item {
                    width: Math.max(0, parent.width - notifTitle.width - clearItem.width - 16)
                    height: 1
                }
                Item {
                    id: clearItem
                    visible: Notifs.history.length > 0
                    width: clearLabel.implicitWidth
                    height: clearLabel.implicitHeight
                    Text {
                        id: clearLabel
                        anchors.fill: parent
                        text: I18n.tr("Clear all")
                        color: page.accent
                        font.family: Theme.fontPrimary
                        font.pixelSize: 12
                    }
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: Notifs.clearAll()
                    }
                }
            }

            Text {
                visible: Notifs.history.length === 0
                text: I18n.tr("No notifications")
                color: page.dim(0.5)
                font.family: Theme.fontPrimary
                font.pixelSize: 13
            }

            Repeater {
                model: Math.min(3, Notifs.history.length)
                delegate: Row {
                    id: notifRow
                    required property int index
                    width: parent.width
                    spacing: 10

                    Pill.MaterialIcon {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "notifications"
                        color: page.dim(0.55)
                        font.pixelSize: 17
                    }
                    Column {
                        width: parent.width - 27
                        spacing: 1
                        Text {
                            width: parent.width
                            text: Notifs.history[notifRow.index]
                                ? (Notifs.history[notifRow.index].summary || Notifs.history[notifRow.index].appName || "") : ""
                            color: page.ink
                            font.family: Theme.fontPrimary
                            font.pixelSize: 12
                            elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            visible: text.length > 0
                            text: Notifs.history[notifRow.index] ? (Notifs.history[notifRow.index].body || "") : ""
                            color: page.dim(0.55)
                            font.family: Theme.fontPrimary
                            font.pixelSize: 11
                            elide: Text.ElideRight
                        }
                    }
                }
            }
        }
    }
}
