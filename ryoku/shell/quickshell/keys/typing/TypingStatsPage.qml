pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: stats

    required property var store
    property int rangeDays: 30
    readonly property real cutoff: rangeDays > 0 ? Date.now() - rangeDays * 86400000 : 0
    readonly property var ranged: Array.from(store.results).filter(entry =>
        cutoff <= 0 || Number(entry.timestamp || 0) >= cutoff).slice().reverse()
    readonly property real averageWpm: ranged.length
        ? ranged.reduce((sum, entry) => sum + Number(entry.wpm || 0), 0) / ranged.length : 0
    readonly property real bestWpm: ranged.reduce((best, entry) => Math.max(best, Number(entry.wpm || 0)), 0)
    readonly property real averageAccuracy: ranged.length
        ? ranged.reduce((sum, entry) => sum + Number(entry.accuracy || 0), 0) / ranged.length : 0

    function activityCount(offset) {
        const date = new Date();
        date.setHours(12, 0, 0, 0);
        date.setDate(date.getDate() - (363 - offset));
        const key = Qt.formatDate(date, "yyyy-MM-dd");
        const found = Array.from(store.activity).find(entry => entry.d === key);
        return found ? Number(found.n || 0) : 0;
    }

    function bucket(low, high) {
        return ranged.filter(entry => Number(entry.wpm || 0) >= low
            && (high < 0 || Number(entry.wpm || 0) < high)).length;
    }

    Flickable {
        id: scroll
        anchors.fill: parent
        clip: true
        contentWidth: width
        contentHeight: body.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        ScrollRail.vertical: ScrollRail { }
        WheelScroll { }

        ColumnLayout {
            id: body
            width: scroll.width
            spacing: Tokens.s5

            Empty {
                Layout.alignment: Qt.AlignCenter
                visible: stats.store.results.length === 0
                caption: stats.store.historyEnabled
                    ? I18n.tr("Finish a few tests to reveal pace, consistency and activity.")
                    : I18n.tr("Enable history to collect local statistics.")
            }

            RowLayout {
                visible: stats.store.results.length > 0
                Layout.fillWidth: true
                spacing: Tokens.s2
                Repeater {
                    model: [{ label: I18n.tr("7 DAYS"), days: 7 }, { label: I18n.tr("30 DAYS"), days: 30 },
                        { label: I18n.tr("90 DAYS"), days: 90 }, { label: I18n.tr("ALL TIME"), days: 0 }]
                    delegate: Btn {
                        required property var modelData
                        text: modelData.label
                        compact: true
                        primary: stats.rangeDays === modelData.days
                        onAct: stats.rangeDays = modelData.days
                    }
                }
                Item { Layout.fillWidth: true }
                Text {
                    text: I18n.tr("%1 TESTS").arg(String(stats.ranged.length))
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackLabel
                }
            }

            GridLayout {
                visible: stats.store.results.length > 0
                Layout.fillWidth: true
                columns: 4
                columnSpacing: Tokens.s5
                rowSpacing: Tokens.s2
                Repeater {
                    model: [
                        { label: I18n.tr("AVERAGE WPM"), value: Math.round(stats.averageWpm) },
                        { label: I18n.tr("BEST WPM"), value: Math.round(stats.bestWpm) },
                        { label: I18n.tr("ACCURACY"), value: Math.round(stats.averageAccuracy) + "%" },
                        { label: I18n.tr("TIME TYPING"), value: Math.round(stats.store.secondsTyping / 60) + "m" }
                    ]
                    delegate: Rectangle {
                        required property var modelData
                        required property int index
                        property bool entered: Tokens.reduceMotion
                        Layout.fillWidth: true
                        Layout.preferredHeight: 92
                        color: Tokens.tint5
                        radius: Tokens.radius
                        border.width: Tokens.border
                        border.color: Tokens.lineSoft
                        opacity: entered ? 1 : 0
                        transform: Translate {
                            y: parent.entered ? 0 : Tokens.s2
                            Behavior on y {
                                enabled: !Tokens.reduceMotion
                                NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                            }
                        }
                        Behavior on opacity {
                            enabled: !Tokens.reduceMotion
                            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                        }
                        Timer {
                            interval: Math.max(1, parent.index * 28)
                            running: stats.visible && !Tokens.reduceMotion && !parent.entered
                            repeat: false
                            onTriggered: parent.entered = true
                        }
                        Column {
                            anchors.centerIn: parent
                            spacing: Tokens.s1
                            Text { anchors.horizontalCenter: parent.horizontalCenter; text: parent.parent.modelData.label; color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; font.letterSpacing: Tokens.trackLabel }
                            Text { anchors.horizontalCenter: parent.horizontalCenter; text: String(parent.parent.modelData.value); color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fHero; font.weight: Font.DemiBold }
                        }
                    }
                }
            }

            Text {
                visible: stats.store.results.length > 0
                text: I18n.tr("PERSONAL BESTS")
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }

            GridLayout {
                visible: stats.store.results.length > 0
                Layout.fillWidth: true
                columns: 8
                columnSpacing: Tokens.s2
                rowSpacing: Tokens.s2
                Repeater {
                    model: [
                        { mode: "time", value: 15, label: "15s" }, { mode: "time", value: 30, label: "30s" },
                        { mode: "time", value: 60, label: "60s" }, { mode: "time", value: 120, label: "120s" },
                        { mode: "words", value: 10, label: "10w" }, { mode: "words", value: 25, label: "25w" },
                        { mode: "words", value: 50, label: "50w" }, { mode: "words", value: 100, label: "100w" }
                    ]
                    delegate: Rectangle {
                        required property var modelData
                        required property int index
                        property bool entered: Tokens.reduceMotion
                        readonly property var best: stats.store.bestOf(modelData.mode, modelData.value)
                        Layout.fillWidth: true
                        Layout.preferredHeight: 70
                        color: "transparent"
                        radius: Tokens.radius
                        border.width: Tokens.border
                        border.color: Tokens.lineSoft
                        opacity: entered ? 1 : 0
                        transform: Translate {
                            y: parent.entered ? 0 : Tokens.s2
                            Behavior on y {
                                enabled: !Tokens.reduceMotion
                                NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                            }
                        }
                        Behavior on opacity {
                            enabled: !Tokens.reduceMotion
                            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                        }
                        Timer {
                            interval: Math.max(1, parent.index * 18)
                            running: stats.visible && !Tokens.reduceMotion && !parent.entered
                            repeat: false
                            onTriggered: parent.entered = true
                        }
                        Column {
                            anchors.centerIn: parent
                            spacing: 0
                            Text { anchors.horizontalCenter: parent.horizontalCenter; text: parent.parent.modelData.label; color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny }
                            Text { anchors.horizontalCenter: parent.horizontalCenter; text: parent.parent.best ? Math.round(parent.parent.best.wpm) : "–"; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fValue }
                        }
                    }
                }
            }

            Text {
                visible: stats.store.results.length > 0
                text: I18n.tr("ACTIVITY · 52 WEEKS")
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }

            Grid {
                visible: stats.store.results.length > 0
                Layout.alignment: Qt.AlignHCenter
                columns: 52
                spacing: 3
                Repeater {
                    model: 364
                    delegate: Rectangle {
                        required property int index
                        readonly property int count: stats.activityCount(index)
                        width: 10
                        height: 10
                        color: count === 0 ? Tokens.tint5
                            : Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, Math.min(0.95, 0.2 + count * 0.16))
                        border.width: count === 0 ? Tokens.border : 0
                        border.color: Tokens.lineSoft
                        Behavior on color {
                            enabled: !Tokens.reduceMotion
                            ColorAnimation { duration: Tokens.snap }
                        }
                    }
                }
            }

            RowLayout {
                visible: stats.store.results.length > 0
                Layout.fillWidth: true
                Layout.preferredHeight: 230
                spacing: Tokens.s4

                Rectangle {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    color: "transparent"
                    radius: Tokens.radius
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft
                    Text { anchors.left: parent.left; anchors.top: parent.top; anchors.margins: Tokens.s3; text: I18n.tr("PACE OVER TIME"); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; font.letterSpacing: Tokens.trackLabel }
                    Canvas {
                        id: pace
                        anchors.fill: parent
                        anchors.margins: Tokens.s4
                        anchors.topMargin: Tokens.s6
                        readonly property var series: stats.ranged
                        onSeriesChanged: requestPaint()
                        onWidthChanged: requestPaint()
                        onHeightChanged: requestPaint()
                        onPaint: {
                            const ctx = getContext("2d"); ctx.reset();
                            if (series.length < 2) return;
                            const high = Math.max(1, stats.bestWpm);
                            ctx.beginPath();
                            for (let i = 0; i < series.length; i++) {
                                const x = i / (series.length - 1) * width;
                                const y = height - Number(series[i].wpm || 0) / high * (height - 8) - 4;
                                if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
                            }
                            ctx.strokeStyle = Tokens.ink; ctx.lineWidth = 2; ctx.stroke();
                        }
                    }
                    Text { anchors.centerIn: parent; visible: stats.ranged.length < 2; text: I18n.tr("More tests needed"); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                }

                Rectangle {
                    Layout.preferredWidth: Math.min(320, scroll.width * 0.34)
                    Layout.fillHeight: true
                    color: "transparent"
                    radius: Tokens.radius
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft
                    ColumnLayout {
                        anchors.fill: parent
                        anchors.margins: Tokens.s4
                        spacing: Tokens.s2
                        Text { text: I18n.tr("SPEED DISTRIBUTION"); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; font.letterSpacing: Tokens.trackLabel }
                        Repeater {
                            model: [
                                { label: "0–29", n: stats.bucket(0, 30) }, { label: "30–59", n: stats.bucket(30, 60) },
                                { label: "60–89", n: stats.bucket(60, 90) }, { label: "90–119", n: stats.bucket(90, 120) },
                                { label: "120+", n: stats.bucket(120, -1) }
                            ]
                            delegate: RowLayout {
                                id: speedRow
                                required property var modelData
                                Layout.fillWidth: true
                                Text { Layout.preferredWidth: 52; text: parent.modelData.label; color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny }
                                Rectangle {
                                    Layout.fillWidth: true
                                    height: 8
                                    color: Tokens.tint5
                                    Rectangle {
                                        width: parent.width * (stats.ranged.length ? speedRow.modelData.n / stats.ranged.length : 0)
                                        height: parent.height
                                        color: Tokens.ink
                                        Behavior on width {
                                            enabled: !Tokens.reduceMotion
                                            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                                        }
                                    }
                                }
                                Text { Layout.preferredWidth: 22; horizontalAlignment: Text.AlignRight; text: parent.modelData.n; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny }
                            }
                        }
                    }
                }
            }
        }
    }
}
