pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: results
    objectName: "typing-results"

    required property var engine
    property bool personalBest: false
    signal restart(bool sameSeed)

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: Tokens.s2
        spacing: Tokens.s4

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: Tokens.s6

            ColumnLayout {
                Layout.preferredWidth: 210
                spacing: 0
                Text {
                    text: I18n.tr("WPM")
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                }
                Text {
                    id: wpmValue
                    objectName: "typing-results-wpm"
                    text: String(Math.round(results.engine.wpm))
                    color: Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: 76
                    font.weight: Font.DemiBold
                }
                Text {
                    visible: opacity > 0
                    opacity: results.personalBest ? 1 : 0
                    scale: results.personalBest ? 1 : 0.96
                    text: I18n.tr("PERSONAL BEST")
                    color: Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackLabel
                    Behavior on opacity {
                        enabled: !Tokens.reduceMotion
                        NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
                    }
                    Behavior on scale {
                        enabled: !Tokens.reduceMotion
                        NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
                    }
                }
                Item { Layout.preferredHeight: Tokens.s4 }
                Text {
                    text: I18n.tr("ACCURACY")
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                }
                Text {
                    objectName: "typing-results-accuracy"
                    text: String(Math.round(results.engine.accuracy)) + "%"
                    color: Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fHero
                    font.weight: Font.DemiBold
                }
            }

            Rectangle {
                Layout.fillWidth: true
                Layout.fillHeight: true
                Layout.minimumHeight: 180
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.lineSoft
                radius: Tokens.radius

                Canvas {
                    id: chart
                    anchors.fill: parent
                    anchors.margins: Tokens.s3
                    antialiasing: true
                    readonly property var points: Array.from(results.engine.samples || [])
                    onPointsChanged: requestPaint()
                    onWidthChanged: requestPaint()
                    onHeightChanged: requestPaint()
                    onPaint: {
                        const ctx = getContext("2d");
                        ctx.reset();
                        if (points.length < 2)
                            return;
                        let maxValue = 1;
                        for (const point of points)
                            maxValue = Math.max(maxValue, Number(point.wpm || 0), Number(point.raw || 0));
                        function draw(key, color, width) {
                            ctx.beginPath();
                            for (let i = 0; i < points.length; i++) {
                                const x = i / (points.length - 1) * chart.width;
                                const y = chart.height - Number(points[i][key] || 0) / maxValue * (chart.height - 12) - 6;
                                if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
                            }
                            ctx.strokeStyle = color;
                            ctx.lineWidth = width;
                            ctx.stroke();
                        }
                        draw("raw", Tokens.lineStrong, 1);
                        draw("wpm", Tokens.ink, 2);
                    }
                }
                Text {
                    anchors.centerIn: parent
                    visible: results.engine.samples.length < 2
                    text: I18n.tr("Test too short to chart")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
            }
        }

        Rectangle { Layout.fillWidth: true; Layout.preferredHeight: 1; color: Tokens.lineSoft }

        GridLayout {
            Layout.fillWidth: true
            columns: 6
            columnSpacing: Tokens.s5
            rowSpacing: Tokens.s1
            Repeater {
                model: [
                    { label: I18n.tr("RAW"), value: Math.round(results.engine.rawWpm) },
                    { label: I18n.tr("CONSISTENCY"), value: Math.round(results.engine.consistency()) + "%" },
                    { label: I18n.tr("CORRECT"), value: results.engine.characterBreakdown.correct },
                    { label: I18n.tr("ERRORS"), value: results.engine.characterBreakdown.incorrect },
                    { label: I18n.tr("EXTRA"), value: results.engine.characterBreakdown.extra },
                    { label: I18n.tr("MISSED"), value: results.engine.characterBreakdown.missed }
                ]
                delegate: ColumnLayout {
                    required property var modelData
                    spacing: 0
                    Text {
                        text: parent.modelData.label
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fTiny
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Text {
                        text: String(parent.modelData.value)
                        color: Tokens.ink
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fValue
                    }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Item { Layout.fillWidth: true }
            Btn { text: I18n.tr("Repeat words"); onAct: results.restart(true) }
            Btn { text: I18n.tr("Next test"); primary: true; onAct: results.restart(false) }
        }
    }
}
