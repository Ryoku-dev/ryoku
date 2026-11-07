pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import shell.barkit as Pill
import "../kit" as K

Item {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})

    readonly property bool loaded: Weather.available && Weather.current !== null
    readonly property var current: Weather.current
    readonly property var hourly: Weather.hourly ? Weather.hourly.slice(0, 8) : []
    readonly property var daily: Weather.daily ? Weather.daily.slice(0, 7) : []
    readonly property var moon: Weather.moon
    readonly property var air: Weather.air
    readonly property string activeUnit: {
        if (Config.weatherUnit === "celsius" || Config.weatherUnit === "fahrenheit")
            return Config.weatherUnit;
        return current && String(current.temperature || "").indexOf("F") >= 0
            ? "fahrenheit" : "celsius";
    }
    readonly property real weekLow: {
        let value = Infinity;
        for (let index = 0; index < daily.length; ++index)
            value = Math.min(value, Number(daily[index].lo));
        return isFinite(value) ? value : 0;
    }
    readonly property real weekHigh: {
        let value = -Infinity;
        for (let index = 0; index < daily.length; ++index)
            value = Math.max(value, Number(daily[index].hi));
        return isFinite(value) ? value : 1;
    }

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.implicitHeight

    function symbolName(icon) {
        const name = String(icon || "wx-unknown");
        return name.indexOf("wx-") === 0 ? "weather-" + name.slice(3) : name;
    }

    function temperature(value, fallback) {
        if (value !== undefined && value !== null && String(value).length > 0)
            return String(value);
        if (fallback !== undefined && fallback !== null && isFinite(Number(fallback)))
            return String(fallback) + "°";
        return "";
    }

    function lowText(day) {
        return day ? temperature(day.low, day.lo) : "";
    }

    function highText(day) {
        return day ? temperature(day.high, day.hi) : "";
    }

    function windText() {
        if (!current)
            return "";
        const direction = String(current.windDir || "");
        const raw = current.wind !== undefined && current.wind !== null
            ? String(current.wind) : String(current.windValue || "");
        const hasUnits = raw.indexOf("mph") >= 0 || raw.indexOf("km/h") >= 0;
        // The daemon's unit string carries its own spacing and a trailing noun.
        const units = hasUnits ? "" : " " + String(current.windUnits || "km/h")
            .replace(/\bwinds?\b/i, "").trim();
        return ((direction.length > 0 ? direction + " " : "") + raw + units)
            .replace(/\s+/g, " ").trim();
    }

    function airText() {
        if (!air || air.available !== true)
            return "";
        const index = air.eaqi !== undefined ? String(air.eaqi) : "";
        const verdict = String(air.verdict || "");
        return index.length > 0 && verdict.length > 0
            ? index + " " + verdict : index + verdict;
    }

    function moonText() {
        if (!moon)
            return "";
        const name = String(moon.name || "");
        const illumination = moon.illumination !== undefined
            ? String(moon.illumination) + "%" : "";
        return name.length > 0 && illumination.length > 0
            ? name + ", " + illumination : name + illumination;
    }

    function dayLowFraction(day) {
        const span = weekHigh - weekLow;
        if (!day || span <= 0)
            return 0;
        return Math.max(0, Math.min(1, (Number(day.lo) - weekLow) / span));
    }

    function dayHighFraction(day) {
        const span = weekHigh - weekLow;
        if (!day || span <= 0)
            return 1;
        return Math.max(0, Math.min(1, (Number(day.hi) - weekLow) / span));
    }

    K.Segmented {
        id: unitPicker
        model: [
            { value: "celsius", label: "°C" },
            { value: "fahrenheit", label: "°F" }
        ]
        value: root.activeUnit
        onPicked: unit => {
            if (unit !== root.activeUnit)
                Weather.setUnit(unit);
        }
    }

    K.Sheet {
        id: sheet
        pageWidth: 360
        title: "Weather"
        trailing: [unitPicker]

        K.Scroller {
            width: parent.width
            height: Math.min(weatherContent.implicitHeight,
                root.scene ? Math.max(0, root.scene.cardMaxHeight - 92) : weatherContent.implicitHeight)

            Column {
                id: weatherContent
                width: parent.width
                spacing: K.Style.s4

                Row {
                    visible: root.loaded
                    width: parent.width
                    height: 72
                    spacing: K.Style.s4

                    Pill.SymbolIcon {
                        anchors.verticalCenter: parent.verticalCenter
                        size: 64
                        name: root.symbolName(root.current ? root.current.icon : "")
                        color: K.Style.ink
                    }

                    Column {
                        width: Math.max(0, parent.width - 64 - parent.spacing)
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: K.Style.s1

                        Row {
                            spacing: K.Style.s3

                            K.Label {
                                text: root.current
                                    ? String(root.current.temperature || "") : ""
                                role: "display"
                                numeric: true
                            }
                            Column {
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: K.Style.s1

                                K.Label {
                                    text: root.current
                                        ? String(root.current.condition || Weather.condition || "") : ""
                                    role: "bodyStrong"
                                }
                                K.Label {
                                    visible: text.length > 0
                                    text: root.current && root.current.feelsLike
                                        ? "Feels like " + root.current.feelsLike : ""
                                    role: "caption"
                                    tone: "variant"
                                    numeric: true
                                }
                            }
                        }
                    }
                }

                Rectangle {
                    visible: root.loaded
                    width: parent.width
                    height: metricsColumn.implicitHeight + K.Style.s3 * 2
                    radius: K.Style.plateRadius
                    color: K.Style.plate
                    border.width: K.Style.border
                    border.color: K.Style.lineSoft

                    Column {
                        id: metricsColumn
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.leftMargin: K.Style.s3
                        anchors.rightMargin: K.Style.s3
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: K.Style.s2

                        Item {
                            visible: root.current && root.current.humidity !== undefined
                            width: parent.width
                            height: 18

                            K.Label {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                text: "Humidity"
                                role: "body"
                            }
                            K.Label {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: root.current ? String(root.current.humidity) + "%" : ""
                                role: "body"
                                tone: "variant"
                                numeric: true
                            }
                        }

                        Item {
                            visible: root.current && (root.current.wind !== undefined
                                || root.current.windValue !== undefined)
                            width: parent.width
                            height: 18

                            K.Label {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                text: "Wind"
                                role: "body"
                            }
                            K.Label {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: root.windText()
                                role: "body"
                                tone: "variant"
                                numeric: true
                            }
                        }

                        Item {
                            visible: root.air && root.air.available === true
                            width: parent.width
                            height: 18

                            K.Label {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                text: "Air quality"
                                role: "body"
                            }
                            K.Label {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: root.airText()
                                role: "body"
                                tone: "variant"
                                numeric: true
                            }
                        }

                        Item {
                            visible: root.moon !== null
                            width: parent.width
                            height: 18

                            K.Label {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                text: "Moon"
                                role: "body"
                            }
                            K.Label {
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: root.moonText()
                                role: "body"
                                tone: "variant"
                                numeric: true
                            }
                        }
                    }
                }

                Column {
                    visible: root.loaded && root.hourly.length > 0
                    width: parent.width
                    spacing: K.Style.s2

                    K.Label {
                        text: "Hourly"
                        role: "micro"
                    }

                    Row {
                        width: parent.width
                        height: 48

                        Repeater {
                            model: root.hourly

                            delegate: Item {
                                id: hourCell
                                required property var modelData
                                width: root.hourly.length > 0
                                    ? parent.width / root.hourly.length : 0
                                height: parent.height

                                Column {
                                    anchors.centerIn: parent
                                    spacing: K.Style.s1

                                    K.Label {
                                        anchors.horizontalCenter: parent.horizontalCenter
                                        text: String(hourCell.modelData.time
                                            || hourCell.modelData.hour || "").replace(/^0(\d)/, "$1")
                                        role: "micro"
                                        tone: "muted"
                                        numeric: true
                                    }
                                    Pill.SymbolIcon {
                                        anchors.horizontalCenter: parent.horizontalCenter
                                        size: 20
                                        name: root.symbolName(hourCell.modelData.icon)
                                        color: K.Style.inkDim
                                    }
                                    K.Label {
                                        anchors.horizontalCenter: parent.horizontalCenter
                                        text: root.temperature(hourCell.modelData.temperature,
                                            hourCell.modelData.temp)
                                        role: "caption"
                                        numeric: true
                                    }
                                }
                            }
                        }
                    }
                }

                Column {
                    visible: root.loaded && root.daily.length > 0
                    width: parent.width
                    spacing: K.Style.s2

                    K.Label {
                        text: "Seven days"
                        role: "micro"
                    }

                    Column {
                        width: parent.width
                        spacing: 0

                        Repeater {
                            model: root.daily

                            delegate: Item {
                                id: dayRow
                                required property var modelData
                                width: parent.width
                                height: K.Style.rowH

                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.top: parent.top
                                    height: K.Style.border
                                    color: K.Style.lineSoft
                                }

                                K.Label {
                                    id: dayName
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 72
                                    text: String(dayRow.modelData.weekday
                                        || dayRow.modelData.day || "")
                                    role: "body"
                                }

                                Pill.SymbolIcon {
                                    id: dayIcon
                                    anchors.left: dayName.right
                                    anchors.leftMargin: K.Style.s1
                                    anchors.verticalCenter: parent.verticalCenter
                                    size: 20
                                    name: root.symbolName(dayRow.modelData.icon)
                                    color: K.Style.inkDim
                                }

                                K.Label {
                                    id: dayLow
                                    anchors.left: dayIcon.right
                                    anchors.leftMargin: K.Style.s2
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 34
                                    text: root.lowText(dayRow.modelData)
                                    role: "caption"
                                    tone: "variant"
                                    numeric: true
                                }

                                K.Label {
                                    id: dayHigh
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 34
                                    text: root.highText(dayRow.modelData)
                                    role: "caption"
                                    numeric: true
                                    strong: true
                                    horizontalAlignment: Text.AlignRight
                                }

                                Item {
                                    anchors.left: dayLow.right
                                    anchors.leftMargin: K.Style.s2
                                    anchors.right: dayHigh.left
                                    anchors.rightMargin: K.Style.s2
                                    anchors.verticalCenter: parent.verticalCenter
                                    height: 6

                                    Rectangle {
                                        anchors.fill: parent
                                        radius: 1
                                        color: K.Style.lineSoft
                                    }
                                    Rectangle {
                                        readonly property real low: root.dayLowFraction(dayRow.modelData)
                                        readonly property real high: root.dayHighFraction(dayRow.modelData)
                                        x: parent.width * low
                                        width: Math.max(3,
                                            parent.width * Math.max(0, high - low))
                                        height: parent.height
                                        radius: 1
                                        color: K.Style.ink
                                    }
                                }
                            }
                        }
                    }
                }

                Row {
                    visible: root.loaded && (Weather.location.length > 0
                        || Weather.updatedAt.length > 0)
                    anchors.horizontalCenter: parent.horizontalCenter
                    spacing: K.Style.s3

                    K.Label {
                        visible: Weather.location.length > 0
                        text: Weather.location
                        role: "caption"
                        tone: "variant"
                    }
                    K.Label {
                        visible: Weather.updatedAt.length > 0
                        text: Weather.updatedAt
                        role: "caption"
                        tone: "variant"
                        numeric: true
                    }
                }

                Column {
                    visible: !root.loaded
                    width: parent.width
                    spacing: K.Style.s3

                    K.Glyph {
                        anchors.horizontalCenter: parent.horizontalCenter
                        name: "cloud_off"
                        size: 48
                        tone: "faint"
                    }
                    K.Label {
                        width: parent.width
                        text: Weather.status === "loading"
                            ? "Weather loading" : "Weather unavailable"
                        role: "body"
                        tone: "variant"
                        horizontalAlignment: Text.AlignHCenter
                    }
                    K.Label {
                        visible: text.length > 0
                        width: parent.width
                        text: Weather.errorText
                        role: "caption"
                        tone: "variant"
                        wrapMode: Text.Wrap
                        maximumLineCount: 3
                        horizontalAlignment: Text.AlignHCenter
                    }
                    K.Button {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: "Retry"
                        icon: "refresh"
                        onClicked: Weather.retry()
                    }
                }
            }
        }
    }
}
