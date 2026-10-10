import QtQuick
import shell.services
import "../../../../../services/lib/weather.js" as Wx
import Ryoku.Ui.Singletons

// The bar's weather glyph. Reads the shared daemon-fed Weather singleton (the
// same source as the dashboard this widget opens), so it honours the configured
// weatherLocation and weatherUnit and changes the moment either does. It used to
// curl wttr.in with no location, which meant IP geolocation: the bar showed the
// city of the exit node while the dashboard beside it showed the configured one.
Item {
    id: rootMod
    required property var root
    signal activated()

    readonly property var cur: Weather.current
    readonly property bool weatherLoaded: Weather.available && rootMod.cur !== null
    readonly property bool weatherUnavailable: Weather.status === "error"
    readonly property string weatherIcon: rootMod.weatherLoaded
        ? Wx.nerdFor(rootMod.cur.code, rootMod.cur.isDay) : "\ue33d"
    readonly property string weatherPlace: Weather.city !== "" ? Weather.city : Weather.location
    readonly property string weatherDesc: rootMod.weatherLoaded ? Weather.condition : ""
    readonly property color contentColor: root.widgetContentColor("G8", root.ink)

    // The daemon reads and formats the temperature in the configured
    // weatherUnit, so the bar's own imperial toggle only has work to do when
    // that unit is metric. Converting unconditionally would double-convert a
    // shell already set to Fahrenheit.
    readonly property bool daemonImperial: Weather.temp.indexOf("F") >= 0
    readonly property string weatherTempStr:
        (root.weatherImperial && rootMod.weatherLoaded && !rootMod.daemonImperial)
            ? (Math.round(Weather.tempNow * 9 / 5 + 32) + "\u00b0F")
            : Weather.temp
    readonly property string tooltipText: {
        const reading = (rootMod.weatherPlace ? rootMod.weatherPlace + " \u00b7 " : "")
            + rootMod.weatherTempStr
            + (rootMod.weatherDesc ? " / " + rootMod.weatherDesc : "");
        if (rootMod.weatherLoaded)
            return reading;
        return rootMod.weatherUnavailable ? I18n.tr("Weather offline") : I18n.tr("Weather\u2026");
    }

    implicitWidth: root.modWeather ? ico.implicitWidth : 0
    implicitHeight: 28

    Text {
        id: ico
        anchors.centerIn: parent
        text: rootMod.weatherLoaded ? rootMod.weatherIcon
              : (rootMod.weatherUnavailable ? "?" : "·")
        color: rootMod.weatherUnavailable && !rootMod.weatherLoaded
               ? Qt.rgba(rootMod.contentColor.r, rootMod.contentColor.g, rootMod.contentColor.b, 0.4)
               : rootMod.contentColor
        font.family: root.mono
        font.pixelSize: 14
    }

    TooltipMixin { id: tip; root: rootMod.root; owner: rootMod; text: rootMod.tooltipText }

    MouseArea {
        id: mouse
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        onEntered: { if (rootMod.tooltipText) tip.show(); }
        onExited: { tip.hide(); }
        onClicked: (e) => {
            tip.hide();
            if (e.button === Qt.RightButton) TimeFormat.set24h(root.clock12h);
            else rootMod.activated();
        }
    }
}
