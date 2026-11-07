pragma Singleton
import QtQuick
import Quickshell
import shell.services as Svc
import "../lib/weather.js" as Model

// The launcher's weather is a view of the shell Weather service (the daemon's
// `weather` topic), so the hero and the bar show one reading and the launcher
// makes no network call. Only the launcher's own unit override is applied here.
Singleton {
    id: root

    // From the launcher config: "" follows the shell's unit, "C"/"F" force it.
    property string unitOverride: ""

    readonly property var current: Svc.Weather.current
    readonly property bool available: Svc.Weather.available
    readonly property string temp: root.current
        ? Model.displayTemp(root.current.temperature, root.current.temp, root.unitOverride) : ""
    readonly property string condition: Svc.Weather.condition
    readonly property string glyph: Svc.Weather.glyph
    readonly property string city: Svc.Weather.city
    // Seconds since local midnight, -1 when the frame has none.
    readonly property int sunrise: root.current ? Model.clockToSec(root.current.sunrise) : -1
    readonly property int sunset: root.current ? Model.clockToSec(root.current.sunset) : -1
}
