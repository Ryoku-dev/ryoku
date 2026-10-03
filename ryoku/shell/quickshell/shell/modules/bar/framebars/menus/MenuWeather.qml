pragma ComponentBehavior: Bound

import QtQuick
import "../.." as Pill
import shell.services
import "../../../../components"
import "../../../../services/lib/weather.js" as Wx
import Ryoku.Ui.Singletons

// The weather surface shared by the clock menu and weather frame widget. The
// daemon (weather.go) owns the Open-Meteo forecast + air-quality fetch, the moon
// phase, the range context and every display string, so this file makes no
// network call and does no unit maths - it renders the frame verbatim into eight
// quiet cards: a current
// hero, an hourly strip, three daily rows with range bars, a sun row, the moon
// phase strip, a metrics grid, the air-quality block, and an updated-at footer.
// An outer state stack crossfades between loading, error and the loaded body.
Item {
    id: root

    required property real s
    required property bool open

    readonly property string status: Weather.status
    readonly property bool loaded: root.status === "loaded" && Weather.current !== null

    readonly property var cur: Weather.current
    readonly property var moon: Weather.moon
    readonly property var air: Weather.air

    // Ink roles resolved against the surface the card truly sits on: strong text
    // clears 4.5:1, large glyphs relax to 3.0, the accent tracks primary.
    readonly property color ink: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
    readonly property color inkVar: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant)
    readonly property color glyphInk: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface, 3.0)
    readonly property color trackTone: Qt.rgba(root.inkVar.r, root.inkVar.g, root.inkVar.b, 0.20)

    readonly property string windU: (root.cur && String(root.cur.windUnits).indexOf("mph") >= 0) ? I18n.tr("mph") : I18n.tr("km/h")

    // The active display unit: an explicit weatherUnit wins; "auto" is read off
    // the frame's own temperature suffix so the toggle reflects what is shown.
    readonly property string activeUnit: {
        if (Config.weatherUnit === "celsius") return "celsius";
        if (Config.weatherUnit === "fahrenheit") return "fahrenheit";
        return (root.cur && String(root.cur.temperature).indexOf("F") >= 0) ? "fahrenheit" : "celsius";
    }
    readonly property var unitOptions: [ { id: "celsius", label: "\u00b0C" }, { id: "fahrenheit", label: "\u00b0F" } ]

    // The six-metric grid, driven off the daemon's pre-formatted strings.
    readonly property var metrics: root.cur ? [
        { icon: "thermostat",          label: I18n.tr("Feels"),      value: root.cur.feelsLike,                    sub: "" },
        { icon: "humidity_percentage", label: I18n.tr("Humidity"),   value: root.cur.humidity + "%",               sub: "" },
        { icon: "air",                 label: I18n.tr("Wind"),       value: root.cur.windDir + " " + root.cur.windValue, sub: root.windU },
        { icon: "rainy",               label: I18n.tr("Precip"),     value: root.cur.precipProb + "%",             sub: root.cur.precip },
        { icon: "visibility",          label: I18n.tr("Visibility"), value: root.cur.visibility,                   sub: "" },
        { icon: "compress",            label: I18n.tr("Pressure"),   value: root.cur.pressure,                     sub: "" }
    ] : []

    // Canonical illumination for each of the 8 strip glyphs; the current index
    // is lit brighter than the rest.
    readonly property var moonStrip: [
        { frac: 0.0,  waxing: true  },
        { frac: 0.25, waxing: true  },
        { frac: 0.5,  waxing: true  },
        { frac: 0.75, waxing: true  },
        { frac: 1.0,  waxing: true  },
        { frac: 0.75, waxing: false },
        { frac: 0.5,  waxing: false },
        { frac: 0.25, waxing: false }
    ]

    implicitWidth: 320 * root.s
    implicitHeight: root.status === "loaded" ? loadedBox.implicitHeight
        : root.status === "error" ? errorBox.implicitHeight
        : loadingBox.implicitHeight
    Behavior on implicitHeight { NumberAnimation { duration: Motion.weatherFade; easing.type: Easing.OutCubic } }

    // --- loading ---
    Item {
        id: loadingBox
        width: root.width
        implicitHeight: loadingLabel.implicitHeight + 24 * root.s
        opacity: root.status === "loading" ? 1 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: Motion.weatherFade; easing.type: Motion.crossfadeCurve } }
        Text {
            id: loadingLabel
            anchors.centerIn: parent
            text: I18n.tr("Weather loading\u2026")
            color: root.glyphInk
            horizontalAlignment: Text.AlignHCenter
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontMd * root.s
            font.weight: Font.Bold
        }
    }

    // --- error ---
    Column {
        id: errorBox
        width: root.width
        spacing: 8 * root.s
        opacity: root.status === "error" ? 1 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: Motion.weatherFade; easing.type: Motion.crossfadeCurve } }

        Text {
            width: parent.width
            text: Weather.errorText.length > 0 ? Weather.errorText : I18n.tr("Error loading weather.")
            color: Theme.inkOn(Theme.effectiveSurface, Theme.error, 3.0)
            horizontalAlignment: Text.AlignHCenter
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontMd * root.s
            font.weight: Font.Bold
        }
        MenuButton {
            anchors.horizontalCenter: parent.horizontalCenter
            minW: retryLabel.implicitWidth + 2 * pad
            minH: retryLabel.implicitHeight + 2 * pad
            pad: Theme.paddingMd * root.s
            radius: Theme.radiusWidget
            onClicked: Weather.retry()
            Text {
                id: retryLabel
                anchors.centerIn: parent
                text: I18n.tr("Retry")
                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                font.family: Theme.fontPrimary
                font.pixelSize: Theme.fontSm * root.s
            }
        }
    }

    // --- loaded ---
    Column {
        id: loadedBox
        width: root.width
        spacing: 12 * root.s
        opacity: root.loaded ? 1 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: Motion.weatherFade; easing.type: Motion.crossfadeCurve } }

        // 1. Current hero -----------------------------------------------------
        WeatherCard {
            s: root.s
            width: parent.width

            // location line + refresh button
            Item {
                width: parent.width
                height: refreshBtn.height
                Text {
                    anchors.left: parent.left
                    anchors.right: unitToggle.left
                    anchors.rightMargin: 8 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: Weather.location
                    elide: Text.ElideRight
                    color: root.inkVar
                    font.family: Theme.fontPrimary
                    font.pixelSize: Theme.fontSm * root.s
                    font.weight: Font.DemiBold
                }
                // Fahrenheit / Celsius switch: patches the persisted weatherUnit
                // through the daemon, which re-renders the whole surface in the
                // chosen unit. The active option is filled with the primary tint.
                Rectangle {
                    id: unitToggle
                    anchors.right: refreshBtn.left
                    anchors.rightMargin: 8 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    width: unitRow.width + 4 * root.s
                    height: 26 * root.s
                    radius: Theme.radiusWidget
                    color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.06)
                    border.width: 1
                    border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.30)
                    Row {
                        id: unitRow
                        anchors.centerIn: parent
                        spacing: 2 * root.s
                        Repeater {
                            model: root.unitOptions
                            delegate: Rectangle {
                                id: unitSeg
                                required property var modelData
                                readonly property bool active: root.activeUnit === unitSeg.modelData.id
                                width: 24 * root.s
                                height: 20 * root.s
                                radius: Theme.radiusWidget - 4
                                color: unitSeg.active ? Theme.primary
                                    : uTap.containsMouse ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.10)
                                    : "transparent"
                                Behavior on color { ColorAnimation { duration: Motion.crossfade; easing.type: Motion.crossfadeCurve } }
                                Text {
                                    anchors.centerIn: parent
                                    text: I18n.tr(unitSeg.modelData.label)
                                    color: unitSeg.active ? Theme.inkOn(Theme.primary, Theme.onPrimary) : root.inkVar
                                    font.family: Theme.fontPrimary
                                    font.pixelSize: (Theme.fontSm - 2) * root.s
                                    font.weight: unitSeg.active ? Font.DemiBold : Font.Normal
                                }
                                MouseArea {
                                    id: uTap
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: if (!unitSeg.active) Weather.setUnit(unitSeg.modelData.id)
                                }
                            }
                        }
                    }
                }
                QsIconButton {
                    id: refreshBtn
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    icon: "refresh"
                    tip: I18n.tr("Refresh")
                    tipBelow: true
                    tipAlign: "right"
                    onClicked: Weather.retry()
                }
            }

            Row {
                width: parent.width
                spacing: 14 * root.s
                MaterialIcon {
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.cur ? Wx.symbolFor(root.cur.code, root.cur.isDay) : "cloud"
                    font.pixelSize: 56 * root.s
                    color: root.glyphInk
                }
                Column {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 2 * root.s
                    Text {
                        text: root.cur ? root.cur.temperature : ""
                        color: root.ink
                        font.family: Theme.display
                        font.pixelSize: 34 * root.s
                        font.weight: Font.DemiBold
                    }
                    Text {
                        text: root.cur && root.cur.condition ? root.cur.condition : ""
                        color: root.inkVar
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontMd * root.s
                    }
                    Text {
                        text: (root.cur && root.cur.high !== undefined) ? ("\u2191 " + root.cur.high + "    \u2193 " + root.cur.low) : ""
                        color: root.inkVar
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontSm * root.s
                        font.weight: Font.DemiBold
                    }
                }
            }
        }

        // 2. Hourly strip -----------------------------------------------------
        WeatherCard {
            s: root.s
            width: parent.width
            eyebrow: I18n.tr("Hourly")

            Row {
                width: parent.width
                Repeater {
                    model: Math.min(6, Weather.hourly.length)
                    delegate: Column {
                        id: hourCell
                        required property int index
                        readonly property var h: Weather.hourly[hourCell.index]
                        width: parent.width / 6
                        spacing: 6 * root.s
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: hourCell.h.time
                            color: root.inkVar
                            font.family: Theme.mono
                            font.pixelSize: (Theme.fontSm - 3) * root.s
                            font.weight: Font.DemiBold
                        }
                        MaterialIcon {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: Wx.symbolFor(hourCell.h.code, hourCell.h.isDay)
                            font.pixelSize: 21 * root.s
                            color: root.glyphInk
                        }
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: hourCell.h.temp + "\u00b0"
                            color: root.ink
                            font.family: Theme.fontPrimary
                            font.pixelSize: Theme.fontSm * root.s
                            font.weight: Font.DemiBold
                        }
                    }
                }
            }
        }

        // 3. Daily rows with range bars --------------------------------------
        WeatherCard {
            s: root.s
            width: parent.width
            eyebrow: I18n.tr("3-Day")

            Repeater {
                model: Math.min(3, Weather.daily.length)
                delegate: Item {
                    id: dayRow
                    required property int index
                    readonly property var d: Weather.daily[dayRow.index]
                    width: parent.width
                    height: 22 * root.s

                    Text {
                        id: dName
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        width: 34 * root.s
                        text: dayRow.d.day
                        color: root.ink
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontSm * root.s
                        font.weight: Font.DemiBold
                    }
                    // Fixed-width glyph slot: 24px so dLow always starts at a
                    // predictable x regardless of which symbol the font renders.
                    Item {
                        id: dGlyphSlot
                        anchors.left: dName.right
                        anchors.leftMargin: 6 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        width: 24 * root.s
                        height: 22 * root.s
                        MaterialIcon {
                            id: dGlyph
                            anchors.centerIn: parent
                            text: Wx.symbolFor(dayRow.d.code, true)
                            font.pixelSize: 18 * root.s
                            color: root.glyphInk
                        }
                    }
                    // Low temp: 44px right-aligned - wide enough for "64.86°F".
                    Text {
                        id: dLow
                        anchors.left: dGlyphSlot.right
                        anchors.leftMargin: 6 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        width: 44 * root.s
                        horizontalAlignment: Text.AlignRight
                        text: dayRow.d.low
                        color: root.inkVar
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontSm * root.s
                    }
                    // High temp: 44px right-aligned.
                    Text {
                        id: dHigh
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        width: 44 * root.s
                        horizontalAlignment: Text.AlignRight
                        text: dayRow.d.high
                        color: root.ink
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontSm * root.s
                        font.weight: Font.DemiBold
                    }
                    // Range bar flexes in the space between low and high.
                    Item {
                        id: dBar
                        anchors.left: dLow.right
                        anchors.leftMargin: 8 * root.s
                        anchors.right: dHigh.left
                        anchors.rightMargin: 8 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        height: 4 * root.s
                        Rectangle {
                            anchors.fill: parent
                            radius: height / 2
                            color: root.trackTone
                        }
                        Rectangle {
                            x: dBar.width * dayRow.d.loFrac
                            width: Math.max(2 * root.s, dBar.width * (dayRow.d.hiFrac - dayRow.d.loFrac))
                            height: parent.height
                            radius: height / 2
                            color: Theme.primary
                        }
                    }
                }
            }
        }

        // 4. Sun row ----------------------------------------------------------
        WeatherCard {
            s: root.s
            width: parent.width
            eyebrow: I18n.tr("Sun")

            Row {
                width: parent.width
                Repeater {
                    model: [
                        { label: I18n.tr("Sunrise"), icon: "wb_sunny",    value: root.cur ? root.cur.sunrise : "" },
                        { label: I18n.tr("Sunset"),  icon: "wb_twilight", value: root.cur ? root.cur.sunset : "" }
                    ]
                    delegate: Column {
                        required property var modelData
                        width: parent.width / 2
                        spacing: 4 * root.s
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: modelData.label.toUpperCase()
                            color: root.inkVar
                            font.family: Theme.mono
                            font.pixelSize: (Theme.fontSm - 4) * root.s
                            font.letterSpacing: 1.6 * root.s
                        }
                        Row {
                            anchors.horizontalCenter: parent.horizontalCenter
                            spacing: 6 * root.s
                            MaterialIcon {
                                anchors.verticalCenter: parent.verticalCenter
                                text: modelData.icon
                                font.pixelSize: 19 * root.s
                                color: root.glyphInk
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: modelData.value
                                color: root.ink
                                font.family: Theme.fontPrimary
                                font.pixelSize: Theme.fontMd * root.s
                                font.weight: Font.DemiBold
                            }
                        }
                    }
                }
            }
        }

        // 5. Moon -------------------------------------------------------------
        WeatherCard {
            s: root.s
            width: parent.width
            eyebrow: I18n.tr("Moon")

            Row {
                width: parent.width
                spacing: 12 * root.s
                WeatherMoon {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 46 * root.s
                    height: 46 * root.s
                    frac: root.moon ? root.moon.fraction : 0
                    waxing: root.moon ? root.moon.waxing : true
                    litColor: root.glyphInk
                }
                Column {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 2 * root.s
                    Text {
                        text: root.moon ? root.moon.name : ""
                        color: root.ink
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontMd * root.s
                        font.weight: Font.DemiBold
                    }
                    Text {
                        text: I18n.tr("%1% illuminated").arg(root.moon ? root.moon.illumination : 0)
                        color: root.inkVar
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontSm * root.s
                    }
                }
            }

            Row {
                width: parent.width
                Repeater {
                    model: root.moonStrip
                    delegate: Item {
                        id: moonCell
                        required property int index
                        required property var modelData
                        readonly property bool isCurrent: root.moon && moonCell.index === root.moon.phase
                        width: parent.width / 8
                        height: 28 * root.s
                        WeatherMoon {
                            anchors.horizontalCenter: parent.horizontalCenter
                            anchors.top: parent.top
                            width: 21 * root.s
                            height: 21 * root.s
                            frac: moonCell.modelData.frac
                            waxing: moonCell.modelData.waxing
                            litColor: moonCell.isCurrent ? root.glyphInk
                                : Qt.rgba(root.inkVar.r, root.inkVar.g, root.inkVar.b, 0.45)
                        }
                        Rectangle {
                            visible: moonCell.isCurrent
                            anchors.horizontalCenter: parent.horizontalCenter
                            anchors.bottom: parent.bottom
                            width: 3 * root.s
                            height: 3 * root.s
                            radius: width / 2
                            color: Theme.primary
                        }
                    }
                }
            }
        }

        // 6. Metrics grid -----------------------------------------------------
        WeatherCard {
            s: root.s
            width: parent.width
            eyebrow: I18n.tr("Conditions")

            Grid {
                width: parent.width
                columns: 3
                rowSpacing: 12 * root.s
                columnSpacing: 8 * root.s
                Repeater {
                    model: root.metrics
                    delegate: Column {
                        id: metricCell
                        required property var modelData
                        width: (parent.width - 2 * (8 * root.s)) / 3
                        spacing: 3 * root.s
                        Row {
                            spacing: 5 * root.s
                            MaterialIcon {
                                anchors.verticalCenter: parent.verticalCenter
                                text: metricCell.modelData.icon
                                font.pixelSize: 15 * root.s
                                color: root.glyphInk
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: metricCell.modelData.label.toUpperCase()
                                color: root.inkVar
                                font.family: Theme.mono
                                font.pixelSize: (Theme.fontSm - 4) * root.s
                                font.letterSpacing: 1.4 * root.s
                            }
                        }
                        Text {
                            text: metricCell.modelData.value || ""
                            color: root.ink
                            font.family: Theme.fontPrimary
                            font.pixelSize: Theme.fontMd * root.s
                            font.weight: Font.DemiBold
                        }
                        Text {
                            visible: String(metricCell.modelData.sub).length > 0
                            text: metricCell.modelData.sub || ""
                            color: root.inkVar
                            font.family: Theme.fontPrimary
                            font.pixelSize: (Theme.fontSm - 2) * root.s
                        }
                    }
                }
            }
        }

        // 7. Air quality ------------------------------------------------------
        WeatherCard {
            s: root.s
            width: parent.width
            eyebrow: I18n.tr("Air Quality")

            Row {
                width: parent.width
                spacing: 10 * root.s
                visible: root.air && root.air.available
                MaterialIcon {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "eco"
                    font.pixelSize: 24 * root.s
                    color: root.glyphInk
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.air ? root.air.eaqi : "0"
                    color: root.ink
                    font.family: Theme.display
                    font.pixelSize: 24 * root.s
                    font.weight: Font.DemiBold
                }
                Column {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 1 * root.s
                    Text {
                        text: I18n.tr("EU AQI")
                        color: root.inkVar
                        font.family: Theme.mono
                        font.pixelSize: (Theme.fontSm - 4) * root.s
                        font.letterSpacing: 1.4 * root.s
                    }
                    Text {
                        text: root.air ? root.air.verdict : ""
                        color: root.ink
                        font.family: Theme.fontPrimary
                        font.pixelSize: Theme.fontSm * root.s
                        font.weight: Font.DemiBold
                    }
                }
            }

            // scale bar with the value marked
            Item {
                width: parent.width
                height: 10 * root.s
                visible: root.air && root.air.available
                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width
                    height: 4 * root.s
                    radius: height / 2
                    color: root.trackTone
                }
                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width * (root.air ? root.air.frac : 0)
                    height: 4 * root.s
                    radius: height / 2
                    color: Theme.primary
                }
                Rectangle {
                    x: parent.width * (root.air ? root.air.frac : 0) - width / 2
                    anchors.verticalCenter: parent.verticalCenter
                    width: 3 * root.s
                    height: 10 * root.s
                    radius: width / 2
                    color: root.ink
                }
            }

            Row {
                width: parent.width
                visible: root.air && root.air.available
                Repeater {
                    model: root.air ? [
                        { label: I18n.tr("PM2.5"), value: root.air.pm25 },
                        { label: I18n.tr("PM10"),  value: root.air.pm10 },
                        { label: I18n.tr("Ozone"), value: root.air.ozone }
                    ] : []
                    delegate: Column {
                        required property var modelData
                        width: parent.width / 3
                        spacing: 2 * root.s
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: modelData.label.toUpperCase()
                            color: root.inkVar
                            font.family: Theme.mono
                            font.pixelSize: (Theme.fontSm - 4) * root.s
                            font.letterSpacing: 1.2 * root.s
                        }
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: modelData.value
                            color: root.ink
                            font.family: Theme.fontPrimary
                            font.pixelSize: Theme.fontMd * root.s
                            font.weight: Font.DemiBold
                        }
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: I18n.tr("\u00b5g/m\u00b3")
                            color: root.inkVar
                            font.family: Theme.fontPrimary
                            font.pixelSize: (Theme.fontSm - 3) * root.s
                        }
                    }
                }
            }

            Text {
                width: parent.width
                visible: !(root.air && root.air.available)
                horizontalAlignment: Text.AlignHCenter
                text: I18n.tr("Air quality unavailable")
                color: root.inkVar
                font.family: Theme.fontPrimary
                font.pixelSize: Theme.fontSm * root.s
            }
        }

        // 8. Footer -----------------------------------------------------------
        Text {
            width: parent.width
            visible: Weather.updatedAt.length > 0
            horizontalAlignment: Text.AlignHCenter
            text: I18n.tr("Updated %1").arg(Weather.updatedAt)
            color: root.inkVar
            font.family: Theme.fontPrimary
            font.pixelSize: (Theme.fontSm - 1) * root.s
        }
    }
}
