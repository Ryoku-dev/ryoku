pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."

Item {
    id: root

    required property real s
    required property bool open
    required property real reveal
    required property bool tabActive
    property int index: 0
    signal requestClose()

    readonly property var cur: Weather.current
    readonly property bool loaded: Weather.status === "loaded" && root.cur !== null
    readonly property string activeUnit: Config.weatherUnit === "celsius" ? "celsius"
        : Config.weatherUnit === "fahrenheit" ? "fahrenheit"
        : root.cur && String(root.cur.temperature).indexOf("F") >= 0 ? "fahrenheit" : "celsius"
    readonly property real gap: Tokens.s3 * root.s
    readonly property real pad: Tokens.s4 * root.s
    property string dailyMode: "weather"

    implicitHeight: shell.implicitHeight

    function dailyHighs(): var {
        const out = [];
        for (let i = 0; i < Math.min(6, Weather.daily.length); i++)
            out.push(Number(Weather.daily[i].hi || 0));
        return out;
    }

    function dailyLows(): var {
        const out = [];
        for (let i = 0; i < Math.min(6, Weather.daily.length); i++)
            out.push(Number(Weather.daily[i].lo || 0));
        return out;
    }

    function hourlyTemps(): var {
        const out = [];
        for (let i = 0; i < Math.min(6, Weather.hourly.length); i++)
            out.push(Number(Weather.hourly[i].temp || 0));
        return out;
    }

    component Sparkline: Canvas {
        id: spark
        required property var values
        property color stroke: Tokens.sun
        property real strokeWidth: 2 * root.s

        implicitHeight: 38 * root.s
        renderStrategy: Canvas.Cooperative
        onValuesChanged: requestPaint()
        onWidthChanged: requestPaint()
        onHeightChanged: requestPaint()
        onStrokeChanged: requestPaint()
        onPaint: {
            const ctx = getContext("2d");
            ctx.reset();
            if (!spark.values || spark.values.length < 2)
                return;
            let lo = spark.values[0], hi = spark.values[0];
            for (let i = 1; i < spark.values.length; i++) {
                lo = Math.min(lo, spark.values[i]);
                hi = Math.max(hi, spark.values[i]);
            }
            const range = Math.max(1, hi - lo);
            const inset = 4 * root.s;
            ctx.lineWidth = spark.strokeWidth;
            ctx.strokeStyle = spark.stroke;
            ctx.lineCap = "round";
            ctx.lineJoin = "round";
            ctx.beginPath();
            for (let j = 0; j < spark.values.length; j++) {
                const x = inset + j * (spark.width - inset * 2) / (spark.values.length - 1);
                const y = spark.height - inset - (spark.values[j] - lo) / range * (spark.height - inset * 2);
                if (j === 0)
                    ctx.moveTo(x, y);
                else
                    ctx.lineTo(x, y);
            }
            ctx.stroke();
        }
    }

    SidebarCardShell {
        id: shell
        width: root.width
        index: root.index
        open: root.open
        reveal: root.reveal
        tabActive: root.tabActive
        title: I18n.tr("Weather")
        glyph: "cloud"
        eyebrow: Weather.location.length > 0 ? Weather.location : I18n.tr("LOCAL FORECAST")

        Column {
            width: parent.width
            spacing: root.gap

            Rectangle {
                visible: Weather.status === "loading"
                width: parent.width
                implicitHeight: 126 * root.s
                radius: Tokens.radius * root.s
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.line
                Column {
                    anchors.centerIn: parent
                    spacing: Tokens.s2 * root.s
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: "partly_cloudy_day"
                        color: Tokens.sun
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 38 * root.s
                    }
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: I18n.tr("Weather loading…")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                    }
                }
            }

            Rectangle {
                visible: Weather.status === "error"
                width: parent.width
                implicitHeight: errorColumn.implicitHeight + Tokens.s6 * root.s
                radius: Tokens.radius * root.s
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.line
                Column {
                    id: errorColumn
                    anchors.centerIn: parent
                    width: parent.width - root.pad * 2
                    spacing: Tokens.s3 * root.s
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: "cloud_off"
                        color: Tokens.alert
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 34 * root.s
                    }
                    Text {
                        width: parent.width
                        text: Weather.errorText.length > 0 ? Weather.errorText : I18n.tr("Error loading weather.")
                        color: Tokens.inkMuted
                        horizontalAlignment: Text.AlignHCenter
                        wrapMode: Text.Wrap
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                    }
                    Btn {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: I18n.tr("Retry")
                        primary: true
                        onAct: Weather.retry()
                    }
                }
            }

            Rectangle {
                visible: root.loaded
                width: parent.width
                implicitHeight: 248 * root.s
                radius: Tokens.radius * root.s
                clip: true
                border.width: Tokens.border
                border.color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.44)
                gradient: Gradient {
                    orientation: Gradient.Vertical
                    GradientStop { position: 0; color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.28) }
                    GradientStop { position: 0.54; color: Qt.rgba(Tokens.sunDeep.r, Tokens.sunDeep.g, Tokens.sunDeep.b, 0.13) }
                    GradientStop { position: 1; color: Tokens.paperLift }
                }

                Text {
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.rightMargin: -18 * root.s
                    anchors.topMargin: -26 * root.s
                    text: root.cur ? root.cur.icon : "cloud"
                    color: Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.08)
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: 188 * root.s
                }

                Item {
                    anchors.fill: parent
                    anchors.margins: root.pad

                    Item {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.top: parent.top
                        implicitHeight: Math.max(locationLabel.implicitHeight, refreshButton.height)
                        Text {
                            id: locationLabel
                            anchors.left: parent.left
                            anchors.right: unitPicker.left
                            anchors.rightMargin: Tokens.s2 * root.s
                            anchors.verticalCenter: parent.verticalCenter
                            text: Weather.location
                            color: Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall * root.s
                            font.weight: Font.DemiBold
                            elide: Text.ElideRight
                        }
                        Seg {
                            id: unitPicker
                            anchors.right: refreshButton.left
                            anchors.rightMargin: Tokens.s2 * root.s
                            anchors.verticalCenter: parent.verticalCenter
                            options: ["celsius", "fahrenheit"]
                            labels: ({ celsius: "°C", fahrenheit: "°F" })
                            current: root.activeUnit
                            onChose: unit => Weather.setUnit(unit)
                        }
                        Rectangle {
                            id: refreshButton
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            width: 28 * root.s
                            height: width
                            radius: Tokens.radius * root.s
                            color: refreshTap.pressed ? Tokens.tint16 : refreshHover.hovered ? Tokens.tint10 : "transparent"
                            border.width: Tokens.border
                            border.color: refreshHover.hovered ? Tokens.lineStrong : Tokens.line
                            Text {
                                anchors.centerIn: parent
                                text: "refresh"
                                color: Tokens.inkDim
                                font.family: "Material Symbols Rounded"
                                font.pixelSize: 17 * root.s
                            }
                            HoverHandler { id: refreshHover; cursorShape: Qt.PointingHandCursor }
                            TapHandler { id: refreshTap; onTapped: Weather.retry() }
                        }
                    }

                    Column {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: Tokens.s1 * root.s
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: root.cur ? root.cur.condition : ""
                            color: Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fRow * root.s
                            font.weight: Font.Medium
                        }
                        Row {
                            anchors.horizontalCenter: parent.horizontalCenter
                            spacing: Tokens.s3 * root.s
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: root.cur ? root.cur.temp + "°" : ""
                                color: Tokens.ink
                                font.family: Tokens.display
                                font.pixelSize: Tokens.fValue * 2.8 * root.s
                                font.weight: Font.Medium
                                font.features: ({ "tnum": 1 })
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: root.cur ? root.cur.icon : "cloud"
                                color: Tokens.sun
                                font.family: "Material Symbols Rounded"
                                font.pixelSize: 48 * root.s
                            }
                        }
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: root.cur ? I18n.tr("Feels like %1 · High %2 · Low %3").arg(root.cur.feelsLike).arg(root.cur.high).arg(root.cur.low) : ""
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall * root.s
                        }
                    }

                    Text {
                        anchors.left: parent.left
                        anchors.bottom: parent.bottom
                        text: root.cur ? I18n.tr("Humidity %1%").arg(root.cur.humidity) : ""
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro * root.s
                    }
                    Text {
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        text: root.cur ? I18n.tr("Wind %1").arg(root.cur.wind) : ""
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro * root.s
                    }
                }
            }

            Rectangle {
                visible: root.loaded
                width: parent.width
                implicitHeight: dailyColumn.implicitHeight + root.pad * 2
                radius: Tokens.radius * root.s
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.line

                Column {
                    id: dailyColumn
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: root.pad
                    spacing: Tokens.s3 * root.s

                    Row {
                        width: parent.width
                        Text {
                            width: parent.width - dailyChips.width
                            text: I18n.tr("Daily forecast")
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fRow * root.s
                            font.weight: Font.DemiBold
                        }
                        Seg {
                            id: dailyChips
                            options: ["weather", "range"]
                            labels: ({ weather: I18n.tr("Weather"), range: I18n.tr("Range") })
                            current: root.dailyMode
                            onChose: value => root.dailyMode = value
                        }
                    }

                    Row {
                        width: parent.width
                        Repeater {
                            model: Math.min(6, Weather.daily.length)
                            delegate: Column {
                                id: dayCell
                                required property int index
                                readonly property var day: Weather.daily[dayCell.index]
                                width: parent.width / 6
                                spacing: Tokens.s1 * root.s
                                Text {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    text: dayCell.day.weekday || dayCell.day.day
                                    color: Tokens.inkMuted
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny * root.s
                                    font.weight: Font.DemiBold
                                    elide: Text.ElideRight
                                }
                                Text {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    text: dayCell.day.icon || "cloud"
                                    color: Tokens.sun
                                    font.family: "Material Symbols Rounded"
                                    font.pixelSize: 21 * root.s
                                }
                                Text {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    text: dayCell.day.hi + "°"
                                    color: Tokens.ink
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fSmall * root.s
                                    font.weight: Font.DemiBold
                                }
                                Text {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    text: dayCell.day.lo + "°"
                                    color: Tokens.inkMuted
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny * root.s
                                }
                            }
                        }
                    }

                    Item {
                        width: parent.width
                        implicitHeight: 46 * root.s
                        visible: root.dailyMode === "range"
                        Sparkline {
                            anchors.fill: parent
                            anchors.margins: Tokens.s1 * root.s
                            values: root.dailyHighs()
                            stroke: Tokens.sun
                        }
                        Sparkline {
                            anchors.fill: parent
                            anchors.margins: Tokens.s1 * root.s
                            values: root.dailyLows()
                            stroke: Tokens.inkDim
                            strokeWidth: root.s
                        }
                    }
                }
            }

            Rectangle {
                visible: root.loaded
                width: parent.width
                implicitHeight: hourlyColumn.implicitHeight + root.pad * 2
                radius: Tokens.radius * root.s
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.line

                Column {
                    id: hourlyColumn
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: root.pad
                    spacing: Tokens.s3 * root.s

                    Text {
                        text: I18n.tr("Hourly")
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fRow * root.s
                        font.weight: Font.DemiBold
                    }
                    Row {
                        width: parent.width
                        Repeater {
                            model: Math.min(6, Weather.hourly.length)
                            delegate: Column {
                                id: hourCell
                                required property int index
                                readonly property var hour: Weather.hourly[hourCell.index]
                                width: parent.width / 6
                                spacing: Tokens.s1 * root.s
                                Text {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    text: hourCell.hour.time
                                    color: Tokens.inkMuted
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny * root.s
                                }
                                Text {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    text: hourCell.hour.icon || "cloud"
                                    color: Tokens.sun
                                    font.family: "Material Symbols Rounded"
                                    font.pixelSize: 20 * root.s
                                }
                                Text {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    text: hourCell.hour.temp + "°"
                                    color: Tokens.ink
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fSmall * root.s
                                    font.weight: Font.DemiBold
                                }
                            }
                        }
                    }
                    Sparkline {
                        width: parent.width
                        values: root.hourlyTemps()
                        stroke: Tokens.sun
                    }
                }
            }
        }
    }
}
