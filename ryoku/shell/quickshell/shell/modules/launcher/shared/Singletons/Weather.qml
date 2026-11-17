pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import shell.services as Svc
import "../lib/weather.js" as Model

// a view of the shell Weather service so the launcher and the bar show one reading
Singleton {
    id: root

    readonly property var current: Svc.Weather.current
    readonly property bool available: Svc.Weather.available
    readonly property string temp: root.current ? String(root.current.temperature || "") : ""
    readonly property string condition: Svc.Weather.condition
    // lib/weather.js labels stay English; this is the word the UI shows
    readonly property string conditionText: {
        switch (root.condition) {
        case "Clear": return I18n.tr("Clear sky");
        case "Cloudy": return I18n.tr("Cloudy");
        case "Fog": return I18n.tr("Fog");
        case "Thunder": return I18n.tr("Thunderstorm");
        case "Snow": return I18n.tr("Snow");
        case "Rain": return I18n.tr("Rain");
        default: return root.condition;
        }
    }
    readonly property string glyph: Svc.Weather.glyph
    readonly property string city: Svc.Weather.city
    readonly property int sunrise: root.current ? Model.clockToSec(root.current.sunrise) : -1
    readonly property int sunset: root.current ? Model.clockToSec(root.current.sunset) : -1
}
