pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services
import Ryoku.Ui.Singletons
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene

    readonly property date now: clock.date
    readonly property bool showClock: !S.Prefs.hasModule("clock")
    readonly property bool showWeather: S.Prefs.voiceWeather && Weather.available
        && !S.Prefs.hasModule("weather")
    readonly property var currentWeather: Weather.current

    implicitWidth: content.implicitWidth
    implicitHeight: K.Style.barH

    function weatherGlyph(code, isDay) {
        const value = Number(code);
        if (value === 0)
            return isDay ? "sunny" : "clear_night";
        if (value === 1 || value === 2)
            return "partly_cloudy_day";
        if (value === 3)
            return "cloud";
        if (value >= 51 && value <= 67)
            return "rainy";
        if (value >= 71 && value <= 77)
            return "weather_snowy";
        if (value >= 80 && value <= 82)
            return "rainy";
        if (value >= 95)
            return "thunderstorm";
        return "cloud";
    }

    Row {
        id: content
        anchors.centerIn: parent
        spacing: K.Style.s2

        Column {
            visible: root.showClock
            anchors.verticalCenter: parent.verticalCenter
            spacing: 0

            K.Label {
                anchors.horizontalCenter: parent.horizontalCenter
                text: TimeFormat.format(root.now, S.Prefs.seconds)
                role: "bar"
                numeric: true
                strong: true
            }

            K.Label {
                visible: S.Prefs.showDate
                anchors.horizontalCenter: parent.horizontalCenter
                text: root.now.toLocaleDateString(TimeFormat.locale, "ddd d MMM")
                role: "barSub"
                tone: "dim"
                numeric: true
            }
        }

        Row {
            visible: root.showWeather
            anchors.verticalCenter: parent.verticalCenter
            spacing: K.Style.s1

            K.Glyph {
                anchors.verticalCenter: parent.verticalCenter
                name: root.weatherGlyph(root.currentWeather ? root.currentWeather.code : -1,
                    root.currentWeather ? root.currentWeather.isDay === true : true)
                size: K.Style.barIcon
                tone: "dim"
            }

            K.Label {
                anchors.verticalCenter: parent.verticalCenter
                text: Weather.temp
                role: "bar"
                numeric: true
            }
        }
    }

    SystemClock {
        id: clock
        precision: S.Prefs.seconds ? SystemClock.Seconds : SystemClock.Minutes
    }
}
