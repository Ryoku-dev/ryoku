pragma Singleton
import QtQuick
import Quickshell
import shell.services as Svc
import "../lib/weather.js" as Model

// a view of the shell Weather service so the launcher and the bar show one reading
Singleton {
    id: root

    // "" follows the shell's unit
    property string unitOverride: ""

    readonly property var current: Svc.Weather.current
    readonly property bool available: Svc.Weather.available
    readonly property string temp: root.current
        ? Model.displayTemp(root.current.temperature, root.current.tempC, root.unitOverride) : ""
    readonly property string condition: Svc.Weather.condition
    readonly property string glyph: Svc.Weather.glyph
    readonly property string city: Svc.Weather.city
    readonly property int sunrise: root.current ? Model.clockToSec(root.current.sunrise) : -1
    readonly property int sunset: root.current ? Model.clockToSec(root.current.sunset) : -1
}
