import QtQuick
import shell.services
import shell.barkit as Pill
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: Weather.available

    readonly property var current: Weather.current
    readonly property string place: Weather.location.length > 0
        ? Weather.location : Weather.city
    readonly property string hintText: Weather.condition.length > 0
        && place.length > 0 ? Weather.condition + ", " + place
        : Weather.condition.length > 0 ? Weather.condition : place

    implicitWidth: present ? content.implicitWidth + 2 * K.Style.hitPad : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    function fallbackSymbol(code, day) {
        const phase = day ? "day" : "night";
        if (code === 0) return "weather-clear-" + phase;
        if (code === 1 || code === 2) return "weather-partly-cloudy-" + phase;
        if (code === 3) return "weather-overcast";
        if (code >= 51 && code <= 57) return "weather-drizzle";
        if (code >= 95) return "weather-thunderstorm";
        return "weather-cloudy";
    }

    function symbolName() {
        const icon = String(current && current.icon ? current.icon : "");
        if (icon.length > 0)
            return icon.indexOf("wx-") === 0 ? "weather-" + icon.slice(3) : icon;
        return fallbackSymbol(Number(current ? current.code : -1),
            current ? current.isDay === true : true);
    }

    Row {
        id: content
        anchors.centerIn: parent
        spacing: K.Style.s1
        scale: press.pressScale

        Pill.SymbolIcon {
            anchors.verticalCenter: parent.verticalCenter
            width: K.Style.barIcon
            height: K.Style.barIcon
            name: root.symbolName()
            size: K.Style.barIcon
            color: K.Style.inkDim
        }

        K.Label {
            anchors.verticalCenter: parent.verticalCenter
            text: Weather.temp
            role: "bar"
            numeric: true
            strong: true
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.toggleCard("weather", root)
        onRightClicked: root.scene.toggleCard("weather", root)
        onHoveredChanged: root.scene.hoverCard("weather", root, hovered)
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: root.present && press.hovered && root.hintText.length > 0
    }
}
